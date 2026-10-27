-- Create "app_version_gates" table
CREATE TABLE "app_version_gates" (
  "app_type" character varying(16) NOT NULL,
  "platform" character varying(16) NOT NULL,
  "minimum_version" character varying(32) NOT NULL,
  "store_url" text NULL,
  "notice_title" character varying(255) NOT NULL DEFAULT '',
  "notice_body" text NOT NULL DEFAULT '',
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("app_type", "platform"),
  CONSTRAINT "app_version_gates_app_type_check" CHECK ((app_type)::text = ANY ((ARRAY['BUYER'::character varying, 'SELLER'::character varying])::text[])),
  CONSTRAINT "app_version_gates_minimum_version_check" CHECK ((minimum_version)::text ~ '^[0-9]+\.[0-9]+\.[0-9]+$'::text),
  CONSTRAINT "app_version_gates_platform_check" CHECK ((platform)::text = ANY ((ARRAY['IOS'::character varying, 'ANDROID'::character varying])::text[]))
);
