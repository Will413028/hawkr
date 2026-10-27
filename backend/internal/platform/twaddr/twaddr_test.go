package twaddr

import (
	"encoding/csv"
	"os"
	"regexp"
	"testing"
)

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows[1:]
}

// 表的大小是來源的 22 個縣市、368 個鄉鎮市區；代碼唯一，鄉鎮市區代碼的前 5 碼是它的縣市代碼，每個都有郵遞區號。
func TestTableMatchesSource(t *testing.T) {
	if n := len(Towns()); n != 368 {
		t.Fatalf("towns=%d, want 368", n)
	}
	if n := len(Counties()); n != 22 {
		t.Fatalf("counties=%d, want 22", n)
	}
	if len(byCode) != len(towns) {
		t.Fatal("duplicate town codes")
	}
	countyNames := map[string]string{}
	for _, town := range towns {
		if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(town.Code) || town.Code[:5] != town.CountyCode {
			t.Fatalf("town=%+v", town)
		}
		if !regexp.MustCompile(`^[0-9]{3}$`).MatchString(town.PostalCode) {
			t.Fatalf("postal code: %+v", town)
		}
		if name, ok := countyNames[town.CountyCode]; ok && name != town.CountyName {
			t.Fatalf("county %s has two names", town.CountyCode)
		}
		countyNames[town.CountyCode] = town.CountyName
	}
	source := readCSV(t, "moi-towns.csv")
	postal := map[string]string{}
	for _, r := range readCSV(t, "postal-codes.csv") {
		postal[r[0]] = r[1]
	}
	if len(source) != len(towns) || len(postal) != len(towns) {
		t.Fatalf("source=%d postal=%d table=%d：重新執行 go generate", len(source), len(postal), len(towns))
	}
	for i, r := range source {
		want := Town{Code: r[2], Name: r[3], CountyCode: r[0], CountyName: r[1], PostalCode: postal[r[2]]}
		if got := byCode[r[2]]; got != want {
			t.Fatalf("source row %d: table=%+v source=%+v：重新執行 go generate", i, got, want)
		}
	}
}

// 下拉選單的順序：鄉鎮市區依郵遞區號（同號依代碼），縣市依其最小的郵遞區號先後出現。
func TestOrderByPostalCode(t *testing.T) {
	all := Towns()
	for i := 1; i < len(all); i++ {
		a, b := all[i-1], all[i]
		if a.PostalCode > b.PostalCode || (a.PostalCode == b.PostalCode && a.Code >= b.Code) {
			t.Fatalf("not ascending at %d: %+v then %+v", i, a, b)
		}
	}
	counties := Counties()
	if counties[0].Name != "臺北市" {
		t.Fatalf("first county = %+v", counties[0])
	}
	minPostal := map[string]string{}
	for _, town := range all {
		if _, ok := minPostal[town.CountyCode]; !ok {
			minPostal[town.CountyCode] = town.PostalCode
		}
	}
	for i := 1; i < len(counties); i++ {
		if minPostal[counties[i-1].Code] > minPostal[counties[i].Code] {
			t.Fatalf("counties out of order at %d: %+v then %+v", i, counties[i-1], counties[i])
		}
	}
}

func TestTownByCode(t *testing.T) {
	town, ok := TownByCode("63000050")
	if !ok || town.CountyName != "臺北市" || town.Name != "中正區" || town.PostalCode != "100" {
		t.Fatalf("town=%+v ok=%v", town, ok)
	}
	if _, ok := TownByCode("63000"); ok {
		t.Fatal("county code accepted as a town")
	}
}

func TestSplit(t *testing.T) {
	for in, want := range map[string][2]string{
		"臺北市中正區重慶南路一段1號":  {"63000050", "重慶南路一段1號"},
		"台北市中正區台北路一號":     {"63000050", "台北路一號"},
		"  臺北市中正區 重慶南路1號": {"63000050", "重慶南路1號"},
		"新竹科學園區新竹市力行六路8號": {"", ""},
		"中正區重慶南路一號":       {"", ""},
		"":                {"", ""},
	} {
		code, detail, ok := Split(in)
		if code != want[0] || detail != want[1] || ok != (want[0] != "") {
			t.Errorf("Split(%q) = (%q, %q, %v)，預期 %v", in, code, detail, ok, want)
		}
	}
}
