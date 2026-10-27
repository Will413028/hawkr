package postgres

import (
	"crypto/rand"
	"encoding/hex"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// randomSecret 產一段不可能碰巧出現在錯誤訊息裡的值。
//
// 用隨機值 ＋ 反向斷言（「輸出裡找不到它」）而不是比對固定字串：固定字串的斷言在
// 訊息改寫時會一起被改掉，隨機值的斷言只有在真的洩漏時才會紅。
func randomSecret(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "AK" + hex.EncodeToString(b)
}

func newCipher(t *testing.T, secret string) *crypto.Cipher {
	t.Helper()
	c, err := crypto.New(secret)
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return c
}

// ---------------------------------------------------------------------------
// 鑑別力 ①：空明文加密 → 硬錯（不是存空字串）
// ---------------------------------------------------------------------------

func TestEncryptCredential_EmptyPlaintextIsHardError(t *testing.T) {
	enc := newCipher(t, "unit-test-master-key")

	got, err := encryptCredential(enc, "")
	if err == nil {
		t.Fatalf("encryptCredential(\"\") 必須硬錯；實際回 (%q, nil)", got)
	}
	if got != "" {
		t.Fatalf("硬錯時不得回任何密文，實際 %q", got)
	}

	// 只有空白的輸入經過 normalizeAppKey 之後也是空的 —— deriveCredential 必須一樣硬錯，
	// 否則 DB 會存下一列「有憑證但解出來是空字串」的帳戶。
	ring := &KeyRing{Current: enc, CurrentVersion: 1, LookupHMACKey: "lookup"}
	if _, err := ring.deriveCredential("12345678", "   \t\n "); err == nil {
		t.Fatal("deriveCredential 收到全空白 app_key 時必須硬錯")
	}
}

// ---------------------------------------------------------------------------
// 鑑別力 ②：用錯的 key 解密 → 硬錯，且錯誤訊息不含密文或明文任何片段
// ---------------------------------------------------------------------------

func TestDecryptCredential_WrongKeyIsHardErrorAndLeaksNothing(t *testing.T) {
	plaintext := randomSecret(t)
	right := newCipher(t, "master-key-A")
	wrong := newCipher(t, "master-key-B")

	ciphertext, err := encryptCredential(right, plaintext)
	if err != nil {
		t.Fatalf("encryptCredential: %v", err)
	}

	got, err := decryptCredential(wrong, ciphertext)
	if err == nil {
		t.Fatal("用錯的 key 解密必須硬錯，不得回空字串或垃圾明文")
	}
	if got != "" {
		t.Fatalf("硬錯時不得回任何明文，實際 %q", got)
	}

	msg := err.Error()
	if strings.Contains(msg, plaintext) {
		t.Fatalf("錯誤訊息洩漏了明文 AppKey：%q", msg)
	}
	if strings.Contains(msg, ciphertext) {
		t.Fatalf("錯誤訊息洩漏了密文：%q", msg)
	}
	// 密文的任何一段（16 字元以上）都不該出現。整串比對擋不住「只印前 N 碼」這種寫法。
	for i := 0; i+16 <= len(ciphertext); i += 8 {
		if strings.Contains(msg, ciphertext[i:i+16]) {
			t.Fatalf("錯誤訊息含密文片段 %q：%q", ciphertext[i:i+16], msg)
		}
	}
	// 明文同理。
	for i := 0; i+8 <= len(plaintext); i += 4 {
		if strings.Contains(msg, plaintext[i:i+8]) {
			t.Fatalf("錯誤訊息含明文片段 %q：%q", plaintext[i:i+8], msg)
		}
	}
}

// TestRepositoryPackageDoesNotLog 是上一支測試的結構面補充。
//
// 「錯誤訊息不含明文」擋得住錯誤這條路，擋不住「有人在 repository 裡加一行
// logger.Info("appKey", ...)」。本 package 從頭到尾不 log，這條測試把那件事釘住：
// 一旦有人引入 logger，就必須先來改這支測試——而那正是需要有人思考的時刻。
func TestRepositoryPackageDoesNotLog(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	// 只列真正會把值印出去的 package。fmt 不在內：它在本層只用來組 error 訊息，
	// 而「error 訊息不含憑證材料」由上一支測試守。
	banned := []string{"go.uber.org/zap", "log", "log/slog"}
	parsed := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		parsed++
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, b := range banned {
				if path == b {
					t.Errorf("%s 引入了 %s：AppKey 明文所在的 package 不得有 logger", name, path)
				}
			}
		}
	}
	// 沒有這一句，「解析到 0 個檔」會靜默通過（memory: 量測要有鑑別力）。
	if parsed < 5 {
		t.Fatalf("只解析到 %d 個檔，測試本身沒有鑑別力", parsed)
	}
}

// ---------------------------------------------------------------------------
// 鑑別力 ③：fingerprint 綁統編
// ---------------------------------------------------------------------------

func TestCredentialFingerprint_BindsTaxIDAndAppKey(t *testing.T) {
	const secret = "lookup-hmac-key"
	appKey := randomSecret(t)
	other := randomSecret(t)

	same := credentialFingerprint(secret, "12345678", appKey)
	if again := credentialFingerprint(secret, "12345678", appKey); same != again {
		t.Fatal("同統編＋同 AppKey 必須得到相同 fingerprint（GCM 密文每次不同，比對只能靠它）")
	}

	// 同 AppKey ＋ 不同統編 ⇒ 不同 fingerprint。
	// 這一條是「含統編」的全部意義：拿掉統編這裡就會相等，而 uq_ipa_fingerprint 會把
	// 「光貿對不同帳戶換發同一把 AppKey」的兩個真實無關賣家誤判成重複綁定。
	if diff := credentialFingerprint(secret, "87654321", appKey); diff == same {
		t.Fatal("不同統編＋同 AppKey 必須得到不同 fingerprint（fingerprint 沒有把統編算進去）")
	}

	// 同統編 ＋ 不同 AppKey ⇒ 不同 fingerprint（U3 靠它判斷「這把 key 真的換了嗎」）。
	if diff := credentialFingerprint(secret, "12345678", other); diff == same {
		t.Fatal("同統編＋不同 AppKey 必須得到不同 fingerprint")
	}

	// 換一把 lookup key ⇒ 全表 fingerprint 改變。這正是它實務上不可輪替的原因，
	// 也是它必須與 master key 分開的原因（master key 輪替不該引爆全表重算）。
	if diff := credentialFingerprint("another-lookup-key", "12345678", appKey); diff == same {
		t.Fatal("換 lookup key 之後 fingerprint 不該相同")
	}

	// 分隔符必須是換行：沒有分隔符時 ("1234"+"5678AK") 與 ("12345678"+"AK") 會撞在一起。
	if credentialFingerprint(secret, "1234", "5678"+appKey) == credentialFingerprint(secret, "12345678", appKey) {
		t.Fatal("fingerprint 的輸入沒有分隔符，統編與 app_key 的邊界可被構造出碰撞")
	}
}

func TestNormalizeAppKeyAndLast4(t *testing.T) {
	if got := normalizeAppKey("  abcd1234  "); got != "abcd1234" {
		t.Fatalf("normalizeAppKey 只 trim 前後空白，得到 %q", got)
	}
	if got := normalizeAppKey("AbCd"); got != "AbCd" {
		t.Fatalf("normalizeAppKey 不得改大小寫，得到 %q", got)
	}
	if got := credentialLast4("ABCDEFGH1234"); got != "1234" {
		t.Fatalf("credentialLast4 = %q, want 1234", got)
	}
	if got := credentialLast4("ab"); got != "ab" {
		t.Fatalf("短於四碼時原樣回傳，得到 %q", got)
	}
}

func TestDeriveCredential_AllThreeDerivativesComeFromTheSameNormalizedPlaintext(t *testing.T) {
	enc := newCipher(t, "master-key-A")
	ring := &KeyRing{Current: enc, CurrentVersion: 7, LookupHMACKey: "lookup"}
	appKey := randomSecret(t)

	mat, err := ring.deriveCredential("12345678", "  "+appKey+"  ")
	if err != nil {
		t.Fatalf("deriveCredential: %v", err)
	}
	if mat.keyVersion != 7 {
		t.Fatalf("寫入端一律用 current 版本，得到 %d", mat.keyVersion)
	}
	// 密文解回來必須是 **trim 過** 的明文：三個衍生值同源，否則會出現
	// 「fingerprint 對得上但解出來的 key 打不通 AMEGO」這種沒有斷言擋得住的錯位。
	plain, err := decryptCredential(enc, mat.ciphertext)
	if err != nil {
		t.Fatalf("decryptCredential: %v", err)
	}
	if plain != appKey {
		t.Fatalf("密文存的不是正規化後的明文：%q", plain)
	}
	if mat.fingerprint != credentialFingerprint("lookup", "12345678", appKey) {
		t.Fatal("fingerprint 不是由正規化後的明文算出來的")
	}
	if mat.last4 != credentialLast4(appKey) {
		t.Fatal("last4 不是由正規化後的明文取的")
	}
}

// ---------------------------------------------------------------------------
// 鑑別力 ④：輪替期間 KeyRing 依版本選 cipher
// ---------------------------------------------------------------------------

func TestKeyRing_CipherForSelectsByVersionAndNeverFallsBack(t *testing.T) {
	cur := newCipher(t, "master-key-new")
	prev := newCipher(t, "master-key-old")
	ring := &KeyRing{Current: cur, Prev: prev, CurrentVersion: 2, PrevVersion: 1, LookupHMACKey: "lookup"}

	oldPlain := randomSecret(t)
	oldCipherText, err := encryptCredential(prev, oldPlain)
	if err != nil {
		t.Fatalf("encrypt with prev: %v", err)
	}

	// 舊 key 加密的殘列，帶著 version=1 仍解得開（master key 輪替步驟 1~3 的窗口期）。
	got, err := ring.openCredential(gen.InvoiceProviderAccount{
		CredentialKeyVersion: 1,
		CredentialCiphertext: oldCipherText,
	})
	if err != nil {
		t.Fatalf("prev-version row 必須解得開：%v", err)
	}
	if got != oldPlain {
		t.Fatal("prev-version row 解出來的明文不對")
	}

	// 版本對不上一律硬錯，**不 try-and-fallback**：fallback 會把「這一列真的損毀了」與
	// 「這一列是舊 key 加密的」壓成同一條路徑，輪替步驟 3 的驗證述詞失去意義。
	if _, err := ring.openCredential(gen.InvoiceProviderAccount{
		CredentialKeyVersion: 9,
		CredentialCiphertext: oldCipherText,
	}); err == nil {
		t.Fatal("未知的 credential_key_version 必須硬錯")
	}

	// _PREV 已在輪替步驟 4 移除，卻還有殘列 ⇒ 硬錯（而不是靜默解不開）。
	retired := &KeyRing{Current: cur, CurrentVersion: 2, PrevVersion: 1, LookupHMACKey: "lookup"}
	if _, err := retired.openCredential(gen.InvoiceProviderAccount{
		CredentialKeyVersion: 1,
		CredentialCiphertext: oldCipherText,
	}); err == nil {
		t.Fatal("指向已移除 key 版本的殘列必須硬錯")
	}
}

// ---------------------------------------------------------------------------
// 去敏投影：domain 型別裡不得出現密文或 fingerprint
// ---------------------------------------------------------------------------

func TestProviderAccountProjection_CarriesNoCredentialMaterial(t *testing.T) {
	row := gen.InvoiceProviderAccount{
		ID:                    1,
		CredentialCiphertext:  "SHOULD-NEVER-APPEAR",
		CredentialFingerprint: "FINGERPRINT-SHOULD-NEVER-APPEAR",
		CredentialLast4:       "1234",
		PendingCiphertext:     sp("PENDING-SHOULD-NEVER-APPEAR"),
		PendingFingerprint:    sp("PENDING-FP-SHOULD-NEVER-APPEAR"),
	}
	got := toProviderAccount(row)

	if !got.HasPendingCredential {
		t.Fatal("pending_ciphertext 非空時 HasPendingCredential 必須為 true")
	}

	// 型別層的保證：任何名字帶 Ciphertext / Fingerprint 的欄位都不准存在。
	// 這條擋的是「未來有人為了方便加一個欄位」——那不會編譯錯、不會有其他測試紅。
	rt := reflect.TypeOf(got)
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		if strings.Contains(name, "Ciphertext") || strings.Contains(name, "Fingerprint") {
			t.Errorf("domain.ProviderAccount 不得有 %s 欄位（憑證材料不出 repository 層）", name)
		}
	}

	// 值層的保證：整個 struct 的字串內容裡找不到那些哨兵值。
	dump := dumpStrings(reflect.ValueOf(got))
	for _, sentinel := range []string{"SHOULD-NEVER-APPEAR", "FINGERPRINT-SHOULD-NEVER-APPEAR", "PENDING-SHOULD-NEVER-APPEAR", "PENDING-FP-SHOULD-NEVER-APPEAR"} {
		if strings.Contains(dump, sentinel) {
			t.Errorf("投影後仍帶出 %q", sentinel)
		}
	}
}

func dumpStrings(v reflect.Value) string {
	var sb strings.Builder
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.String:
			sb.WriteString(v.String())
			sb.WriteByte('\n')
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i))
			}
		}
	}
	walk(v)
	return sb.String()
}

// ---------------------------------------------------------------------------
// 課稅別：讀路徑 fail-closed，不 fallback 成 1（應稅）
// ---------------------------------------------------------------------------

func TestToDocumentLine_UnknownTaxCategoryIsHardError(t *testing.T) {
	for _, tc := range []int16{1, 2, 3} {
		if _, err := toDocumentLine(gen.InvoiceDocumentLine{ID: 1, TaxCategory: tc}); err != nil {
			t.Fatalf("合法課稅別 %d 不該報錯：%v", tc, err)
		}
	}
	// 9（混合）只合法於表頭，行級不得出現；0 與 4 根本不在值域內。
	for _, tc := range []int16{0, 4, 9, -1} {
		got, err := toDocumentLine(gen.InvoiceDocumentLine{ID: 1, TaxCategory: tc})
		if err == nil {
			t.Fatalf("課稅別 %d 必須硬錯，實際回 %+v", tc, got)
		}
		if got.TaxCategory != 0 {
			t.Fatalf("硬錯時不得回半成品（TaxCategory=%d）", got.TaxCategory)
		}
		// 🔴 最重要的一條：不得 fallback 成 1（應稅）。
		if got.TaxCategory == int16(domain.TaxCategoryTaxable) {
			t.Fatal("轉換失敗被 fallback 成應稅：一張免稅發票會被靜默開成應稅")
		}
	}
}

func TestToAdjustmentLine_UnknownTaxCategoryIsHardError(t *testing.T) {
	if _, err := toAdjustmentLine(gen.InvoiceAdjustmentLine{ID: 1, TaxCategory: 3}); err != nil {
		t.Fatalf("免稅（3）不該報錯：%v", err)
	}
	if _, err := toAdjustmentLine(gen.InvoiceAdjustmentLine{ID: 1, TaxCategory: 9}); err == nil {
		t.Fatal("折讓明細的課稅別值域只有 1/2/3，9 必須硬錯")
	}
}

func TestToDocument_HeaderTaxTypeAllowsMixedButNotGarbage(t *testing.T) {
	for _, tt := range []int16{1, 2, 3, 9} {
		if _, err := toDocument(gen.InvoiceDocument{ID: 1, TaxType: tt}); err != nil {
			t.Fatalf("表頭課稅別 %d 應合法（9 ＝ 混合）：%v", tt, err)
		}
	}
	for _, tt := range []int16{0, 4, 10} {
		if _, err := toDocument(gen.InvoiceDocument{ID: 1, TaxType: tt}); err == nil {
			t.Fatalf("表頭課稅別 %d 必須硬錯", tt)
		}
	}
}
