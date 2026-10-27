import 'package:dio/dio.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';

class PostHashtagSuggestion {
  const PostHashtagSuggestion({required this.name, required this.postCount});

  final String name;
  final int postCount;

  factory PostHashtagSuggestion.fromJson(Map<String, dynamic> json) {
    return PostHashtagSuggestion(
      name: (json['name'] ?? '').toString(),
      postCount: _parseCount(json['postCount']),
    );
  }

  static int _parseCount(dynamic value) {
    if (value is int) return value;
    if (value is num) return value.toInt();
    if (value is String) return int.tryParse(value) ?? 0;
    return 0;
  }
}

class PostHashtagRepository with ApiRequestHelper {
  PostHashtagRepository(this._dio);

  final Dio _dio;

  @override
  Dio get dio => _dio;

  Future<List<PostHashtagSuggestion>> search({
    required String prefix,
    String mode = 'prefix',
    int limit = 10,
  }) async {
    final trimmed = prefix.trim();
    if (trimmed.isEmpty) return const [];

    final result = await apiGet(
      '/api/v1/public/tags/suggest',
      queryParams: {'q': trimmed, 'mode': mode, 'limit': '$limit'},
    );
    if (result['code'] != 0) return const [];

    final data = result['data'];
    final items = data is Map ? data['items'] : null;
    if (items is! List) return const [];

    return items
        .whereType<Map>()
        .map(
          (m) => PostHashtagSuggestion.fromJson(Map<String, dynamic>.from(m)),
        )
        .where((item) => item.name.isNotEmpty)
        .toList(growable: false);
  }
}

final postHashtagRepositoryProvider =
    Provider.autoDispose<PostHashtagRepository>((ref) {
      return PostHashtagRepository(ref.watch(dioProvider));
    });
