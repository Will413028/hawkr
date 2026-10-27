package grpc

import (
	"context"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/category"
	googlegrpc "google.golang.org/grpc"
)

type categoryTreeCaptureClient struct {
	pb.CategoryServiceClient
	request *pb.GetCategoryTreeRequest
}

func (f *categoryTreeCaptureClient) GetCategoryTree(_ context.Context, in *pb.GetCategoryTreeRequest, _ ...googlegrpc.CallOption) (*pb.GetCategoryTreeResponse, error) {
	f.request = in
	return &pb.GetCategoryTreeResponse{Code: 0}, nil
}

func TestCategoryClient_GetCategoryTreeCarriesOnlyActiveFilterInputs(t *testing.T) {
	fake := &categoryTreeCaptureClient{}
	client := &CategoryClient{client: fake}

	if _, err := client.GetCategoryTree(context.Background(), "active", true); err != nil {
		t.Fatalf("GetCategoryTree returned error: %v", err)
	}
	if fake.request == nil {
		t.Fatal("GetCategoryTree request was not sent")
	}
	if fake.request.GetStatus() != "active" || !fake.request.GetIncludeDeleted() {
		t.Fatalf("request = %+v, want status=active include_deleted=true", fake.request)
	}
}
