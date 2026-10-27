package grpc

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

// newStyleServerForWhitelistTest 建最小 AIGrpcServer；只需要 styleUseCase 能產 ID，
// 白名單校驗在 usecase/DB 之前發生，故不需要真的 DB / mediaClient。
func newStyleServerForWhitelistTest() *AIGrpcServer {
	return &AIGrpcServer{styleUseCase: usecase.NewStyleUseCase(nil)}
}

// §8.1：前端直送的 thumbnail key 為完整 URL 時，CreateAiStyle 必須以 400 拒絕，不得回存。
func TestCreateAiStyle_RejectNonWhitelistedThumbnail(t *testing.T) {
	s := newStyleServerForWhitelistTest()
	cases := []struct {
		name  string
		input string
	}{
		{"完整 URL 拒絕", "https://evil.example.com/x.jpg"},
		{"路徑穿越拒絕", "public/../secret/x.jpg"},
		{"前綴不符白名單拒絕", "tmp/staging/x.jpg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := s.CreateAiStyle(context.Background(), &pb.CreateAiStyleRequest{
				Name:         "n",
				Label:        "l",
				Prompt:       "p",
				ThumbnailKey: tc.input, // proto 欄名沿用，值語意為 objectKey
			})
			if err != nil {
				t.Fatalf("unexpected transport error: %v", err)
			}
			if resp.Code != 400 {
				t.Fatalf("expected reject code 400, got %d (%s)", resp.Code, resp.Message)
			}
		})
	}
}

// UpdateAiStyle 走 HasThumbnailKey 直送分支時，同樣必須白名單拒絕。
func TestUpdateAiStyle_RejectNonWhitelistedThumbnail(t *testing.T) {
	s := newStyleServerForWhitelistTest()
	resp, err := s.UpdateAiStyle(context.Background(), &pb.UpdateAiStyleRequest{
		Id:              "123",
		ThumbnailKey:    "https://evil.example.com/x.jpg",
		HasThumbnailKey: true,
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected reject code 400, got %d (%s)", resp.Code, resp.Message)
	}
}
