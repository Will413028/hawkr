package sse

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestServeSSE_EvictedConnection_EndToEndOverRealHTTP 是跨 Hub 與 ServeSSE 接縫
// 的整合驗證：真的起一個 HTTP server、用真的 TCP 連線訂閱、而且**開了連線之後
// 完全不讀 body**——那正是殭屍連線在 server 眼中的樣子（沒有任何存活訊號、寫入
// 也不會失敗）。
//
// 單元測試證明了 Hub 會把位子收回來；這裡要證明「收回來」在真實連線上是完整
// 生效的：被回收那條的 ServeSSE 迴圈必須真的從 conn.Closed() 退出、handler 的
// defer Unsubscribe 必須是安全的 no-op、位子必須真的可以被下一條用掉。
//
// 註：完整的殭屍情境（瀏覽器已離開、但中間層繼續代收 heartbeat）需要一個中介
// 層才能重現，本地端沒有；這裡重現的是它在 origin 端造成的等價狀態——一條沒人
// 在讀、也永遠不會自己結束的連線持續佔著位子。
func TestServeSSE_EvictedConnection_EndToEndOverRealHTTP(t *testing.T) {
	const uid = int64(2685914682)
	cfg := DefaultConfig()
	cfg.MaxConnPerUser = 2
	cfg.HeartbeatInterval = time.Hour // 本測試不要心跳干擾
	hub := NewHub(cfg, nil)

	var wg sync.WaitGroup
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// 與 handler/sse_stream.go 的 serveSSE 同流程。
		conn := NewConnection("notification:user:1", w, flusher, "")
		conn.AccountID = uid

		replay, miss, err := hub.Subscribe(conn)
		if err != nil {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		wg.Add(1)
		defer wg.Done()
		defer hub.Unsubscribe(conn)
		ServeSSE(r.Context(), hub, conn, w, replay, miss, ServeOptions{}, nil)
	}))
	defer func() {
		srv.CloseClientConnections()
		wg.Wait()
		srv.Close()
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	open := func(t *testing.T) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		return resp
	}

	// 兩條「開了就不讀」的連線把 cap 佔滿。
	first := open(t)
	waitForSeats(t, hub, uid, 1)
	second := open(t)
	waitForSeats(t, hub, uid, 2)

	if first.StatusCode != http.StatusOK || second.StatusCode != http.StatusOK {
		t.Fatalf("前置兩條應為 200，got %d / %d", first.StatusCode, second.StatusCode)
	}

	// 第三條：滿座 → 必須回收最舊的位子，而不是回 429。
	third := open(t)
	defer third.Body.Close()
	if third.StatusCode != http.StatusOK {
		t.Fatalf("第三條應該靠回收最舊位子入座（修復前這裡是 429），got %d", third.StatusCode)
	}

	// 被回收那條的 server 端迴圈必須真的結束 → client 讀到 EOF。
	// 若 ServeSSE 沒有從 conn.Closed() 退出，這裡會卡到 client timeout。
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, first.Body)
		done <- err
	}()
	select {
	case <-done:
		// 讀完（EOF 或連線關閉）都代表 server 端確實收掉了這條。
	case <-time.After(3 * time.Second):
		t.Fatal("被回收的連線在 server 端沒有真的結束——ServeSSE 未從 conn.Closed() 退出")
	}
	first.Body.Close()

	// 位子數必須仍等於 cap（回收不放寬硬上限），且第二條沒被波及。
	waitForSeats(t, hub, uid, cfg.MaxConnPerUser)
	if got := hub.ActiveConnections(); got != int64(cfg.MaxConnPerUser) {
		t.Errorf("ActiveConnections = %d，want %d", got, cfg.MaxConnPerUser)
	}
	second.Body.Close()
}

// waitForSeats 等佔位數收斂到期望值。
// 需要等待是因為 handler 是在另一個 goroutine 裡跑 Subscribe，client 拿到
// response header 與 server 端完成佔位之間沒有同步點。
func waitForSeats(t *testing.T, hub *Hub, uid int64, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.CapGroupSize(uid, "") == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("佔位數未收斂到 %d，實際 %d", want, hub.CapGroupSize(uid, ""))
}
