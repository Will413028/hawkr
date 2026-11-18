import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/page_home_feed_screen.dart';
import 'package:hawkr_seller/shared/widgets/share_bottom_sheet.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

import '../../support/my_store_fixtures.dart';

class _Repo extends Mock implements PostRepository {}

const _disabledMessage = '店家公開後才能分享店面連結';

Future<void> _pump(
  WidgetTester tester,
  MyStore? store, {
  String url = '',
}) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final repo = _Repo();
  when(
    () => repo.listMyPosts(),
  ).thenAnswer((_) async => [Post(id: 42, content: '貼文內容', url: url)]);
  when(() => repo.listComments(42)).thenAnswer((_) async => []);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        postRepositoryProvider.overrideWithValue(repo),
        myStoreSnapshotProvider.overrideWithValue(store),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: const PageHomeFeedScreen(initialIndex: 0, sellerName: '測試店家'),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

MyStore _store({required bool listed}) => MyStore(
  myStoreResponse(
    id: '7',
    storeCode: 'FRESH88',
    publicListing: publicListingMissing([
      if (!listed) SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
    ]),
  ),
);

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets(
    'share is disabled with the hint while the store cannot go public',
    (tester) async {
      await _pump(tester, _store(listed: false));

      await tester.tap(find.bySemanticsLabel('分享貼文'));
      await tester.pumpAndSettle();

      expect(find.text(_disabledMessage), findsOneWidget);
      expect(find.text('分享'), findsNothing);
    },
  );

  testWidgets(
    'share is disabled without the not-public hint while the store is unknown',
    (tester) async {
      await _pump(tester, null);

      await tester.tap(find.bySemanticsLabel('分享貼文'));
      await tester.pumpAndSettle();

      expect(find.text(_disabledMessage), findsNothing);
      expect(find.text('分享'), findsNothing);
    },
  );

  testWidgets('a post without a URL shows the failure toast and no sheet', (
    tester,
  ) async {
    await _pump(tester, _store(listed: true));

    await tester.tap(find.bySemanticsLabel('分享貼文'));
    await tester.pumpAndSettle();

    expect(find.text(kShareFailedMessage), findsOneWidget);
    expect(find.text('分享'), findsNothing);
    // 讓 toast 的自動關閉計時器跑完再卸載。
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('the share sheet shows the URL carried by the post', (
    tester,
  ) async {
    await _pump(
      tester,
      _store(listed: true),
      url: 'https://share.example.test/p/42',
    );

    await tester.tap(find.bySemanticsLabel('分享貼文'));
    await tester.pumpAndSettle();

    expect(find.text('https://share.example.test/p/42'), findsOneWidget);
  });
}
