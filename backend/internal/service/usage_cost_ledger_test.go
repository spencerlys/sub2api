//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var ledgerT0 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

func ldec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func lat(min int) time.Time { return ledgerT0.Add(time.Duration(min) * time.Minute) }

func giftCredit(key string, min int, amount string) LedgerCredit {
	return LedgerCredit{Key: key, At: lat(min), Kind: CreditKindAdmin, Amount: ldec(amount)}
}

func offlineCredit(key string, min int, amount string) LedgerCredit {
	return LedgerCredit{Key: key, At: lat(min), Kind: CreditKindAdmin, Source: BalanceSourceOffline, Amount: ldec(amount)}
}

func debit(id int64, min int, cost string) LedgerDebit {
	return LedgerDebit{UsageLogID: id, At: lat(min), ActualCost: ldec(cost)}
}

func splitOf(t *testing.T, splits []CostSplit, id int64) CostSplit {
	t.Helper()
	for _, s := range splits {
		if s.UsageLogID == id {
			return s
		}
	}
	t.Fatalf("no split for usage log %d", id)
	return CostSplit{}
}

func requireSplit(t *testing.T, splits []CostSplit, id int64, gift, paid string) {
	t.Helper()
	s := splitOf(t, splits, id)
	require.True(t, ldec(gift).Equal(s.GiftCost), "usage %d gift: want %s got %s", id, gift, s.GiftCost)
	require.True(t, ldec(paid).Equal(s.PaidCost), "usage %d paid: want %s got %s", id, paid, s.PaidCost)
	require.True(t, s.GiftCost.Add(s.PaidCost).Equal(s.ActualCost), "gift+paid must equal actual_cost")
}

func requireBuckets(t *testing.T, b LedgerBuckets, gift, paid string) {
	t.Helper()
	require.True(t, ldec(gift).Equal(b.Gift), "gift bucket: want %s got %s", gift, b.Gift)
	require.True(t, ldec(paid).Equal(b.Paid), "paid bucket: want %s got %s", paid, b.Paid)
}

func TestLedgerGiftFirstThenPaid(t *testing.T) {
	b, splits := RunLedger(LedgerBuckets{}, []LedgerCredit{
		giftCredit("rc:1", 0, "3"),
		{Key: "rc:2", At: lat(0), Kind: CreditKindPayment, Amount: ldec("10"), Bonus: ldec("2")}, // 8 paid + 2 gift
	}, []LedgerDebit{debit(1, 1, "4"), debit(2, 2, "4.5"), debit(3, 3, "1.25")})

	requireSplit(t, splits, 1, "4", "0")    // gift bucket 5 → 1
	requireSplit(t, splits, 2, "1", "3.5")  // gift drained, rest paid
	requireSplit(t, splits, 3, "0", "1.25") // paid only
	requireBuckets(t, b, "0", "3.25")       // 8 − 3.5 − 1.25
}

func TestLedgerOverdraftCountsAsGift(t *testing.T) {
	b, splits := RunLedger(LedgerBuckets{}, []LedgerCredit{giftCredit("rc:1", 0, "1"), offlineCredit("rc:2", 0, "0.5")},
		[]LedgerDebit{debit(1, 1, "0.4"), debit(2, 2, "1"), debit(3, 3, "2")})
	requireSplit(t, splits, 1, "0.4", "0")
	requireSplit(t, splits, 2, "0.6", "0.4") // gift empties, then paid
	requireSplit(t, splits, 3, "1.9", "0.1") // paid empties; the unfunded overdraft is not paid money
	requireBuckets(t, b, "0", "0")           // buckets never go negative
}

func TestLedgerInterleavedCreditsFollowTime(t *testing.T) {
	_, splits := RunLedger(LedgerBuckets{}, []LedgerCredit{
		offlineCredit("rc:1", 0, "5"),
		giftCredit("rc:2", 10, "5"),
	}, []LedgerDebit{debit(1, 5, "2"), debit(2, 11, "6")})
	requireSplit(t, splits, 1, "0", "2") // gift arrives later: first request is paid
	requireSplit(t, splits, 2, "5", "1") // later request eats the new gift first
}

func TestLedgerCreditsBeforeDebitsOnTies(t *testing.T) {
	_, splits := RunLedger(LedgerBuckets{}, []LedgerCredit{offlineCredit("rc:0", 0, "5"), giftCredit("rc:1", 5, "2")},
		[]LedgerDebit{debit(1, 5, "1"), debit(2, 5, "3")})
	requireSplit(t, splits, 1, "1", "0")
	requireSplit(t, splits, 2, "1", "2")
}

func TestLedgerDebitsOrderedByTimeThenID(t *testing.T) {
	// Inputs out of order: the ledger sorts by (created_at, id).
	_, splits := RunLedger(LedgerBuckets{}, []LedgerCredit{giftCredit("rc:1", 0, "1"), offlineCredit("rc:2", 0, "2")},
		[]LedgerDebit{debit(9, 2, "1"), debit(8, 2, "1"), debit(7, 3, "1")})
	requireSplit(t, splits, 8, "1", "0")
	requireSplit(t, splits, 9, "0", "1")
	requireSplit(t, splits, 7, "0", "1")
}

func TestLedgerCreditClassification(t *testing.T) {
	cases := []struct {
		name       string
		credit     LedgerCredit
		gift, paid string
	}{
		{"untagged admin is gift", LedgerCredit{Kind: CreditKindAdmin, Amount: ldec("5")}, "5", "0"},
		{"tagged gift", LedgerCredit{Kind: CreditKindAdmin, Source: "gift", Amount: ldec("5")}, "5", "0"},
		{"unknown tag is gift", LedgerCredit{Kind: CreditKindAdmin, Source: "bogus", Amount: ldec("5")}, "5", "0"},
		{"offline is paid", LedgerCredit{Kind: CreditKindAdmin, Source: "offline", Amount: ldec("5")}, "0", "5"},
		{"positive correction is gift", LedgerCredit{Kind: CreditKindAdmin, Source: "correction", Amount: ldec("5")}, "5", "0"},
		{"payment splits bonus", LedgerCredit{Kind: CreditKindPayment, Amount: ldec("12"), Bonus: ldec("2")}, "2", "10"},
		{"payment bonus clamps to amount", LedgerCredit{Kind: CreditKindPayment, Amount: ldec("3"), Bonus: ldec("9")}, "3", "0"},
		{"promo is gift", LedgerCredit{Kind: CreditKindPromo, Amount: ldec("1")}, "1", "0"},
		{"redeem code is gift", LedgerCredit{Kind: CreditKindRedeemCode, Amount: ldec("1")}, "1", "0"},
		{"affiliate is gift", LedgerCredit{Kind: CreditKindAffiliate, Amount: ldec("1")}, "1", "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b LedgerBuckets
			b.ApplyCredit(tc.credit)
			requireBuckets(t, b, tc.gift, tc.paid)
		})
	}
}

// 负数规则：带 offline 标签的负数先冲减实付再冲减赠送；其余负数（含更正、无标签、赠送）先冲减赠送再冲减实付；
// 均不低于 0，超出部分忽略。
func TestLedgerNegativeAdjustmentRule(t *testing.T) {
	start := LedgerBuckets{Gift: ldec("3"), Paid: ldec("10")}

	b := start
	b.ApplyCredit(LedgerCredit{Kind: CreditKindAdmin, Amount: ldec("-5")})
	requireBuckets(t, b, "0", "8")

	b = start
	b.ApplyCredit(LedgerCredit{Kind: CreditKindAdmin, Source: BalanceSourceCorrection, Amount: ldec("-2")})
	requireBuckets(t, b, "1", "10")

	b = start
	b.ApplyCredit(LedgerCredit{Kind: CreditKindAdmin, Source: BalanceSourceOffline, Amount: ldec("-12")})
	requireBuckets(t, b, "1", "0")

	b = start
	b.ApplyCredit(LedgerCredit{Kind: CreditKindAdmin, Source: BalanceSourceGift, Amount: ldec("-100")})
	requireBuckets(t, b, "0", "0")
}

func TestLedgerZeroAndSubscriptionDebits(t *testing.T) {
	b, splits := RunLedger(LedgerBuckets{Gift: ldec("1")}, nil, []LedgerDebit{
		debit(1, 0, "0"),
		{UsageLogID: 2, At: lat(1), ActualCost: ldec("4"), BillingType: BillingTypeSubscription, SubscriptionPaid: true},
		{UsageLogID: 3, At: lat(2), ActualCost: ldec("4"), BillingType: BillingTypeSubscription},
	})
	requireSplit(t, splits, 1, "0", "0")
	requireSplit(t, splits, 2, "0", "4") // 付费订阅：实付，不动余额桶
	requireSplit(t, splits, 3, "4", "0") // 赠送/分配的订阅：赠送
	requireBuckets(t, b, "1", "0")
}

func TestLedgerIncrementalEqualsFull(t *testing.T) {
	credits := []LedgerCredit{
		giftCredit("rc:1", 0, "2"),
		offlineCredit("rc:2", 3, "5"),
		{Key: "af:1", At: lat(7), Kind: CreditKindAffiliate, Amount: ldec("1.5")},
		{Key: "rc:3", At: lat(9), Kind: CreditKindAdmin, Source: BalanceSourceCorrection, Amount: ldec("-0.5")},
	}
	debits := []LedgerDebit{debit(1, 1, "1.1"), debit(2, 4, "2"), debit(3, 6, "0.3"), debit(4, 8, "2.7"), debit(5, 10, "1")}

	full, fullSplits := RunLedger(LedgerBuckets{}, credits, debits)

	// 按水位 5 分两段处理：第一段只含 ≤5 的事件，第二段从第一段的余额继续。
	mid, first := RunLedger(LedgerBuckets{}, credits[:2], debits[:2])
	end, second := RunLedger(mid, credits[2:], debits[2:])
	require.True(t, full.Gift.Equal(end.Gift) && full.Paid.Equal(end.Paid))
	require.Equal(t, fullSplits, append(first, second...))
}
