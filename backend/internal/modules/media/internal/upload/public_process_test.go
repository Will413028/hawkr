package upload_test

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // 測試用的 Content-MD5
	"encoding/base64"
	"errors"
	stdimage "image"
	"image/color"
	"image/gif"
	"io"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/upload"
)

// tinyGIF 是 1x1、不透明的單格 GIF。
var tinyGIF = func() []byte {
	var b bytes.Buffer
	img := stdimage.NewPaletted(stdimage.Rect(0, 0, 1, 1), color.Palette{color.Black, color.White})
	if err := gif.Encode(&b, img, nil); err != nil {
		panic(err)
	}
	return b.Bytes()
}()

func md5b64(b []byte) string {
	s := md5.Sum(b) //nolint:gosec // 同上
	return base64.StdEncoding.EncodeToString(s[:])
}

func putTmp(t *testing.T, s *storage.Fake, data []byte) {
	t.Helper()
	if err := s.Put(context.Background(), storage.PutInput{Key: "tmp/a", Body: bytes.NewReader(data), Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
}

func input(data []byte) upload.ProcessInput {
	return upload.ProcessInput{
		Visibility: model.VisibilityPublic, TmpKey: "tmp/a", FinalKey: "public/b",
		DeclaredSize: int64(len(data)), DeclaredMD5: md5b64(data),
	}
}

func wantPermanent(t *testing.T, err error, reason model.RejectReason) {
	t.Helper()
	var p *model.PermanentError
	if !errors.As(err, &p) || p.Reason != reason {
		t.Fatalf("err = %v，預期永久失敗 %s", err, reason)
	}
}

func TestPublicProcess_WritesReencodedObjectWithRealTypeAndSize(t *testing.T) {
	s := storage.NewFake()
	putTmp(t, s, tinyGIF)
	in := input(tinyGIF)
	in.DeclaredContentType = "application/x-wrong"
	res, err := upload.PublicProcess(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.Head(context.Background(), "public/b")
	if err != nil || res.ContentType != "image/jpeg" || info.ContentType != "image/jpeg" || res.SizeBytes != info.Size {
		t.Fatalf("res=%+v info=%+v err=%v", res, info, err)
	}
}

func TestPublicProcess_RejectsAndWritesNothing(t *testing.T) {
	fake := []byte("not an image at all")
	cases := map[string]struct {
		data   []byte
		mutate func(*upload.ProcessInput)
		reason model.RejectReason
	}{
		"fake image":    {fake, func(*upload.ProcessInput) {}, model.RejectUnrecognizedFormat},
		"size smaller":  {tinyGIF, func(in *upload.ProcessInput) { in.DeclaredSize++ }, model.RejectSizeMismatch},
		"size larger":   {tinyGIF, func(in *upload.ProcessInput) { in.DeclaredSize-- }, model.RejectSizeMismatch},
		"md5 mismatch":  {tinyGIF, func(in *upload.ProcessInput) { in.DeclaredMD5 = md5b64([]byte("x")) }, model.RejectChecksumMismatch},
		"md5 malformed": {tinyGIF, func(in *upload.ProcessInput) { in.DeclaredMD5 = "!!" }, model.RejectChecksumMismatch},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := storage.NewFake()
			putTmp(t, s, c.data)
			in := input(c.data)
			c.mutate(&in)
			_, err := upload.PublicProcess(context.Background(), s, in)
			wantPermanent(t, err, c.reason)
			if _, err := s.Head(context.Background(), "public/b"); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("被拒絕的上傳不該寫入 public/：%v", err)
			}
		})
	}
}

type failingStore struct {
	storage.Store
	getErr, putErr error
}

func (f failingStore) Get(ctx context.Context, k string) (io.ReadCloser, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.Store.Get(ctx, k)
}

func (f failingStore) Put(ctx context.Context, in storage.PutInput) error {
	if f.putErr != nil {
		return f.putErr
	}
	return f.Store.Put(ctx, in)
}

func TestPublicProcess_StorageErrorsAreRetryable(t *testing.T) {
	boom := errors.New("r2 unavailable")
	for name, fs := range map[string]failingStore{
		"get": {Store: storage.NewFake(), getErr: boom},
		"put": {Store: func() storage.Store { s := storage.NewFake(); putTmp(t, s, tinyGIF); return s }(), putErr: boom},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := upload.PublicProcess(context.Background(), fs, input(tinyGIF))
			var p *model.PermanentError
			if !errors.Is(err, boom) || errors.As(err, &p) {
				t.Fatalf("err = %v，預期原樣回傳的暫時失敗", err)
			}
		})
	}
}

func TestPublicProcess_MissingTmpObjectIsPermanent(t *testing.T) {
	_, err := upload.PublicProcess(context.Background(), storage.NewFake(), input(tinyGIF))
	wantPermanent(t, err, model.RejectSizeMismatch)
}

func TestNewProcessor_SecureRowIsPermanentFailure(t *testing.T) {
	in := input(tinyGIF)
	in.Visibility = model.VisibilitySecure
	_, err := upload.NewProcessor(storage.NewFake())(context.Background(), in)
	wantPermanent(t, err, model.RejectProcessingFailed)
}

type countingStore struct {
	storage.Store
	gets int
}

func (c *countingStore) Get(ctx context.Context, k string) (io.ReadCloser, error) {
	c.gets++
	return c.Store.Get(ctx, k)
}

func TestPublicProcess_RefusesDeclaredSizeAboveLimitWithoutReading(t *testing.T) {
	s := &countingStore{Store: storage.NewFake()}
	putTmp(t, s.Store.(*storage.Fake), tinyGIF)
	in := input(tinyGIF)
	in.DeclaredSize = model.MaxPublicUploadSize + 1
	_, err := upload.PublicProcess(context.Background(), s, in)
	wantPermanent(t, err, model.RejectSizeMismatch)
	if s.gets != 0 {
		t.Fatal("超過上限的宣告不該讀物件")
	}
}

func TestPublicProcess_CancelledContextIsTemporary(t *testing.T) {
	s := storage.NewFake()
	putTmp(t, s, tinyGIF)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := upload.PublicProcess(ctx, s, input(tinyGIF))
	var p *model.PermanentError
	if !errors.Is(err, context.Canceled) || errors.As(err, &p) {
		t.Fatalf("err = %v，預期 context.Canceled 的暫時失敗", err)
	}
}

func TestProcessJobRunsOnItsOwnQueue(t *testing.T) {
	if got := (upload.ProcessArgs{}).InsertOpts().Queue; got != model.ProcessQueue {
		t.Fatalf("queue = %q，預期 %q", got, model.ProcessQueue)
	}
	if model.ProcessQueue == "default" || model.ProcessQueueWorkers != 2 {
		t.Fatalf("queue %q workers %d", model.ProcessQueue, model.ProcessQueueWorkers)
	}
}
