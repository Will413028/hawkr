package dto

import "time"

type AdminSellerItem struct {
	CertifiedAt                *time.Time
	CurrentCertificationID     *int64
	UpdatedAt                  time.Time
	CreatedAt                  time.Time
	ApprovalStatus             string
	City                       string
	District                   string
	AddressDetail              string
	PostalCode                 string
	SellerType                 string
	ContactPhone               string
	Email                      string
	Logo                       string
	StoreName                  string
	Description                string
	CoverKey                   string
	OverallCertificationStatus string
	SeoTitle                   string
	SeoDescription             string
	// MerStatus/MemberStatus 為 PayUni 原碼直傳，取代單一 collapsed PayuniStatus
	// 值成為 admin client 的 canonical 顯示來源（issue #2806 Phase 1）。
	MerStatus    string
	MemberStatus string
	// LifecycleStatus 為 sellers.status 生命週期值（active / suspended /
	// suspended_governance / closed），admin 停權 badge / 篩選用。
	LifecycleStatus string
	AccountID       int64
	StoreID         int64
	StoreCode       int64
	PayuniStatus    int
	Rating          float64
}

type SellerStats struct {
	Total      int64
	Pending    int64
	Approved   int64
	Rejected   int64
	MonthlyNew int64
}

// AdminSellerListResponse 不含 Stats：統計是全站聚合、不隨分頁改變，走獨立
// RPC GetSellerStatsForAdmin。
type AdminSellerListResponse struct {
	List  []*AdminSellerItem
	Total int64
	Page  int32
	Limit int32
}
