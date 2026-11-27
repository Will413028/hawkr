package seller

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"uuid"

	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 佐證端點的 ServeMux 樣式：通報的佐證以通報 ID、措施上請求（申覆）的佐證以請求 ID、詢問回覆的附件以詢問 ID 定址。
const (
	EvidenceReportRoute  = "GET /api/files/admin/seller/reports/{report_id}/evidence/{position}"
	EvidenceRequestRoute = "GET /api/files/admin/seller/measure_requests/{request_id}/evidence/{position}"
	EvidenceInquiryRoute = "GET /api/files/admin/seller/case_inquiries/{inquiry_id}/evidence/{position}"
)

// EvidenceModule 是端點需要的模組方法。
type EvidenceModule interface {
	ReadEvidence(context.Context, model.ReadEvidenceInput) (model.FileContent, error)
}

// AdminEvidence 是一種佐證（通報或請求）的端點 handler。
type AdminEvidence struct {
	m       EvidenceModule
	kind    model.EvidenceKind
	idParam string
	log     *slog.Logger
}

// NewAdminEvidence 建立 kind 那一種佐證的端點 handler（EvidenceReport 掛在 EvidenceReportRoute，EvidenceRequest 掛在
// EvidenceRequestRoute，EvidenceInquiry 掛在 EvidenceInquiryRoute）。
func NewAdminEvidence(m EvidenceModule, kind model.EvidenceKind, log *slog.Logger) *AdminEvidence {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	idParam := "report_id"
	switch kind {
	case model.EvidenceReport:
	case model.EvidenceRequest:
		idParam = "request_id"
	case model.EvidenceInquiry:
		idParam = "inquiry_id"
	}
	return &AdminEvidence{m: m, kind: kind, idParam: idParam, log: log}
}

func (h *AdminEvidence) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	adminID, ok := adminFile(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue(h.idParam))
	position, perr := strconv.Atoi(r.PathValue("position"))
	if err != nil || perr != nil || position < 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	content, err := h.m.ReadEvidence(r.Context(), model.ReadEvidenceInput{AdminID: adminID, Kind: h.kind, RefID: id, Position: position})
	if err != nil {
		fileFailure(w, r, h.log, err, "治理佐證")
		return
	}
	writeFile(w, r, h.log, content)
}
