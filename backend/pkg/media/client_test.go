package media_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/media"
	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
	"google.golang.org/grpc"
)

// stubMediaServiceClient 是 pb.MediaServiceClient 的 hand-rolled mock。
type stubMediaServiceClient struct {
	finalizeImagesFn    func(ctx context.Context, req *pb.FinalizeImagesRequest) (*pb.FinalizeImagesResponse, error)
	linkImagesCalled    bool
	linkImagesCallCount int
	linkImagesFn        func(ctx context.Context, req *pb.LinkImagesRequest) (*pb.LinkImagesResponse, error)
}

// BatchGetAssetMetadata（#3031）：本 stub **逐支明列**整個介面而不是嵌入
// pb.MediaServiceClient，所以每加一支 RPC 都會在這裡編譯錯。那是刻意的——
// 嵌入版本會讓漏實作變成執行期 nil-deref，而不是編譯錯。
func (s *stubMediaServiceClient) BatchGetAssetMetadata(
	_ context.Context, _ *pb.BatchGetAssetMetadataRequest, _ ...grpc.CallOption,
) (*pb.BatchGetAssetMetadataResponse, error) {
	return &pb.BatchGetAssetMetadataResponse{}, nil
}

func (s *stubMediaServiceClient) FinalizeImages(ctx context.Context, req *pb.FinalizeImagesRequest, _ ...grpc.CallOption) (*pb.FinalizeImagesResponse, error) {
	return s.finalizeImagesFn(ctx, req)
}

func (s *stubMediaServiceClient) LinkImages(ctx context.Context, req *pb.LinkImagesRequest, _ ...grpc.CallOption) (*pb.LinkImagesResponse, error) {
	s.linkImagesCalled = true
	s.linkImagesCallCount++
	if s.linkImagesFn != nil {
		return s.linkImagesFn(ctx, req)
	}
	return &pb.LinkImagesResponse{Code: 0}, nil
}

// 以下為 MediaServiceClient interface 其他方法，測試不需要實作。
func (s *stubMediaServiceClient) CreateUploadSession(_ context.Context, _ *pb.CreateUploadSessionRequest, _ ...grpc.CallOption) (*pb.CreateUploadSessionResponse, error) {
	return nil, nil
}
func (s *stubMediaServiceClient) CompleteUploadSession(_ context.Context, _ *pb.CompleteUploadSessionRequest, _ ...grpc.CallOption) (*pb.CompleteUploadSessionResponse, error) {
	return nil, nil
}
func (s *stubMediaServiceClient) UnlinkImages(_ context.Context, _ *pb.UnlinkImagesRequest, _ ...grpc.CallOption) (*pb.UnlinkImagesResponse, error) {
	return nil, nil
}
func (s *stubMediaServiceClient) ServerUploadAndFinalize(_ context.Context, _ *pb.ServerUploadAndFinalizeRequest, _ ...grpc.CallOption) (*pb.ServerUploadAndFinalizeResponse, error) {
	return nil, nil
}

// 機敏檔後端直傳（58 §8.1.5，2026-08-31 新增）。本 stub 不驗它——FinalizeAndLink
// 不走這條路徑，這裡只是讓型別滿足介面。真正的行為驗證在 invoice-service 那側。
func (s *stubMediaServiceClient) ServerUploadSecureAndFinalize(_ context.Context, _ *pb.ServerUploadSecureAndFinalizeRequest, _ ...grpc.CallOption) (*pb.ServerUploadSecureAndFinalizeResponse, error) {
	return nil, nil
}

// 機敏檔後端讀回（58 §6.1.4）。本 stub 不用它，回 nil 即可——它與本檔測的
// FinalizeAndLink 沒有交集，補在這裡只是為了滿足 pb.MediaServiceClient 介面。
func (s *stubMediaServiceClient) ServerReadSecureAsset(_ context.Context, _ *pb.ServerReadSecureAssetRequest, _ ...grpc.CallOption) (*pb.ServerReadSecureAssetResponse, error) {
	return nil, nil
}

// import_asset 入口（#40 PR2 新增）
func (s *stubMediaServiceClient) ImportDownloadAsset(_ context.Context, _ *pb.ImportDownloadAssetRequest, _ ...grpc.CallOption) (*pb.ImportDownloadAssetResponse, error) {
	return nil, nil
}

func (s *stubMediaServiceClient) FinalizeImportAsset(_ context.Context, _ *pb.FinalizeImportAssetRequest, _ ...grpc.CallOption) (*pb.FinalizeImportAssetResponse, error) {
	return nil, nil
}

func (s *stubMediaServiceClient) BindStagedAsset(_ context.Context, _ *pb.BindStagedAssetRequest, _ ...grpc.CallOption) (*pb.BindStagedAssetResponse, error) {
	return nil, nil
}

func (s *stubMediaServiceClient) ReleaseReferences(_ context.Context, _ *pb.ReleaseReferencesRequest, _ ...grpc.CallOption) (*pb.ReleaseReferencesResponse, error) {
	return nil, nil
}

func TestFinalizeAndLink_Success(t *testing.T) {
	stub := &stubMediaServiceClient{
		finalizeImagesFn: func(_ context.Context, req *pb.FinalizeImagesRequest) (*pb.FinalizeImagesResponse, error) {
			if len(req.StagedTokens) != 1 || req.StagedTokens[0] != "tok-abc" {
				t.Errorf("StagedTokens = %v, want [tok-abc]", req.StagedTokens)
			}
			if req.RefType != "7108" {
				t.Errorf("RefType = %q, want 7108", req.RefType)
			}
			return &pb.FinalizeImagesResponse{
				Code: 0,
				Assets: []*pb.FinalizedAsset{
					{AssetId: "asset-1", ObjectKey: "posts/img.jpg"},
				},
			}, nil
		},
	}

	info, err := media.FinalizeAndLink(context.Background(), stub, "tok-abc", "7108", "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetId != "asset-1" {
		t.Errorf("AssetId = %q, want asset-1", info.AssetId)
	}
	if info.ObjectKey != "posts/img.jpg" {
		t.Errorf("ObjectKey = %q, want posts/img.jpg", info.ObjectKey)
	}
	if !stub.linkImagesCalled {
		t.Error("LinkImages should be called")
	}
}

func TestFinalizeAndLink_FinalizeError(t *testing.T) {
	stub := &stubMediaServiceClient{
		finalizeImagesFn: func(_ context.Context, _ *pb.FinalizeImagesRequest) (*pb.FinalizeImagesResponse, error) {
			return nil, errors.New("network error")
		},
	}

	_, err := media.FinalizeAndLink(context.Background(), stub, "tok-bad", "7108", "99")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if stub.linkImagesCalled {
		t.Error("LinkImages should not be called when FinalizeImages fails")
	}
}

func TestFinalizeAndLink_FinalizeNonZeroCode(t *testing.T) {
	stub := &stubMediaServiceClient{
		finalizeImagesFn: func(_ context.Context, _ *pb.FinalizeImagesRequest) (*pb.FinalizeImagesResponse, error) {
			return &pb.FinalizeImagesResponse{Code: 400, Message: "invalid token"}, nil
		},
	}

	_, err := media.FinalizeAndLink(context.Background(), stub, "tok-bad", "7108", "99")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if stub.linkImagesCalled {
		t.Error("LinkImages should not be called on non-zero code")
	}
}

func TestFinalizeAndLink_EmptyAssets(t *testing.T) {
	stub := &stubMediaServiceClient{
		finalizeImagesFn: func(_ context.Context, _ *pb.FinalizeImagesRequest) (*pb.FinalizeImagesResponse, error) {
			return &pb.FinalizeImagesResponse{Code: 0, Assets: nil}, nil
		},
	}

	_, err := media.FinalizeAndLink(context.Background(), stub, "tok-empty", "7108", "99")
	if err == nil {
		t.Fatal("expected error when assets empty, got nil")
	}
	if stub.linkImagesCalled {
		t.Error("LinkImages should not be called when no assets returned")
	}
}

// link 失敗必須回報 caller。所有呼叫端都在寫入自己的表之前呼叫 FinalizeAndLink 並在
// 出錯時中止，因此回 error 的結果是「沒有任何人引用這個 objectKey」，剛好符合
// media GC 對孤兒資產的處理（09 §8.4）。反之若吞掉錯誤，caller 會把 key 寫進 DB，
// 形成「有人在用但沒有 asset_reference」，7 天後 GC 刪掉的是正在顯示的圖。
func TestFinalizeAndLink_LinkTransportError_Propagated(t *testing.T) {
	stub := &stubMediaServiceClient{
		finalizeImagesFn: func(_ context.Context, _ *pb.FinalizeImagesRequest) (*pb.FinalizeImagesResponse, error) {
			return &pb.FinalizeImagesResponse{
				Code:   0,
				Assets: []*pb.FinalizedAsset{{AssetId: "asset-2", ObjectKey: "chat/img.jpg"}},
			}, nil
		},
		linkImagesFn: func(_ context.Context, _ *pb.LinkImagesRequest) (*pb.LinkImagesResponse, error) {
			return nil, errors.New("link service unavailable")
		},
	}

	_, err := media.FinalizeAndLink(context.Background(), stub, "tok-ok", "7107", "55")
	if err == nil {
		t.Fatal("expected error when LinkImages fails, got nil")
	}
	if stub.linkImagesCallCount != 3 {
		t.Errorf("LinkImages call count = %d, want 3 (retried)", stub.linkImagesCallCount)
	}
}

// 非 0 業務碼與 transport error 同等對待。
func TestFinalizeAndLink_LinkNonZeroCode_Propagated(t *testing.T) {
	stub := &stubMediaServiceClient{
		finalizeImagesFn: func(_ context.Context, _ *pb.FinalizeImagesRequest) (*pb.FinalizeImagesResponse, error) {
			return &pb.FinalizeImagesResponse{
				Code:   0,
				Assets: []*pb.FinalizedAsset{{AssetId: "asset-3", ObjectKey: "chat/img2.jpg"}},
			}, nil
		},
		linkImagesFn: func(_ context.Context, _ *pb.LinkImagesRequest) (*pb.LinkImagesResponse, error) {
			return &pb.LinkImagesResponse{Code: 500, Message: "db down"}, nil
		},
	}

	_, err := media.FinalizeAndLink(context.Background(), stub, "tok-ok", "7107", "55")
	if err == nil {
		t.Fatal("expected error when LinkImages returns non-zero code, got nil")
	}
}

// LinkImages 以 ON CONFLICT DO NOTHING 實作、明定冪等（09 §3.2.4），故重試安全：
// 前兩次失敗、第三次成功時整體視為成功。
func TestFinalizeAndLink_LinkRetriedThenSucceeds(t *testing.T) {
	stub := &stubMediaServiceClient{
		finalizeImagesFn: func(_ context.Context, _ *pb.FinalizeImagesRequest) (*pb.FinalizeImagesResponse, error) {
			return &pb.FinalizeImagesResponse{
				Code:   0,
				Assets: []*pb.FinalizedAsset{{AssetId: "asset-4", ObjectKey: "chat/img3.jpg"}},
			}, nil
		},
	}
	stub.linkImagesFn = func(_ context.Context, _ *pb.LinkImagesRequest) (*pb.LinkImagesResponse, error) {
		if stub.linkImagesCallCount < 3 {
			return nil, errors.New("transient")
		}
		return &pb.LinkImagesResponse{Code: 0}, nil
	}

	info, err := media.FinalizeAndLink(context.Background(), stub, "tok-ok", "7107", "55")
	if err != nil {
		t.Fatalf("expected success on third attempt, got: %v", err)
	}
	if info.ObjectKey != "chat/img3.jpg" {
		t.Errorf("ObjectKey = %q, want chat/img3.jpg", info.ObjectKey)
	}
	if stub.linkImagesCallCount != 3 {
		t.Errorf("LinkImages call count = %d, want 3", stub.linkImagesCallCount)
	}
}
