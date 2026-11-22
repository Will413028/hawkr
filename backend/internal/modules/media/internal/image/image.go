// Package image 辨認 JPEG、PNG、WebP、GIF，並以 libvips 重新編碼：解碼後套用方向、轉成 sRGB、丟掉全部 metadata 再編碼。
//
// 重新編碼讓輸出只含像素，不依賴逐位元組解析各容器的區段結構；代價是有損（JPEG 品質 90）與需要系統的 libvips。
// 輸出：靜態不透明 → JPEG；有透明 → PNG；動態 → 原格式的動態輸出（GIF、WebP）。
// 規格見 docs/adr/2026-11-22-media-public-image-reencode.md。
package image

import (
	"bytes"
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

// ErrTooLarge 表示像素數超過 model 定義的上限；在解碼像素之前就回傳。
var ErrTooLarge = errors.New("image: pixel count over limit")

// jpegQuality 是靜態不透明圖輸出 JPEG 的品質。
const jpegQuality = 90

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

// Limits 是解碼前的像素上限，值由呼叫方從 model 的單一定義傳入。
type Limits struct {
	// FramePixels 是單一影格的像素上限。
	FramePixels int64
	// AnimatedPixels 是動態圖「影格數 x 每格像素」的上限。
	AnimatedPixels int64
}

// Sanitize 先辨認格式、讀標頭檢查像素上限，再解碼、套用方向、轉 sRGB、丟掉全部 metadata 後重新編碼。
// 辨認不出回 ErrUnrecognized，無法解碼回 ErrMalformed，超過像素上限回 ErrTooLarge。
func Sanitize(d []byte, lim Limits) (Result, error) {
	f, err := Detect(d)
	if err != nil {
		return Result{}, err
	}
	startVips()
	img, err := load(d, f)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	defer img.Close()

	pages := max(img.Pages(), 1)
	animated := pages > 1
	if err := checkPixels(img.Width(), pageHeight(img, pages), pages, lim); err != nil {
		return Result{}, err
	}
	if animated {
		return encodeAnimated(img, f)
	}
	return encodeStill(img)
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
func checkPixels(w, h, pages int, lim Limits) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("%w: empty dimensions", ErrMalformed)
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
func toSRGB(img *vips.Image) error {
	if img.HasICCProfile() {
		if err := img.IccTransform("srgb", &vips.IccTransformOptions{Embedded: true}); err != nil {
			return err
		}
	}
	return img.Colourspace(vips.InterpretationSrgb, nil)
}

func encodeStill(img *vips.Image) (Result, error) {
	if err := img.Autorot(nil); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if err := toSRGB(img); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if img.HasAlpha() {
		out, err := img.PngsaveBuffer(&vips.PngsaveBufferOptions{Keep: vips.KeepNone, Compression: 6})
		if err != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
		}
		return Result{Data: out, Format: PNG}, nil
	}
	out, err := img.JpegsaveBuffer(&vips.JpegsaveBufferOptions{Q: jpegQuality, Keep: vips.KeepNone})
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return Result{Data: out, Format: JPEG}, nil
}

// encodeAnimated 保留影格與延遲；不套用方向（動態 GIF 與 WebP 的各影格疊成一張高圖，旋轉會打亂影格）。
func encodeAnimated(img *vips.Image, f Format) (Result, error) {
	if err := toSRGB(img); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	var out []byte
	var err error
	switch f {
	case GIF:
		out, err = img.GifsaveBuffer(&vips.GifsaveBufferOptions{Keep: vips.KeepNone})
	case WebP:
		out, err = img.WebpsaveBuffer(&vips.WebpsaveBufferOptions{Q: jpegQuality, Keep: vips.KeepNone})
	case JPEG, PNG:
		return Result{}, fmt.Errorf("%w: %s has no animated encoder", ErrMalformed, f)
	}
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return Result{Data: out, Format: f}, nil
}
