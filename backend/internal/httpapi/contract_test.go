package httpapi

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

var (
	protoPackage = regexp.MustCompile(`(?m)^package\s+([\w.]+)\s*;`)
	protoService = regexp.MustCompile(`(?m)^service\s+(\w+)\s*\{`)
)

// contractServices 列出根目錄 api/ 宣告的每個 service 全名。
func contractServices(t *testing.T) []string {
	t.Helper()
	var out []string
	root := filepath.Join("..", "..", "..", "api")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".proto") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pkg := protoPackage.FindSubmatch(src)
		if pkg == nil {
			t.Fatalf("%s 沒有 package", path)
		}
		for _, m := range protoService.FindAllSubmatch(src, -1) {
			out = append(out, string(pkg[1])+"."+string(m[1]))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

// declaredNotYetMounted 是契約已宣告、handler 在後續步驟才接線的 service（值是接線的步驟）。接線之後必須從這裡刪除：下面的測試在它已經掛上時失敗。
var declaredNotYetMounted = rpc.DeferredServices

func TestEveryServiceInTheContractIsMounted(t *testing.T) {
	var mounted []string
	services, err := connectServices(Deps{Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), Images: characterize.Images(t), Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range services {
		mounted = append(mounted, strings.Trim(s.path, "/"))
	}
	slices.Sort(mounted)
	want := contractServices(t)
	for name, step := range declaredNotYetMounted {
		if slices.Contains(mounted, name) {
			t.Errorf("%s 已經掛上，從 declaredNotYetMounted（%s）刪除它", name, step)
		}
		want = slices.DeleteFunc(want, func(s string) bool { return s == name })
	}
	if !slices.Equal(mounted, want) {
		t.Fatalf("單體掛的 service = %v，api/ 宣告的 = %v", mounted, want)
	}
}
