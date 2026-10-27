package dto

type AdminListSellersRequest struct {
	ApprovalStatus string
	Keyword        string
	// LifecycleStatus 篩選 sellers.status（active / suspended /
	// suspended_governance / closed），空字串 = 不篩選。
	LifecycleStatus string
	Page            int32
	Limit           int32
}

type AdminListCertificationsRequest struct {
	Status  string
	Keyword string
	Page    int32
	Limit   int32
}
