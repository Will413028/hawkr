package dto

type ReviewApplication struct {
	ReviewerName     string
	Action           string
	Comment          string
	RejectionReason  string
	RequiredFields   []string
	ApplicationID    int64
	ReviewerID       int64
	PayuniWebsiteURL string
}
