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

class _Repo extends Mock implements PostRepository {}

MyStore _store({required bool listed}) => MyStore(
  myStoreResponse(
    id: '7',
    publicListing: publicListingMissing([
      if (!listed) SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
    ]),
  ),
);

Future<_Repo> _openPreviewComments(
  WidgetTester tester, {
  required bool listed,
  required Map<String, dynamic> post,
  bool tapComments = true,
}) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final repo = _Repo();
  when(
    () => repo.listMyPostsRaw(
      page: any(named: 'page'),
      pageSize: any(named: 'pageSize'),
    ),
  ).thenAnswer((_) async => PostPageResult(posts: [post], hasMore: false));
  when(() => repo.listComments(any())).thenAnswer((_) async => []);
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
    await tester.tap(find.bySemanticsLabel('留言，共 0 則'));
    await tester.pumpAndSettle();
  }
  return repo;
}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('the preview of a draft post does not read public comments', (
    tester,
  ) async {
    final repo = await _openPreviewComments(
      tester,
      listed: true,
      post: {
        'id': 42,
        'content': '貼文內容',
        'contentType': 'text',
        'publishStatus': '4401',
      },
    );

    expect(find.text('貼文發布後，買家的留言會顯示在這裡。'), findsOneWidget);
    verifyNever(() => repo.listComments(any()));
    // 外框不變：關閉鍵在，輸入列沒有。
    expect(find.byKey(const ValueKey('comments-sheet-close')), findsOneWidget);
    expect(find.byKey(const ValueKey('send-comment-action')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('comments-sheet-close')));
    await tester.pumpAndSettle();
    expect(find.text('貼文發布後，買家的留言會顯示在這裡。'), findsNothing);
  });

  testWidgets('the preview of a draft that is also platform-hidden shows the '
      'hidden copy', (tester) async {
    final repo = await _openPreviewComments(
      tester,
      listed: true,
      post: {
        'id': 42,
        'content': '貼文內容',
        'contentType': 'text',
        'publishStatus': '4401',
        'hiddenAt': 1760000000,
      },
    );

    expect(find.text('這則貼文已被平台下架，買家看不到留言。'), findsOneWidget);
    verifyNever(() => repo.listComments(any()));
  });

  testWidgets('the preview of a platform-hidden post in a non-public store '
      'shows the hidden copy', (tester) async {
    final repo = await _openPreviewComments(
      tester,
      listed: false,
      post: {
        'id': 42,
        'content': '貼文內容',
        'contentType': 'text',
        'publishStatus': '4402',
        'hiddenAt': 1760000000,
      },
    );

    expect(find.text('這則貼文已被平台下架，買家看不到留言。'), findsOneWidget);
    verifyNever(() => repo.listComments(any()));
  });

  testWidgets(
    'the preview of a post in a public store reads its comment count',
    (tester) async {
      final repo = await _openPreviewComments(
        tester,
        listed: true,
        post: {
          'id': 42,
          'content': '貼文內容',
          'contentType': 'text',
          'publishStatus': '4402',
        },
        tapComments: false,
      );

      verify(() => repo.listComments(42)).called(greaterThanOrEqualTo(1));
    },
  );
}
