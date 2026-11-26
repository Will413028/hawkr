// Command atlas-extensions 印出平台擁有的 extension 的安裝 SQL（internal/platform/migrate 的 Extensions）。
// scripts/atlas-diff.sh 在 Atlas 的 dev database 上套用它，desired state 才能參照 extension 的 operator class；
// 清單只有 migrate 一處宣告。這是開發工具，不部署。
package main

import (
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
)

func main() {
	fmt.Print(migrate.ExtensionsSQL())
}
