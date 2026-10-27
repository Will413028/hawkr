-- Create "prospect_contacts" table
CREATE TABLE "seller"."prospect_contacts" (
  "prospect_id" bigint NOT NULL,
  "kind" text NOT NULL,
  "value" text NOT NULL,
  "match_key" text NULL,
  "is_primary" boolean NOT NULL,
  "primary_marker" boolean NULL GENERATED ALWAYS AS (
CASE
    WHEN is_primary THEN true
    ELSE NULL::boolean
END) STORED,
  "has_contact" boolean NULL GENERATED ALWAYS AS (true) STORED,
  PRIMARY KEY ("prospect_id", "kind", "value"),
  CONSTRAINT "prospect_contacts_primary" UNIQUE ("prospect_id", "kind", "primary_marker"),
  CONSTRAINT "prospect_contacts_has_primary" FOREIGN KEY ("prospect_id", "kind", "has_contact") REFERENCES "seller"."prospect_contacts" ("prospect_id", "kind", "primary_marker") ON UPDATE NO ACTION ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED,
  CONSTRAINT "prospect_contacts_prospect_id_fkey" FOREIGN KEY ("prospect_id") REFERENCES "seller"."prospects" ("prospect_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "prospect_contacts_kind_check" CHECK (kind = ANY (ARRAY['EMAIL'::text, 'PHONE'::text, 'WEBSITE'::text, 'SOCIAL'::text])),
  CONSTRAINT "prospect_contacts_match_key_check" CHECK ((match_key IS NULL) OR ((char_length(match_key) <= 600) AND
CASE kind
    WHEN 'EMAIL'::text THEN (match_key ~ '^EMAIL:.+$'::text)
    WHEN 'PHONE'::text THEN (match_key ~ '^PHONE:.+$'::text)
    WHEN 'WEBSITE'::text THEN (match_key ~ '^(DOMAIN|SOCIAL):.+$'::text)
    ELSE (match_key ~ '^SOCIAL:.+$'::text)
END)),
  CONSTRAINT "prospect_contacts_value_check" CHECK ((value = btrim(value)) AND (char_length(value) >= 1) AND (char_length(value) <=
CASE kind
    WHEN 'EMAIL'::text THEN 255
    WHEN 'PHONE'::text THEN 50
    ELSE 500
END))
);
-- Create index "prospect_contacts_match_key" to table: "prospect_contacts"
CREATE INDEX "prospect_contacts_match_key" ON "seller"."prospect_contacts" ("match_key", "prospect_id") WHERE (match_key IS NOT NULL);
-- Create "prospect_distinct_pairs" table
CREATE TABLE "seller"."prospect_distinct_pairs" (
  "low_prospect_id" bigint NOT NULL,
  "high_prospect_id" bigint NOT NULL,
  "confirmed_by_admin_id" bigint NOT NULL,
  "confirmed_at" timestamptz NOT NULL,
  PRIMARY KEY ("low_prospect_id", "high_prospect_id"),
  CONSTRAINT "prospect_distinct_pairs_high_prospect_id_fkey" FOREIGN KEY ("high_prospect_id") REFERENCES "seller"."prospects" ("prospect_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "prospect_distinct_pairs_low_prospect_id_fkey" FOREIGN KEY ("low_prospect_id") REFERENCES "seller"."prospects" ("prospect_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "prospect_distinct_pairs_admin_check" CHECK (confirmed_by_admin_id > 0),
  CONSTRAINT "prospect_distinct_pairs_order_check" CHECK (low_prospect_id < high_prospect_id)
);
-- Create index "prospect_distinct_pairs_high" to table: "prospect_distinct_pairs"
CREATE INDEX "prospect_distinct_pairs_high" ON "seller"."prospect_distinct_pairs" ("high_prospect_id");
-- Backfill：既有業者的 public_* 成為各類型的主要聯絡方式，prospect_identifiers 裡沒有被它們認領的鍵（合併帶來的）成為非主要的
-- 聯絡方式。候選的比對鍵用與 model.ContactMatchKey 相同的規則在 SQL 裡算，而且只在候選真的存在於 prospect_identifiers 時才採用，
-- 所以這裡的近似不會產生新的鍵；共用平台（ADR D6）的網域鍵不帶入。
WITH shared(host) AS (
  VALUES ('linktr.ee'), ('pinkoi.com')
), social(host, platform) AS (
  VALUES ('threads.com', 'threads'), ('threads.net', 'threads'), ('facebook.com', 'facebook'), ('m.facebook.com', 'facebook'),
         ('fb.com', 'facebook'), ('instagram.com', 'instagram'), ('line.me', 'line'), ('shopee.tw', 'shopee')
), fields AS (
  SELECT prospect_id, 'EMAIL' AS kind, btrim(public_email) AS value FROM "seller"."prospects" WHERE btrim(public_email) <> ''
  UNION ALL SELECT prospect_id, 'PHONE', btrim(public_phone) FROM "seller"."prospects" WHERE btrim(public_phone) <> ''
  UNION ALL SELECT prospect_id, 'WEBSITE', btrim(public_website) FROM "seller"."prospects" WHERE btrim(public_website) <> ''
  UNION ALL SELECT prospect_id, 'SOCIAL', btrim(public_social) FROM "seller"."prospects" WHERE btrim(public_social) <> ''
), parsed AS (
  SELECT f.prospect_id, f.kind, f.value,
         lower(f.value) AS v,
         regexp_replace(substring(lower(f.value) FROM '^(?:[a-z][a-z0-9+.-]*://)?([^/?#]*)'), '^www\.', '') AS host,
         btrim(substring(lower(f.value) FROM '^(?:[a-z][a-z0-9+.-]*://)?[^/?#]*([^?#]*)'), '/') AS segment
  FROM fields f
), candidates AS (
  SELECT p.prospect_id, p.kind, p.value,
         CASE
           WHEN p.kind = 'EMAIL' THEN 'EMAIL:' || p.v
           WHEN p.kind = 'PHONE' THEN 'PHONE:' || CASE
             WHEN regexp_replace(p.v, '[^0-9]', '', 'g') LIKE '886%' THEN '0' || substr(regexp_replace(p.v, '[^0-9]', '', 'g'), 4)
             ELSE regexp_replace(p.v, '[^0-9]', '', 'g') END
           WHEN s.platform IS NOT NULL THEN CASE
             WHEN p.segment = '' OR position('/' IN p.segment) > 0 THEN NULL
             WHEN s.platform = 'facebook' AND p.segment = 'profile.php' THEN 'SOCIAL:facebook:' || substring(p.v FROM '[?&]id=([^&#]+)')
             WHEN regexp_replace(p.segment, '^@', '') IN ('', 'p', 'reel', 't', 'pages', 'groups', 'watch', 'explore', 'share', 'stories', 'profile.php') THEN NULL
             ELSE 'SOCIAL:' || s.platform || ':' || regexp_replace(p.segment, '^@', '')
           END
           WHEN p.kind = 'WEBSITE' AND NOT EXISTS (SELECT 1 FROM shared h WHERE p.host = h.host OR p.host LIKE '%.' || h.host) THEN 'DOMAIN:' || p.host
         END AS candidate
  FROM parsed p LEFT JOIN social s ON s.host = p.host AND p.kind IN ('WEBSITE', 'SOCIAL')
)
INSERT INTO "seller"."prospect_contacts" ("prospect_id", "kind", "value", "match_key", "is_primary")
SELECT c.prospect_id, c.kind, c.value,
       (SELECT i.kind || ':' || i.value_normalized FROM "seller"."prospect_identifiers" i
        WHERE i.prospect_id = c.prospect_id AND i.kind || ':' || i.value_normalized = c.candidate),
       true
FROM candidates c;
INSERT INTO "seller"."prospect_contacts" ("prospect_id", "kind", "value", "match_key", "is_primary")
SELECT e.prospect_id, e.kind, e.value, e.match_key,
       row_number() OVER (PARTITION BY e.prospect_id, e.kind ORDER BY e.value) = 1
         AND NOT EXISTS (SELECT 1 FROM "seller"."prospect_contacts" c WHERE c.prospect_id = e.prospect_id AND c.kind = e.kind AND c.is_primary)
FROM (
  SELECT i.prospect_id,
         CASE i.kind WHEN 'DOMAIN' THEN 'WEBSITE' ELSE i.kind END AS kind,
         CASE i.kind
           WHEN 'SOCIAL' THEN CASE split_part(i.value_normalized, ':', 1)
             WHEN 'threads' THEN 'https://www.threads.com/@'
             WHEN 'facebook' THEN 'https://www.facebook.com/'
             WHEN 'instagram' THEN 'https://www.instagram.com/'
             WHEN 'line' THEN 'https://line.me/@'
             ELSE 'https://shopee.tw/'
           END || substr(i.value_normalized, strpos(i.value_normalized, ':') + 1)
           ELSE i.value_normalized
         END AS value,
         i.kind || ':' || i.value_normalized AS match_key
  FROM "seller"."prospect_identifiers" i
  WHERE NOT EXISTS (
          SELECT 1 FROM "seller"."prospect_contacts" c
          WHERE c.prospect_id = i.prospect_id AND c.match_key = i.kind || ':' || i.value_normalized)
    AND NOT (i.kind = 'DOMAIN' AND (i.value_normalized IN ('linktr.ee', 'pinkoi.com') OR i.value_normalized LIKE '%.linktr.ee' OR i.value_normalized LIKE '%.pinkoi.com'))
) e
ON CONFLICT ("prospect_id", "kind", "value") DO UPDATE SET "match_key" = COALESCE("seller"."prospect_contacts"."match_key", EXCLUDED."match_key");
-- Rename audit columns on "prospects" table
ALTER TABLE "seller"."prospects" RENAME COLUMN "created_by_account_id" TO "created_by_admin_id";
ALTER TABLE "seller"."prospects" RENAME COLUMN "updated_by_account_id" TO "updated_by_admin_id";
-- Drop "prospect_identifiers" table
DROP TABLE "seller"."prospect_identifiers";
-- 圖片已清掉（封存、合併）的列不留來源網址。
UPDATE "seller"."prospects" SET "logo_source_url" = NULL WHERE "logo_key" IS NULL;
UPDATE "seller"."prospects" SET "cover_source_url" = NULL WHERE "cover_key" IS NULL;
-- Modify "prospects" table
ALTER TABLE "seller"."prospects" DROP CONSTRAINT "prospects_live_key", DROP CONSTRAINT "prospects_text_check", ADD CONSTRAINT "prospects_text_check" CHECK ((char_length(category) >= 1) AND (char_length(category) <= 100) AND (char_length(region) >= 1) AND (char_length(region) <= 100) AND (char_length(product_summary) >= 1) AND (char_length(positioning) >= 1) AND (char_length(research_summary) >= 1)), ADD CONSTRAINT "prospects_admin_check" CHECK ((created_by_admin_id > 0) AND (updated_by_admin_id > 0)), DROP COLUMN "public_email", DROP COLUMN "public_phone", DROP COLUMN "public_website", DROP COLUMN "public_social", ADD CONSTRAINT "prospects_source_urls_check" CHECK (((logo_key IS NOT NULL) OR (logo_source_url IS NULL)) AND ((cover_key IS NOT NULL) OR (cover_source_url IS NULL)));
