package postgres

import (
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

func TestNotificationTemplate_ImageKeyTemplateField(t *testing.T) {
	// Compile-time check that ImageKeyTemplate field exists on the struct.
	tmpl := domain.NotificationTemplate{}
	url := "#{sellerAvatar}"
	tmpl.ImageKeyTemplate = &url
	if tmpl.ImageKeyTemplate == nil || *tmpl.ImageKeyTemplate != "#{sellerAvatar}" {
		t.Fatal("ImageKeyTemplate field assignment failed")
	}
}

func TestNotificationTemplate_PayloadFilterField(t *testing.T) {
	// Compile-time check that PayloadFilter field exists on the struct.
	tmpl := domain.NotificationTemplate{}
	tmpl.PayloadFilter = map[string]domain.FilterCondition{"status": domain.Eq("IN_TRANSIT")}
	if v, ok := tmpl.PayloadFilter["status"]; !ok || v != domain.Eq("IN_TRANSIT") {
		t.Fatal("PayloadFilter field assignment failed")
	}
}

// Nullable primitive fields stay as pointers at the sqlc boundary. In
// particular, a non-nil empty string remains distinguishable from NULL.
func TestNotificationTemplate_OptionalFieldsPreserveNilAndEmpty(t *testing.T) {
	empty := ""
	updatedBy := int64(42)
	params, err := domainToInsertParams(&domain.NotificationTemplate{
		BodyFallback: &empty,
		UpdatedBy:    &updatedBy,
	}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.BodyFallback == nil || *params.BodyFallback != "" {
		t.Fatalf("expected empty optional text to remain non-nil, got %v", params.BodyFallback)
	}
	if params.SchemaUrl != nil {
		t.Fatalf("expected nil schema URL to remain NULL, got %v", params.SchemaUrl)
	}
	if params.UpdatedBy == nil || *params.UpdatedBy != updatedBy {
		t.Fatalf("expected updated_by pointer to remain present, got %v", params.UpdatedBy)
	}
}

func TestMarshalNullableJSON_NilMapReturnsNil(t *testing.T) {
	var m map[string]string
	got, err := marshalNullableJSON(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil bytes for nil map, got %q", string(got))
	}
}

func TestMarshalNullableJSON_EmptyMapReturnsBraces(t *testing.T) {
	got, err := marshalNullableJSON(map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "{}" {
		t.Errorf("expected {}, got %q", string(got))
	}
}

func TestMarshalNullableJSON_PopulatedMap(t *testing.T) {
	got, err := marshalNullableJSON(map[string]string{"status": "IN_TRANSIT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `{"status":"IN_TRANSIT"}` {
		t.Errorf("unexpected JSON: %q", string(got))
	}
}
