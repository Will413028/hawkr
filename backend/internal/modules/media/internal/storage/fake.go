package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"sync"
)

// Fake 是只給 Go 測試接線的記憶體實作；正式啟動不會建構它。
type Fake struct {
	mu      sync.Mutex
	objects map[string]fakeObject
}

type fakeObject struct {
	data        []byte
	contentType string
}

// NewFake 回傳空的 Fake。
func NewFake() *Fake {
	return &Fake{objects: map[string]fakeObject{}}
}

var _ Store = (*Fake)(nil)

// PresignPut 回傳不可連線的假網址；header 與真實 adapter 相同。
func (f *Fake) PresignPut(_ context.Context, in PresignPutInput) (PresignedPut, error) {
	if err := validatePresign(in); err != nil {
		return PresignedPut{}, err
	}
	return PresignedPut{
		URL:     "https://fake-storage.invalid/" + in.Key,
		Headers: presignHeaders(in),
	}, nil
}

// Get 實作 Store。
func (f *Fake) Get(_ context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(o.data))), nil
}

// Head 實作 Store。
func (f *Fake) Head(_ context.Context, key string) (ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[key]
	if !ok {
		return ObjectInfo{}, ErrNotFound
	}
	return ObjectInfo{Size: int64(len(o.data)), ContentType: o.contentType}, nil
}

// Put 實作 Store；Body 長度與 Size 不符是錯誤。
func (f *Fake) Put(_ context.Context, in PutInput) error {
	if in.Key == "" {
		return errors.New("storage: empty key")
	}
	data, err := io.ReadAll(in.Body)
	if err != nil {
		return err
	}
	if int64(len(data)) != in.Size {
		return ErrSizeMismatch
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	contentType := in.ContentType
	if contentType == "" {
		contentType = DefaultContentType
	}
	f.objects[in.Key] = fakeObject{data: data, contentType: contentType}
	return nil
}

// Delete 實作 Store；不存在的物件視為成功。
func (f *Fake) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
	return nil
}

// Keys 回傳目前所有 key（排序），給測試斷言。
func (f *Fake) Keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.objects))
	for k := range f.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
