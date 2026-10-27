package main

import (
	"strings"
	"testing"
)

func TestBuildValid(t *testing.T) {
	m := mustBuild(t, nil)
	ids := map[string]*Func{}
	for _, f := range m.Funcs {
		ids[f.ID] = f
	}
	for _, id := range []string{"fx.Color", "fx.Color.label", "fx.Mode", "fx.Box.Size", "fx.Box.title", "fx.Note.count", "admin.Role", "admin.fx.Level", "common.unknown", "common.error"} {
		if ids[id] == nil {
			t.Errorf("缺函式 %s", id)
		}
	}
	if !ids["fx.Mode"].HasStaff || ids["fx.Color"].HasStaff {
		t.Error("只有 Mode 有 staff")
	}
	if got := ids["fx.Mode"].Params; len(got) != 1 || got[0].Name != "color" || got[0].Kind != ParamEnum {
		t.Errorf("Mode 的參數：%+v", got)
	}
	if len(m.Coverage) != 1 || !strings.Contains(m.Coverage[0], "en：1/") {
		t.Errorf("非 required 語系只回報覆蓋率：%v", m.Coverage)
	}
}

// §2.2 第 1 條：key 文法與解析。
func TestKeyErrors(t *testing.T) {
	add := func(key string, e Entry) func(*Inputs) {
		return func(in *Inputs) { in.Catalog["zh-TW"].Default[key] = e }
	}
	wantError(t, add("fx.Color.COLOR_PURPLE", Entry{Message: "紫"}), "zh-TW.default fx.Color.COLOR_PURPLE", "沒有值")
	wantError(t, add("fx.Missing.COLOR_RED", Entry{Message: "x"}), "fx.Missing.COLOR_RED", "沒有型別")
	wantError(t, add("fx.Color.COLOR_UNSPECIFIED", Entry{Message: "未指定"}), "fx.Color.COLOR_UNSPECIFIED", "值號 0")
	wantError(t, add("fx.Color.Label.COLOR_RED", Entry{Message: "x"}), "要是 lower_snake")
	wantError(t, add("common.error.CANCELED", Entry{Message: "取消", Description: "x"}), "common.error.CANCELED", "不顯示文字")
	wantError(t, add("nope.Color.COLOR_RED", Entry{Message: "x"}), "開頭不是任何 package")
}

// §2.2 第 2 條：伺服器擁有的 enum 不能進目錄。
func TestServerOwnedEnumsAreRejected(t *testing.T) {
	wantError(t, func(in *Inputs) {
		in.Catalog["zh-TW"].Default["common.ErrorReason.BOOM"] = Entry{Message: "爆炸"}
	}, "common.ErrorReason.BOOM", "由伺服器擁有")
	wantError(t, func(in *Inputs) {
		in.Catalog["zh-TW"].Default["fx.GadgetUnavailableReason.GADGET_UNAVAILABLE_REASON_BROKEN"] = Entry{Message: "壞了"}
	}, "GADGET_UNAVAILABLE_REASON_BROKEN", "由伺服器擁有")
	wantError(t, func(in *Inputs) {
		in.Catalog["zh-TW"].Variants["common.ErrorReason.x"] = VariantDecl{Description: "x"}
	}, "variants common.ErrorReason.x", "由伺服器擁有")
}

// §2.2 第 3 條：完整性。
func TestCompleteness(t *testing.T) {
	wantError(t, func(in *Inputs) { delete(in.Catalog["zh-TW"].Default, "fx.Color.COLOR_BLUE") },
		"fx.Color.COLOR_BLUE", "缺 [default zh-TW]")
	wantError(t, func(in *Inputs) { delete(in.Catalog["zh-TW"].Default, "fx.Color.label.COLOR_BLUE") },
		"fx.Color.label.COLOR_BLUE", "缺 [default zh-TW]")
	// staff：一個 enum 只要有一個 staff key，每個值都要有。
	wantError(t, func(in *Inputs) { delete(in.Catalog["zh-TW"].Staff, "fx.Mode.MODE_B") },
		"fx.Mode.MODE_B", "缺 [staff zh-TW]")
	// required 的語系要完整；非 required 只回報覆蓋率。
	wantError(t, func(in *Inputs) { in.Config.Required = []string{"zh-TW", "en"} },
		"fx.Color.COLOR_BLUE", "缺 [default en]")
	// 刪掉 enum 的值之外的任何一個非零值都會被抓到：用 Size 的巢狀 enum 驗證。
	wantError(t, func(in *Inputs) { delete(in.Catalog["zh-TW"].Default, "fx.Box.Size.SIZE_LARGE") },
		"fx.Box.Size.SIZE_LARGE", "缺 [default zh-TW]")
	// 句子與 common。
	wantError(t, func(in *Inputs) { delete(in.Catalog["zh-TW"].Default, "common.unknown") }, "common.unknown", "缺 zh-TW")
	wantError(t, func(in *Inputs) { delete(in.Catalog["zh-TW"].Default, "common.error.NOT_FOUND") },
		"common.error.NOT_FOUND", "缺 zh-TW")
}

// §2.2 第 4 條：沒有多餘的 key。
func TestExtraKeys(t *testing.T) {
	wantError(t, func(in *Inputs) {
		in.Catalog["en"].Default["fx.Color.COLOR_BLUE"] = Entry{Message: "Blue"}
		delete(in.Catalog["zh-TW"].Default, "fx.Color.COLOR_BLUE")
	},
		"en.default fx.Color.COLOR_BLUE", "多餘的 key")
	wantError(t, func(in *Inputs) { in.Catalog["en"].Default["fx.Color.COLOR_PINK"] = Entry{Message: "Pink"} },
		"fx.Color.COLOR_PINK", "沒有值")
	wantError(t, func(in *Inputs) {
		in.Catalog["en"].Staff = map[string]Entry{"fx.Mode.MODE_A": {Message: "x"}}
	}, "en.json：staff 只有 zh-TW")
	wantError(t, func(in *Inputs) {
		in.Catalog["en"].Variants = map[string]VariantDecl{"fx.Color.label": {Description: "x"}}
	}, "en.json：variants 只在來源語系")
	// omit 列的值仍有 default 文字。
	wantError(t, func(in *Inputs) {
		in.Catalog["zh-TW"].Default["fx.Color.label.COLOR_GREEN"] = Entry{Message: "綠色"}
	}, "fx.Color.label.COLOR_GREEN", "omit.default 列了 COLOR_GREEN")
	// omit 列的值不存在、未宣告的變體、宣告了卻沒有 key 的變體。
	wantError(t, func(in *Inputs) {
		d := in.Catalog["zh-TW"].Variants["fx.Color.label"]
		d.Omit["default"] = append(d.Omit["default"], "COLOR_PINK")
		in.Catalog["zh-TW"].Variants["fx.Color.label"] = d
	}, "omit.default 的 \"COLOR_PINK\" 不是列舉")
	wantError(t, func(in *Inputs) { delete(in.Catalog["zh-TW"].Variants, "fx.Color.label") },
		"fx.Color.label.COLOR_RED", "沒有在 variants 宣告")
	wantError(t, func(in *Inputs) {
		in.Catalog["zh-TW"].Variants["fx.Color.orphan"] = VariantDecl{Description: "x"}
	}, "variants fx.Color.orphan", "沒有任何 key")
	wantError(t, func(in *Inputs) {
		d := in.Catalog["zh-TW"].Variants["fx.Color.label"]
		d.Omit["staff"] = []string{"COLOR_GREEN"}
		in.Catalog["zh-TW"].Variants["fx.Color.label"] = d
	}, "omit.staff 沒有意義")
	// 非來源語系只放 message。
	wantError(t, func(in *Inputs) {
		in.Catalog["en"].Default["fx.Color.COLOR_RED"] = Entry{Message: "Red", Description: "x"}
	},
		"fx.Color.COLOR_RED", "非來源語系只放 message")
}

// §2.2 第 5 條：ICU。
func TestICURules(t *testing.T) {
	set := func(key, msg string, params map[string]ParamSpec) func(*Inputs) {
		return func(in *Inputs) {
			e := in.Catalog["zh-TW"].Default[key]
			e.Message = msg
			if params != nil {
				e.Params = params
			}
			in.Catalog["zh-TW"].Default[key] = e
		}
	}
	wantError(t, set("fx.Color.COLOR_RED", "第 # 色", nil), "fx.Color.COLOR_RED", "禁止")
	wantError(t, set("fx.Color.COLOR_RED", "<b>紅</b>", nil), "fx.Color.COLOR_RED", "禁止")
	wantError(t, set("fx.Color.COLOR_RED", "'{x}", nil), "fx.Color.COLOR_RED", "apostrophe")
	wantError(t, set("fx.Mode.MODE_A", "{color, select, COLOR_PURPLE {紫} other {x}}", nil), "fx.Mode.MODE_A", "COLOR_PURPLE", "不是列舉 fx.Color 的值")
	wantError(t, set("fx.Mode.MODE_A", "{color, select, COLOR_UNSPECIFIED {x} other {y}}", nil), "COLOR_UNSPECIFIED")
	wantError(t, set("fx.Mode.MODE_A", "{color, select, COLOR_RED {x}}", nil), "fx.Mode.MODE_A", "缺少 other")
	wantError(t, set("fx.Mode.MODE_A", "{color}", nil), "enum 參數 color 只能用在 select")
	wantError(t, set("fx.Mode.MODE_A", "{color, select, COLOR_RED {x} other {y}} {extra}", nil), "沒有宣告的參數 extra")
	wantError(t, set("fx.Mode.MODE_A", "模式甲", nil), "宣告了參數 color 但訊息沒有用到")
	wantError(t, set("fx.Note.count", "{n, plural, one {1 筆} other {{n} 筆}}", nil), "plural 分支 \"one\" 不屬於 zh-TW")
	wantError(t, set("fx.Note.count", "{n, select, a {x} other {y}}", nil), "n", "integer 參數 n 不能用在 select")
	wantError(t, set("fx.Box.title", "盒子：{color, plural, other {x}}", nil), "名詞參數 color 只能寫成")
	wantError(t, set("fx.Box.title", "盒子", nil), "宣告了參數 color")
	// 語系之間的參數集合必須相同。
	wantError(t, func(in *Inputs) { in.Catalog["en"].Default["fx.Box.title"] = Entry{Message: "Box"} }, "en.default fx.Box.title", "宣告了參數 color 但訊息沒有用到")
	wantError(t, func(in *Inputs) { in.Catalog["en"].Default["fx.Box.title"] = Entry{Message: "Box {color} {other}"} }, "沒有宣告的參數 other")
	// en 的 plural 分支合乎 CLDR 就通過。
	mustBuild(t, func(in *Inputs) {
		in.Catalog["en"].Default["fx.Note.count"] = Entry{Message: "{n, plural, one {{n} item} other {{n} items}}"}
	})
	// 參數宣告本身。
	wantError(t, set("fx.Box.title", "盒子：{color}", map[string]ParamSpec{"color": {Noun: "fx.Tone"}}), "沒有名詞在目錄裡")
	wantError(t, set("fx.Box.title", "盒子：{color}", map[string]ParamSpec{"color": {Noun: "fx.Missing"}}), "沒有型別 Missing")
	wantError(t, set("fx.Box.title", "盒子：{color}", map[string]ParamSpec{"color": {Noun: "fx.Color", Type: "string"}}), "剛好指定")
	wantError(t, set("fx.Box.title", "盒子：{v}", map[string]ParamSpec{"v": {Type: "string"}}), "不合法")
	wantError(t, set("fx.Box.title", "盒子：{x}", map[string]ParamSpec{"x": {Type: "float"}}), "只能是 string 或 integer")
}

// §2.2 第 6 條：說明。
func TestDescriptions(t *testing.T) {
	wantError(t, func(in *Inputs) {
		e := in.Catalog["zh-TW"].Default["fx.Box.title"]
		e.Description = ""
		in.Catalog["zh-TW"].Default["fx.Box.title"] = e
	}, "fx.Box.title", "沒有說明")
	wantError(t, func(in *Inputs) {
		e := in.Catalog["zh-TW"].Default["common.unknown"]
		e.Description = " "
		in.Catalog["zh-TW"].Default["common.unknown"] = e
	}, "common.unknown", "沒有說明")
	wantError(t, func(in *Inputs) {
		d := in.Catalog["zh-TW"].Variants["fx.Color.label"]
		d.Description = ""
		in.Catalog["zh-TW"].Variants["fx.Color.label"] = d
	}, "variants fx.Color.label", "缺 description")
	// 值與列舉都沒有註解、也沒有 descriptionOverride。
	wantError(t, func(in *Inputs) {
		e := in.Catalog["zh-TW"].Default["fx.Box.Size.SIZE_SMALL"]
		e.DescriptionOverride = ""
		in.Catalog["zh-TW"].Default["fx.Box.Size.SIZE_SMALL"] = e
	}, "fx.Box.Size.SIZE_SMALL", "descriptionOverride")
}

func TestDescriptionSync(t *testing.T) {
	m := mustBuild(t, func(in *Inputs) {
		e := in.Catalog["zh-TW"].Default["fx.Mode.MODE_A"]
		e.DescriptionOverride = "覆寫的說明。"
		in.Catalog["zh-TW"].Default["fx.Mode.MODE_A"] = e
	})
	zh := m.Catalog["zh-TW"]
	want := map[string]string{
		"fx.Color.COLOR_RED":            "紅色。\n第二行。",
		"fx.Color.COLOR_BLUE":           "Color 是顏色。（值 COLOR_BLUE）",
		"fx.Mode.MODE_A":                "覆寫的說明。\ncolor 是目前的顏色。",
		"fx.Box.Size.SIZE_SMALL":        "盒子的尺寸：小。",
		"fx.Color.label.COLOR_RED":      "顏色的標籤。（值 COLOR_RED）",
		"fx.Box.title":                  "盒子的標題。",
		"common.error.INVALID_ARGUMENT": "預設錯誤文字。",
	}
	for k, w := range want {
		if got := zh.Default[k].Description; got != w {
			t.Errorf("%s 的 description = %q，want %q", k, got, w)
		}
	}
	if got := zh.Staff["fx.Mode.MODE_B"].Description; got != "Mode 是模式。（值 MODE_B）\n"+staffNote {
		t.Errorf("staff 的 description = %q", got)
	}
}

// §2.2 第 7 條：common。
func TestCommonKeys(t *testing.T) {
	wantError(t, func(in *Inputs) {
		in.Config.Required = []string{"zh-TW", "en"}
		delete(in.Catalog["en"].Default, "common.unknown")
	},
		"common.unknown", "缺 en")
	// Connect 的 code 剛好是 15 個（16 個 code 扣掉 CANCELED）。
	if len(connectCodes) != 15 {
		t.Errorf("connectCodes = %d", len(connectCodes))
	}
	for _, c := range connectCodes {
		if c == "CANCELED" {
			t.Error("CANCELED 不在目錄")
		}
	}
}

// §2.2 第 8 條：D-L2 形狀守門（L′-2 起開啟）。
func TestUnavailableShapeGuard(t *testing.T) {
	bad := func(fds *descriptorSet) {
		fds.File[0].MessageType = append(fds.File[0].MessageType, &messageProto{
			Name:  ptr("Legacy"),
			Field: []*fieldProto{field("unavailable_reasons", 1, typeEnum, ".hawkr.fx.v1alpha1.GadgetUnavailableReason", true)},
		})
	}
	in := validInputs()
	in.Image = fixtureImage(bad)
	if _, err := Build(in); err != nil {
		t.Errorf("關閉時不擋：%v", err)
	}
	in = validInputs()
	in.Image = fixtureImage(bad)
	in.EnforceUnavailableShape = true
	_, err := Build(in)
	if err == nil || !strings.Contains(err.Error(), "Legacy.unavailable_reasons") {
		t.Errorf("開啟時 repeated XxxUnavailableReason 要失敗：%v", err)
	}
	in = validInputs()
	in.EnforceUnavailableShape = true
	if _, err := Build(in); err != nil {
		t.Errorf("reason 欄位是合法形狀：%v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	cases := []struct {
		mutate func(*Config)
		want   string
	}{
		{func(c *Config) { c.SourceLocale = "" }, "都要有值"},
		{func(c *Config) { c.SourceLocale = "en"; c.Required = []string{"zh-TW", "en"} }, "sourceLocale 要是 zh-TW"},
		{func(c *Config) { c.Required = []string{"en"} }, "要包含 sourceLocale"},
		{func(c *Config) { c.Required = []string{"zh-TW", "fr"} }, "不在 locales"},
		{func(c *Config) { c.Locales = []string{"zh-TW", "fr"}; c.Required = []string{"zh-TW"} }, "不認得的語系"},
	}
	for _, c := range cases {
		wantError(t, func(in *Inputs) { c.mutate(in.Config) }, c.want)
	}
}

// 來源檔正規化：key 排序、description 同步，寫出再讀入再 Build，結果不變。
func TestCanonicalFormIsStable(t *testing.T) {
	m := mustBuild(t, nil)
	first, err := m.SourceFiles()
	if err != nil {
		t.Fatal(err)
	}
	in := validInputs()
	in.Catalog = map[string]*File{}
	for loc := range m.Catalog {
		f, err := ParseFile(first[catalogDir+"/"+loc+".json"])
		if err != nil {
			t.Fatal(err)
		}
		in.Catalog[loc] = f
	}
	m2, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m2.SourceFiles()
	if err != nil {
		t.Fatal(err)
	}
	for p, b := range first {
		if string(second[p]) != string(b) {
			t.Errorf("%s 不是不動點", p)
		}
	}
	if !strings.HasSuffix(string(first[catalogDir+"/en.json"]), "\n") {
		t.Error("來源檔要以換行結尾")
	}
}

func TestParseFileIsStrict(t *testing.T) {
	if _, err := ParseFile([]byte(`{"default":{"a":{"message":"x","bogus":1}}}`)); err == nil {
		t.Error("未知欄位要失敗")
	}
	if _, err := ParseFile([]byte(`{} {}`)); err == nil {
		t.Error("JSON 之後多出內容要失敗")
	}
	f, err := ParseFile([]byte(`{}`))
	if err != nil || len(f.Default) != 0 {
		t.Errorf("{} 是合法的空檔：%v", err)
	}
}
