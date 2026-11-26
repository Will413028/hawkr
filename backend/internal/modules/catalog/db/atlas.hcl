# 開發時產生 migration（只在本機，部署與 CI 不需要 Atlas CLI）：
#
#   cd backend && make atlas-diff module=catalog name=<名稱>
#
# 不直接跑 atlas migrate diff：schema.sql 參照 pg_trgm 的 operator class，而 extension 由平台擁有
# （internal/platform/migrate 的 extensions），dev database 要先裝好；make atlas-diff 起暫時的 Postgres container
# （版本與 platform/testdb 相同）、套用同一份 extension 清單再跑 Atlas。
# dev database 由 make atlas-diff 以 --dev-url 帶入，這裡不寫 dev。
# 產生的 migration 由 `hawkr migrate` 套用（internal/platform/migrate）。
env "dev" {
  src = "file://schema.sql"
  migration {
    dir = "file://migrations"
  }
}
