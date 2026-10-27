package delegationservice

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// CodeMeta〈錯誤碼〉的代行段是這裡業務碼的投影（擁有者是 delegation，ADR 2026-09-25-codemeta-retirement 過渡第 1 步）：
// gateway 與後台以 CodeMeta 的名稱解讀這些碼，兩邊的數字不一致時，gateway 會把拒絕對到錯的 HTTP status。
var codemetaNames = map[delegation.DelegationDenial]string{
	delegation.DenialInvalidCommand:             "DELEGATION_INVALID_COMMAND",
	delegation.DenialAdminRequired:              "DELEGATION_ADMIN_SESSION_REQUIRED",
	delegation.DenialCapabilityDenied:           "DELEGATION_CAPABILITY_DENIED",
	delegation.DenialAuthorizationRevoked:       "DELEGATION_AUTHORIZATION_REVOKED",
	delegation.DenialStoreNotDelegable:          "DELEGATION_STORE_NOT_DELEGABLE",
	delegation.DenialStoreClosed:                "DELEGATION_STORE_NOT_DELEGABLE",
	delegation.DenialIdempotencyKeyReused:       "DELEGATION_IDEMPOTENCY_KEY_REUSED",
	delegation.DenialActorBusy:                  "DELEGATION_CONFLICT_ACTOR_OTHER_STORE",
	delegation.DenialStoreBusy:                  "DELEGATION_CONFLICT_STORE_OTHER_ACTOR",
	delegation.DenialSessionExpired:             "DELEGATION_SESSION_EXPIRED",
	delegation.DenialSessionRevoked:             "DELEGATION_SESSION_REVOKED",
	delegation.DenialStoreNotOperable:           "DELEGATION_STORE_NOT_OPERABLE",
	delegation.DenialCredentialInvalid:          "DELEGATION_CREDENTIAL_INVALID",
	delegation.DenialSessionNotFound:            "DELEGATION_SESSION_NOT_FOUND",
	delegation.DenialAuditIntentFailed:          "DELEGATION_AUDIT_INTENT_FAILED",
	delegation.DenialHandoffInvalid:             "DELEGATION_HANDOFF_INVALID",
	delegation.DenialAuthorizationAlreadyActive: "DELEGATION_AUTHORIZATION_ALREADY_ACTIVE",
	delegation.DenialAuthorizationNotFound:      "DELEGATION_AUTHORIZATION_NOT_FOUND",
	delegation.DenialAdminNotAllowlisted:        "DELEGATION_ADMIN_NOT_ALLOWLISTED",
}

func TestBusinessCodesMatchCodeMeta(t *testing.T) {
	md, err := os.ReadFile("../../../docs/代碼管理（CodeMeta）/錯誤碼.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(md), "### 代行營運業務碼")
	if !ok {
		t.Fatal("CodeMeta〈錯誤碼〉沒有代行營運業務碼那一段")
	}
	section, _, _ = strings.Cut(section, "\n### ")
	codemeta := map[string]int32{}
	for _, m := range regexp.MustCompile(`(?m)^\| (DELEGATION_[A-Z_]+) \| (\d+) \|`).FindAllStringSubmatch(section, -1) {
		n, _ := strconv.Atoi(m[2])
		codemeta[m[1]] = int32(n)
	}
	if len(codemeta) == 0 {
		t.Fatal("代行營運業務碼那一段解析不到任何碼")
	}
	for d, code := range bizCodes {
		name, ok := codemetaNames[d]
		if !ok {
			t.Errorf("拒絕 %d（%d）沒有對應的 CodeMeta 名稱", d, code)
			continue
		}
		if want, ok := codemeta[name]; !ok || want != code {
			t.Errorf("拒絕 %d 回 %d，CodeMeta 的 %s 是 %d", d, code, name, want)
		}
	}
}
