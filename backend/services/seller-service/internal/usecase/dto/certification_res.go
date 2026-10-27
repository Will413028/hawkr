package dto

import "time"

type CertificationApplication struct {
	UpdatedAt            time.Time
	CreatedAt            time.Time
	IDCardIssueDate      *time.Time
	RejectedAt           *time.Time
	SubmittedAt          *time.Time
	CompanyRegisterDate  *time.Time
	BirthDate            *time.Time
	CompanyType          string
	CompanyDistrict      string
	IDCardIssueLocation  string
	EnglishName          string
	IDCardFrontAssetID   string
	IDCardBackAssetID    string
	BankBookAssetID      string
	ApplicationNo        string
	BankName             string
	BankCode             string
	BankAccount          string
	Status               string
	CompanyName          string
	TaxID                string
	IDCardNo             string
	CompanyCity          string
	IDCardIssueType      string
	CompanyAddressDetail string
	CompanyEmail         string
	CompanyPhone         string
	ProductType          string
	SellerType           string
	RealName             string
	StepDataJSON         string
	DocumentAssetIDs     []string
	TotalSteps           int
	CurrentStep          int
	ID                   int64
	AccountID            int64
	IsProcessedFood      bool
	FoodBusinessRegNo    string
	PayuniWebsiteURL     string
}

type AdminCertificationItem struct {
	UpdatedAt          time.Time
	CreatedAt          time.Time
	SubmittedAt        *time.Time
	RejectedAt         *time.Time
	SellerType         string
	StoreName          string
	RealName           string
	Status             string
	ApplicationNo      string
	IdCardFrontAssetId string
	IdCardBackAssetId  string
	BankBookAssetId    string
	DocumentAssetIds   []string
	CurrentStep        int
	TotalSteps         int
	ID                 int64
	AccountID          int64
}

type AdminCertificationListResponse struct {
	Stats *CertificationStats
	List  []AdminCertificationItem
	Total int64
	Page  int32
	Limit int32
}

type CertificationStats struct {
	Total     int64
	Submitted int64
	Approved  int64
	Rejected  int64
}

type CertificationReview struct {
	CreatedAt       time.Time
	ReviewerID      *int64
	ReviewerName    string
	Action          string
	Result          string
	Comment         string
	RejectionReason string
	RequiredFields  []string
	ID              int64
	ApplicationID   int64
}
