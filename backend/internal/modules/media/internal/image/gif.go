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
			end, err := skipSubBlocks(d, i+2)
			if err != nil {
				return nil, err
			}
			switch label {
			case 0xF9: // graphic control：固定 4 位元組，多餘的子區塊丟棄
				if d[i+2] != 4 {
					return nil, bad("GIF 圖形控制區塊長度錯誤")
				}
				out.Write([]byte{0x21, 0xF9, 0x04})
				out.Write(d[i+3 : i+7])
				out.WriteByte(0)
			case 0x01: // plain text：顯示的文字內容，不是 metadata，整段保留
				out.Write(d[start:end])
			case 0xFF: // application：只留動畫迴圈，其餘廠商資料（XMP 等）丟棄
				if d[i+2] != 11 {
					break
				}
				id := string(d[i+3 : i+14])
				if id != "NETSCAPE2.0" && id != "ANIMEXTS1.0" {
					break
				}
				if d[i+14] != 3 {
					return nil, bad("GIF 動畫迴圈區塊長度錯誤")
				}
				if d[i+15] != 1 {
					break
				}
				out.Write([]byte{0x21, 0xFF, 0x0B})
				out.WriteString(id)
				out.Write([]byte{0x03, 0x01, d[i+16], d[i+17], 0x00})
			}
			i = end
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
