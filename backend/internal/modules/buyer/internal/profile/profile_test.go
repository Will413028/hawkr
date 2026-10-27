package profile

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
)

func TestProfileValidation(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, gender := range []model.Gender{model.Male, model.Female, model.Other, "invalid"} {
		phone := "+886912345678"
		input := model.UpdateProfileInput{DisplayName: ptr("測試"), Gender: &gender, Phone: &phone}
		err := Validate(&input, now)
		if (err == nil) != (gender != "invalid") {
			t.Fatal(gender, err)
		}
		if err == nil && *input.Phone != "0912345678" {
			t.Fatal(*input.Phone)
		}
	}
	future := now.AddDate(0, 0, 1)
	input := model.UpdateProfileInput{DisplayName: ptr("測試"), BirthDate: &future}
	if Validate(&input, now) == nil {
		t.Fatal("future birth accepted")
	}
	bad := "abc"
	input = model.UpdateProfileInput{DisplayName: ptr("測試"), Phone: &bad}
	if Validate(&input, now) == nil {
		t.Fatal("invalid phone accepted")
	}
}

func ptr[T any](value T) *T { return &value }
