import 'package:hawkr_buyer/features/home/data/home_post_preview_repository.dart';
import 'package:hawkr_buyer/features/home/presentation/home_latest_posts_controller.dart';
import 'package:hawkr_buyer/features/social/data/social_model.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _FakeHomePostPreviewRepository implements HomePostPreviewRepository {
  _FakeHomePostPreviewRepository(this.handler);

  final Future<List<Post>> Function(int limit) handler;
  final List<int> requestedLimits = [];

  @override
  Future<List<Post>> fetchLatest({required int limit}) {
    requestedLimits.add(limit);
    return handler(limit);
  }
}

void main() {
  test(
    'latest-post controller requests and retains at most two posts',
    () async {
      final repository = _FakeHomePostPreviewRepository(
        (_) async => [_post(1), _post(2), _post(3)],
      );
      final container = ProviderContainer(
        overrides: [
          homePostPreviewRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);

      final posts = await container.read(
        homeLatestPostsControllerProvider.future,
      );

      expect(repository.requestedLimits, [
        HomeLatestPostsController.previewLimit,
      ]);
      expect(posts.map((post) => post.id), [1, 2]);
    },
  );

  test('explicit reload recovers after a section-only failure', () async {
    var attempt = 0;
    final repository = _FakeHomePostPreviewRepository((_) async {
      attempt++;
      if (attempt == 1) throw StateError('social unavailable');
      return [_post(9)];
    });
    final container = ProviderContainer(
      overrides: [
        homePostPreviewRepositoryProvider.overrideWithValue(repository),
      ],
    );
    addTearDown(container.dispose);

    await expectLater(
      container.read(homeLatestPostsControllerProvider.future),
      throwsStateError,
    );
    expect(container.read(homeLatestPostsControllerProvider).hasError, isTrue);

    await container.read(homeLatestPostsControllerProvider.notifier).reload();

    expect(container.read(homeLatestPostsControllerProvider).value, [_post(9)]);
    expect(repository.requestedLimits, [2, 2]);
  });
}

Post _post(int id) =>
    Post(id: id, content: '貼文 $id', merchantId: 7, merchantName: '測試商家');
