package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/go-pkg/serviceauth"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const chatStreamTestKey = "01234567890123456789012345678901"

// newStreamTestClient builds a bufconn client/server pair. A non-empty
// serverKey enables the same method-filtered interceptor used by the real IM
// fx graph; clientKey independently controls whether the Gateway side sends a
// credential, allowing the negative auth path to exercise a real gRPC call.
func newStreamTestClient(t *testing.T, hub domain.Hub, msgUC *usecase.MessageUseCase, serverKey, clientKey string) (pb.IMServiceClient, func()) {
	t.Helper()
	var serverOpts []grpc.ServerOption
	if serverKey != "" {
		serverOpts = pkgserver.BuildServerOptions(pkgserver.ServerConfig{
			EnableStreamInterceptors: true,
			ExtraStreamInterceptors: []grpc.StreamServerInterceptor{
				newChatStreamAuthInterceptor(&config.Config{ChatStreamJWTSecret: serverKey}),
			},
		}, zap.NewNop())
	}
	grpcSrv := grpc.NewServer(serverOpts...)
	pb.RegisterIMServiceServer(grpcSrv, NewIMGrpcServer(hub, msgUC))
	lis := bufconn.Listen(1024 * 1024)
	go func() { _ = grpcSrv.Serve(lis) }()

	dialOpts := []grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if clientKey != "" {
		signer, err := serviceauth.NewSigner(serviceauth.Config{
			CurrentKey: clientKey,
			Issuer:     "gateway",
			Audience:   "im-service",
			Purpose:    "im-chat-stream",
		})
		if err != nil {
			t.Fatalf("NewSigner: %v", err)
		}
		dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(
			signer.StreamClientInterceptor(serviceauth.ChatStreamMethod),
		))
	}
	conn, err := grpc.NewClient("passthrough:///bufnet", dialOpts...)
	if err != nil {
		grpcSrv.Stop()
		_ = lis.Close()
		t.Fatalf("grpc.NewClient: %v", err)
	}
	return pb.NewIMServiceClient(conn), func() {
		_ = conn.Close()
		grpcSrv.Stop()
		_ = lis.Close()
	}
}

func TestGRPC_ChatStream_EnforcesServiceAuth(t *testing.T) {
	newStream := func(t *testing.T, clientKey string) pb.IMServiceClient {
		t.Helper()
		client, cleanup := newStreamTestClient(t, &stubHub{}, nil, chatStreamTestKey, clientKey)
		t.Cleanup(cleanup)
		return client
	}

	t.Run("missing token", func(t *testing.T) {
		client := newStream(t, "")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		stream, err := client.ChatStream(ctx)
		if err != nil {
			t.Fatalf("ChatStream: %v", err)
		}
		err = stream.Send(&pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Open{
			Open: &pb.ChatStreamOpen{UserId: "42", LoginRole: 1},
		}})
		if err == nil {
			_, err = stream.Recv()
		}
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("missing token code = %s, want %s (err=%v)", status.Code(err), codes.Unauthenticated, err)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		client := newStream(t, chatStreamTestKey)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		stream, err := client.ChatStream(ctx)
		if err != nil {
			t.Fatalf("ChatStream: %v", err)
		}
		if err := stream.Send(&pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Open{
			Open: &pb.ChatStreamOpen{UserId: "42", LoginRole: 1},
		}}); err != nil {
			t.Fatalf("send open: %v", err)
		}
		if err := stream.Send(&pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Frame{
			Frame: &pb.ChatStreamClientFrame{Type: "ping"},
		}}); err != nil {
			t.Fatalf("send ping: %v", err)
		}
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv pong: %v", err)
		}
		if resp.GetFrame().GetType() != "pong" {
			t.Fatalf("response type = %q, want pong", resp.GetFrame().GetType())
		}
	})
}

func TestValidateChatStreamOpen_ValidatesWireIdentityAndRole(t *testing.T) {
	for _, tc := range []struct {
		name     string
		open     *pb.ChatStreamOpen
		wantID   int64
		wantRole int32
		wantErr  bool
	}{
		{name: "valid buyer", open: &pb.ChatStreamOpen{UserId: "7459123456789012345", LoginRole: 1}, wantID: 7459123456789012345, wantRole: 1},
		{name: "valid both roles", open: &pb.ChatStreamOpen{UserId: "42", LoginRole: 0}, wantID: 42, wantRole: 0},
		{name: "empty id", open: &pb.ChatStreamOpen{LoginRole: 1}, wantErr: true},
		{name: "bad id", open: &pb.ChatStreamOpen{UserId: "not-a-number", LoginRole: 1}, wantErr: true},
		{name: "bad role", open: &pb.ChatStreamOpen{UserId: "42", LoginRole: 3}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, role, err := validateChatStreamOpen(tc.open)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if id != tc.wantID || role != tc.wantRole {
				t.Fatalf("identity = %d/%d, want %d/%d", id, role, tc.wantID, tc.wantRole)
			}
		})
	}
}

func TestChatStreamClientFrameMapping_PreservesStringIDs(t *testing.T) {
	frame, err := chatStreamClientFrameToUsecase(&pb.ChatStreamClientFrame{
		Type:          "sync",
		Conversations: map[string]string{"b1_s2": "7459123456789012345"},
		ToUserId:      "7459123456789012345",
		PeerRole:      2,
	})
	if err != nil {
		t.Fatalf("mapping: %v", err)
	}
	if got := frame.Conversations["b1_s2"]; got != 7459123456789012345 {
		t.Fatalf("cursor = %d, want exact TSID", got)
	}
	if frame.Message.To != 7459123456789012345 || frame.Message.PeerRole != 2 {
		t.Fatalf("message = %+v, want exact target/role", frame.Message)
	}
}

func TestChatStreamClientFrameMapping_RejectsInvalidIDs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		frame *pb.ChatStreamClientFrame
	}{
		{name: "target", frame: &pb.ChatStreamClientFrame{Type: "typing", ToUserId: "bad"}},
		{name: "cursor", frame: &pb.ChatStreamClientFrame{Type: "sync", Conversations: map[string]string{"room": "bad"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := chatStreamClientFrameToUsecase(tc.frame); err == nil {
				t.Fatal("expected invalid ID error")
			}
		})
	}
}

func TestChatStreamServerFrameMapping_PreservesOptionalZeroValues(t *testing.T) {
	online := false
	unread := int64(0)
	msg := &domain.Message{
		Type: "presence", RoomID: "b1_s2", From: 7459123456789012345,
		To: 42, MessageID: 7459123456789012345, SeqID: 7, Content: "x",
		Timestamp: 1800000000000, LastSeenAt: 1799999999000, MsgType: 1,
		PeerRole: 2, HasMore: true, SenderName: "seller", SenderAvatar: "a",
		Online: &online, UnreadCount: &unread, Role: 2,
	}

	got := chatStreamServerFrameFromMessage(msg)
	if got.GetFromUserId() != "7459123456789012345" || got.GetMessageId() != "7459123456789012345" || got.GetTimestamp() != "1800000000000" || got.GetPeerRole() != "2" {
		t.Fatalf("string fields not preserved: %+v", got)
	}
	if got.Online == nil || got.GetOnline() {
		t.Fatalf("online optional value = %v, want present false", got.Online)
	}
	if got.UnreadCount == nil || got.GetUnreadCount() != "0" {
		t.Fatalf("unread optional value = %v, want present zero", got.UnreadCount)
	}
}

func TestChatStreamServerFrameMapping_NilMessage(t *testing.T) {
	if got := chatStreamServerFrameFromMessage(nil); got != nil {
		t.Fatalf("nil message mapped to %+v, want nil", got)
	}
}

func TestGRPC_ChatStream_PingPong(t *testing.T) {
	client, cleanup := newTestServer(t, &stubMsgRepo{}, &stubMediaClient{})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stream, err := client.ChatStream(ctx)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if err := stream.Send(&pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Open{
		Open: &pb.ChatStreamOpen{UserId: "42", LoginRole: 1},
	}}); err != nil {
		t.Fatalf("send open: %v", err)
	}
	if err := stream.Send(&pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Frame{
		Frame: &pb.ChatStreamClientFrame{Type: "ping"},
	}}); err != nil {
		t.Fatalf("send ping: %v", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv pong: %v", err)
	}
	if resp.GetFrame().GetType() != "pong" {
		t.Fatalf("response type = %q, want pong", resp.GetFrame().GetType())
	}
}

func TestGRPC_ChatStream_RejectsMissingOrRepeatedOpen(t *testing.T) {
	cases := []struct {
		name   string
		frames []*pb.ChatStreamRequest
	}{
		{
			name: "missing open",
			frames: []*pb.ChatStreamRequest{{Payload: &pb.ChatStreamRequest_Frame{
				Frame: &pb.ChatStreamClientFrame{Type: "ping"},
			}}},
		},
		{
			name: "repeated open",
			frames: []*pb.ChatStreamRequest{
				{Payload: &pb.ChatStreamRequest_Open{Open: &pb.ChatStreamOpen{UserId: "42", LoginRole: 1}}},
				{Payload: &pb.ChatStreamRequest_Open{Open: &pb.ChatStreamOpen{UserId: "43", LoginRole: 1}}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, cleanup := newTestServer(t, &stubMsgRepo{}, &stubMediaClient{})
			defer cleanup()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			stream, err := client.ChatStream(ctx)
			if err != nil {
				t.Fatalf("ChatStream: %v", err)
			}
			for _, frame := range tc.frames {
				if err := stream.Send(frame); err != nil {
					t.Fatalf("send frame: %v", err)
				}
			}
			if _, err := stream.Recv(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("Recv code = %s, want %s (err=%v)", status.Code(err), codes.InvalidArgument, err)
			}
		})
	}
}

var _ usecase.ClientFrame
