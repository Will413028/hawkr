// Package secure 是機敏檔案的信封加密：內容用 STREAM 分塊 AEAD（AES-256-GCM）加密，每檔一把隨機 DEK，DEK 再以 KEK 包裹存在資產列。
// 只用標準庫。格式與理由見 ADR 2026-11-22-media-secure-stream-aead.md 與 B1-2c2 的設計。
//
// 密文格式 v2（B1-D17，不保留 v1 解碼）：一份資產由 count 個獨立的「上傳段」組成，每個上傳段是一份獨立的 STREAM 密文：
// header（magic 4 位元組、版本 2、AEAD 段大小 uint32 big-endian、這個上傳段自己的隨機 nonce 前綴 7 位元組）後接連續的 AEAD 段；
// 每個 AEAD 段是明文（最後一段可短）的 AES-256-GCM 密文加 16 位元組 tag。nonce ＝ 前綴＋big-endian uint32 段序號＋1 位元組末段旗標。
// 每個 AEAD 段的 AAD ＝ header ＋資產 ID（16 位元組）＋上傳段序號（uint32）＋上傳段總數（uint32）。
// 所以上傳段被對調、刪掉、換成別的資產的上傳段、少了最後一個上傳段都讓 tag 驗證失敗；每次嘗試用新的 nonce 前綴，同一把 DEK 下 nonce 不重用；
// 上傳段內部的 AEAD 段序號與末段旗標進 nonce，所以段內對調、截斷在段邊界、尾端多出資料同樣失敗。
// 同一份資產的所有上傳段共用一把 DEK。
package secure

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"uuid"
)

const (
	// SegmentSize 是每段明文的大小（age 的值）。
	SegmentSize = 64 << 10
	// MaxSegmentSize 是解密端接受的 header 段大小上限，防止惡意 header 要求配置巨大的緩衝。
	MaxSegmentSize = 1 << 20
	// TagSize 是 GCM tag 的長度。
	TagSize = 16
	// HeaderSize 是 header 的長度。
	HeaderSize = 4 + 1 + 4 + noncePrefixSize
	// KeySize 是 DEK 與 KEK 的長度。
	KeySize = 32

	noncePrefixSize = 7
	version         = 2
)

var magic = [4]byte{'H', 'S', 'E', 'C'}

// ErrCorrupt 表示密文被竄改、截斷或格式不合法；錯誤不區分原因，避免成為 oracle。
var ErrCorrupt = errors.New("secure：密文驗證失敗")

// NewDEK 產生每檔隨機的資料金鑰。
func NewDEK() ([]byte, error) {
	dek := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("secure：產生 DEK：%w", err)
	}
	return dek, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secure：金鑰必須是 %d 位元組", KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func segmentNonce(prefix []byte, seq uint32, last bool) []byte {
	n := make([]byte, 12)
	copy(n, prefix)
	binary.BigEndian.PutUint32(n[noncePrefixSize:], seq)
	if last {
		n[11] = 1
	}
	return n
}

// Chunk 指出一個上傳段：它屬於哪份資產、第幾段、資產共幾段；三者都進每個 AEAD 段的 AAD。
type Chunk struct {
	Asset uuid.UUID
	Index uint32
	Count uint32
}

func (c Chunk) valid() bool { return c.Count > 0 && c.Index < c.Count }

func (c Chunk) aad(header []byte) []byte {
	out := make([]byte, 0, len(header)+16+8)
	out = append(out, header...)
	out = append(out, c.Asset[:]...)
	out = binary.BigEndian.AppendUint32(out, c.Index)
	out = binary.BigEndian.AppendUint32(out, c.Count)
	return out
}

// EncryptChunk 從 src 讀一個上傳段的明文、把密文寫到 dst，回傳明文位元組數。每次呼叫產生新的 nonce 前綴。
// 記憶體只佔一個 AEAD 段加一個位元組的預讀。
func EncryptChunk(dst io.Writer, src io.Reader, dek []byte, c Chunk) (int64, error) {
	aead, err := newGCM(dek)
	if err != nil {
		return 0, err
	}
	if !c.valid() {
		return 0, errors.New("secure：上傳段序號或總數不合法")
	}
	header := make([]byte, HeaderSize)
	copy(header, magic[:])
	header[4] = version
	binary.BigEndian.PutUint32(header[5:9], SegmentSize)
	if _, err := io.ReadFull(rand.Reader, header[9:]); err != nil {
		return 0, fmt.Errorf("secure：產生 nonce 前綴：%w", err)
	}
	if _, err := dst.Write(header); err != nil {
		return 0, err
	}
	aad := c.aad(header)
	br := bufio.NewReaderSize(src, SegmentSize+1)
	buf := make([]byte, SegmentSize)
	out := make([]byte, 0, SegmentSize+TagSize)
	var total int64
	for seq := uint64(0); ; seq++ {
		if seq > uint64(^uint32(0)) {
			return total, errors.New("secure：段序號溢位")
		}
		n, err := io.ReadFull(br, buf)
		last := false
		switch {
		case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
			last = true
		case err != nil:
			return total, err
		default:
			if _, perr := br.Peek(1); errors.Is(perr, io.EOF) {
				last = true
			} else if perr != nil {
				return total, perr
			}
		}
		out = aead.Seal(out[:0], segmentNonce(header[9:], uint32(seq), last), buf[:n], aad)
		if _, err := dst.Write(out); err != nil {
			return total, err
		}
		total += int64(n)
		if last {
			return total, nil
		}
	}
}

// Reader 是串流解密：一段的 tag 驗過才交出那一段的明文。
type Reader struct {
	aead    cipher.AEAD
	br      *bufio.Reader
	header  []byte
	aad     []byte
	segSize int
	seq     uint64
	ct      []byte
	plain   []byte
	pending []byte
	done    bool
	err     error
}

// NewChunkReader 讀並檢查一個上傳段的 header，回傳解密的 Reader；header 不合法（magic、版本、段大小）回 ErrCorrupt。
// c 必須是這個上傳段寫入時的資產、序號與總數，否則第一個 AEAD 段就驗證失敗。
func NewChunkReader(src io.Reader, dek []byte, c Chunk) (*Reader, error) {
	aead, err := newGCM(dek)
	if err != nil {
		return nil, err
	}
	if !c.valid() {
		return nil, ErrCorrupt
	}
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(src, header); err != nil {
		return nil, ErrCorrupt
	}
	if [4]byte(header[:4]) != magic || header[4] != version {
		return nil, ErrCorrupt
	}
	seg := binary.BigEndian.Uint32(header[5:9])
	if seg == 0 || seg > MaxSegmentSize {
		return nil, ErrCorrupt
	}
	return &Reader{
		aead: aead, br: bufio.NewReaderSize(src, int(seg)+TagSize+1), header: header, aad: c.aad(header), segSize: int(seg),
		ct: make([]byte, int(seg)+TagSize), plain: make([]byte, 0, int(seg)),
	}, nil
}

// Read 實作 io.Reader；錯誤之後不會再交出任何位元組。
func (r *Reader) Read(p []byte) (int, error) {
	for len(r.pending) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		if r.done {
			return 0, io.EOF
		}
		r.err = r.nextSegment()
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *Reader) nextSegment() error {
	if r.seq > uint64(^uint32(0)) {
		return errors.New("secure：段序號溢位")
	}
	n, err := io.ReadFull(r.br, r.ct)
	last := false
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		last = true
	case err != nil:
		return err
	default:
		if _, perr := r.br.Peek(1); errors.Is(perr, io.EOF) {
			last = true
		} else if perr != nil {
			return perr
		}
	}
	if n < TagSize {
		return ErrCorrupt
	}
	pt, err := r.aead.Open(r.plain[:0], segmentNonce(r.header[9:], uint32(r.seq), last), r.ct[:n], r.aad)
	if err != nil {
		return ErrCorrupt
	}
	r.seq++
	r.pending = pt
	r.done = last
	return nil
}
