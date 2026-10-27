# 開發時產生 migration（只在本機，部署與 CI 不需要 Atlas CLI）：
#
#   cd internal/modules/buyer/db && atlas migrate diff <名稱> --env dev
#
# dev database 由 Atlas 起一個暫時的 Postgres container，版本與 platform/testdb 相同。
# 產生的 migration 由 `hawkr migrate` 套用（internal/platform/migrate）。
env "dev" {
  src = "file://schema.sql"
  dev = "docker://postgres/18-alpine/dev"
  migration {
    dir = "file://migrations"
  }
}
