//go:build !integration

// statementOf 住在 fail_reason_visibility_test.go，那一檔是 `!integration` ⇒
// 本檔必須掛同一個 tag，否則 `go vet -tags integration` 會找不到它而編譯不過
// （§17.4.1 的第四道 gate；本檔第一次提交時就是這樣被 lefthook 擋下來的）。

package usecase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAbortRotationNeverTouchesLiveCredential 守憑證輪替最不能出錯的那一格：
// **驗證失敗時，現役憑證一個位元都不能動。**
//
// 這是「先驗證再覆蓋」的全部意義——賣家換 key 打錯字，代價應該是「這次沒換成」，
// 不是「原本能開票的憑證也沒了」。錯了的話該賣家從那一刻起每一張發票都開不出來，
// 而他自己不知道發生了什麼（畫面只會說憑證錯誤，而他剛好**確實**打錯了）。
//
// 🔴 判準錨在 SQL 本文而不是跑一次 UPDATE：這是那一句的**性質**，
// 而 usecase 層的替身重現不了 SET 子句。既有的
// TestProviderAccount_Rotate_VerifyFails 斷言的是替身的呼叫次數
// （promoted==0／aborted==1），那證明得了「有呼叫還原」，
// **證明不了「還原沒有順手把憑證清掉」**——把 credential_ciphertext = NULL
// 加進那一句，那支測試照樣全綠。
//
// dev 實證（2026-09-05，store 2636840881）：以一把不合法的 AppKey 走端點 27，
// 歷程留下 ACTIVE → ROTATION_PENDING → ACTIVE（相隔 290ms），
// credential_last4 前後都是 IM27、credential_key_version 都是 1、
// automation_cutover_at 不變。這是 dev 上 ROTATION_PENDING 第一次被走過。
func TestAbortRotationNeverTouchesLiveCredential(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "db", "queries", "invoice_provider_accounts.sql"))
	if err != nil {
		t.Fatalf("讀不到 db/queries/invoice_provider_accounts.sql：%v", err)
	}
	body := string(raw)

	abort := statementOf(t, body, "AbortProviderCredentialRotation")

	// 現役憑證四欄一個都不准出現在 SET 子句裡。
	for _, col := range []string{
		"credential_ciphertext",
		"credential_key_version",
		"credential_fingerprint",
		"credential_last4",
	} {
		if strings.Contains(abort, col+" =") {
			t.Errorf("🔴 AbortProviderCredentialRotation 動了 %s —— "+
				"輪替驗證失敗會連現役憑證一起毀掉，該賣家從此開不出任何發票", col)
		}
	}

	// 陽性：它必須真的清掉 pending 三欄並把 status 移回 ACTIVE，
	// 否則 chk_ipa_rotation（雙向等式）當場 23514、帳號永遠卡在 ROTATION_PENDING。
	for _, want := range []string{
		"pending_ciphertext = NULL",
		"pending_fingerprint = NULL",
		"pending_last4 = NULL",
		"status = 'ACTIVE'",
	} {
		if !strings.Contains(abort, want) {
			t.Errorf("AbortProviderCredentialRotation 缺少 %q —— "+
				"chk_ipa_rotation 是雙向等式，少做一半當場 23514", want)
		}
	}

	// 陰性對照：**扶正**那一句必須真的覆蓋憑證，否則上面那組斷言等於在守一個
	// 「兩句都不換 key」的世界，而那時輪替功能整個是假的。
	promote := statementOf(t, body, "UpdateProviderAccountCredential")
	if !strings.Contains(promote, "credential_ciphertext = sqlc.arg(credential_ciphertext)") {
		t.Error("UpdateProviderAccountCredential 沒有覆蓋 credential_ciphertext —— " +
			"輪替成功卻沒換 key，賣家以為換過了而外洩的那一把還在用")
	}
	// 兩句都必須夾住 allowed-from，否則一把還沒被收下的 key 可以直接覆蓋現役憑證。
	for name, stmt := range map[string]string{
		"AbortProviderCredentialRotation": abort,
		"UpdateProviderAccountCredential": promote,
	} {
		if !strings.Contains(stmt, "status = 'ROTATION_PENDING'") {
			t.Errorf("%s 的 WHERE 缺少 status = 'ROTATION_PENDING' 的 allowed-from", name)
		}
	}
}
