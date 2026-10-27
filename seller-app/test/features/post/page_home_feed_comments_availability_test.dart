import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
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

class _Repo extends Mock implements PostRepository {}

MyStore _store({required bool listed}) => MyStore(
  myStoreResponse(
    id: '7',
    publicListing: publicListingMissing([
      if (!listed) SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
    ]),
  ),
);

Future<_Repo> _openComments(
  WidgetTester tester, {
  required bool listed,
  required Post post,
}) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final repo = _Repo();
  when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
  when(() => repo.listCommentsRaw(42)).thenAnswer((_) async => []);
  when(() => repo.listCommentReplies(any())).thenAnswer((_) async => []);
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

Post _post({String? publishStatus, int? hiddenAt}) => Post(
  id: 42,
  sellerId: 7,
  contentType: 'IMAGE',
  content: '貼文內容',
  publishStatus: publishStatus,
  hiddenAt: hiddenAt,
);

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('a store that cannot go public explains where comments appear', (
    tester,
  ) async {
    final repo = await _openComments(
      tester,
      listed: false,
      post: _post(publishStatus: PostPublishStatus.published.value),
    );

    expect(find.text('店家公開後，買家的留言會顯示在這裡。'), findsOneWidget);
    verifyNever(() => repo.listCommentsRaw(any()));
    verifyNever(() => repo.listCommentReplies(any()));
    // 外框不變：標題與關閉鍵都在，只有列表與輸入列換成說明。
    expect(find.text('留言'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('seller-post-close-comments-42')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('seller-post-action-composer-42')),
      findsNothing,
    );
    await tester.tap(
      find.byKey(const ValueKey('seller-post-close-comments-42')),
    );
    await tester.pumpAndSettle();
    expect(find.text('店家公開後，買家的留言會顯示在這裡。'), findsNothing);
  });

  for (final status in [
    PostPublishStatus.draft.value,
    PostPublishStatus.unpublished.value,
  ]) {
    testWidgets('an unpublished post ($status) explains it needs publishing', (
      tester,
    ) async {
      final repo = await _openComments(
        tester,
        listed: true,
        post: _post(publishStatus: status),
      );

      expect(find.text('貼文發布後，買家的留言會顯示在這裡。'), findsOneWidget);
      verifyNever(() => repo.listCommentsRaw(any()));
    });
  }

  testWidgets('a platform-hidden post says buyers cannot see comments', (
    tester,
  ) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(
        publishStatus: PostPublishStatus.published.value,
        hiddenAt: 1760000000,
      ),
    );

    expect(find.text('這則貼文已被平台下架，買家看不到留言。'), findsOneWidget);
    verifyNever(() => repo.listCommentsRaw(any()));
  });

  testWidgets('a draft that is also platform-hidden shows the hidden copy', (
    tester,
  ) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(
        publishStatus: PostPublishStatus.draft.value,
        hiddenAt: 1760000000,
      ),
    );

    expect(find.text('這則貼文已被平台下架，買家看不到留言。'), findsOneWidget);
    verifyNever(() => repo.listCommentsRaw(any()));
  });

  testWidgets('a platform-hidden post of a non-public store shows the hidden '
      'copy', (tester) async {
    final repo = await _openComments(
      tester,
      listed: false,
      post: _post(
        publishStatus: PostPublishStatus.published.value,
        hiddenAt: 1760000000,
      ),
    );

    expect(find.text('這則貼文已被平台下架，買家看不到留言。'), findsOneWidget);
    verifyNever(() => repo.listCommentsRaw(any()));
  });

  testWidgets('a public post loads its comments as before', (tester) async {
    final repo = await _openComments(
      tester,
      listed: true,
      post: _post(publishStatus: PostPublishStatus.published.value),
    );

    expect(find.text('尚無留言'), findsOneWidget);
    verify(() => repo.listCommentsRaw(42)).called(1);
  });
}
