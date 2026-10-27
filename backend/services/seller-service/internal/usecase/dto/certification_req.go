package dto

type CreateCertificationDraft struct {
	SellerType string
	AccountID  int64
}
type SubmitApplication struct {
	CompanyRegisterDate  string
	CompanyEmail         string
	RealName             string
	IDCardNo             string
	BirthDate            string
	IDCardIssueDate      string
	IDCardIssueType      string
	IDCardIssueLocation  string
	EnglishName          string
	IDCardFrontAssetID   string
	IDCardBackAssetID    string
	BankBookAssetID      string
	SellerType           string
	ProductType          string
	CompanyPhone         string
	CompanyType          string
	CompanyName          string
	TaxID                string
	BankAccount          string
	BankCode             string
	CompanyCity          string
	CompanyDistrict      string
	CompanyAddressDetail string
	BankName             string
	DocumentAssetIDs     []string
	ID                   int64
	IsProcessedFood      bool
	FoodBusinessRegNo    string
}
type SaveProgress struct {
	StepData      string
	ApplicationID int64
	CurrentStep   int
}
