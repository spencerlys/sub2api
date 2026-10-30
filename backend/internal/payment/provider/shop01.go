package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

// SHOP01 constants.
const (
	shop01CodeSuccess       = 1
	shop01HTTPTimeout       = 10 * time.Second
	maxShop01ResponseSize   = 1 << 20 // 1MB
	shop01DefaultSubject    = "充值"
	shop01StatusPending     = 0
	shop01StatusPaid        = 1
	shop01StatusExpired     = 2
	shop01NotifyTradeStatus = "TRADE_SUCCESS"
	// shop01QRFetchTimeout bounds the server-side fetch of the gateway's QR
	// image; the order never waits longer than this for the inline image.
	shop01QRFetchTimeout = 5 * time.Second
	// maxShop01QRImageSize caps the QR image (the real one is ~1 KB).
	maxShop01QRImageSize = 256 << 10
)

// shop01QRImageTypes are the image media types accepted for the inline QR.
var shop01QRImageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
}

// shop01NotifyAllowedParams is the exact parameter set of a SHOP01 notify.
// Any other key is rejected so no foreign field can be smuggled into the
// unescaped sign base string.
var shop01NotifyAllowedParams = map[string]bool{
	"merchant_no":  true,
	"order_no":     true,
	"out_trade_no": true,
	"trade_no":     true,
	"amount":       true,
	"status":       true,
	"paid_at":      true,
	"notify_time":  true,
	"sign":         true,
}

// Shop01 implements payment.Provider for the SHOP01 gateway, an Alipay
// personal-QR collector.
//
// SHOP01 matches an incoming Alipay payment to an order purely by its exact
// amount: the payer must pay exactly the submitted `money`, and two pending
// orders with the same amount on one merchant must never coexist (the service
// layer allocates distinct pay amounts per instance). Amounts are therefore
// handled in integer fen only.
//
// Notifications are form-encoded POSTs signed like EasyPay (sorted k=v pairs,
// empty values dropped, merchant key appended, MD5 lowercase hex). The gateway
// expects HTTP 200 with the plain body "success"; anything else makes it retry
// (up to 10 times over ~13 hours), so duplicate notifications are normal and
// fulfillment must stay idempotent.
//
// config keys: merchantNo, merchantKey (sensitive), apiBase, notifyUrl.
type Shop01 struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
	// qrFetchTimeout overrides shop01QRFetchTimeout (tests only).
	qrFetchTimeout time.Duration
}

// NewShop01 creates a new SHOP01 provider.
func NewShop01(instanceID string, config map[string]string) (*Shop01, error) {
	for _, k := range []string{"merchantNo", "merchantKey", "apiBase", "notifyUrl"} {
		if strings.TrimSpace(config[k]) == "" {
			return nil, fmt.Errorf("shop01 config missing required key: %s", k)
		}
	}
	cfg := make(map[string]string, len(config))
	for k, v := range config {
		cfg[k] = v
	}
	base := strings.TrimSpace(cfg["apiBase"])
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("shop01 config apiBase must be an http(s) URL with a host")
	}
	cfg["apiBase"] = strings.TrimRight(base, "/")
	cfg["merchantNo"] = strings.TrimSpace(cfg["merchantNo"])
	return &Shop01{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: shop01HTTPTimeout},
	}, nil
}

func (s *Shop01) Name() string        { return "SHOP01" }
func (s *Shop01) ProviderKey() string { return payment.TypeShop01 }
func (s *Shop01) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay}
}

// MerchantIdentityMetadata exposes the configured merchant number.
func (s *Shop01) MerchantIdentityMetadata() map[string]string {
	if s == nil || s.config["merchantNo"] == "" {
		return nil
	}
	return map[string]string{"merchant_no": s.config["merchantNo"]}
}

// shop01ParseFen strictly parses a CNY amount (at most 2 decimals) into fen and
// requires it to be positive.
func shop01ParseFen(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty amount")
	}
	fen, err := payment.YuanToFen(raw)
	if err != nil {
		return 0, err
	}
	if fen <= 0 {
		return 0, fmt.Errorf("amount must be positive: %s", raw)
	}
	return fen, nil
}

func shop01FormatFen(fen int64) string {
	return decimal.New(fen, -2).StringFixed(2)
}

// shop01Envelope is the common response shape.
type shop01Envelope struct {
	Code json.RawMessage `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// shop01Scalar decodes a JSON string or number into its textual form.
func shop01Scalar(raw json.RawMessage) string {
	raw = json.RawMessage(bytes.TrimSpace(raw))
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return strings.TrimSpace(str)
	}
	var num json.Number
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&num); err == nil {
		return num.String()
	}
	return ""
}

func shop01CodeIsSuccess(raw json.RawMessage) bool {
	n, err := strconv.Atoi(shop01Scalar(raw))
	return err == nil && n == shop01CodeSuccess
}

func (s *Shop01) call(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	params["sign"] = easyPaySign(params, s.config["merchantKey"])
	body, status, err := s.postForm(ctx, s.config["apiBase"]+path, params)
	if err != nil {
		return nil, err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %d: %s", status, summarizeEasyPayResponse(body))
	}
	var env shop01Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("malformed response (HTTP %d): %s", status, summarizeEasyPayResponse(body))
	}
	if !shop01CodeIsSuccess(env.Code) {
		msg := strings.TrimSpace(env.Msg)
		if msg == "" {
			msg = summarizeEasyPayResponse(body)
		}
		return nil, errors.New(msg)
	}
	if len(bytes.TrimSpace(env.Data)) == 0 || string(bytes.TrimSpace(env.Data)) == "null" {
		return nil, errors.New("response missing data")
	}
	return env.Data, nil
}

func (s *Shop01) postForm(ctx context.Context, endpoint string, params map[string]string) ([]byte, int, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: shop01HTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxShop01ResponseSize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// CreatePayment submits the order to SHOP01. The returned QR image goes to
// QRCodeImg (an image to display), never QRCode (content to encode). The
// gateway serves the image over plain http, which an https site cannot embed
// (mixed content + CSP img-src), so the image is fetched server-side and
// inlined as a data: URI; on any failure the original URL is kept.
func (s *Shop01) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	outTradeNo := strings.TrimSpace(req.OrderID)
	if outTradeNo == "" {
		return nil, errors.New("shop01 submit: missing out_trade_no")
	}
	fen, err := shop01ParseFen(req.Amount)
	if err != nil {
		return nil, fmt.Errorf("shop01 submit: invalid amount: %w", err)
	}
	money := shop01FormatFen(fen)
	notifyURL := strings.TrimSpace(req.NotifyURL)
	if notifyURL == "" {
		notifyURL = strings.TrimSpace(s.config["notifyUrl"])
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = shop01DefaultSubject
	}
	params := map[string]string{
		"merchant_no":  s.config["merchantNo"],
		"out_trade_no": outTradeNo,
		"money":        money,
		"subject":      subject,
		"notify_url":   notifyURL,
	}
	data, err := s.call(ctx, "/api/submit", params)
	if err != nil {
		return nil, fmt.Errorf("shop01 submit failed: %w", err)
	}
	var d struct {
		OrderNo   json.RawMessage `json:"order_no"`
		PayURL    string          `json:"pay_url"`
		QRCodeURL string          `json:"qrcode_url"`
		Amount    json.RawMessage `json:"amount"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("shop01 submit: malformed data: %w", err)
	}
	if strings.TrimSpace(d.PayURL) == "" {
		return nil, errors.New("shop01 submit: response missing pay_url")
	}
	if amt := shop01Scalar(d.Amount); amt != "" {
		gotFen, err := shop01ParseFen(amt)
		if err != nil || gotFen != fen {
			return nil, fmt.Errorf("shop01 submit amount mismatch: requested %s, gateway returned %s", money, amt)
		}
	}
	tradeNo := shop01Scalar(d.OrderNo)
	qrImg := strings.TrimSpace(d.QRCodeURL)
	if qrImg != "" {
		if inline, ferr := s.fetchQRCodeDataURI(ctx, qrImg); ferr != nil {
			slog.Warn("shop01 qr image inline failed, falling back to gateway url",
				"instance_id", s.instanceID, "out_trade_no", outTradeNo, "order_no", tradeNo, "error", ferr)
		} else {
			qrImg = inline
		}
	}
	return &payment.CreatePaymentResponse{
		TradeNo:   tradeNo,
		PayURL:    strings.TrimSpace(d.PayURL),
		QRCodeImg: qrImg,
	}, nil
}

// fetchQRCodeDataURI downloads the gateway QR image and returns it as a
// base64 data: URI. Only URLs on the configured gateway host are fetched, no
// redirects are followed, the body is capped at maxShop01QRImageSize and both
// the declared and the sniffed content type must be an accepted image type.
func (s *Shop01) fetchQRCodeDataURI(ctx context.Context, rawURL string) (string, error) {
	target, err := url.Parse(rawURL)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return "", errors.New("qrcode_url is not an absolute http(s) URL")
	}
	base, err := url.Parse(s.config["apiBase"])
	if err != nil || !strings.EqualFold(target.Host, base.Host) {
		return "", fmt.Errorf("qrcode_url host %q does not match gateway host", target.Host)
	}
	timeout := s.qrFetchTimeout
	if timeout <= 0 {
		timeout = shop01QRFetchTimeout
	}
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if s.httpClient != nil {
		client.Transport = s.httpClient.Transport
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch qrcode: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch qrcode: HTTP %d", resp.StatusCode)
	}
	declared, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !shop01QRImageTypes[strings.ToLower(declared)] {
		return "", fmt.Errorf("fetch qrcode: unexpected content type %q", resp.Header.Get("Content-Type"))
	}
	if resp.ContentLength > maxShop01QRImageSize {
		return "", fmt.Errorf("fetch qrcode: image too large (%d bytes)", resp.ContentLength)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxShop01QRImageSize+1))
	if err != nil {
		return "", fmt.Errorf("fetch qrcode: %w", err)
	}
	if len(body) > maxShop01QRImageSize {
		return "", errors.New("fetch qrcode: image too large")
	}
	if len(body) == 0 {
		return "", errors.New("fetch qrcode: empty body")
	}
	sniffed := http.DetectContentType(body)
	if !shop01QRImageTypes[sniffed] {
		return "", fmt.Errorf("fetch qrcode: body is not an accepted image (%s)", sniffed)
	}
	return "data:" + sniffed + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

// QueryOrder queries SHOP01 by our out_trade_no.
func (s *Shop01) QueryOrder(ctx context.Context, outTradeNo string) (*payment.QueryOrderResponse, error) {
	outTradeNo = strings.TrimSpace(outTradeNo)
	if outTradeNo == "" {
		return nil, errors.New("shop01 query: missing out_trade_no")
	}
	params := map[string]string{
		"merchant_no":  s.config["merchantNo"],
		"out_trade_no": outTradeNo,
	}
	data, err := s.call(ctx, "/api/query", params)
	if err != nil {
		return nil, fmt.Errorf("shop01 query failed: %w", err)
	}
	var d struct {
		OrderNo    json.RawMessage `json:"order_no"`
		OutTradeNo json.RawMessage `json:"out_trade_no"`
		Amount     json.RawMessage `json:"amount"`
		Status     json.RawMessage `json:"status"`
		TradeNo    json.RawMessage `json:"trade_no"`
		PaidAt     json.RawMessage `json:"paid_at"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("shop01 query: malformed data: %w", err)
	}
	if got := shop01Scalar(d.OutTradeNo); got != "" && got != outTradeNo {
		return nil, fmt.Errorf("shop01 query: out_trade_no mismatch: requested %s, got %s", outTradeNo, got)
	}
	rawStatus := shop01Scalar(d.Status)
	statusCode, err := strconv.Atoi(rawStatus)
	if err != nil {
		return nil, fmt.Errorf("shop01 query: unknown status %q", rawStatus)
	}
	var status string
	switch statusCode {
	case shop01StatusPending:
		status = payment.ProviderStatusPending
	case shop01StatusPaid:
		status = payment.ProviderStatusPaid
	case shop01StatusExpired:
		status = payment.ProviderStatusFailed
	default:
		return nil, fmt.Errorf("shop01 query: unknown status %q", rawStatus)
	}
	orderNo := shop01Scalar(d.OrderNo)
	metadata := map[string]string{"merchant_no": s.config["merchantNo"], "order_no": orderNo}
	var amount float64
	rawAmount := shop01Scalar(d.Amount)
	if fen, perr := shop01ParseFen(rawAmount); perr == nil {
		amount = payment.FenToYuan(fen)
		metadata[payment.MetadataKeyExactAmount] = shop01FormatFen(fen)
	} else if status == payment.ProviderStatusPaid {
		return nil, fmt.Errorf("shop01 query: paid order has invalid amount %q: %w", rawAmount, perr)
	}
	tradeNo := shop01Scalar(d.TradeNo)
	if tradeNo == "" {
		tradeNo = orderNo
	}
	return &payment.QueryOrderResponse{
		TradeNo:  tradeNo,
		Status:   status,
		Amount:   amount,
		PaidAt:   shop01Scalar(d.PaidAt),
		Metadata: metadata,
	}, nil
}

// VerifyNotification verifies a SHOP01 notify body. Checks run in order:
// allowlist → signature → merchant → status → amount, so no field is trusted
// before the signature is verified.
func (s *Shop01) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	values, err := url.ParseQuery(rawBody)
	if err != nil {
		return nil, fmt.Errorf("parse notify: %w", err)
	}
	params := make(map[string]string, len(values))
	for k, vs := range values {
		if !shop01NotifyAllowedParams[k] {
			return nil, fmt.Errorf("unexpected notify param: %s", k)
		}
		if len(vs) != 1 {
			return nil, fmt.Errorf("duplicate notify param: %s", k)
		}
		params[k] = vs[0]
	}
	sign := strings.ToLower(strings.TrimSpace(params["sign"]))
	if sign == "" {
		return nil, errors.New("missing sign")
	}
	if !easyPayVerifySign(params, s.config["merchantKey"], sign) {
		return nil, errors.New("invalid signature")
	}
	merchantNo := strings.TrimSpace(params["merchant_no"])
	if merchantNo != s.config["merchantNo"] {
		return nil, fmt.Errorf("merchant_no mismatch: got %q", merchantNo)
	}
	if params["status"] != shop01NotifyTradeStatus {
		return nil, fmt.Errorf("unexpected notify status: %q", params["status"])
	}
	outTradeNo := strings.TrimSpace(params["out_trade_no"])
	if outTradeNo == "" {
		return nil, errors.New("missing out_trade_no")
	}
	fen, err := shop01ParseFen(params["amount"])
	if err != nil {
		return nil, fmt.Errorf("invalid notify amount: %w", err)
	}
	orderNo := strings.TrimSpace(params["order_no"])
	tradeNo := strings.TrimSpace(params["trade_no"])
	if tradeNo == "" {
		tradeNo = orderNo
	}
	return &payment.PaymentNotification{
		TradeNo: tradeNo,
		OrderID: outTradeNo,
		Amount:  payment.FenToYuan(fen),
		Status:  payment.ProviderStatusSuccess,
		RawData: rawBody,
		Metadata: map[string]string{
			"merchant_no":                  merchantNo,
			payment.MetadataKeyExactAmount: shop01FormatFen(fen),
			"order_no":                     orderNo,
		},
	}, nil
}

// Refund is not supported by SHOP01.
func (s *Shop01) Refund(_ context.Context, _ payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, errors.New("shop01 does not support refunds: refund the payer manually in the SHOP01/Alipay console")
}
