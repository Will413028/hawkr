package assistant

import (
	"context"
	"errors"
	"iter"
	"testing"

	adkmodel "google.golang.org/adk/v2/model"
)

type stubLLM struct{}

func (stubLLM) Name() string { return "stub" }
func (stubLLM) GenerateContent(context.Context, *adkmodel.LLMRequest, bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(func(*adkmodel.LLMResponse, error) bool) {}
}

type stubOrders struct{}

func (stubOrders) MyOrders(context.Context, int64, string, int, int) ([]BuyerOrder, error) {
	return nil, errors.New("stub")
}

func (stubOrders) OrderDetail(context.Context, int64, string) (BuyerOrderDetail, error) {
	return BuyerOrderDetail{}, errors.New("stub")
}

func TestNewDefaultsNilNowToTheSystemClock(t *testing.T) {
	m := New(Deps{LLM: stubLLM{}, Orders: stubOrders{}})
	if m.turns == nil {
		t.Fatal("New 沒有建出輪次執行器")
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
