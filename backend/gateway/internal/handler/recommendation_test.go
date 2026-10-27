package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// setupRecommendationRouter creates a Gin engine with the public recommendation route (no auth).
func setupRecommendationRouter(h *Holder) *gin.Engine {
	r := gin.New()
	public := r.Group("/api/v1/public")
	public.GET("/recommendations", h.GetRecommendations)
	return r
}

func TestGetRecommendations_Success(t *testing.T) {
	mock := &mockRecommendationServiceClient{
		PersonalizedRecommendFn: func(ctx context.Context, in *aipb.PersonalizedRequest, opts ...grpc.CallOption) (*aipb.PersonalizedRecommendResponse, error) {
			// Public API passes userID=0 (anonymous)
			if in.UserId != 0 {
				t.Errorf("UserId = %d, want 0 for public endpoint", in.UserId)
			}
			return &aipb.PersonalizedRecommendResponse{
				Code:    0,
				Message: "success",
				Data: &aipb.PersonalizedRecommendResult{
					Sections: []*aipb.PersonalizedSection{
						{
							Title: "精選推薦",
							Type:  "featured",
							Products: []*aipb.RecommendedProduct{
								{Id: "1", Name: "Test Product", Score: 0.85, PriceMin: 100, PriceMax: 200},
							},
						},
					},
					ModelVersion: "seller-revenue-v1",
				},
			}, nil
		},
	}
	h := &Holder{AI: grpcclient.NewRecommendationClientForTest(mock)}

	router := setupRecommendationRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/recommendations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", body["data"])
	}
	sections, ok := data["sections"].([]interface{})
	if !ok {
		t.Fatalf("expected sections array, got %T", data["sections"])
	}
	if len(sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(sections))
	}

	section := sections[0].(map[string]interface{})
	if section["title"] != "精選推薦" {
		t.Errorf("section title = %q, want %q", section["title"], "精選推薦")
	}

	// Verify no double envelope (data should NOT contain "code")
	if _, hasCode := data["code"]; hasCode {
		t.Error("response has double envelope: data contains 'code' field")
	}
}

func TestGetRecommendations_NoAuthRequired(t *testing.T) {
	// Verify that calling the endpoint without any token still returns 200
	mock := &mockRecommendationServiceClient{
		PersonalizedRecommendFn: func(ctx context.Context, in *aipb.PersonalizedRequest, opts ...grpc.CallOption) (*aipb.PersonalizedRecommendResponse, error) {
			return &aipb.PersonalizedRecommendResponse{
				Code: 0,
				Data: &aipb.PersonalizedRecommendResult{},
			}, nil
		},
	}
	h := &Holder{AI: grpcclient.NewRecommendationClientForTest(mock)}

	router := setupRecommendationRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/recommendations", nil)
	// Intentionally no Authorization header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

func TestGetRecommendations_AIServiceDown(t *testing.T) {
	mock := &mockRecommendationServiceClient{
		PersonalizedRecommendFn: func(ctx context.Context, in *aipb.PersonalizedRequest, opts ...grpc.CallOption) (*aipb.PersonalizedRecommendResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{AI: grpcclient.NewRecommendationClientForTest(mock)}

	router := setupRecommendationRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/recommendations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 503)
}

func TestGetRecommendations_AIServiceError(t *testing.T) {
	mock := &mockRecommendationServiceClient{
		PersonalizedRecommendFn: func(ctx context.Context, in *aipb.PersonalizedRequest, opts ...grpc.CallOption) (*aipb.PersonalizedRecommendResponse, error) {
			return nil, grpcstatus.Error(codes.Internal, "internal error")
		},
	}
	h := &Holder{AI: grpcclient.NewRecommendationClientForTest(mock)}

	router := setupRecommendationRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/recommendations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusInternalServerError)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 500)
}
