package image

import "bytes"

//nolint:gocritic // FourCC 本來就含空白（"VP8 "）
var webpKeep = map[string]bool{"VP8 ": true, "VP8L": true, "VP8X": true, "ALPH": true, "ANIM": true, "ICCP": true}

const (
	webpFlagXMP  = 0x04
	webpFlagEXIF = 0x08
)

func stripWebP(d []byte) ([]byte, error) {
	riffSize := int64(u32le(d, 4))
	limit := int64(len(d))
	if riffSize+8 < limit {
		limit = riffSize + 8
	}
	if limit < 12 {
		return nil, bad("WebP 長度錯誤")
	}
	var chunks [][]byte
	hasVP8X := false
	orientation := 1
	i := int64(12)
	for i < limit {
		if i+8 > limit {
			return nil, bad("WebP 區塊不完整")
		}
		typ := string(d[i : i+4])
		length := int64(u32le(d, int(i)+4))
		padded := length + (length & 1)
		if i+8+length > limit {
			return nil, bad("WebP 區塊長度錯誤")
		}
		dataStart := i + 8
		end := dataStart + padded
		if end > limit {
			end = limit
		}
		if typ == "EXIF" {
			p, n := int(dataStart), int(length)
			if hasPrefixAt(d, p, "Exif\x00\x00", n) {
				p += 6
				n -= 6
			}
			orientation = orientationOfTIFF(d, p, n)
		} else if typ == "ANMF" {
			c, err := rebuildANMF(d, int(dataStart), int(length))
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, c)
		} else if webpKeep[typ] {
			c := make([]byte, 8+padded)
			copy(c, d[i:end])
			if typ == "VP8X" {
				if length < 10 {
					return nil, bad("WebP VP8X 長度錯誤")
				}
				hasVP8X = true
				c[8] &^= webpFlagEXIF | webpFlagXMP
			}
			chunks = append(chunks, c)
		}
		i = end
	}
	var withExif bool
	if !hasVP8X && orientation > 1 {
		// 簡單格式沒有放 EXIF 的地方：補一個 VP8X，畫布大小取自影像區塊的標頭。
		vp8x, err := synthesizeVP8X(chunks)
		if err != nil {
			return nil, err
		}
		chunks = append([][]byte{vp8x}, chunks...)
		hasVP8X = true
	}
	withExif = hasVP8X && orientation > 1
	if withExif {
		for _, c := range chunks {
			if string(c[0:4]) == "VP8X" {
				c[8] |= webpFlagEXIF
			}
		}
	}
	var body bytes.Buffer
	for _, c := range chunks {
		body.Write(c)
	}
	if withExif {
		tiff := orientationTIFF(orientation)
		body.WriteString("EXIF")
		putU32le(&body, uint32(len(tiff))) //nolint:gosec // 最小 TIFF 固定 26 位元組
		body.Write(tiff)
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	putU32le(&out, uint32(body.Len()+4)) //nolint:gosec // body 只含輸入的子集，輸入長度由 RIFF 的 32 位元欄位限制
	out.WriteString("WEBP")
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

// rebuildANMF 只留影格的 ALPH、VP8、VP8L 子區塊，並以實際內容重算大小；影格的 16 位元組標頭（位置、尺寸、時長、旗標）原樣保留。
func rebuildANMF(d []byte, start, length int) ([]byte, error) {
	if length < 16 {
		return nil, bad("WebP ANMF 長度錯誤")
	}
	var payload bytes.Buffer
	payload.Write(d[start : start+16])
	i, end := start+16, start+length
	for i < end {
		if i+8 > end {
			return nil, bad("WebP ANMF 子區塊不完整")
		}
		typ := string(d[i : i+4])
		n := int(u32le(d, i+4))
		if n < 0 || n > end-i-8 {
			return nil, bad("WebP ANMF 子區塊長度錯誤")
		}
		pad := n + n&1
		if typ == "ALPH" || typ == "VP8 " || typ == "VP8L" {
			c := make([]byte, 8+pad)
			copy(c, d[i:i+8+n])
			payload.Write(c)
		}
		i += 8 + pad
	}
	var c bytes.Buffer
	c.WriteString("ANMF")
	putU32le(&c, uint32(payload.Len())) //nolint:gosec // 只含輸入的子集
	c.Write(payload.Bytes())
	return c.Bytes(), nil
}

// synthesizeVP8X 為簡單格式的檔案組出 VP8X 區塊（旗標先不含 EXIF，由呼叫方設定）。
func synthesizeVP8X(chunks [][]byte) ([]byte, error) {
	for _, c := range chunks {
		typ := string(c[0:4])
		if typ != "VP8 " && typ != "VP8L" {
			continue
		}
		p := c[8:]
		var w, h int
		var flags byte
		switch {
		case typ == "VP8 " && len(p) >= 10 && p[3] == 0x9D && p[4] == 0x01 && p[5] == 0x2A:
			w = int(p[6]) | int(p[7]&0x3F)<<8
			h = int(p[8]) | int(p[9]&0x3F)<<8
		case typ == "VP8L" && len(p) >= 5 && p[0] == 0x2F:
			v := u32le(p, 1)
			w = int(v&0x3FFF) + 1
			h = int(v>>14&0x3FFF) + 1
			if v>>28&1 == 1 {
				flags |= 0x10 // alpha
			}
		default:
			return nil, bad("WebP 影像標頭錯誤")
		}
		if w == 0 || h == 0 {
			return nil, bad("WebP 影像尺寸為零")
		}
		var b bytes.Buffer
		b.WriteString("VP8X")
		putU32le(&b, 10)
		b.Write([]byte{flags, 0, 0, 0})
		var dim bytes.Buffer
		putU32le(&dim, uint32(w-1)) //nolint:gosec // w 最大 16384
		b.Write(dim.Bytes()[:3])
		dim.Reset()
		putU32le(&dim, uint32(h-1)) //nolint:gosec // h 最大 16384
		b.Write(dim.Bytes()[:3])
		return b.Bytes(), nil
	}
	return nil, bad("WebP 缺少影像區塊")
}
