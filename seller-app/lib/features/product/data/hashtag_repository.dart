import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';
import 'hashtag_model.dart';

part 'hashtag_repository.g.dart';

/// Hashtag autocomplete API。對齊 OpenAPI `GET /public/hashtags`。
///
/// 後端只回 `product_count > 0` 的 hashtag，prefix 是模糊匹配。
/// 全 token / 全選用前端決定即可（OpenAPI 寫 mode 支援 prefix / contain）。
class HashtagRepository with ApiRequestHelper {
  HashtagRepository(this._dio);
  final Dio _dio;

  @override
  Dio get dio => _dio;

  /// 依 prefix 拿 autocomplete 建議。
  ///
  /// - [prefix] 空字串會直接回空 list（避免一次拉全表）
  /// - [mode] `prefix`（預設）或 `contain`（IG 風格）
  /// - [limit] 上限 1~50，預設 10
  Future<List<HashtagItem>> search({
    required String prefix,
    String mode = 'prefix',
    int limit = 10,
  }) async {
    final trimmed = prefix.trim();
    if (trimmed.isEmpty) return const [];

    final result = await apiGet(
      '/api/v1/public/hashtags',
      queryParams: {'prefix': trimmed, 'mode': mode, 'limit': '$limit'},
    );
    if (result['code'] != 0) return const [];

    final data = result['data'];
    final items = data is Map ? data['items'] : null;
    if (items is! List) return const [];

    return items
        .whereType<Map>()
        .map((m) => HashtagItem.fromJson(Map<String, dynamic>.from(m)))
        .toList(growable: false);
  }
}

@riverpod
HashtagRepository hashtagRepository(Ref ref) {
  return HashtagRepository(ref.watch(dioProvider));
}
