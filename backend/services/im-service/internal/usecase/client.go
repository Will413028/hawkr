package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Will413028/hawkr/im-service/internal/domain"
	"go.uber.org/zap"
)

const (
	typingRateWindow = time.Second // 每個 (sender, receiver) pair 最多 1 個 typing 事件/秒
)

// ClientFrame is the transport-neutral form of an inbound receive-only chat
// frame. The Gateway typed gRPC stream feeds this processor so the whitelist,
// authorization, and rate limits stay at the IM domain boundary.
type ClientFrame struct {
	Message       domain.Message
	Conversations map[string]int64
}

// ClientFrameProcessor owns state that must live for one connection, such as
// the typing rate-limit window. It is transport-neutral and must not be shared
// between connections.
type ClientFrameProcessor struct {
	typingLastSent map[int64]time.Time
}

// NewClientFrameProcessor creates a processor for one client connection.
func NewClientFrameProcessor() *ClientFrameProcessor {
	return &ClientFrameProcessor{typingLastSent: make(map[int64]time.Time)}
}

// ProcessClientFrame applies the receive-only chat frame policy used by the
// Gateway-to-IM gRPC stream. In particular there is deliberately no inbound
// "chat" case: persistence remains POST REST →
// Gateway → SendMessage unary, so client frames cannot inject cross-user chat.
func ProcessClientFrame(ctx context.Context, client *domain.Client, h domain.Hub, msgUC *MessageUseCase, frame ClientFrame) {
	NewClientFrameProcessor().Process(ctx, client, h, msgUC, frame)
}

// Process applies one frame using this connection's rate-limit state.
func (p *ClientFrameProcessor) Process(ctx context.Context, client *domain.Client, h domain.Hub, msgUC *MessageUseCase, frame ClientFrame) {
	if p == nil {
		p = NewClientFrameProcessor()
	}
	msg := frame.Message
	switch msg.Type {
	case "sync":
		handleSyncCursors(ctx, client, h, msgUC, frame.Conversations)

	case "presence_sub":
		if peer, ok := parsePresenceKey(msg); ok {
			h.SubscribePresence(client, peer)
		}

	case "presence_unsub":
		if peer, ok := parsePresenceKey(msg); ok {
			h.UnsubscribePresence(client, peer)
		}

	case "typing", "stop_typing":
		// Each gRPC stream owns one processor goroutine, so this per-connection
		// map remains lock-free and rate-limits one pair.
		// The transport adapter supplies the target in msg.To.
		if msg.To > 0 && allowTypingRate(p.typingLastSent, msg.To) {
			h.SendToUser(msg.To, &msg)
			zap.L().Debug("typing relayed", zap.String("type", msg.Type),
				zap.Int64("from", client.UserID), zap.Int64("to", msg.To))
		}

	case "read_receipt":
		// Peer-targeted read receipt: relay to the reader's counterparty so
		// their sent bubbles flip to 已讀 live. Persisted read state stays on
		// HTTP (MarkMessagesRead); this frame is only the live cue.
		if msg.To > 0 {
			h.SendToUser(msg.To, &msg)
		}

	case "ping":
		// Reply on THIS connection only (not h.SendToUser, which fans to every
		// device of the user). gRPC transport liveness is handled by keepalive;
		// the app-level pong remains part of the public client contract.
		if data, err := json.Marshal(&domain.Message{
			Type:      "pong",
			Timestamp: time.Now().UnixMilli(),
		}); err == nil {
			select {
			case client.Send <- data:
			default:
				// A full queue means the writer is already wedged; dropping the
				// pong lets the client reconnect instead of blocking this loop.
			}
		}

	default:
		// Whitelist drop: unknown types and inbound chat are never relayed.
		zap.L().Debug("dropping unhandled chat frame",
			zap.String("type", msg.Type), zap.Int64("from", client.UserID))
	}
}

// allowTypingRate 檢查是否允許對 to 發送 typing 事件（每秒最多 1 個）。
// lastSent 是 per-goroutine map，無需加鎖。
func allowTypingRate(lastSent map[int64]time.Time, to int64) bool {
	now := time.Now()
	if last, ok := lastSent[to]; ok && now.Sub(last) < typingRateWindow {
		return false
	}
	lastSent[to] = now
	return true
}

// parsePresenceKey extracts peer (userID, loginRole) from a presence_sub/unsub
// message. Both `to` (peer userID) and `peerRole` (1=buyer, 2=seller) must be set.
func parsePresenceKey(msg domain.Message) (domain.PresenceKey, bool) {
	if msg.To <= 0 {
		return domain.PresenceKey{}, false
	}
	if msg.PeerRole != 1 && msg.PeerRole != 2 {
		return domain.PresenceKey{}, false
	}
	return domain.PresenceKey{UserID: msg.To, LoginRole: msg.PeerRole}, true
}

// maxSyncRoomsPerFrame bounds how many rooms a single sync frame may request
// (#2499); draft value, tune after observing real dev traffic.
const maxSyncRoomsPerFrame = 20

func handleSyncCursors(ctx context.Context, client *domain.Client, h domain.Hub, msgUC *MessageUseCase, conversations map[string]int64) {
	if len(conversations) > maxSyncRoomsPerFrame {
		zap.L().Warn("sync request exceeds room cap, dropping frame",
			zap.Int64("userID", client.UserID), zap.Int("rooms", len(conversations)))
		return
	}

	if !msgUC.allowSyncFrame(ctx, client.UserID) {
		zap.L().Warn("sync frame rate-limited", zap.Int64("userID", client.UserID))
		return
	}

	for roomID, lastSeq := range conversations {
		if err := authorizeRoomAccess(roomID, client.UserID, client.LoginRole); err != nil {
			// #2499: identical to "no new messages" — no repository call, no
			// signal that distinguishes "not yours" from "empty room".
			h.SendToUser(client.UserID, &domain.Message{
				Type:      "sync_done",
				RoomID:    roomID,
				HasMore:   false,
				Timestamp: time.Now().UnixMilli(),
			})
			continue
		}

		msgs, err := msgUC.SyncMessages(ctx, roomID, lastSeq, MaxSyncLimit)
		if err != nil {
			zap.L().Error("SyncMessages failed", zap.String("roomID", roomID), zap.Error(err))
			continue
		}

		for _, m := range msgs {
			h.SendToUser(client.UserID, &domain.Message{
				Type:   "chat",
				RoomID: m.RoomID,
				From:   m.SenderID,
				To:     m.ReceiverID,
				// gap-fill (補洞) is a read path too: moderation-hidden / recalled
				// messages must render a placeholder here, not the stored text, so a
				// cold-start resync can't bypass a HideMessage (B-135 compliance).
				Content:   TombstoneContent(m),
				MessageID: m.ID,
				SeqID:     m.SeqID,
				Timestamp: timeValue(m.CreatedAt).UnixMilli(),
			})
		}

		// 每個 room 回傳 sync_done，帶上 has_more 讓 client 知道是否需要繼續 sync
		h.SendToUser(client.UserID, &domain.Message{
			Type:      "sync_done",
			RoomID:    roomID,
			HasMore:   len(msgs) == int(MaxSyncLimit),
			Timestamp: time.Now().UnixMilli(),
		})
	}
}
