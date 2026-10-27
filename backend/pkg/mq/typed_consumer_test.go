package mq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.uber.org/zap"
)

type typedConsumerTestEvent struct {
	BaseEvent
	Value int64 `json:"value"`
}

func TestBindTypedHandler_UnmarshalSuccess(t *testing.T) {
	var got *typedConsumerTestEvent
	var gotRoutingKey string
	h := BindTypedHandler(zap.NewNop(), func(_ context.Context, routingKey string, event *typedConsumerTestEvent) error {
		got = event
		gotRoutingKey = routingKey
		return nil
	})

	body, err := json.Marshal(typedConsumerTestEvent{
		BaseEvent: NewBaseEvent(EventTypeAccountDeleted, "test"),
		Value:     42,
	})
	if err != nil {
		t.Fatalf("marshal test event: %v", err)
	}

	if err := h(context.Background(), "account.deleted", body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("fn was not called")
	}
	if got.Value != 42 {
		t.Errorf("Value = %d, want 42", got.Value)
	}
	if got.EventType != EventTypeAccountDeleted {
		t.Errorf("EventType = %q, want %q", got.EventType, EventTypeAccountDeleted)
	}
	if gotRoutingKey != "account.deleted" {
		t.Errorf("routingKey = %q, want account.deleted", gotRoutingKey)
	}
}

func TestBindTypedHandler_UnmarshalError(t *testing.T) {
	called := false
	h := BindTypedHandler(zap.NewNop(), func(context.Context, string, *typedConsumerTestEvent) error {
		called = true
		return nil
	})

	if err := h(context.Background(), "account.deleted", []byte("not json")); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if called {
		t.Error("fn must not be called when unmarshal fails")
	}
}

func TestBindTypedHandler_FnErrorPassthrough(t *testing.T) {
	wantErr := errors.New("db connection refused")
	h := BindTypedHandler(zap.NewNop(), func(context.Context, string, *typedConsumerTestEvent) error {
		return wantErr
	})

	body, _ := json.Marshal(typedConsumerTestEvent{Value: 1})
	if err := h(context.Background(), "account.deleted", body); !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped/passthrough fn error, got: %v", err)
	}
}
