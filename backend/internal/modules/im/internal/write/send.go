package write

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/live"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/requestid"
	"github.com/Will413028/hawkr/backend/internal/platform/textfilter"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// target 是一次送出的雙方：對話的買家、店家，以及店家的完整資料（G1、G2 用）。
type target struct {
	buyerID int64
	store   seller.Store
}

// Send 送出一則訊息。順序：輸入驗證 → 解析對話與對方（含同一人擋）→ 重送判定的快速路徑（transaction 外，同一個 RequestID 回第一則）→ 守門 G1..G5（transaction 外的讀取）→
// transaction 內先取 (買家, 店家, 寄件方, RequestID) 的 advisory lock，再在 transaction 內重做重送判定，之後才取買家與店家的守門鎖（FOR KEY SHARE，取到即是這次寫入看到的狀態）、
// Confirm 圖片、配發 seq 並推進寄件人的已讀游標、寫訊息、發 MessageSent；任何一步失敗整筆回滾，圖片仍是暫存。
//
// 並行的同一個 RequestID 以入口的 advisory lock 序列化（B1-D26）：輸家等到贏家提交後，在 transaction 內的重送判定看到第一則並回傳，不會走到 Confirm。
// messages 的唯一鍵留作最後防線，撞到就是程式錯誤（Internal）。快速路徑只為「已送出的訊息不因之後才發生的封鎖或停權而重送失敗」：守門在 transaction 之外，
// 重送判定不能排在它後面。
func (s *Service) Send(ctx context.Context, in model.SendInput) (model.Message, error) {
	l := uitext.LocaleFrom(ctx)
	body, err := s.checkInput(l, &in)
	if err != nil {
		return model.Message{}, err
	}
	t, buyerAccount, err := s.resolve(ctx, l, in)
	if err != nil {
		return model.Message{}, err
	}
	if first, found, err := s.replay(ctx, l, in, t, body); err != nil || found {
		return first, err
	}
	if err := s.guard(ctx, l, in.Sender, t, body); err != nil {
		return model.Message{}, err
	}

	uploader := buyerAccount
	if in.Sender.Kind == model.PartyStore {
		uploader = t.store.AccountID
	}

	var out model.Message
	err = s.inTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		sender := in.Sender.Kind
		if err := q.LockSendRequest(ctx, store.LockSendRequestParams{
			BuyerID: t.buyerID, StoreID: t.store.StoreID, SenderKind: string(sender), RequestID: in.RequestID,
		}); err != nil {
			return err
		}
		if first, found, err := s.replay(ctx, l, in, t, body); err != nil || found {
			out = first
			return err
		}
		// 守門鎖緊接在 LockSendRequest 與 transaction 內的重送判定之後，在 guard（含 Orders 的網路呼叫，transaction 之外）之後、AllocateSeq 之前：
		// 買家先於店家（鎖序同 accountdeletion），持有到 transaction 結束。取到之後才算這次寫入看到的狀態，不重用 guard 的讀取結果。
		// 不得把 guard 或 legacy 呼叫搬進這個 transaction（列鎖會跨網路呼叫）。
		// 兩個守門鎖都取（買家先於店家），再依寄件人優先翻譯 NotFound：雙方都已停用時回「寄件人已停用」，不因取鎖順序變成收件人訊息。
		buyerErr := s.d.Buyers.LockNotClosed(ctx, t.buyerID)
		if buyerErr != nil && apperr.KindOf(buyerErr) != apperr.KindNotFound {
			return buyerErr
		}
		storeErr := s.d.Stores.LockNotRetired(ctx, t.store.StoreID)
		if storeErr != nil && apperr.KindOf(storeErr) != apperr.KindNotFound {
			return storeErr
		}
		if err := s.closedGuardError(l, buyerErr, storeErr, sender); err != nil {
			return err
		}
		seq, err := q.AllocateSeq(ctx, store.AllocateSeqParams{
			NewID: s.d.NewID(), BuyerID: t.buyerID, StoreID: t.store.StoreID, SenderKind: string(sender),
		})
		if err != nil {
			return err
		}
		now := seq.LastMessageAt.UTC()
		msgID := s.d.NewID()
		p := store.InsertMessageParams{ID: msgID, ConversationID: seq.ID, Seq: seq.LastSeq, SenderKind: string(sender), RequestID: in.RequestID, MessageType: string(in.Type), Now: now}
		if in.Type == model.TypeText {
			p.Content = &body
		} else {
			asset, err := media.ConfirmField(ctx, s.d.Media, media.Uploader{ID: uploader}, "StagedToken", in.StagedToken, media.VisibilityPublic)
			if err != nil {
				return err
			}
			p.ImageAssetID, p.ImageObjectKey = &asset.ID, &asset.ObjectKey
		}
		row, err := q.InsertMessage(ctx, p)
		if err != nil {
			return err
		}
		if err := events.Publish(ctx, s.d.Events, model.MessageSent{
			EventID: s.d.NewID(), OccurredAt: now, ConversationID: seq.ID, MessageID: msgID, Seq: seq.LastSeq,
			SenderKind: sender, BuyerID: t.buyerID, StoreID: t.store.StoreID,
		}); err != nil {
			return err
		}
		if err := q.Notify(ctx, live.Message(seq.ID, t.buyerID, t.store.StoreID, seq.LastSeq).Encode()); err != nil {
			return err
		}
		out = rows.Message(rows.MessageRow{
			ID: row.ID, ConversationID: row.ConversationID, Seq: row.Seq, SenderKind: row.SenderKind, MessageType: row.MessageType,
			Content: row.Content, ImageAssetID: row.ImageAssetID, ImageObjectKey: row.ImageObjectKey, CreatedAt: row.CreatedAt, ErasedAt: row.ErasedAt,
		})
		return nil
	})
	return out, err
}

// replay 是送出冪等的重送判定：同一個 (對話, 寄件方, RequestID) 已有訊息時回傳第一次寫入的那則（found 為真），不配 seq、不通知、不發事件、不碰圖片；
// 內容不同（Type 不同，或文字訊息正規化後的文字不同）回 InvalidInput 指向 RequestID。圖片訊息只比 Type（staged token 在第一次已被消耗）。
// 對話還不存在時沒有列，照常往下走。判定在對話解析與授權之後、守門之前：已送出的訊息不因之後才發生的封鎖或停權而重送失敗。
func (s *Service) replay(ctx context.Context, l uitext.Locale, in model.SendInput, t target, body string) (model.Message, bool, error) {
	row, err := s.q(ctx).MessageByRequest(ctx, store.MessageByRequestParams{
		BuyerID: t.buyerID, StoreID: t.store.StoreID, SenderKind: string(in.Sender.Kind), RequestID: in.RequestID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Message{}, false, nil
	}
	if err != nil {
		return model.Message{}, false, err
	}
	same := row.MessageType == string(in.Type)
	if same && in.Type == model.TypeText && row.ErasedAt == nil {
		same = row.Content != nil && *row.Content == body
	}
	if !same {
		return model.Message{}, false, apperr.InvalidInputOn([]string{"RequestID"}, "%s", text.ImRequestIdReused(l))
	}
	return rows.Message(rows.MessageRow(row)), true, nil
}

// closedGuardError 把兩個守門的 NotFound（已關閉、已退場或不存在）翻成原有的訊息與 reason：寄件人那一方先翻（寄件人已停用），
// 收件人那一方其次（對方已刪除）；兩者都通過回 nil。呼叫前非 NotFound 的錯誤已原樣回傳（fail-closed）。
func (s *Service) closedGuardError(l uitext.Locale, buyerErr, storeErr error, sender model.PartyKind) error {
	senderErr, receiverErr := storeErr, buyerErr
	if sender == model.PartyBuyer {
		senderErr, receiverErr = buyerErr, storeErr
	}
	if senderErr != nil {
		return apperr.FailedPrecondition("%s", text.ImSenderInactive(l))
	}
	if receiverErr != nil {
		return model.WithReason(apperr.FailedPrecondition("%s", text.ImReceiverDeleted(l)), model.ReasonReceiverDeleted)
	}
	return nil
}

func counterpart(k model.PartyKind) model.PartyKind {
	if k == model.PartyBuyer {
		return model.PartyStore
	}
	return model.PartyBuyer
}

func partyID(k model.PartyKind, t target) int64 {
	if k == model.PartyBuyer {
		return t.buyerID
	}
	return t.store.StoreID
}

// checkInput 驗證輸入並回傳文字訊息去頭尾空白後的內容（圖片訊息回空字串）。其他 message_type 一律拒絕，不把空字串或未知值當有效值。
func (s *Service) checkInput(l uitext.Locale, in *model.SendInput) (string, error) {
	if !in.Sender.Kind.Valid() || in.Sender.ID() <= 0 || (in.Sender.Kind == model.PartyStore && in.Sender.Store == nil) {
		return "", errors.New("im：寄件人身分不完整")
	}
	if !requestid.IsV4(in.RequestID) {
		return "", apperr.InvalidInputOn([]string{"RequestID"}, "%s", text.ImRequestIdInvalid(l))
	}
	switch in.Type {
	case model.TypeText:
		if !utf8.ValidString(in.Text) || strings.ContainsRune(in.Text, 0) {
			return "", apperr.InvalidInputOn([]string{"Text"}, "%s", text.ImTextInvalid(l))
		}
		body := trimtext.Trim(in.Text)
		if body == "" {
			return "", apperr.InvalidInputOn([]string{"Text"}, "%s", text.ImTextRequired(l))
		}
		if utf8.RuneCountInString(body) > model.MaxTextRunes {
			return "", apperr.InvalidInputOn([]string{"Text"}, "%s", text.ImTextTooLong(l, text.ImTextTooLongArgs{Max: model.MaxTextRunes}))
		}
		return body, nil
	case model.TypeImage:
		in.StagedToken = trimtext.Trim(in.StagedToken)
		if in.StagedToken == "" {
			return "", apperr.InvalidInputOn([]string{"StagedToken"}, "%s", text.ImImageTokenRequired(l))
		}
		return "", nil
	}
	return "", apperr.InvalidInputOn([]string{"Type"}, "%s", text.ImTypeUnsupported(l))
}

func unavailable(l uitext.Locale, cause error) error {
	return apperr.WithCause(apperr.Unavailable("%s", text.ImIdentityUnavailable(l)), cause)
}

// resolve 解析對話與對方（不信 client 的寄件人）：給 ConversationID 時以對話授權，寄件人不是該對話的一方回 NotFound；
// 首則訊息用 CounterpartID，並確認對方存在。同一人（買家的帳號等於店家的帳號）在這裡擋。回傳買家的 account_id（上傳者與同一人判斷用）。
func (s *Service) resolve(ctx context.Context, l uitext.Locale, in model.SendInput) (target, int64, error) {
	var buyerID, storeID int64
	existing := in.ConversationID != (uuid.UUID{})
	if existing {
		c, err := s.q(ctx).GetConversation(ctx, in.ConversationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return target{}, 0, apperr.NotFound("%s", text.ImConversationNotFound(l))
		}
		if err != nil {
			return target{}, 0, err
		}
		if (in.Sender.Kind == model.PartyBuyer && c.BuyerID != in.Sender.BuyerID) || (in.Sender.Kind == model.PartyStore && c.StoreID != in.Sender.Store.StoreID) {
			return target{}, 0, apperr.NotFound("%s", text.ImConversationNotFound(l))
		}
		buyerID, storeID = c.BuyerID, c.StoreID
	} else {
		if in.CounterpartID <= 0 {
			return target{}, 0, apperr.InvalidInputOn([]string{"CounterpartID"}, "%s", text.ImCounterpartRequired(l))
		}
		if in.Sender.Kind == model.PartyBuyer {
			buyerID, storeID = in.Sender.BuyerID, in.CounterpartID
		} else {
			buyerID, storeID = in.CounterpartID, in.Sender.Store.StoreID
		}
	}

	t := target{buyerID: buyerID}
	if in.Sender.Kind == model.PartyStore {
		t.store = *in.Sender.Store
		if !existing {
			if _, err := s.d.Buyers.ProfileByBuyerID(ctx, buyerID); err != nil {
				return target{}, 0, s.counterpartErr(l, err)
			}
		}
	} else {
		st, err := s.d.Stores.Store(ctx, storeID)
		if err != nil {
			return target{}, 0, s.counterpartErr(l, err)
		}
		t.store = st
	}
	account, err := s.d.Buyers.AccountIDByBuyerID(ctx, buyerID)
	if err != nil {
		return target{}, 0, s.counterpartErr(l, err)
	}
	if account == t.store.AccountID {
		return target{}, 0, model.WithReason(apperr.InvalidInput("%s", text.ImSelfMessageNotAllowed(l)), model.ReasonSelfMessageNotAllowed)
	}
	return t, account, nil
}

// counterpartErr 把「對方不存在」與「查詢失敗」分開：前者 NotFound，後者 Unavailable（不放行，不當成不存在）。
func (s *Service) counterpartErr(l uitext.Locale, err error) error {
	if apperr.KindOf(err) == apperr.KindNotFound {
		return apperr.NotFound("%s", text.ImCounterpartNotFound(l))
	}
	return unavailable(l, err)
}

// guard 是守門 G1..G5，順序固定：停權 → 停業 → 封鎖 → 收件人已刪除 → 詞典。每一項都是 in-process 讀取，任何查詢錯誤都回錯誤（fail-closed），
// 例外（進行中的訂單或售後）只在 G1 或 G3 命中時才查，兩者共用一次查詢結果。body 是文字訊息的內容（圖片訊息為空，不過詞典）。
func (s *Service) guard(ctx context.Context, l uitext.Locale, sender model.Party, t target, body string) error {
	var openKnown, open bool
	hasOpenOrder := func() (bool, error) {
		if !openKnown {
			v, err := s.d.Orders.OpenOrderBetween(ctx, t.buyerID, t.store.StoreID)
			if err != nil {
				return false, err
			}
			open, openKnown = v, true
		}
		return open, nil
	}

	// G1 停權：只擋寄件人是店家的情況；雙方之間有進行中的訂單或售後時放行。
	if sender.Kind == model.PartyStore && sender.Store.Suspended() {
		v, err := hasOpenOrder()
		if err != nil {
			return unavailable(l, err)
		}
		if !v {
			return model.WithReason(apperr.PermissionDenied("%s", text.ImSellerSuspended(l)), model.ReasonSellerSuspended)
		}
	}

	// G2 停業：只有店家狀態 CLOSED 算停業（等待刪除雙向都可以聊）；能力判斷以 seller 為準，兩個方向同一個條件。
	if err := seller.Authorize(t.store, seller.CapChatSend); err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return model.WithReason(apperr.FailedPrecondition("%s", text.ImStoreClosed(l)), model.ReasonStoreClosed)
		}
		return unavailable(l, err)
	}

	// G3 封鎖：任一方向都擋；有進行中的訂單或售後時放行。
	blocked, err := s.d.Buyers.IsBlockingStore(ctx, t.buyerID, t.store.StoreID)
	if err != nil {
		return unavailable(l, err)
	}
	if !blocked {
		if blocked, err = s.d.Stores.StoreBlocksBuyer(ctx, t.store.StoreID, t.buyerID); err != nil {
			return unavailable(l, err)
		}
	}
	if blocked {
		v, err := hasOpenOrder()
		if err != nil {
			return unavailable(l, err)
		}
		if !v {
			return model.WithReason(apperr.FailedPrecondition("%s", text.ImChatBlocked(l)), model.ReasonChatBlocked)
		}
	}

	// G4 收件人已刪除：transaction 外的這次檢查只是優化，讓已刪除的收件人不必先跑一次媒體 Finalize；正確性由 transaction 內的守門鎖（LockNotClosed、LockNotRetired）保證。
	receiver := counterpart(sender.Kind)
	gone, err := s.inactive(ctx, receiver, partyID(receiver, t))
	if err != nil {
		return err
	}
	if gone {
		return model.WithReason(apperr.FailedPrecondition("%s", text.ImReceiverDeleted(l)), model.ReasonReceiverDeleted)
	}

	// G5 詞典：只查文字。
	if body != "" {
		if hit, _ := textfilter.Check(body); hit {
			return model.WithReason(apperr.InvalidInputOn([]string{"Text"}, "%s", text.ImModerationRejected(l)), model.ReasonModerationRejected)
		}
	}
	return nil
}
