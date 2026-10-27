import 'dart:typed_data';

import 'package:flutter_image_compress/flutter_image_compress.dart';

import 'hk_log.dart';

class ImageCompressionResult {
  const ImageCompressionResult({
    required this.bytes,
    required this.compressed,
    required this.originalSize,
  });

  final Uint8List bytes;
  final bool compressed;
  final int originalSize;
}

class ImageProcessingService {
  ImageProcessingService._();

  static const int defaultQuality = 80;
  static const int defaultMaxWidth = 1920;
  static const int defaultMaxHeight = 1920;
  static const int defaultThresholdBytes = 2 * 1024 * 1024;

  static bool needsCompression(
    Uint8List imageBytes, {
    int maxSizeBytes = defaultThresholdBytes,
  }) {
    return imageBytes.length > maxSizeBytes;
  }

  static Future<ImageCompressionResult> compressIfNeeded(
    Uint8List imageBytes, {
    int maxSizeBytes = defaultThresholdBytes,
    int quality = defaultQuality,
    int maxWidth = defaultMaxWidth,
    int maxHeight = defaultMaxHeight,
  }) async {
    if (!needsCompression(imageBytes, maxSizeBytes: maxSizeBytes)) {
      return ImageCompressionResult(
        bytes: imageBytes,
        compressed: false,
        originalSize: imageBytes.length,
      );
    }

    try {
      final compressed = await FlutterImageCompress.compressWithList(
        imageBytes,
        minWidth: maxWidth,
        minHeight: maxHeight,
        quality: quality,
        keepExif: false,
        format: CompressFormat.jpeg,
      );

      if (compressed.isEmpty || compressed.length >= imageBytes.length) {
        HKLog.logW(
          '[ImageProcessing] compression skipped: original=${imageBytes.length}, compressed=${compressed.length}',
        );
        return ImageCompressionResult(
          bytes: imageBytes,
          compressed: false,
          originalSize: imageBytes.length,
        );
      }

      HKLog.logD(
        '[ImageProcessing] compressed ${imageBytes.length} -> ${compressed.length} bytes',
      );
      return ImageCompressionResult(
        bytes: compressed,
        compressed: true,
        originalSize: imageBytes.length,
      );
    } catch (e, st) {
      HKLog.logE('[ImageProcessing] compression failed', e, st);
      return ImageCompressionResult(
        bytes: imageBytes,
        compressed: false,
        originalSize: imageBytes.length,
      );
    }
  }
}
