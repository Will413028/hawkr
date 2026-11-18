import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_home_tab.dart';
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

Future<_Repo> _openPreviewComments(
  WidgetTester tester, {
  required bool listed,
  required Post post,
  bool tapComments = true,
}) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final repo = _Repo();
  when(
    () => repo.listMyPostsPage(
      pageToken: any(named: 'pageToken'),
      pageSize: any(named: 'pageSize'),
    ),
  ).thenAnswer((_) async => PostPageResult(posts: [post], nextPageToken: ''));
  when(
    () => repo.listComments(any()),
  ).thenAnswer((_) async => [testComment(body: '買家的留言')]);
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
          home: Scaffold(
            body: PageHomeTab(sellerName: '測試店家', onReloadSeller: () async {}),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  await tester.longPress(
    find.byKey(const ValueKey('storefront-post-grid-item-42')),
  );
  await tester.pumpAndSettle();
  if (tapComments) {
    await tester.tap(find.bySemanticsLabel(RegExp('^留言，共')));
    await tester.pumpAndSettle();
  }
  return repo;
}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('the preview of a draft post still reads and shows comments', (
    tester,
  ) async {
    final repo = await _openPreviewComments(
      tester,
      listed: true,
      post: Post(
        id: 42,
        content: '貼文內容',
        status: PostStatus.POST_STATUS_DRAFT,
        viewerCanComment: true,
      ),
    );

    expect(find.text('貼文發布後，買家的留言會顯示在這裡。'), findsOneWidget);
    expect(find.text('買家的留言'), findsOneWidget);
    verify(() => repo.listComments(42)).called(greaterThanOrEqualTo(1));
    // 輸入列只看伺服器的 viewer_can_comment，不看說明。
    expect(find.byKey(const ValueKey('comments-sheet-close')), findsOneWidget);
    expect(find.byKey(const ValueKey('send-comment-action')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('comments-sheet-close')));
    await tester.pumpAndSettle();
    expect(find.text('貼文發布後，買家的留言會顯示在這裡。'), findsNothing);
  });

  testWidgets('the preview of a draft that is also platform-hidden shows the '
      'hidden copy', (tester) async {
    final repo = await _openPreviewComments(
      tester,
      listed: true,
      post: Post(
        id: 42,
        content: '貼文內容',
        status: PostStatus.POST_STATUS_DRAFT,
        takedownMeasureId: 'measure-1',
      ),
    );

    expect(find.text('這則貼文已被平台下架，買家看不到留言。'), findsOneWidget);
    expect(find.text('買家的留言'), findsOneWidget);
    verify(() => repo.listComments(42)).called(greaterThanOrEqualTo(1));
    expect(find.byKey(const ValueKey('send-comment-action')), findsNothing);
  });

  testWidgets('the preview of a platform-hidden post in a non-public store '
      'shows the hidden copy', (tester) async {
    final repo = await _openPreviewComments(
      tester,
      listed: false,
      post: Post(
        id: 42,
        content: '貼文內容',
        status: PostStatus.POST_STATUS_PUBLISHED,
        takedownMeasureId: 'measure-1',
      ),
    );

    expect(find.text('這則貼文已被平台下架，買家看不到留言。'), findsOneWidget);
    expect(find.text('買家的留言'), findsOneWidget);
    verify(() => repo.listComments(42)).called(greaterThanOrEqualTo(1));
  });

  testWidgets(
    'the preview of a non-public store shows its notice and comments',
    (tester) async {
      final repo = await _openPreviewComments(
        tester,
        listed: false,
        post: Post(
          id: 42,
          content: '貼文內容',
          status: PostStatus.POST_STATUS_PUBLISHED,
        ),
      );

      expect(find.text('店家目前不公開，買家看不到這些留言。'), findsOneWidget);
      expect(find.text('買家的留言'), findsOneWidget);
      verify(() => repo.listComments(42)).called(greaterThanOrEqualTo(1));
    },
  );

  testWidgets(
    'the preview shows the server comment_count without paging ListComments',
    (tester) async {
      final repo = await _openPreviewComments(
        tester,
        listed: true,
        post: Post(
          id: 42,
          content: '貼文內容',
          commentCount: 7,
          status: PostStatus.POST_STATUS_PUBLISHED,
        ),
        tapComments: false,
      );

      verifyNever(
        () => repo.listComments(any(), parentId: any(named: 'parentId')),
      );
      expect(find.bySemanticsLabel('留言，共 7 則'), findsOneWidget);
    },
  );
}
