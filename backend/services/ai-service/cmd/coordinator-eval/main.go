package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
	"gopkg.in/yaml.v3"

	agentpkg "github.com/Will413028/hawkr/ai-service/internal/agent"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
)

type Case struct {
	Q      string   `yaml:"q"`
	Expect []string `yaml:"expect"`
}

type Corpus struct {
	SellerSingle []Case `yaml:"seller_single"`
	SellerMulti  []Case `yaml:"seller_multi"`
	BuyerSingle  []Case `yaml:"buyer_single"`
	BuyerMulti   []Case `yaml:"buyer_multi"`
}

type Result struct {
	Group    string   `json:"group"`
	Path     string   `json:"path"`
	Query    string   `json:"query"`
	Expect   []string `json:"expect"`
	Actual   []string `json:"actual"`
	Hit      bool     `json:"hit"`
	Complete bool     `json:"complete"`
	Extra    []string `json:"extra,omitempty"`
	Millis   int64    `json:"millis"`
	Err      string   `json:"err,omitempty"`
}

func main() {
	corpusPath := flag.String("corpus", "services/ai-service/cmd/coordinator-eval/testdata/corpus.yaml", "corpus yaml")
	limit := flag.Int("limit", 0, "每組最多跑幾條（0＝全部）")
	roleFilter := flag.String("role", "both", "seller|buyer|both")
	out := flag.String("out", "", "把逐條結果寫成 JSON")
	dryRun := flag.Bool("dry-run", false, "只印出會跑什麼，不呼叫 LLM")
	repeat := flag.Int("repeat", 1, "每條跑幾次。**多步驟委派是隨機的**（實測同一條 query 同一份指令 1/3 通過），單次結果不足以下結論")
	flag.Parse()

	raw, err := os.ReadFile(*corpusPath)
	if err != nil {
		fail("讀 corpus 失敗: %v", err)
	}
	var corpus Corpus
	if err := yaml.Unmarshal(raw, &corpus); err != nil {
		fail("解析 corpus 失敗: %v", err)
	}

	type group struct {
		name  string
		role  string
		multi bool
		cases []Case
	}
	groups := []group{
		{"seller_single", "seller", false, corpus.SellerSingle},
		{"seller_multi", "seller", true, corpus.SellerMulti},
		{"buyer_single", "buyer", false, corpus.BuyerSingle},
		{"buyer_multi", "buyer", true, corpus.BuyerMulti},
	}

	planned := 0
	for i := range groups {
		if *roleFilter != "both" && groups[i].role != *roleFilter {
			groups[i].cases = nil
			continue
		}
		if *limit > 0 && len(groups[i].cases) > *limit {
			groups[i].cases = groups[i].cases[:*limit]
		}
		planned += len(groups[i].cases)
	}
	fmt.Printf("將跑 %d 條 × %d 次 = %d 次對話（只有 coordinator 一條路徑）\n", planned, *repeat, planned**repeat)
	if *dryRun {
		for _, g := range groups {
			for _, c := range g.cases {
				fmt.Printf("  [%s] %s → %v\n", g.name, c.Q, c.Expect)
			}
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fail("讀設定失敗: %v", err)
	}
	if cfg.GeminiAPIKey == "" {
		fail("GEMINI_API_KEY 未設定——這支工具必須真的呼叫 LLM，沒有 key 就不要假裝跑過")
	}
	log := zap.NewNop()
	assistant := llmprovider.NewAssistant(cfg, log)

	deps := &agentpkg.ToolDeps{}
	agents, err := agentpkg.NewAgents(assistant, deps)
	if err != nil {
		fail("建 agents 失敗: %v", err)
	}
	sessionSvc := session.InMemoryService()
	runners, err := agentpkg.NewRunners(agents, sessionSvc)
	if err != nil {
		fail("建 runners 失敗: %v", err)
	}
	owners := agentpkg.SpecialistToolOwners(deps)

	var results []Result
	for _, g := range groups {
		for _, c := range g.cases {
			for _, path := range []string{"coordinator"} {
				r := runnerFor(runners, g.role, path)
				if r == nil {
					continue
				}
				passes := 0
				for i := 0; i < *repeat; i++ {
					res := runOne(context.Background(), r, sessionSvc, owners, g.name, path, g.multi, c)
					results = append(results, res)
					if res.Hit {
						passes++
					}
					if *repeat == 1 {
						mark := "✗"
						if res.Hit {
							mark = "✓"
						}
						fmt.Printf("%s [%-13s %-11s] %-28s expect=%v actual=%v %dms %s\n",
							mark, g.name, path, truncate(c.Q, 28), c.Expect, res.Actual, res.Millis, res.Err)
					}
				}
				if *repeat > 1 {
					fmt.Printf("%d/%d [%-13s] %-28s expect=%v\n",
						passes, *repeat, g.name, truncate(c.Q, 28), c.Expect)
				}
			}
		}
	}

	summarize(results)
	if *out != "" {
		b, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(*out, b, 0o644); err != nil {
			fail("寫結果失敗: %v", err)
		}
		fmt.Printf("\n逐條結果已寫入 %s\n", *out)
	}
}

// runnerFor 只剩 coordinator 一條路徑：all-tools fallback 已於 2026-09-20 移除
// （ADR 2026-09-20-coordinator-vs-all-tools-fallback）。本工具的用途因此從「兩條
// 路徑比較」變成「coordinator 路由準確率的回歸量測」——corpus 與判讀邏輯不變，
// 那一次比較的結果存在 ADR 裡。
func runnerFor(rs *agentpkg.Runners, role, _ string) *runner.Runner {
	return rs.Coordinators[role]
}

func runOne(ctx context.Context, r *runner.Runner, sessionSvc session.Service, owners map[string]string,
	group, path string, multi bool, c Case) Result {

	res := Result{Group: group, Path: path, Query: c.Q, Expect: c.Expect}
	userID := "eval-user"
	createResp, err := sessionSvc.Create(ctx, &session.CreateRequest{
		AppName: string(agentpkg.AppNameAssistant), UserID: userID,
	})
	if err != nil {
		res.Err = "session: " + err.Error()
		return res
	}

	msg := genai.NewContentFromText(c.Q, genai.RoleUser)
	seen := map[string]bool{}
	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	for event, err := range r.Run(runCtx, userID, createResp.Session.ID(), msg, adkagent.RunConfig{}) {
		if err != nil {
			res.Err = err.Error()
			break
		}
		if event == nil || event.Content == nil {
			continue
		}
		for _, part := range event.Content.Parts {
			if part.FunctionResponse == nil {
				continue
			}
			if owner, ok := owners[part.FunctionResponse.Name]; ok {
				seen[owner] = true
			}
		}
	}
	res.Millis = time.Since(start).Milliseconds()

	for k := range seen {
		res.Actual = append(res.Actual, k)
	}
	sort.Strings(res.Actual)

	want := map[string]bool{}
	for _, e := range c.Expect {
		want[e] = true
	}
	missing := 0
	for e := range want {
		if !seen[e] {
			missing++
		}
	}
	for a := range seen {
		if !want[a] {
			res.Extra = append(res.Extra, a)
		}
	}
	sort.Strings(res.Extra)
	res.Complete = missing == 0
	if multi {
		res.Hit = res.Complete
	} else {
		res.Hit = res.Complete && len(res.Extra) == 0
	}
	return res
}

func summarize(rs []Result) {
	type agg struct{ hit, scored, errored int }
	by := map[string]*agg{}
	for _, r := range rs {
		k := r.Group + "/" + r.Path
		if by[k] == nil {
			by[k] = &agg{}
		}
		// 沒跑到模型的不是「答錯」，不能混進分母——否則環境問題會被讀成準確率 0%。
		if r.Err != "" {
			by[k].errored++
			continue
		}
		by[k].scored++
		if r.Hit {
			by[k].hit++
		}
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("\n=== 摘要（單一意圖＝選對且無多餘；多步驟＝全部呼叫到）===")
	fmt.Println("⚠️ 委派是隨機的：--repeat 1 的數字不能當結論，多步驟尤其。")
	totalErr := 0
	for _, k := range keys {
		a := by[k]
		totalErr += a.errored
		if a.scored == 0 {
			fmt.Printf("  %-28s 無有效樣本（%d 條因錯誤未計分）\n", k, a.errored)
			continue
		}
		note := ""
		if a.errored > 0 {
			note = fmt.Sprintf("，另 %d 條因錯誤未計分", a.errored)
		}
		fmt.Printf("  %-28s %d/%d (%.0f%%)%s\n", k, a.hit, a.scored,
			100*float64(a.hit)/float64(a.scored), note)
	}
	if totalErr > 0 {
		fmt.Printf("\n⚠️ 共 %d 條未跑到模型（見上方每條的錯誤訊息）。"+
			"這些**不是答錯**，不計入準確率——環境問題不該被讀成 coordinator 選錯。\n", totalErr)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func fail(f string, a ...any) {
	fmt.Fprintf(os.Stderr, strings.TrimRight(f, "\n")+"\n", a...)
	os.Exit(1)
}
