package image

import "bytes"

func stripGIF(d []byte) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(len(d))
	need := func(i int) (int, error) {
		if i < 0 || i >= len(d) {
			return 0, bad("圖片資料不完整")
		}
		return int(d[i]), nil
	}
	flags, err := need(10)
	if err != nil {
		return nil, err
	}
	i := 13
	if flags&0x80 != 0 {
		i += 3 * (1 << ((flags & 0x07) + 1))
	}
	if i > len(d) {
		return nil, bad("GIF 色盤不完整")
	}
	out.Write(d[:i])
	for {
		b, err := need(i)
		if err != nil {
			return nil, err
		}
		switch b {
		case 0x3B:
			out.WriteByte(0x3B)
			return out.Bytes(), nil
		case 0x2C:
			start := i
			lflags, err := need(i + 9)
			if err != nil {
				return nil, err
			}
			i += 10
			if lflags&0x80 != 0 {
				i += 3 * (1 << ((lflags & 0x07) + 1))
			}
			i++ // LZW 最小碼長
			if i, err = skipSubBlocks(d, i); err != nil {
				return nil, err
			}
			out.Write(d[start:i])
		case 0x21:
			start := i
			label, err := need(i + 1)
			if err != nil {
				return nil, err
			}
			keep := label == 0xF9 // graphic control
			if label == 0xFF {
				if _, err := need(i + 14); err != nil {
					return nil, err
				}
				id := string(d[i+3 : i+14])
				keep = id == "NETSCAPE2.0" || id == "ANIMEXTS1.0"
			}
			if i, err = skipSubBlocks(d, i+2); err != nil {
				return nil, err
			}
			if keep {
				out.Write(d[start:i])
			}
		default:
			return nil, bad("GIF 區塊標記錯誤")
		}
	}
}

func skipSubBlocks(d []byte, i int) (int, error) {
	for {
		if i < 0 || i >= len(d) {
			return 0, bad("圖片資料不完整")
		}
		n := int(d[i])
		i += 1 + n
		if n == 0 {
			return i, nil
		}
	}
}
