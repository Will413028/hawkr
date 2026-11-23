// Package llm 是單體唯一允許建構 LLM client 的地方。
//
// 兩個由此消除的缺陷類別：provider quirk 在拷貝之間漂移（DeepSeek 少了 reasoning.effort=none，強制 tool_choice 一律
// HTTP 400），以及缺金鑰讓整個服務起不來。消費者拿到的是一個已經決定好開通與否的 Model，自己不建、也不判；
// internal/arch 禁止其他套件 import 各 provider 的 SDK。
//
// 模型名只能取自封閉的登記表，不是自由字串：模型名與 provider quirk 綁定，未驗過的模型可能靜默送出被端點拒絕的 request。
// 設定不完整時 New 回停用的 Model（它的 LLM 一律回 ErrDisabled），不讓啟動失敗，並以 log 與 Status 說明原因。
package llm

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"sort"

	"github.com/openai/openai-go/v3/option"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

// Provider 是登記表裡的 provider。
type Provider string

const (
	Gemini   Provider = "gemini"
	DeepSeek Provider = "deepseek"
)

const (
	// DefaultModel 是模型名未設定時的值；沒有金鑰時視為尚未開通，而不是設定錯誤。
	DefaultModel = "gemini-2.5-flash"
	// DeepSeekFlash 與 DeepSeekV4Pro 是 DeepSeek 已驗過的模型名。
	DeepSeekFlash = "deepseek-flash"
	DeepSeekV4Pro = "deepseek-v4-pro"
	// GeminiImage 是修圖用的圖片模型，只有 ImageAI 角色能用，ImageAI 也只能用圖片模型。
	GeminiImage = "gemini-2.5-flash-image"

	deepSeekBaseURL = "https://api.deepseek.com"
)

// Role 是 LLM 的用途；每個用途各自選模型。
type Role string

const (
	// Assistant 是買家助理，只能用 Gemini。
	Assistant Role = "assistant"
	// Procurement 是採購對話與抽取，登記表內的任一模型。
	Procurement Role = "procurement"
	// ImageAI 是賣家的 AI 修圖，只能用 Gemini 的圖片模型；它的 client 是 ImageModel，不是 ADK 的 model.LLM。
	ImageAI Role = "imageai"
)

// Status 說明一個 Model 為什麼開通或停用。
type Status string

const (
	StatusEnabled Status = "enabled"
	// StatusNotConfigured 是預設模型沒有金鑰：沒有人選過它，不算設定錯誤。
	StatusNotConfigured Status = "not_configured"
	// StatusMisconfigured 是模型名不在登記表、角色不允許該模型、明確選了的 provider 缺金鑰，或 client 建構失敗。
	StatusMisconfigured Status = "misconfigured"
)

// ErrDisabled 是 LLM 尚未開通時呼叫端得到的錯誤。
var ErrDisabled = apperr.Unavailable("AI 助理尚未開通，請稍後再試")

type modelSpec struct {
	provider Provider
	baseURL  string
	// image 是圖片模型：只給 ImageAI 角色，其他角色不得選它。
	image bool
	// options 是這個模型專屬的 RequestOption；逐筆列出，讓哪個模型需要哪個 quirk 可讀。
	options func() []option.RequestOption
}

// deepSeekOptions 是 DeepSeek 系列的 RequestOption。不帶 reasoning.effort=none 時，tool_choice 為 required 的請求會得到
// HTTP 400 "Thinking mode does not support this tool_choice"。關掉 SDK 自帶重試，重試只留在呼叫端一層。
func deepSeekOptions() []option.RequestOption {
	return []option.RequestOption{
		option.WithMaxRetries(0),
		option.WithJSONSet("reasoning", map[string]string{"effort": "none"}),
	}
}

var registry = map[string]modelSpec{
	DefaultModel:  {provider: Gemini},
	GeminiImage:   {provider: Gemini, image: true},
	DeepSeekFlash: {provider: DeepSeek, baseURL: deepSeekBaseURL, options: deepSeekOptions},
	DeepSeekV4Pro: {provider: DeepSeek, baseURL: deepSeekBaseURL, options: deepSeekOptions},
}

var roleProviders = map[Role][]Provider{
	Assistant:   {Gemini},
	Procurement: {Gemini, DeepSeek},
	ImageAI:     {Gemini},
}

// roleDefaults 是各用途沒設定模型名時的預設；沒有列的用途取 DefaultModel。
var roleDefaults = map[Role]string{ImageAI: GeminiImage}

func defaultModel(role Role) string {
	if name, ok := roleDefaults[role]; ok {
		return name
	}
	return DefaultModel
}

func roleAccepts(role Role, spec modelSpec) bool {
	return slices.Contains(roleProviders[role], spec.provider) && spec.image == (role == ImageAI)
}

// KnownModels 回傳登記表的模型名（已排序）。
func KnownModels() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Config 是 LLM 的設定；空字串的模型名取 DefaultModel。
type Config struct {
	GeminiAPIKey   string
	DeepSeekAPIKey string
	// Models 是各用途選的模型名。
	Models map[Role]string
}

// ReadConfig 讀 GEMINI_API_KEY、DEEPSEEK_API_KEY、LLM_ASSISTANT_MODEL、LLM_PROCUREMENT_MODEL、LLM_IMAGEAI_MODEL；全部選填。
func ReadConfig(e *config.Env) Config {
	return Config{
		GeminiAPIKey:   e.StringOr("GEMINI_API_KEY", ""),
		DeepSeekAPIKey: e.StringOr("DEEPSEEK_API_KEY", ""),
		Models: map[Role]string{
			Assistant:   e.StringOr("LLM_ASSISTANT_MODEL", ""),
			Procurement: e.StringOr("LLM_PROCUREMENT_MODEL", ""),
			ImageAI:     e.StringOr("LLM_IMAGEAI_MODEL", ""),
		},
	}
}

func (c Config) apiKey(p Provider) string {
	if p == DeepSeek {
		return c.DeepSeekAPIKey
	}
	return c.GeminiAPIKey
}

func keyName(p Provider) string {
	if p == DeepSeek {
		return "DEEPSEEK_API_KEY"
	}
	return "GEMINI_API_KEY"
}

// Model 是一個已決定開通與否的 LLM。零值不可用，由 New 建構。
type Model struct {
	llm      model.LLM
	name     string
	provider Provider
	status   Status
}

// LLM 永遠非 nil：停用時是回 ErrDisabled 的實作。
func (m Model) LLM() model.LLM { return m.llm }

// Name 是開通的模型名，停用時為空字串。
func (m Model) Name() string { return m.name }

// Provider 是開通的 provider，停用時為空字串。
func (m Model) Provider() Provider { return m.provider }

func (m Model) Status() Status { return m.status }
func (m Model) Enabled() bool  { return m.status == StatusEnabled }

// New 決定 role 的 LLM 開通狀態並建構 client。永遠不回 error。
func New(role Role, cfg Config, log *slog.Logger) Model {
	return build(role, cfg, log, "")
}

// NewModel 以給定的 client 組出已開通的 Model，讓組裝測試與整合測試用替身 client 走正式的 Model 路徑；正式程式只經 New。
func NewModel(l model.LLM, name string, provider Provider) Model {
	return Model{llm: l, name: name, provider: provider, status: StatusEnabled}
}

func disabled(status Status) Model { return Model{llm: disabledLLM{}, status: status} }

// build 是全服務唯一的 client 建構點；baseURLOverride 只給測試把 OpenAI 相容的端點指到替身 server。
func build(role Role, cfg Config, log *slog.Logger, baseURLOverride string) Model {
	if role == ImageAI {
		log.Error("ImageAI 角色沒有 ADK model，請用 NewImage，已停用", "role", role)
		return disabled(StatusMisconfigured)
	}
	name, spec, key, status := resolve(role, cfg, log)
	if status != StatusEnabled {
		return disabled(status)
	}
	llm, err := newClient(name, spec, key, baseURLOverride)
	if err != nil {
		log.Error("LLM client 建立失敗，已停用", "role", role, "model", name, "error", err)
		return disabled(StatusMisconfigured)
	}
	log.Info("LLM 已開通", "role", role, "model", name, "provider", spec.provider)
	return Model{llm: llm, name: name, provider: spec.provider, status: StatusEnabled}
}

// resolve 決定 role 用哪個模型與金鑰；status 不是 StatusEnabled 時其餘回傳值無意義。
func resolve(role Role, cfg Config, log *slog.Logger) (name string, spec modelSpec, key string, status Status) {
	name = cfg.Models[role]
	if name == "" {
		name = defaultModel(role)
	}
	spec, known := registry[name]
	if !known || !roleAccepts(role, spec) {
		log.Error("LLM 模型不在這個用途可用的登記表內，已停用",
			"role", role, "model", name, "known_models", KnownModels())
		return "", modelSpec{}, "", StatusMisconfigured
	}
	key = cfg.apiKey(spec.provider)
	if key == "" {
		if name == defaultModel(role) && cfg.Models[role] == "" {
			log.Warn("LLM 尚未開通：沒有 API key", "role", role, "missing_key", keyName(spec.provider))
			return "", modelSpec{}, "", StatusNotConfigured
		}
		log.Error("LLM 指定的 provider 缺 API key，已停用",
			"role", role, "model", name, "missing_key", keyName(spec.provider))
		return "", modelSpec{}, "", StatusMisconfigured
	}
	return name, spec, key, StatusEnabled
}

func newClient(name string, spec modelSpec, apiKey, baseURLOverride string) (model.LLM, error) {
	switch spec.provider {
	case Gemini:
		return gemini.NewModel(context.Background(), name, &genai.ClientConfig{APIKey: apiKey})
	case DeepSeek:
		baseURL := spec.baseURL
		if baseURLOverride != "" {
			baseURL = baseURLOverride
		}
		return openaimodel.NewModel(context.Background(), name, &openaimodel.ClientConfig{APIKey: apiKey, BaseURL: baseURL, Options: spec.options()})
	default:
		return nil, fmt.Errorf("llm：不認得的 provider %q", spec.provider)
	}
}

type disabledLLM struct{}

func (disabledLLM) Name() string { return "disabled" }

func (disabledLLM) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) { yield(nil, ErrDisabled) }
}
