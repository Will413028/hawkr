package dto

// SellerTarget names the stint a seller write is aimed at.
//
// account_id 與 store_id 是 44 號身分模型裡的兩根不同軸：account_id 是登入
// principal（在 sellers 裡非唯一，re-onboard 後一個帳號有多列），store_id 是營運鍵
// 兼 sellers PK。**首任賣家兩者相等**，所以「把 store_id 當 account_id 用」在任何
// grandfather 資料上都測得過 —— 這個型別存在的唯一理由，就是讓呼叫端無法不表態
// 自己手上拿的是哪一根軸。
//
// 欄位刻意不可匯出：外部套件只能經由兩個具名建構子建值，寫下建構子名稱就是表態。
type SellerTarget struct {
	accountID int64
	storeID   int64
}

// TargetByAccountID resolves the account's current stint — the seller's own
// paths (/app/sellers/me and friends), where the id comes from the JWT.
func TargetByAccountID(accountID int64) SellerTarget {
	return SellerTarget{accountID: accountID}
}

// TargetByStoreID pins one stint by its PK — the delegated paths, whose target
// store is named by the admin session, not derived from a login identity.
func TargetByStoreID(storeID int64) SellerTarget {
	return SellerTarget{storeID: storeID}
}

// AccountID reports the account axis (0 when the target is store-scoped).
func (t SellerTarget) AccountID() int64 { return t.accountID }

// StoreID reports the store axis (0 when the target is account-scoped).
func (t SellerTarget) StoreID() int64 { return t.storeID }

// IsZero reports the zero value, which names nothing and must never reach a
// query — a resolver that treated it as "any row" would be a cross-store write.
func (t SellerTarget) IsZero() bool { return t.accountID <= 0 && t.storeID <= 0 }

type UpdateSellerProfile struct {
	StoreName     string
	Description   string
	ContactPhone  string
	City          string
	District      string
	AddressDetail string
	Birthday      string
	Gender        string
	Prize         string
	WebsiteURL    string
	// Target names the row to update. It replaced a bare AccountID field: the
	// delegated path was filling that field with a store_id, and the write
	// (WHERE account_id = ?) landed on a different seller entirely while
	// returning code: 0.
	Target SellerTarget
	// ExpectedVersion is nil on the seller's own path and set on a delegated
	// write, where a mismatch must fail with 9266 rather than overwrite (56 §5.4).
	ExpectedVersion *int64
}
