package upload

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestSniffSecure_AllowsOnlyPDFAndImages(t *testing.T) {
	allowed := map[string]struct {
		head []byte
		want string
	}{
		"pdf":  {[]byte("%PDF-1.7\n..."), "application/pdf"},
		"png":  {[]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0}, "image/png"},
		"jpeg": {[]byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg"},
		"gif":  {[]byte("GIF89a...."), "image/gif"},
		"webp": {[]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "image/webp"},
	}
	for name, c := range allowed {
		got, err := sniffSecure(c.head)
		if err != nil || got != c.want {
			t.Errorf("%s：得到 %q, %v，預期 %q", name, got, err, c.want)
		}
	}
	rejected := map[string][]byte{
		"empty":      nil,
		"fake-pdf":   []byte("%PD not a pdf"),
		"text":       []byte("hello world"),
		"heic":       append([]byte{0, 0, 0, 0x18}, []byte("ftypheic")...),
		"zip":        {'P', 'K', 3, 4},
		"pdf-inside": []byte("xx%PDF-1.7"),
	}
	for name, head := range rejected {
		if _, err := sniffSecure(head); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 invalid_input", name, err)
		}
	}
}

func TestCapReader_FailsOncePastTheLimit(t *testing.T) {
	const max = 100
	ok := &capReader{r: bytes.NewReader(make([]byte, max)), max: max}
	if n, err := io.Copy(io.Discard, ok); err != nil || n != max {
		t.Fatalf("剛好等於上限：%d, %v", n, err)
	}
	over := &capReader{r: bytes.NewReader(make([]byte, max+1)), max: max}
	if _, err := io.Copy(io.Discard, over); !errors.Is(err, errTooLarge) {
		t.Fatalf("超過上限一個位元組：err = %v", err)
	}
	if got := apperr.KindOf(secureReadErr(errTooLarge)); got != apperr.KindInvalidInput {
		t.Fatalf("超過上限的分類 %v", got)
	}
}
