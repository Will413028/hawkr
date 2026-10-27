package model

// CardStatus 是店家卡片上的營運狀態（訂單、聊天、收藏、追蹤這類「參照卡片」的呈現）。
type CardStatus string

const (
	// CardOpen 營業中：此刻可以公開曝光（Exposable），所以有公開的店面頁、可以被追蹤。
	CardOpen CardStatus = "OPEN"
	// CardPaused 暫停：沒有關閉，但此刻不可公開曝光（停權、等待刪除、PayUni 被拒絕、身分認證改回草稿、缺店名、logo 或所在地）。
	CardPaused CardStatus = "PAUSED"
	// CardClosed 已關閉。
	CardClosed CardStatus = "CLOSED"
)

// CardStatusValues 回傳全部卡片狀態。
func CardStatusValues() []CardStatus { return []CardStatus{CardOpen, CardPaused, CardClosed} }

// CardStatusOf 由生命週期與「此刻是否可公開曝光」（seller.pub_exposable_stores，唯一的規則）推導卡片狀態（唯一的推導點）。
func CardStatusOf(status Status, exposable bool) CardStatus {
	switch {
	case status == Closed:
		return CardClosed
	case exposable:
		return CardOpen
	}
	return CardPaused
}

// StoreCard 是曾經公開過（first_exposed_at 有值或此刻可曝光）的店家的卡片資料（ADR 2026-10-03-public-store-identity-and-visibility）：店名、logo、封面與
// 營運狀態與所在地（內政部 8 碼鄉鎮市區代碼）。個資清除之後 Name 與 TownCode 是 nil（town_code 在個資清除時設成 NULL）；關閉的店家沒有目前的 logo。
type StoreCard struct {
	StoreID, StoreCode int64
	Name               *string
	TownCode           *string
	LogoKey, CoverKey  *string
	Status             CardStatus
}
