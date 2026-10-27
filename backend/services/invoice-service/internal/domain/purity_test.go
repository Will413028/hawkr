package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// internal/domain 是純函式層：零 I/O、零框架相依。這條靠靜態掃描守——
// 一旦有人在這一層 import pgx／fx／grpc／net/http，金額算式就不再能被單元測試逐條釘死。
func TestDomainHasNoIOOrFrameworkImports(t *testing.T) {
	banned := []string{
		"net/http", "database/sql", "os/exec",
		"github.com/jackc/pgx", "go.uber.org/fx", "google.golang.org/grpc",
		"github.com/gin-gonic/gin", "github.com/Will413028/hawkr/go-pkg/database",
		"github.com/Will413028/hawkr/go-pkg/mq", "github.com/Will413028/hawkr/go-pkg/logger",
	}
	dirs := []string{".", "fsm"}
	fset := token.NewFileSet()
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, imp := range f.Imports {
				p, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("%s: bad import %s", path, imp.Path.Value)
				}
				for _, b := range banned {
					if p == b || strings.HasPrefix(p, b+"/") {
						t.Errorf("%s imports %q —— domain 層不得有 I/O 或框架相依", path, p)
					}
				}
			}
		}
	}
}

// 鑑別力：上面的掃描必須真的看得到檔案（目錄打錯時它會靜默全綠）。
func TestDomainPurityScanSeesFiles(t *testing.T) {
	count := 0
	for _, dir := range []string{".", "fsm"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				count++
			}
		}
	}
	if count < 9 {
		t.Fatalf("only %d non-test .go files found in domain —— 掃描範圍可能是空的", count)
	}
}
