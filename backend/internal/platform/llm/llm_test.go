package llm

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRegistryIsClosed(t *testing.T) {
	if want := []string{DefaultModel, GeminiImage, DeepSeekFlash, DeepSeekV4Pro}; !slices.Equal(KnownModels(), slices.Sorted(slices.Values(want))) {
		t.Fatalf("KnownModels = %v", KnownModels())
	}
	for name, spec := range registry {
		if spec.provider == DeepSeek && (spec.options == nil || spec.baseURL == "") {
			t.Errorf("%s 缺 provider 設定", name)
		}
	}
	m := New(Procurement, Config{GeminiAPIKey: "k", Models: map[Role]string{Procurement: "gemma-free-text"}}, quiet())
	if m.Enabled() || m.Status() != StatusMisconfigured {
		t.Fatalf("未登記的模型名被接受：%v", m.Status())
	}
}

func TestAssistantOnlyAcceptsGemini(t *testing.T) {
	cfg := Config{GeminiAPIKey: "k", DeepSeekAPIKey: "k", Models: map[Role]string{Assistant: DeepSeekFlash}}
	if m := New(Assistant, cfg, quiet()); m.Enabled() || m.Status() != StatusMisconfigured {
		t.Fatalf("assistant 選了 DeepSeek：%v", m.Status())
	}
	cfg.Models[Assistant] = ""
	m := New(Assistant, cfg, quiet())
	if !m.Enabled() || m.Name() != DefaultModel || m.Provider() != Gemini {
		t.Fatalf("assistant = %+v", m)
	}
}

func TestMissingKeyDegradesInsteadOfFailing(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg  Config
		want Status
	}{
		"default model without key":   {Config{}, StatusNotConfigured},
		"explicit gemini without key": {Config{Models: map[Role]string{Procurement: DefaultModel}}, StatusMisconfigured},
		"deepseek without key":        {Config{GeminiAPIKey: "k", Models: map[Role]string{Procurement: DeepSeekFlash}}, StatusMisconfigured},
	} {
		m := New(Procurement, tc.cfg, quiet())
		if m.Enabled() || m.Status() != tc.want || m.LLM() == nil || m.Name() != "" || m.Provider() != "" {
			t.Errorf("%s：%+v", name, m)
			continue
		}
		var gotErr error
		for _, err := range m.LLM().GenerateContent(context.Background(), &model.LLMRequest{}, false) {
			gotErr = err
		}
		if !errors.Is(gotErr, ErrDisabled) || apperr.KindOf(gotErr) != apperr.KindUnavailable {
			t.Errorf("%s：停用的 LLM 回 %v", name, gotErr)
		}
	}
}

func TestEnabledModels(t *testing.T) {
	cfg := Config{GeminiAPIKey: "g", DeepSeekAPIKey: "d", Models: map[Role]string{Procurement: DeepSeekV4Pro}}
	m := New(Procurement, cfg, quiet())
	if !m.Enabled() || m.Name() != DeepSeekV4Pro || m.Provider() != DeepSeek {
		t.Fatalf("deepseek = %+v", m)
	}
	cfg.Models[Procurement] = ""
	m = New(Procurement, cfg, quiet())
	if !m.Enabled() || m.Name() != DefaultModel || m.Provider() != Gemini {
		t.Fatalf("gemini = %+v", m)
	}
}

func TestReadConfig(t *testing.T) {
	cfg := ReadConfig(config.FromMap(map[string]string{"GEMINI_API_KEY": "g", "LLM_PROCUREMENT_MODEL": DeepSeekFlash}))
	if cfg.GeminiAPIKey != "g" || cfg.DeepSeekAPIKey != "" || cfg.Models[Procurement] != DeepSeekFlash || cfg.Models[Assistant] != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
	empty := config.FromMap(nil)
	ReadConfig(empty)
	if err := empty.Err(); err != nil {
		t.Fatalf("全部選填，卻回報 %v", err)
	}
}

func TestDeepSeekSendsReasoningEffortNone(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies [][]byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, b)
		mu.Unlock()
		http.Error(w, `{"error":{"message":"stub"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()
	m := build(Procurement, Config{DeepSeekAPIKey: "k", Models: map[Role]string{Procurement: DeepSeekFlash}}, quiet(), srv.URL)
	if !m.Enabled() {
		t.Fatalf("status = %v", m.Status())
	}
	req := &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText("hi", genai.RoleUser)}}
	for _, err := range m.LLM().GenerateContent(context.Background(), req, false) {
		if err == nil {
			t.Fatal("stub 回 400，卻沒有錯誤")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 {
		t.Fatalf("請求次數 = %d（SDK 自帶重試必須關閉）", len(bodies))
	}
	if !strings.Contains(string(bodies[0]), `"reasoning":{"effort":"none"}`) {
		t.Fatalf("body 缺 reasoning.effort=none：%s", bodies[0])
	}
}
