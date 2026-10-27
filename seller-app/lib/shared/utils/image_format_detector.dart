/// 圖片格式偵測工具
///
/// 支援 JPEG, PNG, GIF, WebP, HEIF/HEIC (iPhone), AVIF
class ImageFormatDetector {
  ImageFormatDetector._();

  /// 偵測圖片格式，回傳 (副檔名, MIME type)
  static (String, String) detect(List<int> bytes) {
    if (bytes.length < 12) return ('jpg', 'image/jpeg');

    // JPEG: FF D8 FF
    if (bytes[0] == 0xFF && bytes[1] == 0xD8 && bytes[2] == 0xFF) {
      return ('jpg', 'image/jpeg');
    }

    // PNG: 89 50 4E 47 0D 0A 1A 0A
    if (bytes[0] == 0x89 &&
        bytes[1] == 0x50 &&
        bytes[2] == 0x4E &&
        bytes[3] == 0x47) {
      return ('png', 'image/png');
    }

    // GIF: 47 49 46 38
    if (bytes[0] == 0x47 &&
        bytes[1] == 0x49 &&
        bytes[2] == 0x46 &&
        bytes[3] == 0x38) {
      return ('gif', 'image/gif');
    }

    // WebP: 52 49 46 46 ... 57 45 42 50 (RIFF....WEBP)
    if (bytes[0] == 0x52 &&
        bytes[1] == 0x49 &&
        bytes[2] == 0x46 &&
        bytes[3] == 0x46 &&
        bytes.length > 11 &&
        bytes[8] == 0x57 &&
        bytes[9] == 0x45 &&
        bytes[10] == 0x42 &&
        bytes[11] == 0x50) {
      return ('webp', 'image/webp');
    }

    // HEIF/HEIC: ftyp box at offset 4, brand = heic, heix, hevc, hevx, mif1, msf1
    // Format: [4 bytes size][4 bytes 'ftyp'][4 bytes brand]
    if (bytes.length > 11 &&
        bytes[4] == 0x66 &&
        bytes[5] == 0x74 &&
        bytes[6] == 0x79 &&
        bytes[7] == 0x70) {
      // 'ftyp'
      final brand = String.fromCharCodes(bytes.sublist(8, 12));
      if (brand == 'heic' || brand == 'heix' || brand == 'mif1') {
        return ('heic', 'image/heic');
      }
      if (brand == 'hevc' || brand == 'hevx') {
        return ('heic', 'image/heic');
      }
      if (brand == 'avif') {
        return ('avif', 'image/avif');
      }
    }

    // Default to JPEG
    return ('jpg', 'image/jpeg');
  }

  /// 從檔名副檔名取得 MIME type
  static String mimeFromExtension(String filename) {
    final ext = filename.split('.').last.toLowerCase();
    switch (ext) {
      case 'jpg':
      case 'jpeg':
        return 'image/jpeg';
      case 'png':
        return 'image/png';
      case 'gif':
        return 'image/gif';
      case 'webp':
        return 'image/webp';
      case 'heic':
      case 'heif':
        return 'image/heic';
      case 'avif':
        return 'image/avif';
      default:
        return 'image/jpeg';
    }
  }
}
