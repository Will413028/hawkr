-- seller_tax_profiles｜賣家稅務資格與治理
-- SSOT：設計書 58 號 §5.1.2（DDL）＋ §7 稅務資格狀態機（T01–T16）＋ §5.2.3 第 11 組
--
-- INV.gov 的兩件事在本表上性質不同，不要混：
--   ① 60 天待審／補件到期掃描（T10）＝ 集合式 UPDATE，**例外 A**、不需要 claim
--   ② 「屆期待複審」＝ **唯讀**推指標（invoice_seller_approval_recheck_due），**不改任何狀態**
--      （《後端服務責任邊界規範》：偵測告警可以，補償任務不行）

-- name: ExpirePendingTaxProfiles :many
-- T10：SELF_ISSUE_PENDING／SUPPLEMENT_REQUIRED 且 pending_valid_until <= now ⇒ EXPIRED。
-- 吃 idx_stp_pending_expiry。SUPPLEMENT_REQUIRED **必須**在述詞內，否則補件中的賣家永不逾期；
-- 因採 extension_count 屬性而非 EXTENDED 狀態，本述詞天然涵蓋被延長過的賣家（I-34）。
-- cooldown_until = now + 7 天（駁回／逾期後可重申的冷卻，T01 的守衛讀它）。
-- ⚠️ 本轉移**不執行任何接單限制**——與 §7.3.5「違規計數與停權」同一裁決：發票模組只發訊號，
--    處分走 seller-service 既有治理流程。要送的 P11 事件寫進 invoice_outbox_events（同一交易）。
UPDATE seller_tax_profiles
SET review_status = 'EXPIRED',
    cooldown_until = sqlc.arg(cooldown_until),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id IN (
  SELECT id FROM seller_tax_profiles
  WHERE review_status IN ('SELF_ISSUE_PENDING', 'SUPPLEMENT_REQUIRED')
    AND pending_valid_until <= sqlc.arg(now)
  ORDER BY pending_valid_until LIMIT sqlc.arg(row_limit))
  AND review_status IN ('SELF_ISSUE_PENDING', 'SUPPLEMENT_REQUIRED')
  AND pending_valid_until <= sqlc.arg(now)
RETURNING *;

-- name: CountApprovalRecheckDue :one
-- A03「屆期待複審」分頁 ＋ INV.gov 每日推 invoice_seller_approval_recheck_due 指標。
-- 吃 idx_stp_recheck_due。**DB 就是 SSOT**：不需要 worker 改狀態、不需要事件、更不需要
-- 「已通知」旗標（那是《多副本排程搶佔規範》禁止的永久旗標）。
-- approval_valid_until 屆滿**不改狀態**，只餵這支查詢。
SELECT COUNT(*) FROM seller_tax_profiles
WHERE review_status IN ('APPROVED_INVOICE_REQUIRED', 'APPROVED_INVOICE_EXEMPT')
  AND approval_valid_until IS NOT NULL
  AND approval_valid_until <= sqlc.arg(now);

-- name: GetSellerTaxProfileByIssuer :one
-- uq_seller_tax_profiles_issuer ⇒ 一 issuer 一列。
SELECT * FROM seller_tax_profiles WHERE issuer_id = sqlc.arg(issuer_id);

-- name: GetSellerTaxProfileByID :one
SELECT * FROM seller_tax_profiles WHERE id = sqlc.arg(id);

-- ===========================================================================
-- M3 稅務資格狀態機（T01–T16）的寫入面。§7.3.3 的「來源狀態」欄就是每一支的 allowed-from
-- SSOT：漏一個起點 ＝ CAS rows=0 靜默失敗，不拋例外、不進 DLQ、不告警，該筆永遠留在原狀態。
-- 一律正面列出狀態值（禁 <> / NOT IN），與 chk_stp_review_status 的 10 值同源。
-- ===========================================================================

-- name: InsertSellerTaxProfile :one
-- T01 的第一種形態（`[*]` → DRAFT，賣家第一次開啟 S02）。
-- uq_seller_tax_profiles_issuer ⇒ 一 issuer 一列；ON CONFLICT DO NOTHING 不指名 target
-- （理由同全書：未來加第二個唯一約束時不會突然 23505 並讓交易 aborted）。
-- rows=0 ⇒ 該 issuer 已有 profile，呼叫端改讀 GetSellerTaxProfileByIssuer。
INSERT INTO seller_tax_profiles (id, issuer_id, review_status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(issuer_id), 'DRAFT', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ResetSellerTaxProfileToDraft :execrows
-- T01 的第二種形態（REJECTED／EXPIRED → DRAFT，冷卻屆滿後重新申請）。
-- 守衛 `cooldown_until IS NULL OR cooldown_until <= now` 就是決策 C3 的 7 天冷卻，寫在述詞裡
-- ——寫在應用層的 if 會在併發重送時同時通過兩次。
-- 回 DRAFT 時清空上一輪的時鐘欄：submitted_at／pending_valid_until 留著會讓 T02 算出的新
-- 60 天窗口與舊值並存，而 chk_stp_pending_window 只檢查非空、不檢查新鮮度。
UPDATE seller_tax_profiles
SET review_status = 'DRAFT',
    submitted_at = NULL,
    pending_valid_until = NULL,
    extension_count = 0,
    cooldown_until = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('REJECTED', 'EXPIRED')
  AND (cooldown_until IS NULL OR cooldown_until <= sqlc.arg(now));

-- name: UpdateSellerTaxProfileDraft :execrows
-- 端點 18 的草稿存檔（不是狀態轉移，status 不動）。
-- ⚠️ evidence_object_keys 在**這裡**就已經是換發後的 objectKey（端點 18 在寫本表之前把
--    evidenceStagedTokens[] 換成 objectKey，§6.1.7），T02 的守衛讀的正是本欄 ⇒ 只上傳沒存草稿
--    就直接送審，那個守衛必然不成立。
-- allowed-from 只有 DRAFT：已送審的資料由 ADM 流程處理，賣家不得就地改內容。
UPDATE seller_tax_profiles
SET claimed_obligation = sqlc.arg(claimed_obligation),
    entity_tax_type = sqlc.narg(entity_tax_type),
    legal_name = sqlc.narg(legal_name),
    tax_id = sqlc.narg(tax_id),
    responsible_person = sqlc.narg(responsible_person),
    business_address = sqlc.narg(business_address),
    -- #3031：三段地址分開存。business_address 是串接後的完整地址（AMEGO 送件與
    -- 單行顯示用），city／district 是回程要還原到表單的兩格——只存串接結果的話
    -- joinAddress 折起來就拆不回去，賣家重新編輯時那兩格是空的。
    address_city = sqlc.narg(address_city),
    address_district = sqlc.narg(address_district),
    default_tax_category = sqlc.arg(default_tax_category),
    evidence_object_keys = sqlc.arg(evidence_object_keys),
    review_note = sqlc.narg(review_note),
    -- 只在 review_note **真的改變**時推進（#3089 第 11 項）：同值重寫不算一次新備註，
    -- 否則 A03 上那則備註的時間會隨每一次無關的審核動作往前跳。
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'DRAFT'
  AND version = sqlc.arg(expected_version);

-- name: SubmitSellerTaxProfile :execrows
-- T02 DRAFT → SUBMITTED。**單一時鐘**：submitted_at ＝ 60 天期限的唯一起算點（決策 C4），
-- pending_valid_until = submitted_at + 60 天由呼叫端算好傳入（測試要能注入 clock）。
-- 兩欄必須同一句寫：chk_stp_pending_window 對 SELF_ISSUE_PENDING／SUPPLEMENT_REQUIRED
-- 要求兩者皆非空，而那兩個狀態的**唯一**上游就是本列（T04／T05）——這裡少寫，下游當場 23514。
--
-- 🔴 **attestation_version／attested_at 也在同一句**（裁決 C，2026-08-31）。
--    不變式：**送審成功 ⇒ 條款必已簽**。做成同一句而不是「同一個交易內兩句」是刻意的：
--    兩句版本的原子性靠呼叫端記得包 tx，而那是一次重構就會掉的保證；同一句版本則是
--    「rows=1 ⇒ 五欄全寫」，任何呼叫順序都拆不開。
--    ⚠️ **兩欄無條件寫，不留 COALESCE／CASE 的退化分支**：那個形狀原本是要把「參數
--    不小心傳空」變成「保留舊值」，實測的結果卻是**該列被寫成 SUBMITTED 而兩個條款欄
--    留 NULL，且呼叫端收到 ok=true** —— 一個沒有簽署記錄的送審，而且靜默。現在改成
--    直接寫入，配合 schema 的三條 CHECK（chk_stp_attestation_pair／
--    _version_nonblank／_attested_when_committed），空字串或 NULL 一律 23514 當場失敗、
--    一列不寫。**吵一次好過寫出一列查不到憑據的資料**（同 RequestSellerTaxProfileSupplement
--    那一句「23514 而不是靜默寫壞」的取捨）。
--    ⚠️ **不動 review_status 的 allowed-from（仍是 DRAFT）**。
UPDATE seller_tax_profiles
SET review_status = 'SUBMITTED',
    submitted_at = sqlc.arg(now),
    pending_valid_until = sqlc.arg(pending_valid_until),
    attestation_version = sqlc.narg(attestation_version),
    attested_at = sqlc.arg(now),
    -- #3031：三元組一起寫。條款是自然人的意思表示，只有版本與時間舉不出簽署主體。
    attested_by_account_id = sqlc.narg(attested_by_account_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'DRAFT'
  AND version = sqlc.arg(expected_version);

-- name: AcceptSellerTaxProfileReview :execrows
-- T03 SUBMITTED → UNDER_REVIEW（ADM 受理）。
UPDATE seller_tax_profiles
SET review_status = 'UNDER_REVIEW',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'SUBMITTED'
  AND version = sqlc.arg(expected_version);

-- name: AttestSellerSelfIssue :execrows
-- T04 SUBMITTED → SELF_ISSUE_PENDING（賣家接受自行開票責任條款）。
-- ⚠️ **不重算 pending_valid_until**（逐字要求）：重算等於讓賣家用「停在未接受條款」把 60 天
--    窗口無限延長。本句刻意不碰那一欄。
UPDATE seller_tax_profiles
SET review_status = 'SELF_ISSUE_PENDING',
    attestation_version = sqlc.arg(attestation_version),
    attested_at = sqlc.arg(now),
    attested_by_account_id = sqlc.narg(attested_by_account_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'SUBMITTED'
  AND version = sqlc.arg(expected_version);

-- name: RequestSellerTaxProfileSupplement :execrows
-- T05 UNDER_REVIEW → SUPPLEMENT_REQUIRED（ADM 決策 REQUEST_SUPPLEMENT）。
-- chk_stp_pending_window 把 SUPPLEMENT_REQUIRED 與 SELF_ISSUE_PENDING 一起納入
-- 「必須有 submitted_at ＋ pending_valid_until」⇒ 若上游沒走過 T02（例：ADM 直接建列），
-- 本句會 23514 而不是靜默寫壞。
UPDATE seller_tax_profiles
SET review_status = 'SUPPLEMENT_REQUIRED',
    review_note = sqlc.narg(review_note),
    -- 只在 review_note **真的改變**時推進（#3089 第 11 項）：同值重寫不算一次新備註，
    -- 否則 A03 上那則備註的時間會隨每一次無關的審核動作往前跳。
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'UNDER_REVIEW'
  AND version = sqlc.arg(expected_version);

-- name: ResubmitSellerTaxProfileSupplement :execrows
-- T06 SUPPLEMENT_REQUIRED → UNDER_REVIEW（賣家補齊）。
-- 與 T12／T13／T15 分開是刻意的：那三條是 ADM 入口，合併會讓賣家的「補齊」按鈕
-- 也能把 EXPIRED／REVOKED 的 profile 推回審核中，繞過 T12「ADM 明示且必填理由」。
UPDATE seller_tax_profiles
SET review_status = 'UNDER_REVIEW',
    evidence_object_keys = sqlc.arg(evidence_object_keys),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'SUPPLEMENT_REQUIRED'
  AND version = sqlc.arg(expected_version);

-- name: ApproveSellerTaxProfile :execrows
-- T07／T08 UNDER_REVIEW／SELF_ISSUE_PENDING → APPROVED_INVOICE_REQUIRED／APPROVED_INVOICE_EXEMPT。
-- 兩條邊形狀相同，差別（REQUIRED 必有統編＋法定名稱、EXEMPT 不得帶統編）由
-- chk_stp_required_needs_taxid／chk_stp_exempt_no_taxid 在 DB 層擋 ⇒ 用 next_status 參數共用一句
-- 不會放行錯誤組合（傳錯值當場 23514，不是靜默寫進去）。
-- I-63：核准必寫 approved_at，approval_valid_until 恆由它算出（＝approved_at + 365 天，
-- 呼叫端算好傳入）；chk_stp_valid_until_needs_approved 守右半。
-- ⚠️ approval_valid_until 屆滿**不改狀態**——它只餵 A03「屆期待複審」分頁與
--    CountApprovalRecheckDue，本表沒有任何一條邊因它而觸發。
-- 🔴 收據代製授權的**核定依據文件在這一句一起寫**（§6.1.9：由 ADM 在端點 40 指名），
--    不另開一句 UPDATE：兩句 CAS 放在同一個交易裡，第二句的 expected_version 必須是
--    「第一句 +1」——那就是在 Go 端重建一份 SQL 已經算過的算式（同 §6.1.14 的
--    「重讀而不是就地 +1」）。折進來之後只有一次 version 遞增，也只有一個守衛。
-- ⚠️ 兩欄同進同出（chk_stp_receipt_basis_pair）；皆傳 NULL ＝ 不指名／取消指名，
--    端點 28 於是回 approvalBasisAvailable=false ＋ NOT_DESIGNATED。
UPDATE seller_tax_profiles
SET review_status = sqlc.arg(next_status),
    approved_at = sqlc.arg(now),
    approval_valid_until = sqlc.arg(approval_valid_until),
    receipt_approval_basis_key = sqlc.narg(approval_basis_key),
    receipt_approval_basis_asset_id = sqlc.narg(approval_basis_asset_id),
    cooldown_until = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('UNDER_REVIEW', 'SELF_ISSUE_PENDING')
  AND version = sqlc.arg(expected_version);

-- name: RejectSellerTaxProfile :execrows
-- T09 UNDER_REVIEW／SELF_ISSUE_PENDING → REJECTED（ADM 決策 REJECT）。
-- cooldown_until = now + 7 天由呼叫端傳入；它是 T01 第二形態（ResetSellerTaxProfileToDraft）
-- 那個守衛讀的欄位——少寫這一欄，被駁回的賣家可以立刻無限重送。
UPDATE seller_tax_profiles
SET review_status = 'REJECTED',
    cooldown_until = sqlc.arg(cooldown_until),
    review_note = sqlc.narg(review_note),
    -- 只在 review_note **真的改變**時推進（#3089 第 11 項）：同值重寫不算一次新備註，
    -- 否則 A03 上那則備註的時間會隨每一次無關的審核動作往前跳。
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('UNDER_REVIEW', 'SELF_ISSUE_PENDING')
  AND version = sqlc.arg(expected_version);

-- name: ExtendSellerTaxProfilePending :execrows
-- T11 SELF_ISSUE_PENDING →（狀態不變），就地改寫 pending_valid_until ＋ extension_count = 1。
-- `extension_count = 0` 是「只能延一次」的守衛，寫在述詞裡；chk_stp_extension 只擋 0–1 的值域，
-- 擋不了「延兩次但第二次也寫 1」，所以那個 0 不可省。
-- ⚠️ 刻意**不設** EXTENDED 狀態：設了的話 idx_stp_pending_expiry 與 ExpirePendingTaxProfiles
--    的述詞都得多一個值，漏寫任一個 ⇒ 被延長過的賣家永不逾期（I-34）。
UPDATE seller_tax_profiles
SET pending_valid_until = sqlc.arg(pending_valid_until),
    extension_count = 1,
    review_note = sqlc.narg(review_note),
    -- 只在 review_note **真的改變**時推進（#3089 第 11 項）：同值重寫不算一次新備註，
    -- 否則 A03 上那則備註的時間會隨每一次無關的審核動作往前跳。
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'SELF_ISSUE_PENDING'
  AND extension_count = 0
  AND version = sqlc.arg(expected_version);

-- name: AdminReopenSellerTaxProfileReview :execrows
-- T12（EXPIRED，ADM 受理逾期補件，免冷卻）／T13（APPROVED_* 複審）／T15（REVOKED 重新審核）
-- 三條邊 → UNDER_REVIEW。共用一句的條件：三者都是 ADM 入口、都帶必填理由、SET 形狀相同。
-- 四個來源狀態逐字對應那三列的「來源狀態」欄；賣家自助路徑（T06）另有一支，不共用。
-- cooldown_until 清空 ＝ T12 的「免冷卻」。
-- ⚠️ APPROVED_* 進入複審**不追溯已代製收據**（T13），本句因此不動 approved_at／
--    approval_valid_until：那兩欄是核定當下的事實，複審中不代表核定沒發生過。
UPDATE seller_tax_profiles
SET review_status = 'UNDER_REVIEW',
    recheck_reason = sqlc.arg(recheck_reason),
    cooldown_until = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('EXPIRED', 'APPROVED_INVOICE_REQUIRED', 'APPROVED_INVOICE_EXEMPT', 'REVOKED')
  AND version = sqlc.arg(expected_version);

-- name: RevokeSellerTaxProfile :execrows
-- T14 APPROVED_* → REVOKED（ADM 撤銷資格）。
-- 守衛是「必填理由 ＋ confirmed=true 二次確認 ＋ 完整稽核」，**不是雙人確認**——全案唯一的
-- 雙人機制是端點 54／55 的人工解鎖。二次確認發生在端點層，DB 這一側只有 expectedVersion CAS。
-- 撤銷後新訂單走 LEGACY_MANUAL；已付款未開票的訂單維持付款當下的快照模式（不回溯）。
UPDATE seller_tax_profiles
SET review_status = 'REVOKED',
    review_note = sqlc.narg(review_note),
    -- 只在 review_note **真的改變**時推進（#3089 第 11 項）：同值重寫不算一次新備註，
    -- 否則 A03 上那則備註的時間會隨每一次無關的審核動作往前跳。
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('APPROVED_INVOICE_REQUIRED', 'APPROVED_INVOICE_EXEMPT')
  AND version = sqlc.arg(expected_version);

-- name: KeepSellerTaxProfileUnderReview :execrows
-- T16 UNDER_REVIEW → UNDER_REVIEW（**自轉移**，ADM 決策 KEEP_UNDER_REVIEW）。
-- 存在理由：ADM 需要一個「我看過了、但還不能決定」的可稽核動作，否則那件案子會被反覆重看
-- 而沒有任何痕跡，而唯一能留痕的做法會是誤用 REQUEST_SUPPLEMENT（那會真的去催賣家補件）。
-- ⚠️ **不改任何時鐘欄位**：pending_valid_until 不重算、extension_count 不動。
--    留痕落在 invoice_status_history（from_status = to_status），不在本表。
-- version 照常 +1：它是一次業務操作，且同一列被 ADM 寫的次數有上界（§5.3.6 的判準是
-- 「同一列被寫的次數有沒有上界」，不是「誰觸發的」）。
UPDATE seller_tax_profiles
SET review_note = sqlc.narg(review_note),
    -- 只在 review_note **真的改變**時推進（#3089 第 11 項）：同值重寫不算一次新備註，
    -- 否則 A03 上那則備註的時間會隨每一次無關的審核動作往前跳。
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'UNDER_REVIEW'
  AND version = sqlc.arg(expected_version);

-- name: BumpSellerTaxProfilePendingOrderCounters :execrows
-- 待審期間接單額度的累加。口徑逐字取自 §5.1.2 欄位註解：「已付款訂單，**只在建立
-- invoice_documents 的同交易內累加**」⇒ 本句沒有第二個呼叫者，也沒有任何遞減路徑
-- （遞減需要有人在窗口滑出時掃描，那就是《後端服務責任邊界規範》禁止的 reconciliation job）。
-- 不動 version：它不是狀態轉移，也不參與 ADM 的 expectedVersion 樂觀鎖語意。
UPDATE seller_tax_profiles
SET pending_order_count = pending_order_count + 1,
    pending_order_amount = pending_order_amount + sqlc.arg(order_amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: GetSellerTaxProfileByStoreID :one
-- 端點 17／39（賣家自己看、ADM 看單筆）。seller_tax_profiles 上只有 issuer_id，
-- store_id 掛在 invoice_issuers 上 ⇒ 一定要 join。
-- 賣家路徑用它就是 owner-scoped（store_id 由 Gateway sellerid resolver 注入、前端
-- 不可控）；ADM 路徑用同一支，差別只在 store_id 從 path param 來。
SELECT p.* FROM seller_tax_profiles p
JOIN invoice_issuers i ON i.id = p.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint;

-- name: ListTaxReviewsForAdmin :many
-- 端點 38（A02 稅務審核佇列）。吃 idx_stp_review_queue。
--
-- 🔴 六個 `filter` **不是互斥分頁**（§6.1.12）：`NEAR_DEADLINE` 與 `EXTENDED` 是
--    「前四狀態之一 AND 額外條件」的**交集視圖** ⇒ 三個條件在 SQL 裡是 **AND**
--    （呼叫端送 status_filters = 前四狀態 ＋ 對應旗標），不是 OR。
--    做成互斥會讓案件從原分頁消失；而六個 filter 的筆數加總 ≠ 總筆數 ⇒
--    **A02 的 UI 不得顯示「合計」**，也不得用「總數 − 其他五頁」倒算任何一頁。
--
-- `open_complaint_count` 供 riskFlags 的 `OPEN_COMPLAINTS` 判定（其餘兩個 flag 的
--    資料源不在本服務，見呼叫端）。
-- keyword 比對 legal_name／tax_id／**store_id**（＝ invoice_issuers.issuer_ref_id）。
--
-- 🔴 store_id 這一格是 2026-09-04 補的（#3089 第 16 項）：本註解原本寫著
--    「依店名搜尋要由呼叫端先解析成 store_id 再進來」，而述詞裡**根本沒有
--    store_id 的比對** —— 呼叫端就算解析出來了也搜不到。
--    ADM 的搜尋框逐字寫「搜尋店名 / store_id」，於是照螢幕上的字去搜必然 0 筆。
--    issuer_ref_id 就在本句已經 JOIN 的那張表上，零跨服務成本。
--    轉成 text 比對而不是把 keyword 轉 bigint：搜尋框收的是自由文字，
--    非數字輸入應該是「查無」而不是 22P02（invalid input syntax）＝ HTTP 500。
--
-- ⚠️ **store_name（店名）仍然不在本服務**（seller-service 才有）⇒ 那一格搜不了，
--    ADM 的文案已同步改成「法定名稱 / 統編 / store_id」，不再承諾做不到的事。
SELECT sqlc.embed(p),
       i.issuer_ref_id AS issuer_ref_id,
       (SELECT COUNT(*)::bigint FROM invoice_complaints c
         WHERE c.issuer_id = p.issuer_id
           AND c.status = ANY(sqlc.arg(open_complaint_statuses)::text[])) AS open_complaint_count
FROM seller_tax_profiles p
JOIN invoice_issuers i ON i.id = p.issuer_id
WHERE (COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
       OR p.review_status = ANY(sqlc.arg(status_filters)::text[]))
  AND (NOT sqlc.arg(near_deadline_only)::boolean
       OR (p.pending_valid_until IS NOT NULL
           AND p.pending_valid_until <= sqlc.arg(near_deadline_before)::timestamptz))
  AND (NOT sqlc.arg(extended_only)::boolean OR p.extension_count >= 1)
  AND (sqlc.arg(keyword)::text = ''
       OR p.tax_id = sqlc.arg(keyword)::text
       OR i.issuer_ref_id::text = sqlc.arg(keyword)::text
       OR p.legal_name ILIKE '%' || sqlc.arg(keyword)::text || '%')
ORDER BY p.created_at ASC, p.id ASC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountTaxReviewsForAdmin :one
-- 端點 38 的 total。述詞與 ListTaxReviewsForAdmin 逐字相同。
SELECT COUNT(*)::bigint
FROM seller_tax_profiles p
JOIN invoice_issuers i ON i.id = p.issuer_id
WHERE (COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
       OR p.review_status = ANY(sqlc.arg(status_filters)::text[]))
  AND (NOT sqlc.arg(near_deadline_only)::boolean
       OR (p.pending_valid_until IS NOT NULL
           AND p.pending_valid_until <= sqlc.arg(near_deadline_before)::timestamptz))
  AND (NOT sqlc.arg(extended_only)::boolean OR p.extension_count >= 1)
  AND (sqlc.arg(keyword)::text = ''
       OR p.tax_id = sqlc.arg(keyword)::text
       OR i.issuer_ref_id::text = sqlc.arg(keyword)::text
       OR p.legal_name ILIKE '%' || sqlc.arg(keyword)::text || '%');

-- name: ConfirmSellerReceiptAuthorization :execrows
-- 端點 29（S05 賣家同意平台代製收據）。
--
-- 🔴 **資格守衛寫在這句的 WHERE，不在 CHECK 也不在應用層的 if**：
--    allowed-from 之外 ⇒ rows=0 回 **9335**。
--    寫成 CHECK 的後果見 §5.1.2 的紅字：review_status 之後會被複審推翻（T-C04），
--    而一條綁著它的 CHECK 會讓**之後每一次**對該列的更新都撞 23514。
--    寫在應用層的 if 則擋不住兩個分頁同時送出。
--
-- 🔴 **允許在「核定之前」就簽**（2026-09-05 使用者裁決 DEC-20260905-015836 選 A）。
--    原本這裡只有 `APPROVED_INVOICE_EXEMPT`，等於**授權只可能發生在核定之後**，
--    於是「ADM 核定完成」到「賣家簽授權」之間的每一筆已付訂單，收據都會落
--    AWAITING_MANUAL / RECEIPT_AUTH_MISSING 等人工處理——窗口有多長就有多少筆。
--    放寬之後賣家可在等待審核期間先簽，核定當下授權已成立，該窗口消失；
--    核定側改由 T08 守衛（applyDecision 的 9354）確保 EXEMPT 一定已授權。
--    ⚠️ 這四個「審核進行中」狀態逐字對應 T02～T06 那幾條邊的目的地——它們都還
--       可能走向 APPROVED_INVOICE_EXEMPT，所以先簽有意義。刻意**不含** DRAFT
--       （還沒送出，沒有任何審核在進行）與 APPROVED_INVOICE_REQUIRED
--       （已核定應開發票，代製收據對它沒有意義）。
--    ⚠️ 一律正面列出狀態值（禁 <> / NOT IN），與 chk_stp_review_status 的 10 值同源。
-- ⚠️ 授權是**冪等的取消／再授權開關**：authorized=false 時把兩欄一起清成 NULL
--    （chk_stp_receipt_auth_pair 要求同進同出），authorized=true 時一起寫。
--    只清其中一欄會當場 23514。
UPDATE seller_tax_profiles
SET receipt_authorized_at = sqlc.narg(authorized_at),
    receipt_consent_version = sqlc.narg(consent_version),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('SUBMITTED', 'UNDER_REVIEW', 'SUPPLEMENT_REQUIRED',
                        'SELF_ISSUE_PENDING', 'APPROVED_INVOICE_EXEMPT')
  AND version = sqlc.arg(expected_version);
