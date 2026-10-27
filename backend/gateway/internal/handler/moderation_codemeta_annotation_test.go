package handler

import (
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// Swagger 註解是 Apidog 的契約來源（10 檔 §16），而檢舉端點的註解裡**手抄**了三份
// 值域：狀態篩選（PENDING／RESOLVED_*／DISMISSED／APPEALED）、裁決動作
// （HIDE_TARGET／WARN_USER／BAN_USER／DISMISS）與 request struct 的行內註解。
//
// 手抄不掉是結構性的：註解是字串，引用不了 Go 常數——`Enums(...)` 與 `@Description`
// 都由 swag 逐字讀取。所以收斂不了的東西只能盯著。抄錯的後果不是編譯錯也不是測試
// 紅，是前端照 Apidog 送一個後端不認得的值，然後在執行期拿到 400。
//
// 本 gate 的判準：moderation.go 的**所有註解**裡出現的 UPPER_SNAKE token，必須落在
// CodeMeta 的檢舉四張碼表的值域內（或在下面那份逐項具名的非碼值豁免清單裡）。
// deny-by-default：新造一個碼表沒有的值 → 紅；新造一個非碼值縮寫 → 也紅，要嘛登記
// 要嘛改寫。前者是本 gate 要擋的，後者是必付的代價——放行未知 token 等於放行前者。
func TestModerationSwaggerAnnotationsMatchCodeMeta(t *testing.T) {
	// 非碼值縮寫。逐項具名（不是 pattern），新增一個就要在這裡寫下它是什麼——
	// 這正是「未知 token 一律紅」換來的東西。
	exempt := map[string]struct{}{
		"JSON":  {}, // 序列化格式
		"JSONB": {}, // PostgreSQL 欄位型別
		"URL":   {},
		"TSID":  {},
		"REST":  {},
		"DTO":   {},
		// 下面那支完整性 gate 的具名豁免標記本身。它出現在註解裡就是為了被讀到，
		// 不是碼值。
		"CODEMETA_SUBSET_OK": {},
	}

	known := knownModerationCodeValues()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "moderation.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse moderation.go: %v", err)
	}

	// 行內註解也要掃：AdminResolveReportRequest.Action 的值域就寫在欄位右邊，而它與
	// @Description 那一份是各自獨立的手抄——只掃 @ 開頭的 Swagger 行會漏掉它。
	tokenRe := regexp.MustCompile(`[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*(?:_\*)?`)
	for _, group := range f.Comments {
		for _, c := range group.List {
			text := strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*")
			for _, tok := range tokenRe.FindAllString(text, -1) {
				line := fset.Position(c.Pos()).Line
				if strings.HasSuffix(tok, "_*") {
					// `RESOLVED_*` 是刻意的前綴寫法（四個終態裡有三個同前綴）。前綴
					// 至少要對得上一個真實碼值，否則它就是一個指向空集合的萬用字元。
					if !hasPrefixInCodeMeta(known, strings.TrimSuffix(tok, "*")) {
						t.Errorf("moderation.go:%d 註解的萬用值域 %q 在 CodeMeta 檢舉碼表中沒有任何對應碼值", line, tok)
					}
					continue
				}
				if len(tok) < 3 {
					continue
				}
				if _, ok := exempt[tok]; ok {
					continue
				}
				if _, ok := known[tok]; !ok {
					t.Errorf("moderation.go:%d 註解列出的 %q 不是 CodeMeta 檢舉碼表的合法值（Apidog 會照它產契約，前端送出後端不認得）", line, tok)
				}
			}
		}
	}
}

// knownModerationCodeValues 是檢舉相關四張碼表的值域聯集。四張一起收：同一份註解裡
// 同時出現狀態、裁決動作與對象類型，逐張分開比對只會讓判定變成「這個 token 屬於哪
// 張表」這個註解本身沒有回答的問題。
func knownModerationCodeValues() map[string]struct{} {
	known := map[string]struct{}{}
	for _, v := range codemeta.AllReportStatuss {
		known[v.Value] = struct{}{}
	}
	for _, v := range codemeta.AllReportResolveActions {
		known[v.Value] = struct{}{}
	}
	for _, v := range codemeta.AllReportTargetTypes {
		known[v.Value] = struct{}{}
	}
	for _, v := range codemeta.AllReportReasonCodes {
		known[v.Value] = struct{}{}
	}
	return known
}

func hasPrefixInCodeMeta(known map[string]struct{}, prefix string) bool {
	for v := range known {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

// 上面那支只擋「列出了碼表沒有的值」。它擋不住相反的一半——**列漏**：每個列出的值
// 都合法，集合卻少一個。post.go 的 ReportPostRequest.ReasonCode 就是這樣少掉 FRAUD
// 的：Apidog 照它產契約 ⇒ 前端的檢舉原因選單少一個「詐騙」，而後端一直收得下。
// 症狀不是錯誤而是「那個選項不存在」，沒有任何東西會為此轉紅。
//
// 判準：一段註解裡若出現某張碼表 ≥3 個值，就當它是在「列舉該值域」，必須列全。
// ≥3 是為了不誤傷「舉例兩個」的散文；`X_*` 前綴寫法會先展開成它涵蓋的真實碼值。
// 真的需要列子集時（例如「本欄位只接受其中兩個」），在同一段註解寫下
// CODEMETA_SUBSET_OK 具名豁免——那句話本身就是在要求作者說明為什麼。
func TestReportAnnotationsEnumerateFullDomain(t *testing.T) {
	domains := map[string][]string{}
	for _, v := range codemeta.AllReportStatuss {
		domains["ReportStatus"] = append(domains["ReportStatus"], v.Value)
	}
	for _, v := range codemeta.AllReportResolveActions {
		domains["ReportResolveAction"] = append(domains["ReportResolveAction"], v.Value)
	}
	for _, v := range codemeta.AllReportTargetTypes {
		domains["ReportTargetType"] = append(domains["ReportTargetType"], v.Value)
	}
	for _, v := range codemeta.AllReportReasonCodes {
		domains["ReportReasonCode"] = append(domains["ReportReasonCode"], v.Value)
	}

	tokenRe := regexp.MustCompile(`[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*(?:_\*)?`)
	for _, file := range []string{"moderation.go", "post.go"} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, group := range f.Comments {
			text := group.Text()
			if strings.Contains(text, "CODEMETA_SUBSET_OK") {
				continue
			}
			seen := map[string]struct{}{}
			for _, tok := range tokenRe.FindAllString(text, -1) {
				if strings.HasSuffix(tok, "_*") {
					prefix := strings.TrimSuffix(tok, "*")
					for _, vals := range domains {
						for _, v := range vals {
							if strings.HasPrefix(v, prefix) {
								seen[v] = struct{}{}
							}
						}
					}
					continue
				}
				seen[tok] = struct{}{}
			}
			for name, vals := range domains {
				var hit, missing []string
				for _, v := range vals {
					if _, ok := seen[v]; ok {
						hit = append(hit, v)
					} else {
						missing = append(missing, v)
					}
				}
				if len(hit) >= 3 && len(missing) > 0 {
					t.Errorf("%s:%d 這段註解列舉了 %s 的 %d/%d 個值，漏掉 %v"+
						"（Apidog 照它產契約 ⇒ 前端會少掉這些選項；真要列子集請在同段註解標 CODEMETA_SUBSET_OK 並說明理由）",
						file, fset.Position(group.Pos()).Line, name, len(hit), len(vals), missing)
				}
			}
		}
	}
}
