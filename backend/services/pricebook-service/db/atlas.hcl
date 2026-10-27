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
  src = "file://schema/schema.sql"

  # Target Database: hawkr_pricebook
  url = "postgres://dbadmin:${var.DB_PASSWORD}@${var.DB_HOST}:5432/hawkr_pricebook?search_path=public&sslmode=disable"

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
  src = "file://schema/schema.sql"

  # Target Database: hawkr_pricebook
  url = "postgres://${var.DB_USER}:${var.DB_PASSWORD}@${var.DB_HOST}:5432/hawkr_pricebook?search_path=public&sslmode=require"

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
