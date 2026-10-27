-- issue #2471：訂單時間軸的結案節點沒有時間戳 —— 取消側
--
-- 設計書：03 §5.3.1（取消）；56 號 §6.1（代行訂單時間軸）
--
-- 「完成」側早就有 confirmed_at（V1:62，completeDeliveredOrderByStatus 一直在寫）、
-- 「已送達」側有 delivered_at（V1:75）；只有「取消」這一格沒有任何業務時間戳，
-- 於是訂單時間軸最重要的一格（這筆單什麼時候結束的）永遠是空的。
ALTER TABLE orders ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ;

COMMENT ON COLUMN orders.cancelled_at IS
    'issue #2471：訂單取消時間戳，由三支取消 CAS（cancelOrderInternalCas / cancelOrderByBuyerCas / cancelOrderByPaymentExpireCas）寫入。NULL = 未取消，或取消於本 migration 之前（無法回填）。';

-- ⚠️ 刻意 nullable、不設 DEFAULT：DEFAULT 會把**每一列**既有訂單（含從未取消的）填成
--    migration 執行時刻，那是憑空捏造的業務時間（10 檔 §11.2 的 2026-08-01 教訓）。
--
-- ⚠️ 刻意**不回填**既有的已取消訂單：唯一看似可用的來源是 updated_at，但它是 audit 欄不是
--    業務時間——dev 實測 95 筆有 outbox 佐證的取消單裡 11 筆對不上，最大偏離 22.7 天
--    （取消之後仍有 MQ 事件、退款、評論等寫入把 updated_at 往後推）。拿它冒充結案時間會產生
--    「看起來合理但錯得離譜」的畫面，比留白更糟（10 檔 §21：audit time 不可冒充業務狀態時間）。
--    既有已取消訂單的 cancelled_at 就是 NULL，前端據此不渲染該節點。
