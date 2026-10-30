//go:build unit

package provider

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

const (
	shop01TestKey      = "testkey"
	shop01TestMerchant = "m1001"
	// md5("merchant_no=m1001&money=10.01&out_trade_no=ORD001&subject=会员月卡testkey"),
	// computed independently with python3 hashlib.
	shop01DocSignVector = "119ced1c36286524de8e5cae8f8f2d2e"
)

func newShop01ForTest(t *testing.T, apiBase string) *Shop01 {
	t.Helper()
	p, err := NewShop01("7", map[string]string{
		"merchantNo":  shop01TestMerchant,
		"merchantKey": shop01TestKey,
		"apiBase":     apiBase + "/",
		"notifyUrl":   "https://pay.example.com/api/v1/payment/webhook/shop01",
	})
	require.NoError(t, err)
	return p
}

func TestShop01SignDocVector(t *testing.T) {
	params := map[string]string{
		"merchant_no":  "m1001",
		"money":        "10.01",
		"out_trade_no": "ORD001",
		"subject":      "会员月卡",
	}
	sum := md5.Sum([]byte("merchant_no=m1001&money=10.01&out_trade_no=ORD001&subject=会员月卡testkey"))
	require.Equal(t, shop01DocSignVector, hex.EncodeToString(sum[:]))
	require.Equal(t, shop01DocSignVector, easyPaySign(params, "testkey"))

	// Empty values are excluded from the sign base.
	withEmpty := cloneStringMap(params)
	withEmpty["notify_url"] = ""
	withEmpty["sign"] = "ignored"
	require.Equal(t, shop01DocSignVector, easyPaySign(withEmpty, "testkey"))

	// notify_url is part of the signature when present.
	withNotify := cloneStringMap(params)
	withNotify["notify_url"] = "https://a.example.com/n"
	signA := easyPaySign(withNotify, "testkey")
	require.NotEqual(t, shop01DocSignVector, signA)
	withNotify["notify_url"] = "https://b.example.com/n"
	require.NotEqual(t, signA, easyPaySign(withNotify, "testkey"))
}

func TestNewShop01Validation(t *testing.T) {
	base := map[string]string{"merchantNo": "m", "merchantKey": "k", "apiBase": "https://gw.example.com", "notifyUrl": "https://n.example.com"}
	for _, k := range []string{"merchantNo", "merchantKey", "apiBase", "notifyUrl"} {
		cfg := cloneStringMap(base)
		cfg[k] = "  "
		_, err := NewShop01("1", cfg)
		require.ErrorContains(t, err, "shop01 config missing required key: "+k)
	}
	for _, bad := range []string{"gw.example.com", "ftp://gw.example.com", "https://", "::"} {
		cfg := cloneStringMap(base)
		cfg["apiBase"] = bad
		_, err := NewShop01("1", cfg)
		require.Error(t, err, bad)
	}
	p, err := NewShop01("1", base)
	require.NoError(t, err)
	require.Equal(t, payment.TypeShop01, p.ProviderKey())
	require.Equal(t, "SHOP01", p.Name())
	require.Equal(t, []payment.PaymentType{payment.TypeAlipay}, p.SupportedTypes())
	require.Equal(t, map[string]string{"merchant_no": "m"}, p.MerchantIdentityMetadata())
}

// shop01FormHandler parses the form, verifies the signature and hands the
// params to respond.
func shop01FormHandler(t *testing.T, path string, respond func(w http.ResponseWriter, form map[string]string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		form := map[string]string{}
		for k := range r.PostForm {
			form[k] = r.PostForm.Get(k)
		}
		if !easyPayVerifySign(form, shop01TestKey, form["sign"]) {
			_, _ = io.WriteString(w, `{"code":0,"msg":"签名错误"}`)
			return
		}
		respond(w, form)
	}
}

func TestShop01CreatePaymentSuccess(t *testing.T) {
	var seen map[string]string
	srv := httptest.NewServer(shop01FormHandler(t, "/api/submit", func(w http.ResponseWriter, form map[string]string) {
		seen = form
		_, _ = io.WriteString(w, `{"code":1,"msg":"下单成功","data":{"order_no":"S0001","pay_url":"https://gw.example.com/pay/S0001","qrcode_url":"https://gw.example.com/qr/S0001.png","amount":"10.01","expire_minutes":30}}`)
	}))
	defer srv.Close()
	p := newShop01ForTest(t, srv.URL)

	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "ORD001", Amount: "10.01", Subject: "会员月卡"})
	require.NoError(t, err)
	require.Equal(t, "S0001", resp.TradeNo)
	require.Equal(t, "https://gw.example.com/pay/S0001", resp.PayURL)
	require.Equal(t, "https://gw.example.com/qr/S0001.png", resp.QRCodeImg)
	require.Empty(t, resp.QRCode)

	require.Equal(t, "10.01", seen["money"])
	require.Equal(t, shop01TestMerchant, seen["merchant_no"])
	require.Equal(t, "ORD001", seen["out_trade_no"])
	require.Equal(t, "会员月卡", seen["subject"])
	require.Equal(t, "https://pay.example.com/api/v1/payment/webhook/shop01", seen["notify_url"])
	// The signature covers notify_url: dropping it breaks verification.
	withoutNotify := cloneStringMap(seen)
	delete(withoutNotify, "notify_url")
	require.False(t, easyPayVerifySign(withoutNotify, shop01TestKey, seen["sign"]))
}

func TestShop01CreatePaymentNotifyOverrideAndNumericAmount(t *testing.T) {
	var seen map[string]string
	srv := httptest.NewServer(shop01FormHandler(t, "/api/submit", func(w http.ResponseWriter, form map[string]string) {
		seen = form
		_, _ = io.WriteString(w, `{"code":"1","msg":"ok","data":{"order_no":12,"pay_url":"https://gw/p","qrcode_url":"https://gw/q.png","amount":10.1}}`)
	}))
	defer srv.Close()
	p := newShop01ForTest(t, srv.URL)
	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "O2", Amount: "10.1", NotifyURL: "https://override.example.com/n"})
	require.NoError(t, err)
	require.Equal(t, "12", resp.TradeNo)
	require.Equal(t, "https://override.example.com/n", seen["notify_url"])
	require.Equal(t, "10.10", seen["money"])
	require.Equal(t, "充值", seen["subject"])
}

func TestShop01CreatePaymentErrors(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"bad sign", 200, `{"code":0,"msg":"签名错误"}`, "签名错误"},
		{"ip whitelist", 200, `{"code":0,"msg":"IP 1.2.3.4 不在白名单内"}`, "IP 1.2.3.4 不在白名单内"},
		{"malformed", 200, `<html>oops</html>`, "malformed response"},
		{"http 500", 500, `{"code":1}`, "HTTP 500"},
		{"amount mismatch", 200, `{"code":1,"data":{"order_no":"S1","pay_url":"https://gw/p","amount":"10.02"}}`, "amount mismatch"},
		{"missing pay_url", 200, `{"code":1,"data":{"order_no":"S1","amount":"10.01"}}`, "missing pay_url"},
		{"missing data", 200, `{"code":1,"msg":"ok"}`, "missing data"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			p := newShop01ForTest(t, srv.URL)
			_, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "ORD001", Amount: "10.01"})
			require.ErrorContains(t, err, "shop01 submit")
			require.ErrorContains(t, err, tc.wantErr)
		})
	}

	p := newShop01ForTest(t, "http://127.0.0.1:1")
	for _, amt := range []string{"", "0", "-1", "10.011", "abc"} {
		_, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "O", Amount: amt})
		require.ErrorContains(t, err, "invalid amount", amt)
	}
	_, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: " ", Amount: "1.00"})
	require.ErrorContains(t, err, "missing out_trade_no")
}

func TestShop01QueryOrder(t *testing.T) {
	var body string
	srv := httptest.NewServer(shop01FormHandler(t, "/api/query", func(w http.ResponseWriter, form map[string]string) {
		require.Equal(t, shop01TestMerchant, form["merchant_no"])
		require.Equal(t, "ORD001", form["out_trade_no"])
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	p := newShop01ForTest(t, srv.URL)
	ctx := context.Background()

	body = `{"code":1,"data":{"order_no":"S1","out_trade_no":"ORD001","amount":"10.01","status":1,"trade_no":"2026T1","paid_at":"2026-10-24 10:00:00","notify_status":1}}`
	resp, err := p.QueryOrder(ctx, "ORD001")
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusPaid, resp.Status)
	require.Equal(t, 10.01, resp.Amount)
	require.Equal(t, "2026T1", resp.TradeNo)
	require.Equal(t, "2026-10-24 10:00:00", resp.PaidAt)
	require.Equal(t, "10.01", resp.Metadata[payment.MetadataKeyExactAmount])
	require.Equal(t, shop01TestMerchant, resp.Metadata["merchant_no"])
	require.Equal(t, "S1", resp.Metadata["order_no"])

	body = `{"code":1,"data":{"order_no":"S1","out_trade_no":"ORD001","amount":"10.01","status":"0","trade_no":""}}`
	resp, err = p.QueryOrder(ctx, "ORD001")
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusPending, resp.Status)
	require.Equal(t, "S1", resp.TradeNo) // trade_no fallback to order_no

	body = `{"code":1,"data":{"order_no":"S1","out_trade_no":"ORD001","amount":"10.01","status":2}}`
	resp, err = p.QueryOrder(ctx, "ORD001")
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusFailed, resp.Status)

	errCases := map[string]string{
		`{"code":0,"msg":"订单不存在"}`: "订单不存在",
		`not json`:                 "malformed response",
		`{"code":1,"data":{"out_trade_no":"OTHER","amount":"10.01","status":1}}`:  "out_trade_no mismatch",
		`{"code":1,"data":{"out_trade_no":"ORD001","amount":"10.01","status":7}}`: "unknown status",
		`{"code":1,"data":{"out_trade_no":"ORD001","amount":"10.01"}}`:            "unknown status",
		`{"code":1,"data":{"out_trade_no":"ORD001","amount":"x","status":1}}`:     "invalid amount",
	}
	for b, want := range errCases {
		body = b
		_, err := p.QueryOrder(ctx, "ORD001")
		require.ErrorContains(t, err, "shop01 query", b)
		require.ErrorContains(t, err, want, b)
	}
}

func shop01NotifyValues() url.Values {
	return url.Values{
		"merchant_no":  {shop01TestMerchant},
		"order_no":     {"S0001"},
		"out_trade_no": {"ORD001"},
		"trade_no":     {"2026102422001"},
		"amount":       {"10.01"},
		"status":       {"TRADE_SUCCESS"},
		"paid_at":      {"2026-10-24 10:00:00"},
		"notify_time":  {"2026-10-24 10:00:01"},
	}
}

func shop01SignValues(v url.Values) url.Values {
	params := map[string]string{}
	for k := range v {
		params[k] = v.Get(k)
	}
	v.Set("sign", easyPaySign(params, shop01TestKey))
	return v
}

func TestShop01VerifyNotificationValid(t *testing.T) {
	p := newShop01ForTest(t, "https://gw.example.com")
	body := shop01SignValues(shop01NotifyValues()).Encode()
	n, err := p.VerifyNotification(context.Background(), body, nil)
	require.NoError(t, err)
	require.Equal(t, "ORD001", n.OrderID)
	require.Equal(t, "2026102422001", n.TradeNo)
	require.Equal(t, 10.01, n.Amount)
	require.Equal(t, payment.ProviderStatusSuccess, n.Status)
	require.Equal(t, body, n.RawData)
	require.Equal(t, "10.01", n.Metadata[payment.MetadataKeyExactAmount])
	require.Equal(t, shop01TestMerchant, n.Metadata["merchant_no"])
	require.Equal(t, "S0001", n.Metadata["order_no"])

	upper := shop01SignValues(shop01NotifyValues())
	upper.Set("sign", strings.ToUpper(upper.Get("sign")))
	_, err = p.VerifyNotification(context.Background(), upper.Encode(), nil)
	require.NoError(t, err)

	noTrade := shop01NotifyValues()
	noTrade.Del("trade_no")
	n, err = p.VerifyNotification(context.Background(), shop01SignValues(noTrade).Encode(), nil)
	require.NoError(t, err)
	require.Equal(t, "S0001", n.TradeNo)
}

func TestShop01VerifyNotificationRejects(t *testing.T) {
	p := newShop01ForTest(t, "https://gw.example.com")
	cases := map[string]func() string{
		"tampered amount": func() string {
			v := shop01SignValues(shop01NotifyValues())
			v.Set("amount", "0.01")
			return v.Encode()
		},
		"tampered sign": func() string {
			v := shop01SignValues(shop01NotifyValues())
			v.Set("sign", strings.Repeat("0", 32))
			return v.Encode()
		},
		"wrong merchant": func() string {
			v := shop01NotifyValues()
			v.Set("merchant_no", "m9999")
			return shop01SignValues(v).Encode()
		},
		"extra param sign_type": func() string {
			v := shop01NotifyValues()
			v.Set("sign_type", "MD5")
			return shop01SignValues(v).Encode()
		},
		"extra param foo": func() string {
			v := shop01NotifyValues()
			v.Set("foo", "bar")
			return shop01SignValues(v).Encode()
		},
		"status not success": func() string {
			v := shop01NotifyValues()
			v.Set("status", "WAIT_BUYER_PAY")
			return shop01SignValues(v).Encode()
		},
		"duplicate key": func() string {
			v := shop01SignValues(shop01NotifyValues())
			v.Add("amount", "10.01")
			return v.Encode()
		},
		"three decimals": func() string {
			v := shop01NotifyValues()
			v.Set("amount", "10.011")
			return shop01SignValues(v).Encode()
		},
		"zero amount": func() string {
			v := shop01NotifyValues()
			v.Set("amount", "0.00")
			return shop01SignValues(v).Encode()
		},
		"missing out_trade_no": func() string {
			v := shop01NotifyValues()
			v.Del("out_trade_no")
			return shop01SignValues(v).Encode()
		},
		"missing sign": func() string {
			return shop01NotifyValues().Encode()
		},
		"wrong key": func() string {
			v := shop01NotifyValues()
			params := map[string]string{}
			for k := range v {
				params[k] = v.Get(k)
			}
			v.Set("sign", easyPaySign(params, "otherkey"))
			return v.Encode()
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			n, err := p.VerifyNotification(context.Background(), build(), nil)
			require.Error(t, err)
			require.Nil(t, n)
		})
	}
}

func TestShop01RefundUnsupported(t *testing.T) {
	p := newShop01ForTest(t, "https://gw.example.com")
	resp, err := p.Refund(context.Background(), payment.RefundRequest{OrderID: "ORD001", Amount: "10.01"})
	require.Nil(t, resp)
	require.ErrorContains(t, err, "shop01 does not support refunds")
}
