-- issue #3149：media 從不儲存使用者上傳的原始檔名
--
-- 設計書：08 §4.9
--
-- 在此之前 CreateUploadSession 的 filename 只被 generateObjectKey 拿去取副檔名
-- （AbstractStorageService.sanitizedExtension），建立 upload_session 時即丟棄；
-- media_asset 也沒有任何檔名欄。於是讀取端只能拿 object_key 尾段當檔名，而該尾段是
-- {uuid}{ext} —— 機敏上傳自 #2759 起更是 {uuid}{ext}.attempt-{uuid}。實際後果：賣家
-- 上傳三份稅籍證明，賣家中心（S02）與 ADM 稅務審核（A03）都顯示成三串無法分辨的 uuid。
--
-- 兩欄都是**顯示用的補充資料，不是定址鍵**：
--   object_key 仍是唯一的定址與去重依據（uk_media_asset_object_key，V7），
--   本次不動它的生成規則 —— 改它會讓既有物件與新 key 規則不一致，而那個風險
--   與本次要解決的問題無關（08 §4.9.3〈作用範圍〉）。
--
-- 🔴 為什麼 nullable 且**不回填**（與 §11.2「新增可推導欄位要回填」的分野）：
--   既有列的原始檔名**真的不存在**，不是「存了沒讀」—— R2 的物件 metadata 也沒有
--   （R2StorageService 的兩處 PutObjectRequest 只設 contentType，無 .metadata(...)）。
--   ⇒ 無從推導，回填不了。NULL 是合法終態，讀取端以 AssetFilename.resolveDisplayName
--   回退到 object_key 尾段（並剝去 .attempt-{uuid} 後綴）。
--   ⚠️ 這與 V8 的 gc_lease_until 是不同的「不回填」：那個 NULL 帶業務語意（可搶佔），
--   這個 NULL 的語意是「這筆資料從來就沒有被記錄過」。
--
-- 長度 255：對齊既有先例 product_import_job.source_filename。PostgreSQL 的 VARCHAR(n)
--   計**字元**不計位元組，應用層的截斷口徑必須一致（08 §4.9.3；特別是不得誤用
--   InputValidator.truncateByDisplayLength —— 那支算的是顯示寬度，CJK 記 2）。
--
-- 無索引：本欄只被讀出來顯示，沒有任何查詢以它為條件。

-- ==================== 上傳會話：原始檔名的入口 ====================
ALTER TABLE upload_session
    ADD COLUMN original_filename VARCHAR(255);

COMMENT ON COLUMN upload_session.original_filename IS
    'issue #3149：使用者上傳時宣告的原始檔名，建立 session 時經 AssetFilename.sanitize 消毒後寫入。NULL = V9 之前的列，或消毒後為空。';

-- ==================== 資源資產：Finalize 時複製過來 ====================
ALTER TABLE media_asset
    ADD COLUMN original_filename VARCHAR(255);

COMMENT ON COLUMN media_asset.original_filename IS
    'issue #3149：原始檔名（顯示用，非定址鍵）。一般上傳於 Finalize 時由 upload_session 複製；後端直傳取 RPC 參數；商品匯入依 source_type 解析（08 §4.9.2）。NULL 時讀取端回退至 object_key 尾段。';
