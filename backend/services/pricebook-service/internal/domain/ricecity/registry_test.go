package ricecity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const (
	realCSV      = "../../../reference/rice_city_registry.v1.csv"
	realManifest = "../../../reference/rice_city_registry.v1.manifest.json"
)

func TestLoad_RealRegistry(t *testing.T) {
	r, err := Load(realCSV, realManifest)
	if err != nil {
		t.Fatalf("載入實際 registry 失敗: %v", err)
	}
	if r.Len() != 20 {
		t.Errorf("縣市數 = %d，預期 20", r.Len())
	}
	if r.Version() == "" {
		t.Error("registry_version 為空")
	}
}

func TestResolve_UsesOfficialCityCodes(t *testing.T) {
	r, err := Load(realCSV, realManifest)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"基隆市": "10017",
		"台北市": "63000",
		"澎湖縣": "10016",
	} {
		got, err := r.Resolve(name)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", name, err)
		}
		if got != want {
			t.Errorf("Resolve(%q) = %s，預期 %s", name, got, want)
		}
	}
}

func TestResolve_TrimsPaddingAndRefusesTaiVariant(t *testing.T) {
	r, err := Load(realCSV, realManifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve("基隆市  "); err != nil {
		t.Errorf("帶尾隨空白的縣市名應可解析: %v", err)
	}
	if _, err := r.Resolve("臺北市"); !errors.Is(err, ErrUnknownMarket) {
		t.Errorf("Resolve(臺北市) 應為 ErrUnknownMarket，got %v", err)
	}
}

func TestResolve_UnknownCityFailsClosed(t *testing.T) {
	r, err := Load(realCSV, realManifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve("金門縣"); !errors.Is(err, ErrUnknownMarket) {
		t.Errorf("未知縣市應回 ErrUnknownMarket，got %v", err)
	}
}

func TestLoad_DigestMismatchRejected(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "r.csv")
	mfPath := filepath.Join(dir, "r.manifest.json")
	if err := os.WriteFile(csvPath, []byte("registry_version,market_code,source_market_name\nv1,10017,基隆市\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mfPath, []byte(`{"sha256":"deadbeef","row_count":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(csvPath, mfPath); !errors.Is(err, ErrDigestMismatch) {
		t.Errorf("應回 ErrDigestMismatch，got %v", err)
	}
}
