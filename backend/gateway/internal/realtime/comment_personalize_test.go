package realtime

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/gateway/internal/sse"
)

func prepare(t *testing.T, name, data string) sse.PreparedEvent {
	t.Helper()
	p, err := NewCommentChannel(&config.Config{}).Personalizer()(name, []byte(data))
	if err != nil {
		t.Fatalf("Personalizer(%s): %v", name, err)
	}
	return p
}

// 解析失敗要丟掉事件，不能把含內部 id 的原始 payload 送出去。
func TestCommentPersonalizer_ParseFailureFailsClosed(t *testing.T) {
	cases := map[string][2]string{
		"created not json":        {"comment.created", `not json`},
		"created without comment": {"comment.created", `{"clientRequestId":"x"}`},
		"created null comment":    {"comment.created", `{"comment":null}`},
		"created bad userId":      {"comment.created", `{"comment":{"userId":"abc","role":1}}`},
		"like not json":           {"comment.like_changed", `[`},
		"like bad userId":         {"comment.like_changed", `{"userId":"abc","userKind":1}`},
		"batch with bad entry":    {"comment.batch", `{"events":[{"event":"comment.like_changed","data":{"userId":"x","userKind":1},"id":"1"}]}`},
		"batch not json":          {"comment.batch", `{`},
	}
	for name, c := range cases {
		if _, err := NewCommentChannel(&config.Config{}).Personalizer()(c[0], []byte(c[1])); err == nil {
			t.Errorf("%s: want an error so the event is dropped", name)
		}
	}
}

// 任何輸出都不含內部欄位 userId／userKind；like 用 likedByViewer，留言用 isMine。
func TestCommentPersonalizer_NoInternalIDsInAnyVariant(t *testing.T) {
	events := map[string]string{
		"comment.created":      `{"comment":{"commentId":"1","userId":"9","role":1}}`,
		"comment.replied":      `{"comment":{"commentId":"1","userId":"9","role":2}}`,
		"comment.like_changed": `{"commentId":"1","likeCount":1,"action":"LIKE","userId":"9","userKind":1}`,
		"comment.batch":        `{"events":[{"event":"comment.created","data":{"comment":{"userId":"9","role":1}},"id":"1"},{"event":"comment.like_changed","data":{"userId":"9","userKind":2},"id":"2"}]}`,
	}
	for name, data := range events {
		p := prepare(t, name, data)
		for _, v := range []sse.Viewer{{}, {BuyerID: 9}, {StoreID: 9}, {BuyerID: 9, StoreID: 9}} {
			out, err := p.Render(v)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if strings.Contains(string(out), `"userId"`) || strings.Contains(string(out), `"userKind"`) {
				t.Errorf("%s viewer %+v leaks internal identity: %s", name, v, out)
			}
		}
	}
	like := prepare(t, "comment.like_changed", events["comment.like_changed"])
	out, _ := like.Render(sse.Viewer{BuyerID: 9})
	if !strings.Contains(string(out), `"likedByViewer":true`) || strings.Contains(string(out), "isMine") {
		t.Errorf("like_changed for the liker = %s, want likedByViewer=true and no isMine", out)
	}
	out, _ = like.Render(sse.Viewer{StoreID: 9}) // store 9 is not buyer 9
	if !strings.Contains(string(out), `"likedByViewer":false`) {
		t.Errorf("like_changed for a same-number store = %s, want likedByViewer=false", out)
	}
}

// 單筆事件只有兩個 variant，與觀看者數量無關。
func TestCommentPersonalizer_VariantsAreMineOrNot(t *testing.T) {
	p := prepare(t, "comment.created", `{"comment":{"userId":"9","role":1}}`)
	keys := map[string]bool{}
	for _, v := range []sse.Viewer{{}, {BuyerID: 1}, {BuyerID: 2, StoreID: 3}, {StoreID: 9}, {BuyerID: 9}, {BuyerID: 9, StoreID: 5}} {
		keys[p.Variant(v)] = true
	}
	if len(keys) != 2 {
		t.Errorf("variants = %v, want exactly 2", keys)
	}
}
