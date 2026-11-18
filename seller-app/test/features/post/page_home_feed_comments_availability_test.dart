import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_comment.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/page_home_feed_screen.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

import '../../support/my_store_fixtures.dart';
import 'post_comment_fixtures.dart';

class _Repo extends Mock implements PostRepository {}

MyStore _store({required bool listed}) => MyStore(
  myStoreResponse(
    id: '7',
    publicListing: publicListingMissing([
      if (!listed) SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
    ]),
  ),
);

Future<_Repo> _openComments(
  WidgetTester tester, {
  required bool listed,
  required Post post,
  List<PostComment>? comments,
}) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final repo = _Repo();
  when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
  when(
    () => repo.listComments(42),
  ).thenAnswer((_) async => comments ?? [testComment(body: '買家的留言')]);
  when(
    () => repo.listComments(
      42,
      parentId: any(named: 'parentId', that: isNotNull),
    ),
  ).thenAnswer((_) async => const <PostComment>[]);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        postRepositoryProvider.overrideWithValue(repo),
        sellerStoreIdProvider.overrideWithValue('7'),
        myStoreSnapshotProvider.overrideWithValue(_store(listed: listed)),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: const PageHomeFeedScreen(initialIndex: 0, sellerName: '測試商家'),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  await tester.tap(find.bySemanticsLabel('留言，共 0 則'));
  await tester.pumpAndSettle();
  return repo;
}

Post _post({
  PostStatus publishStatus = PostStatus.POST_STATUS_UNSPECIFIED,
  int? hiddenAt,
}) => Post(
  id: 42,
  content: '貼文內容',
  status: publishStatus,
  takedownMeasureId: hiddenAt == null ? null : 'measure-1',
);

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  Future<void> expectCommentsShownWithNotice(
    WidgetTester tester,
    _Repo repo,
    String notice,
  ) async {
    verify(() => repo.listComments(42)).called(1);
    expect(find.text(notice), findsOneWidget);
    expect(find.text('買家的留言'), findsOneWidget);
    expect(find.byTooltip('留言更多操作'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('seller-post-action-composer-42')),
      findsNothing,
    );
  }

  testWidgets(
    'a store that is not public still shows and can delete comments',
    (tester) async {
      final repo = await _openComments(
        tester,
        listed: false,
        post: _post(publishStatus: PostStatus.POST_STATUS_PUBLISHED),
      );

      await expectCommentsShownWithNotice(tester, repo, '店家目前不公開，買家看不到這些留言。');
      expect(
        find.byKey(const ValueKey('seller-post-close-comments-42')),
        findsOneWidget,
      );
      await tester.tap(
        find.byKey(const ValueKey('seller-post-close-comments-42')),
      );
      await tester.pumpAndSettle();
      expect(find.text('店家目前不公開，買家看不到這些留言。'), findsNothing);
    },
  );

  for (final status in [
    PostStatus.POST_STATUS_DRAFT,
    PostStatus.POST_STATUS_UNPUBLISHED,
  ]) {
    testWidgets('an unpublished post ($status) still shows comments', (
      tester,
    ) async {
      final repo = await _openComments(
        tester,
        listed: true,
        post: _post(publishStatus: status),
      );

      await expectCommentsShownWithNotice(tester, repo, '這則貼文已下架，買家看不到這些留言。');
    });
  }

  testWidgets('a platform-hidden post still shows comments with its notice', (
    tester,
  ) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(
        publishStatus: PostStatus.POST_STATUS_PUBLISHED,
        hiddenAt: 1760000000,
      ),
    );

    await expectCommentsShownWithNotice(tester, repo, '這則貼文已被平台下架，買家看不到留言。');
  });

  testWidgets('a draft that is also platform-hidden shows the hidden copy', (
    tester,
  ) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(
        publishStatus: PostStatus.POST_STATUS_DRAFT,
        hiddenAt: 1760000000,
      ),
    );

    await expectCommentsShownWithNotice(tester, repo, '這則貼文已被平台下架，買家看不到留言。');
  });

  testWidgets('a platform-hidden post of a non-public store shows the hidden '
      'copy', (tester) async {
    final repo = await _openComments(
      tester,
      listed: false,
      post: _post(
        publishStatus: PostStatus.POST_STATUS_PUBLISHED,
        hiddenAt: 1760000000,
      ),
    );

    await expectCommentsShownWithNotice(tester, repo, '這則貼文已被平台下架，買家看不到留言。');
  });

  testWidgets('a public post shows comments without a notice', (tester) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(publishStatus: PostStatus.POST_STATUS_PUBLISHED),
    );

    verify(() => repo.listComments(42)).called(1);
    expect(find.text('買家的留言'), findsOneWidget);
    expect(find.textContaining('買家看不到'), findsNothing);
    expect(
      find.byKey(const ValueKey('seller-post-action-composer-42')),
      findsOneWidget,
    );
  });

  testWidgets('deleted and hidden top-level comments render placeholders', (
    tester,
  ) async {
    await _openComments(
      tester,
      listed: true,
      post: _post(publishStatus: PostStatus.POST_STATUS_PUBLISHED),
      comments: [
        testComment(
          id: '1',
          state: CommentState.COMMENT_STATE_DELETED,
          replyCount: 1,
        ),
        testComment(
          id: '2',
          state: CommentState.COMMENT_STATE_HIDDEN,
          replyCount: 1,
        ),
      ],
    );

    expect(find.text('此留言已刪除'), findsOneWidget);
    expect(find.text('此留言已被隱藏'), findsOneWidget);
    expect(find.byTooltip('留言更多操作'), findsNothing);
  });
}
