package mq

import (
	"encoding/json"
	"strings"
	"testing"
)

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
