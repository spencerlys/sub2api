package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 账本后台任务：每分钟（多实例由 leader 锁选主）把截至 horizon（当前时间 − 延迟）的新入账与新请求按用户依序处理，
// 写入 usage_cost_splits，并在 usage_cost_ledger_users 中保存每个用户的剩余赠送/实付额度与已处理水位。
//
//   - 增量：只处理水位之后的事件；同一用户的事件处理在单个事务内，并以咨询锁串行化，可安全重复执行。
//   - 迟到事件：用户出现时间不晚于其水位的新请求，或水位之前的入账有任何增删改（含来源前缀修改），
//     该用户从头重建（简单且正确）。
//   - 全量重建：管理员 POST /admin/finance/rebuild-ledger 只登记请求，下一轮任务在 leader 锁内执行；
//     迁移 253 首次部署时也会登记一次，用于为历史用量补算拆分。
//
// 账本不在请求计费热路径上：只读取 usage_logs 与各类入账记录，从不修改余额。

const (
	usageCostLedgerLeaderLockKey = "usage:cost-ledger:leader"
	// usageCostLedgerLeaderLockTTL 只是崩溃兜底；正常情况每轮结束即释放。全量重建可能较慢，留足余量。
	usageCostLedgerLeaderLockTTL = 2 * time.Hour
	// UsageCostLedgerLag 只处理早于当前时间该延迟的事件，给异步写入的用量记录留出落库时间。
	UsageCostLedgerLag = 2 * time.Minute
	// usageCostLedgerLookbackIDs 候选扫描在 id 水位之下回看的行数，兜住提交顺序与 id 顺序不一致的少量记录。
	usageCostLedgerLookbackIDs int64 = 5000
	usageCostLedgerPageSize          = 5000
)

// LedgerUserState 用户账本状态：ProcessedUntil 之前（含）的全部事件均已计入。
type LedgerUserState struct {
	UserID         int64
	Buckets        LedgerBuckets
	ProcessedUntil time.Time
	// CreditFP 截至 ProcessedUntil 的全部入账指纹，用于发现历史入账的增删改。
	CreditFP      string
	LastRebuildAt *time.Time
}

// CostLedgerCursor 全局任务进度。
type CostLedgerCursor struct {
	LastUsageID        int64
	ProcessedUntil     *time.Time
	LastRunAt          *time.Time
	LastError          string
	RebuildRequestedAt *time.Time
	LastRebuildAt      *time.Time
}

// RebuildPending 存在晚于上次重建开始时间的重建请求。
func (c *CostLedgerCursor) RebuildPending() bool {
	if c == nil || c.RebuildRequestedAt == nil {
		return false
	}
	return c.LastRebuildAt == nil || c.RebuildRequestedAt.After(*c.LastRebuildAt)
}

// LedgerCandidate 本轮需要处理的用户；MinNewDebitAt 为扫描到的最早一条未拆分请求时间。
type LedgerCandidate struct {
	UserID        int64
	MinNewDebitAt *time.Time
}

// LedgerUserStore 单个用户的账本读写，处于同一事务内且已持有该用户的咨询锁。
type LedgerUserStore interface {
	State(ctx context.Context) (*LedgerUserState, error)
	// CreditFingerprint 截至 until（含）的全部入账指纹。
	CreditFingerprint(ctx context.Context, until time.Time) (string, error)
	// Credits 返回 (after, until] 内的入账（after 为 nil 表示从头），以及同一快照下截至 until 的入账指纹。
	Credits(ctx context.Context, after *time.Time, until time.Time) ([]LedgerCredit, string, error)
	// Debits 按 (created_at, id) 升序返回 (after, until] 内、键大于 (cursorAt, cursorID) 的扣费，最多 limit 条。
	// all=true（重建）时包含已拆分的请求，以及原始用量已被清理、只剩拆分记录的请求；否则只含尚未拆分的请求。
	Debits(ctx context.Context, after *time.Time, until, cursorAt time.Time, cursorID int64, all bool, limit int) ([]LedgerDebit, error)
	SaveSplits(ctx context.Context, splits []CostSplit) error
	SaveState(ctx context.Context, st LedgerUserState) error
}

// UsageCostLedgerRepository 账本任务的数据访问。
type UsageCostLedgerRepository interface {
	GetLedgerCursor(ctx context.Context) (*CostLedgerCursor, error)
	// SaveLedgerCursor 更新进度；不改动重建请求时间。LastRebuildAt 为 nil 时保留原值。
	SaveLedgerCursor(ctx context.Context, c CostLedgerCursor) error
	RequestLedgerRebuild(ctx context.Context, at time.Time) error
	// ListLedgerUsers 有过用量、拆分或入账的全部用户。
	ListLedgerUsers(ctx context.Context) ([]int64, error)
	// FindLedgerCandidates id > fromUsageID 且不晚于 horizon 的未拆分请求所属用户，
	// 以及入账有变化（水位前指纹不符或有水位后、不晚于 horizon 的新入账）的用户。
	FindLedgerCandidates(ctx context.Context, horizon time.Time, fromUsageID int64) ([]LedgerCandidate, error)
	// NextUsageWatermark 新的 id 水位：不越过任何晚于 horizon 的请求。
	NextUsageWatermark(ctx context.Context, prev int64, horizon time.Time) (int64, error)
	WithUserLedger(ctx context.Context, userID int64, fn func(ctx context.Context, store LedgerUserStore) error) error
}

// CostLedgerStatus 账本进度（财务与结算报表展示用）。
type CostLedgerStatus struct {
	ProcessedUntil *time.Time `json:"processed_until"`
	LastRunAt      *time.Time `json:"last_run_at"`
	// LagSeconds 当前时间距 ProcessedUntil 的秒数；从未运行为 -1。
	LagSeconds     int64      `json:"lag_seconds"`
	LastError      string     `json:"last_error"`
	RebuildPending bool       `json:"rebuild_pending"`
	LastRebuildAt  *time.Time `json:"last_rebuild_at"`
}

// LedgerRunResult 一轮处理的统计。
type LedgerRunResult struct {
	Users   int
	Rebuilt int
	Full    bool
	Errors  int
}

type UsageCostLedgerService struct {
	repo     UsageCostLedgerRepository
	now      func() time.Time
	lag      time.Duration
	interval time.Duration

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
}

func NewUsageCostLedgerService(repo UsageCostLedgerRepository, interval time.Duration) *UsageCostLedgerService {
	return &UsageCostLedgerService{
		repo:       repo,
		now:        time.Now,
		lag:        UsageCostLedgerLag,
		interval:   interval,
		instanceID: uuid.NewString(),
		stopCh:     make(chan struct{}),
	}
}

func (s *UsageCostLedgerService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *UsageCostLedgerService) Start() {
	if s == nil || s.repo == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		s.tick()
		for {
			select {
			case <-ticker.C:
				s.tick()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *UsageCostLedgerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *UsageCostLedgerService) tick() {
	lockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, usageCostLedgerLeaderLockKey, s.instanceID, usageCostLedgerLeaderLockTTL)
	cancel()
	if !ok {
		return
	}
	defer release()
	ctx, cancelRun := context.WithTimeout(context.Background(), usageCostLedgerLeaderLockTTL-time.Minute)
	defer cancelRun()
	res, err := s.RunOnce(ctx)
	if err != nil {
		slog.Warn("[UsageCostLedger] run failed", "error", err)
		return
	}
	if res.Users > 0 || res.Errors > 0 {
		slog.Info("[UsageCostLedger] processed", "users", res.Users, "rebuilt", res.Rebuilt, "full", res.Full, "errors", res.Errors)
	}
}

func (s *UsageCostLedgerService) horizon() time.Time {
	return s.now().Add(-s.lag).UTC().Truncate(time.Microsecond)
}

// RunOnce 执行一轮：有待处理的重建请求时全量重建，否则增量处理候选用户。
func (s *UsageCostLedgerService) RunOnce(ctx context.Context) (LedgerRunResult, error) {
	cur, err := s.repo.GetLedgerCursor(ctx)
	if err != nil {
		return LedgerRunResult{}, err
	}
	if cur == nil {
		cur = &CostLedgerCursor{}
	}
	horizon := s.horizon()
	if cur.ProcessedUntil != nil && horizon.Before(*cur.ProcessedUntil) {
		horizon = *cur.ProcessedUntil // 时钟回拨：不倒退水位
	}
	if cur.RebuildPending() {
		return s.rebuildAll(ctx, cur, horizon)
	}

	from := cur.LastUsageID - usageCostLedgerLookbackIDs
	if from < 0 {
		from = 0
	}
	cands, err := s.repo.FindLedgerCandidates(ctx, horizon, from)
	if err != nil {
		return LedgerRunResult{}, err
	}
	res := LedgerRunResult{}
	var firstErr error
	for _, c := range cands {
		rebuilt, err := s.processUser(ctx, c.UserID, horizon, false, c.MinNewDebitAt)
		if err != nil {
			res.Errors++
			if firstErr == nil {
				firstErr = fmt.Errorf("user %d: %w", c.UserID, err)
			}
			continue
		}
		res.Users++
		if rebuilt {
			res.Rebuilt++
		}
	}
	return res, s.finishRun(ctx, cur, horizon, firstErr, nil)
}

// RebuildAll 立即全量重建（测试与运维使用）；常规路径为 RequestRebuild + 下一轮任务。
func (s *UsageCostLedgerService) RebuildAll(ctx context.Context) (LedgerRunResult, error) {
	cur, err := s.repo.GetLedgerCursor(ctx)
	if err != nil {
		return LedgerRunResult{}, err
	}
	if cur == nil {
		cur = &CostLedgerCursor{}
	}
	return s.rebuildAll(ctx, cur, s.horizon())
}

func (s *UsageCostLedgerService) rebuildAll(ctx context.Context, cur *CostLedgerCursor, horizon time.Time) (LedgerRunResult, error) {
	started := s.now().UTC()
	users, err := s.repo.ListLedgerUsers(ctx)
	if err != nil {
		return LedgerRunResult{}, err
	}
	res := LedgerRunResult{Full: true}
	var firstErr error
	for _, id := range users {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if _, err := s.processUser(ctx, id, horizon, true, nil); err != nil {
			res.Errors++
			if firstErr == nil {
				firstErr = fmt.Errorf("user %d: %w", id, err)
			}
			continue
		}
		res.Users++
		res.Rebuilt++
	}
	var rebuiltAt *time.Time
	if firstErr == nil {
		rebuiltAt = &started
	}
	return res, s.finishRun(ctx, cur, horizon, firstErr, rebuiltAt)
}

// finishRun 保存进度。出错时不推进 id 与时间水位（下一轮重试），只记录错误。
func (s *UsageCostLedgerService) finishRun(ctx context.Context, cur *CostLedgerCursor, horizon time.Time, runErr error, rebuiltAt *time.Time) error {
	now := s.now().UTC()
	next := *cur
	next.LastRunAt = &now
	next.LastRebuildAt = rebuiltAt
	if runErr != nil {
		next.LastError = runErr.Error()
	} else {
		prev := cur.LastUsageID
		if rebuiltAt != nil {
			prev = 0 // 重建覆盖了全部历史，水位从头计算。
		}
		w, err := s.repo.NextUsageWatermark(ctx, prev, horizon)
		if err != nil {
			return err
		}
		next.LastUsageID = w
		h := horizon
		next.ProcessedUntil = &h
		next.LastError = ""
	}
	if err := s.repo.SaveLedgerCursor(ctx, next); err != nil {
		return err
	}
	return runErr
}

// processUser 处理单个用户到 horizon；返回是否从头重建。
func (s *UsageCostLedgerService) processUser(ctx context.Context, userID int64, horizon time.Time, force bool, minNewDebitAt *time.Time) (bool, error) {
	rebuilt := false
	err := s.repo.WithUserLedger(ctx, userID, func(ctx context.Context, store LedgerUserStore) error {
		state, err := store.State(ctx)
		if err != nil {
			return err
		}
		rebuild := force || state == nil
		if !rebuild {
			if minNewDebitAt != nil && !minNewDebitAt.After(state.ProcessedUntil) {
				rebuild = true // 迟到的请求落在已处理区间内
			} else {
				fp, err := store.CreditFingerprint(ctx, state.ProcessedUntil)
				if err != nil {
					return err
				}
				rebuild = fp != state.CreditFP // 已处理区间内的入账有变化
			}
			if !rebuild && !horizon.After(state.ProcessedUntil) {
				return nil
			}
		}
		// 首次建账不算重建。
		rebuilt = rebuild && (force || state != nil)

		var after *time.Time
		start := LedgerBuckets{}
		if !rebuild {
			pu := state.ProcessedUntil
			after = &pu
			start = state.Buckets
		}
		credits, fp, err := store.Credits(ctx, after, horizon)
		if err != nil {
			return err
		}
		SortLedgerCredits(credits)
		m := NewLedgerMerger(start, credits)

		var cursorAt time.Time
		var cursorID int64
		for {
			page, err := store.Debits(ctx, after, horizon, cursorAt, cursorID, rebuild, usageCostLedgerPageSize)
			if err != nil {
				return err
			}
			if len(page) == 0 {
				break
			}
			splits := make([]CostSplit, 0, len(page))
			for _, d := range page {
				sp := m.Debit(d)
				sp.UserID = userID
				splits = append(splits, sp)
			}
			if err := store.SaveSplits(ctx, splits); err != nil {
				return err
			}
			last := page[len(page)-1]
			cursorAt, cursorID = last.At, last.UsageLogID
			if len(page) < usageCostLedgerPageSize {
				break
			}
		}

		next := LedgerUserState{UserID: userID, Buckets: m.Finish(), ProcessedUntil: horizon, CreditFP: fp}
		if state != nil {
			next.LastRebuildAt = state.LastRebuildAt
		}
		if rebuilt {
			now := s.now().UTC()
			next.LastRebuildAt = &now
		}
		return store.SaveState(ctx, next)
	})
	return rebuilt, err
}

// RequestRebuild 登记一次全量重建，由下一轮任务执行。
func (s *UsageCostLedgerService) RequestRebuild(ctx context.Context) (*CostLedgerStatus, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("usage cost ledger unavailable")
	}
	if err := s.repo.RequestLedgerRebuild(ctx, s.now().UTC()); err != nil {
		return nil, err
	}
	return s.Status(ctx)
}

// Status 账本进度。
func (s *UsageCostLedgerService) Status(ctx context.Context) (*CostLedgerStatus, error) {
	if s == nil || s.repo == nil {
		return &CostLedgerStatus{LagSeconds: -1}, nil
	}
	cur, err := s.repo.GetLedgerCursor(ctx)
	if err != nil {
		return nil, err
	}
	return BuildCostLedgerStatus(cur, s.now()), nil
}

func BuildCostLedgerStatus(cur *CostLedgerCursor, now time.Time) *CostLedgerStatus {
	st := &CostLedgerStatus{LagSeconds: -1}
	if cur == nil {
		return st
	}
	st.ProcessedUntil = cur.ProcessedUntil
	st.LastRunAt = cur.LastRunAt
	st.LastError = cur.LastError
	st.RebuildPending = cur.RebuildPending()
	st.LastRebuildAt = cur.LastRebuildAt
	if cur.ProcessedUntil != nil {
		lag := int64(now.Sub(*cur.ProcessedUntil) / time.Second)
		if lag < 0 {
			lag = 0
		}
		st.LagSeconds = lag
	}
	return st
}
