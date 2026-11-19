import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/networking/connect_provider.dart';

class PostHashtagSuggestion {
  const PostHashtagSuggestion({required this.name, required this.postCount});

  final String name;
  final int postCount;

  factory PostHashtagSuggestion.fromProto(pb.TagCount tag) {
    return PostHashtagSuggestion(
      name: tag.name,
      postCount: tag.postCount.toInt(),
    );
  }
}

class PostHashtagRepository {
  PostHashtagRepository(this._socialPublic);

  final SocialPublicServiceClient _socialPublic;

  Future<List<PostHashtagSuggestion>> search({
    required String prefix,
    int limit = 10,
  }) async {
    final trimmed = prefix.trim();
    if (trimmed.isEmpty) return const [];

    final response = await _socialPublic.suggestTags(
      pb.SuggestTagsRequest(query: trimmed, maxResults: limit),
    );
    return response.tags
        .map(PostHashtagSuggestion.fromProto)
        .where((item) => item.name.isNotEmpty)
        .toList(growable: false);
  }
}

final postHashtagRepositoryProvider =
    Provider.autoDispose<PostHashtagRepository>((ref) {
      return PostHashtagRepository(ref.watch(socialPublicClientProvider));
    });
