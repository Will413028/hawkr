-- issue #2294：多副本排程搶佔（Scheduler Claim）—— media 側新增兩個「租約欄」
--
-- 設計書：08 §8；規範：docs/開發規範/多副本排程搶佔規範.md
--
-- MediaGarbageCollectionJob 的副作用是**外部儲存刪除（不可逆）**，多副本下同一個 objectKey
-- 會被兩個 pod 各刪一次；更嚴重的是舊實作的「整批 markExpiredSessions」語意無法只標記
-- 成功的那幾筆，任一筆失敗就整輪不標記 ⇒ 併發下兩個 pod 互相把對方的成果沖掉。
--
-- 兩欄都是 **租約欄（lease）**，不是「已 GC 旗標」：
--   值 = 目前持有者的租約到期時間；NULL 或已過期 = 可被重新搶佔。
--   NULL 的正確語意就是「從未被搶佔、立即可搶」＝新欄位的預期初始狀態，
--   **不需要也不可以回填**（回填成 NOW() 會讓既有列全部沉默一個租約週期）。
--
-- 為什麼不挪用既有欄位：
--   upload_session.expires_at → **業務有效性判準**（UploadSessionService / AdminMediaController
--                                都拿它擋過期 session）。往後推等於重新打開一個已過期的上傳窗口。
--   upload_session.updated_at → MetaObjectHandler 統一維護，任何無關寫入都會動它。
--   media_asset 無任何時間欄可挪用（只有 created_at，是候選述詞本身的判準）。

-- ==================== 過期 upload_session 清理 ====================
-- 租約長度 = 60 分鐘（cron 每日 03:00 ⇒ 租約 ≪ 週期，失敗的列在**下一次** GC 就會重試）
ALTER TABLE upload_session
    ADD COLUMN gc_lease_until TIMESTAMPTZ;

COMMENT ON COLUMN upload_session.gc_lease_until IS
    'issue #2294 L8：GC 租約到期時間（非「已 GC 旗標」）。NULL 或 <= NOW() 表示可搶佔。';

-- 候選述詞：status IN ('7001','7002') AND expires_at < #{now}
--           AND (gc_lease_until IS NULL OR gc_lease_until <= #{now})
CREATE INDEX idx_upload_session_gc_lease
    ON upload_session (expires_at, gc_lease_until)
    WHERE status IN ('7001', '7002');

-- ==================== 孤兒 media_asset 清理 ====================
ALTER TABLE media_asset
    ADD COLUMN gc_lease_until TIMESTAMPTZ;

COMMENT ON COLUMN media_asset.gc_lease_until IS
    'issue #2294 L8：GC 租約到期時間（非「已 GC 旗標」）。NULL 或 <= NOW() 表示可搶佔。';

-- 候選述詞：無 asset_reference AND created_at < #{threshold}
--           AND (gc_lease_until IS NULL OR gc_lease_until <= #{now})
CREATE INDEX idx_media_asset_gc_lease
    ON media_asset (created_at, gc_lease_until);
