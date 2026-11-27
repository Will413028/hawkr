package config

import (
	"strings"
	"testing"
)

func defaultDelegation() DelegationConfig {
	return DelegationConfig{
		GrpcDeadlinePreflightMs:    3000,
		GrpcDeadlineCreateMs:       2000,
		GrpcDeadlineCurrentMs:      800,
		GrpcDeadlineStoreStateMs:   800,
		GrpcDeadlineDomainReadMs:   800,
		GrpcDeadlineAuditIntentMs:  800,
		GrpcDeadlineAuditConfirmMs: 400,
		GrpcDeadlineDomainWriteMs:  4000,

		GrpcDeadlineBatchDomainReadMs:   3000,
		GrpcDeadlineBatchAuditIntentMs:  4000,
		GrpcDeadlineBatchDomainWriteMs:  20000,
		GrpcDeadlineBatchAuditConfirmMs: 3000,

		AuditConfirmTotalBudgetMs:      2000,
		AuditConfirmBatchTotalBudgetMs: 10000,
	}
}

// 值域上界是逐項的，加總不保證成立：單筆組全部調到各自上界的加總是 27 秒，遠超過
// 該路徑的 handler 預算。這兩條斷言就是為此存在，所以驗收必須含把加總推爆的反例。
func TestConfig_ValidateDelegationBudgets(t *testing.T) {
	base := func() *Config {
		return &Config{
			MediaBaseURL: "https://test-media.example.com",
			Delegation:   defaultDelegation(),
		}
	}

	t.Run("defaults pass", func(t *testing.T) {
		if err := base().Validate(); err != nil {
			t.Fatalf("default budgets should pass: %v", err)
		}
	})

	t.Run("single write budget exceeded", func(t *testing.T) {
		cfg := base()
		cfg.Delegation.GrpcDeadlineDomainWriteMs = 8000
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected fail-fast when the single-item budget is exceeded")
		}
		if !strings.Contains(err.Error(), "single-item delegated write budget") {
			t.Errorf("error should name the single-item budget: %v", err)
		}
	})

	t.Run("batch write budget exceeded", func(t *testing.T) {
		cfg := base()
		cfg.Delegation.GrpcDeadlineBatchDomainWriteMs = 35000
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected fail-fast when the batch budget is exceeded")
		}
		if !strings.Contains(err.Error(), "batch delegated write budget") {
			t.Errorf("error should name the batch budget: %v", err)
		}
	})

	t.Run("confirm total budget counts toward the sum", func(t *testing.T) {
		cfg := base()
		cfg.Delegation.AuditConfirmTotalBudgetMs = 5000
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected the confirm total budget to be part of the single-item sum")
		}
	})
}
