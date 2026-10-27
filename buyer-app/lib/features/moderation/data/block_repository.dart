import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import 'block_model.dart';

part 'block_repository.g.dart';

@riverpod
BlockRepository blockRepository(Ref ref) =>
    BlockRepository(ref.watch(dioProvider));

class BlockRepository {
  final Dio _dio;
  BlockRepository(this._dio);

  /// POST /app/blocks/{userId} — 封鎖使用者
  Future<void> blockUser(String userId) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>('/app/blocks/$userId');
      final body = resp.data;
      if (body == null) return;
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '封鎖失敗').toString());
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      final bizMsg = _dioBizErrorMessage(e);
      if (bizMsg != null) throw ServerException(bizMsg);
      throw const NetworkException();
    }
  }

  /// DELETE /app/blocks/{userId} — 解除封鎖
  Future<void> unblockUser(String userId) async {
    try {
      final resp = await _dio.delete<Map<String, dynamic>>(
        '/app/blocks/$userId',
      );
      final body = resp.data;
      if (body == null) return;
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '解除封鎖失敗').toString());
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      final bizMsg = _dioBizErrorMessage(e);
      if (bizMsg != null) throw ServerException(bizMsg);
      throw const NetworkException();
    }
  }

  /// GET /app/blocks — 我的封鎖名單
  Future<List<BlockedUser>> getBlockedUsers({
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/blocks',
        queryParameters: {'page': page, 'page_size': pageSize},
      );
      final body = resp.data;
      if (body == null) return [];
      final code = body['code'];
      if (code != null && code != 0) {
        throw ServerException((body['message'] ?? '查詢封鎖名單失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      final records = data['records'] as List? ?? [];
      return records
          .whereType<Map>()
          .map((e) => BlockedUser.fromJson(Map<String, dynamic>.from(e)))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      final bizMsg = _dioBizErrorMessage(e);
      if (bizMsg != null) throw ServerException(bizMsg);
      throw const NetworkException();
    }
  }
}

/// 從 Dio 錯誤回應體讀取後端業務訊息（gateway envelope 統一欄位為 `message`，
/// `msg` 僅作少數舊路徑相容 fallback）。讀不到時回傳 null，
/// 呼叫端 fallback 到通用 [NetworkException]。
String? _dioBizErrorMessage(DioException e) {
  final data = e.response?.data;
  if (data is! Map) return null;
  final msg = data['message']?.toString().trim();
  return (msg != null && msg.isNotEmpty) ? msg : null;
}
