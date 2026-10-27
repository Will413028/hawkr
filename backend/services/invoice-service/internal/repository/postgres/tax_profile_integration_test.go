//go:build integration

// 本檔跑真的 SQL，驗 M3 稅務資格狀態機寫入面（§7.3.3 T01–T16）。
//
// 這些規則**沒有一條**是單元測試驗得到的：
//   - 12 支 CAS 的 allowed-from 述詞邊界（fake 照抄一份，抄錯就兩邊一起錯）
//   - `chk_stp_pending_window`：SELF_ISSUE_PENDING／SUPPLEMENT_REQUIRED 必須有
//     submitted_at ＋ pending_valid_until。**fake 沒有 CHECK** ⇒ 對它零鑑別力，
//     而少寫任一欄的症狀是 23514 而不是靜默寫壞
//   - `chk_stp_required_needs_taxid` / `chk_stp_exempt_no_taxid`：核定成
//     APPROVED_INVOICE_REQUIRED 卻沒有統編、或核定成 EXEMPT 卻帶著統編，兩者都由
//     DB 擋（Approve 用 next_status 參數共用一句，正是靠這兩條 CHECK 不讓錯誤組合過關）
//   - `chk_stp_extension`：extension_count 值域 0–1
//   - ResetToDraft 的 7 天冷卻述詞（決策 C3）
package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// tTermsVersion 是本檔用的自行開票責任條款版本字串。
//
// 刻意**不 import internal/usecase 的 SelfIssueTermsVersion**：repository 層對版本值
// 沒有任何語意，它只負責「傳什麼寫什麼」；反向 import 會把依賴方向倒過來。
// 「Go 端傳的是不是當期版本」由 usecase 那層的測試守。
const tTermsVersion = "2026-10-25"

// newTaxFixture 建一列 issuer ＋ 一列 seller_tax_profiles，回 repository 與 profile。
// tAttestedBy 是簽署人的 account_id（#3031）。非 0 才驗得到「三元組一起寫」——
// 傳 0 會被 nzp 折成 NULL，那一格就退化成與修這件事之前相同。
const tAttestedBy int64 = 987654321

func newTaxFixture(t *testing.T, status string) (*SellerTaxProfileRepository, context.Context, *pgxpool.Pool, int64, int64) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)
	issuerID := seedIssuerFor(ctx, t, pool, 2685914682)
	repo := NewSellerTaxProfileRepository(pool)

	profile, inserted, err := repo.Create(ctx, nextID(), issuerID, time.Now().UTC())
	if err != nil || !inserted {
		t.Fatalf("建列失敗：inserted=%v err=%v", inserted, err)
	}
	if status != "DRAFT" {
		seedTaxStatus(ctx, t, pool, profile.ID, status)
	}
	return repo, ctx, pool, issuerID, profile.ID
}

// seedTaxStatus 直接把列推到指定狀態。
//
// 🔴 一併補上該狀態需要的時鐘欄位：chk_stp_pending_window 對 SELF_ISSUE_PENDING 與
// SUPPLEMENT_REQUIRED 要求 submitted_at ＋ pending_valid_until 皆非空，少寫就 23514
// ——那正是本檔要驗的其中一條，seed 自己踩到會變成 seed 失敗而不是測試失敗。
//
// 🔴 同理，chk_stp_attested_when_committed 對 SUBMITTED 與 SELF_ISSUE_PENDING 要求
// 簽署記錄兩欄皆非空（裁決 C 的不變式）⇒ seed 也得寫。**這正是那條 CHECK 的價值**：
// 「把列推到已送審而沒有簽署記錄」在 DB 層就辦不到，連測試 seed 都繞不過去。
func seedTaxStatus(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64, status string) {
	t.Helper()
	now := time.Now().UTC()
	switch status {
	case "SUBMITTED":
		mustExec(ctx, t, pool,
			`UPDATE seller_tax_profiles SET review_status = $2, submitted_at = $3, pending_valid_until = $4,
			        attestation_version = $5, attested_at = $3 WHERE id = $1`,
			id, status, now.Add(-24*time.Hour), now.Add(30*24*time.Hour), tTermsVersion)
	case "SELF_ISSUE_PENDING":
		mustExec(ctx, t, pool,
			`UPDATE seller_tax_profiles SET review_status = $2, submitted_at = $3, pending_valid_until = $4,
			        attestation_version = $5, attested_at = $3 WHERE id = $1`,
			id, status, now.Add(-24*time.Hour), now.Add(30*24*time.Hour), tTermsVersion)
	case "SUPPLEMENT_REQUIRED":
		mustExec(ctx, t, pool,
			`UPDATE seller_tax_profiles SET review_status = $2, submitted_at = $3, pending_valid_until = $4
			 WHERE id = $1`, id, status, now.Add(-24*time.Hour), now.Add(30*24*time.Hour))
	case "APPROVED_INVOICE_REQUIRED":
		mustExec(ctx, t, pool,
			`UPDATE seller_tax_profiles SET review_status = $2, tax_id = '12345678', legal_name = '範例生鮮股份有限公司',
			        approved_at = $3, approval_valid_until = $4 WHERE id = $1`,
			id, status, now, now.Add(365*24*time.Hour))
	case "APPROVED_INVOICE_EXEMPT":
		mustExec(ctx, t, pool,
			`UPDATE seller_tax_profiles SET review_status = $2, tax_id = NULL, legal_name = '某某工作室',
			        approved_at = $3, approval_valid_until = $4 WHERE id = $1`,
			id, status, now, now.Add(365*24*time.Hour))
	default:
		mustExec(ctx, t, pool, `UPDATE seller_tax_profiles SET review_status = $2 WHERE id = $1`, id, status)
	}
}

func taxRow(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) domain.SellerTaxProfile {
	t.Helper()
	row, err := NewSellerTaxProfileRepository(pool).GetByID(ctx, id)
	if err != nil {
		t.Fatalf("讀回失敗：%v", err)
	}
	return row
}

// T01：Create 是 ON CONFLICT DO NOTHING ⇒ 第二次回 inserted=false 而不是 23505。
//
// 🔴 形狀必須是 inserted=false 而不是 error：ON CONFLICT 若指名了 conflict target，
// 衝突會變成 23505 而整個交易 aborted（25P02），之後那句「改讀既有列」的 SELECT
// 會炸成 500，而正確行為是把既有列回給呼叫端。
func TestIntegration_TaxProfile_CreateIsIdempotent(t *testing.T) {
	repo, ctx, pool, issuerID, first := newTaxFixture(t, "DRAFT")

	_, inserted, err := repo.Create(ctx, nextID(), issuerID, time.Now().UTC())
	if err != nil {
		t.Fatalf("第二次建列不該是 error：%v", err)
	}
	if inserted {
		t.Fatal("uq_seller_tax_profiles_issuer 應該擋下第二列")
	}
	// 交易沒有 aborted：後續查詢仍然可用。
	if taxRow(ctx, t, pool, first).ReviewStatus != "DRAFT" {
		t.Fatal("建列後狀態應為 DRAFT（DDL DEFAULT）")
	}
}

// T02：Submit 的 allowed-from 只有 DRAFT，且 submitted_at 與 pending_valid_until
// 必須同一句寫（下游狀態的 chk_stp_pending_window 靠它）。
func TestIntegration_TaxProfile_SubmitCASAndClock(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()
	until := now.Add(60 * 24 * time.Hour)

	// 陰性①：版本不符 ⇒ rows=0。
	ok, err := repo.Submit(ctx, id, 99, tTermsVersion, tAttestedBy, until, now)
	if err != nil || ok {
		t.Fatalf("版本不符應 rows=0：ok=%v err=%v", ok, err)
	}
	// 陽性。
	ok, err = repo.Submit(ctx, id, 0, tTermsVersion, tAttestedBy, until, now)
	if err != nil || !ok {
		t.Fatalf("DRAFT → SUBMITTED 應成功：ok=%v err=%v", ok, err)
	}
	row := taxRow(ctx, t, pool, id)
	if row.ReviewStatus != "SUBMITTED" || row.Version != 1 {
		t.Fatalf("status=%s version=%d，want SUBMITTED/1", row.ReviewStatus, row.Version)
	}
	if row.SubmittedAt == nil || row.PendingValidUntil == nil {
		t.Fatal("單一時鐘的兩欄必須同一句寫，否則 T04／T05 當場 23514")
	}
	// 🔴 裁決 C 的不變式在 **SQL 層**的實證：狀態、兩個時鐘欄與條款兩欄由同一句
	// UPDATE 寫 ⇒ 真實 PostgreSQL 上不存在「已 SUBMITTED 但沒簽條款」的中間態。
	// 這一支與 usecase 那支不重複：那邊驗的是 fake，這邊驗的是實際跑的 SQL。
	if row.AttestationVersion == nil || *row.AttestationVersion != tTermsVersion {
		t.Fatalf("attestation_version = %v，want %q（同一句沒寫到）",
			row.AttestationVersion, tTermsVersion)
	}
	if row.AttestedAt == nil {
		t.Fatal("attested_at 必須與狀態同一句寫")
	}
	// 陰性②：已經 SUBMITTED 再送一次 ⇒ allowed-from 不符 ⇒ rows=0。
	ok, _ = repo.Submit(ctx, id, row.Version, tTermsVersion, tAttestedBy, until, now)
	if ok {
		t.Fatal("SUBMITTED 不是 T02 的來源狀態")
	}
}

// 🔴 T02 × T04 的組合：端點 19 之後列仍是 SUBMITTED ⇒ T04（補簽）的 allowed-from
// 依然成立（裁決 C 逐字：「T04 保留給補簽等例外路徑」）。
//
// 這一支存在的理由：裁決把簽署搬進送審之後，很容易以為 T04 就此變成死碼。
// 死碼與活碼的差別在 SQL 的 WHERE，不在註解——所以只能實跑一次。
func TestIntegration_TaxProfile_AttestStillReachableAfterSubmitWithTerms(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()
	until := now.Add(60 * 24 * time.Hour)

	if ok, err := repo.Submit(ctx, id, 0, tTermsVersion, tAttestedBy, until, now); err != nil || !ok {
		t.Fatalf("前置 T02 失敗：ok=%v err=%v", ok, err)
	}
	row := taxRow(ctx, t, pool, id)

	// 🔴 端到端的中段：送審之後、具結之前，**條款已簽但責任尚未生效**。
	// 這一格是裁決 C 造出來的新狀態，也是整條鏈最容易被判錯的一格：
	//   ① `SelfIssueEffective()` 必須是 false —— true 代表判準退回讀 attested_at，
	//      S01／S02 會在後端還是 LEGACY_MANUAL 的時候宣告「發票由你自行開立」。
	//   ② 列必須停在 SUBMITTED —— 那正是 58A §S04 主 CTA 的渲染條件
	//      （`taxReviewStatus === "SUBMITTED"`）。若這裡不是 SUBMITTED，
	//      S04 的 CTA 不渲染 ⇒ 端點 22 按不到 ⇒ SELF_ISSUE_PENDING 不可達。
	if row.AttestedAt == nil {
		t.Fatal("送審那一句就該把條款簽好（裁決 C）")
	}
	if row.SelfIssueEffective() {
		t.Fatal("SUBMITTED：條款已簽但自行開票責任尚未生效，effective 必須是 false")
	}
	if row.ReviewStatus != "SUBMITTED" {
		t.Fatalf("列必須停在 SUBMITTED（S04 主 CTA 的渲染條件），實際 %s", row.ReviewStatus)
	}

	ok, err := repo.Attest(ctx, id, row.Version, tTermsVersion, tAttestedBy, now)
	if err != nil || !ok {
		t.Fatalf("送審已帶條款之後，T04 仍必須走得通：ok=%v err=%v", ok, err)
	}
	after := taxRow(ctx, t, pool, id)
	if after.ReviewStatus != "SELF_ISSUE_PENDING" {
		t.Fatalf("status=%s，want SELF_ISSUE_PENDING", after.ReviewStatus)
	}
	// 端到端的末段：走完 T04 之後，責任才生效。
	if !after.SelfIssueEffective() {
		t.Fatal("SELF_ISSUE_PENDING：自行開票責任必須生效（S01 的琥珀 pill 就靠它）")
	}
	// 60 天窗口不得因為補簽而重算（決策 C4 的單一時鐘，裁決 C 明令不動）。
	if !after.PendingValidUntil.Equal(*row.PendingValidUntil) {
		t.Fatalf("pending_valid_until 被重算了：%v → %v", row.PendingValidUntil, after.PendingValidUntil)
	}
}

// 陰性（SQL 層）：attestation_version 傳空 ⇒ **整句失敗、一列不寫**。
//
// 🔴 這一支換過方向，舊版是錯的。 舊的 SQL 用 COALESCE／CASE 把空版本「退化成保留舊值」，
// 而實測那個退化的真實後果是：**該列被寫成 SUBMITTED、兩個條款欄留 NULL，呼叫端還收到
// ok=true** —— 一次沒有簽署記錄的送審，而且完全靜默。裁決 C 的承諾（送審成功 ⇒ 條款
// 必已簽）當場被自己的 SQL 破壞。現在無條件寫入 ＋ 三條 CHECK，空字串一律 23514。
//
// 它在正常路徑上走不到（Go 端 accepted_terms=false 已在交易外擋成 9325）⇒ 沒有這一支，
// 那條不變式在 SQL 層沒有任何東西在守。
func TestIntegration_TaxProfile_SubmitEmptyAttestationIsRejected(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()
	until := now.Add(60 * 24 * time.Hour)
	before := taxRow(ctx, t, pool, id)

	ok, err := repo.Submit(ctx, id, before.Version, "", 0, until, now)
	if err == nil {
		t.Fatalf("空條款版本的 T02 必須失敗（chk_stp_attestation_version_nonblank），實際 ok=%v", ok)
	}
	if ok {
		t.Fatal("失敗的 T02 不得回 ok=true")
	}
	after := taxRow(ctx, t, pool, id)
	if after.ReviewStatus != "DRAFT" {
		t.Fatalf("一列都不該寫，狀態卻變成 %s —— 這正是舊 COALESCE 形狀的靜默失效", after.ReviewStatus)
	}
	if after.AttestationVersion != nil || after.AttestedAt != nil {
		t.Fatalf("失敗的送審不得留下半套簽署記錄：%v / %v", after.AttestationVersion, after.AttestedAt)
	}
	if after.Version != before.Version {
		t.Fatalf("失敗的送審不得推進樂觀鎖版本：%d → %d", before.Version, after.Version)
	}
}

// 結構層不變式：**任何**寫入路徑都不能把列寫成「已送審／責任生效中而無簽署記錄」。
//
// 🔴 這一支刻意繞過 repository、直接下裸 SQL —— 上面那支驗的是「我們這一句寫對了」，
// 這一支驗的是「就算有人另外寫一句，DB 也不讓他寫壞」。裁決 C 之前這條保證只有
// usecase 那一道 9325 守衛（一道防線、且在交易之外），新 query／維運 SQL／未來的
// ADM 代送都繞得過，而繞過去之後沒有任何訊號。
func TestIntegration_TaxProfile_AttestationInvariantsAreEnforcedBySchema(t *testing.T) {
	_, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()

	cases := []struct {
		name string
		sql  string
		args []any
	}{
		{
			"SUBMITTED 而無簽署記錄（chk_stp_attested_when_committed）",
			`UPDATE seller_tax_profiles SET review_status='SUBMITTED',
			   submitted_at=$2, pending_valid_until=$2 WHERE id=$1`,
			[]any{id, now},
		},
		{
			"SELF_ISSUE_PENDING 而無簽署記錄（責任生效中卻查不到憑據）",
			`UPDATE seller_tax_profiles SET review_status='SELF_ISSUE_PENDING',
			   submitted_at=$2, pending_valid_until=$2 WHERE id=$1`,
			[]any{id, now},
		},
		{
			"只有時點沒有版本（chk_stp_attestation_pair）",
			`UPDATE seller_tax_profiles SET attested_at=$2 WHERE id=$1`,
			[]any{id, now},
		},
		{
			"只有版本沒有時點（chk_stp_attestation_pair）",
			`UPDATE seller_tax_profiles SET attestation_version='v1' WHERE id=$1`,
			[]any{id},
		},
		{
			"版本是空字串（chk_stp_attestation_version_nonblank）",
			`UPDATE seller_tax_profiles SET attestation_version='', attested_at=$2 WHERE id=$1`,
			[]any{id, now},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, c.sql, c.args...); err == nil {
				t.Fatalf("DB 應該擋下這一句，卻寫成功了：%s", c.sql)
			}
		})
	}

	// 鑑別力：合法的那一組必須寫得進去，否則上面五條全紅只是因為語法壞掉。
	if _, err := pool.Exec(ctx,
		`UPDATE seller_tax_profiles SET review_status='SUBMITTED',
		   submitted_at=$2, pending_valid_until=$2,
		   attestation_version=$3, attested_at=$2 WHERE id=$1`,
		id, now, tTermsVersion); err != nil {
		t.Fatalf("合法的送審寫入被誤擋：%v", err)
	}
}

// T04：Attest 的 allowed-from 是 SUBMITTED，且**不重算 pending_valid_until**
// （決策 C4 的單一時鐘：重算等於讓賣家用「停在未接受條款」把窗口無限延長）。
//
// 這一支同時是 chk_stp_pending_window 的實證：轉進 SELF_ISSUE_PENDING 而 SQL 沒帶
// 那兩欄，靠的是 T02 已經寫好了它們。
func TestIntegration_TaxProfile_AttestKeepsDeadline(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()
	until := now.Add(60 * 24 * time.Hour)
	if ok, err := repo.Submit(ctx, id, 0, tTermsVersion, tAttestedBy, until, now); err != nil || !ok {
		t.Fatalf("前置 T02 失敗：%v", err)
	}
	before := taxRow(ctx, t, pool, id)

	ok, err := repo.Attest(ctx, id, before.Version, "v1", tAttestedBy, now)
	if err != nil || !ok {
		t.Fatalf("T04 應成功：ok=%v err=%v", ok, err)
	}
	after := taxRow(ctx, t, pool, id)
	if after.ReviewStatus != "SELF_ISSUE_PENDING" {
		t.Fatalf("status=%s，want SELF_ISSUE_PENDING", after.ReviewStatus)
	}
	if !after.PendingValidUntil.Equal(*before.PendingValidUntil) {
		t.Fatalf("pending_valid_until 被重算了：%v → %v", before.PendingValidUntil, after.PendingValidUntil)
	}
	if after.AttestedAt == nil || after.AttestationVersion == nil {
		t.Fatal("attested_at 與 attestation_version 必須同一句寫（T04 的守衛條件）")
	}
}

// T01 第二形態：ResetToDraft 的 7 天冷卻述詞（決策 C3）。
//
// 🔴 守衛寫在 SQL 的 WHERE 不是應用層的 if：寫在 if 會在併發重送時同時通過兩次。
func TestIntegration_TaxProfile_ResetToDraftHonoursCooldown(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()

	// 走完整條 T09 讓 cooldown_until 由 SQL 寫入（而不是 seed 直接塞）。
	seedTaxStatus(ctx, t, pool, id, "UNDER_REVIEW")
	row := taxRow(ctx, t, pool, id)
	if ok, err := repo.Reject(ctx, id, row.Version, "統編不符", now.Add(7*24*time.Hour), now); err != nil || !ok {
		t.Fatalf("T09 應成功：ok=%v err=%v", ok, err)
	}
	if got := taxRow(ctx, t, pool, id); got.CooldownUntil == nil {
		t.Fatal("T09 必須寫 cooldown_until，少了它被駁回的賣家可以立刻無限重送")
	}

	// 陰性：冷卻未到 ⇒ rows=0。
	if ok, err := repo.ResetToDraft(ctx, id, now); err != nil || ok {
		t.Fatalf("冷卻未到應 rows=0：ok=%v err=%v", ok, err)
	}
	// 陽性：時間走到冷卻之後。
	if ok, err := repo.ResetToDraft(ctx, id, now.Add(8*24*time.Hour)); err != nil || !ok {
		t.Fatalf("冷卻屆滿應可重申：ok=%v err=%v", ok, err)
	}
	after := taxRow(ctx, t, pool, id)
	if after.ReviewStatus != "DRAFT" {
		t.Fatalf("status=%s，want DRAFT", after.ReviewStatus)
	}
	// 回 DRAFT 必須清空上一輪的時鐘欄：留著會讓 T02 算出的新 60 天窗口與舊值並存，
	// 而 chk_stp_pending_window 只檢查非空、不檢查新鮮度。
	if after.SubmittedAt != nil || after.PendingValidUntil != nil || after.CooldownUntil != nil {
		t.Fatalf("重申時時鐘欄沒清乾淨：%+v", after)
	}
}

// T07／T08：Approve 用 next_status 參數共用一句，錯誤組合由 CHECK 擋。
//
// 🔴 **這一支只有真 SQL 驗得到**：fake 沒有 chk_stp_required_needs_taxid 與
// chk_stp_exempt_no_taxid ⇒ 在單元測試裡「把沒有統編的賣家核定成應開統一發票」
// 會靜默成功，而那張發票開出去就是抬頭錯誤，只能作廢重開。
func TestIntegration_TaxProfile_ApproveIsGuardedByChecks(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()
	seedTaxStatus(ctx, t, pool, id, "UNDER_REVIEW")

	// 沒有統編／法定名稱卻核定成 APPROVED_INVOICE_REQUIRED ⇒ 23514。
	row := taxRow(ctx, t, pool, id)
	_, err := repo.Approve(ctx, domain.ApproveSellerTaxProfile{ID: id, ExpectedVersion: row.Version, NextStatus: "APPROVED_INVOICE_REQUIRED", ApprovalValidUntil: now.Add(365 * 24 * time.Hour), Now: now})
	if err == nil {
		t.Fatal("chk_stp_required_needs_taxid 應該擋下「沒有統編卻核定應開統一發票」")
	}
}

// 同一條 CHECK 的另一向：帶著統編卻核定成免用統一發票 ⇒ 23514。
func TestIntegration_TaxProfile_ApproveExemptRejectsTaxID(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()
	mustExec(ctx, t, pool,
		`UPDATE seller_tax_profiles SET review_status = 'UNDER_REVIEW', tax_id = '12345678',
		        legal_name = '範例生鮮股份有限公司' WHERE id = $1`, id)

	row := taxRow(ctx, t, pool, id)
	if _, err := repo.Approve(ctx, domain.ApproveSellerTaxProfile{ID: id, ExpectedVersion: row.Version, NextStatus: "APPROVED_INVOICE_EXEMPT", ApprovalValidUntil: now.Add(365 * 24 * time.Hour), Now: now}); err == nil {
		t.Fatal("chk_stp_exempt_no_taxid 應該擋下「帶統編卻核定免用統一發票」")
	}

	// 對照組：統編清掉之後同一句成功 ⇒ 證明剛才擋下的是 CHECK 不是別的東西。
	mustExec(ctx, t, pool, `UPDATE seller_tax_profiles SET tax_id = NULL WHERE id = $1`, id)
	row = taxRow(ctx, t, pool, id)
	if ok, err := repo.Approve(ctx, domain.ApproveSellerTaxProfile{ID: id, ExpectedVersion: row.Version, NextStatus: "APPROVED_INVOICE_EXEMPT", ApprovalValidUntil: now.Add(365 * 24 * time.Hour), Now: now}); err != nil || !ok {
		t.Fatalf("清掉統編後應可核定：ok=%v err=%v", ok, err)
	}
	after := taxRow(ctx, t, pool, id)
	if after.ApprovedAt == nil || after.ApprovalValidUntil == nil {
		t.Fatal("核定必寫 approved_at 與 approval_valid_until（chk_stp_valid_until_needs_approved 守右半）")
	}
}

// T11：ExtendPending 的 `extension_count = 0` 述詞是「只能延一次」的守衛。
//
// 🔴 chk_stp_extension 只擋 0–1 的值域，擋不了「延兩次但第二次也寫 1」
// ⇒ 述詞裡那個 0 不可省，而它只有真 SQL 驗得到。
func TestIntegration_TaxProfile_ExtendOnlyOnce(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "SELF_ISSUE_PENDING")
	now := time.Now().UTC()
	row := taxRow(ctx, t, pool, id)
	first := now.Add(60 * 24 * time.Hour)

	if ok, err := repo.ExtendPending(ctx, id, row.Version, first, "賣家已提供進度", now); err != nil || !ok {
		t.Fatalf("第一次延長應成功：ok=%v err=%v", ok, err)
	}
	after := taxRow(ctx, t, pool, id)
	if after.ExtensionCount != 1 || after.ReviewStatus != "SELF_ISSUE_PENDING" {
		t.Fatalf("T11 應只改期限與次數，實際 count=%d status=%s", after.ExtensionCount, after.ReviewStatus)
	}
	// 第二次：述詞 extension_count = 0 不成立 ⇒ rows=0，期限不得再被推遲。
	if ok, err := repo.ExtendPending(ctx, id, after.Version, now.Add(120*24*time.Hour), "再延", now); err != nil || ok {
		t.Fatalf("第二次延長應 rows=0：ok=%v err=%v", ok, err)
	}
	if !taxRow(ctx, t, pool, id).PendingValidUntil.Equal(after.PendingValidUntil.UTC()) {
		t.Fatal("守衛不成立時期限不得被改動")
	}
}

// T12／T13／T15：ReopenReview 的四個來源狀態逐一驗，並確認**不動 approved_at**
// （複審中不代表核定沒發生過，那兩欄是核定當下的事實）。
func TestIntegration_TaxProfile_ReopenFromFourStates(t *testing.T) {
	for _, st := range []string{"EXPIRED", "APPROVED_INVOICE_REQUIRED", "APPROVED_INVOICE_EXEMPT", "REVOKED"} {
		t.Run(st, func(t *testing.T) {
			repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
			now := time.Now().UTC()
			seedTaxStatus(ctx, t, pool, id, st)
			before := taxRow(ctx, t, pool, id)

			ok, err := repo.ReopenReview(ctx, id, before.Version, "年度複審", now)
			if err != nil || !ok {
				t.Fatalf("%s 應該可以被 ADM 重開：ok=%v err=%v", st, ok, err)
			}
			after := taxRow(ctx, t, pool, id)
			if after.ReviewStatus != "UNDER_REVIEW" {
				t.Fatalf("status=%s，want UNDER_REVIEW", after.ReviewStatus)
			}
			if after.CooldownUntil != nil {
				t.Fatal("T12 的「免冷卻」＝ cooldown_until 清空")
			}
			if before.ApprovedAt != nil && after.ApprovedAt == nil {
				t.Fatal("複審不得抹掉 approved_at：那是核定當下的事實")
			}
		})
	}
}

// 陰性：DRAFT／SUBMITTED 不在 ReopenReview 的來源狀態內 ⇒ rows=0。
func TestIntegration_TaxProfile_ReopenRejectsOtherStates(t *testing.T) {
	for _, st := range []string{"DRAFT", "SUBMITTED", "UNDER_REVIEW", "SUPPLEMENT_REQUIRED", "SELF_ISSUE_PENDING", "REJECTED"} {
		t.Run(st, func(t *testing.T) {
			repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
			seedTaxStatus(ctx, t, pool, id, st)
			row := taxRow(ctx, t, pool, id)

			ok, err := repo.ReopenReview(ctx, id, row.Version, "理由", time.Now().UTC())
			if err != nil || ok {
				t.Fatalf("%s 不是 T12／T13／T15 的來源狀態，應 rows=0：ok=%v err=%v", st, ok, err)
			}
			if taxRow(ctx, t, pool, id).ReviewStatus != st {
				t.Fatalf("%s 被擋下後狀態仍被改動", st)
			}
		})
	}
}

// T16 自轉移：KeepUnderReview **不動任何時鐘欄位**（pending_valid_until 不重算、
// extension_count 不動），只把 version +1 與 review_note 寫進去。
func TestIntegration_TaxProfile_KeepUnderReviewTouchesNoClock(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "SELF_ISSUE_PENDING")
	// 先延長一次，讓時鐘欄位有可觀察的非預設值。
	now := time.Now().UTC()
	row := taxRow(ctx, t, pool, id)
	if ok, _ := repo.ExtendPending(ctx, id, row.Version, now.Add(90*24*time.Hour), "延長", now); !ok {
		t.Fatal("前置延長失敗")
	}
	mustExec(ctx, t, pool, `UPDATE seller_tax_profiles SET review_status = 'UNDER_REVIEW' WHERE id = $1`, id)
	before := taxRow(ctx, t, pool, id)

	ok, err := repo.KeepUnderReview(ctx, id, before.Version, "等補充說明", now)
	if err != nil || !ok {
		t.Fatalf("T16 應成功：ok=%v err=%v", ok, err)
	}
	after := taxRow(ctx, t, pool, id)
	if after.ReviewStatus != "UNDER_REVIEW" {
		t.Fatalf("T16 是自轉移，status 應不變，實際 %s", after.ReviewStatus)
	}
	if !after.PendingValidUntil.Equal(before.PendingValidUntil.UTC()) || after.ExtensionCount != before.ExtensionCount {
		t.Fatalf("T16 不得動時鐘欄位：%v/%d → %v/%d",
			before.PendingValidUntil, before.ExtensionCount, after.PendingValidUntil, after.ExtensionCount)
	}
	if after.Version != before.Version+1 {
		t.Fatalf("version = %d，want %d（它是一次業務操作）", after.Version, before.Version+1)
	}
}

// T14：Revoke 的 allowed-from 只有兩個 APPROVED_*。
func TestIntegration_TaxProfile_RevokeOnlyFromApproved(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()

	// 陰性：UNDER_REVIEW 不是來源狀態。
	seedTaxStatus(ctx, t, pool, id, "UNDER_REVIEW")
	row := taxRow(ctx, t, pool, id)
	if ok, err := repo.Revoke(ctx, id, row.Version, "撤銷", now); err != nil || ok {
		t.Fatalf("UNDER_REVIEW 不得被撤銷：ok=%v err=%v", ok, err)
	}
	// 陽性。
	seedTaxStatus(ctx, t, pool, id, "APPROVED_INVOICE_REQUIRED")
	row = taxRow(ctx, t, pool, id)
	if ok, err := repo.Revoke(ctx, id, row.Version, "資格不符", now); err != nil || !ok {
		t.Fatalf("APPROVED_* 應可撤銷：ok=%v err=%v", ok, err)
	}
	if taxRow(ctx, t, pool, id).ReviewStatus != "REVOKED" {
		t.Fatal("撤銷後狀態應為 REVOKED")
	}
}

// T05：RequestSupplement 轉進 SUPPLEMENT_REQUIRED，而該狀態受 chk_stp_pending_window
// 拘束 ⇒ 沒走過 T02 的列會當場 23514 而不是靜默寫壞。
func TestIntegration_TaxProfile_RequestSupplementNeedsPendingWindow(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()

	// 沒有 submitted_at／pending_valid_until 的 UNDER_REVIEW（ADM 直接建列的情境）。
	mustExec(ctx, t, pool, `UPDATE seller_tax_profiles SET review_status = 'UNDER_REVIEW' WHERE id = $1`, id)
	row := taxRow(ctx, t, pool, id)
	if _, err := repo.RequestSupplement(ctx, id, row.Version, "請補稅籍證明", now); err == nil {
		t.Fatal("chk_stp_pending_window 應擋下「沒有時鐘欄位卻要求補件」")
	}

	// 對照組：補上時鐘欄位之後同一句成功。
	mustExec(ctx, t, pool,
		`UPDATE seller_tax_profiles SET submitted_at = $2, pending_valid_until = $3 WHERE id = $1`,
		id, now.Add(-24*time.Hour), now.Add(30*24*time.Hour))
	row = taxRow(ctx, t, pool, id)
	if ok, err := repo.RequestSupplement(ctx, id, row.Version, "請補稅籍證明", now); err != nil || !ok {
		t.Fatalf("時鐘齊全後應成功：ok=%v err=%v", ok, err)
	}
	after := taxRow(ctx, t, pool, id)
	if after.ReviewStatus != "SUPPLEMENT_REQUIRED" {
		t.Fatalf("status=%s，want SUPPLEMENT_REQUIRED", after.ReviewStatus)
	}
	// **不重算 60 天**：ADM 每要一次補件就等於送賣家一次延期。
	if !after.PendingValidUntil.Equal(now.Add(30*24*time.Hour).UTC().Truncate(time.Microsecond)) &&
		after.PendingValidUntil.Sub(now.Add(30*24*time.Hour)).Abs() > time.Millisecond {
		t.Fatalf("補件不得重算期限：%v", after.PendingValidUntil)
	}
}

// UpdateDraft：allowed-from 只有 DRAFT，且六個可空欄的空字串必須落 NULL。
//
// 🔴 tax_id 尤其致命：chk_stp_tax_id 是 `IS NULL OR ~ '^[0-9]{8}$'`，
// 存空字串當場 23514 ——而 fake 沒有那條 CHECK。
func TestIntegration_TaxProfile_UpdateDraftNullsEmptyStrings(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "DRAFT")
	now := time.Now().UTC()

	ok, err := repo.UpdateDraft(ctx, domain.UpdateSellerTaxDraft{
		ID: id, ExpectedVersion: 0,
		ClaimedObligation: "EXEMPT", EntityTaxType: "FARMER", LegalName: "某某農場",
		TaxID: "", ResponsiblePerson: "陳小華", BusinessAddress: "宜蘭縣三星鄉",
		DefaultTaxCategory: 1,
		EvidenceObjectKeys: []string{"secure/certification/1/a.pdf"},
		Now:                now,
	})
	if err != nil || !ok {
		t.Fatalf("草稿存檔應成功（空 tax_id 必須落 NULL）：ok=%v err=%v", ok, err)
	}
	row := taxRow(ctx, t, pool, id)
	if row.TaxID != nil {
		t.Fatalf("空字串必須落 NULL，實際 %q", *row.TaxID)
	}
	var keys []string
	if err := json.Unmarshal(row.EvidenceObjectKeys, &keys); err != nil || len(keys) != 1 {
		t.Fatalf("evidence_object_keys = %s", row.EvidenceObjectKeys)
	}

	// 陰性：非 DRAFT 不得就地改內容。
	seedTaxStatus(ctx, t, pool, id, "UNDER_REVIEW")
	cur := taxRow(ctx, t, pool, id)
	if ok, err := repo.UpdateDraft(ctx, domain.UpdateSellerTaxDraft{
		ID: id, ExpectedVersion: cur.Version, ClaimedObligation: "REQUIRED",
		DefaultTaxCategory: 1, EvidenceObjectKeys: []string{}, Now: now,
	}); err != nil || ok {
		t.Fatalf("UNDER_REVIEW 不是草稿存檔的來源狀態：ok=%v err=%v", ok, err)
	}
}

// invoice_issuers：Create 冪等 ＋ UpdateDisplayName 同步法定名稱。
//
// display_name 留著佔位字串會一路寫進 invoice_documents.issuer_name_snapshot，
// 而發票上的銷售人名稱寫錯只能作廢重開。
func TestIntegration_Issuer_CreateAndDisplayName(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewIssuerRepository(pool)
	now := time.Now().UTC()
	storeID := int64(2632049831)

	first, inserted, err := repo.Create(ctx, domain.NewIssuer{
		ID: nextID(), IssuerType: "SELLER", IssuerRefID: storeID,
		DisplayName: "store-2632049831", Now: now,
	})
	if err != nil || !inserted {
		t.Fatalf("建列失敗：inserted=%v err=%v", inserted, err)
	}
	// 第二次 ⇒ ON CONFLICT DO NOTHING（不是 23505）。
	if _, inserted, err = repo.Create(ctx, domain.NewIssuer{
		ID: nextID(), IssuerType: "SELLER", IssuerRefID: storeID,
		DisplayName: "另一個名字", Now: now,
	}); err != nil || inserted {
		t.Fatalf("uq_invoice_issuers_ref 應擋下第二列：inserted=%v err=%v", inserted, err)
	}

	changed, err := repo.UpdateDisplayName(ctx, first.ID, "範例生鮮股份有限公司", now)
	if err != nil || !changed {
		t.Fatalf("同步法定名稱失敗：changed=%v err=%v", changed, err)
	}
	// 值相同時 rows=0 ⇒ **no-op 不是守衛不成立**：重複存檔不空轉 version。
	if changed, err = repo.UpdateDisplayName(ctx, first.ID, "範例生鮮股份有限公司", now); err != nil || changed {
		t.Fatalf("同值應為 no-op：changed=%v err=%v", changed, err)
	}
	got, err := repo.GetByID(ctx, first.ID)
	if err != nil || got.DisplayName != "範例生鮮股份有限公司" {
		t.Fatalf("display_name = %q err=%v", got.DisplayName, err)
	}
}

// ---------------------------------------------------------------------------
// S05 收據代製授權（§5.1.2 四欄、端點 29）
//
// 三條 CHECK 與一句 CAS 的述詞，單元測試（fake 沒有 CHECK）一條都驗不到。
// ---------------------------------------------------------------------------

// 🔴 資格守衛必須在 CAS 的 WHERE：allowed-from 之外 ⇒ rows=0（可回 9335），
// 而不是靠 CHECK（那會回 23514 ＝ 一句 500），也不是靠應用層的 if（擋不住併發）。
//
// allowed-from 自 2026-09-05（DEC-20260905-015836 選 A）起含四個「審核進行中」狀態：
// 賣家可在核定之前就簽，消滅「已核定但未授權」那段每一筆收據都要人工處理的窗口。
// APPROVED_INVOICE_REQUIRED 仍在集合之外 —— 它走發票流程，代製收據對它沒有意義，
// 所以它繼續是這支測試的陰性組。
func TestIntegration_ReceiptAuth_NonEligibleStatusCannotAuthorize(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "APPROVED_INVOICE_REQUIRED")
	now := time.Now().UTC()
	ver := "v2026-08"

	ok, err := repo.ConfirmReceiptAuthorization(ctx, domain.ConfirmReceiptAuthorization{
		ID: id, AuthorizedAt: &now, ConsentVersion: &ver,
		ExpectedVersion: taxRow(ctx, t, pool, id).Version, Now: now,
	})
	if err != nil {
		t.Fatalf("不該是 SQL 錯誤（那表示擋下來的是 CHECK 不是 CAS）：%v", err)
	}
	if ok {
		t.Fatal("非免用統一發票核准賣家不得授權")
	}
	if taxRow(ctx, t, pool, id).ReceiptAuthorizedAt != nil {
		t.Fatal("守衛不成立時不得留下任何寫入")
	}

	// 鑑別力：換成 EXEMPT 就過得去（上面的 false 不是因為別的原因）。
	seedTaxStatus(ctx, t, pool, id, "APPROVED_INVOICE_EXEMPT")
	ok, err = repo.ConfirmReceiptAuthorization(ctx, domain.ConfirmReceiptAuthorization{
		ID: id, AuthorizedAt: &now, ConsentVersion: &ver,
		ExpectedVersion: taxRow(ctx, t, pool, id).Version, Now: now,
	})
	if err != nil || !ok {
		t.Fatalf("EXEMPT 賣家應可授權：ok=%v err=%v", ok, err)
	}
	row := taxRow(ctx, t, pool, id)
	if row.ReceiptAuthorizedAt == nil || row.ReceiptConsentVersion == nil || *row.ReceiptConsentVersion != ver {
		t.Fatalf("兩欄必須同時寫入，實得 %+v／%v", row.ReceiptAuthorizedAt, row.ReceiptConsentVersion)
	}
}

// 裁決 A 的本體：**還沒被核定**的賣家就要簽得了，否則「核定→授權」之間的每一筆
// 已付訂單都會落 AWAITING_MANUAL / RECEIPT_AUTH_MISSING 等人工處理。
//
// 四個狀態逐一實打，不抽樣：它們在 SQL 裡是逐字列出的，漏掉任何一個都是靜默的
// 「這個狀態的賣家永遠簽不了」，而 rows=0 在呼叫端長得跟版本衝突一模一樣。
func TestIntegration_ReceiptAuth_InReviewSellerCanAuthorizeBeforeApproval(t *testing.T) {
	for _, st := range []string{
		"SUBMITTED", "UNDER_REVIEW", "SUPPLEMENT_REQUIRED", "SELF_ISSUE_PENDING",
	} {
		t.Run(st, func(t *testing.T) {
			repo, ctx, pool, _, id := newTaxFixture(t, st)
			now := time.Now().UTC()
			ver := "v2026-09"

			ok, err := repo.ConfirmReceiptAuthorization(ctx, domain.ConfirmReceiptAuthorization{
				ID: id, AuthorizedAt: &now, ConsentVersion: &ver,
				ExpectedVersion: taxRow(ctx, t, pool, id).Version, Now: now,
			})
			if err != nil {
				t.Fatalf("不該是 SQL 錯誤：%v", err)
			}
			if !ok {
				t.Fatalf("%s 的賣家應該簽得了（DEC-20260905-015836 選 A）", st)
			}
			row := taxRow(ctx, t, pool, id)
			if row.ReceiptAuthorizedAt == nil || row.ReceiptConsentVersion == nil {
				t.Fatalf("兩欄必須同時寫入，實得 %+v／%v",
					row.ReceiptAuthorizedAt, row.ReceiptConsentVersion)
			}
			// 核定尚未發生 —— 授權不得順手把狀態往前推。
			if row.ReviewStatus != st {
				t.Fatalf("授權不得改動 review_status，實得 %q", row.ReviewStatus)
			}
		})
	}
}

// chk_stp_receipt_auth_pair：授權時點與條款版本同進同出（雙向）。
func TestIntegration_ReceiptAuth_PairCheckIsBidirectional(t *testing.T) {
	_, ctx, pool, _, id := newTaxFixture(t, "APPROVED_INVOICE_EXEMPT")

	if _, err := pool.Exec(ctx,
		`UPDATE seller_tax_profiles SET receipt_authorized_at = now() WHERE id = $1`, id); err == nil {
		t.Fatal("只有時點沒有版本 ⇒ 事後不知道賣家同意的是哪一版條款，必須被擋下")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE seller_tax_profiles SET receipt_consent_version = 'v1' WHERE id = $1`, id); err == nil {
		t.Fatal("只有版本沒有時點 ⇒ 看起來像已授權但從未發生，必須被擋下")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE seller_tax_profiles SET receipt_authorized_at = now(), receipt_consent_version = 'v1'
		  WHERE id = $1`, id); err != nil {
		t.Fatalf("兩欄一起寫應該可以：%v", err)
	}
}

// chk_stp_receipt_consent_nonblank：Go 的零值是空字串，而 NULL 與 ” 在
// chk_stp_receipt_auth_pair 底下的判定相反。
func TestIntegration_ReceiptAuth_ConsentVersionRejectsEmptyString(t *testing.T) {
	_, ctx, pool, _, id := newTaxFixture(t, "APPROVED_INVOICE_EXEMPT")

	if _, err := pool.Exec(ctx,
		`UPDATE seller_tax_profiles SET receipt_authorized_at = now(), receipt_consent_version = ''
		  WHERE id = $1`, id); err == nil {
		t.Fatal("空字串條款版本必須被 chk_stp_receipt_consent_nonblank 擋下")
	}
}

// chk_stp_receipt_basis_pair：key 與 asset id 同進同出。
func TestIntegration_ReceiptAuth_BasisPairCheck(t *testing.T) {
	repo, ctx, pool, _, id := newTaxFixture(t, "APPROVED_INVOICE_EXEMPT")
	now := time.Now().UTC()
	key := "invoice/tax/2685914682/approval.pdf"
	assetID := int64(778899)

	if _, err := pool.Exec(ctx,
		`UPDATE seller_tax_profiles SET receipt_approval_basis_key = $2 WHERE id = $1`, id, key); err == nil {
		t.Fatal("只有 key 沒有 asset id ⇒ Gateway 組不出下載路徑，必須被擋下")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE seller_tax_profiles SET receipt_approval_basis_asset_id = $2 WHERE id = $1`, id, assetID); err == nil {
		t.Fatal("只有 asset id 沒有 key ⇒ §25 的「objectKey 是 SSOT」就斷了，必須被擋下")
	}

	// 指名走的是端點 40 的核定那一句 CAS（§6.1.9：由 ADM 在核定決策中指名，
	// 不另開一句 UPDATE）⇒ 先把列推回可核定的來源狀態。
	seedTaxStatus(ctx, t, pool, id, "UNDER_REVIEW")
	ok, err := repo.Approve(ctx, domain.ApproveSellerTaxProfile{
		ID: id, ExpectedVersion: taxRow(ctx, t, pool, id).Version,
		NextStatus:         "APPROVED_INVOICE_EXEMPT",
		ApprovalValidUntil: now.Add(365 * 24 * time.Hour),
		ApprovalBasisKey:   &key, ApprovalBasisAssetID: &assetID, Now: now,
	})
	if err != nil || !ok {
		t.Fatalf("ADM 指名依據文件應成功：ok=%v err=%v", ok, err)
	}
	row := taxRow(ctx, t, pool, id)
	if row.ReceiptApprovalBasisKey == nil || *row.ReceiptApprovalBasisKey != key ||
		row.ReceiptApprovalBasisAssetID == nil || *row.ReceiptApprovalBasisAssetID != assetID {
		t.Fatalf("兩欄必須同時寫入，實得 %+v", row)
	}

	// 取消指名：再核定一次但不帶依據文件 ⇒ 兩欄一起清成 NULL
	// （端點 28 於是回 NOT_DESIGNATED）。這也是「同一句 CAS」的必然結果：
	// 核定寫什麼，依據文件就是什麼，不會留下上一次的殘值。
	seedTaxStatus(ctx, t, pool, id, "UNDER_REVIEW")
	ok, err = repo.Approve(ctx, domain.ApproveSellerTaxProfile{
		ID: id, ExpectedVersion: taxRow(ctx, t, pool, id).Version,
		NextStatus:         "APPROVED_INVOICE_EXEMPT",
		ApprovalValidUntil: now.Add(365 * 24 * time.Hour), Now: now,
	})
	if err != nil || !ok {
		t.Fatalf("取消指名應成功：ok=%v err=%v", ok, err)
	}
	row = taxRow(ctx, t, pool, id)
	if row.ReceiptApprovalBasisKey != nil || row.ReceiptApprovalBasisAssetID != nil {
		t.Fatalf("取消後兩欄必須同時為 NULL，實得 %+v", row)
	}
}

// 🔴 **不得有「只有 EXEMPT 才能保有 receipt_authorized_at」的 CHECK**：
// review_status 之後會被複審推翻（T-C04），而一條綁著它的 CHECK 會讓**之後每一次**
// 對該列的更新都撞 23514 —— 包括撤銷資格本身。本支就是那個情境的實跑。
func TestIntegration_ReceiptAuth_RevokingApprovalDoesNotBlockFutureUpdates(t *testing.T) {
	_, ctx, pool, _, id := newTaxFixture(t, "APPROVED_INVOICE_EXEMPT")
	mustExec(ctx, t, pool,
		`UPDATE seller_tax_profiles SET receipt_authorized_at = now(), receipt_consent_version = 'v1'
		  WHERE id = $1`, id)

	if _, err := pool.Exec(ctx,
		`UPDATE seller_tax_profiles SET review_status = 'REVOKED' WHERE id = $1`, id); err != nil {
		t.Fatalf("撤銷資格不得被授權欄卡住：%v", err)
	}
	row := taxRow(ctx, t, pool, id)
	if row.ReceiptAuthorizedAt == nil {
		t.Fatal("撤銷資格不會抹掉授權紀錄——那是曾經發生過的事實")
	}
	// 而讀取面會把它投影成 ineligible（APPROVAL_REVOKED），資格與授權紀錄是兩件事。
}
