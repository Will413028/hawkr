package dto

// AutofillRequest 商品表單預填請求
type AutofillRequest struct {
	Text     string `json:"text"`
	ImageURL string `json:"image_url,omitempty"`
}

// CategoryCandidate 分類候選
type CategoryCandidate struct {
	CatalogID  string `json:"catalogId"`
	Path       string `json:"path"`
	Confidence string `json:"confidence"`
}

// SkuDraft AI 建議的 SKU 草稿
type SkuDraft struct {
	SkuName string `json:"skuName"`
	Price   string `json:"price"`
	Unit    string `json:"unit"`
}

// ProductDraft AI 預填的商品草稿（對外回應格式）
type ProductDraft struct {
	Name               string              `json:"name"`
	Origin             string              `json:"origin,omitempty"`
	Description        string              `json:"description,omitempty"`
	CategoryCandidates []CategoryCandidate `json:"categoryCandidates"`
	Skus               []SkuDraft          `json:"skus"`
	Tags               []string            `json:"tags"`
	Warnings           []string            `json:"warnings"`
}

// AutofillResponse 商品表單預填回應
type AutofillResponse struct {
	Data *ProductDraft `json:"data,omitempty"`
	Msg  string        `json:"msg,omitempty"`
	Code int           `json:"code"`
}

// rawDraft LLM 回傳的內部 JSON 結構（schema 強制型別）
type RawDraft struct {
	Name            string   `json:"name"`
	Origin          string   `json:"origin"`
	CategoryKeyword string   `json:"categoryKeyword"`
	Description     string   `json:"description"`
	Skus            []RawSku `json:"skus"`
	Tags            []string `json:"tags"`
}

// RawSku LLM 回傳的 SKU（price 為 integer）
type RawSku struct {
	SkuName string `json:"skuName"`
	Unit    string `json:"unit"`
	Price   int64  `json:"price"`
}
