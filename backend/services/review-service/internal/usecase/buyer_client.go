package usecase

import "context"

// BuyerMeta 買家輕量資訊
type BuyerMeta struct {
	Nickname  string
	AvatarURL string
	// Deleted = 這個 buyer stint 已註銷。Nickname 此時是墓碑字串（由 buyer-service
	// 回，不是本服務組的），可直接顯示；AvatarURL 為空。
	//
	// 🔴 判斷「是否已註銷」請讀本欄，不要拿 Nickname 去跟墓碑字串做字串比對——
	// 那會在每個顯示端硬編碼一份文案。
	Deleted bool
}

// BuyerClient 從 buyer-service 取得買家資訊（service-to-service）
type BuyerClient interface {
	// BatchGetBuyerDisplayByBuyerIDs 取顯示用的名字（#2830）。
	//
	// 刻意**不用** BatchGetBuyerMetaByBuyerIDs：那支底層帶 `deleted_at IS NULL`
	// （刻意的 PII 邊界），買家註銷後解不出來 ⇒ 公開評論的作者名變成空字串。
	// 本支讓已註銷者也解得出來但只回墓碑，真名不離開 buyer-service 的 DB。
	BatchGetBuyerDisplayByBuyerIDs(ctx context.Context, buyerIDs []int64) (map[int64]*BuyerMeta, error)
}
