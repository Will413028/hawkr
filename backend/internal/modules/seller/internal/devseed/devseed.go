// Package devseed 寫入本機開發的種子店家：固定的 store_id 與 store_code、已核准的 KYC、已綁定而且 PayUni 核准的註冊、logo，以及
// 開店時一併建立的公告列（version 0；由 ReannounceExposures 全量公告）。只給本機開發（cmd/hawkr 的 dev-seed）使用：它直接寫列，
// 繞過開店、送出、審核與綁定的狀態轉移（不寫那些轉移的事件；審核紀錄只寫送出那一筆，入駐同意要綁定它）；還原 profile 時發 StoreProfileChanged（與模組自己寫 profile 同一個事件），
// 刪掉的證據與證件經模組的退場機制釋放 media 的引用。
//
// 明文的 fixture 是來源：KYC 的識別欄位經 kyc.Service.SeedApproved（SaveDraft 同一支加密與 hash）、以執行中的金鑰寫入，種子檔不含密文。
package devseed

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/agreement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/kyc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payuni"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Store 是一家種子店家（明文）。StoreID 與 StoreCode 固定（Java 資料庫的 psql 種子寫死 store_id，MerID 是 HAWK 加 StoreCode）；
// KYC 是核准的申請內容（含明文的識別欄位）；PayUniMerStatus、PayUniMemberStatus 是 PayUni 回報的原碼，推導出的結果必須是核准。
type Store struct {
	StoreID, StoreCode, AccountID                            int64
	Name, Description, ContactPhone, TownCode, AddressDetail string
	ContactName, ContactMobile, SupportEmail                 string
	LogoKey, LogoAssetID                                     string
	KYC                                                      model.KYCDraftInput
	PayUniMerStatus, PayUniMemberStatus                      string
}

// Deps 外部依賴。
type Deps struct {
	DB     *db.DB
	KYC    *kyc.Service
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Now    func() time.Time
}

// Service 寫入種子店家。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service {
	if d.DB == nil || d.KYC == nil || d.Events == nil || d.Jobs == nil || d.Now == nil {
		panic("seller/devseed: missing dependency")
	}
	return &Service{d: d}
}

// 種子的原則：idempotent，只還原自己擁有的資料，不繞過領域的副作用（像 Rails 的 seeds，不是 db:reset）。種子擁有的是這兩家店的 fixture 資料：
// profile、KYC、PayUni 註冊與 logo；有下游消費者的生命週期狀態（停權措施、關店、等待刪除、刪帳申請）不是種子的資料，種子不悄悄還原它，
// 前提檢查發現它們時拒絕執行（見 Check）。還原的方式：
//   - 店家列 upsert：profile 與個資欄位、版本鎖回到 fixture；status 只因生命週期而變，upsert 不寫它們，只以 WHERE 保證列還在 fixture 的
//     狀態（ACTIVE、沒有關閉紀錄），不是就整筆失敗；logo 與封面是 media 的 ref，不覆寫模組已登記的資產（見 upsertStoreSQL）；
//   - profile 有變時發 StoreProfileChanged 並排入快取失效（與模組自己寫 profile 同一個事件、同一個 transaction），舊服務的快取才看得到；
//     可曝光由最後的全量重新公告（ReannounceExposures）處理；
//   - 公告列保留、不刪不重設：version 只增不減，全量重新公告把它加一，在途的舊事件版本較低，Java 正確丟棄；
//   - KYC 申請、PayUni 註冊與觀測、通報與案件、違規、瀏覽彙總這些「只因使用而存在」的子列，只刪這幾家店的，再以 fixture 重寫；刪掉的證件與佐證圖片先經
//     mediaref 記成退場（同一般的換圖與關店），media 的引用才會釋放；PayUni 的 intent_seq 不重來（取舊值加一、synced_seq 記到新值），在途的 job 結束時 payuni.finish 看到 synced_seq 比自己讀到的 seq 新，不寫入結果（compare-and-set）；
//   - 先鎖店家列（與模組的寫入者同一個順序：店家、申請、註冊），才刪子列；
//   - media_ref_sync 與 retired_assets 不刪（ref 的列刪了再建會讓 seq 從頭算，見 schema.sql 的註解）。
//
// 它不碰別人的資料：不在種子的店家（含 SELLER 角色）原封不動。前提不成立時拒絕執行：01-account.sql 會刪除重建的帳號區間若擁有種子以外
// 的未關閉店家，重建帳號會讓店家失去 SELLER 角色（有店沒角色是壞掉的不變量），Check 在 01 執行之前就擋下，不自動刪店。已關閉的店家（角色已移除）
// 不受影響。
var clearChildrenSQL = []string{
	`DELETE FROM seller.evidence_accesses WHERE report_id IN (SELECT id FROM seller.reports WHERE store_id = ANY($1)) OR request_id IN (SELECT id FROM seller.measure_requests WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))) OR inquiry_id IN (SELECT id FROM seller.case_inquiries WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_request_evidence WHERE request_id IN (SELECT id FROM seller.measure_requests WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.measure_requests WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_identities WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.governance_notes WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1)) OR case_id IN (SELECT id FROM seller.cases WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_findings WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_revisions WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_contents WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measures WHERE store_id = ANY($1)`,
	`DELETE FROM seller.finding_authority_orders WHERE violation_id IN (SELECT id FROM seller.violations WHERE store_id = ANY($1))`,
	// 主管機關來函只要有一個對象碰到種子店家（或其帳號），整份來函連同全部對象一起刪：對象的位置必須連續（DB 的 prev_position 外鍵保證），不能只刪其中幾個。
	// 只有純種子的來函會走到這裡：混合的來函（也有非種子的對象、措施或認定）由 Check 在這些語句之前拒絕（mixedOrdersSQL），不刪非種子的資料。
	// 措施的 source_authority_order_id 隨措施一起刪（上面），認定引用隨認定的清除（上面）。
	`DELETE FROM seller.authority_target_notice_history WHERE order_id IN (SELECT order_id FROM seller.authority_order_targets WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.authority_order_prepurged_stores WHERE order_id IN (SELECT order_id FROM seller.authority_order_targets WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.authority_order_hold_extensions WHERE order_id IN (SELECT order_id FROM seller.authority_order_targets WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.authority_order_targets WHERE order_id IN (SELECT order_id FROM seller.authority_order_targets WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.authority_orders WHERE id NOT IN (SELECT order_id FROM seller.authority_order_targets) AND cardinality($1::bigint[]) >= 0`,
	`DELETE FROM seller.finding_reports WHERE violation_id IN (SELECT id FROM seller.violations WHERE store_id = ANY($1))`,
	`DELETE FROM seller.finding_moderation_reports WHERE violation_id IN (SELECT id FROM seller.violations WHERE store_id = ANY($1))`,
	`DELETE FROM seller.violations WHERE store_id = ANY($1)`,
	`DELETE FROM seller.suspension_periods WHERE store_id = ANY($1)`,
	`DELETE FROM seller.store_blocks WHERE store_id = ANY($1)`,
	`DELETE FROM seller.report_case_moves WHERE store_id = ANY($1)`,
	`DELETE FROM seller.report_evidence WHERE report_id IN (SELECT id FROM seller.reports WHERE store_id = ANY($1))`,
	`DELETE FROM seller.report_supplements WHERE report_id IN (SELECT id FROM seller.reports WHERE store_id = ANY($1))`,
	`DELETE FROM seller.reports WHERE store_id = ANY($1)`,
	`DELETE FROM seller.case_inquiry_reply_evidence WHERE inquiry_id IN (SELECT id FROM seller.case_inquiries WHERE store_id = ANY($1))`,
	`DELETE FROM seller.case_inquiries WHERE store_id = ANY($1)`,
	`DELETE FROM seller.cases WHERE store_id = ANY($1)`,
	`DELETE FROM seller.payuni_observations WHERE store_id = ANY($1)`,
	`DELETE FROM seller.payuni_registrations WHERE store_id = ANY($1)`,
	`DELETE FROM seller.agreement_acceptances WHERE store_id = ANY($1)`,
	`DELETE FROM seller.kyc_document_accesses WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_reveals WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_review_issues WHERE review_id IN (SELECT review_id FROM seller.kyc_reviews WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.kyc_reviews WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_registration_checks WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_documents WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_identity_hashes WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_applications WHERE store_id = ANY($1)`,
	`DELETE FROM seller.product_views_daily WHERE store_id = ANY($1)`,
	`DELETE FROM seller.product_view_events WHERE store_id = ANY($1)`,
}

// 公告列：沒有才建立（version 0，與開店同一個結果，queries/exposure.sql 的 InsertStoreExposure）；已有的保留原樣，版本不重設。
const insertExposureSQL = `INSERT INTO seller.store_exposures (store_id) VALUES ($1) ON CONFLICT (store_id) DO NOTHING`

// profile 的快照：upsert 還原的欄位（不含 status，那是生命週期；不含版本）。前後兩次快照不同才發 StoreProfileChanged。
const profileExpr = `jsonb_build_array(s.store_name, s.description, s.contact_phone, s.contact_name, s.contact_mobile, s.support_email, s.town_code, s.address_detail, s.logo_key, s.logo_asset_id,
    s.prize, s.website_url, s.onboarding_completed)::text`

// 鎖店家列（依 store_id，與 stint.LockAccountOf 同一個順序）並取得還原前的 profile 快照。
const lockStoresSQL = `SELECT s.store_id, ` + profileExpr + ` FROM seller.stores s WHERE s.store_id = ANY($1) ORDER BY s.store_id FOR UPDATE`

const profileSnapshotSQL = `SELECT s.store_id, ` + profileExpr + ` FROM seller.stores s WHERE s.store_id = ANY($1)`

// 申請列、註冊列接著鎖（店家、申請、註冊的順序，同模組的寫入者）；註冊列的 intent_seq 是重寫之後要延續的底。
const (
	lockApplicationsSQL  = `SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1) ORDER BY store_id, application_id FOR UPDATE`
	lockRegistrationsSQL = `SELECT store_id, intent_seq FROM seller.payuni_registrations WHERE store_id = ANY($1) ORDER BY store_id FOR UPDATE`
	setIntentSeqSQL      = `UPDATE seller.payuni_registrations SET intent_seq = $2 WHERE store_id = $1 RETURNING intent_seq`
)

// 即將刪除的證件與佐證圖片（按 ref 分組）：'C' 是 KYC 證件（ref 是申請），'E' 是通報、措施上請求（申覆）與詢問回覆的佐證（ref 是通報、請求或詢問）。
const evidenceAssetsSQL = `
SELECT 'C', d.application_id::text, d.asset_id FROM seller.kyc_documents d
WHERE d.application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))
UNION ALL
SELECT 'E', e.report_id::text, e.asset_id FROM seller.report_evidence e
WHERE e.report_id IN (SELECT id FROM seller.reports WHERE store_id = ANY($1))
UNION ALL
SELECT 'E', a.request_id::text, a.asset_id FROM seller.measure_request_evidence a
WHERE a.request_id IN (SELECT id FROM seller.measure_requests WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1)))
UNION ALL
SELECT 'E', i.inquiry_id::text, i.asset_id FROM seller.case_inquiry_reply_evidence i
WHERE i.inquiry_id IN (SELECT id FROM seller.case_inquiries WHERE store_id = ANY($1))
ORDER BY 1, 2, 3`

const retiredAssetsSQL = `SELECT asset_id FROM seller.retired_assets WHERE ref_type = $1 AND ref_id = $2`

// 店家列 upsert。衝突時把 fixture 擁有的 profile 與個資欄位寫回（版本鎖加一）；first_exposed_at 是歷史事實，保留。
// status 與 pii_purged_at 只因生命週期（關店、等待刪除、個資清理）而變，不是種子的資料：不寫，只在 WHERE 要求它們還在 fixture 的
// 狀態（ACTIVE 的 stint 沒有目前的關店紀錄），否則 0 列、呼叫方整筆失敗（前提檢查已經先擋下，這是同一個 transaction 內的保底）。
// logo、封面是 media 的 ref（media_ref_sync 追蹤資產的同步）：列上已有 logo 時保留，封面不動，不覆寫模組登記過的資產（覆寫而不遞增 ref 的 seq
// 會讓 media 的引用與業務列脫節；種子本身不寫 ref 列，fixture 的 logo 是不經 media 的假資產）。沒有 logo 的列才補上 fixture 的，而且資產 ID 不能是
// 已退場的（current 與 retired 不相交，見 freshLogoAsset）。
const upsertStoreSQL = `
INSERT INTO seller.stores (
    store_id, store_code, account_id, store_name, description, contact_phone, town_code, address_detail,
    logo_key, logo_asset_id, onboarding_completed, created_at, updated_at, contact_name, contact_mobile, support_email
) OVERRIDING SYSTEM VALUE
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, $11, $11, $12, $13, $14)
ON CONFLICT (store_id) DO UPDATE SET
    store_code = EXCLUDED.store_code, account_id = EXCLUDED.account_id,
    version = seller.stores.version + 1,
    store_name = EXCLUDED.store_name, description = EXCLUDED.description, contact_phone = EXCLUDED.contact_phone,
    contact_name = EXCLUDED.contact_name, contact_mobile = EXCLUDED.contact_mobile, support_email = EXCLUDED.support_email,
    town_code = EXCLUDED.town_code, address_detail = EXCLUDED.address_detail,
    logo_key = COALESCE(seller.stores.logo_key, EXCLUDED.logo_key), logo_asset_id = COALESCE(seller.stores.logo_asset_id, EXCLUDED.logo_asset_id),
    prize = NULL, website_url = NULL,
    onboarding_completed = true,
    updated_at = EXCLUDED.updated_at
WHERE seller.stores.status = 'ACTIVE' AND seller.stores.pii_purged_at IS NULL`

// 把 IDENTITY 序號推到固定 ID 之後，新開的店不會撞到它們（同 01-account.sql）。只往上推：取目前序號、現有最大的 store_id 與 9999 的最大值，
// 序號永遠不會倒退。
const advanceSequenceSQL = `
SELECT setval(seq, GREATEST(
    COALESCE((SELECT last_value FROM pg_sequences WHERE schemaname || '.' || sequencename = seq), 0),
    COALESCE((SELECT max(store_id) FROM seller.stores), 0),
    9999))
FROM (SELECT pg_get_serial_sequence('seller.stores', 'store_id') AS seq) s`

// AccountRange 是 01-account.sql 會刪除重建的一段帳號 ID（含兩端）。
type AccountRange struct{ From, To int64 }

// 01 重建的帳號擁有種子以外的未關閉店家、或種子以外的店家佔用種子的 store_code：已關閉的店家沒有 SELLER 角色（關閉時同一個 transaction 移除），
// 01 動不到它的不變量，不在此限（stores_current_account 的 UNIQUE 也只管未關閉的）；store_code 的 unique index 涵蓋所有狀態，所以那一支不篩選。
const conflictingStoresSQL = `
SELECT store_id, account_id, store_code FROM seller.stores
WHERE store_id <> ALL($1)
  AND (store_code = ANY($2)
       OR (status <> 'CLOSED'
           AND EXISTS (SELECT 1 FROM unnest($3::bigint[], $4::bigint[]) r(lo, hi) WHERE account_id BETWEEN r.lo AND r.hi)))
ORDER BY store_id`

// 種子店家身上還在進行的生命週期狀態：不是 ACTIVE（關閉的有目前的關店紀錄）或個資已清理；還沒結束的停權期間（到期之後要等 worker 的 Recompute 結束它並發
// StoreUnsuspended）。期間是帳號層級的，所以不只看種子的 store_id，而是種子帳號名下的所有店家：否則種子帳號在別的（已關閉）任期上有進行中的期間時，
// 種子會替被停權的帳號新開任期。它們都有下游消費者，種子不悄悄還原。（$1 種子 store_id、$2 種子帳號。）
// 生效中的措施（停權與終止合作）不在這裡寫第二份定義：問 stint.EffectiveMany（EffectiveMeasures 那一支查詢，check 讀它）。
const lifecycleStatesSQL = `
SELECT s.store_id, '店家狀態 ' || s.status || CASE WHEN s.current_closed_at IS NOT NULL THEN '（已有關閉紀錄）' ELSE '' END FROM seller.stores s
WHERE s.store_id = ANY($1) AND (s.status <> 'ACTIVE' OR s.pii_purged_at IS NOT NULL)
UNION ALL
SELECT p.store_id, '進行中的停權期間' FROM seller.suspension_periods p
WHERE (p.store_id = ANY($1) OR p.store_id IN (SELECT store_id FROM seller.stores WHERE account_id = ANY($2)))
  AND p.ended_at IS NULL
ORDER BY 1, 2`

// 種子店家在資料庫裡所屬的帳號（種子的帳號 ID 以外，店家列可能屬於別的帳號）。
const seedAccountsSQL = `SELECT DISTINCT account_id FROM seller.stores WHERE store_id = ANY($1) ORDER BY 1`

// effectiveLines 列出種子帳號（種子的帳號加種子店家在資料庫裡所屬的帳號）此刻生效中的措施（停權與終止合作），以 stint.EffectiveMany——
// 「生效中」的定義只有 EffectiveMeasures 一處——判斷，已結束與預定生效的不算。
func effectiveLines(ctx context.Context, conn db.DBTX, storeIDs, seedAccounts []int64, now time.Time) ([]string, error) {
	owned, err := lines(ctx, conn, seedAccountsSQL, func(rows scanner) (string, error) {
		var id int64
		err := rows.Scan(&id)
		return strconv.FormatInt(id, 10), err
	}, storeIDs)
	if err != nil {
		return nil, err
	}
	accounts := slices.Clone(seedAccounts)
	for _, raw := range owned {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, id)
	}
	slices.Sort(accounts)
	accounts = slices.Compact(accounts)
	byAccount, err := stint.EffectiveMany(ctx, store.New(conn), accounts, now)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, id := range accounts {
		for _, m := range byAccount[id].Measures {
			what := "生效中的停權措施（" + string(m.Basis) + "）"
			if m.Kind == model.MeasureTermination {
				what = "生效中的終止合作措施"
			}
			out = append(out, fmt.Sprintf("  種子帳號 %d：%s %s", id, what, m.MeasureID))
		}
	}
	return out, nil
}

// 混合的主管機關來函：碰到種子店家（或其帳號）的來函，同時還有非種子的資料依附在它上面——不碰種子的對象、非種子店家的措施
// （source_authority_order_id）或非種子店家認定的引用（finding_authority_orders）。種子只能整份刪來函（對象位置必須連續），
// 刪它會連帶刪掉非種子的對象與延長歷史、並撞上措施與認定的外鍵；種子不擁有這些資料，所以拒絕並列出來函 id（同停權措施的做法）。
// 「碰到種子」與 clearChildrenSQL 的刪除條件是同一個謂詞。「非種子」與 clearChildrenSQL 擁有的範圍一致，以店家為單位：措施、認定與指名店家的對象
// 看 store_id 是不是種子店家（種子帳號較早關閉的 stint 不是種子店家，它的資料不歸種子）；不指定店家的對象（store_id 為 NULL）沒有店家可看，
// 看帳號是不是種子帳號。（$1 種子 store_id。）
const mixedOrdersSQL = `
WITH touching AS (
    SELECT DISTINCT order_id FROM seller.authority_order_targets
    WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1))
)
SELECT t.order_id::text,
       EXISTS (SELECT 1 FROM seller.authority_order_targets x WHERE x.order_id = t.order_id
               AND CASE WHEN x.store_id IS NULL
                        THEN x.account_id NOT IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1))
                        ELSE x.store_id <> ALL($1) END),
       EXISTS (SELECT 1 FROM seller.measures m WHERE m.source_authority_order_id = t.order_id AND m.store_id <> ALL($1)),
       EXISTS (SELECT 1 FROM seller.finding_authority_orders f JOIN seller.violations v ON v.id = f.violation_id
               WHERE f.order_id = t.order_id AND v.store_id <> ALL($1))
FROM touching t
ORDER BY 1`

// Check 在不寫任何資料的前提下確認種子可以安全執行：重建的帳號區間（rebuilt）不擁有種子以外的未關閉店家，沒有種子以外的店家佔用種子的
// store_code，而且種子店家沒有還在進行的生命週期狀態（停權措施、關閉、等待刪除）。不成立時回錯誤（列出店家與處理方式），不自動刪店、不還原狀態。
// （種子帳號的刪帳申請由 accountdeletion 擁有，呼叫方在 cmd 另外檢查。）
func (s *Service) Check(ctx context.Context, stores []Store, rebuilt []AccountRange) error {
	return check(ctx, s.d.DB.Conn(ctx), stores, rebuilt, s.d.Now().UTC())
}

func check(ctx context.Context, conn db.DBTX, stores []Store, rebuilt []AccountRange, now time.Time) error {
	if err := validate(stores); err != nil {
		return err
	}
	ids, codes, accountIDs := make([]int64, len(stores)), make([]int64, len(stores)), make([]int64, len(stores))
	for i, st := range stores {
		ids[i], codes[i], accountIDs[i] = st.StoreID, st.StoreCode, st.AccountID
	}
	lo, hi := make([]int64, len(rebuilt)), make([]int64, len(rebuilt))
	for i, r := range rebuilt {
		lo[i], hi[i] = r.From, r.To
	}
	conflicts, err := lines(ctx, conn, conflictingStoresSQL, func(rows scanner) (string, error) {
		var id, account, code int64
		if err := rows.Scan(&id, &account, &code); err != nil {
			return "", err
		}
		return fmt.Sprintf("  店家 %d（帳號 %d、store_code %d）", id, account, code), nil
	}, ids, codes, lo, hi)
	if err != nil {
		return err
	}
	states, err := lines(ctx, conn, lifecycleStatesSQL, func(rows scanner) (string, error) {
		var id int64
		var what string
		if err := rows.Scan(&id, &what); err != nil {
			return "", err
		}
		return fmt.Sprintf("  種子帳號名下的店家 %d：%s", id, what), nil
	}, ids, accountIDs)
	if err != nil {
		return err
	}
	measures, err := effectiveLines(ctx, conn, ids, accountIDs, now)
	if err != nil {
		return err
	}
	states = append(states, measures...)
	mixed, err := lines(ctx, conn, mixedOrdersSQL, func(rows scanner) (string, error) {
		var id string
		var targets, derived, findings bool
		if err := rows.Scan(&id, &targets, &derived, &findings); err != nil {
			return "", err
		}
		var what []string
		if targets {
			what = append(what, "有不碰種子的對象")
		}
		if derived {
			what = append(what, "非種子店家的措施依它而生")
		}
		if findings {
			what = append(what, "非種子店家的認定引用它")
		}
		if len(what) == 0 {
			return "", nil
		}
		return fmt.Sprintf("  主管機關來函 %s：%s", id, strings.Join(what, "、")), nil
	}, ids)
	if err != nil {
		return err
	}
	mixed = slices.DeleteFunc(mixed, func(l string) bool { return l == "" })
	var sections []string
	if len(conflicts) > 0 {
		sections = append(sections, "下列店家屬於 01-account.sql 會重建的帳號，或佔用種子的 store_code；重建帳號會讓未關閉的店家失去 SELLER 角色。\n"+strings.Join(conflicts, "\n")+"\n"+
			"請重建本機資料庫（01 的前提：改用這份 seed 前先重建本機 DB）；手動測試改用全新的 dev_<名稱> 帳號（.agents/skills/backend-e2e-verify/SKILL.md §2），不要用種子的帳號區間")
	}
	if len(states) > 0 {
		sections = append(sections, "種子店家有還在進行的生命週期狀態；它們有下游消費者（事件、停權期間、刪帳流程），種子不繞過它們去還原。\n"+strings.Join(states, "\n")+"\n"+
			"處理：停權措施用後台 API 結束或撤銷（SellerGovernanceAdminService.EndMeasure、RevokeMeasure 或裁定申覆，"+
			"到期的停權要等 worker 結束停權期間）；關店與等待刪除沒有還原的 API，請重建本機資料庫")
	}
	if len(mixed) > 0 {
		sections = append(sections, "下列主管機關來函同時碰到種子店家與非種子的資料；種子只能整份刪除來函，會連帶刪掉不屬於它的資料，所以不清除。\n"+strings.Join(mixed, "\n")+"\n"+
			"請重建本機資料庫，或先在後台處理掉非種子的措施與認定")
	}
	if len(sections) == 0 {
		return nil
	}
	return errors.New("dev-seed 中止，沒有寫入任何資料：" + strings.Join(sections, "\n\n"))
}

type scanner interface{ Scan(dest ...any) error }

func lines(ctx context.Context, conn db.DBTX, query string, line func(scanner) (string, error), args ...any) ([]string, error) {
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("seller：檢查種子的前提：%w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		l, err := line(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Seed 在一個 transaction 內把種子店家還原成 fixture（upsert，見檔頭的說明）；先鎖店家列再做 Check，前提不成立時不寫任何資料。
// 必須在 transaction 外呼叫。
func (s *Service) Seed(ctx context.Context, stores []Store, rebuilt []AccountRange) error {
	if _, ok := db.TxFrom(ctx); ok {
		return errors.New("seller：種子必須在 transaction 外開始")
	}
	if len(stores) == 0 {
		return errors.New("seller：沒有種子店家")
	}
	if err := validate(stores); err != nil {
		return err
	}
	ids := make([]int64, len(stores))
	for i, st := range stores {
		ids[i] = st.StoreID
	}
	now := s.d.Now().UTC()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		conn := s.d.DB.Conn(ctx)
		before, err := snapshots(ctx, conn, lockStoresSQL, ids)
		if err != nil {
			return fmt.Errorf("seller：鎖住種子店家：%w", err)
		}
		if err := check(ctx, conn, stores, rebuilt, now); err != nil {
			return err
		}
		intentSeqs, err := lockPayUni(ctx, conn, ids)
		if err != nil {
			return fmt.Errorf("seller：鎖住種子店家的申請與註冊：%w", err)
		}
		q := store.New(conn)
		if err := s.retireEvidence(ctx, conn, q, ids, now); err != nil {
			return err
		}
		for _, stmt := range clearChildrenSQL {
			if _, err := conn.Exec(ctx, stmt, ids); err != nil {
				return fmt.Errorf("seller：清除種子店家的使用痕跡：%w", err)
			}
		}
		for _, st := range stores {
			if err := s.seedOne(ctx, q, st, now, intentSeqs[st.StoreID]); err != nil {
				return fmt.Errorf("seller：種子店家 %d：%w", st.StoreID, err)
			}
		}
		if err := s.publishProfileChanges(ctx, conn, q, stores, ids, before, now); err != nil {
			return err
		}
		_, err = conn.Exec(ctx, advanceSequenceSQL)
		return err
	})
}

// snapshots 回傳每家店的 profile 快照（lockStoresSQL 同時鎖住那些列）。
func snapshots(ctx context.Context, conn db.DBTX, query string, ids []int64) (map[int64]string, error) {
	rows, err := conn.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var snap string
		if err := rows.Scan(&id, &snap); err != nil {
			return nil, err
		}
		out[id] = snap
	}
	return out, rows.Err()
}

// lockPayUni 依「店家、申請、註冊」的順序鎖住申請與註冊列，回傳註冊列目前的 intent_seq。
func lockPayUni(ctx context.Context, conn db.DBTX, ids []int64) (map[int64]int64, error) {
	rows, err := conn.Query(ctx, lockApplicationsSQL, ids)
	if err != nil {
		return nil, err
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = conn.Query(ctx, lockRegistrationsSQL, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var id, seq int64
		if err := rows.Scan(&id, &seq); err != nil {
			return nil, err
		}
		out[id] = seq
	}
	return out, rows.Err()
}

// retireEvidence 在刪除之前，把種子店家的 KYC 證件與通報、申覆的佐證圖片記成退場並排入同步（mediaref.Changed，與一般的換圖、關店同一個機制），
// media 的引用才會隨業務列一起釋放。
func (s *Service) retireEvidence(ctx context.Context, conn db.DBTX, q *store.Queries, ids []int64, now time.Time) error {
	type key struct{ kind, id string }
	var order []key
	assets := map[key][]string{}
	rows, err := conn.Query(ctx, evidenceAssetsSQL, ids)
	if err != nil {
		return fmt.Errorf("seller：讀種子店家的證件與佐證：%w", err)
	}
	for rows.Next() {
		var k key
		var asset string
		if err := rows.Scan(&k.kind, &k.id, &asset); err != nil {
			rows.Close()
			return err
		}
		if _, ok := assets[k]; !ok {
			order = append(order, k)
		}
		assets[k] = append(assets[k], asset)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, k := range order {
		id, err := uuid.Parse(k.id)
		if err != nil {
			return err
		}
		ref := mediaref.EvidenceRef(id)
		if k.kind == "C" {
			ref = mediaref.CertificationRef(id)
		}
		if err := mediaref.Changed(ctx, q, s.d.Jobs, now, ref, assets[k]...); err != nil {
			return fmt.Errorf("seller：退場種子店家的證件與佐證：%w", err)
		}
	}
	return nil
}

// publishProfileChanges 對 profile 在還原之後與之前不同的店家（新建的列沒有「之前」，也算）發 StoreProfileChanged 並排入快取失效：
// 與 profile.Service.changed 同一組呼叫，同一個 transaction。
func (s *Service) publishProfileChanges(ctx context.Context, conn db.DBTX, q *store.Queries, stores []Store, ids []int64, before map[int64]string, now time.Time) error {
	after, err := snapshots(ctx, conn, profileSnapshotSQL, ids)
	if err != nil {
		return err
	}
	for _, st := range stores {
		if prev, ok := before[st.StoreID]; ok && prev == after[st.StoreID] {
			continue
		}
		if err := events.Publish(ctx, s.d.Events, model.StoreProfileChanged{EventID: uuid.NewV7(), OccurredAt: now, StoreID: st.StoreID}); err != nil {
			return err
		}
		row, err := q.StoreByID(ctx, st.StoreID)
		if err != nil {
			return err
		}
		if err := stint.Invalidate(ctx, s.d.Jobs, row); err != nil {
			return err
		}
	}
	return nil
}

// freshLogoAsset 回傳 fixture 的 logo 資產 ID；它已經在這家店 logo ref 的退場名單裡（例如店家曾經關閉）就換一個沒用過的，
// 讓「目前的資產」與「退場的資產」不相交（mediaref.Reject 在模組的寫入端守的同一條不變量）。
func freshLogoAsset(base string, retired []string) string {
	id := base
	for n := 2; slices.Contains(retired, id); n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

func validate(stores []Store) error {
	var ids, codes, accounts []int64
	for _, st := range stores {
		if st.StoreID <= 0 || st.StoreCode <= 0 || st.AccountID <= 0 {
			return fmt.Errorf("seller：種子店家的 store_id、store_code、account_id 必須是正整數：%+v", st.StoreID)
		}
		if slices.Contains(ids, st.StoreID) || slices.Contains(codes, st.StoreCode) || slices.Contains(accounts, st.AccountID) {
			return fmt.Errorf("seller：種子店家 %d 的 store_id、store_code 或 account_id 重複", st.StoreID)
		}
		ids, codes, accounts = append(ids, st.StoreID), append(codes, st.StoreCode), append(accounts, st.AccountID)
		if st.Name == "" || st.TownCode == "" || st.LogoKey == "" || st.LogoAssetID == "" {
			return fmt.Errorf("seller：種子店家 %d 缺少店名、鄉鎮市區代碼或 logo（可曝光的條件）", st.StoreID)
		}
		if model.PayUniOutcomeOf(st.PayUniMerStatus, st.PayUniMemberStatus) != model.OutcomeApproved {
			return fmt.Errorf("seller：種子店家 %d 的 PayUni 狀態碼（%q、%q）不是核准", st.StoreID, st.PayUniMerStatus, st.PayUniMemberStatus)
		}
	}
	return nil
}

func (s *Service) seedOne(ctx context.Context, q *store.Queries, st Store, now time.Time, prevIntentSeq int64) error {
	conn := s.d.DB.Conn(ctx)
	logoRef := mediaref.StoreRef(media.RefTypeSellerLogo, st.StoreID)
	retired, err := retiredAssets(ctx, conn, logoRef)
	if err != nil {
		return err
	}
	tag, err := conn.Exec(ctx, upsertStoreSQL, st.StoreID, st.StoreCode, st.AccountID, st.Name, st.Description, st.ContactPhone,
		st.TownCode, st.AddressDetail, st.LogoKey, freshLogoAsset(st.LogoAssetID, retired), now,
		nullable(st.ContactName), nullable(st.ContactMobile), nullable(st.SupportEmail))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("店家列不在 fixture 的生命週期狀態（ACTIVE、沒有關閉紀錄），種子不還原它")
	}
	if _, err := conn.Exec(ctx, insertExposureSQL, st.StoreID); err != nil {
		return err
	}
	app, submitID, err := s.d.KYC.SeedApproved(ctx, q, st.StoreID, st.KYC, now)
	if err != nil {
		return err
	}
	if err := agreement.Record(ctx, q, st.StoreID, st.AccountID, current(model.ApplyAgreements), now, agreement.Subject{}, agreement.Source{}); err != nil {
		return err
	}
	if err := agreement.Record(ctx, q, st.StoreID, st.AccountID, current(model.SubmitKYCAgreements), now, agreement.Subject{SubmissionReviewID: &submitID}, agreement.Source{}); err != nil {
		return err
	}
	row, err := q.StoreByID(ctx, st.StoreID)
	if err != nil {
		return err
	}
	reg, err := q.UpsertPayUniIntent(ctx, payuni.IntentParams(payuni.BindIntent{Store: row, Application: app}, now))
	if err != nil {
		return err
	}
	// 註冊列是重建的，intent_seq 不重來：延續舊值加一，下面再把 synced_seq 記到新值。在途的 job（以舊的 seq 讀的資料）結束時，payuni.finish
	// 在鎖內發現 synced_seq 比自己的 seq 新，不寫入、不再跑一輪：結果不會覆蓋重建的列。
	if reg.IntentSeq <= prevIntentSeq {
		if err := conn.QueryRow(ctx, setIntentSeqSQL, st.StoreID, prevIntentSeq+1).Scan(&reg.IntentSeq); err != nil {
			return err
		}
	}
	if _, err := q.MarkPayUniBound(ctx, store.MarkPayUniBoundParams{StoreID: st.StoreID, SyncedSeq: reg.IntentSeq, Now: &now}); err != nil {
		return err
	}
	_, err = q.ApplyPayUniStatus(ctx, store.ApplyPayUniStatusParams{
		StoreID: st.StoreID, MerStatus: &st.PayUniMerStatus, MemberStatus: &st.PayUniMemberStatus,
		Outcome: string(model.PayUniOutcomeOf(st.PayUniMerStatus, st.PayUniMemberStatus)), ExecutedAt: &now, Now: now,
	})
	return err
}

func current(kinds []model.AgreementKind) []model.AgreementAcceptance {
	out := make([]model.AgreementAcceptance, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, model.AgreementAcceptance{Kind: k, Version: model.CurrentAgreements[k].Version})
	}
	return out
}

func retiredAssets(ctx context.Context, conn db.DBTX, ref media.Ref) ([]string, error) {
	rows, err := conn.Query(ctx, retiredAssetsSQL, ref.Type, ref.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
