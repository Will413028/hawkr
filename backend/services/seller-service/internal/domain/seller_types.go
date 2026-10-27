package domain

//go:generate enumer -type=SellerType -trimprefix=SellerType -json -text -transform=snake --output=zzz_enumer_sellerType.go
type SellerType int

const (
	SellerTypeIndividual SellerType = iota
	SellerTypeBusiness
)

//go:generate enumer -type=PayuniMerStatus -trimprefix=PayuniMerStatus -json -text --output=zzz_enumer_payuniMerStatus.go
type PayuniMerStatus int

const (
	PayuniMerStatusUnregistered PayuniMerStatus = 0  // 未註冊 PAYUNi
	PayuniMerStatusActive       PayuniMerStatus = 1  // 啟用
	PayuniMerStatusClosed       PayuniMerStatus = 2  // 關閉
	PayuniMerStatusDenied       PayuniMerStatus = 8  // 拒絕
	PayuniMerStatusUnderReview  PayuniMerStatus = 9  // 審核中
	PayuniMerStatusPendingInfo  PayuniMerStatus = 91 // 待補資料
	PayuniMerStatusRejected     PayuniMerStatus = 92 // 審核失敗
	PayuniMerStatusOtherClosed  PayuniMerStatus = 99 // 其它關閉
)
