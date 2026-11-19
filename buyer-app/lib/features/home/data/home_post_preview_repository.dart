import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../social/data/social_content_repository.dart';
import '../../social/data/social_model.dart';

final homePostPreviewRepositoryProvider = Provider<HomePostPreviewRepository>(
  (ref) => SocialHomePostPreviewRepository(
    ref.watch(socialContentRepositoryProvider),
  ),
);

/// 首頁最新貼文的資料邊界。
///
/// 首頁只取得少量預覽；完整分頁、按讚與留言仍由 social feature 擁有。
abstract interface class HomePostPreviewRepository {
  Future<List<Post>> fetchLatest({required int limit});
}

class SocialHomePostPreviewRepository implements HomePostPreviewRepository {
  const SocialHomePostPreviewRepository(this._socialRepository);

  final SocialContentRepository _socialRepository;

  @override
  Future<List<Post>> fetchLatest({required int limit}) async {
    final page = await _socialRepository.listPosts(size: limit);
    return List<Post>.unmodifiable(page.posts.take(limit));
  }
}
