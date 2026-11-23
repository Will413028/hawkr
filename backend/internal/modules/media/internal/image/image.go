// Package image 辨認 JPEG、PNG、WebP、GIF，並以 libvips 重新編碼：解碼後套用方向、轉成 sRGB、丟掉全部 metadata 再編碼。
//
// 重新編碼讓輸出只含像素，不依賴逐位元組解析各容器的區段結構；代價是有損（JPEG 品質 90）與需要系統的 libvips。
// 輸出：靜態不透明 → JPEG；有透明 → PNG；動態 → 原格式的動態輸出（GIF、WebP）。
// 規格見 docs/adr/2026-11-22-media-public-image-reencode.md。
package image

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/cshum/vipsgen/vips"
)

// Format 是輸入或輸出的圖片格式。
type Format string

const (
	JPEG Format = "jpeg"
	PNG  Format = "png"
	WebP Format = "webp"
	GIF  Format = "gif"
)

// ContentType 回傳格式對應的 MIME 型別。
func (f Format) ContentType() string {
	switch f {
	case JPEG:
		return "image/jpeg"
	case PNG:
		return "image/png"
	case WebP:
		return "image/webp"
	case GIF:
		return "image/gif"
	}
	return ""
}

// ErrUnrecognized 表示開頭的位元組不是四種支援的格式（HEIC 等也在此）。
var ErrUnrecognized = errors.New("image: unrecognized format")

// ErrMalformed 表示辨認得出格式但無法解碼；呼叫方必須拒絕，不可退回原檔。
var ErrMalformed = errors.New("image: malformed data")

// ErrProcessing 表示輸入以外的永久失敗：管線遇到本不該出現的狀態（例如無法判斷的 alpha 格式），重試不會變好，也不是輸入的格式問題。
var ErrProcessing = errors.New("image: processing failed")

// ErrTooLarge 表示像素數超過 model 定義的上限；在解碼像素之前就回傳。
var ErrTooLarge = errors.New("image: pixel count over limit")

// jpegQuality 是靜態不透明圖輸出 JPEG 的品質。
const jpegQuality = 90

// webpQuality 是動態 WebP 輸出的品質。
const webpQuality = 90

var startOnce sync.Once

// startVips 只初始化一次 libvips，可被並行的處理 job 同時呼叫。
// libvips 內部執行緒設為 1：並行度來自同時跑的處理 job，不讓單一 job 再各自展開執行緒池；
// 運算快取全關：每個上傳的內容都不同，快取不會命中，只會多佔記憶體。
func startVips() {
	startOnce.Do(func() {
		vips.Startup(&vips.Config{ConcurrencyLevel: 1, MaxCacheFiles: 0, MaxCacheMem: 0, MaxCacheSize: 0})
	})
}

// Detect 依 magic bytes 辨認格式；辨認不出回 ErrUnrecognized。
func Detect(d []byte) (Format, error) {
	switch {
	case len(d) >= 3 && d[0] == 0xFF && d[1] == 0xD8 && d[2] == 0xFF:
		return JPEG, nil
	case bytes.HasPrefix(d, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return PNG, nil
	case len(d) >= 12 && string(d[0:4]) == "RIFF" && string(d[8:12]) == "WEBP":
		return WebP, nil
	case len(d) >= 6 && (string(d[0:6]) == "GIF87a" || string(d[0:6]) == "GIF89a"):
		return GIF, nil
	}
	return "", ErrUnrecognized
}

// Result 是 Sanitize 的輸出：重新編碼後的位元組與實際輸出格式。
type Result struct {
	Data   []byte
	Format Format
}

// Limits 是解碼前的像素上限，值由呼叫方從 model 的單一定義傳入；三個欄位都必須為正，否則 Sanitize 回錯誤、不處理任何輸入。
type Limits struct {
	// FramePixels 是單一影格的像素上限。
	FramePixels int64
	// AnimatedPixels 是動態圖「影格數 x 每格像素」的上限。
	AnimatedPixels int64
	// MaxDimension 是單邊像素上限（寬或高），套用在所有輸出。
	MaxDimension int
}

// MaxAnimatedWebPDimension 是動態 WebP 單邊像素的編碼上限（WebP 格式的 14 位元尺寸欄位）；超過的輸入在標頭階段永久拒收，不進編碼。
const MaxAnimatedWebPDimension = 16383

// ErrInvalidLimits 表示 Limits 有非正的欄位：零值會讓每個輸入都被當成超過上限，那是設定錯誤，不是輸入的問題。
var ErrInvalidLimits = errors.New("image: limits must be positive")

func (l Limits) validate() error {
	if l.FramePixels <= 0 || l.AnimatedPixels <= 0 || l.MaxDimension <= 0 {
		return fmt.Errorf("%w: frame=%d animated=%d dimension=%d", ErrInvalidLimits, l.FramePixels, l.AnimatedPixels, l.MaxDimension)
	}
	return nil
}

// Sanitize 先檢查 Limits，再辨認格式、讀標頭檢查像素上限與單邊上限，接著完整解碼、套用方向（靜態圖）、轉 sRGB、
// 判斷有無透明、丟掉全部 metadata 後重新編碼。
// Limits 有非正的欄位回 ErrInvalidLimits；辨認不出（含 APNG）回 ErrUnrecognized；輸入的載入、標頭或像素解碼失敗回 ErrMalformed；
// 超過像素或單邊上限回 ErrTooLarge；
// 無法判斷 alpha 的像素格式與沒有動態編碼器的格式回 ErrProcessing（永久、非輸入問題）；
// 轉色、套用方向與編碼的失敗是輸入以外的原因，回一般錯誤（呼叫方當暫時失敗重試）。
// ctx 在載入前與每個耗時步驟之間檢查，取消回 ctx 的錯誤；libvips 的單一運算跑起來後 vipsgen 沒有中止的介面，不可取消。
func Sanitize(ctx context.Context, d []byte, lim Limits) (Result, error) {
	return sanitize(ctx, d, lim, nil)
}

// sanitize 的 fail 只給測試用：在指定階段（stageColour、stageAutorot、stageAlpha、stageFlatten、stageEncode）注入失敗。
func sanitize(ctx context.Context, d []byte, lim Limits, fail func(stage string) error) (Result, error) {
	if err := lim.validate(); err != nil {
		return Result{}, err
	}
	if err := live(ctx); err != nil {
		return Result{}, err
	}
	f, err := Detect(d)
	if err != nil {
		return Result{}, err
	}
	if f == PNG && isAPNG(d) {
		return Result{}, fmt.Errorf("%w: animated PNG", ErrUnrecognized)
	}
	startVips()
	img, err := load(d, f)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	defer img.Close()

	pages := max(img.Pages(), 1)
	animated := pages > 1
	if err := checkPixels(img.Width(), pageHeight(img, pages), pages, f, lim); err != nil {
		return Result{}, err
	}
	if err := live(ctx); err != nil {
		return Result{}, err
	}
	// 標頭通過上限後才解碼全部像素；之後的失敗都不再是輸入的問題。
	if _, err := img.Avg(); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if err := live(ctx); err != nil {
		return Result{}, err
	}
	s := stages{ctx: ctx, fail: fail}
	if animated {
		return s.encodeAnimated(img, f)
	}
	return s.encodeStill(img)
}

func live(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("image: %w", err)
	}
	return nil
}

const (
	stageColour  = "colour"
	stageAutorot = "autorot"
	stageAlpha   = "alpha"
	stageFlatten = "flatten"
	stageEncode  = "encode"
)

type stages struct {
	ctx  context.Context
	fail func(stage string) error
}

// step 先檢查 ctx，再（測試注入時）回注入的失敗，否則執行 fn；失敗一律是一般錯誤。
func (s stages) step(stage string, fn func() error) error {
	if err := live(s.ctx); err != nil {
		return err
	}
	var err error
	if s.fail != nil {
		err = s.fail(stage)
	}
	if err == nil {
		err = fn()
	}
	if err != nil {
		return fmt.Errorf("image: %s: %w", stage, err)
	}
	return nil
}

// isAPNG 在 IDAT 之前找 acTL 區段；libvips 只讀 APNG 的第一格，放行會靜默丟掉動畫。
func isAPNG(d []byte) bool {
	p := 8
	for p+8 <= len(d) {
		n := int(binary.BigEndian.Uint32(d[p:]))
		typ := string(d[p+4 : p+8])
		switch typ {
		case "acTL":
			return true
		case "IDAT", "IEND":
			return false
		}
		next := p + 12 + n
		if n < 0 || next <= p || next > len(d) {
			return false
		}
		p = next
	}
	return false
}

func load(d []byte, f Format) (*vips.Image, error) {
	switch f {
	case JPEG:
		return vips.NewJpegloadBuffer(d, &vips.JpegloadBufferOptions{Shrink: 1, FailOn: vips.FailOnError})
	case PNG:
		return vips.NewPngloadBuffer(d, &vips.PngloadBufferOptions{FailOn: vips.FailOnError})
	case WebP:
		return vips.NewWebploadBuffer(d, &vips.WebploadBufferOptions{N: -1, FailOn: vips.FailOnError})
	case GIF:
		return vips.NewGifloadBuffer(d, &vips.GifloadBufferOptions{N: -1, FailOn: vips.FailOnError})
	}
	return nil, ErrUnrecognized
}

func pageHeight(img *vips.Image, pages int) int {
	if pages > 1 {
		return img.PageHeight()
	}
	return img.Height()
}

// checkPixels 以標頭的尺寸檢查：每格像素與（動態時）影格數乘每格像素。尺寸是 int，先以 int64 相乘避免溢位。
func checkPixels(w, h, pages int, f Format, lim Limits) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("%w: empty dimensions", ErrMalformed)
	}
	if w > lim.MaxDimension || h > lim.MaxDimension {
		return fmt.Errorf("%w: %dx%d exceeds %d per side", ErrTooLarge, w, h, lim.MaxDimension)
	}
	if f == WebP && pages > 1 && (w > MaxAnimatedWebPDimension || h > MaxAnimatedWebPDimension) {
		return fmt.Errorf("%w: animated WebP %dx%d exceeds %d per side", ErrTooLarge, w, h, MaxAnimatedWebPDimension)
	}
	frame := int64(w) * int64(h)
	if frame > lim.FramePixels {
		return fmt.Errorf("%w: %d pixels per frame", ErrTooLarge, frame)
	}
	if pages > 1 && frame*int64(pages) > lim.AnimatedPixels {
		return fmt.Errorf("%w: %d frames x %d pixels", ErrTooLarge, pages, frame)
	}
	return nil
}

// toSRGB 把像素轉成 sRGB：有內嵌 ICC 描述檔就依它轉換，之後（含沒有描述檔的灰階、CMYK、16 位元）統一轉成 sRGB 色彩空間。
func (s stages) toSRGB(img *vips.Image) error {
	return s.step(stageColour, func() error {
		if img.HasICCProfile() {
			if err := img.IccTransform("srgb", &vips.IccTransformOptions{Embedded: true}); err != nil {
				return err
			}
		}
		return img.Colourspace(vips.InterpretationSrgb, nil)
	})
}

// hasTransparency 回報影像是否有任何不完全不透明的像素；有 alpha 通道但全部不透明不算。
func hasTransparency(img *vips.Image) (bool, error) {
	if !img.HasAlpha() {
		return false, nil
	}
	a, err := img.Copy(nil)
	if err != nil {
		return false, err
	}
	defer a.Close()
	if err := a.ExtractBand(a.Bands()-1, nil); err != nil {
		return false, err
	}
	m, err := a.Min(nil)
	if err != nil {
		return false, err
	}
	opaque, err := alphaOpaque(a)
	if err != nil {
		return false, err
	}
	return m < opaque, nil
}

// alphaOpaque 是影像格式下完全不透明的 alpha 值（8 位元 255、16 位元 65535）；其他格式回 ErrProcessing，不猜值。
// 四個載入器只產生 8 或 16 位元，轉 sRGB 之後更只剩 8 位元，所以正常輸入到不了這個分支；萬一到了是永久拒絕，不讓它重試到用盡，也不說成使用者的圖片格式不對。
func alphaOpaque(a *vips.Image) (float64, error) {
	if f := a.BandFormat(); f == vips.BandFormatUchar {
		return 255, nil
	} else if f == vips.BandFormatUshort {
		return 65535, nil
	}
	return 0, fmt.Errorf("%w: unsupported alpha band format %d", ErrProcessing, a.BandFormat())
}

func (s stages) encodeStill(img *vips.Image) (Result, error) {
	if err := s.step(stageAutorot, func() error { return img.Autorot(nil) }); err != nil {
		return Result{}, err
	}
	if err := s.toSRGB(img); err != nil {
		return Result{}, err
	}
	var transparent bool
	if err := s.step(stageAlpha, func() (err error) { transparent, err = hasTransparency(img); return }); err != nil {
		return Result{}, err
	}
	if transparent {
		var out []byte
		err := s.step(stageEncode, func() (err error) {
			out, err = img.PngsaveBuffer(&vips.PngsaveBufferOptions{Keep: vips.KeepNone, Compression: 6})
			return
		})
		return Result{Data: out, Format: PNG}, err
	}
	if img.HasAlpha() {
		if err := s.step(stageFlatten, func() error { return img.Flatten(nil) }); err != nil {
			return Result{}, err
		}
	}
	var out []byte
	err := s.step(stageEncode, func() (err error) {
		out, err = img.JpegsaveBuffer(&vips.JpegsaveBufferOptions{Q: jpegQuality, Keep: vips.KeepNone})
		return
	})
	return Result{Data: out, Format: JPEG}, err
}

// encodeAnimated 保留影格與延遲；不套用方向（動態 GIF 與 WebP 的各影格疊成一張高圖，旋轉會打亂影格）。
func (s stages) encodeAnimated(img *vips.Image, f Format) (Result, error) {
	if err := s.toSRGB(img); err != nil {
		return Result{}, err
	}
	if f != GIF && f != WebP {
		return Result{}, fmt.Errorf("%w: %s has no animated encoder", ErrProcessing, f)
	}
	var out []byte
	err := s.step(stageEncode, func() (err error) {
		if f == GIF {
			out, err = img.GifsaveBuffer(&vips.GifsaveBufferOptions{Keep: vips.KeepNone})
		} else {
			out, err = img.WebpsaveBuffer(&vips.WebpsaveBufferOptions{Q: webpQuality, Keep: vips.KeepNone})
		}
		return
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Data: out, Format: f}, nil
}
