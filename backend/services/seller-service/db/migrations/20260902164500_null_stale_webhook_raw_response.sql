-- payuni_status_history.raw_response 在 source='webhook' 的列上從來就不是那次
-- AGENT_NOTIFY 的 payload：HandleAgentNotify 不曾寫 info.RawResponse，而稽核列
-- 無條件從 registration 複製該欄，所以 webhook 列存的是上一次拉取（或註冊建立
-- 呼叫）留下的快照。dev 上可見兩個方向的證據：真實 webhook 列帶著建店回應信封
-- {"Status":"SUCCESS","Message":"合作商店資料建立成功"}，而 publish 過
-- MemberStatus 9/8/77/92 的列卻讀回 "1"。修法是同一個 PR 的
-- PayuniStatusObservation 重構，此後每列都由該次觀測自己建構。
--
-- 這些既有列的真實 payload 已不可能取回。留著只會讓下一個查稽核表的人拿到
-- 一個看起來有效、實際屬於別次事件的答案——這正是 PR #2828 當初承諾的抽查
-- 方法失效的原因。清成 NULL，讓「沒有保留原始 payload」誠實可見。
--
-- 解析後的 mer_status / member_status / reason 等欄位刻意不動：那些一直都直接
-- 來自 payload，是可信的，也是查 webhook 真實輸入該用的欄位。
--
-- Data-only（無 schema 變更）；idempotent：已為 NULL 的列不匹配 WHERE，重跑零副作用。
UPDATE "payuni_status_history" SET "raw_response" = NULL
WHERE "source" = 'webhook' AND "raw_response" IS NOT NULL;
