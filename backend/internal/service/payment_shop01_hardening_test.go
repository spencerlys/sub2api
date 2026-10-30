//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func (e *shop01NotifyEnv) auditCount(t *testing.T, action string) int {
	t.Helper()
	n, err := e.client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(e.order.ID, 10)), paymentauditlog.ActionEQ(action)).
		Count(context.Background())
	require.NoError(t, err)
	return n
}

// setOrderTimes rewrites status and timestamps directly so a test can place
// the order anywhere in its lifecycle (ent always bumps updated_at itself).
func (e *shop01NotifyEnv) setOrderTimes(t *testing.T, status string, createdAt, updatedAt, expiresAt time.Time) {
	t.Helper()
	_, err := e.client.ExecContext(context.Background(),
		"UPDATE payment_orders SET status = ?, created_at = ?, updated_at = ?, expires_at = ? WHERE id = ?",
		status, createdAt, updatedAt, expiresAt, e.order.ID)
	require.NoError(t, err)
}

// --- Finding 1: late signed notify after the order was closed locally -------

func TestShop01LateNotifyRecoversClosedOrderWithinNotifyWindow(t *testing.T) {
	for _, closed := range []string{OrderStatusExpired, OrderStatusCancelled} {
		t.Run(closed, func(t *testing.T) {
			env := newShop01NotifyEnv(t, 10.01)
			ctx := context.Background()
			now := time.Now()
			// Closed well beyond the generic 5-minute expiry grace, but the order
			// is only 2h old: SHOP01 may still deliver the signed notify.
			env.setOrderTimes(t, closed, now.Add(-2*time.Hour), now.Add(-90*time.Minute), now.Add(-90*time.Minute))

			require.NoError(t, env.notify(t, "10.01"))
			require.NoError(t, env.notify(t, "10.01"), "a repeated late notify stays idempotent")

			require.InDelta(t, 10.01, *env.credited, 1e-9, "credited exactly once")
			require.Equal(t, 1, env.redeem.createCalls)
			reloaded, err := env.client.PaymentOrder.Get(ctx, env.order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusCompleted, reloaded.Status)
			require.Equal(t, 1, env.auditCount(t, "ORDER_RECOVERED"))
		})
	}
}

func TestShop01LateNotifyOutsideNotifyWindowIsNotAckedAndAuditedOnce(t *testing.T) {
	for _, closed := range []string{OrderStatusExpired, OrderStatusCancelled} {
		t.Run(closed, func(t *testing.T) {
			env := newShop01NotifyEnvWith(t, 10.01, "https://gw.invalid", false)
			ctx := context.Background()
			now := time.Now()
			env.setOrderTimes(t, closed, now.Add(-25*time.Hour), now.Add(-24*time.Hour), now.Add(-24*time.Hour))

			err := env.notify(t, "10.01")
			require.Error(t, err, "a payment we can no longer credit must not be acked")
			require.Contains(t, err.Error(), "notify window")
			require.Error(t, env.notify(t, "10.01"))

			require.Zero(t, *env.credited)
			require.Zero(t, env.redeem.createCalls)
			reloaded, err := env.client.PaymentOrder.Get(ctx, env.order.ID)
			require.NoError(t, err)
			require.Equal(t, closed, reloaded.Status)
			require.Equal(t, 1, env.auditCount(t, "PAYMENT_AFTER_NOTIFY_WINDOW"), "visible for manual handling, written once")
		})
	}
}

// --- Finding 5: unsigned "paid" query results ---------------------------------

func newShop01PaidQueryGateway(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/query" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		out := r.PostForm.Get("out_trade_no")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "ok", "data": map[string]any{
			"order_no": "G" + out, "out_trade_no": out, "amount": "10.01", "status": 1,
			"trade_no": "T" + out, "paid_at": "2026-10-24 12:00:00",
		}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestShop01UnsignedPaidQueryAuditIsWrittenOnce(t *testing.T) {
	env := newShop01NotifyEnvWith(t, 10.01, newShop01PaidQueryGateway(t).URL, false)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		require.Equal(t, checkPaidResultAlreadyPaid, env.svc.reconcilePaid(ctx, env.order))
	}
	require.Equal(t, 1, env.auditCount(t, "PAYMENT_QUERY_PAID_AWAITING_NOTIFY"))
	reloaded, err := env.client.PaymentOrder.Get(ctx, env.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, reloaded.Status)
	require.Zero(t, *env.credited)
}

func TestShop01UnsignedPaidQueryOrderExpiresAfterNotifyWindow(t *testing.T) {
	env := newShop01NotifyEnvWith(t, 10.01, newShop01PaidQueryGateway(t).URL, true)
	ctx := context.Background()
	now := time.Now()

	// Inside the window the unsigned "paid" answer still holds off expiry.
	env.setOrderTimes(t, OrderStatusPending, now.Add(-2*time.Hour), now.Add(-2*time.Hour), now.Add(-90*time.Minute))
	_, err := env.svc.ExpireTimedOutOrders(ctx)
	require.NoError(t, err)
	reloaded, err := env.client.PaymentOrder.Get(ctx, env.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, reloaded.Status)

	// Past the window it expires normally; crediting is manual from here.
	env.setOrderTimes(t, OrderStatusPending, now.Add(-25*time.Hour), now.Add(-25*time.Hour), now.Add(-24*time.Hour))
	n, err := env.svc.ExpireTimedOutOrders(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	reloaded, err = env.client.PaymentOrder.Get(ctx, env.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusExpired, reloaded.Status)
	require.Zero(t, *env.credited)
}

// --- Finding 3: merchant numbers are unique across SHOP01 instances ---------

func TestShop01InstanceMerchantNoMustBeUnique(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewPaymentConfigService(client, &paymentConfigSettingRepoStub{values: map[string]string{}}, nil)
	cfg := func(merchantNo string) map[string]string {
		return map[string]string{"merchantNo": merchantNo, "merchantKey": shop01NotifyTestKey, "apiBase": "https://gw.invalid", "notifyUrl": "https://app.test/api/v1/payment/webhook/shop01"}
	}
	create := func(name, merchantNo string) (int64, error) {
		inst, err := svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{
			ProviderKey: payment.TypeShop01, Name: name, Config: cfg(merchantNo),
			SupportedTypes: []string{payment.TypeAlipay}, PaymentMode: "qrcode",
		})
		if err != nil {
			return 0, err
		}
		return int64(inst.ID), nil
	}

	first, err := create("shop01-a", "m1001")
	require.NoError(t, err)

	_, err = create("shop01-dup", " m1001 ")
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "SHOP01_MERCHANT_NO_DUPLICATE", infraerrors.Reason(err))

	second, err := create("shop01-b", "m1002")
	require.NoError(t, err)

	_, err = svc.UpdateProviderInstance(ctx, second, UpdateProviderInstanceRequest{Config: cfg("m1001")})
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))

	// Re-saving an instance with its own merchant number is fine.
	_, err = svc.UpdateProviderInstance(ctx, first, UpdateProviderInstanceRequest{Config: cfg("m1001")})
	require.NoError(t, err)

	// Other providers are unaffected by SHOP01 merchant numbers.
	_, err = svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{
		ProviderKey: payment.TypeEasyPay, Name: "easypay", Config: map[string]string{"merchantNo": "m1001"},
	})
	require.NoError(t, err)
}
