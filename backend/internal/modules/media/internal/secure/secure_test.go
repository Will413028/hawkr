package secure_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/secure"
)

func newDEK(t *testing.T) []byte {
	t.Helper()
	dek, err := secure.NewDEK()
	if err != nil {
		t.Fatal(err)
	}
	return dek
}

var single = secure.Chunk{Asset: uuid.UUID{1}, Index: 0, Count: 1}

func seal(t *testing.T, dek, plain []byte) []byte {
	t.Helper()
	return sealChunk(t, dek, plain, single)
}

func sealChunk(t *testing.T, dek, plain []byte, c secure.Chunk) []byte {
	t.Helper()
	var buf bytes.Buffer
	n, err := secure.EncryptChunk(&buf, bytes.NewReader(plain), dek, c)
	if err != nil {
		t.Fatalf("Encrypt：%v", err)
	}
	if n != int64(len(plain)) {
		t.Fatalf("Encrypt 回報明文 %d 位元組，實際 %d", n, len(plain))
	}
	return buf.Bytes()
}

func open(dek, ct []byte) ([]byte, error) {
	return openChunk(dek, ct, single)
}

func openChunk(dek, ct []byte, c secure.Chunk) ([]byte, error) {
	r, err := secure.NewChunkReader(bytes.NewReader(ct), dek, c)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func TestRoundTrip_SegmentBoundaries(t *testing.T) {
	seg := secure.SegmentSize
	for _, n := range []int{0, 1, seg - 1, seg, seg + 1, 3*seg + 17, 4 * seg} {
		dek := newDEK(t)
		plain := randBytes(n)
		got, err := open(dek, seal(t, dek, plain))
		if err != nil {
			t.Fatalf("n=%d：%v", n, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("n=%d：來回後內容不同", n)
		}
	}
}

func TestCiphertextHoldsNoPlaintext(t *testing.T) {
	dek := newDEK(t)
	plain := bytes.Repeat([]byte("SECRET-PASSPORT-"), 64)
	if bytes.Contains(seal(t, dek, plain), []byte("SECRET-PASSPORT-")) {
		t.Fatal("密文含有明文片段")
	}
}

func TestDecrypt_RejectsTamperingAndDeliversNothingFromBadSegment(t *testing.T) {
	seg := secure.SegmentSize
	dek := newDEK(t)
	plain := randBytes(3*seg + 100)
	ct := seal(t, dek, plain)
	hdr := secure.HeaderSize
	full := seg + secure.TagSize

	flip := func(i int) []byte {
		c := bytes.Clone(ct)
		c[i] ^= 0x01
		return c
	}
	cases := []struct {
		name      string
		ct        []byte
		maxPlain  int
		wantError bool
	}{
		{"header-magic", flip(0), 0, true},
		{"header-version", flip(4), 0, true},
		{"header-segment-size", flip(5), 0, true},
		{"header-nonce-prefix", flip(hdr - 1), 0, true},
		{"first-segment", flip(hdr + 3), 0, true},
		{"middle-segment", flip(hdr + full + 3), seg, true},
		{"last-segment", flip(len(ct) - 1), 3 * seg, true},
		{"truncated-in-last-segment", ct[:len(ct)-5], 3 * seg, true},
		{"truncated-at-segment-boundary", ct[:hdr+2*full], seg, true},
		{"truncated-header", ct[:hdr-1], 0, true},
		{"trailing-noise", append(bytes.Clone(ct), 0x00), 3 * seg, true},
		{"trailing-segment", append(bytes.Clone(ct), ct[hdr:hdr+full]...), 3 * seg, true},
		{"header-only", ct[:hdr], 0, true},
	}
	swapped := bytes.Clone(ct)
	copy(swapped[hdr:hdr+full], ct[hdr+full:hdr+2*full])
	copy(swapped[hdr+full:hdr+2*full], ct[hdr:hdr+full])
	cases = append(cases, struct {
		name      string
		ct        []byte
		maxPlain  int
		wantError bool
	}{"swapped-segments", swapped, 0, true})

	for _, c := range cases {
		got, err := open(dek, c.ct)
		if err == nil {
			t.Errorf("%s：竄改沒有被發現", c.name)
			continue
		}
		if len(got) > c.maxPlain {
			t.Errorf("%s：錯誤之前交出 %d 位元組明文，上限 %d", c.name, len(got), c.maxPlain)
		}
		if !bytes.Equal(got, plain[:len(got)]) {
			t.Errorf("%s：交出的明文與原文不符", c.name)
		}
	}
}

func TestDecrypt_RejectsWrongKey(t *testing.T) {
	ct := seal(t, newDEK(t), []byte("hello"))
	if _, err := open(newDEK(t), ct); err == nil {
		t.Fatal("錯的 DEK 解得開")
	}
}

func headerWith(version byte, segSize uint32) []byte {
	h := make([]byte, secure.HeaderSize)
	copy(h, "HSEC")
	h[4] = version
	h[5], h[6], h[7], h[8] = byte(segSize>>24), byte(segSize>>16), byte(segSize>>8), byte(segSize)
	return h
}

func TestDecrypt_RejectsBadHeaderParameters(t *testing.T) {
	dek := newDEK(t)
	for name, h := range map[string][]byte{
		"v1-is-not-decoded": headerWith(1, uint32(secure.SegmentSize)),
		"unknown-version":   headerWith(3, uint32(secure.SegmentSize)),
		"zero-segment":      headerWith(2, 0),
		"huge-segment":      headerWith(2, uint32(secure.MaxSegmentSize)+1),
	} {
		if _, err := secure.NewChunkReader(bytes.NewReader(append(h, make([]byte, 64)...)), dek, single); err == nil {
			t.Errorf("%s：header 沒有被拒絕", name)
		}
	}
}

func TestEncrypt_SourceErrorIsReturned(t *testing.T) {
	boom := errors.New("boom")
	_, err := secure.EncryptChunk(io.Discard, io.MultiReader(strings.NewReader("abc"), errReader{boom}), newDEK(t), single)
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func testKeyring(t *testing.T, s string) *secure.Keyring {
	t.Helper()
	k, err := secure.ParseKeyring(s)
	if err != nil {
		t.Fatalf("ParseKeyring：%v", err)
	}
	return k
}

func kekText(id string) string { return id + ":" + base64.StdEncoding.EncodeToString(randBytes(32)) }

func TestParseKeyring_RejectsBadConfig(t *testing.T) {
	good := kekText("k1")
	short := "k2:" + base64.StdEncoding.EncodeToString(randBytes(16))
	for name, s := range map[string]string{
		"empty":        "",
		"blank":        "  ",
		"no-colon":     "k1",
		"empty-id":     ":" + base64.StdEncoding.EncodeToString(randBytes(32)),
		"not-base64":   "k1:%%%",
		"short-key":    short,
		"long-key":     "k1:" + base64.StdEncoding.EncodeToString(randBytes(33)),
		"duplicate-id": good + "," + good,
		"empty-entry":  good + ",",
	} {
		if _, err := secure.ParseKeyring(s); err == nil {
			t.Errorf("%s：應回錯誤", name)
		}
	}
}

func TestKeyring_WrapRoundTripAndPrimary(t *testing.T) {
	k := testKeyring(t, kekText("k2")+","+kekText("k1"))
	if k.PrimaryID() != "k2" {
		t.Fatalf("primary=%s", k.PrimaryID())
	}
	id := uuid.New()
	dek := newDEK(t)
	kid, wrapped, err := k.Wrap(id, dek)
	if err != nil || kid != "k2" {
		t.Fatalf("Wrap：%v %s", err, kid)
	}
	got, err := k.Unwrap(id, kid, wrapped)
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("Unwrap：%v", err)
	}
}

func TestKeyring_UnwrapFailures(t *testing.T) {
	k := testKeyring(t, kekText("k1"))
	id := uuid.New()
	kid, wrapped, err := k.Wrap(id, newDEK(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.Unwrap(id, "unknown", wrapped); err == nil {
		t.Error("未知的 kek_id 應失敗")
	}
	if _, err := k.Unwrap(uuid.New(), kid, wrapped); err == nil {
		t.Error("wrapped DEK 換到別的資產 ID 應失敗")
	}
	bad := bytes.Clone(wrapped)
	bad[len(bad)-1] ^= 1
	if _, err := k.Unwrap(id, kid, bad); err == nil {
		t.Error("竄改的 wrapped DEK 應失敗")
	}
	if _, err := k.Unwrap(id, kid, wrapped[:5]); err == nil {
		t.Error("過短的 wrapped DEK 應失敗")
	}
}

func TestKeyring_UnwrapWithAnotherKeyOfSameID(t *testing.T) {
	a := testKeyring(t, kekText("k1"))
	b := testKeyring(t, kekText("k1"))
	id := uuid.New()
	kid, wrapped, _ := a.Wrap(id, newDEK(t))
	if _, err := b.Unwrap(id, kid, wrapped); err == nil {
		t.Fatal("不同金鑰解得開")
	}
}

func threeChunks(t *testing.T, dek []byte, asset uuid.UUID) (plain, ct [3][]byte) {
	t.Helper()
	for i := range 3 {
		plain[i] = randBytes(secure.SegmentSize + 10*i + 1)
		ct[i] = sealChunk(t, dek, plain[i], secure.Chunk{Asset: asset, Index: uint32(i), Count: 3})
	}
	return plain, ct
}

func TestChunks_RoundTripEachBoundToItsPosition(t *testing.T) {
	dek, asset := newDEK(t), uuid.New()
	plain, ct := threeChunks(t, dek, asset)
	for i := range 3 {
		got, err := openChunk(dek, ct[i], secure.Chunk{Asset: asset, Index: uint32(i), Count: 3})
		if err != nil || !bytes.Equal(got, plain[i]) {
			t.Fatalf("上傳段 %d 來回失敗：%v", i, err)
		}
	}
}

func TestChunks_SwappedChunksFailToDecrypt(t *testing.T) {
	dek, asset := newDEK(t), uuid.New()
	_, ct := threeChunks(t, dek, asset)
	at := func(i int) secure.Chunk { return secure.Chunk{Asset: asset, Index: uint32(i), Count: 3} }
	if got, err := openChunk(dek, ct[1], at(0)); err == nil || len(got) != 0 {
		t.Fatalf("把第 1 段當第 0 段讀應失敗且不交出明文：err=%v 交出 %d 位元組", err, len(got))
	}
	if got, err := openChunk(dek, ct[0], at(2)); err == nil || len(got) != 0 {
		t.Fatalf("把第 0 段當第 2 段讀應失敗且不交出明文：err=%v 交出 %d 位元組", err, len(got))
	}
}

func TestChunks_DroppingTheLastChunkChangesTheCountAndFails(t *testing.T) {
	dek, asset := newDEK(t), uuid.New()
	_, ct := threeChunks(t, dek, asset)
	for i := range 2 {
		if got, err := openChunk(dek, ct[i], secure.Chunk{Asset: asset, Index: uint32(i), Count: 2}); err == nil || len(got) != 0 {
			t.Fatalf("少了最後一段（總數 2）時第 %d 段應失敗：err=%v", i, err)
		}
	}
	if _, err := openChunk(dek, ct[0], secure.Chunk{Asset: asset, Index: 0, Count: 4}); err == nil {
		t.Fatal("總數被放大時應失敗")
	}
}

func TestChunks_AnotherAssetsChunkFailsToDecrypt(t *testing.T) {
	dek := newDEK(t)
	a, b := uuid.New(), uuid.New()
	ctA := sealChunk(t, dek, randBytes(100), secure.Chunk{Asset: a, Index: 0, Count: 1})
	if got, err := openChunk(dek, ctA, secure.Chunk{Asset: b, Index: 0, Count: 1}); err == nil || len(got) != 0 {
		t.Fatalf("別的資產的上傳段應失敗：err=%v", err)
	}
}

func TestChunks_EachAttemptUsesAFreshNoncePrefix(t *testing.T) {
	dek := newDEK(t)
	plain := randBytes(100)
	a, b := seal(t, dek, plain), seal(t, dek, plain)
	if bytes.Equal(a[secure.HeaderSize-7:secure.HeaderSize], b[secure.HeaderSize-7:secure.HeaderSize]) {
		t.Fatal("同一個位置兩次嘗試的 nonce 前綴相同")
	}
}

func TestEncryptChunk_RejectsInvalidPosition(t *testing.T) {
	for _, c := range []secure.Chunk{{Index: 0, Count: 0}, {Index: 3, Count: 3}} {
		if _, err := secure.EncryptChunk(io.Discard, strings.NewReader("x"), newDEK(t), c); err == nil {
			t.Errorf("%+v 應被拒絕", c)
		}
	}
}
