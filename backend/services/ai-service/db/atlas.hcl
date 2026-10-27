variable "DB_PASSWORD" {
  type    = string
  default = getenv("DB_PASSWORD")
}

variable "DB_USER" {
  type    = string
  default = getenv("DB_USER")
}

variable "DB_HOST" {
  type    = string
  default = getenv("DB_HOST")
}

env "dev" {
  dev = "docker://postgres/18/dev?search_path=public"
  # 目錄形式：既有 schema.sql 與新增的 procurement.sql 並存。
  # procurement 域刻意獨立成一檔且不與 AI 表建外鍵，未來抽成獨立服務時可整檔搬走。
  src = "file://schema"

  # Target Database: hawkr_ai
  url = "postgres://dbadmin:${var.DB_PASSWORD}@${var.DB_HOST}:5432/hawkr_ai?search_path=public&sslmode=disable"

  migration {
    dir = "file://migrations"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

env "prod" {
  dev = "docker://postgres/18/dev?search_path=public"
  # 目錄形式：既有 schema.sql 與新增的 procurement.sql 並存。
  # procurement 域刻意獨立成一檔且不與 AI 表建外鍵，未來抽成獨立服務時可整檔搬走。
  src = "file://schema"

  # Target Database: hawkr_ai
  url = "postgres://${var.DB_USER}:${var.DB_PASSWORD}@${var.DB_HOST}:5432/hawkr_ai?search_path=public&sslmode=require"

  migration {
    dir = "file://migrations"
    revisions_schema = "atlas_schema_revisions"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
