import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';

part 'block_repository.g.dart';

@riverpod
BlockRepository blockRepository(Ref ref) =>
    BlockRepository(ref.watch(dioProvider));

/// 聊天入口的封鎖（舊 REST，A6 搬進 buyer 模組前的允許殘留）。社群的封鎖與封鎖名單在 `BlockingRepository`。
class BlockRepository {
  final Dio _dio;
  BlockRepository(this._dio);

  /// POST /app/blocks/{userId} — 封鎖使用者（只給聊天入口）
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
