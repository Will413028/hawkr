package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/gorilla/websocket"
)

// publicChatFrame is the stable JSON input accepted by the public WebSocket.
// Numeric IDs deliberately use a strict string-or-integer decoder so a client
// cannot round a TSID through float64 on the Gateway boundary.
type publicChatFrame struct {
	Type          string                     `json:"type"`
	Conversations map[string]json.RawMessage `json:"conversations"`
	RoomID        string                     `json:"roomId"`
	To            chatJSONInt64              `json:"to"`
	PeerRole      int32                      `json:"peerRole"`
	MessageID     chatJSONInt64              `json:"messageId"`
	SeqID         chatJSONInt64              `json:"seqId"`
	Content       string                     `json:"content"`
}

type chatJSONInt64 struct {
	value   int64
	present bool
}

func (v *chatJSONInt64) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		if s == "" {
			return errors.New("empty integer string")
		}
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.value, v.present = parsed, true
		return nil
	}
	parsed, err := strconv.ParseInt(string(trimmed), 10, 64)
	if err != nil {
		return err
	}
	v.value, v.present = parsed, true
	return nil
}

func decodePublicChatFrame(data []byte) (*pb.ChatStreamRequest, error) {
	var input publicChatFrame
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}
	frame := &pb.ChatStreamClientFrame{
		Type:     input.Type,
		RoomId:   input.RoomID,
		PeerRole: input.PeerRole,
		Content:  input.Content,
	}
	if input.To.present {
		frame.ToUserId = strconv.FormatInt(input.To.value, 10)
	}
	if input.MessageID.present {
		frame.MessageId = strconv.FormatInt(input.MessageID.value, 10)
	}
	if input.SeqID.present {
		frame.SeqId = strconv.FormatInt(input.SeqID.value, 10)
	}
	if input.Conversations != nil {
		frame.Conversations = make(map[string]string, len(input.Conversations))
		for roomID, rawCursor := range input.Conversations {
			var cursor chatJSONInt64
			if err := json.Unmarshal(rawCursor, &cursor); err != nil {
				return nil, fmt.Errorf("invalid conversation cursor for %s: %w", roomID, err)
			}
			if !cursor.present {
				cursor.value = 0
			}
			frame.Conversations[roomID] = strconv.FormatInt(cursor.value, 10)
		}
	}
	return &pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Frame{Frame: frame}}, nil
}

// publicChatMessage is intentionally shaped like the old domain.Message JSON,
// without importing IM's private domain package into Gateway. Pointer fields
// preserve explicit false/zero values for presence and unread events.
type publicChatMessage struct {
	Online       *bool  `json:"online,omitempty"`
	RoomID       string `json:"roomId,omitempty"`
	Type         string `json:"type"`
	Content      string `json:"content,omitempty"`
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
	MessageID    string `json:"messageId,omitempty"`
	SeqID        string `json:"seqId,omitempty"`
	Timestamp    int64  `json:"timestamp"`
	LastSeenAt   int64  `json:"lastSeenAt,omitempty"`
	MsgType      int32  `json:"msgType,omitempty"`
	PeerRole     int32  `json:"peerRole,omitempty"`
	HasMore      bool   `json:"hasMore,omitempty"`
	SenderName   string `json:"senderName,omitempty"`
	SenderAvatar string `json:"avatarKey,omitempty"`
	UnreadCount  *int64 `json:"unreadCount,omitempty"`
	Role         int32  `json:"role,omitempty"`
}

func publicChatJSONFromStreamResponse(response *pb.ChatStreamResponse) ([]byte, error) {
	if response == nil || response.GetFrame() == nil {
		return nil, errors.New("chat stream response frame is required")
	}
	frame := response.GetFrame()
	timestamp, err := parseChatStreamNumber("timestamp", frame.GetTimestamp(), 64)
	if err != nil {
		return nil, err
	}
	lastSeenAt, err := parseChatStreamNumber("last_seen_at", frame.GetLastSeenAt(), 64)
	if err != nil {
		return nil, err
	}
	msgType, err := parseChatStreamNumber("msg_type", frame.GetMsgType(), 32)
	if err != nil {
		return nil, err
	}
	peerRole, err := parseChatStreamNumber("peer_role", frame.GetPeerRole(), 32)
	if err != nil {
		return nil, err
	}
	role, err := parseChatStreamNumber("role", frame.GetRole(), 32)
	if err != nil {
		return nil, err
	}
	message := publicChatMessage{
		RoomID:       frame.GetRoomId(),
		Type:         frame.GetType(),
		Content:      frame.GetContent(),
		Timestamp:    timestamp,
		LastSeenAt:   lastSeenAt,
		MsgType:      int32(msgType),
		PeerRole:     int32(peerRole),
		HasMore:      frame.GetHasMore(),
		SenderName:   frame.GetSenderName(),
		SenderAvatar: frame.GetSenderAvatar(),
		Role:         int32(role),
	}
	for name, value := range map[string]string{
		"from_user_id": frame.GetFromUserId(),
		"to_user_id":   frame.GetToUserId(),
		"message_id":   frame.GetMessageId(),
		"seq_id":       frame.GetSeqId(),
	} {
		if value == "" {
			continue
		}
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return nil, fmt.Errorf("invalid %s from IM stream", name)
		}
		switch name {
		case "from_user_id":
			message.From = value
		case "to_user_id":
			message.To = value
		case "message_id":
			message.MessageID = value
		case "seq_id":
			message.SeqID = value
		}
	}
	if frame.Online != nil {
		online := frame.GetOnline()
		message.Online = &online
	}
	if frame.UnreadCount != nil {
		unread, err := parseChatStreamNumber("unread_count", frame.GetUnreadCount(), 64)
		if err != nil {
			return nil, err
		}
		message.UnreadCount = &unread
	}
	return json.Marshal(&message)
}

func parseChatStreamNumber(name, value string, bits int) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, bits)
	if err != nil {
		return 0, fmt.Errorf("invalid %s from IM stream", name)
	}
	return parsed, nil
}

func newChatStreamOpenRequest(wireID int64, loginRole int32) *pb.ChatStreamRequest {
	return &pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Open{
		Open: &pb.ChatStreamOpen{
			UserId:    strconv.FormatInt(wireID, 10),
			LoginRole: loginRole,
		},
	}}
}

func (h *Holder) serveChatWebSocketGRPC(ctx context.Context, clientConn *websocket.Conn, wireID int64, loginRole int32) {
	client := &wsProxyConn{Conn: clientConn}
	_ = client.Conn.SetReadDeadline(time.Now().Add(wsProxyPongWait))
	client.Conn.SetPongHandler(func(string) error {
		return client.Conn.SetReadDeadline(time.Now().Add(wsProxyPongWait))
	})

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := h.IM.ChatStream(streamCtx)
	if err != nil {
		_ = client.write(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "failed to connect to IM service"))
		return
	}
	defer func() { _ = stream.CloseSend() }()
	if err := stream.Send(newChatStreamOpenRequest(wireID, loginRole)); err != nil {
		_ = client.write(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "failed to open chat stream"))
		return
	}

	errChan := make(chan error, 2)
	go pipePublicWSToIM(client, stream, errChan)
	go pipeIMToPublicWS(stream, client, errChan)

	ticker := time.NewTicker(wsProxyPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-errChan:
			return
		case <-ticker.C:
			if err := client.write(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func pipePublicWSToIM(src *wsProxyConn, dst pb.IMService_ChatStreamClient, errChan chan<- error) {
	for {
		_, data, err := src.Conn.ReadMessage()
		if err != nil {
			errChan <- err
			return
		}
		_ = src.Conn.SetReadDeadline(time.Now().Add(wsProxyPongWait))
		request, err := decodePublicChatFrame(data)
		if err != nil {
			// Keep malformed public frames from tearing down the public WS, as the
			// previous Gateway edge behavior did.
			continue
		}
		if err := dst.Send(request); err != nil {
			errChan <- err
			return
		}
	}
}

func pipeIMToPublicWS(src pb.IMService_ChatStreamClient, dst *wsProxyConn, errChan chan<- error) {
	for {
		response, err := src.Recv()
		if err != nil {
			errChan <- err
			return
		}
		data, err := publicChatJSONFromStreamResponse(response)
		if err != nil {
			// A malformed internal frame is not a public protocol response; drop
			// it without changing the public WebSocket contract.
			continue
		}
		if err := dst.write(websocket.TextMessage, data); err != nil {
			errChan <- err
			return
		}
	}
}
