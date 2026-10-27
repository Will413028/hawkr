package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type overviewClientStub struct {
	pb.PricebookServiceClient
	get    func(*pb.GetMarketOverviewRequest) (*pb.GetMarketOverviewResponse, error)
	create func(*pb.CreateMarketWatchTargetRequest) (*pb.CreateMarketWatchTargetResponse, error)
	delete func(*pb.DeleteMarketWatchTargetRequest) (*pb.DeleteMarketWatchTargetResponse, error)
}

func (s overviewClientStub) GetMarketOverview(_ context.Context, r *pb.GetMarketOverviewRequest, _ ...grpc.CallOption) (*pb.GetMarketOverviewResponse, error) {
	return s.get(r)
}
func (s overviewClientStub) CreateMarketWatchTarget(_ context.Context, r *pb.CreateMarketWatchTargetRequest, _ ...grpc.CallOption) (*pb.CreateMarketWatchTargetResponse, error) {
	return s.create(r)
}
func (s overviewClientStub) DeleteMarketWatchTarget(_ context.Context, r *pb.DeleteMarketWatchTargetRequest, _ ...grpc.CallOption) (*pb.DeleteMarketWatchTargetResponse, error) {
	return s.delete(r)
}

func overviewTestRouter(client overviewClientStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Holder{Pricebook: grpcclient.NewPricebookClientForTest(client)}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("accountId", int64(123)) })
	r.GET("/overview", h.GetMarketOverview)
	r.POST("/watch-targets", h.CreateMarketWatchTarget)
	r.DELETE("/watch-targets/:watch_id", h.DeleteMarketWatchTarget)
	return r
}

func TestMarketOverviewHTTP_PreservesQueryPresenceAndWireTypes(t *testing.T) {
	r := overviewTestRouter(overviewClientStub{get: func(in *pb.GetMarketOverviewRequest) (*pb.GetMarketOverviewResponse, error) {
		require.Equal(t, "watchlist", in.Scope)
		require.Equal(t, "", in.GetWindowDays())
		require.NotNil(t, in.WindowDays)
		require.Equal(t, "100", in.GetPageSize())
		require.Equal(t, "2", in.GetPage())
		require.Nil(t, in.MarketCode)
		require.Equal(t, "高麗菜", in.Query)
		return &pb.GetMarketOverviewResponse{Page: "2", PageSize: "100", TotalItems: "0"}, nil
	}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/overview?scope=watchlist&window_days=&page_size=100&page=2&query=%E9%AB%98%E9%BA%97%E8%8F%9C", nil))
	require.Equal(t, 200, w.Code)
	var out struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "0", out.Data["totalItems"])
	require.Equal(t, false, out.Data["hasNext"])
	require.Equal(t, []any{}, out.Data["items"])
	require.Equal(t, []any{}, out.Data["marketOptions"])
}

func TestMarketOverviewHTTP_OmittedAndInvalidQueries(t *testing.T) {
	calls := 0
	r := overviewTestRouter(overviewClientStub{get: func(in *pb.GetMarketOverviewRequest) (*pb.GetMarketOverviewResponse, error) {
		calls++
		require.Nil(t, in.WindowDays)
		require.Nil(t, in.PageSize)
		require.Nil(t, in.Page)
		return nil, status.Error(codes.InvalidArgument, "invalid overview query")
	}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/overview", nil))
	require.Equal(t, 400, w.Code)
	require.Equal(t, 1, calls)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/overview?page=1&page=2", nil))
	require.Equal(t, 400, w.Code)
	require.Equal(t, 1, calls)
}

func TestMarketWatchHTTP_IdempotentResponseAndIdentity(t *testing.T) {
	r := overviewTestRouter(overviewClientStub{
		create: func(in *pb.CreateMarketWatchTargetRequest) (*pb.CreateMarketWatchTargetResponse, error) {
			require.Equal(t, "A1", in.CropCode)
			require.Nil(t, in.MarketCode)
			return &pb.CreateMarketWatchTargetResponse{WatchTargetId: "9007199254740993", Created: false}, nil
		},
		delete: func(in *pb.DeleteMarketWatchTargetRequest) (*pb.DeleteMarketWatchTargetResponse, error) {
			require.Equal(t, "9007199254740993", in.WatchId)
			return &pb.DeleteMarketWatchTargetResponse{}, nil
		},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/watch-targets", strings.NewReader(`{"cropCode":"A1","accountId":"999"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"watchTargetId":"9007199254740993"`)
	require.Contains(t, w.Body.String(), `"created":false`)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/watch-targets/9007199254740993", nil))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"data":{}`)
}
