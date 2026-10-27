package grpc

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// orderNoExtraKey is the SendMessageRequest.extra map key the front-end「聯絡賣家/
// 買家」entry uses to carry the order_no *intent* parameter (14 號 §6.5.1 路徑①：
// 訂單情境發起). Never trusted directly — it only seeds a server-side three-way
// validation against order-service in usecase.resolveOrderBinding.
const orderNoExtraKey = "order_no"

// previewExtraKey is the SendMessageRequest.extra map key the client uses to
// carry a human-readable one-line summary for structured (card) messages —
// e.g. "傳送了一張商品卡片". im-service never parses card JSON itself; this
// hint is the only signal it needs. See usecase.WithPreviewHint.
const previewExtraKey = "preview"

// sellerSuspendedFromContext reads the forwarded suspension flag from incoming
// gRPC metadata (14 號 §6.5.4). Fail-open at the transport read, matching
// Gateway's own injectSellerSuspension "不 deny" semantics — the actual
// gating decision happens in usecase.SendMessage. Propagation chain and
// metadata key documented on the shared helper:
// pkg/middleware/grpc/interceptor.SellerSuspendedFromContext.
func sellerSuspendedFromContext(ctx context.Context) bool {
	_, suspended := interceptor.SellerSuspendedFromContext(ctx)
	return suspended
}

// IMGrpcServer implements the gRPC IM service.
type IMGrpcServer struct {
	pb.UnimplementedIMServiceServer
	hub          domain.Hub
	msgUC        *usecase.MessageUseCase
	startTime    time.Time
	messagesSent int64
	messagesRecv int64
}

// NewIMGrpcServer creates a new gRPC server instance.
func NewIMGrpcServer(hub domain.Hub, msgUC *usecase.MessageUseCase) *IMGrpcServer {
	return &IMGrpcServer{
		hub:       hub,
		msgUC:     msgUC,
		startTime: time.Now(),
	}
}

// SendMessage sends a message to a specific user, persisting it to DB.
func (s *IMGrpcServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	pkglogger.Info(ctx, "gRPC SendMessage", zap.String("from", req.FromUserId), zap.String("to", req.ToUserId), zap.Any("type", req.Type), zap.String("accountType", req.AccountType))

	fromUserID, err := strconv.ParseInt(req.FromUserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from_user_id: %s", req.FromUserId)
	}
	toUserID, err := strconv.ParseInt(req.ToUserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to_user_id: %s", req.ToUserId)
	}
	accountType := parseAccountTypeString(req.AccountType)

	saved, err := s.msgUC.SendMessageByUsers(ctx, fromUserID, toUserID, accountType, int32(req.Type), req.Content,
		usecase.WithOrderNoHint(req.GetExtra()[orderNoExtraKey]),
		usecase.WithPreviewHint(req.GetExtra()[previewExtraKey]),
		usecase.WithSellerSuspended(sellerSuspendedFromContext(ctx)))
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.SendMessageResponse{Code: int32(code), Message: err.Error()}, nil
		}
		pkglogger.Error(ctx, "gRPC SendMessage persist failed", zap.Error(err))
		return nil, err
	}
	atomic.AddInt64(&s.messagesSent, 1)
	timestamp := int64(0)
	if saved.CreatedAt != nil {
		timestamp = saved.CreatedAt.UnixMilli()
	}

	return &pb.SendMessageResponse{
		Code:      0,
		Message:   "success",
		Delivered: true,
		MessageId: strconv.FormatInt(saved.ID, 10),
		Timestamp: strconv.FormatInt(timestamp, 10),
		SeqId:     strconv.FormatInt(saved.SeqID, 10),
		// 49 §6.6 im/S4：圖片訊息 content 為 objectKey，回應直送裸 key，顯示 URL 由 Gateway 組。
		Content: saved.Content,
	}, nil
}

// BroadcastMessage sends a message to all connected users.
func (s *IMGrpcServer) BroadcastMessage(ctx context.Context, req *pb.BroadcastRequest) (*pb.BroadcastResponse, error) {
	pkglogger.Info(ctx, "gRPC BroadcastMessage", zap.String("from", req.FromUserId), zap.Any("type", req.Type))

	fromUserID, err := strconv.ParseInt(req.FromUserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from_user_id: %s", req.FromUserId)
	}

	msg := &domain.Message{
		Type:      messageTypeToString(req.Type),
		From:      fromUserID,
		Content:   req.Content,
		Timestamp: time.Now().Unix(),
	}

	excludeMap := make(map[int64]bool)
	for _, uidStr := range req.ExcludeUsers {
		uid, parseErr := strconv.ParseInt(uidStr, 10, 64)
		if parseErr == nil {
			excludeMap[uid] = true
		}
	}

	sentCount := s.hub.BroadcastExcept(msg, excludeMap)
	atomic.AddInt64(&s.messagesSent, int64(sentCount))

	return &pb.BroadcastResponse{
		Code:      0,
		Message:   "success",
		SentCount: strconv.Itoa(sentCount),
	}, nil
}

// IsUserOnline checks if a user is online.
func (s *IMGrpcServer) IsUserOnline(ctx context.Context, req *pb.UserOnlineRequest) (*pb.UserOnlineResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %s", req.UserId)
	}
	online := s.hub.IsOnline(userID)
	lastActive := ""
	if online {
		lastActive = strconv.FormatInt(time.Now().Unix(), 10)
	}

	return &pb.UserOnlineResponse{
		Code:       0,
		Online:     online,
		LastActive: lastActive,
	}, nil
}

// GetConnectionStats returns connection statistics.
func (s *IMGrpcServer) GetConnectionStats(ctx context.Context, req *pb.EmptyRequest) (*pb.ConnectionStatsResponse, error) {
	uptime := int64(time.Since(s.startTime).Seconds())
	connections := s.hub.ConnectionCount()

	return &pb.ConnectionStatsResponse{
		Code:              0,
		TotalConnections:  strconv.Itoa(connections),
		ActiveConnections: strconv.Itoa(connections),
		UptimeSeconds:     strconv.FormatInt(uptime, 10),
		MessagesSent:      strconv.FormatInt(atomic.LoadInt64(&s.messagesSent), 10),
		MessagesReceived:  strconv.FormatInt(atomic.LoadInt64(&s.messagesRecv), 10),
	}, nil
}

// DisconnectUser forcefully disconnects a user.
func (s *IMGrpcServer) DisconnectUser(ctx context.Context, req *pb.DisconnectRequest) (*pb.DisconnectResponse, error) {
	pkglogger.Info(ctx, "gRPC DisconnectUser", zap.String("user", req.UserId), zap.String("reason", req.Reason))

	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %s", req.UserId)
	}
	success := s.hub.DisconnectUser(userID)

	return &pb.DisconnectResponse{
		Code:    0,
		Message: "user disconnected",
		Success: success,
	}, nil
}

// GetUnreadCount returns unread message count for a user, filtered by role.
func (s *IMGrpcServer) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %s", req.UserId)
	}
	accountType := parseAccountTypeString(req.AccountType)

	count, err := s.msgUC.GetUnreadCount(ctx, userID, accountType)
	if err != nil {
		return nil, err
	}
	return &pb.GetUnreadCountResponse{
		Code:        0,
		UnreadCount: strconv.FormatInt(count, 10),
	}, nil
}

// GetChatHistory returns chat history between two users.
// Uses account_type to determine the correct room_id.
func (s *IMGrpcServer) GetChatHistory(ctx context.Context, req *pb.GetChatHistoryRequest) (*pb.GetChatHistoryResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %s", req.UserId)
	}
	targetID, err := strconv.ParseInt(req.TargetId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid target_id: %s", req.TargetId)
	}
	var beforeID int64
	if req.BeforeId != "" {
		beforeID, _ = strconv.ParseInt(req.BeforeId, 10, 64)
	}
	accountType := parseAccountTypeString(req.AccountType)

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	// Use account_type to determine room_id; fall back to trying both
	var room1, room2 string
	switch accountType {
	case 2: // SELLER: user is seller, target is buyer
		room1 = usecase.BuildRoomID(targetID, userID)
		room2 = usecase.BuildRoomID(userID, targetID)
	default: // BUYER or unknown: user is buyer, target is seller
		room1 = usecase.BuildRoomID(userID, targetID)
		room2 = usecase.BuildRoomID(targetID, userID)
	}

	roomID := room1
	msgs, err := s.msgUC.GetHistory(ctx, room1, beforeID, pageSize)
	if err != nil || len(msgs) == 0 {
		roomID = room2
		msgs, err = s.msgUC.GetHistory(ctx, room2, beforeID, pageSize)
		if err != nil {
			return nil, err
		}
	}

	pbMsgs := make([]*pb.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		pbMsgs = append(pbMsgs, toPbChatMessage(m))
	}

	// Enrich with participant profiles
	participants := s.msgUC.GetParticipants(ctx, roomID)
	pbParticipants := make([]*pb.ChatParticipant, 0, len(participants))
	for _, p := range participants {
		pbParticipants = append(pbParticipants, &pb.ChatParticipant{
			UserId:      strconv.FormatInt(p.UserID, 10),
			Role:        p.Role,
			DisplayName: p.DisplayName,
			AvatarKey:   p.AvatarKey,
		})
	}

	return &pb.GetChatHistoryResponse{
		Code:         0,
		Messages:     pbMsgs,
		Total:        strconv.Itoa(len(pbMsgs)),
		HasMore:      int32(len(pbMsgs)) == pageSize,
		Participants: pbParticipants,
	}, nil
}

// GetConversations returns list of conversations for a user, filtered by account_type.
func (s *IMGrpcServer) GetConversations(ctx context.Context, req *pb.GetConversationsRequest) (*pb.GetConversationsResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %s", req.UserId)
	}
	accountType := parseAccountTypeString(req.AccountType)

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}

	convs, err := s.msgUC.GetConversations(ctx, userID, accountType, page, pageSize)
	if err != nil {
		return nil, err
	}

	pbConvs := make([]*pb.Conversation, 0, len(convs))
	for _, c := range convs {
		pbConvs = append(pbConvs, toPbConversation(c))
	}

	return &pb.GetConversationsResponse{
		Code:          0,
		Conversations: pbConvs,
		Total:         strconv.Itoa(len(pbConvs)),
	}, nil
}

// MarkMessagesRead marks specific messages as read.
func (s *IMGrpcServer) MarkMessagesRead(ctx context.Context, req *pb.MarkMessagesReadRequest) (*pb.MarkMessagesReadResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %s", req.UserId)
	}
	messageIDs := make([]int64, 0, len(req.MessageIds))
	for _, idStr := range req.MessageIds {
		id, parseErr := strconv.ParseInt(idStr, 10, 64)
		if parseErr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid message_id: %s", idStr)
		}
		messageIDs = append(messageIDs, id)
	}

	if err := s.msgUC.MarkMessagesByIDs(ctx, userID, messageIDs); err != nil {
		return nil, err
	}
	return &pb.MarkMessagesReadResponse{
		Code:         0,
		Message:      "ok",
		UpdatedCount: strconv.Itoa(len(messageIDs)),
	}, nil
}

// MarkConversationRead marks all messages in a conversation as read.
// Uses account_type to determine room_id; falls back to trying both.
func (s *IMGrpcServer) MarkConversationRead(ctx context.Context, req *pb.MarkConversationReadRequest) (*pb.MarkConversationReadResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %s", req.UserId)
	}
	targetID, err := strconv.ParseInt(req.TargetId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid target_id: %s", req.TargetId)
	}
	accountType := parseAccountTypeString(req.AccountType)

	var room1, room2 string
	switch accountType {
	case 2: // SELLER
		room1 = usecase.BuildRoomID(targetID, userID)
		room2 = usecase.BuildRoomID(userID, targetID)
	default: // BUYER or unknown
		room1 = usecase.BuildRoomID(userID, targetID)
		room2 = usecase.BuildRoomID(targetID, userID)
	}
	_ = s.msgUC.MarkConversationRead(ctx, room1, userID)
	_ = s.msgUC.MarkConversationRead(ctx, room2, userID)

	return &pb.MarkConversationReadResponse{
		Code:    0,
		Message: "ok",
	}, nil
}

// MarkAllRead marks all messages as read for a user, filtered by role.
func (s *IMGrpcServer) MarkAllRead(ctx context.Context, req *pb.MarkAllReadRequest) (*pb.MarkAllReadResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %s", req.UserId)
	}
	accountType := parseAccountTypeString(req.AccountType)

	if err := s.msgUC.MarkAllRead(ctx, userID, accountType); err != nil {
		return nil, err
	}
	return &pb.MarkAllReadResponse{
		Code:    0,
		Message: "ok",
	}, nil
}

// HideMessage — internal RPC called by social-service AdminResolveReport (B-135).
func (s *IMGrpcServer) HideMessage(ctx context.Context, req *pb.HideMessageRequest) (*pb.HideMessageResponse, error) {
	if req.MessageId == 0 {
		return nil, status.Error(codes.InvalidArgument, "message_id required")
	}
	if err := s.msgUC.HideMessage(ctx, req.MessageId, req.HiddenReason); err != nil {
		return nil, err
	}
	return &pb.HideMessageResponse{Code: 0}, nil
}

func toPbChatMessage(m gen.Message) *pb.ChatMessage {
	var isRead bool
	if m.IsRead != nil {
		isRead = *m.IsRead
	}
	var createdAt string
	if m.CreatedAt != nil {
		createdAt = strconv.FormatInt(m.CreatedAt.UnixMilli(), 10)
	}
	return &pb.ChatMessage{
		Id:         strconv.FormatInt(m.ID, 10),
		FromUserId: strconv.FormatInt(m.SenderID, 10),
		ToUserId:   strconv.FormatInt(m.ReceiverID, 10),
		Type:       toProtoMessageType(m.Type),
		// Moderation-hidden / recalled messages render a neutral placeholder; the
		// original text is never sent on the read path (B-135 compliance).
		// 圖片訊息 DB 存 objectKey，此處現組完整顯示 URL（49 §5.3 #2）。
		Content:   usecase.TombstoneContent(m),
		Timestamp: createdAt,
		IsRead:    isRead,
		SeqId:     strconv.FormatInt(m.SeqID, 10),
	}
}

func toPbConversation(c usecase.ConversationWithProfile) *pb.Conversation {
	var lastMsg string
	if c.LastMessageContent != nil {
		// 圖片訊息最後預覽 DB 存 objectKey，現組完整顯示 URL（49 §5.3 #2）
		lastMsg = *c.LastMessageContent
	}
	var lastMsgTime string
	if c.UpdatedAt != nil {
		lastMsgTime = strconv.FormatInt(c.UpdatedAt.UnixMilli(), 10)
	}
	return &pb.Conversation{
		TargetUserId:    strconv.FormatInt(c.TargetUserID, 10),
		TargetNickname:  c.TargetNickname,
		TargetAvatarKey: c.TargetAvatar,
		LastMessage:     lastMsg,
		LastMessageType: toProtoMessageType(c.LastMessageType),
		LastMessageTime: lastMsgTime,
		UnreadCount:     strconv.FormatInt(c.UnreadCount, 10),
		StoreName:       c.StoreName,
		LogoKey:         c.LogoKey,
		StoreCode:       c.StoreCode,
	}
}

// parseAccountTypeString converts a string account_type from proto to int32.
func parseAccountTypeString(s string) int32 {
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int32(v)
}

func toProtoMessageType(t int32) pb.MessageType {
	if _, ok := pb.MessageType_name[t]; ok {
		return pb.MessageType(t)
	}
	return pb.MessageType_MESSAGE_TYPE_UNSPECIFIED
}
