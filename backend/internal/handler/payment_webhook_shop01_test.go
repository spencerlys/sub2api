//go:build unit

package handler

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

const shop01WebhookTestKey = "test-merchant-key-not-real"

func shop01WebhookSign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k != "sign" && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(sum[:])
}

func shop01WebhookBody(outTradeNo, amount, key string) string {
	params := map[string]string{
		"merchant_no": "m1001", "order_no": "G1", "out_trade_no": outTradeNo,
		"trade_no": "2026102422001400001", "amount": amount, "status": "TRADE_SUCCESS",
		"paid_at": "2026-10-24 12:00:00", "notify_time": "2026-10-24 12:00:01",
	}
	params["sign"] = shop01WebhookSign(params, key)
	v := url.Values{}
	for k, val := range params {
		v.Set(k, val)
	}
	return v.Encode()
}

type shop01WebhookEnv struct {
	client *dbent.Client
	router *gin.Engine
}

func newShop01WebhookEnv(t *testing.T) *shop01WebhookEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })

	svc := service.NewPaymentService(client, payment.NewRegistry(), payment.NewDefaultLoadBalancer(client, nil), nil, nil, nil, nil, nil, nil)
	h := NewPaymentWebhookHandler(svc, payment.NewRegistry())
	router := gin.New()
	router.GET("/api/v1/payment/webhook/shop01", h.Shop01Notify)
	router.POST("/api/v1/payment/webhook/shop01", h.Shop01Notify)
	return &shop01WebhookEnv{client: client, router: router}
}

func (e *shop01WebhookEnv) createInstance(t *testing.T, cfg map[string]string) string {
	t.Helper()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	inst, err := e.client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeShop01).SetName("shop01").SetConfig(string(raw)).
		SetSupportedTypes("alipay").SetEnabled(true).Save(context.Background())
	require.NoError(t, err)
	return strconv.FormatInt(inst.ID, 10)
}

func validShop01Config() map[string]string {
	return map[string]string{"merchantNo": "m1001", "merchantKey": shop01WebhookTestKey, "apiBase": "https://gw.invalid", "notifyUrl": "https://app.test/api/v1/payment/webhook/shop01"}
}

func (e *shop01WebhookEnv) createOrder(t *testing.T, instanceID string) *dbent.PaymentOrder {
	t.Helper()
	ctx := context.Background()
	user, err := e.client.User.Create().SetEmail("wh@example.com").SetPasswordHash("h").SetUsername("wh").Save(ctx)
	require.NoError(t, err)
	order, err := e.client.PaymentOrder.Create().
		SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(10.01).SetPayAmount(10.01).SetFeeRate(0).SetRechargeCode("PAY-WH").
		SetOutTradeNo("sub2_shop01_known").SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).SetStatus(service.OrderStatusPending).
		SetExpiresAt(time.Now().Add(30 * time.Minute)).SetClientIP("127.0.0.1").SetSrcHost("app.test").
		SetProviderInstanceID(instanceID).SetProviderKey(payment.TypeShop01).
		SetProviderSnapshot(map[string]any{"schema_version": 2, "provider_instance_id": instanceID, "provider_key": payment.TypeShop01, "merchant_id": "m1001"}).
		Save(ctx)
	require.NoError(t, err)
	return order
}

func (e *shop01WebhookEnv) post(body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/webhook/shop01", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	e.router.ServeHTTP(w, req)
	return w
}

func (e *shop01WebhookEnv) requireStillPending(t *testing.T, id int64) {
	t.Helper()
	o, err := e.client.PaymentOrder.Get(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, service.OrderStatusPending, o.Status)
}

func TestShop01WebhookUnknownOrderWithValidSignatureAcksSuccess(t *testing.T) {
	env := newShop01WebhookEnv(t)
	env.createInstance(t, validShop01Config())

	w := env.post(shop01WebhookBody("sub2_shop01_not_ours", "10.01", shop01WebhookTestKey))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "success", w.Body.String())
}

// Without a provider to verify the signature nothing may be acked: an
// unverified "success" would stop SHOP01's retries for a possibly real payment.
func TestShop01WebhookWithoutAnyInstanceIsNotAcked(t *testing.T) {
	env := newShop01WebhookEnv(t)

	w := env.post(shop01WebhookBody("sub2_shop01_not_ours", "10.01", shop01WebhookTestKey))
	require.NotEqual(t, http.StatusOK, w.Code)
	require.NotEqual(t, "success", w.Body.String())
}

func TestShop01WebhookKnownOrderWithMisconfiguredInstanceIsNotAcked(t *testing.T) {
	env := newShop01WebhookEnv(t)
	cfg := validShop01Config()
	delete(cfg, "merchantKey") // the instance can no longer build a provider
	instID := env.createInstance(t, cfg)
	order := env.createOrder(t, instID)

	w := env.post(shop01WebhookBody(order.OutTradeNo, "10.01", shop01WebhookTestKey))
	require.NotEqual(t, http.StatusOK, w.Code, "acking would stop gateway retries and lose a real payment")
	require.NotEqual(t, "success", w.Body.String())
	env.requireStillPending(t, order.ID)
}

func TestShop01WebhookBadSignatureRejected(t *testing.T) {
	env := newShop01WebhookEnv(t)
	instID := env.createInstance(t, validShop01Config())
	order := env.createOrder(t, instID)

	w := env.post(shop01WebhookBody(order.OutTradeNo, "10.01", "wrong-key"))
	require.Equal(t, http.StatusBadRequest, w.Code)
	env.requireStillPending(t, order.ID)
}

func TestShop01WebhookAmountMismatchIsNotAcked(t *testing.T) {
	env := newShop01WebhookEnv(t)
	instID := env.createInstance(t, validShop01Config())
	order := env.createOrder(t, instID)

	w := env.post(shop01WebhookBody(order.OutTradeNo, "10.02", shop01WebhookTestKey))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NotEqual(t, "success", w.Body.String())
	env.requireStillPending(t, order.ID)
}

func TestShop01WriteSuccessResponseAndExtractOutTradeNo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeSuccessResponse(c, payment.TypeShop01)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "success", w.Body.String())

	require.Equal(t, "sub2_x", extractOutTradeNo("merchant_no=m1&out_trade_no=sub2_x&amount=1.00", payment.TypeShop01))
}
