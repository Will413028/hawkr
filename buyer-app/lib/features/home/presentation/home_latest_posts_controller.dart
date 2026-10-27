import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../social/data/social_model.dart';
import '../data/home_post_preview_repository.dart';

Duration? _disableAutomaticRetry(int _, Object __) => null;

final homeLatestPostsControllerProvider =
    AsyncNotifierProvider<HomeLatestPostsController, List<Post>>(
      HomeLatestPostsController.new,
      retry: _disableAutomaticRetry,
    );

/// Session-scoped latest-post preview for Home.
///
/// Keeping this independent from the product controller means a social API
/// failure can only replace this section, never the rest of Home.
class HomeLatestPostsController extends AsyncNotifier<List<Post>> {
  static const int previewLimit = 2;

  HomePostPreviewRepository get _repository =>
      ref.read(homePostPreviewRepositoryProvider);

  @override
  Future<List<Post>> build() => _load();

  Future<List<Post>> _load() async {
    final posts = await ref
        .watch(homePostPreviewRepositoryProvider)
        .fetchLatest(limit: previewLimit);
    return List<Post>.unmodifiable(posts.take(previewLimit));
  }

  Future<void> reload() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => _repository
          .fetchLatest(limit: previewLimit)
          .then((posts) => List<Post>.unmodifiable(posts.take(previewLimit))),
    );
  }
}
