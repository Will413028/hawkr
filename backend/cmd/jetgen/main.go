// Command jetgen 生成用 go-jet 建 query 的模組所需的兩種程式碼（ADR 2026-09-27-module-internal-structure 的修訂）。這是開發工具，不部署。
//
//	jetgen tables -module social -out internal/modules/social/internal/jet
//	jetgen pub -module social -out internal/modules/social/internal/pubsql \
//	    -use seller:pub_exposable_stores
//
// tables 起一個暫時的 Postgres container（版本同 testdb.Image），只套用該模組自己的 migration，再對模組自己的 schema 跑 jet 的生成器：
// 生成的表型別只有模組自己的表，query 因此無法用型別參照別的模組的表（跨模組的唯一入口是 pub 包裝）。
// pub 讀擁有者的 db/pub.sql（契約檔）生成每個 pub_* 函式的型別化包裝；原始 SQL 只允許出現在這些生成檔
// （internal/modules/<模組>/internal/reads 的 AST 測試擋其他地方）。
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "jetgen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法：jetgen tables|pub -module <模組> -out <目錄> [-use <擁有者>:<pub 函式>]")
	}
	switch args[0] {
	case "tables":
		return runTables(args[1:])
	case "pub":
		return runPub(args[1:])
	default:
		return fmt.Errorf("不認得的子命令 %q（tables 或 pub）", args[0])
	}
}
