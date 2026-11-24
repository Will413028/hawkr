package account

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
)

func TestTierForSplitsTheWindowAtTheLastDay(t *testing.T) {
	for _, c := range []struct {
		remaining time.Duration
		want      model.DeadlineTier
	}{
		{7 * 24 * time.Hour, model.DeadlineSevenDays},
		{24*time.Hour + time.Second, model.DeadlineSevenDays},
		{24 * time.Hour, model.DeadlineOneDay},
		{time.Second, model.DeadlineOneDay},
	} {
		if got := TierFor(c.remaining); got != c.want {
			t.Errorf("TierFor(%v) = %s，預期 %s", c.remaining, got, c.want)
		}
	}
}
