package main

import (
	"strings"
	"testing"
)

const moduleDir = "backend/internal/modules/fx/internal/text"

func withModule(files map[string]*File) func(*Inputs) {
	return func(in *Inputs) {
		in.Modules = []ModuleInput{{Dir: moduleDir, Files: files}}
	}
}

func validModuleFiles() map[string]*File {
	return map[string]*File{
		"zh-TW": {Default: map[string]Entry{
			"appeal.unavailable.revoked": {Message: "這筆措施已撤銷", Description: "申覆不可用：措施已撤銷。"},
			"remediation.unavailable.cooldown": {
				Message:     "解除請求目前在冷卻期內，{time} 後可再提出",
				Params:      map[string]ParamSpec{"time": {Type: "string"}},
				Description: "解除請求不可用：冷卻期。time 是台北時間。",
			},
			"remediation.cooldown_days": {
				Message:     "{days, plural, other {冷卻 {days} 日}}",
				Params:      map[string]ParamSpec{"days": {Type: "integer"}},
				Description: "冷卻天數。",
			},
		}},
		"en": {},
	}
}

func TestModuleSentences(t *testing.T) {
	m := mustBuild(t, withModule(validModuleFiles()))
	out, err := m.Outputs()
	if err != nil {
		t.Fatal(err)
	}
	src := string(out[moduleDir+"/text_gen.go"])
	if err := gofmtCheck([]byte(src)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"package text",
		"func AppealUnavailableRevoked(l uitext.Locale) uitext.Message",
		"func RemediationUnavailableCooldown(l uitext.Locale, a RemediationUnavailableCooldownArgs) uitext.Message",
		"type RemediationUnavailableCooldownArgs struct {",
		"Time string",
		`"解除請求目前在冷卻期內，" + a.Time + " 後可再提出"`,
		"strconv.Itoa(a.Days)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("text_gen.go 缺 %q\n%s", want, src)
		}
	}
	// 來源檔也被正規化。
	files, err := m.SourceFiles()
	if err != nil || len(files[moduleDir+"/zh-TW.json"]) == 0 || string(files[moduleDir+"/en.json"]) != "{}\n" {
		t.Errorf("模組來源檔：%v %q", err, files[moduleDir+"/en.json"])
	}
}

func TestModuleEnglishOverridesAndPlural(t *testing.T) {
	files := validModuleFiles()
	files["en"] = &File{Default: map[string]Entry{
		"appeal.unavailable.revoked": {Message: "This measure was revoked"},
		"remediation.cooldown_days":  {Message: "{days, plural, one {{days} day} other {{days} days}}"},
	}}
	m := mustBuild(t, withModule(files))
	out, err := m.Outputs()
	if err != nil {
		t.Fatal(err)
	}
	src := string(out[moduleDir+"/text_gen.go"])
	for _, want := range []string{"case uitext.EN:", `uitext.Message{Text: "This measure was revoked", Locale: uitext.EN}`, "uitext.PluralCategory(uitext.EN, a.Days)"} {
		if !strings.Contains(src, want) {
			t.Errorf("text_gen.go 缺 %q", want)
		}
	}
	// 有 en 的句子標 en；en 缺字的句子即使請求 en，也標實際使用的 zh-TW（ADR D3）。
	if strings.Contains(src, `uitext.Message{Text: "這筆措施已撤銷", Locale: uitext.EN}`) {
		t.Error("zh-TW 的文字不能標成 en")
	}
	if !strings.Contains(src, `uitext.Message{Text: "解除請求目前在冷卻期內，" + a.Time + " 後可再提出", Locale: uitext.ZhTW}`) {
		t.Errorf("en 缺字的句子要標 zh-TW\n%s", src)
	}
}

// 模組句子可以宣告 staff 受眾：只有 zh-TW，函式沒有語系參數，Message.Locale 固定 zh-TW；staff 的 en 文字、en 的 staff 段、
// 同一句同時在 default 與 staff 都是錯誤（ADR D1：staff 受眾的文字只有 zh-TW）。
func TestModuleStaffSentences(t *testing.T) {
	staffEntry := Entry{
		Message:     "{days, plural, other {建立後 {days} 天生效}}",
		Params:      map[string]ParamSpec{"days": {Type: "integer"}},
		Description: "給員工的說明。",
	}
	withStaff := func(f func(map[string]*File)) func(*Inputs) {
		return func(in *Inputs) {
			files := validModuleFiles()
			files["zh-TW"].Staff = map[string]Entry{"termination.effect": staffEntry}
			f(files)
			withModule(files)(in)
		}
	}
	m := mustBuild(t, withStaff(func(map[string]*File) {}))
	out, err := m.Outputs()
	if err != nil {
		t.Fatal(err)
	}
	src := string(out[moduleDir+"/text_gen.go"])
	if err := gofmtCheck([]byte(src)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func TerminationEffect(a TerminationEffectArgs) uitext.Message",
		"Days int",
		`uitext.Message{Text: "建立後 " + strconv.Itoa(a.Days) + " 天生效", Locale: uitext.ZhTW}`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("text_gen.go 缺 %q\n%s", want, src)
		}
	}
	wantError(t, withStaff(func(f map[string]*File) {
		f["en"].Default = map[string]Entry{"termination.effect": {Message: "Effective in {days} days"}}
	}), "termination.effect", "staff 句子只有 zh-TW")
	wantError(t, withStaff(func(f map[string]*File) {
		f["en"].Staff = map[string]Entry{"termination.effect": {Message: "Effective in {days} days"}}
	}), "en.json", "staff 句子只有 zh-TW")
	wantError(t, withStaff(func(f map[string]*File) {
		f["zh-TW"].Default["termination.effect"] = Entry{Message: "x", Description: "x"}
	}), "termination.effect", "同時在 default 與 staff")
	wantError(t, withStaff(func(f map[string]*File) {
		e := staffEntry
		e.Description = ""
		f["zh-TW"].Staff["termination.effect"] = e
	}), "termination.effect", "缺 description")
}

// §2.2 第 9 條：key 是 lower_snake 的點分路徑，其餘同 3–6。
func TestModuleErrors(t *testing.T) {
	mod := func(f func(map[string]*File)) func(*Inputs) {
		return func(in *Inputs) {
			files := validModuleFiles()
			f(files)
			withModule(files)(in)
		}
	}
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Default["Appeal.Revoked"] = Entry{Message: "x", Description: "x"}
	}), moduleDir, "Appeal.Revoked", "lower_snake")
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Default["appeal-x"] = Entry{Message: "x", Description: "x"}
	}), "appeal-x", "lower_snake")
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Default["appeal.x"] = Entry{Message: "x"}
	}), "appeal.x", "缺 description")
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Default["appeal.x"] = Entry{Message: "第 # 筆", Description: "x"}
	}), "appeal.x", "禁止")
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Default["appeal.x"] = Entry{Message: "{c}", Params: map[string]ParamSpec{"c": {Enum: "fx.Color"}}, Description: "x"}
	}), "appeal.x", "只能是 string 或 integer")
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Default["appeal.x"] = Entry{Message: "{c, select, a {x} other {y}}", Params: map[string]ParamSpec{"c": {Type: "string"}}, Description: "x"}
	}), "appeal.x", "不能用 select")
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Default["appeal.x"] = Entry{Message: "x {y}", Description: "x"}
	}), "appeal.x", "沒有宣告的參數 y")
	wantError(t, mod(func(f map[string]*File) {
		f["en"].Default = map[string]Entry{"appeal.extra": {Message: "x"}}
	}), "appeal.extra", "多餘的 key")
	wantError(t, mod(func(f map[string]*File) {
		f["en"].Default = map[string]Entry{"appeal.unavailable.revoked": {Message: "x", Description: "x"}}
	}), "appeal.unavailable.revoked", "非來源語系只放 message")
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Variants = map[string]VariantDecl{"appeal.x": {Description: "x"}}
	}), "沒有 variants")
	wantError(t, mod(func(f map[string]*File) {
		f["zh-TW"].Staff = map[string]Entry{"appeal.unavailable.revoked": {Message: "x", Description: "x"}}
	}), "appeal.unavailable.revoked", "同時在 default 與 staff")
	wantError(t, mod(func(f map[string]*File) { delete(f, "zh-TW") }), moduleDir, "缺來源語系 zh-TW")
	wantError(t, mod(func(f map[string]*File) { f["fr"] = &File{} }), "語系 fr 不在 config.json")
	wantError(t, func(in *Inputs) {
		in.Config.Required = []string{"zh-TW", "en"}
		withModule(validModuleFiles())(in)
	}, "appeal.unavailable.revoked", "缺 required 語系的文字")
}
