package seller

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"uuid"

	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// DocumentRoute 是補充文件端點的 ServeMux 樣式。
const DocumentRoute = "GET /api/files/admin/seller/kyc_applications/{application_id}/documents/{position}"

func documentURI(applicationID uuid.UUID, position int) string {
	return "/api/files/admin/seller/kyc_applications/" + applicationID.String() + "/documents/" + strconv.Itoa(position)
}

// DocumentModule 是端點需要的模組方法。
type DocumentModule interface {
	ReadKYCDocument(context.Context, model.ReadKYCDocumentInput) (model.FileContent, error)
}

// AdminDocuments 是證件圖片端點的 handler。
type AdminDocuments struct {
	m   DocumentModule
	log *slog.Logger
}

// NewAdminDocuments 建立端點的 handler。
func NewAdminDocuments(m DocumentModule, log *slog.Logger) *AdminDocuments {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &AdminDocuments{m: m, log: log}
}

func (h *AdminDocuments) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	adminID, ok := adminFile(w, r)
	if !ok {
		return
	}
	in, found := documentRequest(r)
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	in.AdminID = adminID
	content, err := h.m.ReadKYCDocument(r.Context(), in)
	if err != nil {
		fileFailure(w, r, h.log, err, "KYC 證件")
		return
	}
	writeFile(w, r, h.log, content)
}

func documentRequest(r *http.Request) (model.ReadKYCDocumentInput, bool) {
	id, err := uuid.Parse(r.PathValue("application_id"))
	if err != nil {
		return model.ReadKYCDocumentInput{}, false
	}
	position, err := strconv.Atoi(r.PathValue("position"))
	if err != nil || position < 0 {
		return model.ReadKYCDocumentInput{}, false
	}
	return model.ReadKYCDocumentInput{ApplicationID: id, Position: position}, true
}
