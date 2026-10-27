-- Modify "certification_applications" table
-- #2073: bank_account 改為 AES-256-GCM Base64 密文存放（與 id_card_no 同一把 key、
-- 同一組 repo↔domain choke point）。密文長度 = Base64(12 nonce + N + 16 tag)，
-- 14 字帳號約 56 字，原 varchar(50) 會直接 22001 寫入失敗，故必須先放寬欄位。
--
-- expand 階段，可單獨部署：放寬長度不影響既有明文的讀寫，
-- 加密寫入與存量 backfill 是後續獨立步驟。
ALTER TABLE "certification_applications" ALTER COLUMN "bank_account" TYPE character varying(255);
