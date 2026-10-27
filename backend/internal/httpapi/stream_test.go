package httpapi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 串流的回應經過單體的中介層（tracing、快取規則、interceptor 之前的錯誤 log）時逐筆送出：假的 service
// 每送出一筆，就等測試確認收到了才送下一筆。任何一層把回應緩衝起來，測試就收不到 header 或第一筆。
// 驗的是中介層的性質，與哪個 service 無關；gateway 那一側是 TestConnectStreamIsNotBuffered。
func TestConnectStreamIsNotBuffered(t *testing.T) {
	const frames = 3
	received := make(chan struct{}, frames)
	stream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/connect+json")
		for i := 1; i <= frames; i++ {
			writeEnvelope(w, 0, fmt.Sprintf(`{"sequence":"%d"}`, i))
			w.(http.Flusher).Flush()
			select {
			case <-received:
			case <-r.Context().Done():
				return
			case <-time.After(5 * time.Second):
				return
			}
		}
		writeEnvelope(w, connectEndStream, `{}`)
	})
	const path = "/hawkr.test.v1alpha1.StreamPublicService/"
	h, err := assemble(Deps{Logger: slog.New(slog.DiscardHandler)}, []service{{path, stream}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	var body bytes.Buffer
	writeEnvelope(&body, 0, `{}`)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path+"Watch", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("Connect-Protocol-Version", "1")
	// header 也要在第一筆送出時就到：緩衝的中介層連 header 都等到回應結束才送。
	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 3 * time.Second
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		t.Fatalf("沒有收到 header：中介層把串流緩衝了（%v）", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	for i := 1; i <= frames; i++ {
		flags, payload := readEnvelope(t, resp.Body)
		if want := fmt.Sprintf(`{"sequence":"%d"}`, i); flags != 0 || payload != want {
			t.Fatalf("第 %d 筆 = %#x %s，預期 %s", i, flags, payload, want)
		}
		received <- struct{}{}
	}
	if flags, _ := readEnvelope(t, resp.Body); flags != connectEndStream {
		t.Fatalf("最後一筆的 flags = %#x，預期 end-stream", flags)
	}
}

// connectEndStream 是 Connect 串流最後一筆（end-stream）的 envelope flag。
const connectEndStream = 0x02

// writeEnvelope 寫出一筆 Connect 串流的 envelope：1 byte 的 flags、4 bytes 的長度，接著是內容。
func writeEnvelope(w io.Writer, flags byte, payload string) {
	head := [5]byte{flags}
	binary.BigEndian.PutUint32(head[1:], uint32(len(payload)))
	_, _ = w.Write(head[:])
	_, _ = io.WriteString(w, payload)
}

// readEnvelope 讀出一筆 envelope。
func readEnvelope(t *testing.T, r io.Reader) (byte, string) {
	t.Helper()
	var head [5]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		t.Fatalf("讀 envelope 的開頭：%v", err)
	}
	var payload strings.Builder
	if _, err := io.CopyN(&payload, r, int64(binary.BigEndian.Uint32(head[1:]))); err != nil {
		t.Fatalf("讀 envelope 的內容：%v", err)
	}
	return head[0], payload.String()
}
