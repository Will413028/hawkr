package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

const (
	aliasToolName      = "suggest_category_aliases"
	aliasMaxPerNode    = 8
	aliasMaxOutputToks = 512
)

// aliasSource 抽成介面，讓共識與檢查邏輯可以在沒有 LLM 的情況下被測。
type aliasSource interface {
	Aliases(ctx context.Context, target node, siblings []string) ([]string, error)
}

// consensus 只保留出現在至少 min 次取樣裡的字串。同一次取樣內的重複不重複計票。
func consensus(samples [][]string, min int) []string {
	counts := map[string]int{}
	var order []string
	for _, sample := range samples {
		seen := map[string]bool{}
		for _, raw := range sample {
			s := strings.TrimSpace(raw)
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			if counts[s] == 0 {
				order = append(order, s)
			}
			counts[s]++
		}
	}
	var out []string
	for _, s := range order {
		if counts[s] >= min {
			out = append(out, s)
		}
	}
	return out
}

// generateAll 逐個 L2 呼叫 samples 次，取共識後攤成候選清單。
//
// 一個節點一次呼叫（而非整批）：這是逐節點的生成任務，一次餵一百多個節點要
// 模型為每個都產別名會輸出過長、易漏項或截斷，且出錯難以歸因。
func generateAll(ctx context.Context, src aliasSource, t tree, samples, minAgree int) ([]candidate, error) {
	siblingsByL1 := map[string][]string{}
	for _, n := range t.L2 {
		siblingsByL1[n.L1Name] = append(siblingsByL1[n.L1Name], n.Name)
	}

	var out []candidate
	for _, target := range t.L2 {
		siblings := make([]string, 0, len(siblingsByL1[target.L1Name]))
		for _, name := range siblingsByL1[target.L1Name] {
			if name != target.Name {
				siblings = append(siblings, name)
			}
		}
		rounds := make([][]string, 0, samples)
		for range samples {
			got, err := src.Aliases(ctx, target, siblings)
			if err != nil {
				return nil, fmt.Errorf("generating aliases for %s(%s): %w", target.Name, target.ID, err)
			}
			rounds = append(rounds, got)
		}
		for _, alias := range consensus(rounds, minAgree) {
			out = append(out, candidate{Alias: alias, Target: target})
		}
	}
	return out, nil
}

const aliasInstruction = `你是台灣生鮮採購的分類專家。使用者會給你一個商品分類節點，
請列出台灣買家在採購時可能用來稱呼「這個分類」的其他說法（俗名、慣用名、常見錯字）。

規則：
1. 只列真的會被打出來的詞，寧缺勿濫。想不到就回空陣列。
2. 不要列出分類樹上已經存在的名稱——目標名稱本身、它的子分類名稱、以及提供的兄弟分類名稱都不可以。
3. 不要列出比這個分類更細的品種名，也不要列出更廣的上位詞。
4. 只回繁體中文詞彙。`

type modelAliasSource struct {
	llm       model.LLM
	modelName string
	toolCall  bool
}

func newModelAliasSource(p llmprovider.Procurement) aliasSource {
	return &modelAliasSource{llm: p.LLM(), modelName: p.ModelName(), toolCall: p.Provider() != "gemini"}
}

func (s *modelAliasSource) Aliases(ctx context.Context, target node, siblings []string) ([]string, error) {
	children := make([]string, 0, len(target.Children))
	for _, c := range target.Children {
		children = append(children, c.Name)
	}
	payload, err := json.Marshal(struct {
		Category string   `json:"category"`
		Parent   string   `json:"parent"`
		Children []string `json:"children"`
		Siblings []string `json:"siblings"`
	}{target.Name, target.L1Name, children, siblings})
	if err != nil {
		return nil, err
	}

	schema := &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"aliases"},
		Properties: map[string]*genai.Schema{
			"aliases": {Type: genai.TypeArray, MaxItems: genai.Ptr(int64(aliasMaxPerNode)), Items: &genai.Schema{Type: genai.TypeString}},
		},
	}
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(aliasInstruction, genai.RoleUser),
		Temperature:       genai.Ptr(float32(0.7)),
		MaxOutputTokens:   aliasMaxOutputToks,
	}
	if s.toolCall {
		cfg.Tools = []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{{Name: aliasToolName, Description: "列出這個商品分類的消費者慣用別名。", Parameters: schema}}}}
		cfg.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAny}}
	} else {
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseSchema = schema
	}

	request := &model.LLMRequest{Model: s.modelName, Contents: []*genai.Content{genai.NewContentFromText(string(payload), genai.RoleUser)}, Config: cfg}
	var response *model.LLMResponse
	for result, callErr := range s.llm.GenerateContent(ctx, request, false) {
		if callErr != nil {
			return nil, callErr
		}
		response = result
	}
	if response == nil || response.Content == nil {
		return nil, fmt.Errorf("alias response empty")
	}
	var raw []byte
	for _, part := range response.Content.Parts {
		if part == nil {
			continue
		}
		if s.toolCall && part.FunctionCall != nil && part.FunctionCall.Name == aliasToolName {
			raw, err = json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, err
			}
			break
		}
		if !s.toolCall && strings.TrimSpace(part.Text) != "" {
			raw = []byte(part.Text)
			break
		}
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("alias structured output missing")
	}
	var parsed struct {
		Aliases []string `json:"aliases"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	return parsed.Aliases, nil
}
