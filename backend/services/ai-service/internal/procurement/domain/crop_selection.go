package domain

// CropSelection is taxonomy progress, separate from the confirmed ItemResolution.
// The values are internal wizard stages persisted in the existing JSON snapshot.
const (
	CropSelectionGroup    = "GROUP"
	CropSelectionItem     = "ITEM"
	CropSelectionDescribe = "DESCRIBE"
)

type CropSelection struct {
	Stage string `json:"stage"`
	// QueryText carries a buyer refinement to the next category stage.
	QueryText        string `json:"queryText,omitempty"`
	ParentCategoryID string `json:"parentCategoryId,omitempty"`
	ParentLabel      string `json:"parentLabel,omitempty"`
	CategoryID       string `json:"categoryId,omitempty"`
	Label            string `json:"label,omitempty"`
}

// ConfirmedCategory returns the deepest buyer-confirmed category. A failed
// child lookup or manual escape at ITEM still retains the confirmed parent.
func (s *CropSelection) ConfirmedCategory() (string, string) {
	if s == nil {
		return "", ""
	}
	if s.Stage == CropSelectionDescribe && s.CategoryID != "" {
		return s.CategoryID, s.Label
	}
	if s.Stage == CropSelectionItem || s.Stage == CropSelectionDescribe {
		return s.ParentCategoryID, s.ParentLabel
	}
	return "", ""
}
