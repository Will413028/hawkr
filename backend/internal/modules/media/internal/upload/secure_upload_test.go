package upload

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/secure"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
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
		got, err := sniffSecure(t.Context(), c.head)
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
		if _, err := sniffSecure(t.Context(), head); apperr.KindOf(err) != apperr.KindInvalidInput {
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
}

func TestSecureChunkSizeIsAMultipleOfTheAEADSegmentAndFitsTheUploadCeiling(t *testing.T) {
	if model.SecureChunkSize%secure.SegmentSize != 0 {
		t.Fatalf("SecureChunkSize %d 不是 secure.SegmentSize %d 的倍數", model.SecureChunkSize, secure.SegmentSize)
	}
	if model.SecureChunkSize != 4<<20 || model.MaxSecureUploadSize%model.SecureChunkSize != 0 {
		t.Fatalf("SecureChunkSize=%d MaxSecureUploadSize=%d", model.SecureChunkSize, model.MaxSecureUploadSize)
	}
}

func TestSecureChunkCountAndLength(t *testing.T) {
	const c = model.SecureChunkSize
	for _, tc := range []struct {
		size    int64
		count   int
		lengths []int64
	}{
		{1, 1, []int64{1}},
		{c, 1, []int64{c}},
		{c + 1, 2, []int64{c, 1}},
		{2*c + 5, 3, []int64{c, c, 5}},
		{model.MaxSecureUploadSize, 25, nil},
	} {
		if got := secureChunkCount(tc.size); got != tc.count {
			t.Errorf("size=%d：段數 %d，預期 %d", tc.size, got, tc.count)
		}
		var total int64
		for i := range tc.count {
			l := secureChunkLength(tc.size, tc.count, i)
			if l < 1 || l > c || (i < tc.count-1 && l != c) {
				t.Errorf("size=%d 第 %d 段長度 %d", tc.size, i, l)
			}
			if len(tc.lengths) > 0 && l != tc.lengths[i] {
				t.Errorf("size=%d 第 %d 段長度 %d，預期 %d", tc.size, i, l, tc.lengths[i])
			}
			total += l
		}
		if total != tc.size {
			t.Errorf("size=%d：各段加起來 %d", tc.size, total)
		}
	}
}

func TestOwnedBy_BindsDelegatedStore(t *testing.T) {
	admin := model.Uploader{ID: 7, Admin: true}
	delegated := func(store int64) model.Uploader { return model.Uploader{ID: 7, Admin: true, Store: store} }
	row := func(u model.Uploader) store.MediaStagedUpload {
		return store.MediaStagedUpload{UploaderID: u.ID, UploaderAdmin: u.Admin, DelegatedStoreID: delegatedStore(u)}
	}
	cases := []struct {
		name string
		row  store.MediaStagedUpload
		who  model.Uploader
		want bool
	}{
		{"same delegated store", row(delegated(5)), delegated(5), true},
		{"other delegated store", row(delegated(5)), delegated(6), false},
		{"delegated row, no store", row(delegated(5)), admin, false},
		{"plain row, with store", row(admin), delegated(5), false},
		{"plain row, plain admin", row(admin), admin, true},
		{"other admin", row(delegated(5)), model.Uploader{ID: 8, Admin: true, Store: 5}, false},
		{"account with same id", row(delegated(5)), model.Uploader{ID: 7, Store: 5}, false},
	}
	for _, c := range cases {
		if got := ownedBy(c.row, c.who); got != c.want {
			t.Errorf("%s：ownedBy = %v，預期 %v", c.name, got, c.want)
		}
	}
}

func TestCheckUploader_RejectsStoreWithoutAdmin(t *testing.T) {
	for name, tc := range map[string]struct {
		u  model.Uploader
		ok bool
	}{
		"account":             {model.Uploader{ID: 7}, true},
		"admin":               {model.Uploader{ID: 7, Admin: true}, true},
		"delegated admin":     {model.Uploader{ID: 7, Admin: true, Store: 5}, true},
		"store without admin": {model.Uploader{ID: 7, Store: 5}, false},
		"non-positive id":     {model.Uploader{ID: 0}, false},
	} {
		err := checkUploader(t.Context(), tc.u)
		if (err == nil) != tc.ok {
			t.Errorf("%s：err=%v，預期 ok=%v", name, err, tc.ok)
		}
		if err != nil && apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：種類 %v，預期 InvalidInput", name, apperr.KindOf(err))
		}
	}
}

func TestReadStagedSecure_RejectsMalformedUploaderBeforeAnyLookup(t *testing.T) {
	ring, err := secure.ParseKeyring("k1:AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=")
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{d: Deps{Keyring: ring}}
	for name, u := range map[string]model.Uploader{
		"store without admin": {ID: 7, Store: 5},
		"non-positive id":     {ID: 0},
	} {
		_, err := s.ReadStagedSecure(t.Context(), u, "tok")
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v，預期 InvalidInput", name, err)
		}
	}
}
