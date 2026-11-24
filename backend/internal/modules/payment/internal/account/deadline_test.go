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

// 掃描週期要讓每個期限在 ONE_DAY 檔位裡至少被掃到一次、且離期限至少 23 小時：每日掃描時，期限落在兩次掃描之間的帳戶會在期限前幾分鐘才收到、或整個跳過。
func TestDeadlineScanReachesEveryDeadlineInTheLastDayTierEarly(t *testing.T) {
	for offset := time.Duration(0); offset < 24*time.Hour; offset += 7 * time.Minute {
		deadline := 8*24*time.Hour + offset
		earliest := time.Duration(-1)
		for scan := time.Duration(0); scan < deadline; scan += deadlineScanInterval {
			if remaining := deadline - scan; TierFor(remaining) == model.DeadlineOneDay {
				earliest = remaining
				break
			}
		}
		if earliest < 23*time.Hour {
			t.Fatalf("期限在 %v 的帳戶：ONE_DAY 最早在期限前 %v 發出（-1 是沒發），預期至少 23 小時", deadline, earliest)
		}
	}
}
