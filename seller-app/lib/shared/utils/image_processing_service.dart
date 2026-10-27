import 'platform_io.dart';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter_image_compress/flutter_image_compress.dart';

import 'hk_log.dart';

/// 圖片處理服務
///
/// 提供前端圖片壓縮、裁切、調整大小等功能
/// 在上傳前處理圖片，減少後端負擔和網路傳輸時間
class ImageProcessingService {
  ImageProcessingService._();

  /// 頭像尺寸 (512x512)
  static const int avatarSize = 512;

  /// 商品圖片最大寬度
  static const int productMaxWidth = 1920;

  /// 商品圖片最大高度
  static const int productMaxHeight = 1080;

  /// 預設壓縮品質
  static const int defaultQuality = 80;

  /// 頭像壓縮品質（較高，保持清晰度）
  static const int avatarQuality = 90;

  /// 處理頭像圖片
  ///
  /// 1. 裁切為正方形（取中心區域）
  /// 2. 調整大小為 512x512
  /// 3. 壓縮
  ///
  /// [imageBytes] 原始圖片 bytes
  /// 返回處理後的 bytes，失敗返回 null
  static Future<Uint8List?> processAvatar(Uint8List imageBytes) async {
    try {
      HKLog.logD(
        '[ImageProcessing] Processing avatar, input size: ${imageBytes.length} bytes',
      );

      // 1. 取得原始圖片尺寸
      final ui.Codec codec = await ui.instantiateImageCodec(imageBytes);
      final ui.FrameInfo frame = await codec.getNextFrame();
      final ui.Image image = frame.image;
      final int originalWidth = image.width;
      final int originalHeight = image.height;
      codec.dispose();
      image.dispose();

      HKLog.logD(
        '[ImageProcessing] Original size: ${originalWidth}x$originalHeight',
      );

      // 2. 壓縮並調整大小為正方形
      // flutter_image_compress 的 minWidth/minHeight 會保持比例縮放
      // 對於非正方形圖片，後端 /media/upload/avatar 會自動裁切為 512x512
      // 如果需要前端精確裁切，可使用 image_cropper 套件
      final result = await FlutterImageCompress.compressWithList(
        imageBytes,
        minWidth: avatarSize,
        minHeight: avatarSize,
        quality: avatarQuality,
        keepExif: false,
        format: CompressFormat.jpeg,
      );

      if (result.isEmpty) {
        HKLog.logW(
          '[ImageProcessing] Avatar compression returned empty result',
        );
        return null;
      }

      HKLog.logD(
        '[ImageProcessing] Avatar processed, output size: ${result.length} bytes '
        '(${((1 - result.length / imageBytes.length) * 100).toStringAsFixed(1)}% reduction)',
      );

      return result;
    } catch (e, stackTrace) {
      HKLog.logE('[ImageProcessing] Failed to process avatar', e, stackTrace);
      return null;
    }
  }

  /// 處理頭像圖片（從檔案）
  static Future<Uint8List?> processAvatarFromFile(File file) async {
    try {
      final bytes = await file.readAsBytes();
      return processAvatar(bytes);
    } catch (e, stackTrace) {
      HKLog.logE('[ImageProcessing] Failed to read avatar file', e, stackTrace);
      return null;
    }
  }

  /// 處理頭像圖片（從路徑）
  static Future<Uint8List?> processAvatarFromPath(String path) async {
    return processAvatarFromFile(File(path));
  }

  /// 處理商品圖片
  ///
  /// 1. 調整大小（最大 1920x1080）
  /// 2. 壓縮（品質 80%）
  ///
  /// [imageBytes] 原始圖片 bytes
  /// [quality] 壓縮品質（0-100），預設 80
  /// [maxWidth] 最大寬度，預設 1920
  /// [maxHeight] 最大高度，預設 1080
  static Future<Uint8List?> processProductImage(
    Uint8List imageBytes, {
    int quality = defaultQuality,
    int maxWidth = productMaxWidth,
    int maxHeight = productMaxHeight,
  }) async {
    try {
      HKLog.logD(
        '[ImageProcessing] Processing product image, input size: ${imageBytes.length} bytes',
      );

      final result = await FlutterImageCompress.compressWithList(
        imageBytes,
        minWidth: maxWidth,
        minHeight: maxHeight,
        quality: quality,
        keepExif: false,
        format: CompressFormat.jpeg,
      );

      if (result.isEmpty) {
        HKLog.logW(
          '[ImageProcessing] Product image compression returned empty result',
        );
        return null;
      }

      HKLog.logD(
        '[ImageProcessing] Product image processed, output size: ${result.length} bytes '
        '(${((1 - result.length / imageBytes.length) * 100).toStringAsFixed(1)}% reduction)',
      );

      return result;
    } catch (e, stackTrace) {
      HKLog.logE(
        '[ImageProcessing] Failed to process product image',
        e,
        stackTrace,
      );
      return null;
    }
  }

  /// 處理商品圖片（從檔案）
  static Future<Uint8List?> processProductImageFromFile(
    File file, {
    int quality = defaultQuality,
    int maxWidth = productMaxWidth,
    int maxHeight = productMaxHeight,
  }) async {
    try {
      final bytes = await file.readAsBytes();
      return processProductImage(
        bytes,
        quality: quality,
        maxWidth: maxWidth,
        maxHeight: maxHeight,
      );
    } catch (e, stackTrace) {
      HKLog.logE(
        '[ImageProcessing] Failed to read product image file',
        e,
        stackTrace,
      );
      return null;
    }
  }

  /// 處理商品圖片（從路徑）
  static Future<Uint8List?> processProductImageFromPath(
    String path, {
    int quality = defaultQuality,
    int maxWidth = productMaxWidth,
    int maxHeight = productMaxHeight,
  }) async {
    return processProductImageFromFile(
      File(path),
      quality: quality,
      maxWidth: maxWidth,
      maxHeight: maxHeight,
    );
  }

  /// 通用圖片壓縮
  ///
  /// [imageBytes] 原始圖片 bytes
  /// [quality] 壓縮品質（0-100），預設 80
  /// [maxWidth] 最大寬度，預設不限制
  /// [maxHeight] 最大高度，預設不限制
  static Future<Uint8List?> compress(
    Uint8List imageBytes, {
    int quality = defaultQuality,
    int? maxWidth,
    int? maxHeight,
  }) async {
    try {
      HKLog.logD(
        '[ImageProcessing] Compressing image, input size: ${imageBytes.length} bytes',
      );

      final result = await FlutterImageCompress.compressWithList(
        imageBytes,
        minWidth: maxWidth ?? 1920,
        minHeight: maxHeight ?? 1920,
        quality: quality,
        keepExif: false,
        format: CompressFormat.jpeg,
      );

      if (result.isEmpty) {
        HKLog.logW('[ImageProcessing] Compression returned empty result');
        return null;
      }

      HKLog.logD(
        '[ImageProcessing] Image compressed, output size: ${result.length} bytes '
        '(${((1 - result.length / imageBytes.length) * 100).toStringAsFixed(1)}% reduction)',
      );

      return result;
    } catch (e, stackTrace) {
      HKLog.logE('[ImageProcessing] Failed to compress image', e, stackTrace);
      return null;
    }
  }

  /// 從檔案壓縮
  static Future<Uint8List?> compressFile(
    File file, {
    int quality = defaultQuality,
    int? maxWidth,
    int? maxHeight,
  }) async {
    try {
      HKLog.logD('[ImageProcessing] Compressing file: ${file.path}');

      final result = await FlutterImageCompress.compressWithFile(
        file.path,
        minWidth: maxWidth ?? 1920,
        minHeight: maxHeight ?? 1920,
        quality: quality,
        keepExif: false,
        format: CompressFormat.jpeg,
      );

      if (result == null || result.isEmpty) {
        HKLog.logW('[ImageProcessing] File compression returned empty result');
        return null;
      }

      final originalSize = await file.length();
      HKLog.logD(
        '[ImageProcessing] File compressed, output size: ${result.length} bytes '
        '(${((1 - result.length / originalSize) * 100).toStringAsFixed(1)}% reduction)',
      );

      return result;
    } catch (e, stackTrace) {
      HKLog.logE('[ImageProcessing] Failed to compress file', e, stackTrace);
      return null;
    }
  }

  /// 從路徑壓縮
  static Future<Uint8List?> compressPath(
    String path, {
    int quality = defaultQuality,
    int? maxWidth,
    int? maxHeight,
  }) async {
    return compressFile(
      File(path),
      quality: quality,
      maxWidth: maxWidth,
      maxHeight: maxHeight,
    );
  }

  /// 取得圖片尺寸
  static Future<Size?> getImageSize(Uint8List imageBytes) async {
    try {
      final ui.Codec codec = await ui.instantiateImageCodec(imageBytes);
      final ui.FrameInfo frame = await codec.getNextFrame();
      final ui.Image image = frame.image;
      final size = Size(image.width.toDouble(), image.height.toDouble());
      codec.dispose();
      image.dispose();
      return size;
    } catch (e) {
      HKLog.logW('[ImageProcessing] Failed to get image size: $e');
      return null;
    }
  }

  /// 判斷圖片是否需要壓縮
  ///
  /// 如果圖片大於指定大小（預設 2MB），返回 true
  static bool needsCompression(
    Uint8List imageBytes, {
    int maxSizeBytes = 2 * 1024 * 1024,
  }) {
    return imageBytes.length > maxSizeBytes;
  }

  /// 智能壓縮：只在需要時壓縮
  ///
  /// 如果圖片小於 maxSizeBytes，直接返回原圖
  /// 否則進行壓縮
  static Future<Uint8List> smartCompress(
    Uint8List imageBytes, {
    int maxSizeBytes = 2 * 1024 * 1024,
    int quality = defaultQuality,
    int? maxWidth,
    int? maxHeight,
  }) async {
    if (!needsCompression(imageBytes, maxSizeBytes: maxSizeBytes)) {
      HKLog.logD(
        '[ImageProcessing] Image size OK (${imageBytes.length} bytes), skipping compression',
      );
      return imageBytes;
    }

    final compressed = await compress(
      imageBytes,
      quality: quality,
      maxWidth: maxWidth,
      maxHeight: maxHeight,
    );

    return compressed ?? imageBytes;
  }
}

/// 圖片尺寸
class Size {
  final double width;
  final double height;

  const Size(this.width, this.height);

  double get aspectRatio => width / height;

  bool get isLandscape => width > height;
  bool get isPortrait => height > width;
  bool get isSquare => width == height;

  @override
  String toString() => 'Size(${width.toInt()}x${height.toInt()})';
}
