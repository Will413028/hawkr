import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/receipt_photo_model.dart';
import '../data/receipt_photo_repository.dart';

part 'receipt_photo_controller.g.dart';

@riverpod
class ReceiptPhotoController extends _$ReceiptPhotoController {
  @override
  Future<ReceiptPhotoState> build({
    required String orderSn,
    required List<String> backendUrls,
  }) {
    return ref
        .watch(receiptPhotoRepositoryProvider)
        .load(orderSn: orderSn, backendUrls: backendUrls);
  }

  Future<bool> cachePreview(String url) async {
    final normalized = url.trim();
    final result = await ref
        .read(receiptPhotoRepositoryProvider)
        .cachePreview(orderSn: orderSn, url: normalized);
    if (result.isSuccess && normalized.isNotEmpty) {
      state = AsyncData(
        ReceiptPhotoState(
          photos: [normalized],
          source: ReceiptPhotoSource.local,
        ),
      );
    }
    return result.isSuccess;
  }
}
