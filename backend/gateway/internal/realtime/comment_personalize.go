package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/sse"
)

// Actor kind 與 social 的 cms_comment.role／actor_kind 同一套值域。
const (
	actorKindBuyer int32 = 1
	actorKindStore int32 = 2
)

// errCommentPersonalize 包住所有個人化失敗；事件會被丟掉，不會改送內部形狀。
var errCommentPersonalize = errors.New("comment event personalization failed")

// CommentPersonalizer 是留言串流的 sse.Personalizer。
//
// Hub 內的事件是所有連線共用的內部形狀，帶著留言者／按讚者的 (kind, id)；公開輸出不能有內部身分 ID，
// 所以事件寫出前依觀看者此刻的 buyer_id、store_id（沒有為 0，訪客兩者皆 0）把 (kind, id) 換成
// 布林並拿掉 userId、userKind：留言事件是 isMine（留言者是不是觀看者），按讚事件是 likedByViewer
// （按讚者是不是觀看者）。buyer 7 與 store 7 是不同的人：只有 kind 與 id 同時相符才算。
//
// 解析在事件進 Hub 時只做一次；每個事件的版本數有限（單筆是「是我」與「不是我」兩種），
// 版本相同的連線共用同一份序列化結果。解析失敗回 error，事件被丟掉（fail closed）。
func (c *CommentChannel) Personalizer() sse.Personalizer {
	return func(eventName string, data []byte) (sse.PreparedEvent, error) {
		node, err := prepareCommentEvent(eventName, data)
		if err != nil {
			return nil, fmt.Errorf("%w: event %s: %w", errCommentPersonalize, eventName, err)
		}
		return node, nil
	}
}

type commentIdentity struct {
	kind int32
	id   int64
}

func (i commentIdentity) isViewer(v sse.Viewer) bool {
	switch i.kind {
	case actorKindBuyer:
		return v.BuyerID != 0 && i.id == v.BuyerID
	case actorKindStore:
		return v.StoreID != 0 && i.id == v.StoreID
	}
	return false
}

// commentNode 是一個已解析的事件：idents 是它內含的身分（依輸出順序），render 依每個身分「是不是觀看者」
// 產生 payload。
type commentNode struct {
	idents []commentIdentity
	render func(mine []bool) ([]byte, error)
}

func (n *commentNode) mineFor(v sse.Viewer) []bool {
	mine := make([]bool, len(n.idents))
	for i, id := range n.idents {
		mine[i] = id.isViewer(v)
	}
	return mine
}

func (n *commentNode) Variant(v sse.Viewer) string {
	mine := n.mineFor(v)
	key := make([]byte, len(mine))
	for i, m := range mine {
		key[i] = '0'
		if m {
			key[i] = '1'
		}
	}
	return string(key)
}

func (n *commentNode) Render(v sse.Viewer) ([]byte, error) {
	return n.render(n.mineFor(v))
}

func prepareCommentEvent(eventName string, data []byte) (*commentNode, error) {
	switch eventName {
	case commentBatchEventName:
		return prepareCommentBatch(data)
	case "comment.created", "comment.replied":
		return prepareCommentSnapshotEvent(data)
	case "comment.like_changed":
		return prepareCommentLikeChanged(data)
	}
	// 其他事件不含身分欄位，原樣送出。
	return &commentNode{render: func([]bool) ([]byte, error) { return data, nil }}, nil
}

func prepareCommentBatch(data []byte) (*commentNode, error) {
	var batch struct {
		Events []struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
			ID    string          `json:"id"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &batch); err != nil {
		return nil, err
	}
	children := make([]*commentNode, len(batch.Events))
	node := &commentNode{}
	for i := range batch.Events {
		child, err := prepareCommentEvent(batch.Events[i].Event, batch.Events[i].Data)
		if err != nil {
			return nil, err
		}
		children[i] = child
		node.idents = append(node.idents, child.idents...)
	}
	node.render = func(mine []bool) ([]byte, error) {
		off := 0
		for i, child := range children {
			out, err := child.render(mine[off : off+len(child.idents)])
			if err != nil {
				return nil, err
			}
			batch.Events[i].Data = out
			off += len(child.idents)
		}
		return json.Marshal(batch)
	}
	return node, nil
}

func prepareCommentSnapshotEvent(data []byte) (*commentNode, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	rawComment, ok := envelope["comment"]
	if !ok || string(rawComment) == "null" {
		return nil, errors.New("missing comment")
	}
	var identity struct {
		UserID int64 `json:"userId,string"`
		Role   int32 `json:"role"`
	}
	if err := json.Unmarshal(rawComment, &identity); err != nil {
		return nil, err
	}
	var comment map[string]json.RawMessage
	if err := json.Unmarshal(rawComment, &comment); err != nil {
		return nil, err
	}
	delete(comment, "userId")
	return &commentNode{
		idents: []commentIdentity{{kind: identity.Role, id: identity.UserID}},
		render: func(mine []bool) ([]byte, error) {
			comment["isMine"] = boolJSON(mine[0])
			rewritten, err := json.Marshal(comment)
			if err != nil {
				return nil, err
			}
			envelope["comment"] = rewritten
			return json.Marshal(envelope)
		},
	}, nil
}

func prepareCommentLikeChanged(data []byte) (*commentNode, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	var liker struct {
		UserID   string `json:"userId"`
		UserKind int32  `json:"userKind"`
	}
	if err := json.Unmarshal(data, &liker); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(liker.UserID, 10, 64)
	if err != nil {
		return nil, err
	}
	delete(payload, "userId")
	delete(payload, "userKind")
	return &commentNode{
		idents: []commentIdentity{{kind: liker.UserKind, id: id}},
		render: func(mine []bool) ([]byte, error) {
			payload["likedByViewer"] = boolJSON(mine[0])
			return json.Marshal(payload)
		},
	}, nil
}

func boolJSON(v bool) json.RawMessage {
	if v {
		return json.RawMessage("true")
	}
	return json.RawMessage("false")
}
