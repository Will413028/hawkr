//go:build integration

// 本檔跑真的 SQL，守的是 §6.1.8／§6.1.13 那八支狀態轉移的三種失效形狀，
// 而三種在單元測試裡**全部驗不到**：
//
//	① **allowed-from 的邊界**：CAS 的 rows=0 是靜默的（不拋例外、不告警），
//	   而 fake 的 ok 旗標是測試自己設的——它證明「呼叫端有處理 rows=0」，
//	   證明不了「SQL 的述詞真的排除了那個狀態」。
//	② **DB CHECK**：chk_ipa_cutover_requires_verified／chk_ipa_active_verified／
//	   chk_ipa_rotation（雙向等式）。fake 沒有 CHECK ⇒ 對它們的鑑別力是零。
//	③ **uq_ipa_fingerprint 的 partial 述詞**：REVOKED 不在述詞內 ⇒ 撤銷會釋放名額。
//	   那是 §5 I-37 的第二向，賣家關店重開能不能綁回同一組憑證取決於它。
package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// providerRow 讀回本檔關心的四欄（domain.ProviderAccount 沒有 automation_cutover_at
// 以外的全部，直接查表比較不受投影變動影響）。
type providerRow struct {
	Status     string
	Version    int64
	VerifiedAt *time.Time
	CutoverAt  *time.Time
	Pending    *string
}

func readProviderRow(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) providerRow {
	t.Helper()
	var r providerRow
	err := pool.QueryRow(ctx,
		`SELECT status, version, verified_at, automation_cutover_at, pending_ciphertext
		   FROM invoice_provider_accounts WHERE id = $1`, id).
		Scan(&r.Status, &r.Version, &r.VerifiedAt, &r.CutoverAt, &r.Pending)
	if err != nil {
		t.Fatalf("讀 provider account: %v", err)
	}
	return r
}

// seedProviderAccount 建一列並把它推到指定狀態（必要時補 verified_at）。
func seedProviderAccount(ctx context.Context, t *testing.T, pool *pgxpool.Pool, repo *ProviderAccountRepository,
	issuerID int64, taxID, appKey, status string, verified bool) int64 {
	t.Helper()
	id := issuerID + 1
	if err := repo.Create(ctx, newAccount(id, issuerID, taxID, appKey)); err != nil {
		t.Fatalf("seed provider account: %v", err)
	}
	var verifiedAt any
	if verified {
		verifiedAt = time.Now().UTC()
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts SET status = $2, verified_at = $3 WHERE id = $1`,
		id, status, verifiedAt); err != nil {
		t.Fatalf("seed status=%s: %v", status, err)
	}
	return id
}

func uniqueAppKey() string { return fmt.Sprintf("AK%d", time.Now().UnixNano()) }

// ---------------------------------------------------------------------------
// 端點 26：I-61 ＋ chk_ipa_cutover_requires_verified
// ---------------------------------------------------------------------------

// TestIntegration_SetAutomationCutover_OnlyFromActive 驗 I-61 的 allowed-from。
//
// 🔴 這一支是單元測試的鏡像不了的那一半：fake 的 cutoverOK 旗標是測試自己設的，
// 它證明「呼叫端把 rows=0 當守衛不成立」，證明不了述詞真的只放行 ACTIVE。
func TestIntegration_SetAutomationCutover_OnlyFromActive(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	cutover := time.Now().UTC().Add(2 * time.Hour)

	for _, tc := range []struct {
		status   string
		verified bool
		wantOK   bool
	}{
		{"ACTIVE", true, true},
		// DEGRADED 是 §6.1.8 端點 26 散文允許、而 I-61 明令排除的那一格。
		// 本測試釘死實作採用的是 I-61（衝突已登記在交付回報）。
		{"DEGRADED", true, false},
		{"CREDENTIAL_VERIFYING", false, false},
		{"SUSPENDED", true, false},
		{"REVOKED", true, false},
	} {
		t.Run(tc.status, func(t *testing.T) {
			issuer := seedIssuer(ctx, t, pool, nextIssuerRefID())
			id := seedProviderAccount(ctx, t, pool, repo, issuer, "12345678", uniqueAppKey(), tc.status, tc.verified)
			before := readProviderRow(ctx, t, pool, id)

			ok, err := repo.SetAutomationCutover(ctx, id, before.Version, cutover, time.Now().UTC())
			if err != nil {
				t.Fatalf("SetAutomationCutover 不該回錯（守衛不成立是 rows=0 不是例外）：%v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("status=%s: ok = %v, want %v", tc.status, ok, tc.wantOK)
			}
			after := readProviderRow(ctx, t, pool, id)
			if tc.wantOK {
				if after.CutoverAt == nil {
					t.Fatalf("切點未寫入")
				}
				// 🔴 端點 26 **不寫 status**：步驟 4 與 5 的分界是切點有沒有值。
				if after.Status != before.Status {
					t.Fatalf("端點 26 不得改動 status：%s → %s", before.Status, after.Status)
				}
			} else if after.CutoverAt != nil {
				t.Fatalf("守衛不成立時不得寫入切點")
			}
		})
	}
}

// TestIntegration_LandVerification_NilVerifiedAtBreaksCutoverCheck 是 CHECK 的專屬斷言。
//
// 🔴 這一支證明的是「呼叫端**必須**逐次明給 verified_at」這條規則的必要性：
// 落失敗狀態時傳 nil，chk_ipa_cutover_requires_verified 會在該列已設切點時當場 23514。
// 單元測試對它的鑑別力是零（fake 沒有 CHECK），而失效形狀是 prod 上的一句 500。
func TestIntegration_LandVerification_NilVerifiedAtBreaksCutoverCheck(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	issuer := seedIssuer(ctx, t, pool, 9401)
	id := seedProviderAccount(ctx, t, pool, repo, issuer, "12345678", uniqueAppKey(), "ACTIVE", true)

	// 先設一個切點（走正式路徑，確保 CHECK 在這一步是滿足的）。
	row := readProviderRow(ctx, t, pool, id)
	ok, err := repo.SetAutomationCutover(ctx, id, row.Version, time.Now().UTC().Add(time.Hour), time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("設定切點失敗：ok=%v err=%v", ok, err)
	}
	// 推到 CREDENTIAL_VERIFYING（LandVerification 的 allowed-from）。
	row = readProviderRow(ctx, t, pool, id)
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts SET status='CREDENTIAL_VERIFYING' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}

	// ① 傳 nil ⇒ 23514（這是**錯誤示範**，用來證明 CHECK 真的在守）。
	_, err = repo.LandVerification(ctx, domain.LandProviderVerification{
		ID: id, NextStatus: "INVALID_CREDENTIAL", VerifiedAt: nil,
		NextCheckAt:     time.Now().UTC().Add(15 * time.Minute),
		ExpectedVersion: row.Version, Now: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("清空 verified_at 而該列已有切點，DB 應以 23514 擋下（chk_ipa_cutover_requires_verified 失效）")
	}

	// ② 沿用原值 ⇒ 成功。這是正式路徑實際採用的寫法。
	row = readProviderRow(ctx, t, pool, id)
	ok, err = repo.LandVerification(ctx, domain.LandProviderVerification{
		ID: id, NextStatus: "INVALID_CREDENTIAL", VerifiedAt: row.VerifiedAt,
		NextCheckAt:     time.Now().UTC().Add(15 * time.Minute),
		ExpectedVersion: row.Version, Now: time.Now().UTC(),
	})
	if err != nil || !ok {
		t.Fatalf("沿用原 verified_at 應成功：ok=%v err=%v", ok, err)
	}
	after := readProviderRow(ctx, t, pool, id)
	if after.Status != "INVALID_CREDENTIAL" || after.CutoverAt == nil {
		t.Fatalf("落地後狀態或切點不符：%+v", after)
	}
}

// ---------------------------------------------------------------------------
// 端點 25／44／66：StartVerification 的 allowed-from
// ---------------------------------------------------------------------------

func TestIntegration_StartVerification_AllowedFrom(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	for _, tc := range []struct {
		status   string
		verified bool
		wantOK   bool
	}{
		{"CREDENTIAL_SUBMITTED", false, true},
		{"CREDENTIAL_VERIFYING", false, true},
		{"INVALID_CREDENTIAL", false, true},
		{"PROVIDER_REJECTED", false, true},
		{"PROVIDER_UNAVAILABLE", false, true},
		// P15：端點 66 恢復連線的唯一入口。
		{"SUSPENDED", true, true},
		// 🔴 §7.3.4 沒有 ACTIVE → CREDENTIAL_VERIFYING 這條邊：ACTIVE／DEGRADED 的重測
		// 走「不改 status」那條路，這正是 §6.1.13 端點 44 定版③ 的效果。
		{"ACTIVE", true, false},
		{"DEGRADED", true, false},
		{"REVOKED", true, false},
	} {
		t.Run(tc.status, func(t *testing.T) {
			issuer := seedIssuer(ctx, t, pool, nextIssuerRefID())
			id := seedProviderAccount(ctx, t, pool, repo, issuer, "12345678", uniqueAppKey(), tc.status, tc.verified)
			row := readProviderRow(ctx, t, pool, id)

			ok, err := repo.StartVerification(ctx, id, row.Version, time.Now().UTC())
			if err != nil {
				t.Fatalf("StartVerification 不該回錯：%v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("status=%s: ok = %v, want %v", tc.status, ok, tc.wantOK)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 端點 45／67：Suspend／Revoke 的 allowed-from ＋ uq_ipa_fingerprint 第二向
// ---------------------------------------------------------------------------

func TestIntegration_SuspendAndRevoke_AllowedFrom(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	for _, tc := range []struct {
		status      string
		wantSuspend bool
		wantRevoke  bool
	}{
		{"ACTIVE", true, true},
		{"DEGRADED", true, true},
		// 停用是可逆的、撤銷不是：SUSPENDED 只能被撤銷，不能被再停用一次。
		{"SUSPENDED", false, true},
		// 🔴 破壞性防護②：已 REVOKED 的列撤不了第二次，而且不會靜默成功。
		{"REVOKED", false, false},
		{"CREDENTIAL_VERIFYING", false, false},
	} {
		t.Run(tc.status, func(t *testing.T) {
			issuer := seedIssuer(ctx, t, pool, nextIssuerRefID())
			id := seedProviderAccount(ctx, t, pool, repo, issuer, "12345678", uniqueAppKey(), tc.status, true)

			row := readProviderRow(ctx, t, pool, id)
			gotSuspend, err := repo.Suspend(ctx, id, row.Version, "測試停用", time.Now().UTC())
			if err != nil {
				t.Fatalf("Suspend 不該回錯：%v", err)
			}
			if gotSuspend != tc.wantSuspend {
				t.Fatalf("status=%s: Suspend ok = %v, want %v", tc.status, gotSuspend, tc.wantSuspend)
			}

			// 還原成原狀態再驗 Revoke（Suspend 成功時已把它改成 SUSPENDED）。
			if _, err := pool.Exec(ctx,
				`UPDATE invoice_provider_accounts SET status = $2 WHERE id = $1`, id, tc.status); err != nil {
				t.Fatal(err)
			}
			row = readProviderRow(ctx, t, pool, id)
			gotRevoke, err := repo.Revoke(ctx, id, row.Version, "測試撤銷", time.Now().UTC())
			if err != nil {
				t.Fatalf("Revoke 不該回錯：%v", err)
			}
			if gotRevoke != tc.wantRevoke {
				t.Fatalf("status=%s: Revoke ok = %v, want %v", tc.status, gotRevoke, tc.wantRevoke)
			}
		})
	}
}

// TestIntegration_Revoke_KeepsCutoverAndReleasesFingerprint 是 P16 的兩條後果。
//
//	① automation_cutover_at **保留不清空**：歷史訂單的模式判定要用它。
//	② uq_ipa_fingerprint 的 partial 述詞不含 REVOKED ⇒ 撤銷**釋放名額**，
//	   賣家關店重開能綁回同一組憑證（§5 I-37 的第二向）。
//
// 第二向在單元測試裡構造不出來（fake 沒有 partial unique），而缺這一向時
// 「索引寫成無條件唯一」也會綠——症狀是賣家關店重開後永遠設定不了憑證。
func TestIntegration_Revoke_KeepsCutoverAndReleasesFingerprint(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	const taxID = "12345678"
	appKey := uniqueAppKey()

	oldIssuer := seedIssuer(ctx, t, pool, 9501)
	id := seedProviderAccount(ctx, t, pool, repo, oldIssuer, taxID, appKey, "ACTIVE", true)

	row := readProviderRow(ctx, t, pool, id)
	cutover := time.Now().UTC().Add(time.Hour)
	if ok, err := repo.SetAutomationCutover(ctx, id, row.Version, cutover, time.Now().UTC()); err != nil || !ok {
		t.Fatalf("設定切點失敗：ok=%v err=%v", ok, err)
	}

	// 撤銷之前，同一組（統編, AppKey）不得被新的 issuer 綁走。
	newIssuer := seedIssuer(ctx, t, pool, 9502)
	if err := repo.Create(ctx, newAccount(newIssuer+1, newIssuer, taxID, appKey)); err == nil {
		t.Fatal("撤銷之前，第二個 issuer 不該綁得到同一組憑證")
	}

	row = readProviderRow(ctx, t, pool, id)
	if ok, err := repo.Revoke(ctx, id, row.Version, "賣家離場", time.Now().UTC()); err != nil || !ok {
		t.Fatalf("Revoke 失敗：ok=%v err=%v", ok, err)
	}

	after := readProviderRow(ctx, t, pool, id)
	if after.Status != "REVOKED" {
		t.Fatalf("狀態應為 REVOKED，實得 %q", after.Status)
	}
	// ① 切點保留。
	if after.CutoverAt == nil {
		t.Fatal("撤銷不得清空 automation_cutover_at（歷史訂單的模式判定要用它）")
	}
	// ② 名額釋放（I-37 第二向）。
	if err := repo.Create(ctx, newAccount(newIssuer+1, newIssuer, taxID, appKey)); err != nil {
		t.Fatalf("撤銷之後同一組憑證應可被新 issuer 綁定（uq_ipa_fingerprint 的述詞不含 REVOKED）：%v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 27：輪替三段的 chk_ipa_rotation 雙向等式
// ---------------------------------------------------------------------------

// TestIntegration_RotationLifecycle 驗 P11 → P12／P13 的三個落點都滿足
// chk_ipa_rotation（(status='ROTATION_PENDING') = (pending_ciphertext IS NOT NULL)）
// 與 chk_ipa_rotation_triplet（三欄同進同出）。
//
// 這兩條 CHECK 在單元測試裡不存在，而「少清一半」的失效形狀是寫入當下 23514。
func TestIntegration_RotationLifecycle(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	t.Run("P13 驗證失敗保留原 key", func(t *testing.T) {
		issuer := seedIssuer(ctx, t, pool, 9601)
		oldKey := uniqueAppKey()
		id := seedProviderAccount(ctx, t, pool, repo, issuer, "12345678", oldKey, "ACTIVE", true)

		row := readProviderRow(ctx, t, pool, id)
		if err := repo.StartCredentialRotation(ctx, domain.StartCredentialRotation{
			ID: id, ProviderTaxID: "12345678", AppKey: uniqueAppKey(), ExpectedVersion: row.Version,
		}); err != nil {
			t.Fatalf("StartCredentialRotation: %v", err)
		}
		mid := readProviderRow(ctx, t, pool, id)
		if mid.Status != "ROTATION_PENDING" || mid.Pending == nil {
			t.Fatalf("輪替中應同時有 ROTATION_PENDING 與 pending_ciphertext：%+v", mid)
		}

		ok, err := repo.AbortCredentialRotation(ctx, id, mid.Version, "憑證驗證失敗", time.Now().UTC())
		if err != nil || !ok {
			t.Fatalf("AbortCredentialRotation：ok=%v err=%v", ok, err)
		}
		after := readProviderRow(ctx, t, pool, id)
		if after.Status != "ACTIVE" || after.Pending != nil {
			t.Fatalf("P13 之後應回 ACTIVE 且 pending 三欄全清：%+v", after)
		}
		// 🔴 原 key 必須還解得開——這正是「先驗證再覆蓋」的全部意義。
		if err := repo.WithDecryptedCredential(ctx, id, func(_, appKey string) error {
			if appKey != oldKey {
				t.Fatalf("P13 之後現行憑證被覆蓋了：got %q want %q", appKey, oldKey)
			}
			return nil
		}); err != nil {
			t.Fatalf("WithDecryptedCredential: %v", err)
		}
	})

	t.Run("P12 扶正", func(t *testing.T) {
		issuer := seedIssuer(ctx, t, pool, 9602)
		id := seedProviderAccount(ctx, t, pool, repo, issuer, "12345678", uniqueAppKey(), "ACTIVE", true)
		newKey := uniqueAppKey()

		row := readProviderRow(ctx, t, pool, id)
		if err := repo.StartCredentialRotation(ctx, domain.StartCredentialRotation{
			ID: id, ProviderTaxID: "12345678", AppKey: newKey, ExpectedVersion: row.Version,
		}); err != nil {
			t.Fatalf("StartCredentialRotation: %v", err)
		}
		mid := readProviderRow(ctx, t, pool, id)
		if err := repo.PromoteCredential(ctx, domain.PromoteCredential{
			ID: id, ProviderTaxID: "12345678", AppKey: newKey,
			NextStatus: "ACTIVE", ExpectedVersion: mid.Version,
		}); err != nil {
			t.Fatalf("PromoteCredential: %v", err)
		}
		after := readProviderRow(ctx, t, pool, id)
		if after.Status != "ACTIVE" || after.Pending != nil {
			t.Fatalf("P12 之後應為 ACTIVE 且 pending 三欄全清：%+v", after)
		}
		if err := repo.WithDecryptedCredential(ctx, id, func(_, appKey string) error {
			if appKey != newKey {
				t.Fatalf("扶正之後現行憑證應為新 key：got %q want %q", appKey, newKey)
			}
			return nil
		}); err != nil {
			t.Fatalf("WithDecryptedCredential: %v", err)
		}
	})

	// 🔴 P11 的 allowed-from：ROTATION_PENDING 只能從 ACTIVE 進去。
	// 少了它，一把還沒被收下的 key 可以直接覆蓋現役憑證。
	t.Run("P11 只能從 ACTIVE", func(t *testing.T) {
		issuer := seedIssuer(ctx, t, pool, 9603)
		id := seedProviderAccount(ctx, t, pool, repo, issuer, "12345678", uniqueAppKey(), "INVALID_CREDENTIAL", true)
		row := readProviderRow(ctx, t, pool, id)
		err := repo.StartCredentialRotation(ctx, domain.StartCredentialRotation{
			ID: id, ProviderTaxID: "12345678", AppKey: uniqueAppKey(), ExpectedVersion: row.Version,
		})
		if err == nil {
			t.Fatal("非 ACTIVE 的列不該進得了 ROTATION_PENDING")
		}
		after := readProviderRow(ctx, t, pool, id)
		if after.Status != "INVALID_CREDENTIAL" || after.Pending != nil {
			t.Fatalf("守衛不成立時不得寫入任何欄位：%+v", after)
		}
	})
}
