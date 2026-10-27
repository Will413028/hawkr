-- invoice_provider_accounts｜provider 憑證與帳戶狀態
-- SSOT：設計書 58 號 §5.1.3（DDL）＋ §5.2.3 第 5 組（INV.hc 健康檢查租約）＋ §7 provider 狀態機（P01–P16）
--
-- ⚠️ 租約欄是 **health_check_next_at**，不是 last_health_checked_at（§4.6 W-02）。
--    後者是**事實時間戳**（最後一次真的打了 AMEGO），被 claim 推到未來就變成謊言：憑證看起來
--    剛檢查過、實際上從未檢查，而沒有任何斷言比對這兩欄——同時它又是「這個帳戶多久沒被檢查過」
--    這條告警的來源。
-- ⚠️ credential_ciphertext 的明文只在 internal/repository/postgres/mapper.go 出入
--    （AppKey 密文↔明文的唯一收斂點）。本檔任何 query 都不得回傳解密後的值。

-- name: FetchDueHealthChecks :many
-- INV.hc 的候選。狀態正面列出，與 idx_ipa_health 述詞逐字相同。
-- 述詞與 ClaimHealthCheck 逐字鏡射。租約保證同一時刻只有一個副本在打 AMEGO。
--
-- 三個狀態、兩種語意：
--   ACTIVE／DEGRADED — 巡檢，無終態（健康檢查本來就該一直跑）
--   PROVIDER_UNAVAILABLE — §7.3.4 P08 的自動重驗，**有 24 小時上界**
--
-- 🔴 P08 的上界錨在 breaker_opened_at（進入 PROVIDER_UNAVAILABLE 的時刻），
--    不能錨在 updated_at：ClaimHealthCheck 每輪都會推 updated_at ⇒ 永遠不滿 24 小時
--    ⇒ 無限重驗。錨錯欄位的後果是「上界寫了但不存在」，而它不會發出任何訊號。
-- 🔴 breaker_opened_at IS NULL 的 PROVIDER_UNAVAILABLE 列**不納入**（fail-closed）：
--    那是本欄啟用前留下的歷史列，算不出上界；它們仍可由 ADM 端點 44 手動重測。
SELECT * FROM invoice_provider_accounts
WHERE (
    status IN ('ACTIVE', 'DEGRADED')
    OR (
      status = 'PROVIDER_UNAVAILABLE'
      AND breaker_opened_at IS NOT NULL
      AND breaker_opened_at > sqlc.arg(unavailable_since_floor)
    )
  )
  AND health_check_next_at <= sqlc.arg(now)
ORDER BY health_check_next_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimHealthCheck :execrows
-- 租約 120s（下界 10.3s 單打／20.3s 同時打 track_status），基準逐筆在 claim 當下取。
-- 只推租約欄，**不動 last_health_checked_at**（見檔頭）、不動 version。
UPDATE invoice_provider_accounts
SET health_check_next_at = sqlc.arg(lease_until), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND (
    status IN ('ACTIVE', 'DEGRADED')
    OR (
      status = 'PROVIDER_UNAVAILABLE'
      AND breaker_opened_at IS NOT NULL
      AND breaker_opened_at > sqlc.arg(unavailable_since_floor)
    )
  )
  AND health_check_next_at <= sqlc.arg(claimable_before);

-- name: RecordHealthCheckResult :execrows
-- 一輪結束**不論成敗**都寫 health_check_next_at = now + 15min（巡檢週期，與 §7 狀態機 P08 同值）
-- ——沒有這一句，claim 推上去的 120s 租約會變成事實上的巡檢週期。
-- last_health_checked_at 只在「真的打了 AMEGO」時由本句寫入（事實時間戳）。
-- 巡檢週期是**容量決策**不是即時性偏好：每個 ACTIVE／DEGRADED 憑證 4×24 ＝ 96 次／日，
-- 掛在賣家數 S 而非訂單數 D 上，零開票的日子照打（§9.2.1）。改週期要回頭重算三檔呼叫量。
UPDATE invoice_provider_accounts
SET last_health_checked_at = sqlc.arg(now),
    health_check_next_at = sqlc.arg(next_check_at),
    status = sqlc.arg(next_status),
    status_reason = sqlc.narg(status_reason),
    breaker_opened_at = sqlc.narg(breaker_opened_at),
    breaker_probe_success = sqlc.arg(breaker_probe_success),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id);

-- name: ListActiveProviderAccounts :many
-- INV.trk（字軌餘量）掃的是 status='ACTIVE' 的**全集**，不帶租約欄——它在 DDL 上沒有候選集合
-- （§5.2.3 末段）。有 ClaimLease ≠ 有候選集合租約，不要因為 §7.5.4 給了它 10min 就回頭在
-- §5 加欄位。
SELECT * FROM invoice_provider_accounts
WHERE status = 'ACTIVE'
ORDER BY id;

-- name: GetProviderAccountByIssuer :one
-- uq_ipa_issuer_provider ⇒ 一個 issuer 對一個 provider 只有一列。
SELECT * FROM invoice_provider_accounts
WHERE issuer_id = sqlc.arg(issuer_id) AND provider = sqlc.arg(provider);

-- name: GetProviderAccountByID :one
SELECT * FROM invoice_provider_accounts WHERE id = sqlc.arg(id);

-- name: InsertProviderAccount :execrows
-- ⚠️ ON CONFLICT DO NOTHING **不指名 conflict target**：本表有兩個唯一索引
--    （uq_ipa_issuer_provider 與 partial 的 uq_ipa_fingerprint），指名其一 ⇒ 另一個衝突時拋 23505，
--    而 PostgreSQL 交易內任一句失敗即整個交易 aborted（25P02），catch 了也救不回來。
--    不指名時對任何唯一約束衝突都靜默跳過，rows=0 ⇒ 呼叫端回「這組憑證已被綁走」。
-- uq_ipa_fingerprint 擋的是 AMEGO S17：同一組統編＋AppKey 不得被兩個同時有效的 issuer 綁走
--    （一組字軌只能由一套系統使用）。它的述詞是 9 值正面列舉。
INSERT INTO invoice_provider_accounts (
  id, issuer_id, provider, provider_tax_id,
  credential_ciphertext, credential_key_version, credential_fingerprint, credential_last4,
  status, health_check_next_at, credential_updated_at, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(issuer_id), sqlc.arg(provider), sqlc.arg(provider_tax_id),
  sqlc.arg(credential_ciphertext), sqlc.arg(credential_key_version),
  sqlc.arg(credential_fingerprint), sqlc.arg(credential_last4),
  sqlc.arg(status), sqlc.arg(health_check_next_at),
  sqlc.arg(now)::timestamptz, sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: UpdateProviderAccountStatus :execrows
-- ADM／賣家路徑的狀態轉移，走 expectedVersion CAS（P07 缺這條邊時 rows=0 會靜默失敗、
-- 前端顯示成功而狀態沒變、賣家永遠串不上）。呼叫端必須把 rows=0 當衝突處理，不可當成功。
UPDATE invoice_provider_accounts
SET status = sqlc.arg(next_status),
    status_reason = sqlc.narg(status_reason),
    verified_at = sqlc.narg(verified_at),
    health_check_next_at = sqlc.arg(health_check_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version);

-- name: UpdateProviderAccountCredential :execrows
-- 憑證輪替落地：先驗證再覆蓋，失敗保留原 key ⇒ 新舊密文需同時存在一段時間。
-- ⚠️ chk_ipa_rotation 是雙向等式 (status='ROTATION_PENDING') = (pending_ciphertext IS NOT NULL)、
--    chk_ipa_rotation_triplet 要求 pending 三欄同進同出 ⇒ 落地時必須**同一句**把三欄清成 NULL
--    並把 status 移出 ROTATION_PENDING，少做一半當場 23514。
-- 🔴 allowed-from ＝ ROTATION_PENDING 單值（P12）。2026-08-30 補：本句原本只有 version CAS，
--    而註解已經在宣稱「status 已不是可輪替的狀態 ⇒ rows=0」——那句話當時是假的。
--    少了它，一把還沒被 StartCredentialRotation 收下的 key 可以直接覆蓋現役憑證。
UPDATE invoice_provider_accounts
SET credential_ciphertext = sqlc.arg(credential_ciphertext),
    credential_key_version = sqlc.arg(credential_key_version),
    credential_fingerprint = sqlc.arg(credential_fingerprint),
    credential_last4 = sqlc.arg(credential_last4),
    pending_ciphertext = NULL,
    pending_fingerprint = NULL,
    pending_last4 = NULL,
    rotation_started_at = NULL,
    status = sqlc.arg(next_status),
    -- 憑證真的換掉了 ⇒ 這一欄前進（#3051）。它與 updated_at 的差別就是本票的全部：
    -- updated_at 被健康檢查每分鐘改寫，這一欄只有三個寫入點（建列／輪替落地／重新提交）。
    credential_updated_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'ROTATION_PENDING';

-- name: StartProviderCredentialRotation :execrows
-- 輪替開始：寫 pending 三欄並把 status 推到 ROTATION_PENDING（兩者必須同一句，見上）。
UPDATE invoice_provider_accounts
SET pending_ciphertext = sqlc.arg(pending_ciphertext),
    pending_fingerprint = sqlc.arg(pending_fingerprint),
    pending_last4 = sqlc.arg(pending_last4),
    rotation_started_at = sqlc.arg(now),
    status = 'ROTATION_PENDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'ACTIVE';

-- name: GetProviderAccountByStoreID :one
-- 端點 22／43。invoice_provider_accounts 上只有 issuer_id ⇒ 經 invoice_issuers 解析
-- store_id。查無列時回 pgx.ErrNoRows ⇒ 單筆端點翻 404＋9330；**列表端點不共用它**
-- （列表的「查無列」要投影成 NOT_STARTED，見 ListProviderAccountsForAdmin）。
SELECT a.* FROM invoice_provider_accounts a
JOIN invoice_issuers i ON i.id = a.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint
  AND a.provider = sqlc.arg(provider);

-- name: ListProviderAccountsForAdmin :many
-- 端點 42（A04 串接監控）。
--
-- 🔴 **FROM 是 invoice_issuers 不是 invoice_provider_accounts，且是 LEFT JOIN。**
--    需求書的「狀態空值視為配置錯誤」照字面實作會讓上線當天每一個既有賣家都被標成
--    配置錯誤（§6.1.13 三態表）：查無列 ⇒ `NOT_STARTED`（尚未開始設定），有列但
--    status 未知 ⇒ `CONFIG_ERROR`。從 provider_accounts 起手就永遠列不出第一種，
--    而那是上線當天的全體。三態的投影在呼叫端做（`NOT_STARTED`／`CONFIG_ERROR`
--    不落 DB，chk_ipa_status 是該 enum 的真子集）。
--    ⚠️ 仍有一層漏斗：**沒有 invoice_issuers 列的賣家連這裡都不會出現**（issuer 列
--       是首次開票／首次設定時才建）。要涵蓋「全平台每一個賣家」得由呼叫端拿
--       seller-service 的賣家清單來對，不是在本查詢加 join（跨服務 DB join 是禁區）。
--
-- keyword 比對 provider_tax_id 與 issuer 的 display_name。
SELECT sqlc.embed(i),
       a.id                     AS account_id,
       a.provider               AS provider,
       a.provider_tax_id        AS provider_tax_id,
       a.status                 AS account_status,
       a.status_reason          AS status_reason,
       a.credential_last4       AS credential_last4,
       a.verified_at            AS verified_at,
       a.automation_cutover_at  AS automation_cutover_at,
       a.last_health_checked_at AS last_health_checked_at,
       a.version                AS account_version,
       a.updated_at             AS account_updated_at
FROM invoice_issuers i
LEFT JOIN invoice_provider_accounts a
       ON a.issuer_id = i.id AND a.provider = sqlc.arg(provider)::text
WHERE i.issuer_type = 'SELLER'
  AND (sqlc.arg(status_filter)::text = ''
       OR a.status = sqlc.arg(status_filter)::text
       OR (sqlc.arg(match_not_started)::boolean AND a.id IS NULL))
  AND (sqlc.arg(keyword)::text = ''
       OR a.provider_tax_id = sqlc.arg(keyword)::text
       OR i.display_name ILIKE '%' || sqlc.arg(keyword)::text || '%')
ORDER BY i.created_at DESC, i.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountProviderAccountsForAdmin :one
-- 端點 42 的 total。述詞與 ListProviderAccountsForAdmin 逐字相同——**含那個
-- LEFT JOIN 的 ON 條件**：把 `a.provider = ...` 從 ON 挪到 WHERE 會把 LEFT JOIN
-- 退化成 INNER JOIN，total 就少掉全部 NOT_STARTED 的賣家而列表還在顯示他們。
SELECT COUNT(*)::bigint
FROM invoice_issuers i
LEFT JOIN invoice_provider_accounts a
       ON a.issuer_id = i.id AND a.provider = sqlc.arg(provider)::text
WHERE i.issuer_type = 'SELLER'
  AND (sqlc.arg(status_filter)::text = ''
       OR a.status = sqlc.arg(status_filter)::text
       OR (sqlc.arg(match_not_started)::boolean AND a.id IS NULL))
  AND (sqlc.arg(keyword)::text = ''
       OR a.provider_tax_id = sqlc.arg(keyword)::text
       OR i.display_name ILIKE '%' || sqlc.arg(keyword)::text || '%');

-- ===========================================================================
-- §6.1.8 賣家憑證流程（端點 24／25／26／27）與 §6.1.13 ADM 生命週期（端點 44／45／66／67）
-- 的狀態轉移。**每一支的 allowed-from 都逐字列在自己的 WHERE 裡**（§7.3.0 規則 4）：
-- 放在 Go 的 if 擋不住兩個分頁同時送出，而 rows=0 是完全靜默的。
--
-- 🔴 為什麼不寫成一支「next_status ＋ allowed_from[] 都當參數」的通用轉移：
--    那等於把 allowed-from 搬回呼叫端，本檔就不再是那份清單的 SSOT。next_status
--    當參數是可以的（值域由 CodeMeta 守），allowed-from 不行。
-- ===========================================================================

-- name: ResubmitProviderCredential :execrows
-- 端點 24 的**覆寫**分支（既有列已存在時）：P01 建列走 InsertProviderAccount，本句走
-- 「憑證錯了、賣家重新填一次」那一半，落地即 CREDENTIAL_VERIFYING（§6.1.8 端點 24
-- 「寫入後 status = CREDENTIAL_VERIFYING」＝ P01＋P02 的合併，中間態不對外呈現）。
--
-- allowed-from **不含 ACTIVE／DEGRADED**：那兩個狀態的換 key 一律走端點 27 輪替
-- （先驗證新 key 再覆蓋）。少了這條界線，賣家可以用端點 24 直接覆蓋一把還在服役的
-- 憑證，而覆蓋後驗證失敗 ＝ 從那一刻起每一筆開票都失敗，UI 上 status 卻仍是 ACTIVE。
-- 也不含 ROTATION_PENDING（pending_* 還在，chk_ipa_rotation 是雙向等式會當場 23514）、
-- SUSPENDED／REVOKED（那兩個只有 ADM 進得去也只有 ADM 出得來）。
--
-- ⚠️ 本句可能撞 uq_ipa_fingerprint（這組統編＋AppKey 已被另一個同時有效的 issuer 綁走）。
--    UPDATE 沒有 ON CONFLICT ⇒ 只會拋 23505 並讓整個交易 aborted（25P02）。
--    repository 把 23505 翻成 9337，呼叫端在那之後不得再下任何一句 SQL。
UPDATE invoice_provider_accounts
SET credential_ciphertext = sqlc.arg(credential_ciphertext),
    credential_key_version = sqlc.arg(credential_key_version),
    credential_fingerprint = sqlc.arg(credential_fingerprint),
    credential_last4 = sqlc.arg(credential_last4),
    provider_tax_id = sqlc.arg(provider_tax_id),
    status = 'CREDENTIAL_VERIFYING',
    status_reason = NULL,
    health_check_next_at = sqlc.arg(health_check_next_at),
    credential_updated_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('NOT_CONFIGURED','CREDENTIAL_SUBMITTED','CREDENTIAL_VERIFYING',
                 'INVALID_CREDENTIAL','PROVIDER_REJECTED','PROVIDER_UNAVAILABLE');

-- name: StartProviderVerification :execrows
-- P02（CREDENTIAL_SUBMITTED → CREDENTIAL_VERIFYING）＋ P07（三個錯誤狀態 → CREDENTIAL_VERIFYING）
-- ＋ P15（SUSPENDED → CREDENTIAL_VERIFYING，端點 66 恢復前強制重驗，**不得直接回 ACTIVE**）。
-- 端點 25（賣家測試連線）／44（ADM 重新測試）／66（ADM 恢復連線）共用。
-- ⚠️ SUSPENDED 在 allowed-from 內是 P15 的唯一入口；端點 25／44 走到它的機率是零
--    （呼叫端在進來之前就把 SUSPENDED 擋成 9333），但述詞不為了那個而拆成兩句：
--    多一句就多一份要同步維護的 allowed-from。
--
-- 🔴 **ACTIVE／DEGRADED 不在 allowed-from 內**：§7.3.4 沒有 `ACTIVE → CREDENTIAL_VERIFYING`
--    這條邊。對那兩個狀態的重測走「不改 status、只落健康檢查事實」那條路
--    （TouchProviderHealthCheck ／ FailProviderVerificationFromActive），
--    這正是 §6.1.13 端點 44 定版 ③ 要的效果。
-- ⚠️ CREDENTIAL_VERIFYING 自己也在 allowed-from 內：上一輪在「已推到驗證中、結果還沒
--    落地」之間崩潰的那些列，只能靠賣家或 ADM 再按一次才回得來（健康檢查排程的候選
--    集合只有 ACTIVE／DEGRADED，撈不到它）。
UPDATE invoice_provider_accounts
SET status = 'CREDENTIAL_VERIFYING',
    status_reason = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('CREDENTIAL_SUBMITTED','CREDENTIAL_VERIFYING',
                 'INVALID_CREDENTIAL','PROVIDER_REJECTED','PROVIDER_UNAVAILABLE','SUSPENDED');

-- name: LandProviderVerification :execrows
-- P03（→ ACTIVE）／P04（→ INVALID_CREDENTIAL）／P05（→ PROVIDER_REJECTED）／
-- P06（→ PROVIDER_UNAVAILABLE）四條邊的共同落點。allowed-from 只有 CREDENTIAL_VERIFYING。
--
-- 🔴 verified_at 是 sqlc.narg 而**呼叫端必須逐次明給**：它同時被 chk_ipa_active_verified
--    （ACTIVE ⇒ 非空）與 chk_ipa_cutover_requires_verified（有切點 ⇒ 非空）讀。落 ACTIVE
--    時傳 now；落三個失敗狀態時傳**該列原本的值**——傳 nil 會把它清成 NULL，而那一列
--    若已設過 automation_cutover_at 就當場 23514。
-- last_health_checked_at 是事實時間戳（真的打了 AMEGO 才寫），與租約欄 health_check_next_at
--    不同源（§4.6 W-02）。
--
-- 🔴 breaker_opened_at 在本句是「這個帳戶從什麼時候開始連不上」的錨，由 P08 的 24 小時
--    上界讀（FetchDueHealthChecks）。落 PROVIDER_UNAVAILABLE 時呼叫端給 now、落其餘三個
--    狀態時給 nil。**它必須逐次明給**：漏給就是 NULL，而 NULL 的 PROVIDER_UNAVAILABLE
--    列被 P08 的述詞 fail-closed 排除 ⇒ 自動重驗靜默失效（沒有錯誤、沒有告警）。
UPDATE invoice_provider_accounts
SET status = sqlc.arg(next_status),
    status_reason = sqlc.narg(status_reason),
    verified_at = sqlc.narg(verified_at),
    breaker_opened_at = sqlc.narg(breaker_opened_at),
    last_health_checked_at = sqlc.arg(now),
    health_check_next_at = sqlc.arg(health_check_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'CREDENTIAL_VERIFYING';

-- name: FailProviderVerificationFromActive :execrows
-- §6.1.13 端點 44 定版 ③ 的前半：ACTIVE／DEGRADED 的憑證**確實**壞了（AMEGO code 12／16
-- ⇒ INVALID_CREDENTIAL；13／19／22 ⇒ PROVIDER_REJECTED）⇒ 落成賣家自己修得掉的狀態。
-- verified_at **不動**（那是「上次驗過」的事實，不因為這次失敗而消失，而且它一被清空，
-- 已設定的 automation_cutover_at 會讓本句 23514）。
UPDATE invoice_provider_accounts
SET status = sqlc.arg(next_status),
    status_reason = sqlc.narg(status_reason),
    last_health_checked_at = sqlc.arg(now),
    health_check_next_at = sqlc.arg(health_check_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('ACTIVE','DEGRADED');

-- name: TouchProviderHealthCheck :execrows
-- §6.1.13 端點 44 定版 ③ 的後半：**status 完全不動**，只寫「這一刻真的打了 AMEGO」。
-- 兩種情形共用：① ACTIVE／DEGRADED 檢查通過 ② 任何來源狀態遇到 PROVIDER_UNAVAILABLE
-- 而該來源是 ACTIVE／DEGRADED。把「AMEGO 現在連不上」寫成賣家的帳號狀態，會在 AMEGO
-- 恢復後留下一個假的錯誤狀態，而沒有人會回來清。
UPDATE invoice_provider_accounts
SET last_health_checked_at = sqlc.arg(now),
    health_check_next_at = sqlc.arg(health_check_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('ACTIVE','DEGRADED');

-- name: AbortProviderCredentialRotation :execrows
-- P13：新 key 驗證**失敗** ⇒ 回 ACTIVE 並**保留原 key**。
-- 三個 pending 欄與 status 必須同一句清（chk_ipa_rotation 雙向等式 ＋ chk_ipa_rotation_triplet）。
-- credential_* 一個字都不碰——這正是「先驗證再覆蓋」的全部意義。
UPDATE invoice_provider_accounts
SET pending_ciphertext = NULL,
    pending_fingerprint = NULL,
    pending_last4 = NULL,
    rotation_started_at = NULL,
    status = 'ACTIVE',
    status_reason = sqlc.narg(status_reason),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'ROTATION_PENDING';

-- name: SetProviderAutomationCutover :execrows
-- 端點 26（確認啟用切點）＝ 不變式 I-61：`automation_cutover_at` 只能在 status='ACTIVE'
-- 當下被設定，且一經設定永不清空。
--
-- 🔴 allowed-from 取 **ACTIVE 單值**，不取 §6.1.8 端點 26 硬規則 1 的「ACTIVE / DEGRADED」：
--    I-61 在 §5.2 與 §7.3.4 兩處都逐字寫成 `AND status = 'ACTIVE'`，而 §6.1.8 只有一句散文。
--    兩者衝突已登記在交付回報；取窄的那一邊的代價是「AMEGO 暫時降級時設不了切點、
--    等它回 ACTIVE 再設」，取寬的代價是破壞一條列了 DB 兜底與測試計畫的不變式。
-- ⚠️ `verified_at IS NOT NULL` 是把 chk_ipa_cutover_requires_verified 的 23514（對賣家是
--    一句 500）換成 rows=0（可回 9333）。它不放寬任何東西：chk_ipa_active_verified 已保證
--    ACTIVE ⇒ verified_at 非空，本條在正常資料上恆真。
-- ⚠️ **不寫 status**：§6.1.8 端點 26 硬規則 1 逐字「沒有『測試成功待啟用』的中間態」，
--    步驟 4 與 5 的分界是 automation_cutover_at 有沒有值，不是 status。
UPDATE invoice_provider_accounts
SET automation_cutover_at = sqlc.arg(cutover_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status = 'ACTIVE'
  AND verified_at IS NOT NULL;

-- name: SuspendProviderAccount :execrows
-- P14（端點 45 ADM 停用連線）。ACTIVE／DEGRADED → SUSPENDED，可逆（P15 恢復／P16 撤銷）。
-- verified_at 不動（理由同 FailProviderVerificationFromActive）；
-- automation_cutover_at 不動（歷史訂單的模式判定要用它）。
UPDATE invoice_provider_accounts
SET status = 'SUSPENDED',
    status_reason = sqlc.narg(status_reason),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('ACTIVE','DEGRADED');

-- name: RevokeProviderAccount :execrows
-- P16（端點 67 ADM 撤銷）。M4 唯一終態，不可逆。
-- 🔴 `automation_cutover_at` **保留不清空**（§7.3.4 P16 逐字）：歷史訂單的模式判定要用它。
-- 🔴 REVOKED 不在 uq_ipa_fingerprint 的 partial 述詞內 ⇒ 本句同時**釋放那組（統編, AppKey）
--    的名額**，賣家關店重開才綁得回同一組憑證（§5 I-37 第二向）。
UPDATE invoice_provider_accounts
SET status = 'REVOKED',
    status_reason = sqlc.narg(status_reason),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('ACTIVE','DEGRADED','SUSPENDED');
