package config

import (
	"strings"
	"testing"
)

// defaultScheduledPostPublish 是 §8.5.1 (C) 的定版預設值。
func defaultScheduledPostPublish() ScheduledPostPublishConfig {
	return ScheduledPostPublishConfig{
		PollIntervalSeconds: 30,
		BatchSize:           50,
		LeaseSeconds:        120,
		MaxRetry:            5,
		BackoffBaseSeconds:  60,
		BackoffMaxSeconds:   1800,

		AuditIntentDeadlineMs:     800,
		AuditConfirmDeadlineMs:    400,
		AuditConfirmMaxAttempts:   3,
		AuditConfirmTotalBudgetMs: 2000,
		TxBudgetMs:                5000,
		WindowHours:               24,
	}
}

// 三條斷言各自守著一個「服務照常啟動、boot 照常綠」的失敗模式，所以每條都要有反例。
func TestConfig_ValidateScheduledPostPublish(t *testing.T) {
	base := func() *Config {
		return &Config{
			MediaBaseURL:         "https://test-media.example.com",
			ScheduledPostPublish: defaultScheduledPostPublish(),
		}
	}

	t.Run("defaults pass", func(t *testing.T) {
		if err := base().Validate(); err != nil {
			t.Fatalf("default (C) values should pass: %v", err)
		}
	})

	t.Run("lease below worst-case per-row cost", func(t *testing.T) {
		// 單筆最壞耗時 = 800 + 5000 + 2000 = 7800 ms；租約 5s 比它短 ⇒ 持有者還在
		// 發布就被合法搶走 ⇒ 同一則貼文重複發布。
		cfg := base()
		cfg.ScheduledPostPublish.LeaseSeconds = 5
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected fail-fast when the lease is shorter than one row's worst case")
		}
		if !strings.Contains(err.Error(), "worst-case per-row cost") {
			t.Errorf("error should name the lower bound: %v", err)
		}
	})

	t.Run("lease too close to the candidate window", func(t *testing.T) {
		// 窗口縮到 1 小時（值域下界）＋租約拉到 600s（值域上界）⇒ 6000 >= 3600，
		// 租約退化成永久旗標（§29 的 L4 形狀）。
		cfg := base()
		cfg.ScheduledPostPublish.WindowHours = 1
		cfg.ScheduledPostPublish.LeaseSeconds = 600
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected fail-fast when the lease approaches the candidate window")
		}
		if !strings.Contains(err.Error(), "too close to the candidate window") {
			t.Errorf("error should name the upper bound: %v", err)
		}
	})

	t.Run("backoff series outlives the candidate window", func(t *testing.T) {
		// 600+1200+2400+4800 = 9000s，窗口只有 3600s ⇒ 重試還沒用完就逾窗轉 expired，
		// OBS8 的 retry_exhausted 被 window_exceeded 蓋掉。
		cfg := base()
		cfg.ScheduledPostPublish.WindowHours = 1
		cfg.ScheduledPostPublish.BackoffBaseSeconds = 600
		cfg.ScheduledPostPublish.BackoffMaxSeconds = 7200
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected fail-fast when the backoff series cannot finish inside the window")
		}
		if !strings.Contains(err.Error(), "backoff series totals") {
			t.Errorf("error should name the backoff series: %v", err)
		}
	})
}

func TestBackoffSeriesTotalSeconds(t *testing.T) {
	cases := []struct {
		base, capSeconds, maxRetry, want int
	}{
		{60, 1800, 5, 900},   // (C) 組預設：60+120+240+480
		{60, 1800, 1, 0},     // 只有一次嘗試 ⇒ 沒有任何退避
		{60, 100, 5, 360},    // 60+100+100+100，封頂生效（120 已越過 100）
		{600, 7200, 5, 9000}, // 反例用值
	}
	for _, c := range cases {
		if got := backoffSeriesTotalSeconds(c.base, c.capSeconds, c.maxRetry); got != c.want {
			t.Errorf("backoffSeriesTotalSeconds(%d, %d, %d) = %d, want %d",
				c.base, c.capSeconds, c.maxRetry, got, c.want)
		}
	}
}
