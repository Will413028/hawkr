package image_test

import (
	"bytes"
	"errors"
	"hash/crc32"
	stdimage "image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/image"
)

const gpsMarker = "GPS-25.0330N-121.5654E-SERIAL-A1B2C3"

func fixtureImage() *stdimage.RGBA {
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 16), B: 0x40, A: 0xFF})
		}
	}
	return img
}

func encodeJPEG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, fixtureImage(), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, fixtureImage()); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodeGIF(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{color.Black, color.White, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	p := stdimage.NewPaletted(stdimage.Rect(0, 0, 16, 16), pal)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			p.SetColorIndex(x, y, uint8((x+y)%4))
		}
	}
	var b bytes.Buffer
	if err := gif.Encode(&b, p, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func contains(d []byte, s string) bool { return bytes.Contains(d, []byte(s)) }

// exifTIFF 是含 Orientation、GPS IFD（指標指向 ASCII 欄位）與 IFD1 的小端 TIFF，內文帶有可搜尋的敏感字串。
func exifTIFF(orientation int) []byte {
	marker := append([]byte(gpsMarker), 0)
	var o bytes.Buffer
	o.Write([]byte{'I', 'I', 42, 0, 8, 0, 0, 0})
	o.Write([]byte{2, 0})
	o.Write([]byte{0x12, 0x01, 3, 0, 1, 0, 0, 0, byte(orientation), 0, 0, 0})
	o.Write([]byte{0x25, 0x88, 4, 0, 1, 0, 0, 0, 38, 0, 0, 0})
	o.Write([]byte{0, 0, 0, 0})
	o.Write([]byte{1, 0})
	o.Write([]byte{0x01, 0x00, 2, 0, byte(len(marker)), 0, 0, 0, 56, 0, 0, 0})
	o.Write([]byte{0, 0, 0, 0})
	o.Write(marker)
	return o.Bytes()
}

func jpegSegment(marker byte, payload []byte) []byte {
	n := len(payload) + 2
	return cat([]byte{0xFF, marker, byte(n >> 8), byte(n)}, payload)
}

func jpegWithMetadata(t *testing.T, orientation int) []byte {
	t.Helper()
	plain := encodeJPEG(t)
	exif := jpegSegment(0xE1, cat([]byte("Exif\x00\x00"), exifTIFF(orientation)))
	xmp := jpegSegment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>"+gpsMarker+"</x:xmpmeta>"))
	iptc := jpegSegment(0xED, []byte("Photoshop 3.0\x00"+gpsMarker))
	com := jpegSegment(0xFE, []byte(gpsMarker))
	trailer := []byte("TRAILER" + gpsMarker)
	insertAt := 2
	if plain[2] == 0xFF && plain[3] == 0xE0 {
		insertAt = 4 + (int(plain[4])<<8 | int(plain[5]))
	}
	return cat(plain[:insertAt], exif, xmp, iptc, com, plain[insertAt:], trailer)
}

// orientationAt 直接讀固定位移取出最小 TIFF 的 Orientation，不經過被測的解析函式。
func orientationAt(t *testing.T, out []byte, tiffAt int) int {
	t.Helper()
	// 最小 TIFF：'MM' 0 42 0 0 0 8, count=1, tag 0x0112 在位移 10，值在位移 18。
	if !bytes.Equal(out[tiffAt:tiffAt+10], []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1}) ||
		!bytes.Equal(out[tiffAt+10:tiffAt+12], []byte{0x01, 0x12}) {
		t.Fatalf("不是只含 Orientation 的最小 TIFF：% x", out[tiffAt:tiffAt+26])
	}
	return int(out[tiffAt+19])
}

func samePixels(t *testing.T, a, b stdimage.Image) {
	t.Helper()
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				t.Fatalf("像素 (%d,%d) 不同", x, y)
			}
		}
	}
}

func mustStrip(t *testing.T, in []byte) []byte {
	t.Helper()
	out, _, err := image.Strip(in)
	if err != nil {
		t.Fatalf("Strip：%v", err)
	}
	return out
}

func TestJPEGIsStrippedLosslessly(t *testing.T) {
	in := jpegWithMetadata(t, 1)
	if !contains(in, gpsMarker) {
		t.Fatal("fixture 必須含敏感字串")
	}
	out := mustStrip(t, in)
	for _, s := range []string{gpsMarker, "Exif", "xmp", "Photoshop", "TRAILER"} {
		if contains(out, s) {
			t.Fatalf("輸出仍含 %q", s)
		}
	}
	a, err := jpeg.Decode(bytes.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	b, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("輸出不可解碼：%v", err)
	}
	samePixels(t, a, b)
}

func TestJPEGKeepsOnlyOrientation(t *testing.T) {
	out := mustStrip(t, jpegWithMetadata(t, 6))
	if contains(out, gpsMarker) {
		t.Fatal("輸出帶了 GPS")
	}
	at := bytes.Index(out, []byte("Exif\x00\x00"))
	if at <= 0 {
		t.Fatal("方向大於 1 時要保留 EXIF 區段")
	}
	if got := orientationAt(t, out, at+6); got != 6 {
		t.Fatalf("方向 = %d，預期 6", got)
	}
	if n := bytes.Count(out, []byte("Exif\x00\x00")); n != 1 {
		t.Fatalf("EXIF 區段 %d 個，預期 1", n)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("輸出不可解碼：%v", err)
	}
}

func TestJPEGWithDefaultOrientationHasNoExif(t *testing.T) {
	if out := mustStrip(t, jpegWithMetadata(t, 1)); contains(out, "Exif") {
		t.Fatal("方向 1 時不該有 EXIF")
	}
}

func TestTruncatedJPEGIsRejected(t *testing.T) {
	in := jpegWithMetadata(t, 1)
	_, _, err := image.Strip(in[:len(in)/2])
	if !errors.Is(err, image.ErrMalformed) {
		t.Fatalf("err = %v，預期 ErrMalformed", err)
	}
}

func pngChunk(typ string, data []byte) []byte {
	var b bytes.Buffer
	b.Write([]byte{byte(len(data) >> 24), byte(len(data) >> 16), byte(len(data) >> 8), byte(len(data))})
	td := append([]byte(typ), data...)
	b.Write(td)
	c := crc32.ChecksumIEEE(td)
	b.Write([]byte{byte(c >> 24), byte(c >> 16), byte(c >> 8), byte(c)})
	return b.Bytes()
}

func TestPNGIsStripped(t *testing.T) {
	plain := encodePNG(t)
	afterIHDR := 8 + 12 + 13
	withMeta := cat(plain[:afterIHDR],
		pngChunk("tEXt", []byte("Comment\x00"+gpsMarker)),
		pngChunk("iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"+gpsMarker)),
		pngChunk("tIME", []byte{7, 0xEA, 1, 1, 1, 1, 1}),
		pngChunk("eXIf", exifTIFF(8)),
		plain[afterIHDR:])
	if !contains(withMeta, gpsMarker) {
		t.Fatal("fixture 必須含敏感字串")
	}
	out := mustStrip(t, withMeta)
	for _, s := range []string{gpsMarker, "tEXt", "iTXt", "tIME"} {
		if contains(out, s) {
			t.Fatalf("輸出仍含 %q", s)
		}
	}
	at := bytes.Index(out, []byte("eXIf"))
	if at <= 0 || at >= bytes.Index(out, []byte("IDAT")) {
		t.Fatal("最小 eXIf 要在 IDAT 之前")
	}
	if got := orientationAt(t, out, at+4); got != 8 {
		t.Fatalf("方向 = %d，預期 8", got)
	}
	want := crc32.ChecksumIEEE(out[at : at+4+26])
	stored := uint32(out[at+30])<<24 | uint32(out[at+31])<<16 | uint32(out[at+32])<<8 | uint32(out[at+33])
	if want != stored {
		t.Fatalf("CRC %08x 與儲存的 %08x 不符", want, stored)
	}
	a, err := png.Decode(bytes.NewReader(plain))
	if err != nil {
		t.Fatal(err)
	}
	b, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("輸出不可解碼：%v", err)
	}
	samePixels(t, a, b)
}

func le32(v int) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }

func riffChunk(typ string, data []byte) []byte {
	var pad []byte
	if len(data)%2 == 1 {
		pad = []byte{0}
	}
	return cat([]byte(typ), le32(len(data)), data, pad)
}

func webp(chunks ...[]byte) []byte {
	body := cat(chunks...)
	return cat([]byte("RIFF"), le32(len(body)+4), []byte("WEBP"), body)
}

func TestWebPIsStripped(t *testing.T) {
	vp8x := make([]byte, 10)
	vp8x[0] = 0x08 | 0x04 | 0x20
	img := []byte{1, 2, 3, 4, 5}
	in := webp(riffChunk("VP8X", vp8x), riffChunk("VP8 ", img), riffChunk("EXIF", exifTIFF(3)),
		riffChunk("XMP ", []byte("<x>"+gpsMarker+"</x>")))
	out := mustStrip(t, in)
	if contains(out, gpsMarker) || contains(out, "XMP ") {
		t.Fatal("輸出仍含 GPS 或 XMP")
	}
	if string(out[:4]) != "RIFF" {
		t.Fatal("缺 RIFF 標頭")
	}
	if got := int(out[4]) | int(out[5])<<8 | int(out[6])<<16; got != len(out)-8 {
		t.Fatalf("RIFF 大小 %d，預期 %d", got, len(out)-8)
	}
	if out[20] != 0x08|0x20 {
		t.Fatalf("VP8X 旗標 %#x，預期 XMP 清掉、EXIF 因保留方向而在、ICC 不動", out[20])
	}
	at := bytes.Index(out, []byte("EXIF"))
	if at <= 0 || out[at+4] != 26 {
		t.Fatal("要有 26 位元組的最小 EXIF 區塊")
	}
	if got := orientationAt(t, out, at+8); got != 3 {
		t.Fatalf("方向 = %d，預期 3", got)
	}
	if !contains(out, string(riffChunk("VP8 ", img))) {
		t.Fatal("影像區塊必須逐位元組不變")
	}
}

func TestWebPWithDefaultOrientationHasNoExif(t *testing.T) {
	vp8x := make([]byte, 10)
	vp8x[0] = 0x08
	out := mustStrip(t, webp(riffChunk("VP8X", vp8x), riffChunk("VP8 ", []byte{9, 9}), riffChunk("EXIF", exifTIFF(1))))
	if contains(out, "EXIF") {
		t.Fatal("方向 1 時不該留 EXIF")
	}
	if out[20] != 0 {
		t.Fatalf("旗標 %#x，預期 0", out[20])
	}
}

func TestGIFIsStripped(t *testing.T) {
	plain := encodeGIF(t)
	i := 13
	if flags := plain[10]; flags&0x80 != 0 {
		i += 3 * (1 << ((flags & 0x07) + 1))
	}
	marker := []byte(gpsMarker)
	comment := cat([]byte{0x21, 0xFE, byte(len(marker))}, marker, []byte{0})
	xmp := cat([]byte{0x21, 0xFF, 11}, []byte("XMP DataXMP"), []byte{byte(len(marker))}, marker, []byte{0})
	in := cat(plain[:i], comment, xmp, plain[i:])
	out := mustStrip(t, in)
	if contains(out, gpsMarker) || contains(out, "XMP Data") {
		t.Fatal("輸出仍含 GPS 或 XMP")
	}
	a, err := gif.Decode(bytes.NewReader(plain))
	if err != nil {
		t.Fatal(err)
	}
	b, err := gif.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("輸出不可解碼：%v", err)
	}
	samePixels(t, a, b)
	if !bytes.Equal(plain, mustStrip(t, plain)) {
		t.Fatal("沒有 metadata 的 GIF 必須逐位元組不變")
	}
}

func TestGIFKeepsAnimationExtension(t *testing.T) {
	plain := encodeGIF(t)
	i := 13
	if flags := plain[10]; flags&0x80 != 0 {
		i += 3 * (1 << ((flags & 0x07) + 1))
	}
	netscape := cat([]byte{0x21, 0xFF, 11}, []byte("NETSCAPE2.0"), []byte{3, 1, 0, 0, 0})
	in := cat(plain[:i], netscape, plain[i:])
	if out := mustStrip(t, in); !bytes.Equal(out, in) {
		t.Fatal("動畫擴充必須保留")
	}
}

func TestUnknownFormatIsRejected(t *testing.T) {
	for _, in := range [][]byte{[]byte("%PDF-1.7"), nil, {}, []byte("fake image bytes")} {
		if _, _, err := image.Strip(in); !errors.Is(err, image.ErrUnrecognized) {
			t.Fatalf("Strip(%q) err = %v，預期 ErrUnrecognized", in, err)
		}
	}
}

// 假圖片（內容不是圖片）必須被拒，不能原樣寫進 public/。
func TestFakeImageIsRejected(t *testing.T) {
	fake := []byte("<html><script>alert(1)</script></html> " + gpsMarker)
	out, _, err := image.Strip(fake)
	if !errors.Is(err, image.ErrUnrecognized) || out != nil {
		t.Fatalf("out=%v err=%v，預期無輸出與 ErrUnrecognized", out, err)
	}
}

func TestDetectEachFormat(t *testing.T) {
	cases := map[image.Format][]byte{
		image.JPEG: encodeJPEG(t), image.PNG: encodePNG(t), image.GIF: encodeGIF(t),
		image.WebP: webp(riffChunk("VP8 ", []byte{1, 2})),
	}
	for want, in := range cases {
		got, err := image.Detect(in)
		if err != nil || got != want || got.ContentType() == "" {
			t.Fatalf("Detect = %q, %v，預期 %q", got, err, want)
		}
	}
}

func TestMalformedStructuresAreRejected(t *testing.T) {
	png := encodePNG(t)
	cases := map[string][]byte{
		"PNG 缺 IEND":     png[:len(png)-12],
		"PNG 第一塊不是 IHDR": cat(png[:8], pngChunk("tEXt", []byte("a")), png[8:]),
		"GIF 截斷":         encodeGIF(t)[:20],
		"WebP 區塊長度超出":    cat([]byte("RIFF"), le32(20), []byte("WEBP"), []byte("VP8 "), le32(100), []byte{1, 2, 3, 4}),
		"WebP VP8X 太短":   webp(riffChunk("VP8X", []byte{1, 2})),
		"JPEG 區段長度錯":     {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x01},
	}
	for name, in := range cases {
		if _, _, err := image.Strip(in); !errors.Is(err, image.ErrMalformed) {
			t.Errorf("%s：err = %v，預期 ErrMalformed", name, err)
		}
	}
}

func TestOrientationEdgeCasesFallBackToNoRotation(t *testing.T) {
	// 方向值 0、9、超出範圍的 IFD 偏移，都當作 1：輸出沒有 EXIF。
	for _, o := range []int{0, 1, 9} {
		seg := jpegSegment(0xE1, cat([]byte("Exif\x00\x00"), exifTIFF(o)))
		plain := encodeJPEG(t)
		in := cat(plain[:2], seg, plain[2:])
		if out := mustStrip(t, in); contains(out, "Exif") {
			t.Fatalf("方向 %d 不該保留 EXIF", o)
		}
	}
	badTIFF := []byte{'I', 'I', 42, 0, 0xFF, 0xFF, 0, 0, 0, 0, 0, 0}
	plain := encodeJPEG(t)
	in := cat(plain[:2], jpegSegment(0xE1, cat([]byte("Exif\x00\x00"), badTIFF)), plain[2:])
	if out := mustStrip(t, in); contains(out, "Exif") {
		t.Fatal("壞掉的 TIFF 不該保留 EXIF")
	}
}

func TestBigEndianOrientationIsRead(t *testing.T) {
	be := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 5, 0, 0, 0, 0, 0, 0}
	plain := encodeJPEG(t)
	in := cat(plain[:2], jpegSegment(0xE1, cat([]byte("Exif\x00\x00"), be)), plain[2:])
	out := mustStrip(t, in)
	at := bytes.Index(out, []byte("Exif\x00\x00"))
	if at < 0 || orientationAt(t, out, at+6) != 5 {
		t.Fatal("大端 TIFF 的方向 5 要保留")
	}
}
