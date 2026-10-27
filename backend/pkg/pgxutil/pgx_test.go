package pgxutil

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestDateParamAndValue(t *testing.T) {
	when := time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input *time.Time
		valid bool
	}{
		{name: "nil", input: nil, valid: false},
		{name: "zero", input: &time.Time{}, valid: false},
		{name: "valid", input: &when, valid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DateParam(tt.input)
			if got.Valid != tt.valid {
				t.Fatalf("DateParam(%v).Valid = %v, want %v", tt.input, got.Valid, tt.valid)
			}
			if tt.valid && !got.Time.Equal(when) {
				t.Fatalf("DateParam(%v).Time = %v, want %v", tt.input, got.Time, when)
			}

			value := DateValue(got)
			if !tt.valid {
				if value != nil {
					t.Fatalf("DateValue(%v) = %v, want nil", got, value)
				}
				return
			}
			if value == nil || !value.Equal(when) {
				t.Fatalf("DateValue(%v) = %v, want %v", got, value, when)
			}
		})
	}
}

func TestTimestamptzParam(t *testing.T) {
	when := time.Date(2026, time.August, 27, 8, 30, 0, 0, time.UTC)

	if got := TimestamptzParam(time.Time{}); got.Valid {
		t.Fatalf("TimestamptzParam(zero).Valid = true, want false")
	}
	got := TimestamptzParam(when)
	if !got.Valid || !got.Time.Equal(when) {
		t.Fatalf("TimestamptzParam(%v) = %+v, want valid timestamp", when, got)
	}
}

func TestJSONBoundaryPreservesRawMessageTypeAndEmptyPolicy(t *testing.T) {
	if got := JSONBytes(nil); got != nil {
		t.Fatalf("JSONBytes(nil) = %v, want nil", got)
	}
	if got := JSONBytes(json.RawMessage{}); got != nil {
		t.Fatalf("JSONBytes(empty) = %v, want nil", got)
	}
	input := json.RawMessage(`{"ok":true}`)
	if got := JSONBytes(input); !reflect.DeepEqual(got, input) {
		t.Fatalf("JSONBytes(%s) = %s, want unchanged payload", input, got)
	}
	if _, ok := any(JSONBytes(input)).(json.RawMessage); !ok {
		t.Fatalf("JSONBytes(%s) returned a non-json.RawMessage value", input)
	}

	if got := JSONRawMessage(nil); got != nil {
		t.Fatalf("JSONRawMessage(nil) = %v, want nil", got)
	}
	if got := JSONRawMessage([]byte(`{"ok":true}`)); string(got) != `{"ok":true}` {
		t.Fatalf("JSONRawMessage() = %s, want payload", got)
	}
}
