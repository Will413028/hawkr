// Package imrelay 將 im 模組事件轉成還在 RabbitMQ 上的消費者（notification 的離線推播）所需的訊息。
//
// 對應（送到 hawkr.im，wire 形狀由 wire_test 逐欄位釘住，模板讀的 key 由 golden_test 對照）：
//
//	im.MessageSent → chat.message.sent
//
// 與舊 im-service 的差異只有兩個值：conversation_id 從房名 b<id>_s<id> 變成對話 UUID，message_id 從 TSID 十進位字串變成訊息 UUID
// （wire 型別都還是 string）。其他欄位的值與身分空間不變：receiver_id 收件人是買家時送 buyer_id、
// 是店家時送店家的 account_id。
//
// cmd/hawkr 在 serve 與 worker 訂閱事件、worker 登記 job。舊 im-service 已刪除，所有聊天訊息都由單體的 im 送出。notification 搬進單體後刪除。
package imrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	exchangeIM = "hawkr.im"
	keySent    = "chat.message.sent"
	// previewMaxRunes 是舊 PreviewMaxLength：超過時截斷並加上 "..."。
	previewMaxRunes = 100
	// 推播預覽的過渡句子：共用文案目錄沒有聊天條目，有了之後改取同一批（見 im 的 CARRYOVER）。
	imagePreview = "傳送了一張圖片"
)

// MessageSentArgs 轉成 chat.message.sent。以整組參數（含事件 ID）做 unique：同一則事件只排一個。
type MessageSentArgs struct {
	EventID   string    `json:"event_id"`
	Occurred  time.Time `json:"occurred"`
	MessageID string    `json:"message_id"`
}

// Kind 是 River 的 job 種類。
func (MessageSentArgs) Kind() string { return "legacy.im_message_sent" }

// InsertOpts 以整組參數做 unique：父 job 重試重排同一組時只留一個。
func (MessageSentArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// OnMessageSent 在發布事件的 transaction 內把事件轉成 job（transactional outbox）：排入失敗時訊息的寫入一起回滾。
func OnMessageSent(q jobs.Enqueuer) func(context.Context, im.MessageSent) error {
	return func(ctx context.Context, e im.MessageSent) error {
		return q.Enqueue(ctx, MessageSentArgs{EventID: e.EventID.String(), Occurred: e.OccurredAt, MessageID: e.MessageID.String()}, nil)
	}
}

// Messages 是 relay 讀訊息的 port；*im.Module 實作它。
type Messages interface {
	MessageForRelay(ctx context.Context, id uuid.UUID) (im.RelayMessage, error)
}

// Buyers 是 relay 補暱稱與頭像的 port；*buyer.Module 實作它。
type Buyers interface {
	BatchDisplay(ctx context.Context, ids []int64) ([]buyer.Display, error)
	BuyerIDsByAccount(ctx context.Context, accountID int64) ([]int64, error)
}

// Stores 是 relay 取店家名稱、logo 與帳號的 port；*seller.Module 實作它。
type Stores interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// Worker 組出舊 publisher 的 payload 並排入送出的 job。
type Worker struct {
	Messages Messages
	Buyers   Buyers
	Stores   Stores
	Jobs     jobs.Enqueuer
	Log      *slog.Logger
}

// NewWorker 建立 Worker。
func NewWorker(messages Messages, buyers Buyers, stores Stores, queue jobs.Enqueuer, log *slog.Logger) *Worker {
	return &Worker{Messages: messages, Buyers: buyers, Stores: stores, Jobs: queue, Log: log}
}

type typedWorker[T river.JobArgs] struct {
	river.WorkerDefaults[T]
	work func(context.Context, T) error
}

func (w *typedWorker[T]) Work(ctx context.Context, job *river.Job[T]) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return w.work(ctx, job.Args)
}

// AddWorkers 登記 relay 的 River worker。
func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker[MessageSentArgs]{work: w.messageSent})
}

// chatMessageSent 是 chat.message.sent 的 wire 形狀（沿用舊 im-service 的 ChatMessageSentEvent）：十個欄位全是 string，沒有 BaseEvent 外殼。
type chatMessageSent struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	SenderName     string `json:"sender_name"`
	SenderAvatar   string `json:"sender_avatar"`
	ReceiverID     string `json:"receiver_id"`
	ReceiverType   string `json:"receiver_type"`
	StoreID        string `json:"store_id"`
	ContentPreview string `json:"content_preview"`
	SentAt         string `json:"sent_at"`
}

// messageSent 在 job 執行當下讀訊息現況組 payload：訊息已被抹除或隱藏、收件人身分已刪除或退場（或訊息已不存在）時不送，推播不能帶出已抹除的內容。
// 查詢失敗（買家、店家）回錯誤讓 River 重試；查不到（刪帳的買家、不存在的店家）的名稱與頭像送空字串（舊碼同樣降級）。
func (w *Worker) messageSent(ctx context.Context, a MessageSentArgs) error {
	id, err := uuid.Parse(a.MessageID)
	if err != nil {
		return fmt.Errorf("imrelay：訊息 ID %q 不是 UUID：%w", a.MessageID, err)
	}
	m, err := w.Messages.MessageForRelay(ctx, id)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return nil // 訊息已經不存在：沒有東西可通知
	}
	if err != nil {
		return err
	}
	if m.Hidden || m.Erased || m.ReceiverInactive {
		return nil // 收件人身分已刪除或退場：沒有人可通知
	}

	store, storeFound, err := w.store(ctx, m.StoreID)
	if err != nil {
		return err
	}
	preview, err := previewSentence(m)
	if err != nil {
		return err
	}
	event := chatMessageSent{
		MessageID: m.ID.String(), ConversationID: m.ConversationID.String(), StoreID: itoa(m.StoreID),
		ContentPreview: preview, SentAt: m.CreatedAt.UTC().Format(time.RFC3339),
	}
	switch m.SenderKind {
	case im.PartyBuyer:
		event.SenderID = itoa(m.BuyerID)
		event.SenderName, event.SenderAvatar, err = w.buyerDisplay(ctx, m.BuyerID)
		if err != nil {
			return err
		}
		if !storeFound {
			return nil // 收件的店家不存在：沒有人可通知
		}
		event.ReceiverType, event.ReceiverID = "seller", itoa(store.AccountID)
	case im.PartyStore:
		event.SenderID = itoa(m.StoreID)
		if storeFound {
			event.SenderName = store.StoreName
			event.SenderAvatar, err = w.storeAvatar(ctx, store)
			if err != nil {
				return err
			}
		}
		event.ReceiverType, event.ReceiverID = "buyer", itoa(m.BuyerID)
	default:
		return fmt.Errorf("imrelay：不認得的寄件人 %q", m.SenderKind)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return w.Jobs.Enqueue(ctx, rabbitmq.PublishArgs{EventID: a.EventID, Exchange: exchangeIM, RoutingKey: keySent, Body: body}, rabbitmq.InsertOpts())
}

func (w *Worker) store(ctx context.Context, storeID int64) (seller.Store, bool, error) {
	st, err := w.Stores.Store(ctx, storeID)
	switch {
	case err == nil:
		return st, true, nil
	case apperr.KindOf(err) == apperr.KindNotFound:
		return seller.Store{}, false, nil
	default:
		return seller.Store{}, false, err
	}
}

// buyerDisplay 回傳買家的暱稱與頭像 key；刪帳或查不到的買家沒有。
func (w *Worker) buyerDisplay(ctx context.Context, buyerID int64) (name, avatar string, err error) {
	list, err := w.Buyers.BatchDisplay(ctx, []int64{buyerID})
	if err != nil {
		return "", "", err
	}
	for _, d := range list {
		if d.BuyerID != buyerID || d.Deleted {
			continue
		}
		if d.DisplayName != nil {
			name = *d.DisplayName
		}
		if d.Avatar != nil {
			avatar = *d.Avatar
		}
	}
	return name, avatar, nil
}

// storeAvatar 是店家寄件人的頭像：舊碼用店家 logo，logo 沒設時退回店主帳號買家身分的頭像。
// 帳號的 stint 可能有多個（含已關閉的），取存活的那個；查詢失敗回錯誤，查不到或沒有頭像回空字串。
func (w *Worker) storeAvatar(ctx context.Context, st seller.Store) (string, error) {
	if st.LogoKey != nil && *st.LogoKey != "" {
		return *st.LogoKey, nil
	}
	ids, err := w.Buyers.BuyerIDsByAccount(ctx, st.AccountID)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return "", nil
		}
		return "", err
	}
	if len(ids) == 0 {
		return "", nil
	}
	list, err := w.Buyers.BatchDisplay(ctx, ids)
	if err != nil {
		return "", err
	}
	for _, d := range list {
		if !d.Deleted && d.Avatar != nil {
			return *d.Avatar, nil
		}
	}
	return "", nil
}

// previewSentence 是舊 computePreview 的結果：圖片是固定句子；文字超過 100 個字截斷並加 "..."（文字在寫入時已去頭尾空白）。
// 不認得的類型或沒有內容的文字訊息回錯誤，不送出空預覽。
func previewSentence(m im.RelayMessage) (string, error) {
	switch {
	case m.Type == im.TypeImage:
		return imagePreview, nil
	case m.Type == im.TypeText && m.Text != nil:
		r := []rune(*m.Text)
		if len(r) > previewMaxRunes {
			return string(r[:previewMaxRunes]) + "...", nil
		}
		return *m.Text, nil
	}
	return "", fmt.Errorf("imrelay：訊息 %s 的類型 %q 沒有可用的預覽", m.ID, m.Type)
}

func itoa(v int64) string { return fmt.Sprintf("%d", v) }
