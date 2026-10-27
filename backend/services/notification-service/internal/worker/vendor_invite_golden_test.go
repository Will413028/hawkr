package worker

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mail"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite the vendor invite golden file")

// 這封信是站外業者對平台的第一個接觸點，而它沒有任何 UI 測試看得到——
// 改壞了只會在真的寄出去之後才被發現。golden 比對讓模板改動在 PR 上就轉紅。
//
// 要看信真正的渲染結果（Gmail／Outlook 實際長相），走 dev 的 redirect：
// MAIL_PROVIDER=resend ＋ MAIL_RECIPIENT_POLICY=redirect，信會進 MAIL_REDIRECT_TO
// 那個信箱。本測試守的是內容不被無聲改掉，不是渲染。
func TestBuildVendorInviteMessage_Golden(t *testing.T) {
	got := buildVendorInviteMessage(
		mail.Address{Email: "supports@example.com", Name: "範例生鮮"},
		"vendor@example.com",
		"https://web-dev.example.com",
		"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		externalInvitePayload{
			InviteID:      "7500000000000000001",
			PublicationID: "7500000000000000002",
			ProspectID:    "7500000000000000003",
			ExpiresAt:     "2026-10-11T15:27:07Z",
		},
	)

	// ⛔ 收件人、secret 與三個 ID 全部是合成值（CLAUDE.md §19）：golden 檔進版控，
	// 真實業者信箱是個資、真實 secret 是憑證，兩者都不得落在 fixture 裡。
	path := filepath.Join("testdata", "vendor_invite_message.json")
	encoded, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')

	if *updateGolden {
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Log("golden 已更新:", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀 golden 失敗（第一次建立請跑 go test ./internal/worker -run Golden -update-golden）: %v", err)
	}
	if string(encoded) != string(want) {
		t.Errorf("邀請信內容與 golden 不符。\n--- got ---\n%s\n--- want ---\n%s", encoded, want)
	}
}

// 🔴 secret 必須在 fragment（#）不是 query（?）。fragment 不會隨 HTTP 請求送出，
// 所以不會落進 access log、也不會經由 Referer 外洩給連結指向的第三方資源。
// 改成 query string 是個看起來無害、但會讓一次性憑證進入伺服器日誌的改動。
func TestBuildVendorInviteMessage_SecretStaysInFragment(t *testing.T) {
	const secret = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	got := buildVendorInviteMessage(
		mail.Address{Email: "supports@example.com"},
		"vendor@example.com",
		"https://web-dev.example.com",
		secret,
		externalInvitePayload{InviteID: "1", ExpiresAt: "2026-10-11T15:27:07Z"},
	)
	for name, body := range map[string]string{"text": got.Text, "html": got.HTML} {
		if !contains(body, "#invite="+secret) {
			t.Errorf("%s 少了 fragment 形式的 secret", name)
		}
		if contains(body, "?invite=") {
			t.Errorf("%s 把 secret 放進 query string —— 那會進 access log 與 Referer", name)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
