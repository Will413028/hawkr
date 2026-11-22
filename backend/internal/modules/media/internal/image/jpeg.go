package image

import "bytes"

func stripJPEG(d []byte) ([]byte, error) {
	var segs [][]byte
	orientation := 1
	i := 2
	sawEOI := false
	need := func(k int) (int, error) {
		if k < 0 || k >= len(d) {
			return 0, bad("圖片資料不完整")
		}
		return int(d[k]), nil
	}
	for i < len(d) {
		if d[i] != 0xFF {
			return nil, bad("JPEG 區段標記錯誤")
		}
		m, err := need(i + 1)
		if err != nil {
			return nil, err
		}
		if m == 0xFF { // 填充位元組
			i++
			continue
		}
		if m == 0xD9 {
			sawEOI = true
			break
		}
		if m == 0x01 || (m >= 0xD0 && m <= 0xD8) { // 沒有長度的標記
			segs = append(segs, append([]byte(nil), d[i:i+2]...))
			i += 2
			continue
		}
		hi, err := need(i + 2)
		if err != nil {
			return nil, err
		}
		lo, err := need(i + 3)
		if err != nil {
			return nil, err
		}
		length := hi<<8 | lo
		end := i + 2 + length
		if length < 2 || end > len(d) {
			return nil, bad("JPEG 區段長度錯誤")
		}
		keep := true
		if m >= 0xE0 && m <= 0xEF {
			p, n := i+4, length-2
			switch m {
			case 0xE0:
				keep = hasPrefixAt(d, p, "JFIF\x00", n)
			case 0xE2:
				keep = hasPrefixAt(d, p, "ICC_PROFILE\x00", n)
			case 0xEE:
				keep = hasPrefixAt(d, p, "Adobe", n)
			default:
				keep = false
			}
			if m == 0xE1 && hasPrefixAt(d, p, "Exif\x00\x00", n) {
				orientation = orientationOfTIFF(d, p+6, n-6)
			}
		} else if m == 0xFE {
			keep = false
		}
		if keep {
			segs = append(segs, append([]byte(nil), d[i:end]...))
		}
		i = end
		if m == 0xDA { // 掃描資料：複製到下一個真正的標記
			j := i
			for {
				if j+1 >= len(d) {
					return nil, bad("JPEG 缺少結尾")
				}
				if d[j] != 0xFF {
					j++
					continue
				}
				n := d[j+1]
				if n == 0x00 || (n >= 0xD0 && n <= 0xD7) {
					j += 2
				} else if n == 0xFF {
					j++
				} else {
					break
				}
			}
			segs = append(segs, append([]byte(nil), d[i:j]...))
			i = j
		}
	}
	if !sawEOI {
		return nil, bad("JPEG 缺少結尾")
	}
	var out bytes.Buffer
	out.Grow(len(d))
	out.Write([]byte{0xFF, 0xD8})
	// JFIF 規定緊接在 SOI 後，方向資料放在它後面
	exifAt := 0
	if len(segs) > 0 && len(segs[0]) > 1 && segs[0][1] == 0xE0 {
		exifAt = 1
	}
	for k, s := range segs {
		if k == exifAt && orientation > 1 {
			writeJPEGExif(&out, orientation)
		}
		out.Write(s)
	}
	if exifAt >= len(segs) && orientation > 1 {
		writeJPEGExif(&out, orientation)
	}
	out.Write([]byte{0xFF, 0xD9})
	return out.Bytes(), nil
}

func writeJPEGExif(out *bytes.Buffer, orientation int) {
	tiff := orientationTIFF(orientation)
	length := 2 + 6 + len(tiff)
	out.Write([]byte{0xFF, 0xE1, byte(length >> 8 & 0xFF), byte(length & 0xFF)})
	out.WriteString("Exif\x00\x00")
	out.Write(tiff)
}
