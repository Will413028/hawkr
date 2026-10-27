import 'dart:io';
import 'package:flutter/services.dart';
import '../../../shared/utils/platform_io.dart';
import '../../../shared/utils/hk_log.dart';

/// OCR 引擎類型
enum OcrEngine {
  appleVision, // iOS Vision Framework (支援 Simulator 和真機)
  disabled, // OCR 停用
}

/// 統一 OCR 辨識區塊
class UniversalOcrBlock {
  final String text;
  final double confidence;
  final Map<String, double> boundingBox; // keys: x, y, width, height

  UniversalOcrBlock({
    required this.text,
    required this.confidence,
    required this.boundingBox,
  });
}

/// 統一 OCR 辨識結果
class UniversalOcrResult {
  final bool success;
  final String text;
  final String? errorMessage;
  final List<UniversalOcrBlock> blocks;
  final OcrEngine engine;

  UniversalOcrResult({
    required this.success,
    this.text = '',
    this.errorMessage,
    this.blocks = const [],
    required this.engine,
  });
}

/// 統一的 OCR 服務
///
/// 在 iOS 端使用原生 Vision Framework (支援 Simulator 與真機)
class UniversalOcrService {
  static const _appleChannel = MethodChannel(
    'io.github.will413028.hawkr.seller/apple_vision_ocr',
  );

  /// 檢查 OCR 是否可用
  static Future<bool> isAvailable() async {
    if (Platform.isIOS) {
      try {
        final result = await _appleChannel.invokeMethod<bool>('isAvailable');
        return result ?? false;
      } catch (e) {
        HKLog.logE('[UniversalOCR] isAvailable error (iOS): $e');
        return false;
      }
    }
    return false;
  }

  /// 取得目前可用的 OCR 引擎
  static Future<OcrEngine> getAvailableEngine() async {
    if (Platform.isIOS) {
      final visionAvailable = await isAvailable();
      if (visionAvailable) return OcrEngine.appleVision;
    }
    return OcrEngine.disabled;
  }

  /// 辨識圖片中的文字
  static Future<UniversalOcrResult> recognizeText(File imageFile) async {
    final engine = await getAvailableEngine();

    if (engine == OcrEngine.disabled) {
      return UniversalOcrResult(
        success: false,
        errorMessage: 'OCR is not available on this platform',
        engine: OcrEngine.disabled,
      );
    }

    try {
      HKLog.logD(
        '[UniversalOCR] Recognizing text using iOS Apple Vision for: ${imageFile.path}',
      );
      final result = await _appleChannel.invokeMethod<Map>('recognizeText', {
        'imagePath': imageFile.path,
        'language': 'zh-Hant',
      });
      return _parseNativeResponse(result, OcrEngine.appleVision);
    } catch (e) {
      HKLog.logE('[UniversalOCR] Recognition failed with exception: $e');
      return UniversalOcrResult(
        success: false,
        errorMessage: e.toString(),
        engine: engine,
      );
    }
  }

  static UniversalOcrResult _parseNativeResponse(
    Map? result,
    OcrEngine engine,
  ) {
    if (result == null) {
      return UniversalOcrResult(
        success: false,
        errorMessage: 'Null response from native side',
        engine: engine,
      );
    }

    final text = result['text'] as String? ?? '';
    final rawBlocks = result['blocks'] as List? ?? [];
    final blocks = <UniversalOcrBlock>[];

    for (final raw in rawBlocks) {
      if (raw is Map) {
        final blockText = raw['text'] as String? ?? '';
        final confidence = (raw['confidence'] as num?)?.toDouble() ?? 1.0;
        final rawBox = raw['boundingBox'] as Map?;
        final box = <String, double>{};
        if (rawBox != null) {
          box['x'] = (rawBox['x'] as num?)?.toDouble() ?? 0.0;
          box['y'] = (rawBox['y'] as num?)?.toDouble() ?? 0.0;
          box['width'] = (rawBox['width'] as num?)?.toDouble() ?? 0.0;
          box['height'] = (rawBox['height'] as num?)?.toDouble() ?? 0.0;
        }
        blocks.add(
          UniversalOcrBlock(
            text: blockText,
            confidence: confidence,
            boundingBox: box,
          ),
        );
      }
    }

    return UniversalOcrResult(
      success: true,
      text: text,
      blocks: blocks,
      engine: engine,
    );
  }
}
