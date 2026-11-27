package fsm

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)


var sqlEdgesNotInFSM = map[string]string{
	"FailOpenInvoiceOperationsForIssuer": "需要新的 J 代號（P14／P16 的批次 fail-open，16 個來源狀態），代號由設計書 §7.3.2 定版",
}

var (
	reQueryName = regexp.MustCompile(`^-- name: (\w+)`)
	reSetStatus = regexp.MustCompile(`SET\s+status\s*=\s*'([A-Z_]+)'`)
	reStatusIn  = regexp.MustCompile(`AND\s+(?:\w+\.)?status\s+IN\s*\(([^)]*)\)`)
	reStatusEq  = regexp.MustCompile(`AND\s+(?:\w+\.)?status\s*=\s*'([A-Z_]+)'`)
	reQuoted    = regexp.MustCompile(`'([A-Z_]+)'`)
)

type sqlEdge struct {
	query string
	from  []string
	to    string
}

func parseStatusEdges(t *testing.T, fileName, tableName string) []sqlEdge {
	t.Helper()
	path := filepath.Join("..", "..", "..", "db", "queries", fileName)
	raw, err := os.ReadFile(path) //nolint:gosec // 固定的 repo 內相對路徑
	if err != nil {
		t.Fatalf("讀不到 %s（本測試已經在守一個不存在的東西）：%v", path, err)
	}

	var out []sqlEdge
	for _, part := range strings.Split(string(raw), "\n-- name: ") {
		block := part
		if !strings.HasPrefix(block, "-- name: ") {
			block = "-- name: " + block
		}
		nm := reQueryName.FindStringSubmatch(block)
		if nm == nil {
			continue
		}
		var stmt strings.Builder
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "--") {
				continue
			}
			stmt.WriteString(line)
			stmt.WriteString("\n")
		}
		body := stmt.String()
		if !regexp.MustCompile(`(?m)^\s*UPDATE\s+` + tableName + `\b`).MatchString(body) {
			continue
		}
		to := reSetStatus.FindStringSubmatch(body)
		if to == nil {
			continue
		}
		var from []string
		if m := reStatusIn.FindStringSubmatch(body); m != nil {
			for _, v := range reQuoted.FindAllStringSubmatch(m[1], -1) {
				from = append(from, v[1])
			}
		} else if m := reStatusEq.FindStringSubmatch(body); m != nil {
			from = append(from, m[1])
		}
		sort.Strings(from)
		out = append(out, sqlEdge{query: nm[1], from: from, to: to[1]})
	}
	return out
}

func TestOperationFSMMatchesSQL(t *testing.T) {
	edges := parseStatusEdges(t, "invoice_operations.sql", "invoice_operations")
	if len(edges) < 15 {
		t.Fatalf("只解析到 %d 支寫 status 的 UPDATE，遠少於預期 —— 解析壞了", len(edges))
	}

	allowed := map[string]map[string]bool{}
	for _, tr := range OperationTransitions {
		if allowed[tr.To] == nil {
			allowed[tr.To] = map[string]bool{}
		}
		for _, f := range tr.From {
			allowed[tr.To][f] = true
		}
	}

	used := map[string]bool{}
	for _, e := range edges {
		if reason, ok := sqlEdgesNotInFSM[e.query]; ok {
			used[e.query] = true
			_ = reason
			continue
		}
		set := allowed[e.to]
		if set == nil {
			t.Errorf("SQL %s 寫 status=%q，而 fsm 裡沒有任何一條轉移的 To 是它 —— "+
				"新增了一條邊卻沒進 fsm（或 fsm 的 To 打錯）", e.query, e.to)
			continue
		}
		for _, f := range e.from {
			if !set[f] {
				t.Errorf("SQL %s 的 allowed-from 含 %q → %q，而 fsm 的對應轉移沒有這個來源。\n"+
					"  fsm 認得的來源：%v\n"+
					"  這是「SQL 放寬了而 fsm 沒跟上」——照 fsm 施工的人會漏掉這條路徑。",
					e.query, f, e.to, sortedKeys(set))
			}
		}
	}

	for q := range sqlEdgesNotInFSM {
		if !used[q] {
			t.Errorf("sqlEdgesNotInFSM 的 %q 已經不成立（該 SQL 被改名或刪掉，"+
				"或它現在對得上 fsm 了）⇒ 移除該條目", q)
		}
	}
}

// TestOperationTerminalStatesHaveNoOutgoingEdge 守「終態真的是終態」。
//
// 終態列表若與轉移表分家，就會出現「宣告是終態、卻有一條邊從它出去」——
// 那條邊會被當成不可能發生而沒有人測它。
func TestOperationTerminalStatesHaveNoOutgoingEdge(t *testing.T) {
	terminal := map[string]bool{}
	for _, s := range OperationTerminalStates {
		terminal[s] = true
	}
	if len(terminal) == 0 {
		t.Fatal("終態列表是空的 —— 來源壞了，而壞掉的症狀是本測試恆綠")
	}
	for _, tr := range OperationTransitions {
		for _, f := range tr.From {
			if terminal[f] {
				t.Errorf("%s 從終態 %q 出去（→ %q）—— 終態列表與轉移表分家了",
					tr.Code, f, tr.To)
			}
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var sqlDocEdgesNotInFSM = map[string]string{}

// TestDocumentFSMMatchesSQL 是 M1 側的對帳，與 TestOperationFSMMatchesSQL 同一把尺。
//
// 🔴 為什麼補這一支：M2 有對帳而 M1 沒有，於是
// `fsm.DocumentTransitions`／`CheckDocument` 在非測試碼**零 import** ——
// 它就是本檔檔頭批評的那個「沒有人讀的 SSOT」，只是換一張表。
// 今天（2026-09-05）實測零漂移 ⇒ 這是**gate 缺口**不是資料缺口；補上它是為了讓
// 下一次「SQL 放寬了而 fsm 沒跟上」有人出聲。
//
// 觀測窗與 M2 那一支完全相同（見檔頭）：看得到「SQL 放寬而 fsm 沒跟上」，
// 看不到「fsm 有而 SQL 沒有」與跨表守衛。
//
// ⚠️ **刻意沒有配對的 TestDocumentTerminalStatesHaveNoOutgoingEdge**：
// M1 的終態一致性已經由 `document_test.go:TestDocumentTerminalStates` 的內層迴圈守著，
// 而且它帶了一個**有明文理由的例外**（VOIDED —— §7.3.1 的終態清單列了它，
// 同一節的轉換表卻有 `VOIDED → SUPERSEDED`，mermaid 也畫了；dev 實測該邊發生過 12 次）。
// 在這裡再寫一份不帶例外的同名檢查，只會得到一個與既有測試互相矛盾的第二份判準
// ——那比沒有更糟（`.claude/rules/06-no-small-tails.md` §3.10）。
func TestDocumentFSMMatchesSQL(t *testing.T) {
	edges := parseStatusEdges(t, "invoice_documents.sql", "invoice_documents")
	if len(edges) < 10 {
		t.Fatalf("只解析到 %d 支寫 invoice_documents.status 的 UPDATE，遠少於預期 —— 解析壞了", len(edges))
	}

	allowed := map[string]map[string]bool{}
	for _, tr := range DocumentTransitions {
		if allowed[tr.To] == nil {
			allowed[tr.To] = map[string]bool{}
		}
		for _, f := range tr.From {
			allowed[tr.To][f] = true
		}
	}

	used := map[string]bool{}
	for _, e := range edges {
		if _, ok := sqlDocEdgesNotInFSM[e.query]; ok {
			used[e.query] = true
			continue
		}
		set := allowed[e.to]
		if set == nil {
			t.Errorf("SQL %s 寫 status=%q，而 fsm 的 M1 表裡沒有任何一條轉移的 To 是它 —— "+
				"新增了一條邊卻沒進 DocumentTransitions（或 To 打錯）", e.query, e.to)
			continue
		}
		for _, f := range e.from {
			if !set[f] {
				t.Errorf("SQL %s 的 allowed-from 含 %q → %q，而 M1 表的對應轉移沒有這個來源。\n"+
					"  fsm 認得的來源：%v\n"+
					"  這是「SQL 放寬了而 fsm 沒跟上」——照 fsm 施工的人會漏掉這條路徑。",
					e.query, f, e.to, sortedKeys(set))
			}
		}
	}

	for q := range sqlDocEdgesNotInFSM {
		if !used[q] {
			t.Errorf("sqlDocEdgesNotInFSM 的 %q 已經不成立 ⇒ 移除該條目", q)
		}
	}
}
