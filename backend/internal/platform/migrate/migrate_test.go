package migrate

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

func testModule(name string, files map[string]string) Module {
	fsys := fstest.MapFS{}
	var list []file
	for n, body := range files {
		fsys[n] = &fstest.MapFile{Data: []byte(body)}
		version, _, _ := strings.Cut(strings.TrimSuffix(n, ".sql"), "_")
		list = append(list, file{name: n, version: version, body: []byte(body)})
	}
	sortFiles(list)
	fsys["atlas.sum"] = &fstest.MapFile{Data: []byte(sumFile(list))}
	return Module{Name: name, Dir: fsys}
}

func sortFiles(fs []file) {
	for i := 1; i < len(fs); i++ {
		for j := i; j > 0 && fs[j].name < fs[j-1].name; j-- {
			fs[j], fs[j-1] = fs[j-1], fs[j]
		}
	}
}

// testdata/atlas-generated 是 content-service 由 Atlas 產生的 migration 目錄原樣複製過來的。
func TestSumMatchesWhatAtlasGenerates(t *testing.T) {
	files, err := load(Module{Name: "content", Dir: os.DirFS("testdata/atlas-generated")})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 14 {
		t.Fatalf("讀到 %d 個 migration，預期 14", len(files))
	}
}

func TestLoadRejectsInvalidDirectories(t *testing.T) {
	good := testModule("m", map[string]string{"001_a.sql": "CREATE SCHEMA m;"})
	tampered := testModule("m", map[string]string{"001_a.sql": "CREATE SCHEMA m;"})
	tampered.Dir.(fstest.MapFS)["001_a.sql"] = &fstest.MapFile{Data: []byte("CREATE SCHEMA m; DROP TABLE x;")}
	noSum := testModule("m", map[string]string{"001_a.sql": "CREATE SCHEMA m;"})
	delete(noSum.Dir.(fstest.MapFS), "atlas.sum")

	cases := []struct {
		name string
		mod  Module
		want string
	}{
		{"檔案被改過但 atlas.sum 沒更新", tampered, "atlas.sum 和 migration 檔不符"},
		{"沒有 atlas.sum", noSum, "atlas.sum"},
		{"檔名沒有版本", testModule("m", map[string]string{"init.sql": "SELECT 1;"}), "檔名不是"},
		{"兩個檔案同一個版本", testModule("m", map[string]string{"001_a.sql": "SELECT 1;", "001_b.sql": "SELECT 2;"}), "版本相同"},
		{"尚未支援的 directive", testModule("m", map[string]string{"001_a.sql": "-- atlas:txmode none\n\nCREATE INDEX CONCURRENTLY i ON t (c);"}), "directive"},
	}
	if _, err := load(good); err != nil {
		t.Fatalf("合法的目錄被拒絕：%v", err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := load(c.mod)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v，預期包含「%s」", err, c.want)
			}
		})
	}
}

func TestDirectiveOnlyReadsTheFileHeader(t *testing.T) {
	cases := map[string]string{
		"-- atlas:txmode none\n\nCREATE INDEX CONCURRENTLY i ON t (c);": "-- atlas:txmode none",
		"-- 說明\n-- atlas:txmode none\nSELECT 1;":                        "-- atlas:txmode none",
		"SELECT 1;\n-- atlas:txmode none 只是註解裡提到":                       "",
		"-- 一般註解\nSELECT 1;":                                            "",
	}
	for body, want := range cases {
		if got := directive([]byte(body)); got != want {
			t.Errorf("directive(%q) = %q，預期 %q", body, got, want)
		}
	}
}

func TestExtensionsSQLInstallsEveryPlatformExtension(t *testing.T) {
	want := ""
	for _, e := range Extensions() {
		want += "CREATE EXTENSION IF NOT EXISTS " + e + " WITH SCHEMA public;\n"
	}
	if got := ExtensionsSQL(); got != want || !strings.Contains(got, "pg_trgm") {
		t.Fatalf("ExtensionsSQL = %q，預期 %q（含 pg_trgm，catalog 商品搜尋 D15）", got, want)
	}
}

var createExtension = regexp.MustCompile(`(?im)^\s*CREATE\s+EXTENSION\b`)

// extension 是 database 層級、由平台擁有（extensions）：模組的 desired state 與 migration 不得自己建立，
// 否則 Atlas 產生 migration 時要靠手動步驟，也繞過平台清單。
func TestModulesDoNotCreateExtensions(t *testing.T) {
	roots := []string{"../../modules", "../../platform"}
	checked := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".sql") || !strings.Contains(filepath.ToSlash(p), "/db/") {
				return err
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			checked++
			if createExtension.Match(body) {
				t.Errorf("%s 含 CREATE EXTENSION：extension 只在 internal/platform/migrate 的 extensions 宣告", p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked == 0 {
		t.Fatal("沒有找到任何模組的 SQL，守門沒有意義")
	}
}
