# 開發時產生 migration（只在本機，部署與 CI 不需要 Atlas CLI）：
#
#   cd internal/modules/seller/db && atlas migrate diff <名稱> --env dev
#
# dev database 由 Atlas 起一個暫時的 Postgres container，版本與 platform/testdb 相同。
# 產生的 migration 由 `hawkr migrate` 套用（internal/platform/migrate）。
# src 只有 schema.sql（表、約束與索引）：objects.sql 的 view、函式與複合型別 Atlas 社群版不處理，改它們時以
# `atlas migrate new <名稱> --env dev` 手寫 migration 再 `atlas migrate hash --env dev`（單體規則 6）。
env "dev" {
  src = "file://schema.sql"
  dev = "docker://postgres/18-alpine/dev"
  migration {
    dir = "file://migrations"
  }
}
