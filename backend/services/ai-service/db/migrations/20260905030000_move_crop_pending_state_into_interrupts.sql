-- 待消歧狀態（候選清單＋待處理原因）從 procurement_requests 搬進
-- procurement_agent_run_interrupts.payload_json（ADR
-- 2026-09-04-crop-resolution-into-slots Updates 第 2 點的後續刀）。手寫而非
-- atlas migrate diff 產出：diff 只會生出 ADD/DROP COLUMN，而這支 migration 的
-- 重點是中間那段把在途的待答狀態搬過去。

-- 1) Modify "procurement_agent_run_interrupts" table
ALTER TABLE "procurement_agent_run_interrupts"
    ADD COLUMN "payload_json" jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN "facts_as_of" timestamptz NULL;

-- 2) 把「還開著的」中斷的待答狀態搬進它自己的 payload。
--
-- 只搬 status='PENDING' 且未被解決的中斷：已解決的中斷，它當時問的是什麼已經
-- 無從還原——待答狀態在舊形狀是 request 上的單列可變欄位，每一次新的待答都覆寫
-- 前一次。把 request 現在的值回填給一個歷史中斷，等於宣稱它當時問的就是這個，
-- 那是偽造。已解決的中斷維持 payload_json='{}'，誠實表示「沒有留下這筆紀錄」。
--
-- interrupt → request 的連結只有 run.result_revision_id 這一條（domain_ref_type／
-- domain_ref_id 在 TURN/ANSWER/REWIND 路徑上從未被填），所以必須經由 revision
-- 表 join——與 FindOpenProcurementInterruptForRequest（db/queries/procurement/
-- runs.sql）走的是同一條路徑，不另發明第二種對應關係。
--
-- 條件用 pending_reason 非空 OR options 非空：Cut 1 起 0 候選的待處理狀態
-- （UNAVAILABLE）刻意不落 options，只看 options 會漏掉那種狀態；而 Cut 1 部署
-- 當下的在途 request 可能 pending_reason 還是空的、options 已有候選。兩個判準
-- 都留，跟 crop_gate.go／answer.go 當時的讀取端判準逐字對應。
--
-- pendingReason 補 'AMBIGUOUS' 的預設：走到 OR 右半邊（options 非空但
-- pending_reason 為 NULL）的只有上述相容窗那批在途 request，而舊碼唯一會留下
-- 候選清單的結果就是 AMBIGUOUS。
UPDATE procurement_agent_run_interrupts i
SET payload_json = jsonb_build_object(
        'kind', 'crop_disambiguation',
        'pendingReason', coalesce(q.crop_resolution_pending_reason, 'AMBIGUOUS'),
        'candidates', coalesce(q.crop_disambiguation_options_json, '[]'::jsonb)),
    -- facts_as_of 回填成這個中斷自己的 created_at：pricebook 查詢與中斷建立發生
    -- 在同一次 run 內、相隔秒級，以這個欄位被使用的精度（給買家看「行情資料時間」）
    -- 而言不構成失真。留 NULL 反而讓在途的那批中斷在前端顯示不出時間。
    facts_as_of = i.created_at
FROM procurement_agent_runs r
JOIN procurement_request_revisions rev ON rev.id = r.result_revision_id
JOIN procurement_requests q            ON q.id  = rev.request_id
WHERE i.run_id = r.id
  AND i.status = 'PENDING'
  AND i.resolved_by_child_run_id IS NULL
  AND (
        q.crop_resolution_pending_reason IS NOT NULL
     OR jsonb_array_length(coalesce(q.crop_disambiguation_options_json, '[]'::jsonb)) > 0
  );

-- 3) Modify "procurement_requests" table
ALTER TABLE "procurement_requests" DROP COLUMN "crop_disambiguation_options_json";
ALTER TABLE "procurement_requests" DROP COLUMN "crop_resolution_pending_reason";
