import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../shared/utils/hk_log.dart';
import 'receipt_photo_local_service.dart';
import 'receipt_photo_model.dart';

part 'receipt_photo_repository.g.dart';

@riverpod
ReceiptPhotoRepository receiptPhotoRepository(Ref ref) {
  return ReceiptPhotoRepository(ref.watch(receiptPhotoLocalServiceProvider));
}

final class ReceiptPhotoRepository {
  const ReceiptPhotoRepository(this._localService);

  final ReceiptPhotoLocalService _localService;

  Future<ReceiptPhotoState> load({
    required String orderSn,
    required List<String> backendUrls,
  }) async {
    final authoritative = backendUrls
        .map((url) => url.trim())
        .where((url) => url.isNotEmpty)
        .toSet()
        .toList(growable: false);
    if (authoritative.isNotEmpty) {
      try {
        await _localService.remove(orderSn);
      } on Object catch (error, stackTrace) {
        HKLog.logW(
          '[ReceiptPhotoRepository] stale cache cleanup failed: $error',
          error,
          stackTrace,
        );
      }
      return ReceiptPhotoState(
        photos: authoritative,
        source: ReceiptPhotoSource.backend,
      );
    }

    try {
      final cached = (await _localService.read(orderSn))?.trim() ?? '';
      if (cached.isEmpty) return ReceiptPhotoState.empty;
      return ReceiptPhotoState(
        photos: [cached],
        source: ReceiptPhotoSource.local,
      );
    } on Object catch (error, stackTrace) {
      HKLog.logW(
        '[ReceiptPhotoRepository] local cache read failed: $error',
        error,
        stackTrace,
      );
      return ReceiptPhotoState(
        photos: const [],
        source: ReceiptPhotoSource.none,
        problem: error,
      );
    }
  }

  Future<ReceiptPhotoWriteResult> cachePreview({
    required String orderSn,
    required String url,
  }) async {
    final normalized = url.trim();
    if (normalized.isEmpty) {
      return const ReceiptPhotoWriteResult.success();
    }
    try {
      await _localService.write(orderSn, normalized);
      return const ReceiptPhotoWriteResult.success();
    } on Object catch (error, stackTrace) {
      HKLog.logW(
        '[ReceiptPhotoRepository] local cache write failed: $error',
        error,
        stackTrace,
      );
      return ReceiptPhotoWriteResult.failure(error);
    }
  }

  /// 帳號切換/登出時呼叫，清除本裝置上所有帳號的收據照片快取。
  Future<void> clearAll() async {
    try {
      await _localService.clearAll();
    } on Object catch (error, stackTrace) {
      HKLog.logW(
        '[ReceiptPhotoRepository] clearAll failed: $error',
        error,
        stackTrace,
      );
    }
  }
}
