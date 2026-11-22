package model_test

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
)

func TestStuckUploadGraceOutlastsEveryProcessAttempt(t *testing.T) {
	worst := model.ProcessMaxAttempts * (model.ProcessTimeout + model.ProcessMaxBackoff)
	if model.StuckUploadGrace <= worst {
		t.Fatalf("StuckUploadGrace %v 必須大於 %d 次嘗試加退避的總長 %v", model.StuckUploadGrace, model.ProcessMaxAttempts, worst)
	}
}
