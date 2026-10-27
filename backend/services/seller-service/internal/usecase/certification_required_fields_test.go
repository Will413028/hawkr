package usecase

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMarshalRequiredFields_Empty(t *testing.T) {
	if got := marshalRequiredFields(nil); got != nil {
		t.Errorf("marshalRequiredFields(nil) = %v, want nil", got)
	}
	if got := marshalRequiredFields([]string{}); got != nil {
		t.Errorf("marshalRequiredFields([]string{}) = %v, want nil", got)
	}
}

func TestMarshalRequiredFields_RoundTrip(t *testing.T) {
	fields := []string{"tax_id", "id_front"}
	raw := marshalRequiredFields(fields)
	if raw == nil {
		t.Fatal("expected non-nil json.RawMessage")
	}
	got := unmarshalRequiredFields(raw)
	if !reflect.DeepEqual(got, fields) {
		t.Errorf("round-trip = %v, want %v", got, fields)
	}
}

func TestUnmarshalRequiredFields_EmptyOrInvalid(t *testing.T) {
	if got := unmarshalRequiredFields(nil); got != nil {
		t.Errorf("unmarshalRequiredFields(nil) = %v, want nil", got)
	}
	if got := unmarshalRequiredFields(json.RawMessage(`not json`)); got != nil {
		t.Errorf("unmarshalRequiredFields(invalid) = %v, want nil (fail open, not error)", got)
	}
}
