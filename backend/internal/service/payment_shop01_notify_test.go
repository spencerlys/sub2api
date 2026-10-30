//go:build unit

package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	"github.com/stretchr/testify/require"
)

const shop01NotifyTestKey = "test-merchant-key-not-real"

// shop01TestSign is an independent implementation of the SHOP01 signature
// (sorted k=v&..., empty values and sign dropped, key appended, md5 hex).
func shop01TestSign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(sum[:])
}

func shop01NotifyBody(outTradeNo, amount string) string {
	params := map[string]string{
		"merchant_no": "m1001", "order_no": "G" + outTradeNo, "out_trade_no": outTradeNo,
		"trade_no": "2026102422001400001", "amount": amount, "status": "TRADE_SUCCESS",
		"paid_at": "2026-10-24 12:00:00", "notify_time": "2026-10-24 12:00:01",
	}
	params["sign"] = shop01TestSign(params, shop01NotifyTestKey)
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return values.Encode()
}

type shop01NotifyEnv struct {
	client   *dbent.Client
	svc      *PaymentService
	order    *dbent.PaymentOrder
	prov     payment.Provider
	credited *float64
	redeem   *paymentFulfillmentRedeemRepo
}

func newShop01NotifyEnv(t *testing.T, payAmount float64) *shop01NotifyEnv {
	t.Helper()
	return newShop01NotifyEnvWith(t, payAmount, "https://gw.invalid", true)
}

// newShop01NotifyEnvWith lets a test point the instance at a fake gateway and
// skip the (order_id, action) audit unique index so duplicate audit rows are
// observable instead of being rejected by the database.
func newShop01NotifyEnvWith(t *testing.T, payAmount float64, apiBase string, auditUniqueIndex bool) *shop01NotifyEnv {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	if auditUniqueIndex {
		ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	}

	cfg := map[string]string{"merchantNo": "m1001", "merchantKey": shop01NotifyTestKey, "apiBase": apiBase, "notifyUrl": "https://app.test/api/v1/payment/webhook/shop01"}
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)
	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeShop01).SetName("shop01").SetConfig(string(cfgJSON)).
		SetSupportedTypes("alipay").SetEnabled(true).Save(ctx)
	require.NoError(t, err)
	instID := strconv.FormatInt(inst.ID, 10)

	user, err := client.User.Create().SetEmail("shop01-notify@example.com").SetPasswordHash("hash").SetUsername("shop01-notify").Save(ctx)
	require.NoError(t, err)
	out := "sub2_shop01_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(payAmount).SetPayAmount(payAmount).SetBonusAmount(0.01).SetFeeRate(0).
		SetRechargeCode("PAY-SHOP01-" + out).SetOutTradeNo(out).
		SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).SetStatus(OrderStatusPending).
		SetExpiresAt(time.Now().Add(30 * time.Minute)).SetClientIP("127.0.0.1").SetSrcHost("app.test").
		SetProviderInstanceID(instID).SetProviderKey(payment.TypeShop01).
		SetProviderSnapshot(map[string]any{"schema_version": 2, "provider_instance_id": instID, "provider_key": payment.TypeShop01, "merchant_id": "m1001", "currency": "CNY"}).
		Save(ctx)
	require.NoError(t, err)

	credited := 0.0
	userRepo := &mockUserRepo{getByIDUser: &User{ID: user.ID}}
	userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
		require.Equal(t, user.ID, id)
		credited += amount
		return nil
	}
	redeemRepo := &paymentFulfillmentRedeemRepo{}
	redeemService := NewRedeemService(redeemRepo, userRepo, nil, &paymentFulfillmentRedeemCacheStub{}, nil, client, nil, nil)
	svc := &PaymentService{entClient: client, redeemService: redeemService, userRepo: userRepo, loadBalancer: payment.NewDefaultLoadBalancer(client, nil)}

	prov, err := provider.CreateProvider(payment.TypeShop01, instID, cfg)
	require.NoError(t, err)
	return &shop01NotifyEnv{client: client, svc: svc, order: order, prov: prov, credited: &credited, redeem: redeemRepo}
}

func (e *shop01NotifyEnv) notify(t *testing.T, amount string) error {
	t.Helper()
	n, err := e.prov.VerifyNotification(context.Background(), shop01NotifyBody(e.order.OutTradeNo, amount), nil)
	require.NoError(t, err)
	return e.svc.HandlePaymentNotification(context.Background(), n, payment.TypeShop01)
}

func TestShop01DuplicateNotifyCreditsOnce(t *testing.T) {
	env := newShop01NotifyEnv(t, 10.01)
	ctx := context.Background()

	require.NoError(t, env.notify(t, "10.01"))
	require.NoError(t, env.notify(t, "10.01"), "a duplicate notify must be acked idempotently")
	require.NoError(t, env.notify(t, "10.01"))

	require.InDelta(t, 10.01, *env.credited, 1e-9, "credited exactly once, amount including the k-cent bonus")
	require.Equal(t, 1, env.redeem.createCalls)
	require.Len(t, env.redeem.useCalls, 1)
	reloaded, err := env.client.PaymentOrder.Get(ctx, env.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Equal(t, "2026102422001400001", reloaded.PaymentTradeNo)
}

func TestShop01AmountMismatchIsRejectedAuditedAndNotCredited(t *testing.T) {
	// 10.02 / 10.00 are inside the generic 0.01 CNY tolerance, but under
	// exact-amount matching they belong to a different order.
	for _, paid := range []string{"10.02", "10.00", "100.01"} {
		t.Run(paid, func(t *testing.T) {
			env := newShop01NotifyEnv(t, 10.01)
			ctx := context.Background()

			err := env.notify(t, paid)
			require.Error(t, err)
			require.Contains(t, err.Error(), "amount mismatch")

			require.Zero(t, *env.credited)
			require.Zero(t, env.redeem.createCalls)
			reloaded, err := env.client.PaymentOrder.Get(ctx, env.order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusPending, reloaded.Status)
			audits, err := env.client.PaymentAuditLog.Query().
				Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(env.order.ID, 10)), paymentauditlog.ActionEQ("PAYMENT_AMOUNT_MISMATCH")).
				Count(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, audits)
		})
	}
}

func TestShop01ExactAmountCents(t *testing.T) {
	c, err := shop01ExactAmountCents(0, map[string]string{payment.MetadataKeyExactAmount: "10.01"})
	require.NoError(t, err)
	require.Equal(t, int64(1001), c)
	_, err = shop01ExactAmountCents(0, map[string]string{payment.MetadataKeyExactAmount: "10.011"})
	require.Error(t, err)
	c, err = shop01ExactAmountCents(10.01, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1001), c)
	_, err = shop01ExactAmountCents(10.015, nil)
	require.Error(t, err)
}
