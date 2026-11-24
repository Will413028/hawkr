package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
)

type countingGetStore struct {
	*storage.Fake
	gets int
}

func (c *countingGetStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	c.gets++
	return c.Fake.Get(ctx, key)
}

// Close 發生在段邊界（上一段讀完、下一段還沒開）時，之後的 Read 不得再開下一段的物件。
func TestChunkedReader_CloseAtASegmentBoundaryStopsLaterReads(t *testing.T) {
	st := &countingGetStore{Fake: storage.NewFake()}
	if err := st.Put(t.Context(), storage.PutInput{Key: "k1", Body: bytes.NewReader([]byte("x")), Size: 1, ContentType: "application/octet-stream"}); err != nil {
		t.Fatal(err)
	}
	r := &chunkedReader{ctx: t.Context(), store: st, asset: uuid.New(), keys: []string{"k0", "k1"}, next: 1}
	if err := r.Close(); err != nil {
		t.Fatalf("Close：%v", err)
	}
	if _, err := r.Read(make([]byte, 8)); !errors.Is(err, errClosed) {
		t.Fatalf("Close 之後 Read err = %v，預期 errClosed", err)
	}
	if st.gets != 0 {
		t.Fatalf("Close 之後開了 %d 個物件", st.gets)
	}
}
