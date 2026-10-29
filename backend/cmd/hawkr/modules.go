package main

import (
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
)

// modules 列出單體裡有自己資料的模組與它的 migration，順序就是 migrate 套用的順序。
//
// 模組搬進單體時加在這裡。TestEveryModuleIsRegistered 保證 internal/modules 底下的
// 每個模組都在清單內（或在 withoutData、apiOnly 裡）：漏登記的模組，它的 schema 在部署時不會被建立。
func modules() []migrate.Module {
	return []migrate.Module{
		{Name: "content", Dir: content.Migrations()},
		{Name: "identity", Dir: identity.Migrations()},
		{Name: "delegation", Dir: delegation.Migrations()},
		{Name: "buyer", Dir: buyer.Migrations()},
		{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		{Name: "seller", Dir: seller.Migrations()},
		{Name: "agent", Dir: agent.Migrations()},
		{Name: "discovery", Dir: discovery.Migrations()},
		{Name: "social", Dir: social.Migrations()},
	}
}

// apiOnly 是只有公開 API、實作還在舊服務的模組：由 internal/legacy 的暫時實作在組裝點
// 接線，沒有自己的 schema。實作搬進單體時，從這裡移到 modules()。
var apiOnly = []string{
	"media", // 遷移計畫 B1 改寫之前，實作是 Java 的 media-service
}

// withoutData 是已在單體裡、但沒有自己資料的模組：不建 schema、沒有 migration（backend/AGENTS.md 規則 6），
// sqlc 只讀依賴表允許的 pub.sql。第一次要存自己的資料時，建 schema 與 migration，從這裡移到 modules()。
var withoutData = []string{
	"reviewqueue",       // 待辦由各來源模組的資料推導，讀 seller 發布的 pub_review_items
	"accountsuspension", // 停權紀錄在 identity，這個模組只編排：鎖帳號、確認沒有店家、呼叫 identity、發事件
}
