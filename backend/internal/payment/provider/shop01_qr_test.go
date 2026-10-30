//go:build unit

package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func shop01TestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.White)
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// newShop01QRServer serves /api/submit (returning a qrcode_url built by
// qrURL from the server's own base URL) and GET /qrcode/S0001.png via qr.
func newShop01QRServer(t *testing.T, qrURL func(base string) string, qr http.HandlerFunc) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.Handle("/api/submit", shop01FormHandler(t, "/api/submit", func(w http.ResponseWriter, _ map[string]string) {
		_, _ = io.WriteString(w, `{"code":1,"msg":"ok","data":{"order_no":"S0001","pay_url":"`+srv.URL+`/p/S0001","qrcode_url":"`+qrURL(srv.URL)+`","amount":"10.01"}}`)
	}))
	mux.HandleFunc("/qrcode/S0001.png", qr)
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func shop01SameHostQR(base string) string { return base + "/qrcode/S0001.png" }

func TestShop01CreatePaymentInlinesQRCode(t *testing.T) {
	pngBytes := shop01TestPNG(t)
	var qrHits int
	srv := newShop01QRServer(t, shop01SameHostQR, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		qrHits++
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	})
	p := newShop01ForTest(t, srv.URL)

	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "ORD001", Amount: "10.01"})
	require.NoError(t, err)
	require.Equal(t, 1, qrHits)
	require.Equal(t, "S0001", resp.TradeNo)
	require.Equal(t, srv.URL+"/p/S0001", resp.PayURL)
	require.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(pngBytes), resp.QRCodeImg)
	require.Empty(t, resp.QRCode)
}

func TestShop01CreatePaymentQRCodeFallback(t *testing.T) {
	pngBytes := shop01TestPNG(t)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("foreign host must never be fetched")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer other.Close()

	cases := []struct {
		name  string
		qrURL func(base string) string
		qr    http.HandlerFunc
	}{
		{"non-image content type", shop01SameHostQR, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, "<html>cashier</html>")
		}},
		{"image header but html body", shop01SameHostQR, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = io.WriteString(w, "<html><script>x</script></html>")
		}},
		{"oversize declared", shop01SameHostQR, func(w http.ResponseWriter, _ *http.Request) {
			body := append(append([]byte{}, pngBytes...), make([]byte, maxShop01QRImageSize)...)
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write(body)
		}},
		{"oversize chunked", shop01SameHostQR, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
			w.(http.Flusher).Flush()
			_, _ = w.Write(make([]byte, maxShop01QRImageSize))
		}},
		{"http 404", shop01SameHostQR, func(w http.ResponseWriter, _ *http.Request) {
			http.NotFound(w, nil)
		}},
		{"redirect not followed", shop01SameHostQR, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+"/x.png", http.StatusFound)
		}},
		{"wrong host", func(string) string { return other.URL + "/qrcode/S0001.png" }, func(w http.ResponseWriter, _ *http.Request) {
			t.Errorf("unexpected fetch")
		}},
		{"timeout", shop01SameHostQR, func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newShop01QRServer(t, tc.qrURL, tc.qr)
			p := newShop01ForTest(t, srv.URL)
			p.qrFetchTimeout = 200 * time.Millisecond
			start := time.Now()
			resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "ORD001", Amount: "10.01"})
			require.NoError(t, err, "order must still succeed")
			require.Less(t, time.Since(start), 1500*time.Millisecond)
			require.Equal(t, "S0001", resp.TradeNo)
			require.Equal(t, srv.URL+"/p/S0001", resp.PayURL)
			require.Equal(t, tc.qrURL(srv.URL), resp.QRCodeImg)
			require.False(t, strings.HasPrefix(resp.QRCodeImg, "data:"))
		})
	}
}

func TestShop01CreatePaymentNoQRCodeURL(t *testing.T) {
	srv := httptest.NewServer(shop01FormHandler(t, "/api/submit", func(w http.ResponseWriter, _ map[string]string) {
		_, _ = io.WriteString(w, `{"code":1,"data":{"order_no":"S1","pay_url":"https://gw/p","amount":"10.01"}}`)
	}))
	defer srv.Close()
	p := newShop01ForTest(t, srv.URL)
	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "ORD001", Amount: "10.01"})
	require.NoError(t, err)
	require.Empty(t, resp.QRCodeImg)
}
