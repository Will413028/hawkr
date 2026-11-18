import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';

part 'block_repository.g.dart';

@riverpod
BlockRepository blockRepository(Ref ref) {
  return BlockRepository(ref.watch(dioProvider));
}

/// 聊天室封鎖入口的 Repository。
class BlockRepository with ApiRequestHelper {
  BlockRepository(this._dio);
  final Dio _dio;

  @override
  Dio get dio => _dio;

  static const _base = '/api/v1/app/blocks';

  /// POST /api/v1/app/blocks/{userId}
  ///
  /// A6 允許的殘留：只有聊天室的封鎖入口使用；留言與討論區的封鎖走 `StoreBlockRepository`。
  Future<void> blockUser(String userId) async {
    final result = await apiPost('$_base/$userId');
    if (result['code'] != 0) {
      throw AppException.fromApiError(
        result['code'] as int,
        (result['message'] ?? '封鎖失敗').toString(),
      );
    }
  }
}
