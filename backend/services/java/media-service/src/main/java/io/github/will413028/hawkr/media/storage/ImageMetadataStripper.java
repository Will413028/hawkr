package io.github.will413028.hawkr.media.storage;

import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Set;
import java.util.zip.CRC32;

/**
 * 公開圖片的 metadata 剝除（EXIF、GPS、XMP、IPTC、縮圖、註解、文字區塊）。
 *
 * <p>為什麼存在：使用者上傳的原圖會被 image gateway 的 {@code original} 原樣直出，相機寫進檔案的 GPS 座標、
 * 裝置序號會跟著公開（ADR {@code 2026-09-25-api-image-uri-template.md}）。儲存層在寫入 {@code public/} 前
 * 呼叫這裡，所以儲存的位元組本身就不含這些資料，下載、後台與備份也一併涵蓋。
 *
 * <p>做法是「白名單 + 逐段搬運」，不解碼像素、不重新編碼，所以沒有畫質損失，也沒有新依賴：
 * 只留解碼與顯示必需的區段（影像資料、色彩描述檔、動畫控制），其餘一律丟棄，未知的區段預設丟棄。
 * 唯一保留的 metadata 是方向（EXIF Orientation）：它決定圖片顯示時要不要旋轉，丟掉會讓手機直拍的照片橫躺；
 * 保留時重寫成只含 Orientation 一個欄位的最小 EXIF，原本的 GPS、縮圖、MakerNote 都不會被帶過去。
 *
 * <p>格式：JPEG、PNG、WebP、GIF。無法辨認格式回傳 {@code null}（呼叫端照原樣處理）；辨認得出但結構壞掉拋
 * {@link IllegalArgumentException}（呼叫端必須拒絕，不可退回原檔，否則壞檔就是繞過剝除的路）。
 */
final class ImageMetadataStripper {

    private static final Set<String> PNG_KEEP = Set.of(
        "IHDR", "PLTE", "IDAT", "IEND", "tRNS", "gAMA", "cHRM", "sRGB", "iCCP", "sBIT", "bKGD", "hIST",
        "pHYs", "sPLT", "acTL", "fcTL", "fdAT", "cICP", "mDCV", "cLLI");

    private static final Set<String> WEBP_KEEP = Set.of("VP8 ", "VP8L", "VP8X", "ALPH", "ANIM", "ANMF", "ICCP");

    private static final int WEBP_FLAG_XMP = 0x04;
    private static final int WEBP_FLAG_EXIF = 0x08;

    private ImageMetadataStripper() {
    }

    /** 剝除 metadata。格式無法辨認回傳 null；辨認得出但結構壞掉拋 IllegalArgumentException。 */
    static byte[] strip(byte[] d) {
        if (d == null) {
            return null;
        }
        if (d.length >= 3 && u8(d, 0) == 0xFF && u8(d, 1) == 0xD8 && u8(d, 2) == 0xFF) {
            return jpeg(d);
        }
        if (startsWith(d, 0, new byte[] {(byte) 0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})) {
            return png(d);
        }
        if (d.length >= 12 && ascii(d, 0, 4).equals("RIFF") && ascii(d, 8, 4).equals("WEBP")) {
            return webp(d);
        }
        if (d.length >= 6 && (ascii(d, 0, 6).equals("GIF87a") || ascii(d, 0, 6).equals("GIF89a"))) {
            return gif(d);
        }
        return null;
    }

    // ---------------------------------------------------------------- JPEG

    private static byte[] jpeg(byte[] d) {
        List<byte[]> segs = new ArrayList<>();
        int orientation = 1;
        int i = 2;
        boolean sawEoi = false;
        while (i < d.length) {
            if (u8(d, i) != 0xFF) {
                throw bad("JPEG 區段標記錯誤");
            }
            int m = need(d, i + 1);
            if (m == 0xFF) { // 填充位元組
                i++;
                continue;
            }
            if (m == 0xD9) {
                sawEoi = true;
                break;
            }
            if (m == 0x01 || (m >= 0xD0 && m <= 0xD8)) { // 沒有長度的標記
                segs.add(Arrays.copyOfRange(d, i, i + 2));
                i += 2;
                continue;
            }
            int len = (need(d, i + 2) << 8) | need(d, i + 3);
            int end = i + 2 + len;
            if (len < 2 || end > d.length) {
                throw bad("JPEG 區段長度錯誤");
            }
            boolean keep = true;
            if (m >= 0xE0 && m <= 0xEF) {
                int p = i + 4;
                int n = len - 2;
                keep = switch (m) {
                    case 0xE0 -> startsWith(d, p, "JFIF\0", n);
                    case 0xE2 -> startsWith(d, p, "ICC_PROFILE\0", n);
                    case 0xEE -> startsWith(d, p, "Adobe", n);
                    default -> false;
                };
                if (m == 0xE1 && startsWith(d, p, "Exif\0\0", n)) {
                    orientation = orientationOfTiff(d, p + 6, n - 6);
                }
            } else if (m == 0xFE) {
                keep = false;
            }
            if (keep) {
                segs.add(Arrays.copyOfRange(d, i, end));
            }
            i = end;
            if (m == 0xDA) { // 掃描資料：複製到下一個真正的標記
                int j = i;
                while (true) {
                    if (j + 1 >= d.length) {
                        throw bad("JPEG 缺少結尾");
                    }
                    if (u8(d, j) != 0xFF) {
                        j++;
                        continue;
                    }
                    int n = u8(d, j + 1);
                    if (n == 0x00 || (n >= 0xD0 && n <= 0xD7)) {
                        j += 2;
                    } else if (n == 0xFF) {
                        j++;
                    } else {
                        break;
                    }
                }
                segs.add(Arrays.copyOfRange(d, i, j));
                i = j;
            }
        }
        if (!sawEoi) {
            throw bad("JPEG 缺少結尾");
        }
        ByteArrayOutputStream out = new ByteArrayOutputStream(d.length);
        out.write(0xFF);
        out.write(0xD8);
        // JFIF 規定緊接在 SOI 後，方向資料放在它後面
        int exifAt = !segs.isEmpty() && segs.get(0).length > 1 && (segs.get(0)[1] & 0xFF) == 0xE0 ? 1 : 0;
        for (int k = 0; k < segs.size(); k++) {
            if (k == exifAt && orientation > 1) {
                writeJpegExif(out, orientation);
            }
            out.writeBytes(segs.get(k));
        }
        if (exifAt >= segs.size() && orientation > 1) {
            writeJpegExif(out, orientation);
        }
        out.write(0xFF);
        out.write(0xD9);
        return out.toByteArray();
    }

    private static void writeJpegExif(ByteArrayOutputStream out, int orientation) {
        byte[] tiff = orientationTiff(orientation);
        int len = 2 + 6 + tiff.length;
        out.write(0xFF);
        out.write(0xE1);
        out.write(len >> 8);
        out.write(len & 0xFF);
        out.writeBytes("Exif\0\0".getBytes(StandardCharsets.ISO_8859_1));
        out.writeBytes(tiff);
    }

    // ---------------------------------------------------------------- PNG

    private static byte[] png(byte[] d) {
        List<byte[]> chunks = new ArrayList<>();
        List<String> types = new ArrayList<>();
        int orientation = 1;
        int i = 8;
        boolean sawIend = false;
        while (i < d.length && !sawIend) {
            if (i + 12 > d.length) {
                throw bad("PNG 區塊不完整");
            }
            long len = u32(d, i);
            if (len > d.length - i - 12L) {
                throw bad("PNG 區塊長度錯誤");
            }
            int end = i + 12 + (int) len;
            String type = ascii(d, i + 4, 4);
            if (chunks.isEmpty() && !type.equals("IHDR")) {
                throw bad("PNG 缺少 IHDR");
            }
            if (type.equals("eXIf")) {
                orientation = orientationOfTiff(d, i + 8, (int) len);
            } else if (PNG_KEEP.contains(type)) {
                chunks.add(Arrays.copyOfRange(d, i, end));
                types.add(type);
            }
            sawIend = type.equals("IEND");
            i = end;
        }
        if (!sawIend) {
            throw bad("PNG 缺少 IEND");
        }
        ByteArrayOutputStream out = new ByteArrayOutputStream(d.length);
        out.write(d, 0, 8);
        boolean exifWritten = orientation <= 1;
        for (int k = 0; k < chunks.size(); k++) {
            if (!exifWritten && types.get(k).equals("IDAT")) {
                writePngExif(out, orientation);
                exifWritten = true;
            }
            out.writeBytes(chunks.get(k));
        }
        return out.toByteArray();
    }

    private static void writePngExif(ByteArrayOutputStream out, int orientation) {
        byte[] tiff = orientationTiff(orientation);
        byte[] typeAndData = new byte[4 + tiff.length];
        System.arraycopy("eXIf".getBytes(StandardCharsets.ISO_8859_1), 0, typeAndData, 0, 4);
        System.arraycopy(tiff, 0, typeAndData, 4, tiff.length);
        CRC32 crc = new CRC32();
        crc.update(typeAndData);
        writeU32(out, tiff.length);
        out.writeBytes(typeAndData);
        writeU32(out, crc.getValue());
    }

    // ---------------------------------------------------------------- WebP

    private static byte[] webp(byte[] d) {
        long riffSize = u32le(d, 4);
        int limit = (int) Math.min(d.length, riffSize + 8);
        if (limit < 12) {
            throw bad("WebP 長度錯誤");
        }
        List<byte[]> chunks = new ArrayList<>();
        boolean hasVp8x = false;
        int orientation = 1;
        int i = 12;
        while (i < limit) {
            if (i + 8 > limit) {
                throw bad("WebP 區塊不完整");
            }
            String type = ascii(d, i, 4);
            long len = u32le(d, i + 4);
            long padded = len + (len & 1);
            if (i + 8 + len > limit) {
                throw bad("WebP 區塊長度錯誤");
            }
            int dataStart = i + 8;
            int end = (int) Math.min(limit, dataStart + padded);
            if (type.equals("EXIF")) {
                int p = dataStart;
                int n = (int) len;
                if (startsWith(d, p, "Exif\0\0", n)) {
                    p += 6;
                    n -= 6;
                }
                orientation = orientationOfTiff(d, p, n);
            } else if (WEBP_KEEP.contains(type)) {
                byte[] c = Arrays.copyOf(Arrays.copyOfRange(d, i, end), (int) (8 + padded));
                if (type.equals("VP8X")) {
                    if (len < 10) {
                        throw bad("WebP VP8X 長度錯誤");
                    }
                    hasVp8x = true;
                    c[8] = (byte) (c[8] & ~(WEBP_FLAG_EXIF | WEBP_FLAG_XMP));
                }
                chunks.add(c);
            }
            i = end;
        }
        boolean withExif = hasVp8x && orientation > 1;
        if (withExif) {
            for (byte[] c : chunks) {
                if (ascii(c, 0, 4).equals("VP8X")) {
                    c[8] = (byte) (c[8] | WEBP_FLAG_EXIF);
                }
            }
        }
        ByteArrayOutputStream body = new ByteArrayOutputStream(d.length);
        for (byte[] c : chunks) {
            body.writeBytes(c);
        }
        if (withExif) {
            byte[] tiff = orientationTiff(orientation);
            body.writeBytes("EXIF".getBytes(StandardCharsets.ISO_8859_1));
            writeU32le(body, tiff.length);
            body.writeBytes(tiff);
        }
        ByteArrayOutputStream out = new ByteArrayOutputStream(body.size() + 12);
        out.writeBytes("RIFF".getBytes(StandardCharsets.ISO_8859_1));
        writeU32le(out, body.size() + 4);
        out.writeBytes("WEBP".getBytes(StandardCharsets.ISO_8859_1));
        out.writeBytes(body.toByteArray());
        return out.toByteArray();
    }

    // ---------------------------------------------------------------- GIF

    private static byte[] gif(byte[] d) {
        ByteArrayOutputStream out = new ByteArrayOutputStream(d.length);
        int flags = need(d, 10);
        int i = 13;
        if ((flags & 0x80) != 0) {
            i += 3 * (1 << ((flags & 0x07) + 1));
        }
        if (i > d.length) {
            throw bad("GIF 色盤不完整");
        }
        out.write(d, 0, i);
        while (true) {
            int b = need(d, i);
            if (b == 0x3B) {
                out.write(0x3B);
                return out.toByteArray();
            }
            if (b == 0x2C) {
                int start = i;
                int lflags = need(d, i + 9);
                i += 10;
                if ((lflags & 0x80) != 0) {
                    i += 3 * (1 << ((lflags & 0x07) + 1));
                }
                i += 1; // LZW 最小碼長
                i = skipSubBlocks(d, i);
                out.write(d, start, i - start);
            } else if (b == 0x21) {
                int start = i;
                int label = need(d, i + 1);
                boolean keep = label == 0xF9; // graphic control
                if (label == 0xFF) {
                    need(d, i + 14);
                    String id = ascii(d, i + 3, 11);
                    keep = id.equals("NETSCAPE2.0") || id.equals("ANIMEXTS1.0");
                }
                i = skipSubBlocks(d, i + 2);
                if (keep) {
                    out.write(d, start, i - start);
                }
            } else {
                throw bad("GIF 區塊標記錯誤");
            }
        }
    }

    private static int skipSubBlocks(byte[] d, int i) {
        while (true) {
            int len = need(d, i);
            i += 1 + len;
            if (len == 0) {
                return i;
            }
        }
    }

    // ---------------------------------------------------------------- EXIF orientation

    /** 從 TIFF 結構讀 IFD0 的 Orientation（0x0112）；缺少、超出範圍或結構壞掉一律當作 1（不旋轉）。 */
    static int orientationOfTiff(byte[] d, int off, int len) {
        try {
            if (len < 8 || off < 0 || off + len > d.length) {
                return 1;
            }
            boolean le;
            if (d[off] == 'I' && d[off + 1] == 'I') {
                le = true;
            } else if (d[off] == 'M' && d[off + 1] == 'M') {
                le = false;
            } else {
                return 1;
            }
            long ifd = tiffU32(d, off + 4, le);
            if (ifd < 8 || ifd + 2 > len) {
                return 1;
            }
            int p = off + (int) ifd;
            int count = tiffU16(d, p, le);
            for (int k = 0; k < count; k++) {
                int e = p + 2 + 12 * k;
                if (e + 12 > off + len) {
                    return 1;
                }
                if (tiffU16(d, e, le) == 0x0112 && tiffU16(d, e + 2, le) == 3 && tiffU32(d, e + 4, le) == 1) {
                    int v = tiffU16(d, e + 8, le);
                    return v >= 2 && v <= 8 ? v : 1;
                }
            }
        } catch (RuntimeException ex) {
            return 1;
        }
        return 1;
    }

    private static byte[] orientationTiff(int o) {
        return new byte[] {
            'M', 'M', 0, 42, 0, 0, 0, 8, // 表頭，IFD0 在位移 8
            0, 1, // 一個欄位
            0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, (byte) o, 0, 0, // Orientation：SHORT，count 1
            0, 0, 0, 0}; // 沒有 IFD1
    }

    // ---------------------------------------------------------------- helpers

    private static IllegalArgumentException bad(String msg) {
        return new IllegalArgumentException(msg);
    }

    private static int u8(byte[] d, int i) {
        return d[i] & 0xFF;
    }

    private static int need(byte[] d, int i) {
        if (i < 0 || i >= d.length) {
            throw bad("圖片資料不完整");
        }
        return d[i] & 0xFF;
    }

    private static long u32(byte[] d, int i) {
        return ((long) u8(d, i) << 24) | (u8(d, i + 1) << 16) | (u8(d, i + 2) << 8) | u8(d, i + 3);
    }

    private static long u32le(byte[] d, int i) {
        return ((long) u8(d, i + 3) << 24) | (u8(d, i + 2) << 16) | (u8(d, i + 1) << 8) | u8(d, i);
    }

    private static int tiffU16(byte[] d, int i, boolean le) {
        return le ? (u8(d, i + 1) << 8) | u8(d, i) : (u8(d, i) << 8) | u8(d, i + 1);
    }

    private static long tiffU32(byte[] d, int i, boolean le) {
        return le ? u32le(d, i) : u32(d, i);
    }

    private static void writeU32(ByteArrayOutputStream out, long v) {
        out.write((int) (v >> 24) & 0xFF);
        out.write((int) (v >> 16) & 0xFF);
        out.write((int) (v >> 8) & 0xFF);
        out.write((int) v & 0xFF);
    }

    private static void writeU32le(ByteArrayOutputStream out, long v) {
        out.write((int) v & 0xFF);
        out.write((int) (v >> 8) & 0xFF);
        out.write((int) (v >> 16) & 0xFF);
        out.write((int) (v >> 24) & 0xFF);
    }

    private static String ascii(byte[] d, int off, int n) {
        if (off < 0 || off + n > d.length) {
            throw bad("圖片資料不完整");
        }
        return new String(d, off, n, StandardCharsets.ISO_8859_1);
    }

    private static boolean startsWith(byte[] d, int off, byte[] prefix) {
        if (off + prefix.length > d.length) {
            return false;
        }
        for (int k = 0; k < prefix.length; k++) {
            if (d[off + k] != prefix[k]) {
                return false;
            }
        }
        return true;
    }

    private static boolean startsWith(byte[] d, int off, String prefix, int available) {
        return available >= prefix.length()
            && startsWith(d, off, prefix.getBytes(StandardCharsets.ISO_8859_1));
    }
}
