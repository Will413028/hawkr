package postgres

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/review-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestNullableText(t *testing.T) {
	if got := nullableText(""); got != nil {
		t.Fatalf("nullableText(\"\") = %q, want nil", *got)
	}

	const value = "moderation reason"
	got := nullableText(value)
	if got == nil || *got != value {
		t.Fatalf("nullableText(%q) = %v, want pointer to original value", value, got)
	}
}

func TestStringValue(t *testing.T) {
	if got := stringValue(nil); got != "" {
		t.Fatalf("stringValue(nil) = %q, want empty string", got)
	}

	empty := ""
	if got := stringValue(&empty); got != "" {
		t.Fatalf("stringValue(pointer to empty string) = %q, want empty string", got)
	}

	value := "review content"
	if got := stringValue(&value); got != value {
		t.Fatalf("stringValue(%q) = %q, want original value", value, got)
	}
}

func TestToReview_NullableFields(t *testing.T) {
	createdAt := time.Date(2026, time.August, 26, 11, 22, 33, 0, time.UTC)
	replyAt := time.Date(2026, time.August, 26, 12, 34, 56, 0, time.UTC)
	empty := ""
	row := gen.OrderItemReview{
		ID:           1,
		CreatedAt:    pgtype.Timestamptz{Time: createdAt, Valid: true},
		ReplyContent: &empty,
		RepliedAt:    &replyAt,
	}

	got := toReview(row)
	if got.Content != "" {
		t.Errorf("nil Content = %q, want empty domain value", got.Content)
	}
	if !got.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, createdAt)
	}
	if got.ReplyContent != "" {
		t.Errorf("empty ReplyContent = %q, want empty domain value", got.ReplyContent)
	}
	if !got.RepliedAt.Equal(replyAt) {
		t.Errorf("RepliedAt = %v, want %v", got.RepliedAt, replyAt)
	}
}
