package httpapi

import (
	"bytes"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

func postJSON(t *testing.T, services []service, path, method, body string) int {
	t.Helper()
	for _, s := range services {
		if s.path != path {
			continue
		}
		mux := http.NewServeMux()
		mux.Handle(s.path, s.handler)
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path+method, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(rec, req)
		return rec.Code
	}
	t.Fatalf("沒有掛 %s", path)
	return 0
}

func TestImageAiServiceAcceptsA3MiBRequestAndEveryOtherServiceKeepsTwoMiB(t *testing.T) {
	services, err := connectServices(Deps{Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), Images: characterize.Images(t), Logger: slog.New(slog.DiscardHandler)},
		connect.WithReadMaxBytes(maxRequestBytes))
	if err != nil {
		t.Fatal(err)
	}
	body := func(imageBytes int) string {
		return `{"imageData":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("a"), imageBytes)) + `","mimeType":"image/jpeg","prompt":"p"}`
	}
	const imageAI = "/hawkr.imageai.v1alpha1.ImageAiAppService/"
	if got := postJSON(t, services, imageAI, "TransformImage", body(2<<20)); got == http.StatusTooManyRequests {
		t.Fatalf("2 MiB 原圖的 JSON 請求（約 2.7 MiB）被讀取上限擋下：%d", got)
	}
	if got := postJSON(t, services, imageAI, "TransformImage", body(3<<20)); got != http.StatusTooManyRequests {
		t.Fatalf("超過 3 MiB 的請求要被擋（resource_exhausted）：%d", got)
	}
	const buyerApp = "/hawkr.buyer.v1alpha1.BuyerAppService/"
	padded := func(n int) string { return `{"pad":"` + strings.Repeat("a", n) + `"}` }
	if got := postJSON(t, services, buyerApp, "ListBlockedStores", padded(1<<10)); got == http.StatusTooManyRequests {
		t.Fatalf("小請求被讀取上限擋下：%d", got)
	}
	if got := postJSON(t, services, buyerApp, "ListBlockedStores", padded(2<<20+(256<<10))); got != http.StatusTooManyRequests {
		t.Fatalf("2 到 3 MiB 的請求在其他 service 要被 2 MiB 上限擋下（resource_exhausted）：%d", got)
	}
	if imageAIMaxRequestBytes != 3<<20 || maxRequestBytes != 2<<20 {
		t.Fatalf("上限 = %d / %d", imageAIMaxRequestBytes, maxRequestBytes)
	}
}
