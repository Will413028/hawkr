import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';
import 'block_model.dart';

part 'block_repository.g.dart';

@riverpod
BlockRepository blockRepository(Ref ref) {
  return BlockRepository(ref.watch(dioProvider));
}

/// 封鎖名單 Repository（backend-payuni#645）。
class BlockRepository with ApiRequestHelper {
  BlockRepository(this._dio);
  final Dio _dio;

  @override
  Dio get dio => _dio;

  static const _base = '/api/v1/app/blocks';

  /// POST /api/v1/app/blocks/{userId}
  Future<void> blockUser(String userId) async {
    final result = await apiPost('$_base/$userId');
    if (result['code'] != 0) {
      throw AppException.fromApiError(
        result['code'] as int,
        (result['message'] ?? '封鎖失敗').toString(),
      );
    }
  }

  /// DELETE /api/v1/app/blocks/{userId}
  Future<void> unblockUser(String userId) async {
    final result = await apiDelete('$_base/$userId');
    if (result['code'] != 0) {
      throw AppException.fromApiError(
        result['code'] as int,
        (result['message'] ?? '取消封鎖失敗').toString(),
      );
    }
  }

  /// GET /api/v1/app/blocks?page=&page_size=
  Future<List<BlockedUser>> listBlockedUsers({
    int page = 1,
    int pageSize = 20,
  }) async {
    final result = await apiGet(
      _base,
      queryParams: {'page': '$page', 'page_size': '$pageSize'},
    );
    if (result['code'] == 0) {
      final data = result['data'];
      final records = data is Map<String, dynamic> ? data['records'] : null;
      if (records is List) {
        return records
            .whereType<Map<String, dynamic>>()
            .map(BlockedUser.fromRaw)
            .toList();
      }
      return [];
    }
    throw AppException.fromApiError(
      result['code'] as int,
      (result['message'] ?? '取得封鎖名單失敗').toString(),
    );
  }
}
