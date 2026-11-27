package characterization

import (
	"bytes"
	"context"
	"net/http"
	"strconv"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

var monolithScenarios = []ch.Scenario[MonolithDriver]{
	{Name: "declared pdf that is arbitrary bytes is rejected", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		body := bytes.Repeat([]byte("not a pdf, just arbitrary bytes. "), 4)
		s := ch.Must(t, ch.Ret(d.CreateUploadSession(ctx, ch.Buyer, UploadInput{
			Filename: "doc-" + ch.Nonce() + ".pdf", ContentType: "application/pdf", FileSize: strconv.Itoa(len(body)), IsSecure: true,
		})))
		wantStatus(ctx, t, d, s.ID, 0, body, http.StatusBadRequest)
		pdf := []byte("%PDF-1.7\n" + string(body[:len(body)-9]))
		if code, err := d.PutSecureChunk(ctx, ch.Buyer, s.ID, 0, pdf); err != nil || code != http.StatusPreconditionFailed {
			t.Errorf("被拒絕的會話不該再收段：status = %d, err = %v，預期 412", code, err)
		}
	}},
	{Name: "real pdf bytes are accepted", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		pdf := []byte("%PDF-1.7\n" + ch.Nonce() + "\n%%EOF")
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, int64(len(pdf)))))
		if s.ChunkSize != media.SecureChunkSize || s.ChunkCount != 1 {
			t.Errorf("chunkSize = %d、chunkCount = %d，預期 %d 與 1", s.ChunkSize, s.ChunkCount, media.SecureChunkSize)
		}
		wantStatus(ctx, t, d, s.Token, 0, pdf, http.StatusNoContent)
	}},
	{Name: "oversize secure upload is rejected", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		for _, size := range []int64{media.MaxSecureUploadSize + 1, 0, -1} {
			_, err := d.BeginSecureUpload(ctx, ch.Buyer, size)
			ch.WantKind(t, err, ch.ErrInvalid)
		}
	}},
	{Name: "chunk over segment size is rejected", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, 2*media.SecureChunkSize)))
		body := append([]byte("%PDF-1.7\n"), make([]byte, media.SecureChunkSize)...)
		wantStatus(ctx, t, d, s.Token, 0, body, http.StatusRequestEntityTooLarge)
	}},
	{Name: "chunk length that differs from the declared size is rejected", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, 100)))
		wantStatus(ctx, t, d, s.Token, 0, append([]byte("%PDF-1.7\n"), make([]byte, 41)...), http.StatusBadRequest)
	}},
	{Name: "chunk of another accounts session is not found", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		pdf := []byte("%PDF-1.7\n" + ch.Nonce())
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, int64(len(pdf)))))
		code, err := d.PutSecureChunk(ctx, ch.Buyer2, s.Token, 0, pdf)
		if err != nil || code != http.StatusNotFound {
			t.Errorf("別人的會話：status = %d, err = %v，預期 404", code, err)
		}
	}},
	{Name: "chunk index gap is a conflict", Run: func(ctx context.Context, t ch.T, d MonolithDriver) {
		s := ch.Must(t, ch.Ret(d.BeginSecureUpload(ctx, ch.Buyer, 2*media.SecureChunkSize)))
		wantStatus(ctx, t, d, s.Token, 1, make([]byte, media.SecureChunkSize), http.StatusConflict)
	}},
}

func wantStatus(ctx context.Context, t ch.T, d MonolithDriver, token string, index int, body []byte, want int) {
	t.Helper()
	code, err := d.PutSecureChunk(ctx, ch.Buyer, token, index, body)
	if err != nil || code != want {
		t.Errorf("PUT 第 %d 段：status = %d, err = %v，預期 %d", index, code, err, want)
	}
}
