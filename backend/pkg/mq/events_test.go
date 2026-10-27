package mq

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPostCreatedEvent_HasSellerAvatar(t *testing.T) {
	// 49 §6.3/S3：payload 帶 objectKey（seller_avatar_key），顯示 URL 由消費端組。
	ev := NewPostCreatedEvent(1, 2, 3, "seller-name", "public/seller_avatars/2.jpg", "preview")
	if ev.SellerAvatarKey != "public/seller_avatars/2.jpg" {
		t.Fatalf("SellerAvatarKey = %q, want objectKey", ev.SellerAvatarKey)
	}
	if ev.SellerName != "seller-name" {
		t.Fatalf("SellerName = %q, want seller-name", ev.SellerName)
	}
}

func TestPostCreatedEvent_JSONShape(t *testing.T) {
	ev := NewPostCreatedEvent(1, 7487768729113814280, 3, "seller-name", "public/seller_avatars/2.jpg", "preview")
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal PostCreatedEvent: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"store_id":7487768729113814280`) {
		t.Errorf("PostCreatedEvent JSON = %s, missing store_id", got)
	}
	if strings.Contains(got, "seller_account_id") {
		t.Errorf("PostCreatedEvent JSON = %s, must not contain legacy key seller_account_id", got)
	}
}

func TestSellerLifecycleEvents_JSONShape(t *testing.T) {
	del := &SellerDeletedEvent{
		BaseEvent: NewBaseEvent(EventTypeSellerDeleted, "seller-service"),
		AccountID: 100,
		StoreID:   999,
	}
	b, err := json.Marshal(del)
	if err != nil {
		t.Fatalf("marshal SellerDeletedEvent: %v", err)
	}
	for _, want := range []string{`"accountId":100`, `"store_id":999`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("SellerDeletedEvent JSON = %s, missing %s", b, want)
		}
	}

	onb := &SellerOnboardedEvent{
		BaseEvent: NewBaseEvent(EventTypeSellerOnboarded, "seller-service"),
		AccountID: 100,
		StoreID:   999,
	}
	b, err = json.Marshal(onb)
	if err != nil {
		t.Fatalf("marshal SellerOnboardedEvent: %v", err)
	}
	for _, want := range []string{`"accountId":100`, `"store_id":999`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("SellerOnboardedEvent JSON = %s, missing %s", b, want)
		}
	}
	if RoutingKeySellerOnboarded != "seller.onboarded" {
		t.Errorf("RoutingKeySellerOnboarded = %q", RoutingKeySellerOnboarded)
	}

	susp := &SellerSuspendedEvent{
		BaseEvent:    NewBaseEvent(EventTypeSellerSuspended, "seller-service"),
		SuspensionID: 7482084104842517848,
		AccountID:    900001,
		ReasonCode:   "9601",
		StoreID:      900001,
	}
	b, err = json.Marshal(susp)
	if err != nil {
		t.Fatalf("marshal SellerSuspendedEvent: %v", err)
	}
	for _, want := range []string{`"suspensionId":"7482084104842517848"`, `"accountId":"900001"`, `"store_id":"900001"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("SellerSuspendedEvent JSON = %s, missing %s", b, want)
		}
	}

	unsusp := &SellerUnsuspendedEvent{
		BaseEvent:    NewBaseEvent(EventTypeSellerUnsuspended, "seller-service"),
		SuspensionID: 7482084104842517848,
		AccountID:    900001,
		StoreID:      900001,
	}
	b, err = json.Marshal(unsusp)
	if err != nil {
		t.Fatalf("marshal SellerUnsuspendedEvent: %v", err)
	}
	for _, want := range []string{`"suspensionId":"7482084104842517848"`, `"accountId":"900001"`, `"store_id":"900001"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("SellerUnsuspendedEvent JSON = %s, missing %s", b, want)
		}
	}
}

func TestBuyerDeletedEvent_JSONShape(t *testing.T) {
	ev := &BuyerDeletedEvent{
		BaseEvent: NewBaseEvent(EventTypeBuyerDeleted, "buyer-service"),
		AccountID: 100,
		BuyerID:   999,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal BuyerDeletedEvent: %v", err)
	}
	for _, want := range []string{`"accountId":100`, `"buyerId":999`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("BuyerDeletedEvent JSON = %s, missing %s", b, want)
		}
	}
	if RoutingKeyBuyerDeleted != "buyer.deleted" {
		t.Errorf("RoutingKeyBuyerDeleted = %q", RoutingKeyBuyerDeleted)
	}
}

func TestCounterDeltaEvent_JSONShape(t *testing.T) {
	ev := NewCounterDeltaEvent("post", 10, "comment_count", 1)
	if ev.EntityType != "post" || ev.EntityID != 10 || ev.Field != "comment_count" || ev.Delta != 1 {
		t.Fatalf("unexpected event fields: %+v", ev)
	}
	if ev.EventType != EventTypeCounterDelta {
		t.Fatalf("EventType = %q, want %q", ev.EventType, EventTypeCounterDelta)
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal CounterDeltaEvent: %v", err)
	}
	for _, want := range []string{`"entityType":"post"`, `"entityId":10`, `"field":"comment_count"`, `"delta":1`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("marshaled JSON missing %q, got: %s", want, b)
		}
	}
}

func TestBaseEvent_WireKeyIsSnakeCaseEventType(t *testing.T) {
	b, err := json.Marshal(NewBaseEvent(EventTypeAccountDeleted, "test"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal into map: %v", err)
	}
	if _, ok := m["event_type"]; !ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		t.Errorf("BaseEvent must serialize EventType as \"event_type\"; got keys %v", keys)
	}
}
