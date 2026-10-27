package dto

// TransformResponse 是 POST /ai/image/transform 的回應
type TransformResponse struct {
	ImageBase64 string `json:"imageBase64"`
	MimeType    string `json:"mimeType"`
	WeeklyUsage string `json:"weeklyUsage"`
	WeeklyLimit string `json:"weeklyLimit"`
}
