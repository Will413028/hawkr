package porkmarket

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const (
	realCSV      = "../../../reference/pork_market_registry.v1.csv"
	realManifest = "../../../reference/pork_market_registry.v1.manifest.json"
)

func TestLoad_RealRegistry(t *testing.T) {
	r, err := Load(realCSV, realManifest)
	if err != nil {
		t.Fatalf("載入實際 registry 失敗: %v", err)
	}
	if r.Len() != 19 {
		t.Errorf("市場數 = %d，預期 19", r.Len())
	}
	if r.Version() == "" {
		t.Error("registry_version 為空")
	}
}

func TestResolve_UpstreamMixesTaiVariants(t *testing.T) {
	r, err := Load(realCSV, realManifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"台中大安", "台東縣", "臺南安南", "臺南市"} {
		if _, err := r.Resolve(name); err != nil {
			t.Errorf("Resolve(%q) 失敗: %v", name, err)
		}
	}

	if _, err := r.Resolve("台南市"); !errors.Is(err, ErrUnknownMarket) {
		t.Errorf("Resolve(台南市) 應為 ErrUnknownMarket，got %v", err)
	}
}

func TestResolve_KaohsiungMarketsAreDistinct(t *testing.T) {
	r, err := Load(realCSV, realManifest)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, n := range []string{"高雄市", "高雄岡山", "高雄旗山", "高雄鳳山"} {
		code, err := r.Resolve(n)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", n, err)
		}
		if prev, dup := seen[code]; dup {
			t.Fatalf("%q 與 %q 對到同一個代碼 %s", n, prev, code)
		}
		seen[code] = n
	}
}

func TestResolve_UnknownMarketFailsClosed(t *testing.T) {
	r, err := Load(realCSV, realManifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve("澎湖縣"); !errors.Is(err, ErrUnknownMarket) {
		t.Errorf("未知市場應回 ErrUnknownMarket，got %v", err)
	}
}

func TestLoad_DigestMismatchRejected(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "r.csv")
	mfPath := filepath.Join(dir, "r.manifest.json")
	if err := os.WriteFile(csvPath, []byte("registry_version,market_code,source_market_name\nv1,PORK-01,雲林縣\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mfPath, []byte(`{"sha256":"deadbeef","row_count":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(csvPath, mfPath); !errors.Is(err, ErrDigestMismatch) {
		t.Errorf("應回 ErrDigestMismatch，got %v", err)
	}
}
