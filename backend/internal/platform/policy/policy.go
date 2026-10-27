// Package policy 是平台的政策分類（step10e-reports-brief R10）：檢舉人選的類別、平台判定類別與認定的類別共用這一份，
// 取代 seller 自己的違規類別。每個值對應賣家合作條款的依據，給賣家的理由要能指到條款。
//
// 各入口只接受自己的子集，伺服器照樣驗證（C3：client 的常數只是呈現順序）：
//   - ReporterSelectable：店家治理的檢舉人可以選的類別，除了 PlatformAbuse；
//   - Assessable：店家治理的平台判定類別與認定（含案件的類別）可用的類別，除了 Other（Other 只給檢舉人選，理由要指到條款，
//     所以分案時要改判成有依據的類別，或直接設定結果）；
//   - CommunityAssessable：社群內容（買家寫的留言、討論區留言）的隱藏與警告可用的類別：社群類別 Community 加上對買家言論成立的類別，
//     不含只對商品或賣家成立的類別（FoodSafety、ProhibitedGoods、UnlicensedSale、FakeCertification、Counterfeit）與 Other。
//
// 社群類別（Spam、Harassment、Hate）還不屬於 ReporterSelectable 與 Assessable：店家治理的資料表（reports、cases、violations）
// 的 CHECK 沒有它們，seller 的子集由 A7-1c 決定時再加入。
//
// 值與契約 api/hawkr/common/v1alpha1/policy_category.proto 的 PolicyCategory 一一對應（proto 的值名去掉
// POLICY_CATEGORY_ 前綴）；儲存的代碼是 UPPER_SNAKE，各表的 CHECK 對自己的子集（模組的 TestChecksMatchGo）。platform 不得依賴任何模組。
package policy

import "slices"

// Category 是政策分類的值。
type Category string

const (
	// FoodSafety 有健康風險或疑似食物中毒，或未揭露安全疑慮的商品（條款第十一條、第十六條第 1 款）。
	FoodSafety Category = "FOOD_SAFETY"
	// ProhibitedGoods 違法、違禁或來源不明，而沒有健康風險的商品（第十六條第 1 款）。
	ProhibitedGoods Category = "PROHIBITED_GOODS"
	// UnlicensedSale 依法應有的登錄、許可或資格沒有完成就販售（第十一條第 2 項）。
	UnlicensedSale Category = "UNLICENSED_SALE"
	// MisleadingInfo 虛假資料、標示不實或誤導的商品或店家資訊（第十六條第 2 款）。
	MisleadingInfo Category = "MISLEADING_INFO"
	// FakeCertification 不實的認證、檢驗或標章（第十六條第 2 款的一種）。
	FakeCertification Category = "FAKE_CERTIFICATION"
	// Counterfeit 冒用他人商標或外觀的商品本身（第十六條第 1 款）。
	Counterfeit Category = "COUNTERFEIT"
	// IPInfringement 侵害他人商標、著作權、專利、營業秘密、肖像或其他權利的使用（第十六條第 5 款）；仿冒商品本身是 Counterfeit。
	IPInfringement Category = "IP_INFRINGEMENT"
	// Fraud 偽造交易、洗錢、詐欺、濫用優惠、操縱評價或規避風險控管（第十六條第 2 款）。
	Fraud Category = "FRAUD"
	// OffPlatformTrade 引導買家繞過平台付款，或規避平台服務費（第十六條第 3 款）。
	OffPlatformTrade Category = "OFF_PLATFORM_TRADE"
	// Privacy 未經授權蒐集、利用、揭露或出售買家個人資料（第十六條第 4 款）。
	Privacy Category = "PRIVACY"
	// Impersonation 冒用平台名義、未經授權的保證，或讓買家誤認交易主體（第十六條第 8 款）。
	Impersonation Category = "IMPERSONATION"
	// PlatformAbuse 干擾、探測或破壞平台與資訊安全，或以爬蟲、自動化不當取得資料（第十六條第 6、7 款）；檢舉人不能選。
	PlatformAbuse Category = "PLATFORM_ABUSE"
	// Other 檢舉人無法歸類時選；平台判定類別與認定不能用。
	Other Category = "OTHER"
	// Spam 社群內容的垃圾訊息：重複、無關或以廣告洗版。
	Spam Category = "SPAM"
	// Harassment 社群內容的騷擾或霸凌：針對特定的人或店家的辱罵、威脅或持續打擾。
	Harassment Category = "HARASSMENT"
	// Hate 社群內容的仇恨或歧視言論：以身分、族群或特徵攻擊他人。
	Hate Category = "HATE"
)

// Values 回傳全部類別，順序同契約的數字；每次回傳新的切片，呼叫端可以自由改動。
func Values() []Category {
	return []Category{
		FoodSafety, ProhibitedGoods, UnlicensedSale, MisleadingInfo, FakeCertification, Counterfeit, IPInfringement,
		Fraud, OffPlatformTrade, Privacy, Impersonation, PlatformAbuse, Other, Spam, Harassment, Hate,
	}
}

// Community 回傳社群內容的類別（Spam、Harassment、Hate）。
func Community() []Category { return []Category{Spam, Harassment, Hate} }

// ReporterSelectable 回傳店家治理的檢舉人可以選的類別（全部除了 PlatformAbuse 與社群類別），與 reports.category 的 CHECK 一致。
func ReporterSelectable() []Category {
	return without(without(Values(), PlatformAbuse), Community()...)
}

// Assessable 回傳店家治理的平台判定類別與認定可用的類別（全部除了 Other 與社群類別），與 violations.category、cases.category 的 CHECK 一致。
func Assessable() []Category { return without(without(Values(), Other), Community()...) }

// CommunityAssessable 回傳社群內容的隱藏與警告可用的類別，與 social.hides、social.sanctions 的 CHECK 一致：社群類別（Spam、
// Harassment、Hate）加上對買家言論成立的類別（MisleadingInfo、IPInfringement、Fraud、OffPlatformTrade、Privacy、Impersonation、
// PlatformAbuse）。商品與賣家才成立的類別（FoodSafety、ProhibitedGoods、UnlicensedSale、FakeCertification、Counterfeit）與 Other 不在
// 其中。明列允許的類別而不是排除：新增的類別預設不能用在社群內容，要加進來是一次明確的決定（TestCommunityAssessableIsExplicit）。
func CommunityAssessable() []Category {
	return []Category{
		MisleadingInfo, IPInfringement, Fraud, OffPlatformTrade, Privacy, Impersonation, PlatformAbuse,
		Spam, Harassment, Hate,
	}
}

// ReporterSelectable 說明店家治理的檢舉人可不可以選這個類別。
func (c Category) ReporterSelectable() bool { return c != PlatformAbuse && !c.Community() && c.valid() }

// Assessable 說明店家治理的平台判定與認定可不可以用這個類別。
func (c Category) Assessable() bool { return c != Other && !c.Community() && c.valid() }

// CommunityAssessable 說明社群內容的隱藏與警告可不可以用這個類別。
func (c Category) CommunityAssessable() bool { return slices.Contains(CommunityAssessable(), c) }

// Community 說明這是不是社群內容的類別。
func (c Category) Community() bool { return slices.Contains(Community(), c) }

func (c Category) valid() bool { return slices.Contains(Values(), c) }

func without(values []Category, excluded ...Category) []Category {
	return slices.DeleteFunc(values, func(c Category) bool { return slices.Contains(excluded, c) })
}
