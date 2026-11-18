import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_comment.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/page_home_feed_screen.dart';
import 'package:hawkr_seller/features/post/presentation/post_controller.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

import '../../support/my_store_fixtures.dart';
import 'post_comment_fixtures.dart';

class _Repo extends Mock implements PostRepository {}

class _Offline implements Exception {
  const _Offline();
}

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
  String? replyFailsFor,
  Object replyFailure = const _Offline(),
  Size size = const Size(390, 844),
}) async {
  tester.view.physicalSize = size;
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
  if (replyFailsFor != null) {
    when(
      () => repo.listComments(42, parentId: replyFailsFor),
    ).thenThrow(replyFailure);
  }
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
  await tester.tap(find.bySemanticsLabel('留言，共 ${post.commentCount} 則'));
  await tester.pumpAndSettle();
  return repo;
}

Post _post({
  PostStatus publishStatus = PostStatus.POST_STATUS_UNSPECIFIED,
  int? hiddenAt,
  bool viewerCanComment = true,
  int commentCount = 0,
}) => Post(
  id: 42,
  content: '貼文內容',
  status: publishStatus,
  viewerCanComment: viewerCanComment,
  commentCount: commentCount,
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
      findsOneWidget,
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

  for (final (status, text) in [
    (PostStatus.POST_STATUS_DRAFT, '貼文發布後，買家的留言會顯示在這裡。'),
    (PostStatus.POST_STATUS_UNPUBLISHED, '這則貼文已下架，買家看不到這些留言。'),
  ]) {
    testWidgets('an unpublished post ($status) still shows comments', (
      tester,
    ) async {
      final repo = await _openComments(
        tester,
        listed: true,
        post: _post(publishStatus: status),
      );

      await expectCommentsShownWithNotice(tester, repo, text);
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

  testWidgets('the composer follows viewer_can_comment, not the notice', (
    tester,
  ) async {
    await _openComments(
      tester,
      listed: false,
      post: _post(
        publishStatus: PostStatus.POST_STATUS_PUBLISHED,
        viewerCanComment: false,
      ),
    );
    expect(find.text('店家目前不公開，買家看不到這些留言。'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('seller-post-action-composer-42')),
      findsNothing,
    );
  });

  for (final canComment in [true, false]) {
    testWidgets(
      'short layout composer follows viewer_can_comment=$canComment',
      (tester) async {
        await _openComments(
          tester,
          listed: true,
          size: const Size(390, 250),
          post: _post(
            publishStatus: PostStatus.POST_STATUS_PUBLISHED,
            viewerCanComment: canComment,
          ),
        );
        expect(
          find.byKey(const ValueKey('seller-post-comments-scroll-42')),
          findsOneWidget,
        );
        expect(
          find.byKey(const ValueKey('seller-post-action-composer-42')),
          canComment ? findsOneWidget : findsNothing,
        );
      },
    );
  }

  testWidgets('the sheet title uses the server comment_count', (tester) async {
    await _openComments(
      tester,
      listed: true,
      post: _post(
        publishStatus: PostStatus.POST_STATUS_PUBLISHED,
        commentCount: 5,
      ),
    );
    expect(find.text('留言 5'), findsOneWidget);
  });

  testWidgets('a failed reply fetch only fails its own thread', (tester) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(publishStatus: PostStatus.POST_STATUS_PUBLISHED),
      comments: [
        testComment(id: '1', body: '第一則', replyCount: 1),
        testComment(id: '2', body: '第二則'),
      ],
      replyFailsFor: '1',
    );
    expect(repo, isNotNull);
    expect(find.text('第一則'), findsOneWidget);
    expect(find.text('第二則'), findsOneWidget);
    expect(find.text('回覆載入失敗'), findsOneWidget);
    expect(find.text('留言載入失敗'), findsNothing);
  });

  testWidgets('a programming error in a reply fetch is not swallowed', (
    tester,
  ) async {
    await _openComments(
      tester,
      listed: true,
      post: _post(publishStatus: PostStatus.POST_STATUS_PUBLISHED),
      comments: [testComment(id: '1', body: '第一則', replyCount: 1)],
      replyFailsFor: '1',
      replyFailure: StateError('bug'),
    );
    expect(tester.takeException(), isA<StateError>());
    expect(find.text('回覆載入失敗'), findsNothing);
  });

  testWidgets('retrying one thread refetches only that thread', (tester) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(publishStatus: PostStatus.POST_STATUS_PUBLISHED),
      comments: [
        testComment(id: '1', body: '第一則', replyCount: 1),
        testComment(id: '2', body: '第二則', replyCount: 1),
      ],
      replyFailsFor: '1',
    );
    when(() => repo.listComments(42, parentId: '1')).thenAnswer(
      (_) async => [testComment(id: '5', parentId: '1', body: '重試後的回覆')],
    );
    await tester.tap(
      find.byKey(const ValueKey('seller-comment-replies-retry-1')),
    );
    await tester.pumpAndSettle();
    expect(find.text('重試後的回覆'), findsOneWidget);
    expect(find.text('回覆載入失敗'), findsNothing);
    verify(() => repo.listComments(42)).called(1);
    verify(() => repo.listComments(42, parentId: '2')).called(1);
    verify(() => repo.listComments(42, parentId: '1')).called(2);
  });

  testWidgets(
    'the sheet title keeps the last known count while the list reloads',
    (tester) async {
      final repo = await _openComments(
        tester,
        listed: true,
        post: _post(
          publishStatus: PostStatus.POST_STATUS_PUBLISHED,
          commentCount: 5,
        ),
      );
      verifyNever(() => repo.getMyPost(any()));
      expect(find.text('留言 5'), findsOneWidget);
      final reload = Completer<List<Post>>();
      when(() => repo.listMyPosts()).thenAnswer((_) => reload.future);
      final container = ProviderScope.containerOf(
        tester.element(find.byType(PageHomeFeedScreen)),
      );
      unawaited(container.read(postListProvider.notifier).refresh());
      await tester.pump();
      expect(container.read(postListProvider).isLoading, isTrue);
      expect(find.text('留言 5'), findsOneWidget);
      expect(find.text('留言 0'), findsNothing);
      reload.complete([_post(commentCount: 5)]);
      await tester.pumpAndSettle();
    },
  );

  testWidgets('a created or deleted comment adjusts the count locally', (
    tester,
  ) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(
        publishStatus: PostStatus.POST_STATUS_PUBLISHED,
        commentCount: 1,
      ),
      comments: [testComment(canDelete: true, canReply: true)],
    );
    when(
      () => repo.createComment(42, body: '新的一則'),
    ).thenAnswer((_) async => testComment(id: '7', body: '新的一則'));
    await tester.enterText(
      find.byKey(const ValueKey('seller-post-comment-input-42')),
      '新的一則',
    );
    await tester.pump();
    final send = find.bySemanticsLabel('送出留言');
    await tester.ensureVisible(send);
    await tester.tap(send);
    await tester.pumpAndSettle();
    expect(find.text('留言 2'), findsOneWidget);

    when(
      () => repo.deleteComment('1', etag: 'etag-1'),
    ).thenAnswer((_) async {});
    final more = find.byTooltip('留言更多操作').first;
    await tester.ensureVisible(more);
    await tester.tap(more);
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除留言'));
    await tester.pumpAndSettle();
    expect(find.text('留言 1'), findsOneWidget);
    verifyNever(() => repo.getMyPost(any()));
    await tester.pump(const Duration(seconds: 3));
  });
}
