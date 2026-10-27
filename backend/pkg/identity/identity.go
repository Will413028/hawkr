// Package identity 定義跨服務共用的 identity ID 型別，區分「登入身份」與
// 「業務代理鍵」——兩者平常相等，但會在帳號軟刪重申請、店鋪重開等生命週期
// 事件後分岔。裸 int64 讓型別系統看不出這三者的差別，是這個 bug class 的根因之一。
package identity

// AccountID is the JWT/session login identity — stable for the account's
// lifetime, unaffected by buyer/seller role stint rotation.
type AccountID int64

// BuyerID is the resolved current buyer-stint surrogate — distinct from
// AccountID because it rotates across buyer re-key events within the same
// login session.
type BuyerID int64

// StoreID is the resolved current store surrogate — distinct from AccountID
// because it rotates across store re-key events (delete + re-apply) within
// the same login session.
type StoreID int64
