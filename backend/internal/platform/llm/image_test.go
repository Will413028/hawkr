package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

var png = []byte{0x89, 'P', 'N', 'G', 1, 2, 3}

func imageBody() string {
	return `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"inlineData":{"mimeType":"image/png","data":"` +
		base64.StdEncoding.EncodeToString(png) + `"}}]}}]}`
}

const (
	finishSafetyBody = `{"candidates":[{"finishReason":"IMAGE_SAFETY"}]}`
	blockedBody      = `{"promptFeedback":{"blockReason":"PROHIBITED_CONTENT"}}`
)

func stub(t *testing.T, body string, got *[]byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			*got, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func adkResponse(t *testing.T, url string) (*model.LLMResponse, error) {
	t.Helper()
	m, err := gemini.NewModel(context.Background(), GeminiImage, &genai.ClientConfig{APIKey: "k", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: url}})
	if err != nil {
		t.Fatal(err)
	}
	req := &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText("p", genai.RoleUser)},
		Config:   &genai.GenerateContentConfig{ResponseModalities: []string{"IMAGE"}},
	}
	var resp *model.LLMResponse
	var gotErr error
	for r, err := range m.GenerateContent(context.Background(), req, false) {
		resp, gotErr = r, err
	}
	return resp, gotErr
}

func TestADKImagePathKeepsInlineDataAndFinishReasonButDropsBlockReason(t *testing.T) {
	resp, err := adkResponse(t, stub(t, imageBody(), nil))
	if err != nil || resp.Content == nil || len(resp.Content.Parts) != 1 || resp.Content.Parts[0].InlineData == nil ||
		string(resp.Content.Parts[0].InlineData.Data) != string(png) {
		t.Fatalf("InlineData 沒有原樣回來：%+v %v", resp, err)
	}
	resp, err = adkResponse(t, stub(t, finishSafetyBody, nil))
	if err != nil || resp.FinishReason != genai.FinishReasonImageSafety {
		t.Fatalf("FinishReason 沒有回來：%+v %v", resp, err)
	}
	_, err = adkResponse(t, stub(t, blockedBody, nil))
	if err == nil || strings.Contains(err.Error(), "PROHIBITED_CONTENT") {
		t.Fatalf("預期 ADK 丟掉 BlockReason 只回錯誤，得到 %v", err)
	}
}

func imageModel(t *testing.T, url string) ImageModel {
	t.Helper()
	m := buildImage(Config{GeminiAPIKey: "k"}, quiet(), url)
	if !m.Enabled() || m.Name() != GeminiImage {
		t.Fatalf("image model = %+v", m)
	}
	return m
}

func TestImageModelReturnsImageFinishAndBlockReasons(t *testing.T) {
	var sent []byte
	res, err := imageModel(t, stub(t, imageBody(), &sent)).Generator().GenerateImage(context.Background(), ImageRequest{Image: []byte("src"), MimeType: "image/jpeg", Prompt: "clean"})
	if err != nil || string(res.Image) != string(png) || res.MimeType != "image/png" || res.FinishReason != "STOP" {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	for _, want := range []string{`"responseModalities":["IMAGE"]`, `"mimeType":"image/jpeg"`, `"text":"clean"`, base64.StdEncoding.EncodeToString([]byte("src"))} {
		if !strings.Contains(string(sent), want) {
			t.Errorf("request body 缺 %s：%s", want, sent)
		}
	}
	res, err = imageModel(t, stub(t, finishSafetyBody, nil)).Generator().GenerateImage(context.Background(), ImageRequest{})
	if err != nil || len(res.Image) != 0 || res.FinishReason != "IMAGE_SAFETY" {
		t.Fatalf("finish：%+v %v", res, err)
	}
	res, err = imageModel(t, stub(t, blockedBody, nil)).Generator().GenerateImage(context.Background(), ImageRequest{})
	if err != nil || len(res.Image) != 0 || res.BlockReason != "PROHIBITED_CONTENT" {
		t.Fatalf("block：%+v %v", res, err)
	}
}

func TestImageRoleOnlyAcceptsImageModels(t *testing.T) {
	cfg := Config{GeminiAPIKey: "k", Models: map[Role]string{ImageAI: DefaultModel, Assistant: GeminiImage, Procurement: GeminiImage}}
	if m := New(Assistant, cfg, quiet()); m.Enabled() || m.Status() != StatusMisconfigured {
		t.Fatalf("assistant 選了圖片模型：%v", m.Status())
	}
	if m := New(Procurement, cfg, quiet()); m.Enabled() || m.Status() != StatusMisconfigured {
		t.Fatalf("procurement 選了圖片模型：%v", m.Status())
	}
	if m := New(ImageAI, Config{GeminiAPIKey: "k"}, quiet()); m.Enabled() || m.Status() != StatusMisconfigured || m.Name() != "" {
		t.Fatalf("New 不得為 ImageAI 建 ADK model：%+v", m)
	}
	if m := NewImage(cfg, quiet()); m.Enabled() || m.Status() != StatusMisconfigured {
		t.Fatalf("imageai 選了文字模型：%v", m.Status())
	}
}

func TestImageDefaultsAndDisabledStates(t *testing.T) {
	if m := NewImage(Config{GeminiAPIKey: "k"}, quiet()); !m.Enabled() || m.Name() != GeminiImage {
		t.Fatalf("預設模型 = %+v", m)
	}
	if m := New(Assistant, Config{GeminiAPIKey: "k"}, quiet()); m.Name() != DefaultModel {
		t.Fatalf("assistant 預設模型被改了：%q", m.Name())
	}
	for name, tc := range map[string]struct {
		cfg  Config
		want Status
	}{
		"default without key":  {Config{}, StatusNotConfigured},
		"explicit without key": {Config{Models: map[Role]string{ImageAI: GeminiImage}}, StatusMisconfigured},
	} {
		m := NewImage(tc.cfg, quiet())
		if m.Enabled() || m.Status() != tc.want || m.Generator() == nil || m.Name() != "" {
			t.Errorf("%s：%+v", name, m)
			continue
		}
		_, err := m.Generator().GenerateImage(context.Background(), ImageRequest{})
		if !errors.Is(err, ErrImageDisabled) || apperr.KindOf(err) != apperr.KindUnavailable {
			t.Errorf("%s：停用的圖片模型回 %v", name, err)
		}
	}
}

func TestReadConfigImageModel(t *testing.T) {
	cfg := ReadConfig(config.FromMap(map[string]string{"LLM_IMAGEAI_MODEL": GeminiImage}))
	if cfg.Models[ImageAI] != GeminiImage {
		t.Fatalf("cfg = %+v", cfg)
	}
}

type hangingGenerator struct {
	gotDeadline chan bool
}

func (h hangingGenerator) GenerateImage(ctx context.Context, _ ImageRequest) (ImageResult, error) {
	_, ok := ctx.Deadline()
	h.gotDeadline <- ok
	<-ctx.Done()
	return ImageResult{}, ctx.Err()
}

func TestImageGeneratorIsBoundedByADeadline(t *testing.T) {
	g := hangingGenerator{gotDeadline: make(chan bool, 1)}
	m := NewImageModel(g, "fake").withTimeout(20 * time.Millisecond)
	done := make(chan error, 1)
	go func() {
		_, err := m.Generator().GenerateImage(context.Background(), ImageRequest{})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("generator was not bounded by a deadline")
	}
	if !<-g.gotDeadline {
		t.Fatal("ctx passed to the generator has no deadline")
	}
}

func TestImageGeneratorDefaultDeadlineIsImageCallTimeout(t *testing.T) {
	b, ok := NewImageModel(hangingGenerator{}, "fake").Generator().(boundedImage)
	if !ok || b.timeout != imageCallTimeout {
		t.Fatalf("generator = %+v", b)
	}
}

func TestDisabledImageIsDisabledNotNotConfigured(t *testing.T) {
	m := DisabledImage()
	if m.Enabled() || m.Status() != StatusDisabled {
		t.Fatalf("status = %v", m.Status())
	}
}
