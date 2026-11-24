import 'dart:typed_data';

import 'package:flutter_image_compress/flutter_image_compress.dart';
import 'package:hawkr_api/errors.dart'
    show DefaultErrorText, ErrorAction, ErrorFacts, classifyError, errorText;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/imageai/v1alpha1/imageai.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/imageai/v1alpha1/imageai.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/imageai/v1alpha1/imageai.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'ai_style_model.dart';

part 'ai_image_service.g.dart';

/// 原圖 inline 的上限（`TransformImageRequest.image_data`，2 MiB）；超過伺服器回 invalid_argument。
const int kMaxTransformImageBytes = 2 * 1024 * 1024;

/// 當週 AI 修圖額度用盡（`IMAGE_QUOTA_EXCEEDED`）。UI 另外處理：同步用量、鎖按鈕。
class AiQuotaExceededException implements Exception {
  const AiQuotaExceededException(this.message);
  final String message;
  @override
  String toString() => message;
}

/// AI 修圖失敗。[message] 是伺服器給使用者的文字（沒有時是該 code 的預設文字）。
class AiTransformException implements Exception {
  const AiTransformException(this.message);
  final String message;
  @override
  String toString() => message;
}

/// 把原圖縮到 [maxBytes] 以內（JPEG）；每次先降品質，再縮邊長。
typedef ImageDownsizer =
    Future<Uint8List> Function(Uint8List bytes, {required int maxBytes});

Future<Uint8List> downsizeToLimit(
  Uint8List bytes, {
  required int maxBytes,
}) async {
  var side = 1920;
  var quality = 85;
  var out = bytes;
  for (var i = 0; i < 6; i++) {
    out = await FlutterImageCompress.compressWithList(
      bytes,
      minWidth: side,
      minHeight: side,
      quality: quality,
      keepExif: false,
      format: CompressFormat.jpeg,
    );
    if (out.length <= maxBytes) return out;
    quality = quality > 55 ? quality - 15 : quality;
    side = (side * 0.75).round();
  }
  return out;
}

/// 賣家的 AI 修圖（`ImageAiAppService`）。
class AiImageService {
  AiImageService(
    this._client, {
    required DefaultErrorText defaults,
    ImageDownsizer? downsize,
  }) : _defaults = defaults,
       _downsize = downsize ?? downsizeToLimit;

  final ImageAiAppServiceClient _client;
  final DefaultErrorText _defaults;
  final ImageDownsizer _downsize;

  /// 某個用途的風格與本週用量；失敗回 null（畫面用內建風格）。
  Future<AiStyles?> fetchStyles({String category = 'product'}) async {
    try {
      final r = await _client.listAiStyles(
        pb.ListAiStylesRequest(
          category: category == 'post'
              ? AiStyleCategory.AI_STYLE_CATEGORY_POST
              : AiStyleCategory.AI_STYLE_CATEGORY_PRODUCT,
        ),
      );
      return AiStyles(
        styles: [for (final s in r.styles) AiStyle.fromProto(s)],
        defaultPrompt: r.defaultPrompt,
        weeklyUsage: r.weeklyUsage,
        weeklyLimit: r.weeklyLimit,
      );
    } on ConnectException catch (e, st) {
      HKLog.logE('[AiImageService] fetchStyles failed', e, st);
      return null;
    }
  }

  /// 以提示詞修一張圖，成功回修好的圖片 bytes；失敗拋 [AiQuotaExceededException] 或 [AiTransformException]。
  /// 原圖先縮到 [kMaxTransformImageBytes] 以內再 inline 送出。
  Future<Uint8List> transformWithGemini({
    required Uint8List imageBytes,
    String? prompt,
  }) async {
    try {
      final toUpload = await _downsize(
        imageBytes,
        maxBytes: kMaxTransformImageBytes,
      );
      final r = await _client.transformImage(
        pb.TransformImageRequest(
          imageData: toUpload,
          mimeType: 'image/jpeg',
          prompt: prompt ?? '',
        ),
      );
      if (r.imageData.isEmpty) {
        throw ConnectException(
          Code.unavailable,
          'transform returned no image data',
        );
      }
      return Uint8List.fromList(r.imageData);
    } on ConnectException catch (e) {
      final text = errorText(e, defaults: _defaults);
      if (_isQuotaExceeded(e)) {
        throw AiQuotaExceededException(text);
      }
      throw AiTransformException(text);
    }
  }
}

bool _isQuotaExceeded(ConnectException e) {
  final decision = classifyError(ErrorFacts.fromException(e));
  return decision.action == ErrorAction.reason &&
      decision.reason == ErrorReason.IMAGE_QUOTA_EXCEEDED;
}

@riverpod
AiImageService aiImageService(Ref ref) => AiImageService(
  ref.watch(imageAiClientProvider),
  defaults: ref.watch(defaultErrorTextProvider),
);
