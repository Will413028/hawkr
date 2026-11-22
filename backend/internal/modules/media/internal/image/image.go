// Package image 辨認 JPEG、PNG、WebP、GIF，並直接改位元組剝除 metadata，只留方向（EXIF Orientation）。
//
// 做法是白名單加逐段搬運：不解碼像素、不重新編碼，所以沒有畫質損失；只留解碼與顯示必需的區段，未知的區段一律丟棄。
// 方向保留時重寫成只含 Orientation 一個欄位的最小 EXIF，原本的 GPS、縮圖、MakerNote 不會被帶過去。
// 規格來源：Java `ImageMetadataStripper`（遷移計畫 B1-2c1）。
package image

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// Format 是辨認出的圖片格式。
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

// ErrUnrecognized 表示開頭的位元組不是四種支援的格式。
var ErrUnrecognized = errors.New("image: unrecognized format")

// ErrMalformed 表示辨認得出格式但結構壞掉；呼叫方必須拒絕，不可退回原檔，否則壞檔就是繞過剝除的路。
var ErrMalformed = errors.New("image: malformed data")

func bad(msg string) error { return fmt.Errorf("%w: %s", ErrMalformed, msg) }

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

// Strip 先辨認格式再剝除 metadata。辨認不出回 ErrUnrecognized，結構壞掉回 ErrMalformed。
func Strip(d []byte) ([]byte, Format, error) {
	f, err := Detect(d)
	if err != nil {
		return nil, "", err
	}
	var out []byte
	switch f {
	case JPEG:
		out, err = stripJPEG(d)
	case PNG:
		out, err = stripPNG(d)
	case WebP:
		out, err = stripWebP(d)
	case GIF:
		out, err = stripGIF(d)
	}
	if err != nil {
		return nil, f, err
	}
	return out, f, nil
}

func u32be(d []byte, i int) uint32 {
	return uint32(d[i])<<24 | uint32(d[i+1])<<16 | uint32(d[i+2])<<8 | uint32(d[i+3])
}

func u32le(d []byte, i int) uint32 {
	return uint32(d[i+3])<<24 | uint32(d[i+2])<<16 | uint32(d[i+1])<<8 | uint32(d[i])
}

func putU32be(b *bytes.Buffer, v uint32) {
	var t [4]byte
	binary.BigEndian.PutUint32(t[:], v)
	b.Write(t[:])
}

func putU32le(b *bytes.Buffer, v uint32) {
	var t [4]byte
	binary.LittleEndian.PutUint32(t[:], v)
	b.Write(t[:])
}

func hasPrefixAt(d []byte, off int, prefix string, available int) bool {
	return available >= len(prefix) && off >= 0 && off+len(prefix) <= len(d) && string(d[off:off+len(prefix)]) == prefix
}
