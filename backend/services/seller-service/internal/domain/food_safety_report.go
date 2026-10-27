package domain

import (
	"errors"
	"regexp"
	"time"
)

// B-169 Food Safety Report — 公開無登入食安通報。
// severity 不存 DB 欄位（食安通道全 critical）；admin list 合併 query 時以 'critical' 投影。

// FDAReportChannel admin 通報 FDA / 衛生局時走的管道。
type FDAReportChannel string

const (
	FDAReportChannelHotline1919   FDAReportChannel = "1919_hotline"
	FDAReportChannelOnlineForm    FDAReportChannel = "online_form"
	FDAReportChannelFaxHealthDept FDAReportChannel = "fax_health_dept"
	FDAReportChannelInPerson      FDAReportChannel = "in_person"
	FDAReportChannelOther         FDAReportChannel = "other"
)

// ValidFDAReportChannels 合法值集合。
var ValidFDAReportChannels = map[FDAReportChannel]struct{}{
	FDAReportChannelHotline1919:   {},
	FDAReportChannelOnlineForm:    {},
	FDAReportChannelFaxHealthDept: {},
	FDAReportChannelInPerson:      {},
	FDAReportChannelOther:         {},
}

// IsValid 檢查 FDA channel 字串是否合法。
func (c FDAReportChannel) IsValid() bool {
	_, ok := ValidFDAReportChannels[c]
	return ok
}

// taiwanMobilePhonePattern 台灣手機格式 09xxxxxxxx。NormalizePhone 入口接受。
// 允許前綴：(無) / 0 / 886 / +886；後接 9XXXXXXXX。
var taiwanMobilePhonePattern = regexp.MustCompile(`^(?:\+?886|0)?9\d{8}$`)

// normalizedTWMobilePhonePattern 嚴格的 normalized 形式：必為 09xxxxxxxx 共 10 碼。
// 用於 Validate()，確保 DB 寫入前已 NormalizePhone 過。
var normalizedTWMobilePhonePattern = regexp.MustCompile(`^09\d{8}$`)

// NormalizePhone 移除空白與連字符後檢查格式。回傳 normalized 形式（09 開頭 10 碼）。
//
// 支援輸入：
//   - 0912345678
//   - 0912-345-678
//   - +886912345678
//   - +886-912-345-678
//   - 0912 345 678
func NormalizePhone(raw string) (string, error) {
	cleaned := stripPhoneSeparators(raw)
	if !taiwanMobilePhonePattern.MatchString(cleaned) {
		return "", errors.New("invalid Taiwan mobile phone format (expected 09xxxxxxxx)")
	}
	if len(cleaned) == 13 && cleaned[:4] == "+886" {
		return "0" + cleaned[4:], nil
	}
	if len(cleaned) == 12 && cleaned[:3] == "886" {
		return "0" + cleaned[3:], nil
	}
	return cleaned, nil
}

// stripPhoneSeparators 移除空白與連字符。
func stripPhoneSeparators(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '-' {
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// FoodSafetyReport 公開無登入食安通報。
type FoodSafetyReport struct {
	CreatedAt              time.Time
	UpdatedAt              time.Time
	AcknowledgedAt         *time.Time
	InvestigatingAt        *time.Time
	ResolvedAt             *time.Time
	FDAReportedAt          *time.Time
	AdminResolutionSummary *string

	TargetSellerID        *int64
	TargetProductID       *int64
	TargetOrderNo         *string
	AcknowledgedByAdminID *int64

	ReporterEmail    *string
	ReporterName     *string
	FDAReportChannel *FDAReportChannel
	FDAReferenceNo   *string

	EvidenceAssetIDs []string // media-service asset_id 列表

	ReporterPhone string // 必填，已 normalize
	Description   string
	Status        ReportStatus

	ID int64
}

// Validate 檢查必填欄位與 enum 合法性。phone 應該事先 NormalizePhone 過。
func (r *FoodSafetyReport) Validate() error {
	if r.ReporterPhone == "" {
		return errors.New("reporter_phone is required for food safety report")
	}
	if !normalizedTWMobilePhonePattern.MatchString(r.ReporterPhone) {
		return errors.New("reporter_phone must be normalized (09xxxxxxxx); call NormalizePhone first")
	}
	if r.Description == "" {
		return errors.New("description is required")
	}
	// issue #1527 裁決：匿名食安通報 deny-by-default 不受理佐證附件（匿名 media 上傳路由從未存在，
	// evidence_asset_ids 恆空）。非空輸入一律拒（fail-closed，繞 Gateway 直打 gRPC 亦擋）；需附佐證圖
	// 請登入改走 POST /app/violation_reports（category=food_safety，issue #1516 已接線）。
	if len(r.EvidenceAssetIDs) > 0 {
		return errors.New("anonymous food safety reports do not accept evidence attachments; sign in and use POST /app/violation_reports (category=food_safety) to attach evidence")
	}
	if !r.Status.IsValid() {
		return errors.New("invalid status")
	}
	return nil
}

// IsAcknowledged 是否已 ack。
func (r *FoodSafetyReport) IsAcknowledged() bool { return r.AcknowledgedAt != nil }

// AckLatency 從 created_at 到 acknowledged_at 的延遲（4h SLA 量測用）。
// 未 ack 時回 -1。
func (r *FoodSafetyReport) AckLatency() time.Duration {
	if r.AcknowledgedAt == nil {
		return -1
	}
	return r.AcknowledgedAt.Sub(r.CreatedAt)
}

// IsAckSLAMet 是否在 4h 內 ack。
func (r *FoodSafetyReport) IsAckSLAMet() bool {
	d := r.AckLatency()
	return d >= 0 && d < 4*time.Hour
}

// FoodSafetyReportListFilter admin list filter。
type FoodSafetyReportListFilter struct {
	Status      *ReportStatus
	Since       *time.Time
	UnackedOnly *bool // critical 未 ack 視覺強調用
	Limit       int32
	Offset      int32
}
