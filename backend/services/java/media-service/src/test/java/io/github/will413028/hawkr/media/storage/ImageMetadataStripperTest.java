package io.github.will413028.hawkr.media.storage;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import javax.imageio.ImageIO;
import java.awt.image.BufferedImage;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;
import java.util.zip.CRC32;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 公開圖片 metadata 剝除：夾帶 GPS 的 fixture 剝除後，位元組裡不能再出現任何敏感內容，
 * 影像資料不變（不重新編碼），方向保留。
 */
class ImageMetadataStripperTest {

    private static final String GPS_MARKER = "GPS-25.0330N-121.5654E-SERIAL-A1B2C3";

    private static BufferedImage image() {
        BufferedImage img = new BufferedImage(16, 16, BufferedImage.TYPE_INT_RGB);
        for (int y = 0; y < 16; y++) {
            for (int x = 0; x < 16; x++) {
                img.setRGB(x, y, (x * 16) << 16 | (y * 16) << 8 | 0x40);
            }
        }
        return img;
    }

    private static byte[] encode(String format) throws Exception {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        assertTrue(ImageIO.write(image(), format, out));
        return out.toByteArray();
    }

    private static boolean contains(byte[] d, String s) {
        String hay = new String(d, StandardCharsets.ISO_8859_1);
        return hay.contains(s);
    }

    /** 含 Orientation、GPS IFD（指標指向 ASCII 欄位）與 IFD1 的小端 TIFF，內文帶有可搜尋的敏感字串。 */
    private static byte[] exifTiff(int orientation) {
        byte[] markerBytes = (GPS_MARKER + "\0").getBytes(StandardCharsets.ISO_8859_1);
        ByteArrayOutputStream o = new ByteArrayOutputStream();
        o.writeBytes(new byte[] {'I', 'I', 42, 0, 8, 0, 0, 0}); // IFD0 @8
        o.writeBytes(new byte[] {2, 0}); // 兩個欄位
        o.writeBytes(new byte[] {0x12, 0x01, 3, 0, 1, 0, 0, 0, (byte) orientation, 0, 0, 0}); // Orientation
        o.writeBytes(new byte[] {(byte) 0x25, (byte) 0x88, 4, 0, 1, 0, 0, 0, 38, 0, 0, 0}); // GPS IFD @38
        o.writeBytes(new byte[] {0, 0, 0, 0}); // next IFD
        // @38：GPS IFD，一個 ASCII 欄位指向後面的字串
        o.writeBytes(new byte[] {1, 0});
        o.writeBytes(new byte[] {0x01, 0x00, 2, 0, (byte) markerBytes.length, 0, 0, 0, 56, 0, 0, 0});
        o.writeBytes(new byte[] {0, 0, 0, 0});
        o.writeBytes(markerBytes);
        return o.toByteArray();
    }

    private static byte[] jpegSegment(int marker, byte[] payload) {
        ByteArrayOutputStream o = new ByteArrayOutputStream();
        int len = payload.length + 2;
        o.write(0xFF);
        o.write(marker);
        o.write(len >> 8);
        o.write(len & 0xFF);
        o.writeBytes(payload);
        return o.toByteArray();
    }

    private static byte[] cat(byte[]... parts) {
        ByteArrayOutputStream o = new ByteArrayOutputStream();
        for (byte[] p : parts) {
            o.writeBytes(p);
        }
        return o.toByteArray();
    }

    private static byte[] jpegWithMetadata(int orientation) throws Exception {
        byte[] plain = encode("jpeg");
        byte[] exif = jpegSegment(0xE1, cat("Exif\0\0".getBytes(StandardCharsets.ISO_8859_1), exifTiff(orientation)));
        byte[] xmp = jpegSegment(0xE1, ("http://ns.adobe.com/xap/1.0/\0<x:xmpmeta>" + GPS_MARKER + "</x:xmpmeta>")
            .getBytes(StandardCharsets.ISO_8859_1));
        byte[] iptc = jpegSegment(0xED, ("Photoshop 3.0\0" + GPS_MARKER).getBytes(StandardCharsets.ISO_8859_1));
        byte[] com = jpegSegment(0xFE, GPS_MARKER.getBytes(StandardCharsets.ISO_8859_1));
        byte[] trailer = ("TRAILER" + GPS_MARKER).getBytes(StandardCharsets.ISO_8859_1);
        // SOI + (JFIF) 之後插入所有 metadata，EOI 後再接尾巴
        int insertAt = 2;
        if ((plain[2] & 0xFF) == 0xFF && (plain[3] & 0xFF) == 0xE0) {
            insertAt = 4 + (((plain[4] & 0xFF) << 8) | (plain[5] & 0xFF));
        }
        byte[] head = java.util.Arrays.copyOfRange(plain, 0, insertAt);
        byte[] rest = java.util.Arrays.copyOfRange(plain, insertAt, plain.length);
        return cat(head, exif, xmp, iptc, com, rest, trailer);
    }

    @Test
    @DisplayName("JPEG：GPS、XMP、IPTC、註解與 EOI 後的尾巴都被移除，像素不變")
    void jpegIsStrippedLosslessly() throws Exception {
        byte[] in = jpegWithMetadata(1);
        assertTrue(contains(in, GPS_MARKER), "fixture 必須含敏感字串");
        byte[] out = ImageMetadataStripper.strip(in);
        assertNotNull(out);
        assertFalse(contains(out, GPS_MARKER));
        assertFalse(contains(out, "Exif"));
        assertFalse(contains(out, "xmp"));
        assertFalse(contains(out, "Photoshop"));
        assertFalse(contains(out, "TRAILER"));
        BufferedImage a = ImageIO.read(new ByteArrayInputStream(in));
        BufferedImage b = ImageIO.read(new ByteArrayInputStream(out));
        assertNotNull(b);
        for (int y = 0; y < 16; y++) {
            for (int x = 0; x < 16; x++) {
                assertEquals(a.getRGB(x, y), b.getRGB(x, y));
            }
        }
    }

    @Test
    @DisplayName("JPEG：方向 6 保留成只含 Orientation 的最小 EXIF，不帶 GPS")
    void jpegKeepsOnlyOrientation() throws Exception {
        byte[] out = ImageMetadataStripper.strip(jpegWithMetadata(6));
        assertFalse(contains(out, GPS_MARKER));
        int at = new String(out, StandardCharsets.ISO_8859_1).indexOf("Exif\0\0");
        assertTrue(at > 0, "方向大於 1 時要保留 EXIF 區段");
        assertEquals(6, ImageMetadataStripper.orientationOfTiff(out, at + 6, out.length - at - 6));
        assertEquals(1, new String(out, StandardCharsets.ISO_8859_1).split("Exif\0\0", -1).length - 1);
        assertNotNull(ImageIO.read(new ByteArrayInputStream(out)));
    }

    @Test
    @DisplayName("JPEG：方向 1（不旋轉）時完全沒有 EXIF 區段")
    void jpegWithDefaultOrientationHasNoExif() throws Exception {
        byte[] out = ImageMetadataStripper.strip(jpegWithMetadata(1));
        assertFalse(contains(out, "Exif"));
    }

    @Test
    @DisplayName("JPEG：結構壞掉拋例外，不退回原檔")
    void truncatedJpegIsRejected() throws Exception {
        byte[] in = jpegWithMetadata(1);
        byte[] cut = java.util.Arrays.copyOf(in, in.length / 2);
        assertThrows(IllegalArgumentException.class, () -> ImageMetadataStripper.strip(cut));
    }

    private static byte[] pngChunk(String type, byte[] data) {
        byte[] t = type.getBytes(StandardCharsets.ISO_8859_1);
        CRC32 crc = new CRC32();
        crc.update(t);
        crc.update(data);
        ByteArrayOutputStream o = new ByteArrayOutputStream();
        o.writeBytes(new byte[] {(byte) (data.length >> 24), (byte) (data.length >> 16), (byte) (data.length >> 8),
            (byte) data.length});
        o.writeBytes(t);
        o.writeBytes(data);
        long c = crc.getValue();
        o.writeBytes(new byte[] {(byte) (c >> 24), (byte) (c >> 16), (byte) (c >> 8), (byte) c});
        return o.toByteArray();
    }

    @Test
    @DisplayName("PNG：文字區塊、時間與含 GPS 的 eXIf 被移除，方向保留為最小 eXIf，仍可解碼")
    void pngIsStripped() throws Exception {
        byte[] plain = encode("png");
        int afterIhdr = 8 + 12 + 13;
        byte[] withMeta = cat(
            java.util.Arrays.copyOfRange(plain, 0, afterIhdr),
            pngChunk("tEXt", ("Comment\0" + GPS_MARKER).getBytes(StandardCharsets.ISO_8859_1)),
            pngChunk("iTXt", ("XML:com.adobe.xmp\0\0\0\0\0" + GPS_MARKER).getBytes(StandardCharsets.ISO_8859_1)),
            pngChunk("tIME", new byte[] {7, (byte) 0xEA, 1, 1, 1, 1, 1}),
            pngChunk("eXIf", exifTiff(8)),
            java.util.Arrays.copyOfRange(plain, afterIhdr, plain.length));
        assertTrue(contains(withMeta, GPS_MARKER));
        byte[] out = ImageMetadataStripper.strip(withMeta);
        assertFalse(contains(out, GPS_MARKER));
        assertFalse(contains(out, "tEXt"));
        assertFalse(contains(out, "iTXt"));
        assertFalse(contains(out, "tIME"));
        String s = new String(out, StandardCharsets.ISO_8859_1);
        int at = s.indexOf("eXIf");
        assertTrue(at > 0 && at < s.indexOf("IDAT"), "最小 eXIf 要在 IDAT 之前");
        assertEquals(8, ImageMetadataStripper.orientationOfTiff(out, at + 4, 26));
        // CRC 正確
        CRC32 crc = new CRC32();
        crc.update(out, at, 4 + 26);
        long stored = ((long) (out[at + 30] & 0xFF) << 24) | ((out[at + 31] & 0xFF) << 16)
            | ((out[at + 32] & 0xFF) << 8) | (out[at + 33] & 0xFF);
        assertEquals(crc.getValue(), stored);
        BufferedImage a = ImageIO.read(new ByteArrayInputStream(plain));
        BufferedImage b = ImageIO.read(new ByteArrayInputStream(out));
        for (int y = 0; y < 16; y++) {
            for (int x = 0; x < 16; x++) {
                assertEquals(a.getRGB(x, y), b.getRGB(x, y));
            }
        }
    }

    private static byte[] le32(int v) {
        return new byte[] {(byte) v, (byte) (v >> 8), (byte) (v >> 16), (byte) (v >> 24)};
    }

    private static byte[] riffChunk(String type, byte[] data) {
        byte[] pad = data.length % 2 == 1 ? new byte[1] : new byte[0];
        return cat(type.getBytes(StandardCharsets.ISO_8859_1), le32(data.length), data, pad);
    }

    private static byte[] webp(byte[]... chunks) {
        byte[] body = cat(chunks);
        return cat("RIFF".getBytes(StandardCharsets.ISO_8859_1), le32(body.length + 4),
            "WEBP".getBytes(StandardCharsets.ISO_8859_1), body);
    }

    @Test
    @DisplayName("WebP：EXIF、XMP 被移除且 VP8X 旗標清掉，方向保留，影像區塊逐位元組不變")
    void webpIsStripped() {
        byte[] vp8x = new byte[10];
        vp8x[0] = 0x08 | 0x04 | 0x20; // EXIF + XMP + ICC
        byte[] image = {1, 2, 3, 4, 5};
        byte[] in = webp(
            riffChunk("VP8X", vp8x),
            riffChunk("VP8 ", image),
            riffChunk("EXIF", exifTiff(3)),
            riffChunk("XMP ", ("<x>" + GPS_MARKER + "</x>").getBytes(StandardCharsets.ISO_8859_1)));
        byte[] out = ImageMetadataStripper.strip(in);
        assertFalse(contains(out, GPS_MARKER));
        assertFalse(contains(out, "XMP "));
        assertEquals("RIFF", new String(out, 0, 4, StandardCharsets.ISO_8859_1));
        assertEquals(out.length - 8, (out[4] & 0xFF) | (out[5] & 0xFF) << 8 | (out[6] & 0xFF) << 16);
        assertEquals(0x08 | 0x20, out[20] & 0xFF, "XMP 旗標清掉，EXIF 旗標因為保留方向而在，ICC 不動");
        int at = new String(out, StandardCharsets.ISO_8859_1).indexOf("EXIF");
        assertTrue(at > 0);
        assertEquals(26, (out[at + 4] & 0xFF));
        assertEquals(3, ImageMetadataStripper.orientationOfTiff(out, at + 8, 26));
        assertTrue(contains(out, new String(riffChunk("VP8 ", image), StandardCharsets.ISO_8859_1)));
    }

    @Test
    @DisplayName("WebP：方向 1 時不留 EXIF，旗標也清掉")
    void webpWithDefaultOrientationHasNoExif() {
        byte[] vp8x = new byte[10];
        vp8x[0] = 0x08;
        byte[] out = ImageMetadataStripper.strip(webp(
            riffChunk("VP8X", vp8x), riffChunk("VP8 ", new byte[] {9, 9}), riffChunk("EXIF", exifTiff(1))));
        assertFalse(contains(out, "EXIF"));
        assertEquals(0, out[20] & 0xFF);
    }

    @Test
    @DisplayName("GIF：註解與 XMP 應用擴充被移除，動畫與圖形控制擴充保留，仍可解碼")
    void gifIsStripped() throws Exception {
        byte[] plain = encode("gif");
        int i = 13;
        int flags = plain[10] & 0xFF;
        if ((flags & 0x80) != 0) {
            i += 3 * (1 << ((flags & 0x07) + 1));
        }
        byte[] markerBytes = GPS_MARKER.getBytes(StandardCharsets.ISO_8859_1);
        byte[] comment = cat(new byte[] {0x21, (byte) 0xFE, (byte) markerBytes.length}, markerBytes, new byte[] {0});
        byte[] xmp = cat(new byte[] {0x21, (byte) 0xFF, 11}, "XMP DataXMP".getBytes(StandardCharsets.ISO_8859_1),
            new byte[] {(byte) markerBytes.length}, markerBytes, new byte[] {0});
        byte[] in = cat(java.util.Arrays.copyOfRange(plain, 0, i), comment, xmp,
            java.util.Arrays.copyOfRange(plain, i, plain.length));
        byte[] out = ImageMetadataStripper.strip(in);
        assertFalse(contains(out, GPS_MARKER));
        assertFalse(contains(out, "XMP Data"));
        BufferedImage a = ImageIO.read(new ByteArrayInputStream(plain));
        BufferedImage b = ImageIO.read(new ByteArrayInputStream(out));
        assertNotNull(b);
        assertEquals(a.getRGB(3, 3), b.getRGB(3, 3));
        assertArrayEquals(plain, ImageMetadataStripper.strip(plain), "沒有 metadata 的 GIF 逐位元組不變");
    }

    @Test
    @DisplayName("無法辨認的格式回傳 null")
    void unknownFormatIsNull() {
        assertNull(ImageMetadataStripper.strip("%PDF-1.7".getBytes(StandardCharsets.ISO_8859_1)));
        assertNull(ImageMetadataStripper.strip(new byte[0]));
    }
}
