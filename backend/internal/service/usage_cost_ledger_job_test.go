//go:build unit

package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// memLedgerRepo 内存版账本仓储：语义与 SQL 实现一致（见 repository/usage_cost_ledger_repo.go）。
type memLedgerRepo struct {
	credits map[int64][]LedgerCredit // user → credits
	debits  []memDebit
	splits  map[int64]CostSplit
	states  map[int64]LedgerUserState
	cursor  CostLedgerCursor
}

type memDebit struct {
	UserID int64
	LedgerDebit
}

func newMemLedgerRepo() *memLedgerRepo {
	return &memLedgerRepo{credits: map[int64][]LedgerCredit{}, splits: map[int64]CostSplit{}, states: map[int64]LedgerUserState{}}
}

func (r *memLedgerRepo) addCredit(user int64, c LedgerCredit) {
	r.credits[user] = append(r.credits[user], c)
}

func (r *memLedgerRepo) addDebit(user int64, d LedgerDebit) {
	r.debits = append(r.debits, memDebit{UserID: user, LedgerDebit: d})
}

func (r *memLedgerRepo) fp(user int64, until time.Time) string {
	var parts []string
	for _, c := range r.credits[user] {
		if !c.At.After(until) {
			parts = append(parts, fmt.Sprintf("%s|%s|%s|%s|%s|%d", c.Key, c.Kind, c.Source, c.Amount, c.Bonus, c.At.UnixMicro()))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (r *memLedgerRepo) GetLedgerCursor(context.Context) (*CostLedgerCursor, error) {
	c := r.cursor
	return &c, nil
}

func (r *memLedgerRepo) SaveLedgerCursor(_ context.Context, c CostLedgerCursor) error {
	keep := r.cursor.LastRebuildAt
	req := r.cursor.RebuildRequestedAt
	r.cursor = c
	r.cursor.RebuildRequestedAt = req
	if c.LastRebuildAt == nil {
		r.cursor.LastRebuildAt = keep
	}
	return nil
}

func (r *memLedgerRepo) RequestLedgerRebuild(_ context.Context, at time.Time) error {
	r.cursor.RebuildRequestedAt = &at
	return nil
}

func (r *memLedgerRepo) ListLedgerUsers(context.Context) ([]int64, error) {
	set := map[int64]struct{}{}
	for u := range r.credits {
		set[u] = struct{}{}
	}
	for _, d := range r.debits {
		set[d.UserID] = struct{}{}
	}
	out := make([]int64, 0, len(set))
	for u := range set {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (r *memLedgerRepo) FindLedgerCandidates(_ context.Context, horizon time.Time, fromUsageID int64) ([]LedgerCandidate, error) {
	byUser := map[int64]*LedgerCandidate{}
	get := func(u int64) *LedgerCandidate {
		if c := byUser[u]; c != nil {
			return c
		}
		c := &LedgerCandidate{UserID: u}
		byUser[u] = c
		return c
	}
	for _, d := range r.debits {
		if _, done := r.splits[d.UsageLogID]; done || d.UsageLogID <= fromUsageID || d.At.After(horizon) {
			continue
		}
		c := get(d.UserID)
		if c.MinNewDebitAt == nil || d.At.Before(*c.MinNewDebitAt) {
			t := d.At
			c.MinNewDebitAt = &t
		}
	}
	for u, list := range r.credits {
		st, ok := r.states[u]
		for _, cr := range list {
			if cr.At.After(horizon) {
				continue
			}
			if !ok || cr.At.After(st.ProcessedUntil) {
				get(u)
			}
		}
		if ok && r.fp(u, st.ProcessedUntil) != st.CreditFP {
			get(u)
		}
	}
	out := make([]LedgerCandidate, 0, len(byUser))
	for _, c := range byUser {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out, nil
}

func (r *memLedgerRepo) NextUsageWatermark(_ context.Context, prev int64, horizon time.Time) (int64, error) {
	var maxID int64 = prev
	var minFuture int64
	for _, d := range r.debits {
		if d.UsageLogID > maxID {
			maxID = d.UsageLogID
		}
		if d.UsageLogID > prev && d.At.After(horizon) && (minFuture == 0 || d.UsageLogID < minFuture) {
			minFuture = d.UsageLogID
		}
	}
	if minFuture > 0 {
		return minFuture - 1, nil
	}
	return maxID, nil
}

func (r *memLedgerRepo) WithUserLedger(ctx context.Context, userID int64, fn func(context.Context, LedgerUserStore) error) error {
	return fn(ctx, &memLedgerStore{r: r, user: userID})
}

type memLedgerStore struct {
	r    *memLedgerRepo
	user int64
}

func (s *memLedgerStore) State(context.Context) (*LedgerUserState, error) {
	st, ok := s.r.states[s.user]
	if !ok {
		return nil, nil
	}
	return &st, nil
}

func (s *memLedgerStore) CreditFingerprint(_ context.Context, until time.Time) (string, error) {
	return s.r.fp(s.user, until), nil
}

func (s *memLedgerStore) Credits(_ context.Context, after *time.Time, until time.Time) ([]LedgerCredit, string, error) {
	var out []LedgerCredit
	for _, c := range s.r.credits[s.user] {
		if (after == nil || c.At.After(*after)) && !c.At.After(until) {
			out = append(out, c)
		}
	}
	return out, s.r.fp(s.user, until), nil
}

func (s *memLedgerStore) Debits(_ context.Context, after *time.Time, until, cursorAt time.Time, cursorID int64, all bool, limit int) ([]LedgerDebit, error) {
	var out []LedgerDebit
	for _, d := range s.r.debits {
		if d.UserID != s.user || d.At.After(until) || (after != nil && !d.At.After(*after)) {
			continue
		}
		if d.At.Before(cursorAt) || (d.At.Equal(cursorAt) && d.UsageLogID <= cursorID) {
			continue
		}
		if _, done := s.r.splits[d.UsageLogID]; done && !all {
			continue
		}
		out = append(out, d.LedgerDebit)
	}
	SortLedgerDebits(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *memLedgerStore) SaveSplits(_ context.Context, splits []CostSplit) error {
	for _, sp := range splits {
		s.r.splits[sp.UsageLogID] = sp
	}
	return nil
}

func (s *memLedgerStore) SaveState(_ context.Context, st LedgerUserState) error {
	s.r.states[s.user] = st
	return nil
}

func newTestLedgerService(repo UsageCostLedgerRepository, now *time.Time) *UsageCostLedgerService {
	svc := NewUsageCostLedgerService(repo, time.Minute)
	svc.now = func() time.Time { return *now }
	svc.lag = 0
	return svc
}

func (r *memLedgerRepo) requireSplit(t *testing.T, id int64, gift, paid string) {
	t.Helper()
	sp, ok := r.splits[id]
	require.True(t, ok, "usage %d not split", id)
	require.True(t, ldec(gift).Equal(sp.GiftCost), "usage %d gift want %s got %s", id, gift, sp.GiftCost)
	require.True(t, ldec(paid).Equal(sp.PaidCost), "usage %d paid want %s got %s", id, paid, sp.PaidCost)
}

func TestLedgerJobIncrementalProcessesOnlyUpToHorizon(t *testing.T) {
	repo := newMemLedgerRepo()
	now := lat(10)
	svc := newTestLedgerService(repo, &now)
	repo.addCredit(1, giftCredit("rc:1", 0, "2"))
	repo.addDebit(1, debit(1, 1, "1.5"))
	repo.addDebit(1, debit(2, 20, "1")) // 晚于 horizon，暂不处理

	res, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Users)
	repo.requireSplit(t, 1, "1.5", "0")
	_, pending := repo.splits[2]
	require.False(t, pending)
	require.Equal(t, int64(1), repo.cursor.LastUsageID, "watermark must not pass the unprocessed row")

	now = lat(30)
	repo.addCredit(1, offlineCredit("rc:2", 25, "9")) // 晚于第 2 条请求：不应资助它
	_, err = svc.RunOnce(context.Background())
	require.NoError(t, err)
	repo.requireSplit(t, 2, "0.5", "0.5")
	st := repo.states[1]
	requireBuckets(t, st.Buckets, "0", "9")
	require.Nil(t, st.LastRebuildAt, "plain appends never rebuild")
}

func TestLedgerJobLateCreditRebuildsUser(t *testing.T) {
	repo := newMemLedgerRepo()
	now := lat(10)
	svc := newTestLedgerService(repo, &now)
	repo.addDebit(1, debit(1, 2, "3"))
	repo.addDebit(2, debit(2, 2, "1"))
	_, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	repo.requireSplit(t, 1, "0", "3") // 无入账：透支记为实付

	// 迟到的赠送入账（时间早于已处理的请求）：该用户从头重建，其余用户不受影响。
	repo.addCredit(1, giftCredit("rc:9", 1, "2"))
	now = lat(11)
	res, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, LedgerRunResult{Users: 1, Rebuilt: 1}, res)
	repo.requireSplit(t, 1, "2", "1")
	require.NotNil(t, repo.states[1].LastRebuildAt)
	require.Nil(t, repo.states[2].LastRebuildAt)
}

func TestLedgerJobSourceEditRebuildsUser(t *testing.T) {
	repo := newMemLedgerRepo()
	now := lat(10)
	svc := newTestLedgerService(repo, &now)
	repo.addCredit(1, giftCredit("rc:1", 0, "5"))
	repo.addDebit(1, debit(1, 1, "2"))
	_, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	repo.requireSplit(t, 1, "2", "0")

	// 管理员把该记录改为线下实收：历史入账指纹变化 → 重建。
	repo.credits[1][0].Source = BalanceSourceOffline
	now = lat(11)
	res, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Rebuilt)
	repo.requireSplit(t, 1, "0", "2")
}

func TestLedgerJobLateDebitRebuildsUser(t *testing.T) {
	repo := newMemLedgerRepo()
	now := lat(10)
	svc := newTestLedgerService(repo, &now)
	repo.addCredit(1, giftCredit("rc:1", 0, "1"))
	repo.addDebit(1, debit(1, 5, "1"))
	_, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	repo.requireSplit(t, 1, "1", "0")

	// 晚写入、时间更早的请求：应排在前面吃掉赠送额度。
	repo.addDebit(1, debit(2, 3, "1"))
	now = lat(11)
	res, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Rebuilt)
	repo.requireSplit(t, 2, "1", "0")
	repo.requireSplit(t, 1, "0", "1")
}

func TestLedgerJobIncrementalEqualsFullRebuild(t *testing.T) {
	repo := newMemLedgerRepo()
	now := lat(0)
	svc := newTestLedgerService(repo, &now)
	var id int64
	for step := 0; step < 30; step++ {
		user := int64(step%3 + 1)
		switch step % 4 {
		case 0:
			repo.addCredit(user, giftCredit(fmt.Sprintf("rc:%d", step), step, "1.3"))
		case 1:
			repo.addCredit(user, offlineCredit(fmt.Sprintf("rc:%d", step), step, "0.7"))
		}
		id++
		repo.addDebit(user, debit(id, step, fmt.Sprintf("0.%d5", step%9+1)))
		now = lat(step)
		_, err := svc.RunOnce(context.Background())
		require.NoError(t, err)
	}
	incremental := map[int64]CostSplit{}
	for k, v := range repo.splits {
		incremental[k] = v
	}
	states := map[int64]LedgerUserState{}
	for k, v := range repo.states {
		states[k] = v
	}

	_, err := svc.RebuildAll(context.Background())
	require.NoError(t, err)
	require.Len(t, repo.splits, len(incremental))
	for k, v := range incremental {
		got := repo.splits[k]
		require.True(t, v.GiftCost.Equal(got.GiftCost) && v.PaidCost.Equal(got.PaidCost), "usage %d differs after rebuild", k)
	}
	for u, st := range states {
		require.True(t, st.Buckets.Gift.Equal(repo.states[u].Buckets.Gift) && st.Buckets.Paid.Equal(repo.states[u].Buckets.Paid), "user %d buckets", u)
	}
}

func TestLedgerJobRebuildRequestRunsOnNextTick(t *testing.T) {
	repo := newMemLedgerRepo()
	now := lat(10)
	svc := newTestLedgerService(repo, &now)
	repo.addCredit(1, giftCredit("rc:1", 0, "1"))
	repo.addDebit(1, debit(1, 1, "1"))

	st, err := svc.RequestRebuild(context.Background())
	require.NoError(t, err)
	require.True(t, st.RebuildPending)
	require.Equal(t, int64(-1), st.LagSeconds)

	res, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, res.Full)
	status, err := svc.Status(context.Background())
	require.NoError(t, err)
	require.False(t, status.RebuildPending)
	require.NotNil(t, status.LastRebuildAt)
	require.Equal(t, int64(0), status.LagSeconds)
	repo.requireSplit(t, 1, "1", "0")
}
