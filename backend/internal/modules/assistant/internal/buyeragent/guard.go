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

const (
	codeToolFailed          = "tool_failed"
	codeUpstreamUnavailable = "upstream_unavailable"
	codeOrderNotFound       = "order_not_found"
	codeForbidden           = "forbidden"
	codeUnknownTool         = "unknown_tool"
	codeDelegateFailed      = "delegate_failed"
)

const (
	noteNoPackages              = "no_packages_in_transit"
	noteSomePackagesUnavailable = "some_packages_unavailable"
	noteMorePackagesNotShown    = "more_packages_not_shown"
)

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

type callable interface {
	tool.Tool
	Declaration() *genai.FunctionDeclaration
	Run(ctx agent.Context, args any) (map[string]any, error)
}

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
			te = &toolError{code: codeToolFailed, cause: err}
		}
		slog.WarnContext(ctx, "assistant：tool 失敗", "tool", call.Name, "code", te.code, "error", errors.Unwrap(te), "detail", fmt.Sprint(err))
		return nil, &toolError{code: te.code}
	}
	return result, nil
}

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

func onToolError(known map[string]bool) func(agent.Context, tool.Tool, map[string]any, error) (map[string]any, error) {
	return func(ctx agent.Context, t tool.Tool, _ map[string]any, err error) (map[string]any, error) {
		if _, guarded := t.(*guardedTool); guarded {
			return nil, err
		}
		if !known[t.Name()] {
			slog.WarnContext(ctx, "assistant：模型呼叫了不存在的 tool", "tool", t.Name())
			return map[string]any{"error": codeUnknownTool}, nil
		}
		slog.WarnContext(ctx, "assistant：委派 specialist 失敗", "tool", t.Name(), "error", err)
		return map[string]any{"error": codeDelegateFailed}, nil
	}
}
