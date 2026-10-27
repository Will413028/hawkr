package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/gateway/internal/realtime"
	"github.com/Will413028/hawkr/gateway/internal/sse"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type sseFrame struct {
	event string
	data  string
	id    string
}

// openCommentStream 連上留言 SSE，回傳讀取下一個非 ready／heartbeat frame 的函式。
func openCommentStream(t *testing.T, ts *httptest.Server, accountID, lastEventID string) (next func() sseFrame) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/public/posts/5/comments/stream", nil)
	if accountID != "" {
		req.Header.Set("X-Account-Id", accountID)
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}
	frames := make(chan sseFrame, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		var cur sseFrame
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				cur.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				cur.data = strings.TrimPrefix(line, "data: ")
			case strings.HasPrefix(line, "id: "):
				cur.id = strings.TrimPrefix(line, "id: ")
			case line == "":
				if cur.event != "" && cur.event != "ready" && cur.event != "heartbeat" {
					frames <- cur
				}
				cur = sseFrame{}
			}
		}
	}()
	return func() sseFrame {
		t.Helper()
		select {
		case f := <-frames:
			return f
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for an SSE frame")
			return sseFrame{}
		}
	}
}

func commentStreamServer(t *testing.T) (*httptest.Server, *Holder) {
	t.Helper()
	cfg := sse.DefaultConfig()
	cfg.BatchWindow = 40 * time.Millisecond
	mc := &mockSocialServiceClient{
		GetPostDetailFn: func(context.Context, *socialpb.GetPostDetailRequest, ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
			return &socialpb.PostDetailResponse{Data: &socialpb.PostDetailInfo{Id: "5"}}, nil
		},
	}
	h := newSSEStreamHolder(mc)
	h.BuyerID = buyerResolverForTest(socialTestBuyerID)
	h.Store = storeResolverForTest(socialTestStoreID)
	h.SSEHub = sse.NewHub(cfg, zap.NewNop())
	h.CommentChannel = realtime.NewCommentChannel(&config.Config{})
	ts := httptest.NewServer(setupSSEStreamRouter(h))
	t.Cleanup(ts.Close)
	return ts, h
}

func publishComment(t *testing.T, h *Holder, routingKey string, payload map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(payload)
	name, data, err := h.CommentChannel.BuildSSEEvent(routingKey, raw)
	if err != nil {
		t.Fatal(err)
	}
	h.SSEHub.Publish(realtime.TopicCommentPost(5), name, data, h.CommentChannel.BatchEventName())
}

func createdPayload(userID string, role int) map[string]any {
	return map[string]any{
		"postId":  5,
		"comment": map[string]any{"commentId": "1", "postId": "5", "userId": userID, "role": role, "content": "hi"},
	}
}

func commentOf(t *testing.T, f sseFrame) map[string]any {
	t.Helper()
	var out struct {
		Comment map[string]any `json:"comment"`
	}
	if err := json.Unmarshal([]byte(f.data), &out); err != nil {
		t.Fatalf("frame data %q: %v", f.data, err)
	}
	return out.Comment
}

// 每條連線依自己的 buyer_id、store_id 算 isMine；buyer 9999 與 store 9999 不互相冒充；公開 payload 沒有 userId。
func TestStreamPostComments_PersonalizesIsMinePerConnection(t *testing.T) {
	ts, h := commentStreamServer(t)
	viewer := openCommentStream(t, ts, "42", "")
	anon := openCommentStream(t, ts, "", "")

	cases := []struct {
		name     string
		userID   string
		role     int
		wantMine bool
	}{
		{"own buyer comment", "9999", 1, true},
		{"own store comment", "8001", 2, true},
		{"other buyer", "123", 1, false},
		{"store whose id equals my buyer id", "9999", 2, false},
		{"buyer whose id equals my store id", "8001", 1, false},
	}
	for _, tc := range cases {
		publishComment(t, h, pkgmq.RoutingKeyCommentCreated, createdPayload(tc.userID, tc.role))
		got := viewer()
		c := commentOf(t, got)
		if _, has := c["userId"]; has {
			t.Errorf("%s: viewer payload leaks userId: %s", tc.name, got.data)
		}
		if c["isMine"] != tc.wantMine {
			t.Errorf("%s: viewer isMine = %v, want %v", tc.name, c["isMine"], tc.wantMine)
		}
		ac := commentOf(t, anon())
		if _, has := ac["userId"]; has || ac["isMine"] != false {
			t.Errorf("%s: anonymous payload = %v, want isMine=false and no userId", tc.name, ac)
		}
	}

	publishComment(t, h, pkgmq.RoutingKeyCommentLikeChanged, map[string]any{"postId": 5, "commentId": 1, "likeCount": 3, "action": "LIKE", "userId": 8001, "likerKind": 2})
	like := viewer()
	var lc map[string]any
	_ = json.Unmarshal([]byte(like.data), &lc)
	if like.event != "comment.like_changed" || lc["likedByViewer"] != true || lc["isMine"] != nil || lc["userId"] != nil || lc["userKind"] != nil {
		t.Errorf("like_changed for my store = %s, want likedByViewer=true without userId/userKind/isMine", like.data)
	}
	_ = anon()
}

// batch 內每個 entry 與 Last-Event-ID replay 的副本都要個人化。
func TestStreamPostComments_PersonalizesBatchAndReplay(t *testing.T) {
	ts, h := commentStreamServer(t)
	viewer := openCommentStream(t, ts, "42", "")
	publishComment(t, h, pkgmq.RoutingKeyCommentCreated, createdPayload("9999", 1))
	publishComment(t, h, pkgmq.RoutingKeyCommentCreated, createdPayload("123", 1))
	batch := viewer()
	if batch.event != "comment.batch" {
		t.Fatalf("event = %q, want comment.batch (both events inside the batch window)", batch.event)
	}
	var b struct {
		Events []struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(batch.data), &b); err != nil || len(b.Events) != 2 {
		t.Fatalf("batch payload %q: %v", batch.data, err)
	}
	for i, want := range []bool{true, false} {
		c := commentOf(t, sseFrame{data: string(b.Events[i].Data)})
		if _, has := c["userId"]; has || c["isMine"] != want {
			t.Errorf("batch entry %d = %v, want isMine=%v and no userId", i, c, want)
		}
	}

	publishComment(t, h, pkgmq.RoutingKeyCommentCreated, createdPayload("9999", 1))
	later := viewer()
	replay := openCommentStream(t, ts, "", batch.id)
	rf := replay()
	if rf.id != later.id || strings.Contains(rf.data, "userId") || !strings.Contains(rf.data, `"isMine":false`) {
		t.Errorf("replayed frame for an anonymous viewer = %+v, want the event after %s with isMine=false and no userId", rf, batch.id)
	}
}

// 連線上限以 account_id 計，不是 buyer_id 或 store_id。
func TestStreamPostComments_ConnectionCapKeyIsAccountID(t *testing.T) {
	ts, h := commentStreamServer(t)
	_ = openCommentStream(t, ts, "42", "")
	deadline := time.Now().Add(2 * time.Second)
	for h.SSEHub.CapGroupSize(42, "") != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := h.SSEHub.CapGroupSize(42, ""); got != 1 {
		t.Errorf("cap group for account_id 42 = %d, want 1", got)
	}
	for _, id := range []int64{socialTestBuyerID, socialTestStoreID} {
		if got := h.SSEHub.CapGroupSize(id, ""); got != 0 {
			t.Errorf("cap group for business id %d = %d, want 0", id, got)
		}
	}
}
