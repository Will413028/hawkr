package image

// orientationOfTIFF 從 TIFF 結構讀 IFD0 的 Orientation（0x0112）；缺少、超出範圍或結構壞掉一律當作 1（不旋轉）。
func orientationOfTIFF(d []byte, off, n int) int {
	if n < 8 || off < 0 || off+n > len(d) {
		return 1
	}
	var le bool
	switch {
	case d[off] == 'I' && d[off+1] == 'I':
		le = true
	case d[off] == 'M' && d[off+1] == 'M':
		le = false
	default:
		return 1
	}
	u16 := func(i int) (int, bool) {
		if i < 0 || i+2 > len(d) {
			return 0, false
		}
		if le {
			return int(d[i+1])<<8 | int(d[i]), true
		}
		return int(d[i])<<8 | int(d[i+1]), true
	}
	u32 := func(i int) (uint32, bool) {
		if i < 0 || i+4 > len(d) {
			return 0, false
		}
		if le {
			return u32le(d, i), true
		}
		return u32be(d, i), true
	}
	ifd, ok := u32(off + 4)
	if !ok || ifd < 8 || int64(ifd)+2 > int64(n) {
		return 1
	}
	p := off + int(ifd)
	count, ok := u16(p)
	if !ok {
		return 1
	}
	for k := 0; k < count; k++ {
		e := p + 2 + 12*k
		if e+12 > off+n {
			return 1
		}
		tag, ok1 := u16(e)
		typ, ok2 := u16(e + 2)
		cnt, ok3 := u32(e + 4)
		if !ok1 || !ok2 || !ok3 {
			return 1
		}
		if tag == 0x0112 && typ == 3 && cnt == 1 {
			v, ok := u16(e + 8)
			if ok && v >= 2 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orientationTIFF 是只含 Orientation 一個欄位的最小 TIFF。
func orientationTIFF(o int) []byte {
	return []byte{
		'M', 'M', 0, 42, 0, 0, 0, 8,
		0, 1,
		0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, byte(o & 0xFF), 0, 0,
		0, 0, 0, 0,
	}
}
