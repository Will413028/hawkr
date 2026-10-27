package pricebook

import (
	"context"
	"errors"
	"testing"
	"time"

	pricebookpb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type mappingServiceFake struct {
	pricebookpb.PricebookServiceClient
	response *pricebookpb.ResolveCategoryMappingResponse
	err      error
	request  *pricebookpb.ResolveCategoryMappingRequest
	deadline time.Time
}

func (f *mappingServiceFake) ResolveCategoryMapping(ctx context.Context, request *pricebookpb.ResolveCategoryMappingRequest, _ ...grpc.CallOption) (*pricebookpb.ResolveCategoryMappingResponse, error) {
	f.request = request
	f.deadline, _ = ctx.Deadline()
	return f.response, f.err
}
func TestResolveCategoryMapping_MapsAuthoritativeWireFieldsAndTimeout(t *testing.T) {
	fake := &mappingServiceFake{response: &pricebookpb.ResolveCategoryMappingResponse{CategoryId: "301", Status: "resolved", GroupCode: strPtr("CURATED-GROUP"), Label: strPtr("策展分類行情"), MappingVersion: "pricebook-category-v1"}}
	client := &pricebookClient{client: fake}
	start := time.Now()
	got, err := client.ResolveCategoryMapping(context.Background(), "301")
	require.NoError(t, err)
	require.Equal(t, "301", fake.request.CategoryId)
	require.Equal(t, "301", got.CategoryID)
	require.Equal(t, "resolved", got.Status)
	require.Equal(t, "CURATED-GROUP", *got.GroupCode)
	require.Equal(t, "策展分類行情", *got.Label)
	require.Equal(t, "pricebook-category-v1", got.MappingVersion)
	require.WithinDuration(t, start.Add(5*time.Second), fake.deadline, time.Second)
}
func TestResolveCategoryMapping_PreservesUnmappedAndAmbiguousNulls(t *testing.T) {
	for _, status := range []string{"unmapped", "ambiguous"} {
		fake := &mappingServiceFake{response: &pricebookpb.ResolveCategoryMappingResponse{CategoryId: "301", Status: status, MappingVersion: "pricebook-category-v1"}}
		got, err := (&pricebookClient{client: fake}).ResolveCategoryMapping(context.Background(), "301")
		require.NoError(t, err)
		require.Equal(t, status, got.Status)
		require.Nil(t, got.GroupCode)
		require.Nil(t, got.Label)
	}
}
func TestResolveCategoryMapping_RPCErrorPassesThrough(t *testing.T) {
	want := errors.New("upstream unavailable")
	fake := &mappingServiceFake{err: want}
	_, err := (&pricebookClient{client: fake}).ResolveCategoryMapping(context.Background(), "301")
	require.ErrorIs(t, err, want)
}
