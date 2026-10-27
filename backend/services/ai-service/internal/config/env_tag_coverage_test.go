package config

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestLoad_PopulatesEveryEnvTaggedField(t *testing.T) {
	checked := walkEnvTagged(t, reflect.TypeOf(Config{}), nil)
	if checked == 0 {
		t.Fatal("一個帶 env tag 的欄位都沒走訪到——反射條件寫壞了，這支測試等於沒有")
	}
	t.Logf("走訪 %d 個帶 env tag 的欄位（含巢狀 struct）", checked)
}

func walkEnvTagged(t *testing.T, typ reflect.Type, index []int) int {
	t.Helper()
	checked := 0

	for i := range typ.NumField() {
		field := typ.Field(i)
		path := append(append([]int{}, index...), i)

		if field.Type.Kind() == reflect.Struct {
			checked += walkEnvTagged(t, field.Type, path)
			continue
		}

		tag := field.Tag.Get("env")
		if tag == "" {
			continue
		}

		name := strings.TrimSpace(strings.Split(tag, ",")[0])
		if name == "" {
			t.Fatalf("%s 的 env tag %q 解不出變數名", field.Name, tag)
		}

		sentinel := "sentinel-" + name
		wantBool := false
		// wantInt 取一個不可能是預設值的數：0 是好幾個欄位的合法預設（例如
		// PROCUREMENT_SETTLEMENT_STORE_ID 的「未設定」），拿它當哨兵就分不出
		// 「被填進來了」與「根本沒走到」。
		const wantInt = int64(424242)
		switch field.Type.Kind() {
		case reflect.Bool:
			wantBool = field.Tag.Get("default") != "true"
			sentinel = "false"
			if wantBool {
				sentinel = "true"
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			sentinel = strconv.FormatInt(wantInt, 10)
		}

		prev, had := os.LookupEnv(name)
		if err := os.Setenv(name, sentinel); err != nil {
			t.Fatalf("setenv %s: %v", name, err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() 在 %s=%q 時回 error: %v", name, sentinel, err)
		}
		fv := reflect.ValueOf(*cfg).FieldByIndex(path)

		if had {
			_ = os.Setenv(name, prev)
		} else {
			_ = os.Unsetenv(name)
		}

		hint := "  ⇒ 檢查 load.go 的 fillFromEnv 有沒有走到這個欄位（tag 寫壞、" +
			"型別 setField 不支援、或巢狀 struct 沒被遞迴進去）。"
		switch field.Type.Kind() {
		case reflect.Bool:
			if fv.Bool() != wantBool {
				t.Errorf("Config%s（env %s）沒有被 Load() 填入：got %v, want %v\n%s",
					fieldPath(typ, path, field), name, fv.Bool(), wantBool, hint)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if fv.Int() != wantInt {
				t.Errorf("Config%s（env %s）沒有被 Load() 填入：got %d, want %d\n%s",
					fieldPath(typ, path, field), name, fv.Int(), wantInt, hint)
			}
		default:
			if fv.String() != sentinel {
				t.Errorf("Config%s（env %s）沒有被 Load() 填入：got %q, want %q\n%s",
					fieldPath(typ, path, field), name, fv.String(), sentinel, hint)
			}
		}
		checked++
	}
	return checked
}

func fieldPath(typ reflect.Type, index []int, field reflect.StructField) string {
	if len(index) == 1 {
		return "." + field.Name
	}
	return "." + typ.Name() + "." + field.Name
}

// TestFillFromEnv_RejectsUnparsableBool 鎖住 loader 的 bool 解析：無法辨識的值回
// error，不沿用舊的 `!= "false"`（那會把 flase 這種 typo 靜默當成 true）。
//
// 用測試專屬的 struct 而不是 Config：`AI_USE_COORDINATOR` 隨 all-tools fallback
// 一起移除後，Config 已經沒有 bool 欄位，但 loader 的 bool 能力仍在、仍該被守住。
// 綁在某個恰好存在的 production 欄位上，會讓這道守門隨那個欄位的去留而消失。
func TestFillFromEnv_RejectsUnparsableBool(t *testing.T) {
	var target struct {
		Flag bool `env:"EVAL_LOADER_BOOL_PROBE" default:"true"`
	}
	t.Setenv("EVAL_LOADER_BOOL_PROBE", "flase")
	err := fillFromEnv(&target)
	if err == nil {
		t.Fatalf("無法辨識的布林值必須回 error，實得 Flag=%v", target.Flag)
	}
	if !strings.Contains(err.Error(), "EVAL_LOADER_BOOL_PROBE") {
		t.Errorf("error 訊息要指名是哪個環境變數，實得：%v", err)
	}
}

// TestFillFromEnv_RejectsUnparsableInt 鎖住 loader 的整數解析：無法辨識的值回 error，
// ⛔ 不得靜默退回 0。
//
// 0 對這類欄位是**有意義的哨兵**（PROCUREMENT_SETTLEMENT_STORE_ID 的 0 ＝ 未設定，
// EXTERNAL lane 因此 fail-closed）。把 typo 吞成 0，會讓「設定打錯字」長得跟「刻意不設定」
// 一模一樣，而後者是不會有人去查的正常狀態。形狀同上面那支 bool 的守門。
func TestFillFromEnv_RejectsUnparsableInt(t *testing.T) {
	var target struct {
		Value int64 `env:"EVAL_LOADER_INT_PROBE" default:"0"`
	}
	t.Setenv("EVAL_LOADER_INT_PROBE", "2636840881x")
	err := fillFromEnv(&target)
	if err == nil {
		t.Fatalf("無法辨識的整數必須回 error，實得 Value=%d", target.Value)
	}
	if !strings.Contains(err.Error(), "EVAL_LOADER_INT_PROBE") {
		t.Errorf("error 訊息要指名是哪個環境變數，實得：%v", err)
	}
}

func TestLoad_StaleExtractorProviderIgnoredWhenModelSet(t *testing.T) {
	t.Setenv("EXTRACTOR_PROVIDER", "deepseek")

	t.Setenv("EXTRACTOR_MODEL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 非預期 error: %v", err)
	}
	if cfg.StaleExtractorProvider != "deepseek" {
		t.Errorf("EXTRACTOR_MODEL 未設時應沿用舊名，實得 %q", cfg.StaleExtractorProvider)
	}

	t.Setenv("EXTRACTOR_MODEL", "deepseek-flash")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() 非預期 error: %v", err)
	}
	if cfg.StaleExtractorProvider != "" {
		t.Errorf("EXTRACTOR_MODEL 已設時舊名必須被忽略，實得 %q", cfg.StaleExtractorProvider)
	}
}
