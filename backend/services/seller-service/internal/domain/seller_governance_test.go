package domain

import (
	"strings"
	"testing"
	"time"
)

// ========== Enum IsValid ==========

func TestViolationCategory_IsValid(t *testing.T) {
	cases := map[string]struct {
		c     ViolationCategory
		valid bool
	}{
		"food_safety":        {ViolationCategoryFoodSafety, true},
		"fake_certification": {ViolationCategoryFakeCertification, true},
		"misleading_info":    {ViolationCategoryMisleadingInfo, true},
		"counterfeit":        {ViolationCategoryCounterfeit, true},
		"unauthorized_sale":  {ViolationCategoryUnauthorizedSale, true},
		"fraud":              {ViolationCategoryFraud, true},
		"other":              {ViolationCategoryOther, true},
		"unknown":            {ViolationCategory("garbage"), false},
		"empty":              {ViolationCategory(""), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.c.IsValid(); got != tc.valid {
				t.Errorf("IsValid()=%v, want %v", got, tc.valid)
			}
		})
	}
}

func TestViolationSeverity_Rank(t *testing.T) {
	cases := []struct {
		s    ViolationSeverity
		rank int
	}{
		{ViolationSeverityCritical, 0},
		{ViolationSeverityHigh, 1},
		{ViolationSeverityMedium, 2},
		{ViolationSeverityLow, 3},
		{ViolationSeverity("unknown"), 4},
	}
	for _, tc := range cases {
		if got := tc.s.Rank(); got != tc.rank {
			t.Errorf("severity %q rank=%d, want %d", tc.s, got, tc.rank)
		}
	}
}

func TestReportStatus_CanTransitionTo(t *testing.T) {
	cases := []struct {
		from, to ReportStatus
		ok       bool
	}{
		// 合法路徑
		{ReportStatusPending, ReportStatusInvestigating, true},
		{ReportStatusPending, ReportStatusDismissed, true},
		{ReportStatusInvestigating, ReportStatusVerified, true},
		{ReportStatusInvestigating, ReportStatusDismissed, true},
		// 終態不可改
		{ReportStatusVerified, ReportStatusPending, false},
		{ReportStatusVerified, ReportStatusDismissed, false},
		{ReportStatusDismissed, ReportStatusPending, false},
		// 反向不可
		{ReportStatusInvestigating, ReportStatusPending, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			if got := tc.from.CanTransitionTo(tc.to); got != tc.ok {
				t.Errorf("got %v, want %v", got, tc.ok)
			}
		})
	}
}

// ========== SellerStatus B-169 New Const ==========

func TestSellerStatus_SuspendedGovernance_FailSafe(t *testing.T) {
	// A5: SellerStatusSuspendedGovernance 對 IsActive() / IsClosed() 都回 false。
	s := &Seller{Status: SellerStatusSuspendedGovernance}
	if s.IsActive() {
		t.Error("suspended_governance must NOT be IsActive()")
	}
	if s.IsClosed() {
		t.Error("suspended_governance must NOT be IsClosed()")
	}
}

// ========== Phone Normalization ==========

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"0912345678", "0912345678", false},
		{"0912-345-678", "0912345678", false},
		{"0912 345 678", "0912345678", false},
		{"+886912345678", "0912345678", false},
		{"+886-912-345-678", "0912345678", false},
		{"886912345678", "0912345678", false},
		// invalid
		{"1234567890", "", true},
		{"abcdefghij", "", true},
		{"091234567", "", true},   // 9 碼太短
		{"09123456789", "", true}, // 11 碼太長
		{"", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := NormalizePhone(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ========== SellerViolationReport Validate ==========

func TestSellerViolationReport_Validate(t *testing.T) {
	base := func() *SellerViolationReport {
		return &SellerViolationReport{
			TargetSellerID:    1,
			ReporterAccountID: 2,
			Category:          ViolationCategoryMisleadingInfo,
			Severity:          ViolationSeverityMedium,
			Status:            ReportStatusPending,
			Description:       "test",
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("base should be valid: %v", err)
	}

	cases := []struct {
		name string
		mod  func(r *SellerViolationReport)
	}{
		{"missing target_seller_id", func(r *SellerViolationReport) { r.TargetSellerID = 0 }},
		{"missing reporter (anonymous)", func(r *SellerViolationReport) { r.ReporterAccountID = 0 }},
		{"invalid category", func(r *SellerViolationReport) { r.Category = ViolationCategory("X") }},
		{"invalid severity", func(r *SellerViolationReport) { r.Severity = ViolationSeverity("X") }},
		{"invalid status", func(r *SellerViolationReport) { r.Status = ReportStatus("X") }},
		{"empty description", func(r *SellerViolationReport) { r.Description = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base()
			tc.mod(r)
			if err := r.Validate(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// ========== FoodSafetyReport Validate ==========

func TestFoodSafetyReport_Validate(t *testing.T) {
	base := func() *FoodSafetyReport {
		return &FoodSafetyReport{
			ReporterPhone: "0912345678",
			Description:   "食用後腹瀉",
			Status:        ReportStatusPending,
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("base should be valid: %v", err)
	}

	t.Run("missing phone", func(t *testing.T) {
		r := base()
		r.ReporterPhone = ""
		err := r.Validate()
		if err == nil || !strings.Contains(err.Error(), "reporter_phone is required") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("phone format invalid (need NormalizePhone first)", func(t *testing.T) {
		r := base()
		r.ReporterPhone = "0912-345-678" // dash not normalized
		if err := r.Validate(); err == nil {
			t.Error("expected error for unnormalized phone")
		}
	})

	t.Run("missing description", func(t *testing.T) {
		r := base()
		r.Description = ""
		if err := r.Validate(); err == nil {
			t.Error("expected error")
		}
	})

	// issue #1527：匿名食安通報 deny-by-default 拒收佐證附件（evidence_asset_ids 恆空）。
	t.Run("rejects evidence attachments (issue #1527)", func(t *testing.T) {
		r := base()
		r.EvidenceAssetIDs = []string{"asset_x"}
		err := r.Validate()
		if err == nil || !strings.Contains(err.Error(), "do not accept evidence") {
			t.Errorf("expected evidence rejection, got %v", err)
		}
	})

	t.Run("text-only report is valid (no evidence)", func(t *testing.T) {
		r := base()
		r.EvidenceAssetIDs = nil
		if err := r.Validate(); err != nil {
			t.Errorf("text-only report should be valid: %v", err)
		}
	})
}

func TestFoodSafetyReport_AckSLA(t *testing.T) {
	created := time.Now().Add(-3 * time.Hour)
	acked := time.Now().Add(-2 * time.Hour) // 1h after created → met
	r := &FoodSafetyReport{
		CreatedAt:      created,
		AcknowledgedAt: &acked,
	}
	if !r.IsAckSLAMet() {
		t.Errorf("1h ack should meet 4h SLA")
	}

	acked2 := created.Add(5 * time.Hour) // 5h after → missed
	r.AcknowledgedAt = &acked2
	if r.IsAckSLAMet() {
		t.Errorf("5h ack should NOT meet 4h SLA")
	}

	r.AcknowledgedAt = nil
	if r.IsAckSLAMet() {
		t.Errorf("unacked should NOT meet SLA")
	}
}

// ========== SellerViolation D14 penalty 欄位互斥 ==========

func TestSellerViolation_PenaltyFields(t *testing.T) {
	now := time.Now()
	expires := now.Add(30 * 24 * time.Hour)
	amount := int64(500_000)

	base := func() *SellerViolation {
		return &SellerViolation{
			SellerID:         1,
			CreatedByAdminID: 99,
			Category:         ViolationCategoryFoodSafety,
			Severity:         ViolationSeverityHigh,
			PenaltyType:      PenaltyTypeWarning,
			Status:           ViolationStatusEnforced,
			Description:      "test",
			EffectiveAt:      now,
		}
	}

	t.Run("warning OK", func(t *testing.T) {
		v := base()
		if err := v.Validate(); err != nil {
			t.Errorf("warning base should be valid: %v", err)
		}
	})
	t.Run("warning with penalty_amount_cents fails", func(t *testing.T) {
		v := base()
		v.PenaltyAmountCents = &amount
		if err := v.Validate(); err == nil {
			t.Error("warning + amount should fail")
		}
	})
	t.Run("warning with expires_at fails", func(t *testing.T) {
		v := base()
		v.ExpiresAt = &expires
		if err := v.Validate(); err == nil {
			t.Error("warning + expires_at should fail")
		}
	})
	t.Run("full_suspension without expires_at fails", func(t *testing.T) {
		v := base()
		v.PenaltyType = PenaltyTypeFullSuspension
		// expires_at nil
		if err := v.Validate(); err == nil {
			t.Error("full_suspension w/o expires_at should fail")
		}
	})
	t.Run("full_suspension with expires_at OK", func(t *testing.T) {
		v := base()
		v.PenaltyType = PenaltyTypeFullSuspension
		v.ExpiresAt = &expires
		if err := v.Validate(); err != nil {
			t.Errorf("full_suspension + expires_at should be valid: %v", err)
		}
	})
	t.Run("fine without amount fails", func(t *testing.T) {
		v := base()
		v.PenaltyType = PenaltyTypeFine
		if err := v.Validate(); err == nil {
			t.Error("fine w/o amount should fail")
		}
	})
	t.Run("fine with expires_at fails", func(t *testing.T) {
		v := base()
		v.PenaltyType = PenaltyTypeFine
		v.PenaltyAmountCents = &amount
		v.ExpiresAt = &expires
		if err := v.Validate(); err == nil {
			t.Error("fine + expires_at should fail")
		}
	})
}

// ========== SellerViolation D16 source FK 互斥 ==========

func TestSellerViolation_SourceMutex(t *testing.T) {
	now := time.Now()
	srid := int64(100)
	frid := int64(200)

	base := func() *SellerViolation {
		return &SellerViolation{
			SellerID:         1,
			CreatedByAdminID: 99,
			Category:         ViolationCategoryFoodSafety,
			Severity:         ViolationSeverityHigh,
			PenaltyType:      PenaltyTypeWarning,
			Status:           ViolationStatusEnforced,
			Description:      "test",
			EffectiveAt:      now,
		}
	}

	t.Run("both nil OK (admin direct create)", func(t *testing.T) {
		v := base()
		if err := v.Validate(); err != nil {
			t.Errorf("got %v", err)
		}
	})
	t.Run("only seller_report_id OK", func(t *testing.T) {
		v := base()
		v.SourceSellerReportID = &srid
		if err := v.Validate(); err != nil {
			t.Errorf("got %v", err)
		}
	})
	t.Run("only food_safety_report_id OK", func(t *testing.T) {
		v := base()
		v.SourceFoodSafetyReportID = &frid
		if err := v.Validate(); err != nil {
			t.Errorf("got %v", err)
		}
	})
	t.Run("both set fails", func(t *testing.T) {
		v := base()
		v.SourceSellerReportID = &srid
		v.SourceFoodSafetyReportID = &frid
		if err := v.Validate(); err == nil {
			t.Error("both source set should fail")
		}
	})
}

// ========== SellerViolation D11 IsActive computed ==========

func TestSellerViolation_IsActiveAt(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	expires := now.Add(30 * 24 * time.Hour)

	cases := []struct {
		name string
		v    SellerViolation
		want bool
	}{
		{
			name: "enforced + no expires + no revoke → active",
			v:    SellerViolation{Status: ViolationStatusEnforced},
			want: true,
		},
		{
			name: "enforced + future expires + no revoke → active",
			v:    SellerViolation{Status: ViolationStatusEnforced, ExpiresAt: &expires},
			want: true,
		},
		{
			name: "enforced + revoked → inactive",
			v: SellerViolation{
				Status:    ViolationStatusEnforced,
				RevokedAt: ptrTime(now.Add(-1 * time.Hour)),
			},
			want: false,
		},
		{
			name: "expired → inactive",
			v: SellerViolation{
				Status:    ViolationStatusEnforced,
				ExpiresAt: ptrTime(now.Add(-1 * time.Hour)),
			},
			want: false,
		},
		{
			name: "status=revoked → inactive",
			v:    SellerViolation{Status: ViolationStatusRevoked},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.v.IsActiveAt(now); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// ========== SellerViolation D8 IsPubliclyVisible ==========

func TestSellerViolation_IsPubliclyVisibleAt(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	publicSummary := "違規摘要"

	base := func() SellerViolation {
		return SellerViolation{
			Severity:      ViolationSeverityHigh,
			Status:        ViolationStatusEnforced,
			EffectiveAt:   now.Add(-30 * 24 * time.Hour),
			PublicSummary: &publicSummary,
		}
	}

	t.Run("base high + public_summary → visible", func(t *testing.T) {
		v := base()
		if !v.IsPubliclyVisibleAt(now) {
			t.Error("expected visible")
		}
	})
	t.Run("medium severity → hidden", func(t *testing.T) {
		v := base()
		v.Severity = ViolationSeverityMedium
		if v.IsPubliclyVisibleAt(now) {
			t.Error("medium should NOT be public")
		}
	})
	t.Run("revoked → hidden", func(t *testing.T) {
		v := base()
		v.RevokedAt = ptrTime(now.Add(-1 * time.Hour))
		if v.IsPubliclyVisibleAt(now) {
			t.Error("revoked should NOT be public")
		}
	})
	t.Run("expired → hidden", func(t *testing.T) {
		v := base()
		v.ExpiresAt = ptrTime(now.Add(-1 * time.Hour))
		if v.IsPubliclyVisibleAt(now) {
			t.Error("expired should NOT be public")
		}
	})
	t.Run("90 days old → hidden", func(t *testing.T) {
		v := base()
		v.EffectiveAt = now.Add(-100 * 24 * time.Hour)
		if v.IsPubliclyVisibleAt(now) {
			t.Error("100d old should NOT be public")
		}
	})
	t.Run("public_summary nil → hidden", func(t *testing.T) {
		v := base()
		v.PublicSummary = nil
		if v.IsPubliclyVisibleAt(now) {
			t.Error("nil public_summary should NOT be public")
		}
	})
}

// ========== Appeal state machine ==========

func TestSellerViolation_CanSubmitAppeal(t *testing.T) {
	t.Run("revoked cannot appeal", func(t *testing.T) {
		v := &SellerViolation{Status: ViolationStatusRevoked, Appealed: false}
		if err := v.CanSubmitAppeal(); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("already appealed cannot re-appeal", func(t *testing.T) {
		v := &SellerViolation{Status: ViolationStatusEnforced, Appealed: true}
		if err := v.CanSubmitAppeal(); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("first time enforced OK", func(t *testing.T) {
		v := &SellerViolation{Status: ViolationStatusEnforced, Appealed: false}
		if err := v.CanSubmitAppeal(); err != nil {
			t.Errorf("got %v", err)
		}
	})
}

func TestSellerViolation_CanDecideAppeal(t *testing.T) {
	pending := AppealStatusPending
	accepted := AppealStatusAccepted

	t.Run("not appealed cannot decide", func(t *testing.T) {
		v := &SellerViolation{Appealed: false}
		if err := v.CanDecideAppeal(); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("already decided cannot re-decide", func(t *testing.T) {
		v := &SellerViolation{Appealed: true, AppealStatus: &accepted}
		if err := v.CanDecideAppeal(); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("pending OK", func(t *testing.T) {
		v := &SellerViolation{Appealed: true, AppealStatus: &pending}
		if err := v.CanDecideAppeal(); err != nil {
			t.Errorf("got %v", err)
		}
	})
}

// ========== Phase 1 enabled penalty types ==========

func TestPenaltyType_Phase1Enabled(t *testing.T) {
	cases := map[PenaltyType]bool{
		PenaltyTypeWarning:           true,
		PenaltyTypeFullSuspension:    true,
		PenaltyTypeFine:              false,
		PenaltyTypePartialSuspension: false,
		PenaltyTypePermanentBan:      false,
		PenaltyTypeMandatoryTraining: false,
	}
	for p, want := range cases {
		t.Run(string(p), func(t *testing.T) {
			if got := p.IsPhase1Enabled(); got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

// ========== helpers ==========

func ptrTime(t time.Time) *time.Time { return &t }
