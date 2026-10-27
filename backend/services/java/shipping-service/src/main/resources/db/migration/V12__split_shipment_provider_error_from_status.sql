-- V12: 把「物流商失敗代碼」從 shipments.provider_status 拆出，還原四個語意各自的 ownership。
--
-- 背景：Foundation remediation 讓 provider_status 開始保存「正常貨態」的原始值
-- （updateShipmentStatus 的 COALESCE 保全 + updateProviderStatus 落地未知貨態），
-- 而 ADM 的異常清單述詞是「provider_status IS NOT NULL AND <> ''」
-- ⇒ 正常單被當成異常。這不是既有原狀，是 remediation 放大的 regression。
--
-- 拆分後的 canonical ownership：
--   shipments.status                 → normalized/domain status（狀態機唯一擁有者）
--   shipments.provider_status        → 物流商原始貨態（正常軌跡證據，含尚未 mapping 的未知值）
--   shipments.provider_error_code    → 物流商失敗代碼（建單/取號/叫件失敗）
--   shipments.provider_error_message → 對應的失敗訊息
--   異常判準                          → status IN (失敗態) OR provider_error_code IS NOT NULL
--
-- 一次性回填的判別式：以「值的形狀」分類，而非以 label_print_status / tcat_call_status
-- 這類 flag——實測 flag 判別式在 dev 上有 4 個反例（S691970098 叫件失敗後貨態仍推進到
-- 11 已取貨，flag 會誤判成錯誤碼；S977239637 等三筆 tcat_call_status=4203「成功」但
-- provider 回 FAIL/「處理失敗 (執行失敗 -> E072-黑貓宅急便…」，flag 會漏掉真失敗）。
-- 形狀判別式在既有 354 筆非空資料上零反例：
--   貨態   ^[0-9]+$ 或 '-'  → -,11,21,32,33,53,92                          （312 筆）
--   錯誤碼 其餘              → DEF*,HOME*,SHIP*,FAIL（PayUni 錯誤碼一律帶字母前綴）（42 筆）
-- 形狀只用於這一次歷史資料切分；切分後由寫入端各自寫進正確欄位，語意不再靠形狀推斷。

ALTER TABLE shipments ADD COLUMN IF NOT EXISTS provider_error_code    VARCHAR(64);
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS provider_error_message TEXT;
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS provider_error_at      TIMESTAMPTZ;

COMMENT ON COLUMN shipments.provider_status IS
    '物流商回傳的原始貨態碼（正常軌跡證據，含本系統尚未 mapping 的未知值）。'
    '🔴 不得以「有值」判定異常——異常判準是 provider_error_code IS NOT NULL。';
COMMENT ON COLUMN shipments.provider_error_code IS
    '物流商回傳的失敗代碼（建單/取號/叫件失敗）。非 NULL 即為 provider 層失敗，'
    '是 ADM 異常清單的唯一 provider 側判準。';
COMMENT ON COLUMN shipments.provider_error_message IS 'provider_error_code 對應的失敗訊息原文。';
COMMENT ON COLUMN shipments.provider_error_at IS 'provider 失敗發生時間（與 provider_status_time 分離，後者是貨態證據時間）。';

UPDATE shipments
   SET provider_error_code    = provider_status,
       provider_error_message = provider_message,
       provider_error_at      = provider_status_time,
       provider_status        = NULL,
       provider_message       = NULL,
       provider_status_time   = NULL
 WHERE provider_status IS NOT NULL
   AND provider_status <> ''
   AND provider_status !~ '^[0-9]+$'
   AND provider_status <> '-';
