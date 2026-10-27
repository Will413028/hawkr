package domain

// CustomerType 客戶分類
type CustomerType string

const (
	CustomerTypeNormal    CustomerType = "normal"
	CustomerTypeVIP       CustomerType = "vip"
	CustomerTypeBlocklist CustomerType = "blocklist"
)
