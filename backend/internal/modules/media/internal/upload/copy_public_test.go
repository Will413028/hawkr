package upload

import (
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestCopyPublic_RejectsAnInvalidUploaderBeforeTouchingAnything(t *testing.T) {
	for _, id := range []int64{0, -1} {
		_, err := (&Service{}).CopyPublic(t.Context(), model.Uploader{ID: id}, uuid.NewV7())
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("uploader %d：err = %v，預期 invalid_input", id, err)
		}
	}
}
