import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../social/data/social_model.dart';
import '../../social/data/social_repository.dart';

final homePostPreviewRepositoryProvider = Provider<HomePostPreviewRepository>(
  (ref) => SocialHomePostPreviewRepository(ref.watch(socialRepositoryProvider)),
);

/// 首頁最新貼文的資料邊界。
///
/// 首頁只取得少量預覽；完整分頁、按讚與留言仍由 social feature 擁有。
abstract interface class HomePostPreviewRepository {
  Future<List<Post>> fetchLatest({required int limit});
}

class SocialHomePostPreviewRepository implements HomePostPreviewRepository {
  const SocialHomePostPreviewRepository(this._socialRepository);

  final SocialRepository _socialRepository;

  @override
  Future<List<Post>> fetchLatest({required int limit}) async {
    final posts = await _socialRepository.getPosts(page: 1, pageSize: limit);
    return List<Post>.unmodifiable(posts.take(limit));
  }
}
