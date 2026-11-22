//go:build integration

package media_test

import (
	"bytes"
	"crypto/md5" //nolint:gosec // 測試用的 Content-MD5
	"encoding/base64"
	stdimage "image"
	"image/jpeg"
	"io"
	"log/slog"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const geoMarker = "GPS-25.0330N-121.5654E-SERIAL-A1B2C3"

// realModule 換成不注入 Process 的 Module，走模組內建的 public 圖片處理。
func realModule(f *fixture) {
	f.m = media.New(media.Deps{
		DB: f.d, Jobs: f.jobs, Store: f.store, NewID: uuid.NewV7, Now: f.clock.Now,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// jpegWithGPS 是帶 GPS 與方向 6 的 JPEG（EXIF 的 GPS IFD 指向含 geoMarker 的 ASCII 欄位）。
func jpegWithGPS(t *testing.T) []byte {
	t.Helper()
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, stdimage.NewRGBA(stdimage.Rect(0, 0, 8, 4)), nil); err != nil {
		t.Fatal(err)
	}
	marker := append([]byte(geoMarker), 0)
	var tiff bytes.Buffer
	tiff.Write([]byte{'I', 'I', 42, 0, 8, 0, 0, 0, 2, 0})
	tiff.Write([]byte{0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0})
	tiff.Write([]byte{0x25, 0x88, 4, 0, 1, 0, 0, 0, 38, 0, 0, 0})
	tiff.Write([]byte{0, 0, 0, 0, 1, 0})
	tiff.Write([]byte{0x01, 0x00, 2, 0, byte(len(marker)), 0, 0, 0, 56, 0, 0, 0, 0, 0, 0, 0})
	tiff.Write(marker)
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	n := len(payload) + 2
	seg := append([]byte{0xFF, 0xE1, byte(n >> 8), byte(n)}, payload...)
	b := plain.Bytes()
	return bytes.Join([][]byte{b[:2], seg, b[2:]}, nil)
}

func md5b64(b []byte) string {
	s := md5.Sum(b) //nolint:gosec // 同上
	return base64.StdEncoding.EncodeToString(s[:])
}

// processReal 宣告 declared、實際上傳 actual，然後跑處理 job。
func processReal(t *testing.T, f *fixture, declaredSize int64, declaredMD5 string, actual []byte) media.BeginResult {
	t.Helper()
	res, err := f.m.BeginUpload(t.Context(), owner, media.BeginInput{
		Visibility: media.VisibilityPublic, Size: declaredSize, ContentType: "image/jpeg", MD5: declaredMD5,
	})
	if err != nil {
		t.Fatalf("BeginUpload：%v", err)
	}
	if err := f.store.Fake.Put(t.Context(), storage.PutInput{Key: tmpKeyOf(res), Body: bytes.NewReader(actual), Size: int64(len(actual)), ContentType: "image/jpeg"}); err != nil {
		t.Fatal(err)
	}
	complete(t, f, owner, res)
	if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatalf("ProcessUpload：%v", err)
	}
	return res
}

func wantRejected(t *testing.T, f *fixture, res media.BeginResult, reason media.RejectReason) {
	t.Helper()
	st, err := f.m.GetUpload(t.Context(), owner, res.Token)
	if err != nil || st.State != media.UploadRejected || st.RejectReason != reason {
		t.Fatalf("GetUpload = %+v, %v，預期 REJECTED/%s", st, err, reason)
	}
	if n := assetCount(t, f); n != 0 {
		t.Fatalf("被拒絕的暫存不該有資產，現有 %d", n)
	}
	_, final, _ := uploadKeys(t, f, res.ID)
	if final != nil && f.store.has(*final) {
		t.Fatal("被拒絕的上傳不該有 public/ 物件")
	}
	_, err = confirmInTx(f, owner, res.Token)
	wantKind(t, err, apperr.KindFailedPrecondition)
}

func TestPublicProcess_UnrecognizedImageIsRejectedAndCannotBeConfirmed(t *testing.T) {
	f := setup(t)
	realModule(f)
	fake := []byte("<html>not an image</html>")
	res := processReal(t, f, int64(len(fake)), md5b64(fake), fake)
	wantRejected(t, f, res, media.RejectUnrecognizedFormat)
}

func TestPublicProcess_SizeMismatchIsRejectedAndCannotBeConfirmed(t *testing.T) {
	f := setup(t)
	realModule(f)
	img := jpegWithGPS(t)
	res := processReal(t, f, int64(len(img))+1, md5b64(img), img)
	wantRejected(t, f, res, media.RejectSizeMismatch)
}

func TestPublicProcess_MD5MismatchIsRejectedAndCannotBeConfirmed(t *testing.T) {
	f := setup(t)
	realModule(f)
	img := jpegWithGPS(t)
	res := processReal(t, f, int64(len(img)), md5b64([]byte("other")), img)
	wantRejected(t, f, res, media.RejectChecksumMismatch)
}

func TestPublicProcess_StripsGPSAppliesOrientationAndCanBeConfirmed(t *testing.T) {
	f := setup(t)
	realModule(f)
	img := jpegWithGPS(t)
	if !bytes.Contains(img, []byte(geoMarker)) {
		t.Fatal("fixture 必須含 GPS 字串")
	}
	res := processReal(t, f, int64(len(img)), md5b64(img), img)
	if got := uploadState(t, f, res.ID); got != "READY" {
		t.Fatalf("state = %s，預期 READY", got)
	}
	_, final, asset := uploadKeys(t, f, res.ID)
	if final == nil || asset == nil {
		t.Fatal("READY 必須有正式 key 與資產")
	}
	rc, err := f.store.Fake.Get(t.Context(), *final)
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte(geoMarker)) || bytes.Contains(out, []byte("GPS")) || bytes.Contains(out, []byte("Exif")) {
		t.Fatal("public/ 的物件仍含 GPS 或 EXIF")
	}
	dec, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("輸出不可解碼：%v", err)
	}
	if b := dec.Bounds(); b.Dx() != 4 || b.Dy() != 8 {
		t.Fatalf("輸出尺寸 %dx%d，預期方向 6 旋正後的 4x8", b.Dx(), b.Dy())
	}
	var size int64
	var ct string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT size_bytes, content_type FROM media.assets WHERE id = $1`, *asset).Scan(&size, &ct); err != nil {
		t.Fatal(err)
	}
	if size != int64(len(out)) || ct != "image/jpeg" {
		t.Fatalf("資產列 size=%d type=%s，預期取自實際輸出 %d image/jpeg", size, ct, len(out))
	}
	if _, err := confirmInTx(f, owner, res.Token); err != nil {
		t.Fatalf("Confirm：%v", err)
	}
}
