-- 品項解析結果搬進 procurement_request_revisions.slots_json 的 item_resolution
-- （ADR 2026-09-04-crop-resolution-into-slots）。手寫而非 atlas migrate diff 產出：
-- diff 只會生出 DROP COLUMN，而這支 migration 的重點是先把既有值搬過去。

-- 1) 把每個 request 的解析結果併進它「目前 ACTIVE 那一版」的 slots_json。
--
-- 只動 ACTIVE revision：歷史 revision 的 slots 是當時的事實，回頭補寫等於竄改
-- rewind 會還原出來的內容。代價是 rewind 到搬遷前的舊版會拿到沒有解析結果的
-- slots——那是誠實的（那一版當時確實沒有把解析記在 slots 裡），且 item 一變動
-- crop_gate 就會重新解析。
--
-- 只在 slots 已經有 item 時寫入：維持 domain 層「有 ItemResolution 必有 Item」
-- 的不變量（Slots.Apply 會拒絕沒有 item 的解析結果）。
--
-- provenance 一律填 SYSTEM：舊欄位沒有記錄「這個結果是系統猜的還是買家挑的」，
-- 兩種來源在 DB 上完全同形，無法還原。填 SYSTEM 是保守選擇——前端對 SYSTEM 的
-- 呈現是「這是系統判斷的，可以更正」，對一個來源未知的值講這句話不會錯；反過來
-- 標成 BUYER_CHOICE（確認態）則會對從沒被問過的買家說謊。
--
-- label 留空：舊欄位沒存顯示名。ItemResolution.DisplayValue() 在 label 為空時
-- 退回 code，畫面上會看到代號而不是「甘藍-初秋」，但不會空白。
UPDATE procurement_request_revisions r
SET slots_json = r.slots_json || jsonb_build_object(
        'item_resolution',
        CASE
            WHEN q.crop_resolution_self_priced THEN
                jsonb_strip_nulls(jsonb_build_object(
                    'kind', 'SELF_PRICED',
                    'price_per_kg', q.crop_resolution_self_priced_price_per_kg,
                    'provenance', 'SYSTEM'))
            WHEN q.resolved_crop_code IS NOT NULL THEN
                jsonb_strip_nulls(jsonb_build_object(
                    'kind', 'CROP',
                    'code', q.resolved_crop_code,
                    'fallback_group_code', q.resolved_fallback_group_code,
                    'provenance', 'SYSTEM'))
            ELSE
                jsonb_strip_nulls(jsonb_build_object(
                    'kind', 'GROUP',
                    'code', q.resolved_group_code,
                    'fallback_group_code', q.resolved_fallback_group_code,
                    'provenance', 'SYSTEM'))
        END)
FROM procurement_requests q
WHERE r.request_id = q.id
  AND r.status = 'ACTIVE'
  AND r.slots_json ? 'item'
  AND (
        q.resolved_crop_code IS NOT NULL
     OR q.resolved_group_code IS NOT NULL
     OR (q.crop_resolution_self_priced
         AND q.crop_resolution_self_priced_price_per_kg IS NOT NULL
         AND q.crop_resolution_self_priced_price_per_kg > 0)
  );

-- 2) Modify "procurement_requests" table
ALTER TABLE "procurement_requests" DROP COLUMN "resolved_crop_code";
ALTER TABLE "procurement_requests" DROP COLUMN "resolved_group_code";
ALTER TABLE "procurement_requests" DROP COLUMN "resolved_fallback_group_code";
ALTER TABLE "procurement_requests" DROP COLUMN "crop_resolution_self_priced";
ALTER TABLE "procurement_requests" DROP COLUMN "crop_resolution_self_priced_price_per_kg";
