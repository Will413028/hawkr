import 'dart:convert';
import 'dart:io';

import '../../../shared/utils/image_processing_service.dart';

/// 本機檔案的預覽用 `data:` URI（縮到小圖，可存進本機快取）；讀不到檔案回空字串，呼叫端略過預覽。
Future<String> localImagePreviewDataUri(File file) async {
  try {
    final compression = await ImageProcessingService.compressIfNeeded(
      await file.readAsBytes(),
      maxSizeBytes: 150 * 1024,
      maxWidth: 480,
      maxHeight: 480,
    );
    return 'data:image/jpeg;base64,${base64Encode(compression.bytes)}';
  } catch (_) {
    return '';
  }
}
