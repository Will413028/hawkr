package image

import "bytes"

//nolint:gocritic // FourCC 本來就含空白（"VP8 "）
var webpKeep = map[string]bool{"VP8 ": true, "VP8L": true, "VP8X": true, "ALPH": true, "ANIM": true, "ANMF": true, "ICCP": true}

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
	withExif := hasVP8X && orientation > 1
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
