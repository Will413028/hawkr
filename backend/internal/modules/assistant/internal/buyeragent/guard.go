package buyeragent

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/toolutils"
	"google.golang.org/genai"

	assistantmodel "github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/model"
)

// 給模型看的 tool 錯誤代碼：不帶語言、不帶下游的錯誤文字。模型依系統提示把它們轉成買家看得懂的說法。
const (
	codeToolFailed          = "tool_failed"
	codeUpstreamUnavailable = "upstream_unavailable"
	codeOrderNotFound       = "order_not_found"
	codeForbidden           = "forbidden"
	codeUnknownTool         = "unknown_tool"
	codeDelegateFailed      = "delegate_failed"
)

// 給模型看的 tool 結果說明代碼（tool 成功但有但書）：同樣不帶語言。
const (
	// noteNoPackages：目前沒有追蹤中的包裹。
	noteNoPackages = "no_packages_in_transit"
	// noteSomePackagesUnavailable：有部分包裹的詳情查不到，回的清單不完整。
	noteSomePackagesUnavailable = "some_packages_unavailable"
)

// toolError 是 tool 回給模型的錯誤：Error() 只有代碼。cause 是真正的原因，只進 log，不給模型。
type toolError struct {
	code  string
	cause error
}

func (e *toolError) Error() string { return e.code }
func (e *toolError) Unwrap() error { return e.cause }

func toolFail(code string, cause error) error { return &toolError{code: code, cause: cause} }

// ToolCall 是一次 tool 呼叫的結果，經 Deps.OnTool 回報。
type ToolCall struct {
	Tool assistantmodel.Tool
	// Name 是模型呼叫的 tool 名稱（寫進 proto 之前的內部名稱，測試比對用）。
	Name string
	// Args 是模型送出的參數的深拷貝（沒有參數時是 nil），呼叫者改它不影響 ADK。不進對外事件。
	Args map[string]any
	// Failed 是 tool 沒有成功；模型只會看到錯誤代碼，助理仍會接著回覆。
	Failed bool
}

// callable 是 ADK 在一輪裡執行 function tool 時要的形狀（ADK 的 toolinternal.FunctionTool 在 internal 套件，無法引用）。
type callable interface {
	tool.Tool
	Declaration() *genai.FunctionDeclaration
	Run(ctx agent.Context, args any) (map[string]any, error)
}

// guardedTool 是 agent 唯一能掛的 function tool：包在 tool.Tool.Run 外層（A9-D15 第 3 點），
// 負責 panic recover、把所有錯誤換成固定代碼，以及每次呼叫都回報一次 ToolCall——參數驗證失敗也算。
// agent 的建構函式只接受這個型別，所以沒有 tool 能繞過它。
type guardedTool struct {
	inner callable
	id    assistantmodel.Tool
	out   func(ToolCall)
}

func (g *guardedTool) Name() string                            { return g.inner.Name() }
func (g *guardedTool) Description() string                     { return g.inner.Description() }
func (g *guardedTool) IsLongRunning() bool                     { return g.inner.IsLongRunning() }
func (g *guardedTool) Declaration() *genai.FunctionDeclaration { return g.inner.Declaration() }

// ProcessRequest 把自己（不是 inner）登記進 LLM 請求：ADK 依 req.Tools 的值執行 tool，登記 inner 就繞過了這一層。
func (g *guardedTool) ProcessRequest(_ agent.Context, req *model.LLMRequest) error {
	return toolutils.PackTool(req, g)
}

// Run 執行 inner；呼叫者拿到的錯誤只有代碼。
func (g *guardedTool) Run(ctx agent.Context, args any) (result map[string]any, err error) {
	call := ToolCall{Tool: g.id, Name: g.inner.Name()}
	if m, ok := args.(map[string]any); ok && len(m) > 0 {
		call.Args = cloneArgs(m)
	}
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "assistant：tool panic", "tool", call.Name, "panic", r, "stack", string(debug.Stack()))
			result, err = nil, toolFail(codeToolFailed, nil)
		}
		call.Failed = err != nil
		if g.out != nil {
			g.out(call)
		}
	}()
	result, err = g.inner.Run(ctx, args)
	if err != nil {
		var te *toolError
		if !errors.As(err, &te) {
			// 參數驗證失敗，或 inner 自己轉成 error 的 panic（文字含 stack）：只進 log。
			te = &toolError{code: codeToolFailed, cause: err}
		}
		slog.WarnContext(ctx, "assistant：tool 失敗", "tool", call.Name, "code", te.code, "error", errors.Unwrap(te), "detail", fmt.Sprint(err))
		return nil, &toolError{code: te.code}
	}
	return result, nil
}

// cloneArgs 深拷貝模型送的參數（JSON 解出的 map、slice 與純量）。
func cloneArgs(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return cloneArgs(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = cloneValue(e)
		}
		return out
	default:
		return v
	}
}

// onToolError 是掛在每個 agent 上的 OnToolErrorCallback，處理 guardedTool 管不到的兩種錯誤，模型只看到代碼：
// 模型叫了不存在的 tool（ADK 的預設錯誤文字會列出所有 tool 名稱），以及 coordinator 委派 specialist 失敗。
// 前者沒有對應的 AssistantTool，無法表示成 tool_result（UNSPECIFIED 不得當合法值），所以只記 log。
func onToolError(known map[string]bool) func(agent.Context, tool.Tool, map[string]any, error) (map[string]any, error) {
	return func(ctx agent.Context, t tool.Tool, _ map[string]any, err error) (map[string]any, error) {
		if _, guarded := t.(*guardedTool); guarded {
			return nil, err // 已經是代碼
		}
		if !known[t.Name()] {
			slog.WarnContext(ctx, "assistant：模型呼叫了不存在的 tool", "tool", t.Name())
			return map[string]any{"error": codeUnknownTool}, nil
		}
		slog.WarnContext(ctx, "assistant：委派 specialist 失敗", "tool", t.Name(), "error", err)
		return map[string]any{"error": codeDelegateFailed}, nil
	}
}
