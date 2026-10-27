package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/goldenset"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func mintHS256(secret string, accountID identity.AccountID) (string, error) {
	enc := func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	now := time.Now()
	h, err := enc(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	p, err := enc(map[string]any{
		"accountId": int64(accountID), "accountType": 1, "loginRole": 1,
		"iat": now.Unix(), "exp": now.Add(2 * time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}
	signing := h + "." + p
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	return signing + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

type evalClient struct {
	base  string
	token string
	http  *http.Client
}

func (c *evalClient) post(path string, body any) (map[string]any, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("非 JSON 回應（%d）：%.200s", resp.StatusCode, raw)
	}
	return out, nil
}

func categoryPaths(base string) (map[string]string, error) {
	resp, err := http.Get(base + "/public/categories/tree?status=active")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var payload struct {
		Data []struct {
			ID       string          `json:"id"`
			Name     string          `json:"name"`
			Children json.RawMessage `json:"children"`
		} `json:"data"`
	}
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := map[string]string{}
	var walk func(nodes json.RawMessage, parent string)
	walk = func(nodes json.RawMessage, parent string) {
		var list []struct {
			ID       string          `json:"id"`
			Name     string          `json:"name"`
			Children json.RawMessage `json:"children"`
		}
		if json.Unmarshal(nodes, &list) != nil {
			return
		}
		for _, n := range list {
			path := n.Name
			if parent != "" {
				path = parent + " > " + n.Name
			}
			if n.ID != "" {
				out[n.ID] = path
			}
			walk(n.Children, path)
		}
	}
	for _, root := range payload.Data {
		path := root.Name
		if root.ID != "" {
			out[root.ID] = path
		}
		walk(root.Children, path)
	}
	return out, nil
}

func dig(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func observe(q goldenset.Query, data map[string]any, paths map[string]string) goldenset.Observation {
	o := goldenset.Observation{Query: q}
	if res, ok := dig(data, "slots", "item_resolution").(map[string]any); ok && res != nil {
		if label, _ := res["display_value"].(string); label != "" {
			o.ResolvedLabel = label
		} else if label, _ := res["label"].(string); label != "" {
			o.ResolvedLabel = label
		}
	}
	question, _ := dig(data, "question").(map[string]any)
	if question != nil {
		if slot, _ := question["slotKey"].(string); slot == "crop_disambiguation" {
			opts, _ := question["options"].([]any)
			sawOption := false
			for _, raw := range opts {
				opt, _ := raw.(map[string]any)
				id, _ := opt["optionId"].(string)
				if id == "" || id == "crop-other" {
					continue
				}
				sawOption = true
				if label, _ := opt["label"].(string); label != "" {
					o.RankedLabels = append(o.RankedLabels, label)
				}
				if cid, found := strings.CutPrefix(id, "category-"); found {
					if p := paths[cid]; p != "" {
						o.RankedCategories = append(o.RankedCategories, p)
					}
				}
			}
			if !sawOption {
				o.IsNIL = true
				return o
			}
			if len(o.RankedCategories) == 0 && len(o.RankedLabels) == 0 {
				o.Unmappable = true
			}
			return o
		}
	}
	if o.ResolvedLabel == "" {
		o.IsNIL = true
		return o
	}
	o.Unmappable = true
	return o
}

func main() {
	setPath := flag.String("set", "internal/procurement/goldenset/testdata/golden_queries.yaml", "黃金查詢集 yaml")
	k := flag.Int("k", 10, "Recall@k 的 k")
	base := flag.String("base", "http://localhost:8080/api/v1", "gateway base URL（dev）")
	// 多輪情境模式（R10）。不帶這些 flag 時行為與既有的 IR 評測完全相同。
	scenarioPath := flag.String("scenarios", "", "多輪情境 corpus yaml（與 --set 互斥）")
	reportPath := flag.String("report", "", "把情境評測結果寫成 JSON 報告")
	baselinePath := flag.String("baseline", "", "與這份 baseline 報告比較，只列差異")
	caseID := flag.String("case", "", "只跑這一個 scenario id")
	maxCases := flag.Int("max-cases", 0, "最多跑幾個 scenario（0＝不限）")
	maxTurns := flag.Int("max-turns", 0, "每個 scenario 最多跑幾輪（0＝不限）")
	flag.Parse()

	// ⛔ 互斥：兩份 corpus 驗的是不同的東西，混在一起跑出來的報告沒有意義。
	if *scenarioPath != "" && isFlagSet("set") {
		fmt.Println("--set 與 --scenarios 互斥：前者是單句品項身分的 IR 評測，後者是多輪情境")
		os.Exit(2)
	}
	if *scenarioPath != "" {
		os.Exit(runScenarioEval(scenarioEvalOptions{
			Base: *base, CorpusPath: *scenarioPath, ReportPath: *reportPath,
			BaselinePath: *baselinePath,
			Limits:       scenarioLimits{Case: *caseID, MaxCases: *maxCases, MaxTurns: *maxTurns},
		}))
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		fmt.Println("JWT_SECRET 未設定——這支工具要鑄 dev token 才打得了 buyer 端點")
		os.Exit(1)
	}
	raw, err := os.ReadFile(*setPath)
	if err != nil {
		fmt.Println("讀不到查詢集：", err)
		os.Exit(1)
	}
	queries, err := goldenset.Load(raw)
	if err != nil {
		fmt.Println("查詢集不合法：", err)
		os.Exit(1)
	}
	paths, err := categoryPaths(*base)
	if err != nil || len(paths) == 0 {
		fmt.Println("分類樹取不到，評測會把每條都判成映射不到：", err)
		os.Exit(1)
	}

	accountID := identity.AccountID(time.Now().UnixNano() % 900000000)
	tok, err := mintHS256(secret, accountID)
	if err != nil {
		fmt.Println("鑄 token 失敗：", err)
		os.Exit(1)
	}
	c := &evalClient{base: *base, token: tok, http: &http.Client{Timeout: 90 * time.Second}}

	if _, err := c.post("/app/buyers/me", map[string]any{"nickname": "resolver-eval"}); err != nil {
		fmt.Println("provision buyer 失敗：", err)
		os.Exit(1)
	}
	fmt.Printf("分類樹 %d 個節點，合成買家 %d，查詢 %d 條\n\n", len(paths), accountID, len(queries))

	obs := make([]goldenset.Observation, 0, len(queries))
	for i, q := range queries {
		body := map[string]any{
			"requestId":       "",
			"message":         buyerMessage(q),
			"clientRequestId": fmt.Sprintf("eval-%d-%d", accountID, i),
		}
		resp, err := c.post("/app/buyers/me/procurement/turns", body)
		if err != nil {
			fmt.Printf("  ⚠️  %-10s turn 失敗：%v\n", q.Text, err)
			obs = append(obs, goldenset.Observation{Query: q})
			continue
		}
		if code, _ := resp["code"].(float64); code != 0 {
			fmt.Printf("  ⚠️  %-10s turn 回 %v %v\n", q.Text, resp["code"], resp["message"])
			obs = append(obs, goldenset.Observation{Query: q})
			continue
		}
		data, _ := resp["data"].(map[string]any)
		obs = append(obs, observe(q, data, paths))
	}

	r := goldenset.Evaluate(obs, *k)
	fmt.Printf("\n=== 品項解析 baseline（%d 條，k=%d，%s）===\n", r.Total, *k, time.Now().Format("2006-01-02"))
	fmt.Printf("Precision@1  %.3f – %.3f   （下界＝unmappable 全當沒命中，上界＝全當命中）\n", r.PrecisionAt1, r.PrecisionAt1Upper)
	fmt.Printf("Recall@%-2d    %.3f\n", *k, r.RecallAtK)
	fmt.Printf("MRR          %.3f\n", r.MRR)
	fmt.Printf("unmappable   %d   known_gap %d\n\n", r.Unmappable, r.KnownGap)

	label := map[string]string{
		goldenset.FailureRanking:    "排序問題（正解在候選裡，沒排第一）",
		goldenset.FailureRecall:     "召回/資料問題（正解不在候選裡）",
		goldenset.FailureUnmappable: "評測限制（有解析結果但帶不出分類路徑）",
	}
	for _, kind := range []string{goldenset.FailureRecall, goldenset.FailureRanking, goldenset.FailureUnmappable} {
		first := true
		for _, f := range r.Failures {
			if f.Kind != kind {
				continue
			}
			if first {
				fmt.Printf("── %s\n", label[kind])
				first = false
			}
			fmt.Printf("   %-12s 期望 %s\n                實得 %v\n", f.Query.Text, f.Query.ExpectDescription(), f.Got)
		}
	}
}

// isFlagSet 分辨「使用者真的給了這個 flag」與「用了預設值」——--set 有預設值，
// 不這樣判就無法偵測互斥。
func isFlagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func buyerMessage(q goldenset.Query) string {
	if q.Message != "" {
		return q.Message
	}
	return fmt.Sprintf("我要買%s 50 公斤，送桃園市", q.Text)
}
