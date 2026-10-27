-- 契約收斂（issue #2446）：把改名過渡期留下的舊狀態值收斂成 CodeMeta registry 的正式值。
--
-- 這一支必須排在同一批的 CHECK constraint 之前（檔名時間戳決定套用順序）——CHECK 在
-- ADD CONSTRAINT 當下就會驗證既有列，先加 CHECK 的話這些舊值列會讓 migration 直接失敗。
--
-- 對「migrate 已生效、但新 pod 還沒 Ready」那段 surge 視窗的相容性：那段時間服務中的是
-- 上一版 code，它的 isSucceededRunStatus 同時認 SUCCEEDED／COMPLETED／PARTIAL，
-- 讀到被改寫成 SUCCEEDED 的列行為不變；它自己也只寫新值，不會再產生舊值。
--
-- PARTIAL → SUCCEEDED 不遺失資訊：新值域裡「部分成功」的表達是
-- 「run=SUCCEEDED ＋ 某條 lane=FAILED」，而 lane 狀態本來就各自記在 lane 欄位。
--
-- 冪等：WHERE 只挑舊值，重跑是 0 rows。
UPDATE "procurement_candidate_sets"
SET "status" = 'SUCCEEDED'
WHERE "status" IN ('COMPLETED', 'PARTIAL');

UPDATE "procurement_candidate_sets"
SET "platform_lane_status" = 'SUCCEEDED'
WHERE "platform_lane_status" = 'OK';

UPDATE "procurement_candidate_sets"
SET "external_lane_status" = 'SUCCEEDED'
WHERE "external_lane_status" = 'OK';

-- 清空前**先再補一次回填**。前一波已經做過同樣的 copy（20260819135326），但那是在前一波的
-- migrate 當下跑的；那之後到本波之間，surge 視窗裡仍在服務的更舊 pod 可能又寫出
-- 「error_code 有值、failure_reason 空」的列。少了這一句，下面的清空會把那些列的失敗原因
-- 直接抹掉——而且不會有任何訊號，是純資料遺失。
UPDATE "procurement_candidate_sets"
SET "failure_reason" = "error_code"
WHERE "error_code" <> '' AND "failure_reason" = '';

-- 清空 error_code：本波起沒有任何 code 讀寫它，值留著只有壞處。
--
-- 具體的壞處：本波的 RetryProcurementCandidateSet 只重置 failure_reason，不再碰 error_code。
-- 若本波部署失敗被 helm --atomic 回滾到上一版 code（schema 不會跟著回滾，Atlas 沒有 down
-- migration），上一版的 failureReasonOf 會在 failure_reason 為空時退回讀 error_code——
-- 於是一個剛被重試、正在 QUEUED 的 run 會對外顯示上一次失敗的原因。
-- 先清空就沒有值可以復活。
--
-- 也讓下一波的 DROP COLUMN 落在一個全空的欄位上，真正無資料損失。
UPDATE "procurement_candidate_sets"
SET "error_code" = ''
WHERE "error_code" <> '';
