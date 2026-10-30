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
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
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

// startChatWS 起 fake IM + 閘道，回傳 client 與 IM 收到的 frame channel。
func startChatWS(t *testing.T, seller *mockSellerServiceClient, accountType string, extra func(*Holder)) (*websocket.Conn, *gatewayChatStreamFakeIM) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := grpc.NewServer()
	fakeIM := &gatewayChatStreamFakeIM{opened: make(chan *pb.ChatStreamOpen, 1), frames: make(chan *pb.ChatStreamClientFrame, 8)}
	pb.RegisterIMServiceServer(grpcSrv, fakeIM)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	h := &Holder{
		IM:      grpcclient.NewIMClientForTest(pb.NewIMServiceClient(conn)),
		BuyerID: buyerResolverForTest(87001),
		Store:   storeResolverForTest(87002),
	}
	if seller != nil {
		h.Seller = grpcclient.NewSellerClientForTest(seller)
	}
	if extra != nil {
		extra(h)
	}
	r := gin.New()
	r.GET("/ws", fakeJWTAuth(), h.ChatWebSocket)
	gw := httptest.NewServer(r)
	t.Cleanup(gw.Close)
	header := http.Header{"X-Account-Id": []string{"87001"}, "X-Account-Type": []string{accountType}}
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gw.URL, "http")+"/ws", header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	select {
	case <-fakeIM.opened:
	case <-time.After(time.Second):
		t.Fatal("no stream open")
	}
	return client, fakeIM
}

func sellerMocksForWS(meta func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error)) *mockSellerServiceClient {
	return &mockSellerServiceClient{
		GetSellerFn: func(context.Context, *sellerpb.GetSellerRequest, ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Data: &sellerpb.SellerInfo{StoreId: "100"}}, nil
		},
		BatchGetSellerMetaFn: meta,
	}
}

// 買家 frame 的 to 必須通過與 REST 相同的 has_public_card 檢查；被拒的 frame
// 回 error frame、不轉給 IM，連線保持開著（之後的 ping 照常轉發）。
func TestChatWebSocket_BuyerFrameTargetGate_RejectsAndDoesNotForward(t *testing.T) {
	cases := map[string]func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error){
		"never public": func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			return &sellerpb.BatchGetSellerMetaResponse{Sellers: map[int64]*sellerpb.SellerMeta{100: {HasPublicCard: false}}}, nil
		},
		"meta RPC error": func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			return nil, status.Error(codes.Unavailable, "down")
		},
		"meta code != 0": func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			return &sellerpb.BatchGetSellerMetaResponse{Code: 5000}, nil
		},
		"meta entry missing": func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			return &sellerpb.BatchGetSellerMetaResponse{Sellers: map[int64]*sellerpb.SellerMeta{}}, nil
		},
	}
	for name, meta := range cases {
		t.Run(name, func(t *testing.T) {
			client, fakeIM := startChatWS(t, sellerMocksForWS(meta), "1", nil)
			if err := client.WriteMessage(websocket.TextMessage, []byte(`{"type":"message","to":"100","content":"hi"}`)); err != nil {
				t.Fatal(err)
			}
			_ = client.SetReadDeadline(time.Now().Add(time.Second))
			_, raw, err := client.ReadMessage()
			if err != nil {
				t.Fatalf("expected error frame: %v", err)
			}
			var got map[string]any
			_ = json.Unmarshal(raw, &got)
			if got["type"] != "error" {
				t.Fatalf("frame = %s, want type=error", raw)
			}
			// 連線仍可用，且 IM 收到的第一個 frame 是 ping，不是被拒的 message。
			if err := client.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
				t.Fatal(err)
			}
			select {
			case f := <-fakeIM.frames:
				if f.GetType() != "ping" {
					t.Fatalf("IM received %+v, want only the later ping", f)
				}
			case <-time.After(time.Second):
				t.Fatal("ping not forwarded after rejection")
			}
		})
	}
}

func TestChatWebSocket_BuyerFrameTargetGate_AllowsPublicCardStore(t *testing.T) {
	meta := func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
		return &sellerpb.BatchGetSellerMetaResponse{Sellers: map[int64]*sellerpb.SellerMeta{100: {HasPublicCard: true}}}, nil
	}
	client, fakeIM := startChatWS(t, sellerMocksForWS(meta), "1", nil)
	if err := client.WriteMessage(websocket.TextMessage, []byte(`{"type":"message","to":"100","content":"hi"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case f := <-fakeIM.frames:
		if f.GetToUserId() != "100" {
			t.Fatalf("IM frame = %+v, want to=100", f)
		}
	case <-time.After(time.Second):
		t.Fatal("allowed frame not forwarded")
	}
}

// 賣家 frame 不經店家檢查（Seller client 未設定：被呼叫就 panic）。
func TestChatWebSocket_SellerFrameTargetUnchanged(t *testing.T) {
	client, fakeIM := startChatWS(t, nil, "2", nil)
	if err := client.WriteMessage(websocket.TextMessage, []byte(`{"type":"message","to":"555","content":"hi"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case f := <-fakeIM.frames:
		if f.GetToUserId() != "555" {
			t.Fatalf("IM frame = %+v, want to=555", f)
		}
	case <-time.After(time.Second):
		t.Fatal("seller frame not forwarded")
	}
}
