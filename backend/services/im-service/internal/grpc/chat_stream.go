package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const chatStreamSendBuffer = 256

// ChatStream serves the internal Gateway-to-IM receive-only chat transport.
// The first frame authenticates the resolved chat identity; all subsequent
// operations go through the same transport-neutral processor as legacy WS.
func (s *IMGrpcServer) ChatStream(stream pb.IMService_ChatStreamServer) error {
	first, err := stream.Recv()
	if err != nil {
		return chatStreamError(err)
	}
	open := first.GetOpen()
	if open == nil {
		return status.Error(codes.InvalidArgument, "first chat stream message must be open")
	}
	userID, loginRole, err := validateChatStreamOpen(open)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	ctx, cancel := context.WithCancel(stream.Context())
	conn := newStreamConn(cancel)
	client := &domain.Client{
		Conn:               conn,
		Send:               make(chan []byte, chatStreamSendBuffer),
		SubscribedPresence: make(map[domain.PresenceKey]struct{}),
		UserID:             userID,
		LoginRole:          loginRole,
	}
	s.hub.Register(client)
	defer func() {
		cancel()
		_ = conn.Close()
		s.hub.Unregister(client)
	}()

	errCh := make(chan error, 2)
	go func() {
		errCh <- receiveChatStream(ctx, stream, client, s.hub, s.msgUC)
	}()
	go func() {
		errCh <- sendChatStream(ctx, stream, client)
	}()

	return chatStreamError(<-errCh)
}

func receiveChatStream(ctx context.Context, stream pb.IMService_ChatStreamServer, client *domain.Client, h domain.Hub, msgUC *usecase.MessageUseCase) error {
	processor := usecase.NewClientFrameProcessor()
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		if req.GetOpen() != nil {
			return status.Error(codes.InvalidArgument, "chat stream open must be sent exactly once")
		}
		frame := req.GetFrame()
		if frame == nil {
			return status.Error(codes.InvalidArgument, "chat stream frame is required")
		}
		clientFrame, err := chatStreamClientFrameToUsecase(frame)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		clientFrame.Message.From = client.UserID
		clientFrame.Message.Timestamp = time.Now().UnixMilli()
		processor.Process(ctx, client, h, msgUC, clientFrame)
	}
}

func sendChatStream(ctx context.Context, stream pb.IMService_ChatStreamServer, client *domain.Client) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case data, ok := <-client.Send:
			if !ok {
				return nil
			}
			var msg domain.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				// Hub broadcasts are JSON domain frames. An invalid frame must not
				// terminate every connection; it is equivalent to a malformed
				// legacy WS server frame and is dropped locally.
				continue
			}
			frame := chatStreamServerFrameFromMessage(&msg)
			if frame == nil {
				continue
			}
			if err := stream.Send(&pb.ChatStreamResponse{Frame: frame}); err != nil {
				return err
			}
		}
	}
}

func chatStreamError(err error) error {
	if err == nil || errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
		return nil
	}
	return err
}

func validateChatStreamOpen(open *pb.ChatStreamOpen) (int64, int32, error) {
	if open == nil {
		return 0, 0, errors.New("chat stream open is required")
	}
	userID, err := strconv.ParseInt(open.GetUserId(), 10, 64)
	if err != nil || userID <= 0 {
		return 0, 0, errors.New("chat stream open user_id must be a positive integer")
	}
	if role := open.GetLoginRole(); role != 0 && role != 1 && role != 2 {
		return 0, 0, errors.New("chat stream open login_role must be 0, 1, or 2")
	} else {
		return userID, role, nil
	}
}

func chatStreamClientFrameToUsecase(frame *pb.ChatStreamClientFrame) (usecase.ClientFrame, error) {
	if frame == nil {
		return usecase.ClientFrame{}, errors.New("chat stream frame is required")
	}
	msg := domain.Message{
		Type:     frame.GetType(),
		RoomID:   frame.GetRoomId(),
		PeerRole: frame.GetPeerRole(),
		Content:  frame.GetContent(),
	}
	for name, value := range map[string]string{
		"to_user_id": frame.GetToUserId(),
		"message_id": frame.GetMessageId(),
		"seq_id":     frame.GetSeqId(),
	} {
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return usecase.ClientFrame{}, fmt.Errorf("invalid %s: %s", name, value)
		}
		switch name {
		case "to_user_id":
			msg.To = parsed
		case "message_id":
			msg.MessageID = parsed
		case "seq_id":
			msg.SeqID = parsed
		}
	}

	cursors := make(map[string]int64, len(frame.GetConversations()))
	for roomID, value := range frame.GetConversations() {
		cursor, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return usecase.ClientFrame{}, fmt.Errorf("invalid conversation cursor for %s: %s", roomID, value)
		}
		cursors[roomID] = cursor
	}
	return usecase.ClientFrame{Message: msg, Conversations: cursors}, nil
}

func chatStreamServerFrameFromMessage(msg *domain.Message) *pb.ChatStreamServerFrame {
	if msg == nil {
		return nil
	}
	frame := &pb.ChatStreamServerFrame{
		Type:         msg.Type,
		RoomId:       msg.RoomID,
		Content:      msg.Content,
		HasMore:      msg.HasMore,
		SenderName:   msg.SenderName,
		SenderAvatar: msg.SenderAvatar,
	}
	frame.Timestamp = strconv.FormatInt(msg.Timestamp, 10)
	if msg.LastSeenAt != 0 {
		frame.LastSeenAt = strconv.FormatInt(msg.LastSeenAt, 10)
	}
	if msg.MsgType != 0 {
		frame.MsgType = strconv.FormatInt(int64(msg.MsgType), 10)
	}
	if msg.PeerRole != 0 {
		frame.PeerRole = strconv.FormatInt(int64(msg.PeerRole), 10)
	}
	if msg.Role != 0 {
		frame.Role = strconv.FormatInt(int64(msg.Role), 10)
	}
	if msg.From != 0 {
		frame.FromUserId = strconv.FormatInt(msg.From, 10)
	}
	if msg.To != 0 {
		frame.ToUserId = strconv.FormatInt(msg.To, 10)
	}
	if msg.MessageID != 0 {
		frame.MessageId = strconv.FormatInt(msg.MessageID, 10)
	}
	if msg.SeqID != 0 {
		frame.SeqId = strconv.FormatInt(msg.SeqID, 10)
	}
	if msg.Online != nil {
		online := *msg.Online
		frame.Online = &online
	}
	if msg.UnreadCount != nil {
		unread := strconv.FormatInt(*msg.UnreadCount, 10)
		frame.UnreadCount = &unread
	}
	return frame
}

// streamConn lets the existing Hub lifecycle and DisconnectUser path close a
// gRPC-backed client without teaching the domain package about transports.
// sendChatStream owns the actual gRPC writes.
type streamConn struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func newStreamConn(cancel context.CancelFunc) *streamConn {
	return &streamConn{cancel: cancel, done: make(chan struct{})}
}

func (c *streamConn) Close() error {
	c.once.Do(func() {
		close(c.done)
		if c.cancel != nil {
			c.cancel()
		}
	})
	return nil
}
