package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestDecodePublicChatFrame_PreservesLargeStringIDs(t *testing.T) {
	request, err := decodePublicChatFrame([]byte(`{"type":"sync","conversations":{"b1_s2":"7459123456789012345"},"to":"7459123456789012345","peerRole":2}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	frame := request.GetFrame()
	if frame == nil || frame.GetConversations()["b1_s2"] != "7459123456789012345" {
		t.Fatalf("cursor frame = %+v, want exact string cursor", frame)
	}
	if frame.GetToUserId() != "7459123456789012345" || frame.GetPeerRole() != 2 {
		t.Fatalf("target frame = %+v, want exact string target/role", frame)
	}
}

func TestDecodePublicChatFrame_RejectsInvalidNumericFields(t *testing.T) {
	for _, raw := range []string{
		`{"type":"typing","to":"not-a-number"}`,
		`{"type":"sync","conversations":{"room":"not-a-number"}}`,
	} {
		if _, err := decodePublicChatFrame([]byte(raw)); err == nil {
			t.Fatalf("decode(%s) unexpectedly accepted invalid numeric field", raw)
		}
	}
}

func TestChatStreamResponseJSON_PreservesPublicOptionalFields(t *testing.T) {
	online := false
	unread := "0"
	raw, err := publicChatJSONFromStreamResponse(&pb.ChatStreamResponse{Frame: &pb.ChatStreamServerFrame{
		Type:        "presence",
		FromUserId:  "7459123456789012345",
		ToUserId:    "42",
		MessageId:   "7459123456789012345",
		SeqId:       "7",
		Timestamp:   "1800000000000",
		LastSeenAt:  "1799999999000",
		PeerRole:    "2",
		Online:      &online,
		UnreadCount: &unread,
	}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode public JSON: %v", err)
	}
	for key, want := range map[string]string{
		"from":        `"7459123456789012345"`,
		"to":          `"42"`,
		"messageId":   `"7459123456789012345"`,
		"seqId":       `"7"`,
		"online":      `false`,
		"unreadCount": `0`,
	} {
		if string(got[key]) != want {
			t.Fatalf("%s = %s, want %s (full=%s)", key, got[key], want, raw)
		}
	}
}

func TestChatStreamOpenRequest_UsesResolvedWireIdentity(t *testing.T) {
	request := newChatStreamOpenRequest(7459123456789012345, 2)
	open := request.GetOpen()
	if open == nil || open.GetUserId() != "7459123456789012345" || open.GetLoginRole() != 2 {
		t.Fatalf("open = %+v, want resolved string identity and seller role", open)
	}
}

type gatewayChatStreamFakeIM struct {
	pb.UnimplementedIMServiceServer
	opened chan *pb.ChatStreamOpen
	frames chan *pb.ChatStreamClientFrame
}

func (f *gatewayChatStreamFakeIM) ChatStream(stream pb.IMService_ChatStreamServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	if open := first.GetOpen(); open != nil {
		f.opened <- open
	} else {
		return status.Error(codes.InvalidArgument, "open required")
	}
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		if frame := req.GetFrame(); frame != nil {
			f.frames <- frame
			if frame.GetType() == "ping" {
				if err := stream.Send(&pb.ChatStreamResponse{Frame: &pb.ChatStreamServerFrame{
					Type:      "pong",
					Timestamp: "1800000000000",
				}}); err != nil {
					return err
				}
			}
		}
	}
}

func TestChatWebSocket_UsesGRPCPathAndPreservesPublicContractAndResolvedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := grpc.NewServer()
	fakeIM := &gatewayChatStreamFakeIM{opened: make(chan *pb.ChatStreamOpen, 1), frames: make(chan *pb.ChatStreamClientFrame, 1)}
	pb.RegisterIMServiceServer(grpcSrv, fakeIM)
	go func() { _ = grpcSrv.Serve(lis) }()
	defer grpcSrv.Stop()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	h := &Holder{
		IM:      grpcclient.NewIMClientForTest(pb.NewIMServiceClient(conn)),
		BuyerID: buyerResolverForTest(87001),
	}
	r := gin.New()
	r.GET("/ws", fakeJWTAuth(), h.ChatWebSocket)
	gw := httptest.NewServer(r)
	defer gw.Close()

	header := http.Header{"X-Account-Id": []string{"87001"}, "X-Account-Type": []string{"1"}}
	wsURL := "ws" + strings.TrimPrefix(gw.URL, "http") + "/ws"
	client, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("public WebSocket dial: %v", err)
	}
	defer client.Close()

	select {
	case open := <-fakeIM.opened:
		if open.GetUserId() != "87001" || open.GetLoginRole() != 1 {
			t.Fatalf("stream open = %+v, want resolved buyer identity", open)
		}
	case <-time.After(time.Second):
		t.Fatal("Gateway did not send stream open")
	}

	if err := client.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatalf("public ping: %v", err)
	}
	select {
	case frame := <-fakeIM.frames:
		if frame.GetType() != "ping" {
			t.Fatalf("IM frame = %+v, want ping", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("Gateway did not forward public ping")
	}
	_, raw, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("public pong: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("public pong JSON: %v", err)
	}
	if string(got["type"]) != `"pong"` {
		t.Fatalf("public response type = %s, want pong", got["type"])
	}
}
