// Package history 讀買家最近一次對話的歷史（GetLatestAssistantSession）。
package history

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/buyeragent"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

// MaxMessages 是 Latest 回傳的訊息數上限：只回最近的這麼多則，不分頁。
const MaxMessages = 100

const eventsPerMessage = 4

// Latest 回 buyerID 最近更新的對話與它的文字訊息；只列這個買家（user_id）的 session，
// 所以不可能讀到別人的對話。沒有對話時（含列出後對話被刪除）回空的 History。
//
// 規則照舊 AI 服務：以最後更新時間挑最新的對話；只取有文字的事件，thought 不外流，tool 的呼叫與回應事件沒有文字所以略過；
// 非買家發言一律是助理。
func Latest(ctx context.Context, sessions session.Service, buyerID int64) (model.History, error) {
	userID := strconv.FormatInt(buyerID, 10)
	list, err := sessions.List(ctx, &session.ListRequest{AppName: buyeragent.AppName, UserID: userID})
	if err != nil {
		return model.History{}, fmt.Errorf("assistant：列對話：%w", err)
	}
	if len(list.Sessions) == 0 {
		return model.History{}, nil
	}
	latest := list.Sessions[0]
	for _, s := range list.Sessions[1:] {
		if s.LastUpdateTime().After(latest.LastUpdateTime()) {
			latest = s
		}
	}
	got, err := sessions.Get(ctx, &session.GetRequest{AppName: buyeragent.AppName, UserID: userID, SessionID: latest.ID(), NumRecentEvents: MaxMessages * eventsPerMessage})
	if errors.Is(err, session.ErrNotFound) {
		return model.History{}, nil
	}
	if err != nil {
		return model.History{}, fmt.Errorf("assistant：讀對話：%w", err)
	}
	out := model.History{SessionID: latest.ID(), Messages: []model.Message{}}
	for ev := range got.Session.Events().All() {
		if ev.Content == nil {
			continue
		}
		var text string
		for _, part := range ev.Content.Parts {
			if part.Thought {
				continue
			}
			text += part.Text
		}
		if text == "" {
			continue
		}
		role := model.RoleAssistant
		if ev.Content.Role == string(genai.RoleUser) {
			role = model.RoleUser
		}
		out.Messages = append(out.Messages, model.Message{Role: role, Text: text, Time: ev.Timestamp})
	}
	if len(out.Messages) > MaxMessages {
		out.Messages = out.Messages[len(out.Messages)-MaxMessages:]
	}
	return out, nil
}
