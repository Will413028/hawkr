// Package assistant 是 assistant 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package assistant

import (
	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/feature"
)

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
