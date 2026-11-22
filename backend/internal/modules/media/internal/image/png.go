package image

import (
	"bytes"
	"hash/crc32"
)

var pngKeep = map[string]bool{
	"IHDR": true, "PLTE": true, "IDAT": true, "IEND": true, "tRNS": true, "gAMA": true, "cHRM": true, "sRGB": true,
	"iCCP": true, "sBIT": true, "bKGD": true, "hIST": true, "pHYs": true, "sPLT": true, "acTL": true, "fcTL": true,
	"fdAT": true, "cICP": true, "mDCV": true, "cLLI": true,
}

func stripPNG(d []byte) ([]byte, error) {
	var chunks [][]byte
	var types []string
	orientation := 1
	i := 8
	sawIEND := false
	for i < len(d) && !sawIEND {
		if i+12 > len(d) {
			return nil, bad("PNG 區塊不完整")
		}
		length := int64(u32be(d, i))
		if length > int64(len(d)-i-12) {
			return nil, bad("PNG 區塊長度錯誤")
		}
		end := i + 12 + int(length)
		typ := string(d[i+4 : i+8])
		if len(chunks) == 0 && typ != "IHDR" {
			// 第一個區塊必須是 IHDR，不論該區塊會不會被保留。
			return nil, bad("PNG 缺少 IHDR")
		}
		if typ == "eXIf" {
			orientation = orientationOfTIFF(d, i+8, int(length))
		} else if pngKeep[typ] {
			chunks = append(chunks, append([]byte(nil), d[i:end]...))
			types = append(types, typ)
		}
		sawIEND = typ == "IEND"
		i = end
	}
	if !sawIEND {
		return nil, bad("PNG 缺少 IEND")
	}
	var out bytes.Buffer
	out.Grow(len(d))
	out.Write(d[:8])
	exifWritten := orientation <= 1
	for k, c := range chunks {
		if !exifWritten && types[k] == "IDAT" {
			writePNGExif(&out, orientation)
			exifWritten = true
		}
		out.Write(c)
	}
	return out.Bytes(), nil
}

func writePNGExif(out *bytes.Buffer, orientation int) {
	tiff := orientationTIFF(orientation)
	td := append([]byte("eXIf"), tiff...)
	putU32be(out, uint32(len(tiff))) //nolint:gosec // 最小 TIFF 固定 26 位元組
	out.Write(td)
	putU32be(out, crc32.ChecksumIEEE(td))
}
