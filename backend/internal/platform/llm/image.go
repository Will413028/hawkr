package llm

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// ErrImageDisabled 是圖片模型尚未開通時呼叫端得到的錯誤。
var ErrImageDisabled = apperr.Unavailable("AI 修圖尚未開通，請稍後再試")

// imageCallTimeout 是一次圖片生成呼叫的 deadline：呼叫端的 ctx 沒有更短的 deadline 時用它。
const imageCallTimeout = 60 * time.Second

// ImageRequest 是一次修圖：一張原圖加一段提示詞。
type ImageRequest struct {
	Image    []byte
	MimeType string
	Prompt   string
}

// ImageResult 是模型的回覆。沒有產出圖片時 Image 為空，原因在 BlockReason（prompt 被擋）或 FinishReason（候選被擋或提早結束）；
// 兩者都是 provider 的原字串，由呼叫端分類。
type ImageResult struct {
	Image        []byte
	MimeType     string
	FinishReason string
	BlockReason  string
}

// ImageGenerator 是圖片模型的窄介面：ADK 的 model.LLM 在 prompt 被擋、沒有候選時只回「empty response」錯誤，丟掉 BlockReason，
// 所以圖片角色不經 ADK，直接用 genai client，並只暴露這一個方法。
type ImageGenerator interface {
	GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, error)
}

// ImageModel 是一個已決定開通與否的圖片模型。零值不可用，由 NewImage 建構。
type ImageModel struct {
	gen    ImageGenerator
	name   string
	status Status
}

// Generator 永遠非 nil：停用時是回 ErrImageDisabled 的實作。
func (m ImageModel) Generator() ImageGenerator { return m.gen }

// Name 是開通的模型名，停用時為空字串。
func (m ImageModel) Name() string { return m.name }

func (m ImageModel) Status() Status { return m.status }
func (m ImageModel) Enabled() bool  { return m.status == StatusEnabled }

// NewImage 決定 ImageAI 角色的開通狀態並建構 client。永遠不回 error。
func NewImage(cfg Config, log *slog.Logger) ImageModel {
	return buildImage(cfg, log, "")
}

// NewImageModel 以給定的 generator 組出已開通的 ImageModel，讓測試用替身走正式路徑；正式程式只經 NewImage。
func NewImageModel(g ImageGenerator, name string) ImageModel {
	return ImageModel{gen: g, name: name, status: StatusEnabled}
}

func disabledImage(status Status) ImageModel {
	return ImageModel{gen: disabledImageGenerator{}, status: status}
}

// buildImage 是圖片 client 唯一的建構點；baseURLOverride 只給測試把 genai 指到替身 server。
func buildImage(cfg Config, log *slog.Logger, baseURLOverride string) ImageModel {
	name, _, key, status := resolve(ImageAI, cfg, log)
	if status != StatusEnabled {
		return disabledImage(status)
	}
	cc := &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI}
	if baseURLOverride != "" {
		cc.HTTPOptions = genai.HTTPOptions{BaseURL: baseURLOverride}
	}
	client, err := genai.NewClient(context.Background(), cc)
	if err != nil {
		log.Error("LLM client 建立失敗，已停用", "role", ImageAI, "model", name, "error", err)
		return disabledImage(StatusMisconfigured)
	}
	log.Info("LLM 已開通", "role", ImageAI, "model", name, "provider", Gemini)
	return ImageModel{gen: genaiImage{client: client, name: name}, name: name, status: StatusEnabled}
}

type genaiImage struct {
	client *genai.Client
	name   string
}

func (g genaiImage) GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, error) {
	ctx, cancel := context.WithTimeout(ctx, imageCallTimeout)
	defer cancel()
	contents := []*genai.Content{genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromBytes(req.Image, req.MimeType),
		genai.NewPartFromText(req.Prompt),
	}, genai.RoleUser)}
	resp, err := g.client.Models.GenerateContent(ctx, g.name, contents, &genai.GenerateContentConfig{ResponseModalities: []string{"IMAGE"}})
	if err != nil {
		return ImageResult{}, err
	}
	return imageResult(resp), nil
}

func imageResult(resp *genai.GenerateContentResponse) ImageResult {
	var out ImageResult
	if resp.PromptFeedback != nil {
		out.BlockReason = string(resp.PromptFeedback.BlockReason)
	}
	if len(resp.Candidates) == 0 {
		return out
	}
	c := resp.Candidates[0]
	out.FinishReason = string(c.FinishReason)
	if c.Content == nil {
		return out
	}
	for _, p := range c.Content.Parts {
		if p != nil && p.InlineData != nil && len(p.InlineData.Data) > 0 && strings.HasPrefix(p.InlineData.MIMEType, "image/") {
			out.Image, out.MimeType = p.InlineData.Data, p.InlineData.MIMEType
			break
		}
	}
	return out
}

type disabledImageGenerator struct{}

func (disabledImageGenerator) GenerateImage(context.Context, ImageRequest) (ImageResult, error) {
	return ImageResult{}, ErrImageDisabled
}
