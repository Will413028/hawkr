// Package assistant 是 assistant 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package assistant

import (
	"embed"
	"io/fs"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/feature"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct{}

// Module 是 assistant 模組。
type Module struct {
	feature *feature.Service
}

// New 建立模組。
func New(_ Deps) *Module {
	return &Module{feature: feature.New(feature.Deps{})}
}
