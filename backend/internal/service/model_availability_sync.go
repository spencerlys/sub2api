package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 本地上游模型列表的同步任务（多实例由 leader 锁选主）。
//
// 每 10 分钟检查一次，只同步"到期"的账号：每个账号成功后在 24 小时 + 0~3 小时随机偏移后再同步，
// 失败则保留旧列表、1~3 小时后随机重试；首次见到的账号在 0~15 分钟内随机排期。
// 这样各账号的同步时刻彼此错开，不会同时打到上游。请求路径从不访问上游，只读这里写下的列表。

const (
	modelAvailabilityLeaderLockKey = "upstream-models:sync:leader"
	modelAvailabilityLeaderLockTTL = 30 * time.Minute
	modelAvailabilitySyncInterval  = 10 * time.Minute
	modelAvailabilityMaxPerTick    = 20
	modelAvailabilitySyncSpacing   = 2 * time.Second
	modelAvailabilityFetchTimeout  = 60 * time.Second

	modelAvailabilitySuccessPeriod = 24 * time.Hour
	modelAvailabilitySuccessJitter = 3 * time.Hour
	modelAvailabilityRetryMin      = time.Hour
	modelAvailabilityRetryJitter   = 2 * time.Hour
	modelAvailabilityFirstSpread   = 15 * time.Minute
	modelAvailabilityErrorMaxLen   = 300
)

// modelAvailabilityAccountStore 任务所需的账号读写（AccountRepository 的子集）。
type modelAvailabilityAccountStore interface {
	ListByPlatform(ctx context.Context, platform string) ([]Account, error)
	UpdateExtra(ctx context.Context, id int64, updates map[string]any) error
}

// upstreamModelFetcher 拉取账号的上游模型 ID（AccountTestService 实现，经账号自己的代理）。
type upstreamModelFetcher interface {
	FetchUpstreamSupportedModels(ctx context.Context, account *Account) ([]string, error)
}

type ModelAvailabilitySyncService struct {
	accounts modelAvailabilityAccountStore
	fetcher  upstreamModelFetcher
	interval time.Duration
	spacing  time.Duration
	now      func() time.Time
	// jitter 返回 [0, max) 内的均匀随机时长；测试可替换。
	jitter func(max time.Duration) time.Duration

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
}

func NewModelAvailabilitySyncService(accounts modelAvailabilityAccountStore, fetcher upstreamModelFetcher) *ModelAvailabilitySyncService {
	return &ModelAvailabilitySyncService{
		accounts:   accounts,
		fetcher:    fetcher,
		interval:   modelAvailabilitySyncInterval,
		spacing:    modelAvailabilitySyncSpacing,
		now:        time.Now,
		jitter:     uniformJitter,
		instanceID: uuid.NewString(),
		stopCh:     make(chan struct{}),
	}
}

func uniformJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(max)))
}

func (s *ModelAvailabilitySyncService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *ModelAvailabilitySyncService) Start() {
	if s == nil || s.accounts == nil || s.fetcher == nil || s.interval <= 0 {
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

func (s *ModelAvailabilitySyncService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *ModelAvailabilitySyncService) tick() {
	lockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, modelAvailabilityLeaderLockKey, s.instanceID, modelAvailabilityLeaderLockTTL)
	cancel()
	if !ok {
		return
	}
	defer release()
	ctx, cancelRun := context.WithTimeout(context.Background(), modelAvailabilityLeaderLockTTL-time.Minute)
	defer cancelRun()
	res, err := s.RunOnce(ctx)
	if err != nil {
		slog.Warn("[ModelAvailabilitySync] run failed", "error", err)
		return
	}
	if res.Synced > 0 || res.Failed > 0 {
		slog.Info("[ModelAvailabilitySync] synced", "synced", res.Synced, "failed", res.Failed, "scheduled", res.Scheduled)
	}
}

// ModelAvailabilityRunResult 一轮的统计。
type ModelAvailabilityRunResult struct {
	Synced    int
	Failed    int
	Scheduled int
}

// planModelAvailabilitySync 判断账号是否到期；从未排期的账号返回首次排期时间（0~15 分钟随机），本轮不同步。
func planModelAvailabilitySync(a *Account, now time.Time, jitter func(time.Duration) time.Duration) (due bool, firstNext *time.Time) {
	if next := a.UpstreamModelsNextSyncAt(); next != nil {
		return !now.Before(*next), nil
	}
	t := now.Add(jitter(modelAvailabilityFirstSpread))
	return false, &t
}

func modelAvailabilityNextAfterSuccess(now time.Time, jitter func(time.Duration) time.Duration) time.Time {
	return now.Add(modelAvailabilitySuccessPeriod + jitter(modelAvailabilitySuccessJitter))
}

func modelAvailabilityNextAfterFailure(now time.Time, jitter func(time.Duration) time.Duration) time.Time {
	return now.Add(modelAvailabilityRetryMin + jitter(modelAvailabilityRetryJitter))
}

// RunOnce 同步到期账号（最多 20 个，按到期先后、彼此间隔 2 秒）。
func (s *ModelAvailabilitySyncService) RunOnce(ctx context.Context) (ModelAvailabilityRunResult, error) {
	var res ModelAvailabilityRunResult
	accounts, err := s.accounts.ListByPlatform(ctx, PlatformAnthropic)
	if err != nil {
		return res, err
	}
	now := s.now()
	due := make([]*Account, 0)
	for i := range accounts {
		a := &accounts[i]
		if !modelAvailabilityEligible(a) || a.Status != StatusActive {
			continue
		}
		isDue, firstNext := planModelAvailabilitySync(a, now, s.jitter)
		if firstNext != nil {
			if err := s.accounts.UpdateExtra(ctx, a.ID, map[string]any{UpstreamModelsNextSyncAtExtraKey: firstNext.UTC().Format(time.RFC3339)}); err != nil {
				slog.Warn("[ModelAvailabilitySync] schedule failed", "account_id", a.ID, "error", err)
				continue
			}
			res.Scheduled++
			continue
		}
		if isDue {
			due = append(due, a)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		return due[i].UpstreamModelsNextSyncAt().Before(*due[j].UpstreamModelsNextSyncAt())
	})
	if len(due) > modelAvailabilityMaxPerTick {
		due = due[:modelAvailabilityMaxPerTick]
	}
	for i, a := range due {
		if i > 0 && s.spacing > 0 {
			select {
			case <-time.After(s.spacing):
			case <-ctx.Done():
				return res, ctx.Err()
			case <-s.stopCh:
				return res, nil
			}
		}
		if s.syncAccount(ctx, a) {
			res.Synced++
		} else {
			res.Failed++
		}
	}
	return res, nil
}

// syncAccount 拉取并保存一个账号的列表；失败时只更新重试时间与错误，旧列表保持不变。
func (s *ModelAvailabilitySyncService) syncAccount(ctx context.Context, a *Account) bool {
	fetchCtx, cancel := context.WithTimeout(ctx, modelAvailabilityFetchTimeout)
	models, err := s.fetcher.FetchUpstreamSupportedModels(fetchCtx, a)
	cancel()
	now := s.now()
	if err == nil && len(models) == 0 {
		err = errors.New("upstream returned no models")
	}
	if err != nil {
		updates := map[string]any{
			UpstreamModelsNextSyncAtExtraKey: modelAvailabilityNextAfterFailure(now, s.jitter).UTC().Format(time.RFC3339),
			UpstreamModelsLastErrorExtraKey:  modelAvailabilitySyncErrorMessage(err),
		}
		if uerr := s.accounts.UpdateExtra(ctx, a.ID, updates); uerr != nil {
			slog.Warn("[ModelAvailabilitySync] save failure failed", "account_id", a.ID, "error", uerr)
		}
		slog.Warn("[ModelAvailabilitySync] fetch failed", "account_id", a.ID, "error", modelAvailabilitySyncErrorMessage(err))
		return false
	}
	models = normalizeUpstreamModelIDs(models)
	updates := map[string]any{
		UpstreamModelsSyncedAtExtraKey:   now.UTC().Format(time.RFC3339),
		UpstreamModelsNextSyncAtExtraKey: modelAvailabilityNextAfterSuccess(now, s.jitter).UTC().Format(time.RFC3339),
		UpstreamModelsLastErrorExtraKey:  "",
	}
	// 列表没变时只写时间戳（不触发调度刷新）。
	if stored, _ := a.UpstreamModelList(); !slices.Equal(normalizeUpstreamModelIDs(stored), models) {
		updates[UpstreamModelsExtraKey] = models
	}
	if err := s.accounts.UpdateExtra(ctx, a.ID, updates); err != nil {
		slog.Warn("[ModelAvailabilitySync] save failed", "account_id", a.ID, "error", err)
		return false
	}
	return true
}

func normalizeUpstreamModelIDs(models []string) []string {
	out := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		key := strings.ToLower(m)
		if m == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

func modelAvailabilitySyncErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var syncErr *UpstreamModelSyncError
	if errors.As(err, &syncErr) {
		msg = syncErr.SafeMessage()
	}
	if len(msg) > modelAvailabilityErrorMaxLen {
		msg = msg[:modelAvailabilityErrorMaxLen]
	}
	return msg
}
