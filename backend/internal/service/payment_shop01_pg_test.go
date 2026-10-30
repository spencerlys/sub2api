//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const shop01PGTestKey = "test-merchant-key-not-real"

// newShop01PGClient builds an ent client on an isolated schema of the
// disposable PostgreSQL given by PAYMENT_SHOP01_TEST_DSN (keyword format, same
// convention as the other DB-backed tests) and applies migration 251 on top
// of the ent schema so the partial unique index is exercised too.
func newShop01PGClient(t *testing.T) (*dbent.Client, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("PAYMENT_SHOP01_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL via PAYMENT_SHOP01_TEST_DSN")
	}
	if strings.Contains(dsn, "postgres://") {
		t.Fatal("PAYMENT_SHOP01_TEST_DSN must use keyword format")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("wsb_shop01_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = admin.Close()
	})
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	db.SetMaxOpenConns(40)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Schema.Create(context.Background()))
	migration, err := os.ReadFile("../../migrations/251_payment_shop01_pending_amount.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	return client, db
}

// shop01TestGateway emulates /api/submit and enforces the gateway's own
// invariant: two pending orders with the same amount never coexist.
type shop01TestGateway struct {
	mu      sync.Mutex
	pending map[string]string // money -> out_trade_no
	submits int
	paid    map[string]string // out_trade_no -> money reported paid by /api/query
}

func newShop01TestGateway(t *testing.T) (*shop01TestGateway, *httptest.Server) {
	gw := &shop01TestGateway{pending: map[string]string{}, paid: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/query" && r.Method == http.MethodPost {
			_ = r.ParseForm()
			out := r.PostForm.Get("out_trade_no")
			gw.mu.Lock()
			money, paid := gw.paid[out]
			gw.mu.Unlock()
			status := 0
			if paid {
				status = 1
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "ok", "data": map[string]any{
				"order_no": "G" + out, "out_trade_no": out, "amount": money, "status": status,
				"trade_no": "T" + out, "paid_at": "2026-10-24 12:00:00", "notify_status": 0,
			}})
			return
		}
		if r.URL.Path != "/api/submit" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		money := r.PostForm.Get("money")
		out := r.PostForm.Get("out_trade_no")
		gw.mu.Lock()
		defer gw.mu.Unlock()
		gw.submits++
		if prev, ok := gw.pending[money]; ok {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "相同金额订单未支付: " + prev})
			return
		}
		gw.pending[money] = out
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "下单成功", "data": map[string]any{
			"order_no": "G" + out, "pay_url": "https://gw.test/pay/" + out,
			"qrcode_url": "https://gw.test/qr/" + out + ".png", "amount": money, "expire_minutes": 30,
		}})
	}))
	t.Cleanup(srv.Close)
	return gw, srv
}

type shop01CreateOrderEnv struct {
	client   *dbent.Client
	db       *sql.DB
	svc      *PaymentService
	instance *dbent.PaymentProviderInstance
	users    []int64
	gateway  *shop01TestGateway
}

func newShop01CreateOrderEnv(t *testing.T, userCount int) *shop01CreateOrderEnv {
	t.Helper()
	ctx := context.Background()
	client, db := newShop01PGClient(t)
	gw, srv := newShop01TestGateway(t)
	cfgJSON, err := json.Marshal(map[string]string{
		"merchantNo": "m1001", "merchantKey": shop01PGTestKey,
		"apiBase": srv.URL, "notifyUrl": "https://app.test/api/v1/payment/webhook/shop01",
	})
	require.NoError(t, err)
	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeShop01).SetName("shop01-test").SetConfig(string(cfgJSON)).
		SetSupportedTypes("alipay").SetPaymentMode("qrcode").SetEnabled(true).Save(ctx)
	require.NoError(t, err)

	usersByID := make(map[int64]*User, userCount)
	ids := make([]int64, 0, userCount)
	for i := 0; i < userCount; i++ {
		u, err := client.User.Create().
			SetEmail(fmt.Sprintf("shop01-%d@example.com", i)).
			SetPasswordHash("hash").SetUsername(fmt.Sprintf("shop01-%d", i)).Save(ctx)
		require.NoError(t, err)
		usersByID[u.ID] = &User{ID: u.ID, Email: u.Email, Username: u.Username, Status: payment.EntityStatusActive}
		ids = append(ids, u.ID)
	}
	settings := &settingRepoStub{values: map[string]string{
		SettingPaymentEnabled:      "true",
		SettingMinRechargeAmount:   "1",
		SettingMaxPendingOrders:    "3",
		SettingEnabledPaymentTypes: "alipay",
	}}
	configService := NewPaymentConfigService(client, settings, nil)
	svc := NewPaymentService(client, payment.NewRegistry(), payment.NewDefaultLoadBalancer(client, nil), nil, nil, configService, &userRepoStub{usersByID: usersByID}, nil, nil)
	return &shop01CreateOrderEnv{client: client, db: db, svc: svc, instance: inst, users: ids, gateway: gw}
}

func (e *shop01CreateOrderEnv) createOrder(ctx context.Context, userID int64, amount float64) (*CreateOrderResponse, error) {
	return e.svc.CreateOrder(ctx, CreateOrderRequest{
		UserID: userID, Amount: amount, PaymentType: payment.TypeAlipay,
		OrderType: payment.OrderTypeBalance, ClientIP: "127.0.0.1", SrcHost: "app.test",
	})
}

func TestShop01CreateOrderConcurrentAllocatesDistinctAmountsPG(t *testing.T) {
	env := newShop01CreateOrderEnv(t, 101)
	ctx := context.Background()

	const concurrent = 50
	type result struct {
		resp *CreateOrderResponse
		err  error
	}
	results := make([]result, concurrent)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			resp, err := env.createOrder(ctx, env.users[i], 10)
			results[i] = result{resp, err}
		}(i)
	}
	close(start)
	wg.Wait()

	seen := map[int64]bool{}
	for i, r := range results {
		require.NoError(t, r.err, "order %d", i)
		cents := amountToCents(r.resp.PayAmount)
		require.False(t, seen[cents], "pay amount %d cents allocated twice", cents)
		seen[cents] = true
		require.GreaterOrEqual(t, cents, int64(1000))
		require.LessOrEqual(t, cents, int64(1049), "50 orders must use the 50 smallest offsets")
		// The payer pays k extra cents and is credited exactly that (multiplier 1).
		require.Equal(t, cents, amountToCents(r.resp.Amount), "credited amount must equal paid amount")
		require.Equal(t, cents-1000, amountToCents(r.resp.BonusAmount))
		require.Equal(t, "https://gw.test/pay/"+r.resp.OutTradeNo, r.resp.PayURL)
		require.Equal(t, "https://gw.test/qr/"+r.resp.OutTradeNo+".png", r.resp.QRCodeImg)
		require.Empty(t, r.resp.QRCode, "qrcode_url is an image URL and must not be re-encoded as QR content")
	}
	require.Len(t, seen, concurrent)

	orders, err := env.client.PaymentOrder.Query().Where(paymentorder.StatusEQ(OrderStatusPending)).All(ctx)
	require.NoError(t, err)
	require.Len(t, orders, concurrent)
	for _, o := range orders {
		require.Equal(t, payment.TypeShop01, valueOrEmpty(o.ProviderKey))
		require.NotNil(t, o.QrCodeImg)
		require.Equal(t, "https://gw.test/qr/"+o.OutTradeNo+".png", *o.QrCodeImg)
		require.Nil(t, o.QrCode)
		require.Equal(t, "https://gw.test/pay/"+o.OutTradeNo, valueOrEmpty(o.PayURL))
		require.Equal(t, "m1001", o.ProviderSnapshot["merchant_id"])
	}

	// Fill the remaining 50 offsets, then the 101st must be refused.
	for i := concurrent; i < 100; i++ {
		resp, err := env.createOrder(ctx, env.users[i], 10)
		require.NoError(t, err, "order %d", i)
		require.False(t, seen[amountToCents(resp.PayAmount)])
		seen[amountToCents(resp.PayAmount)] = true
	}
	require.Len(t, seen, 100)
	_, err = env.createOrder(ctx, env.users[100], 10)
	require.Error(t, err)
	require.Equal(t, "PAYMENT_CHANNEL_BUSY", infraerrors.Reason(err))
	require.Equal(t, http.StatusTooManyRequests, infraerrors.Code(err))
	require.Contains(t, err.Error(), "payment channel busy, please retry in a minute")
	require.Equal(t, 100, env.gateway.submits, "a busy channel must not reach the gateway")
}

func TestShop01RecentlyClosedAmountIsNotReusedWithinTenMinutesPG(t *testing.T) {
	env := newShop01CreateOrderEnv(t, 6)
	ctx := context.Background()

	first, err := env.createOrder(ctx, env.users[0], 20)
	require.NoError(t, err)
	require.Equal(t, int64(2000), amountToCents(first.PayAmount))

	for _, closed := range []string{OrderStatusExpired, OrderStatusCancelled} {
		t.Run(closed, func(t *testing.T) {
			_, err := env.client.PaymentOrder.Update().Where(paymentorder.StatusEQ(OrderStatusPending)).SetStatus(closed).Save(ctx)
			require.NoError(t, err)
			// Gateway side: its own pending order also lapses.
			env.gateway.mu.Lock()
			env.gateway.pending = map[string]string{}
			env.gateway.mu.Unlock()

			next, err := env.createOrder(ctx, env.users[1], 20)
			require.NoError(t, err)
			require.NotEqual(t, int64(2000), amountToCents(next.PayAmount),
				"an amount closed <10 minutes ago must not be reused: a late payer would be credited to the new order")

			_, err = env.client.PaymentOrder.Update().Where(paymentorder.StatusEQ(OrderStatusPending)).SetStatus(OrderStatusCompleted).Save(ctx)
			require.NoError(t, err)
			resetGateway := func() {
				env.gateway.mu.Lock()
				env.gateway.pending = map[string]string{}
				env.gateway.mu.Unlock()
			}

			// Closed >10 minutes ago but created <35 minutes ago: the gateway still
			// keeps that order payable, so the amount stays occupied.
			_, err = env.db.Exec(`UPDATE payment_orders SET updated_at = NOW() - INTERVAL '11 minutes' WHERE status = $1`, closed)
			require.NoError(t, err)
			resetGateway()
			stillHeld, err := env.createOrder(ctx, env.users[2], 20)
			require.NoError(t, err)
			require.NotEqual(t, int64(2000), amountToCents(stillHeld.PayAmount),
				"an amount whose gateway order may still be payable must not be reused")
			_, err = env.client.PaymentOrder.Update().Where(paymentorder.StatusEQ(OrderStatusPending)).SetStatus(OrderStatusCompleted).Save(ctx)
			require.NoError(t, err)

			// Past both the local reserve and the gateway TTL + margin, the amount is free again.
			_, err = env.db.Exec(`UPDATE payment_orders SET updated_at = NOW() - INTERVAL '11 minutes', created_at = NOW() - INTERVAL '36 minutes' WHERE status = $1`, closed)
			require.NoError(t, err)
			resetGateway()
			reused, err := env.createOrder(ctx, env.users[3], 20)
			require.NoError(t, err)
			require.Equal(t, int64(2000), amountToCents(reused.PayAmount))
		})
	}
}

func TestShop01RecentlyPaidAmountIsNotReusedWithinTenMinutesPG(t *testing.T) {
	env := newShop01CreateOrderEnv(t, 3)
	ctx := context.Background()
	resetGateway := func() {
		env.gateway.mu.Lock()
		env.gateway.pending = map[string]string{}
		env.gateway.mu.Unlock()
	}

	first, err := env.createOrder(ctx, env.users[0], 20)
	require.NoError(t, err)
	require.Equal(t, int64(2000), amountToCents(first.PayAmount))
	// Paid just now and old enough that only the paid reservation can hold it.
	_, err = env.db.Exec(`UPDATE payment_orders SET status = $1, paid_at = NOW(), created_at = NOW() - INTERVAL '40 minutes', updated_at = NOW() - INTERVAL '11 minutes' WHERE id = $2`,
		OrderStatusCompleted, first.OrderID)
	require.NoError(t, err)
	resetGateway()

	next, err := env.createOrder(ctx, env.users[1], 20)
	require.NoError(t, err)
	require.NotEqual(t, int64(2000), amountToCents(next.PayAmount),
		"an accidental second payment of a just-paid amount must not match a new order")
	_, err = env.client.PaymentOrder.Update().Where(paymentorder.StatusEQ(OrderStatusPending)).SetStatus(OrderStatusCancelled).Save(ctx)
	require.NoError(t, err)
	_, err = env.db.Exec(`UPDATE payment_orders SET created_at = NOW() - INTERVAL '40 minutes', updated_at = NOW() - INTERVAL '11 minutes' WHERE status = $1`, OrderStatusCancelled)
	require.NoError(t, err)

	// Paid more than 10 minutes ago: the amount is free again.
	_, err = env.db.Exec(`UPDATE payment_orders SET paid_at = NOW() - INTERVAL '11 minutes' WHERE id = $1`, first.OrderID)
	require.NoError(t, err)
	resetGateway()
	reused, err := env.createOrder(ctx, env.users[2], 20)
	require.NoError(t, err)
	require.Equal(t, int64(2000), amountToCents(reused.PayAmount))
}

func TestShop01PendingAmountUniqueIndexPG(t *testing.T) {
	client, _ := newShop01PGClient(t)
	ctx := context.Background()
	u, err := client.User.Create().SetEmail("idx@example.com").SetPasswordHash("h").SetUsername("idx").Save(ctx)
	require.NoError(t, err)
	mk := func(out, provider, instance, status string, pay float64) error {
		_, err := client.PaymentOrder.Create().
			SetUserID(u.ID).SetUserEmail(u.Email).SetUserName(u.Username).
			SetAmount(pay).SetPayAmount(pay).SetFeeRate(0).SetRechargeCode("").
			SetOutTradeNo(out).SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("").
			SetOrderType(payment.OrderTypeBalance).SetStatus(status).
			SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("app.test").
			SetProviderInstanceID(instance).SetProviderKey(provider).Save(ctx)
		return err
	}
	require.NoError(t, mk("o1", payment.TypeShop01, "7", OrderStatusPending, 10.01))
	err = mk("o2", payment.TypeShop01, "7", OrderStatusPending, 10.01)
	require.Error(t, err, "two PENDING shop01 orders of one instance must not share a pay amount")
	require.Contains(t, err.Error(), "paymentorder_shop01_pending_pay_amount_unique")
	// Outside the predicate the index must not interfere.
	require.NoError(t, mk("o3", payment.TypeShop01, "8", OrderStatusPending, 10.01))
	require.NoError(t, mk("o4", payment.TypeShop01, "7", OrderStatusExpired, 10.01))
	require.NoError(t, mk("o5", payment.TypeEasyPay, "7", OrderStatusPending, 10.01))
	require.NoError(t, mk("o6", payment.TypeShop01, "7", OrderStatusPending, 10.02))
}

func TestShop01AdvisoryLockKeyIsPerInstance(t *testing.T) {
	require.Equal(t, shop01AdvisoryLockKey("12"), shop01AdvisoryLockKey(" 12 "))
	require.NotEqual(t, shop01AdvisoryLockKey("12"), shop01AdvisoryLockKey("13"))
	require.Equal(t, int64(1001), amountToCents(10.01))
	require.Equal(t, int64(29), amountToCents(0.29))
	require.Equal(t, "0.29", strconv.FormatFloat(centsToAmount(29), 'f', -1, 64))
}

func TestShop01OrderValidityIsAtLeastGatewayTTLPG(t *testing.T) {
	env := newShop01CreateOrderEnv(t, 1)
	ctx := context.Background()
	env.svc.configService.settingRepo.(*settingRepoStub).values[SettingOrderTimeoutMinutes] = "15"

	resp, err := env.createOrder(ctx, env.users[0], 12)
	require.NoError(t, err)
	order, err := env.client.PaymentOrder.Get(ctx, resp.OrderID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Until(order.ExpiresAt), shop01GatewayOrderTTL-time.Minute,
		"a 15-minute local timeout must not expire an order the gateway still accepts payment for")
}

func TestShop01QueryPaidDoesNotCreditWithoutSignedNotifyPG(t *testing.T) {
	env := newShop01CreateOrderEnv(t, 1)
	ctx := context.Background()

	resp, err := env.createOrder(ctx, env.users[0], 15)
	require.NoError(t, err)
	order, err := env.client.PaymentOrder.Get(ctx, resp.OrderID)
	require.NoError(t, err)
	env.gateway.mu.Lock()
	env.gateway.paid[order.OutTradeNo] = fmt.Sprintf("%.2f", order.PayAmount)
	env.gateway.mu.Unlock()

	require.Equal(t, checkPaidResultAlreadyPaid, env.svc.checkPaid(ctx, order),
		"an unsigned paid report must still stop the order from being cancelled")
	after, err := env.client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, after.Status, "unsigned query must not mark the order paid")
	require.Nil(t, after.PaidAt, "unsigned query must not record a payment")
}
