import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../media/data/media_repository.dart';
import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';

part 'seller_repository.g.dart';

class SellerRepository with ApiRequestHelper {
  SellerRepository(this._dio, this._ref);
  final Dio _dio;
  final Ref _ref;

  @override
  Dio get dio => _dio;

  // ── 媒體 ──────────────────────────────────────────────

  Future<Map<String, dynamic>> uploadMedia({
    required List<int> bytes,
    required String filename,
    String? mimeType,
    String? sellerId,
    String? bucket,
  }) async {
    try {
      HKLog.logD('[SellerRepo] UPLOAD filename=$filename size=${bytes.length}');
      final result = await _ref
          .read(mediaRepositoryProvider)
          .uploadWithSession(
            bytes: Uint8List.fromList(bytes),
            filename: filename,
          );
      final url = result.previewUrl?.isNotEmpty == true
          ? result.previewUrl!
          : result.url;
      return {
        'code': 0,
        'data': {'url': url, 'stagedToken': result.stagedToken},
      };
    } catch (e) {
      HKLog.logD('[SellerRepo] UPLOAD error: $e');
      return {'code': -1, 'message': e.toString()};
    }
  }
}

@riverpod
SellerRepository sellerRepository(Ref ref) {
  return SellerRepository(ref.watch(dioProvider), ref);
}
