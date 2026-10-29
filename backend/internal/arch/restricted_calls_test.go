package arch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// identity 的停權、解除入口只有 accountsuspension 呼叫：非測試、非生成的程式裡，其他地方（moderation、legacy、httpapi、cmd）選取
// SuspendAccount、UnsuspendAccount 都失敗。
func TestSuspensionEntriesAreOnlyCalledByAccountsuspension(t *testing.T) {
	root := moduleRoot(t)
	scanned, allowed := 0, 0
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, "/testdata/") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if generatedFile(src) {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			res, err := CheckRestrictedCalls(filepath.ToSlash(rel), src, SuspensionEntries)
			if err != nil {
				return err
			}
			scanned++
			allowed += res.Allowed
			for _, v := range res.Violations {
				t.Error(v)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 300 || allowed < 3 {
		t.Fatalf("只掃到 %d 個檔、在允許的目錄看到 %d 處：檢查範圍不對", scanned, allowed)
	}
}

func TestCheckRestrictedCallsFlagsEveryCallerOutsideTheAllowedDirs(t *testing.T) {
	const call = "package x\n\nfunc f() { _ = m.identity.SuspendAccount(ctx, 1) }\n"
	const unsuspend = "package x\n\nfunc f() { _, _, _ = id.UnsuspendAccount(ctx, 1, \"r\") }\n"
	const value = "package x\n\nfunc f() { g := id.SuspendAccount; _ = g }\n"
	const clean = "package x\n\nfunc f() { _ = m.Suspend(ctx, 1); _ = AuthoritySuspendAccount; _ = pb.SuspendAccountRequest{} }\n"
	cases := []struct {
		name, file, src string
		want            int
	}{
		{"moderation 直接呼叫 SuspendAccount", "internal/modules/moderation/internal/case/case.go", call, 1},
		{"moderation 直接呼叫 UnsuspendAccount", "internal/modules/moderation/moderation.go", unsuspend, 1},
		{"legacy 直接呼叫", "internal/legacy/accountservice/server.go", call, 1},
		{"httpapi 取方法值", "internal/httpapi/identity/admin.go", value, 1},
		{"cmd 直接呼叫", "cmd/hawkr/main.go", call, 1},
		{"名稱相近的識別字不算", "internal/modules/moderation/moderation.go", clean, 0},
		{"accountsuspension 可以呼叫", "internal/modules/accountsuspension/accountsuspension.go", call, 0},
		{"identity 自己的實作可以", "internal/modules/identity/internal/account/account.go", call, 0},
		{"目錄名只是前綴相同的不算", "internal/modules/accountsuspensionx/x.go", call, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := CheckRestrictedCalls(c.file, []byte(c.src), SuspensionEntries)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Violations) != c.want {
				t.Fatalf("違規 %d 處，預期 %d：%v", len(res.Violations), c.want, res.Violations)
			}
			if c.want == 0 && c.name != "名稱相近的識別字不算" && res.Allowed != 1 {
				t.Fatalf("允許的目錄裡看到 %d 處，預期 1", res.Allowed)
			}
		})
	}
}

// 允許的目錄清單只有 accountsuspension 與 identity 自己：放寬它（例如加入 moderation）就是繞過「有店不能停權」。
func TestSuspensionEntriesAllowListIsExact(t *testing.T) {
	got := strings.Join(SuspensionEntries.AllowedDirs, ",")
	if got != "internal/modules/accountsuspension,internal/modules/identity" {
		t.Fatalf("允許呼叫停權入口的目錄 = %s", got)
	}
}
