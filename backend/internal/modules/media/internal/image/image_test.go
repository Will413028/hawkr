package image_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	stdimage "image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/cshum/vipsgen/vips"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/image"
)

const gpsMarker = "GPS-25.0330N-121.5654E-SERIAL-A1B2C3"

// 測試用的小上限：真正的值在 model，image 不依賴 model；這裡只驗「比較」的行為。
var testLimits = image.Limits{FramePixels: 1_000_000, AnimatedPixels: 1_500_000}

func sanitize(d []byte) (image.Result, error) {
	return image.Sanitize(context.Background(), d, testLimits)
}

func solid(w, h int, c color.NRGBA) *stdimage.NRGBA {
	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func pngBytes(t *testing.T, img stdimage.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// exifWithGPS 是含 Orientation 與一個含 gpsMarker 的 GPS IFD 欄位的 EXIF 區段內容（含 "Exif\0\0" 前綴）。
func exifWithGPS(orientation uint16) []byte {
	marker := append([]byte(gpsMarker), 0)
	var tiff bytes.Buffer
	tiff.Write([]byte{'I', 'I', 42, 0, 8, 0, 0, 0})
	le := binary.LittleEndian
	entry := func(tag, typ uint16, count, value uint32) {
		var e [12]byte
		le.PutUint16(e[0:], tag)
		le.PutUint16(e[2:], typ)
		le.PutUint32(e[4:], count)
		le.PutUint32(e[8:], value)
		tiff.Write(e[:])
	}
	tiff.Write([]byte{2, 0})
	entry(0x0112, 3, 1, uint32(orientation))
	entry(0x8825, 4, 1, 38)
	tiff.Write([]byte{0, 0, 0, 0})
	tiff.Write([]byte{1, 0})
	entry(0x0001, 2, uint32(len(marker)), 56)
	tiff.Write([]byte{0, 0, 0, 0})
	tiff.Write(marker)
	return append([]byte("Exif\x00\x00"), tiff.Bytes()...)
}

// withAPP1 在 SOI 之後插入一個 APP1 區段。
func withAPP1(jpg, payload []byte) []byte {
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

func load(t *testing.T, b []byte, f image.Format) *vips.Image {
	t.Helper()
	var img *vips.Image
	var err error
	switch f {
	case image.JPEG:
		img, err = vips.NewJpegloadBuffer(b, nil)
	case image.PNG:
		img, err = vips.NewPngloadBuffer(b, nil)
	case image.WebP:
		img, err = vips.NewWebploadBuffer(b, &vips.WebploadBufferOptions{N: -1})
	case image.GIF:
		img, err = vips.NewGifloadBuffer(b, &vips.GifloadBufferOptions{N: -1})
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(img.Close)
	return img
}

func wantNoMetadata(t *testing.T, img *vips.Image, raw []byte) {
	t.Helper()
	if img.HasICCProfile() {
		t.Error("輸出仍有 ICC 描述檔")
	}
	for _, f := range img.GetFields() {
		l := strings.ToLower(f)
		for _, bad := range []string{"exif", "xmp", "icc", "iptc", "orientation", "gps", "comment"} {
			if strings.Contains(l, bad) {
				t.Errorf("輸出仍有 metadata 欄位 %q", f)
			}
		}
	}
	if bytes.Contains(raw, []byte(gpsMarker)) || bytes.Contains(raw, []byte("Exif\x00\x00")) || bytes.Contains(raw, []byte("XMP Data")) {
		t.Error("輸出位元組仍含 GPS 或 EXIF")
	}
}

// p3JPEG 是 40x20、內嵌 Display P3 描述檔、像素值已換成 P3 數值的 JPEG；來源色是 sRGB c。
func p3JPEG(t *testing.T, c color.NRGBA) []byte {
	t.Helper()
	img, err := vips.NewPngloadBuffer(pngBytes(t, solid(40, 20, c)), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Close()
	if err := img.IccTransform("p3", nil); err != nil {
		t.Fatal(err)
	}
	out, err := img.JpegsaveBuffer(&vips.JpegsaveBufferOptions{Q: 100, Keep: vips.KeepIcc})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSanitizeStripsAllMetadataAndAppliesOrientation(t *testing.T) {
	src := withAPP1(p3JPEG(t, color.NRGBA{200, 60, 60, 255}), exifWithGPS(6))
	if !bytes.Contains(src, []byte(gpsMarker)) {
		t.Fatal("fixture 必須含 GPS 字串")
	}
	srcImg := load(t, src, image.JPEG)
	if !srcImg.HasICCProfile() || srcImg.Orientation() != 6 {
		t.Fatalf("fixture 要有 ICC 與方向 6：icc=%v orientation=%d", srcImg.HasICCProfile(), srcImg.Orientation())
	}
	res, err := sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.JPEG {
		t.Fatalf("format = %s，預期 jpeg", res.Format)
	}
	out := load(t, res.Data, image.JPEG)
	wantNoMetadata(t, out, res.Data)
	if out.Width() != 20 || out.Height() != 40 {
		t.Fatalf("尺寸 %dx%d，預期方向 6 旋正後的 20x40", out.Width(), out.Height())
	}
}

func TestSanitizeConvertsWideGamutToSRGB(t *testing.T) {
	want := color.NRGBA{200, 60, 60, 255}
	src := p3JPEG(t, want)
	rawP3, err := jpeg.Decode(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := rawP3.At(5, 5).RGBA()
	if near(uint8(r>>8), want.R) && near(uint8(g>>8), want.G) && near(uint8(b>>8), want.B) {
		t.Fatal("fixture 的像素值不該已經等於 sRGB 值，否則測不出轉換")
	}
	res, err := sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jpeg.Decode(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ = got.At(5, 5).RGBA()
	if !near(uint8(r>>8), want.R) || !near(uint8(g>>8), want.G) || !near(uint8(b>>8), want.B) {
		t.Fatalf("輸出像素 (%d,%d,%d)，預期接近 sRGB (%d,%d,%d)", r>>8, g>>8, b>>8, want.R, want.G, want.B)
	}
	if load(t, res.Data, image.JPEG).HasICCProfile() {
		t.Fatal("輸出不該帶描述檔")
	}
}

func near(a, b uint8) bool {
	d := int(a) - int(b)
	return d >= -6 && d <= 6
}

func TestSanitizePNGWithAlphaStaysPNG(t *testing.T) {
	src := pngBytes(t, solid(8, 8, color.NRGBA{10, 200, 30, 128}))
	res, err := sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.PNG {
		t.Fatalf("format = %s，預期 png", res.Format)
	}
	if !load(t, res.Data, image.PNG).HasAlpha() {
		t.Fatal("輸出丟了透明通道")
	}
}

func TestFakeImageIsRejected(t *testing.T) {
	for name, d := range map[string][]byte{
		"text":  []byte("<html>not an image</html>"),
		"empty": nil,
		"heic":  append([]byte{0, 0, 0, 0x18}, []byte("ftypheic\x00\x00\x00\x00mif1heic")...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := sanitize(d); !errors.Is(err, image.ErrUnrecognized) {
				t.Fatalf("err = %v，預期 ErrUnrecognized", err)
			}
		})
	}
}

func TestMalformedImageIsRejected(t *testing.T) {
	for name, d := range map[string][]byte{
		"jpeg": append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{7}, 64)...),
		"gif":  []byte("GIF89a\x01\x00"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := sanitize(d); !errors.Is(err, image.ErrMalformed) {
				t.Fatalf("err = %v，預期 ErrMalformed", err)
			}
		})
	}
}

// hugeHeaderPNG 只有標頭宣告的尺寸、沒有像素資料：若上限檢查在解碼之後才做，會得到 ErrMalformed 而不是 ErrTooLarge。
func hugeHeaderPNG(w, h uint32) []byte {
	chunk := func(typ string, data []byte) []byte {
		var b bytes.Buffer
		_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(typ)
		b.Write(data)
		_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), data...)))
		return b.Bytes()
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 0
	out := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	out = append(out, chunk("IHDR", ihdr)...)
	out = append(out, chunk("IDAT", []byte{0x78, 0x9c, 0x03, 0x00, 0x00, 0x00, 0x00, 0x01})...)
	return append(out, chunk("IEND", nil)...)
}

func TestPixelCapIsRejectedFromTheHeader(t *testing.T) {
	_, err := sanitize(hugeHeaderPNG(1001, 1000))
	if !errors.Is(err, image.ErrTooLarge) {
		t.Fatalf("err = %v，預期 ErrTooLarge", err)
	}
}

func TestPixelCapAllowsTheLimitItself(t *testing.T) {
	// 剛好等於上限的尺寸通過檢查（之後因沒有像素資料而是 ErrMalformed，不是 ErrTooLarge）。
	_, err := sanitize(hugeHeaderPNG(1000, 1000))
	if errors.Is(err, image.ErrTooLarge) {
		t.Fatalf("剛好等於上限不該被擋：%v", err)
	}
}

func gifFrames(t *testing.T, w, h, n int) []byte {
	t.Helper()
	pal := color.Palette{color.Black, color.White, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}}
	g := &gif.GIF{}
	for i := range n {
		f := stdimage.NewPaletted(stdimage.Rect(0, 0, w, h), pal)
		for y := range h {
			for x := range w {
				f.SetColorIndex(x, y, uint8((i+x/2)%len(pal)))
			}
		}
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, 5)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestAnimatedGIFKeepsFrameCount(t *testing.T) {
	res, err := sanitize(gifFrames(t, 16, 16, 4))
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.GIF {
		t.Fatalf("format = %s，預期 gif", res.Format)
	}
	if got := load(t, res.Data, image.GIF).Pages(); got != 4 {
		t.Fatalf("影格數 = %d，預期 4", got)
	}
}

func TestAnimatedWebPKeepsFrameCount(t *testing.T) {
	g := load(t, gifFrames(t, 16, 16, 3), image.GIF)
	src, err := g.WebpsaveBuffer(&vips.WebpsaveBufferOptions{Q: 90})
	if err != nil {
		t.Fatal(err)
	}
	if got := load(t, src, image.WebP).Pages(); got != 3 {
		t.Fatalf("fixture 影格數 = %d，預期 3", got)
	}
	res, err := sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.WebP {
		t.Fatalf("format = %s，預期 webp", res.Format)
	}
	if got := load(t, res.Data, image.WebP).Pages(); got != 3 {
		t.Fatalf("影格數 = %d，預期 3", got)
	}
}

func TestAnimatedFramePixelCapIsRejected(t *testing.T) {
	// 單格 1000x1000 = 1 MP 在單格上限內，2 格合計 2 MP 超過動態總量上限 1.5 MP。
	const w, h, n = 1000, 1000, 2
	if int64(w*h) > testLimits.FramePixels || int64(w*h)*n <= testLimits.AnimatedPixels {
		t.Fatal("fixture 與上限不符")
	}
	if _, err := sanitize(gifFrames(t, w, h, n)); !errors.Is(err, image.ErrTooLarge) {
		t.Fatalf("err = %v，預期 ErrTooLarge", err)
	}
}

func TestStaticOpaqueGIFBecomesJPEG(t *testing.T) {
	res, err := sanitize(gifFrames(t, 8, 8, 1))
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.JPEG {
		t.Fatalf("format = %s，預期 jpeg（單格、無透明的 GIF 依靜態不透明處理）", res.Format)
	}
}

func TestContentTypes(t *testing.T) {
	for f, want := range map[image.Format]string{image.JPEG: "image/jpeg", image.PNG: "image/png", image.WebP: "image/webp", image.GIF: "image/gif"} {
		if f.ContentType() != want {
			t.Errorf("%s -> %q", f, f.ContentType())
		}
	}
}

func isPermanentImageErr(err error) bool {
	return errors.Is(err, image.ErrUnrecognized) || errors.Is(err, image.ErrMalformed) || errors.Is(err, image.ErrTooLarge)
}

func TestNonInputFailuresAreNotInputErrors(t *testing.T) {
	src := pngBytes(t, solid(8, 8, color.NRGBA{10, 200, 30, 255}))
	boom := errors.New("boom")
	for _, stage := range []string{image.StageAutorot, image.StageColour, image.StageEncode} {
		t.Run(stage, func(t *testing.T) {
			_, err := image.SanitizeFailing(context.Background(), src, testLimits, stage, boom)
			if err == nil || !errors.Is(err, boom) {
				t.Fatalf("err = %v，預期包著注入的錯誤", err)
			}
			if isPermanentImageErr(err) {
				t.Fatalf("%s 階段的失敗被分類成輸入錯誤（永久失敗）：%v", stage, err)
			}
		})
	}
}

func TestAnimatedEncodeFailureIsNotAnInputError(t *testing.T) {
	boom := errors.New("boom")
	_, err := image.SanitizeFailing(context.Background(), gifFrames(t, 16, 16, 3), testLimits, image.StageEncode, boom)
	if !errors.Is(err, boom) || isPermanentImageErr(err) {
		t.Fatalf("err = %v，預期一般錯誤", err)
	}
}

func TestTruncatedPixelDataIsMalformed(t *testing.T) {
	src := pngBytes(t, solid(64, 64, color.NRGBA{1, 2, 3, 255}))
	_, err := sanitize(src[:len(src)-40])
	if !errors.Is(err, image.ErrMalformed) {
		t.Fatalf("err = %v，預期 ErrMalformed", err)
	}
}

func TestCancelledContextReturnsContextErrorBeforeWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := image.Sanitize(ctx, pngBytes(t, solid(8, 8, color.NRGBA{1, 2, 3, 255})), testLimits)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，預期 context.Canceled", err)
	}
	if isPermanentImageErr(err) {
		t.Fatalf("取消被分類成永久失敗：%v", err)
	}
}

func TestOpaqueAlphaChannelBecomesJPEG(t *testing.T) {
	res, err := sanitize(pngBytes(t, solid(8, 8, color.NRGBA{10, 200, 30, 255})))
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.JPEG {
		t.Fatalf("format = %s，預期 jpeg（有 alpha 通道但全部不透明）", res.Format)
	}
	if load(t, res.Data, image.JPEG).HasAlpha() {
		t.Fatal("輸出仍有 alpha 通道")
	}
}

func TestPartlyTransparentPNGStaysPNG(t *testing.T) {
	img := solid(8, 8, color.NRGBA{10, 200, 30, 255})
	img.SetNRGBA(3, 3, color.NRGBA{10, 200, 30, 0})
	res, err := sanitize(pngBytes(t, img))
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.PNG {
		t.Fatalf("format = %s，預期 png（有一個透明像素）", res.Format)
	}
}

func pngChunk(typ string, data []byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
	b.WriteString(typ)
	b.Write(data)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), data...)))
	return b.Bytes()
}

// insertPNGChunks 把區段插在 IHDR 之後（IDAT 之前）。
func insertPNGChunks(src []byte, chunks ...[]byte) []byte {
	const ihdrEnd = 8 + 12 + 13
	out := append([]byte{}, src[:ihdrEnd]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, src[ihdrEnd:]...)
}

func TestAPNGIsRejectedAsUnrecognized(t *testing.T) {
	src := pngBytes(t, solid(8, 8, color.NRGBA{1, 2, 3, 255}))
	actl := pngChunk("acTL", []byte{0, 0, 0, 2, 0, 0, 0, 0})
	_, err := sanitize(insertPNGChunks(src, actl))
	if !errors.Is(err, image.ErrUnrecognized) {
		t.Fatalf("err = %v，預期 ErrUnrecognized", err)
	}
	if _, err := sanitize(src); err != nil {
		t.Fatalf("一般 PNG 不該被擋：%v", err)
	}
}

func TestTransparentPNGOutputCarriesNoMetadata(t *testing.T) {
	img := solid(8, 8, color.NRGBA{10, 200, 30, 128})
	exif := exifWithGPS(1)[6:]
	xmp := append([]byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"), []byte("<x:xmpmeta>"+gpsMarker+"</x:xmpmeta>")...)
	src := insertPNGChunks(pngBytes(t, img), pngChunk("eXIf", exif), pngChunk("iTXt", xmp))
	if !bytes.Contains(src, []byte(gpsMarker)) {
		t.Fatal("fixture 沒有 GPS 標記")
	}
	res, err := sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.PNG {
		t.Fatalf("format = %s，預期 png", res.Format)
	}
	wantNoMetadata(t, load(t, res.Data, image.PNG), res.Data)
}

// withGIFExtensions 在全域色盤之後插入一個註解與一個 XMP application 延伸區塊。
func withGIFExtensions(t *testing.T, g []byte) []byte {
	t.Helper()
	p := 13
	if g[10]&0x80 != 0 {
		p += 3 << ((g[10] & 7) + 1)
	}
	comment := append([]byte{0x21, 0xFE, byte(len(gpsMarker))}, []byte(gpsMarker)...)
	comment = append(comment, 0)
	xmp := append([]byte{0x21, 0xFF, 11}, []byte("XMP DataXMP")...)
	xmp = append(xmp, byte(len(gpsMarker)))
	xmp = append(xmp, []byte(gpsMarker)...)
	xmp = append(xmp, 0)
	out := append([]byte{}, g[:p]...)
	out = append(out, comment...)
	out = append(out, xmp...)
	return append(out, g[p:]...)
}

func TestAnimatedGIFOutputCarriesNoMetadata(t *testing.T) {
	src := withGIFExtensions(t, gifFrames(t, 16, 16, 3))
	res, err := sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.GIF || load(t, res.Data, image.GIF).Pages() != 3 {
		t.Fatalf("格式或影格數不對：%s", res.Format)
	}
	wantNoMetadata(t, load(t, res.Data, image.GIF), res.Data)
}

// withWebPMetadata 在動態 WebP 加上 EXIF 與 XMP 區段（設 VP8X 旗標並更新 RIFF 長度）。
func withWebPMetadata(t *testing.T, w []byte) []byte {
	t.Helper()
	if string(w[12:16]) != "VP8X" {
		t.Fatal("fixture 不是 VP8X")
	}
	chunk := func(typ string, data []byte) []byte {
		out := append([]byte(typ), 0, 0, 0, 0)
		binary.LittleEndian.PutUint32(out[4:], uint32(len(data)))
		out = append(out, data...)
		if len(data)%2 == 1 {
			out = append(out, 0)
		}
		return out
	}
	out := append([]byte{}, w...)
	out[20] |= 0x08 | 0x04
	out = append(out, chunk("EXIF", exifWithGPS(1)[6:])...)
	out = append(out, chunk("XMP ", []byte("<x:xmpmeta>"+gpsMarker+"</x:xmpmeta>"))...)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	return out
}

func TestAnimatedWebPOutputCarriesNoMetadata(t *testing.T) {
	g := load(t, gifFrames(t, 16, 16, 3), image.GIF)
	plain, err := g.WebpsaveBuffer(&vips.WebpsaveBufferOptions{Q: 90})
	if err != nil {
		t.Fatal(err)
	}
	src := withWebPMetadata(t, plain)
	if !bytes.Contains(src, []byte(gpsMarker)) {
		t.Fatal("fixture 沒有 GPS 標記")
	}
	res, err := sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != image.WebP || load(t, res.Data, image.WebP).Pages() != 3 {
		t.Fatalf("格式或影格數不對：%s", res.Format)
	}
	wantNoMetadata(t, load(t, res.Data, image.WebP), res.Data)
}

// libvips 能解碼、但不在允許清單內的格式必須在辨認階段就被拒絕：回 ErrUnrecognized，且不是解碼失敗的 ErrMalformed。
func TestDecodableButDisallowedFormatIsUnrecognized(t *testing.T) {
	src := load(t, pngBytes(t, solid(8, 8, color.NRGBA{1, 2, 3, 255})), image.PNG)
	tiff, err := src.TiffsaveBuffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if back, err := vips.NewTiffloadBuffer(tiff, nil); err != nil {
		t.Fatalf("fixture 應能被 libvips 解碼：%v", err)
	} else {
		back.Close()
	}
	_, err = sanitize(tiff)
	if !errors.Is(err, image.ErrUnrecognized) || errors.Is(err, image.ErrMalformed) {
		t.Fatalf("err = %v，預期只有 ErrUnrecognized", err)
	}
}
