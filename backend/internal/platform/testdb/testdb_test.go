package testdb_test

import (
	"testing"
	"testing/fstest"

	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

func module(name string, files map[string]string) migrate.Module {
	fsys := fstest.MapFS{}
	for n, body := range files {
		fsys[n] = &fstest.MapFile{Data: []byte(body)}
	}
	return migrate.Module{Name: name, Dir: fsys}
}

func templateName(t *testing.T, steps ...testdb.Step) string {
	t.Helper()
	name, err := testdb.TemplateName(steps...)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

// template 的名稱只由內容決定：內容一樣就共用，長度相同只差一個 byte、檔名、模組名、順序不同就是另一個 template。
func TestTemplateNameFollowsTheContent(t *testing.T) {
	base := map[string]string{"001_init.sql": "CREATE SCHEMA shop;", "atlas.sum": "h1:x"}
	want := templateName(t, testdb.Migrations(module("shop", base)))
	if got := templateName(t, testdb.Migrations(module("shop", map[string]string{"atlas.sum": "h1:x", "001_init.sql": "CREATE SCHEMA shop;"}))); got != want {
		t.Fatalf("同樣的內容得到不同的 template：%s／%s", got, want)
	}
	if templateName(t) != "" {
		t.Fatal("沒有 step 應該是空的 database，不用 template")
	}
	other := module("other", map[string]string{"001_init.sql": "CREATE SCHEMA other;"})
	variants := map[string][]testdb.Step{
		"檔案內容":      {testdb.Migrations(module("shop", map[string]string{"001_init.sql": "CREATE SCHEMA shoq;", "atlas.sum": "h1:x"}))},
		"檔名":        {testdb.Migrations(module("shop", map[string]string{"001_init_.sql": "CREATE SCHEMA shop;", "atlas.sum": "h1:x"}))},
		"多一個檔案":     {testdb.Migrations(module("shop", map[string]string{"001_init.sql": "CREATE SCHEMA shop;", "002_more.sql": "", "atlas.sum": "h1:x"}))},
		"atlas.sum": {testdb.Migrations(module("shop", map[string]string{"001_init.sql": "CREATE SCHEMA shop;", "atlas.sum": "h1:y"}))},
		"模組名":       {testdb.Migrations(module("shop2", base))},
		"多一個模組":     {testdb.Migrations(module("shop", base), other)},
		"模組順序":      {testdb.Migrations(other, module("shop", base))},
		"加上 River":  {testdb.Jobs(), testdb.Migrations(module("shop", base))},
		"step 順序":   {testdb.Migrations(module("shop", base)), testdb.Jobs()},
	}
	seen := map[string]string{want: "原本"}
	for name, steps := range variants {
		got := templateName(t, steps...)
		if prev, dup := seen[got]; dup {
			t.Errorf("%s 與 %s 得到同一個 template %s", name, prev, got)
		}
		seen[got] = name
	}
}
