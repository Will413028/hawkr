import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_echo_tab.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

import '../../support/my_store_fixtures.dart';

class _Repo extends Mock implements PostRepository {}

Future<void> _pump(WidgetTester tester, MyStore? store) async {
  final repo = _Repo();
  when(() => repo.listSellerMessages('7')).thenAnswer((_) async => []);
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
          home: const Scaffold(
            body: PageEchoTab(sellerId: '7', sellerDisplayName: '測試店家'),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

MyStore _store({required bool listed}) => MyStore(
  myStoreResponse(
    id: '7',
    publicListing: publicListingMissing([
      if (!listed) SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
    ]),
  ),
);

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets(
    'a store that cannot go public explains where echoes will appear',
    (tester) async {
      await _pump(tester, _store(listed: false));

      expect(find.text('店家公開後，買家的迴響會顯示在這裡。'), findsOneWidget);
      expect(find.text('買家留言後會顯示在這裡。'), findsNothing);
    },
  );

  testWidgets('a public store keeps the ordinary empty hint', (tester) async {
    await _pump(tester, _store(listed: true));

    expect(find.text('買家留言後會顯示在這裡。'), findsOneWidget);
  });

  testWidgets('an unknown store keeps the ordinary empty hint', (tester) async {
    await _pump(tester, null);

    expect(find.text('買家留言後會顯示在這裡。'), findsOneWidget);
    expect(find.text('店家公開後，買家的迴響會顯示在這裡。'), findsNothing);
  });
}
