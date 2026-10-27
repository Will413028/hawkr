import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_shop_tab.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/shared/widgets/share_bottom_sheet.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

import '../../support/my_store_fixtures.dart';

class _Repo extends Mock implements PostRepository {}

const _disabledMessage = '店家公開後才能分享商品連結';
const _share = ValueKey('seller-product-share');

Future<_Repo> _pump(WidgetTester tester, MyStore? store) async {
  tester.view.physicalSize = const Size(414, 956);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final repo = _Repo();
  when(
    () => repo.getShareUrl(type: 'product', id: 'p-1'),
  ).thenAnswer((_) async => null);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(
          Dio(BaseOptions(baseUrl: 'https://example.test'))
            ..httpClientAdapter = _EmptyAdapter(),
        ),
        postRepositoryProvider.overrideWithValue(repo),
        myStoreSnapshotProvider.overrideWithValue(store),
      ],
      child: ScreenUtilInit(
        designSize: const Size(414, 896),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: Scaffold(
            body: buildSellerProductManagementCardForTest(
              product: const <String, dynamic>{
                'id': 'p-1',
                'productName': '青江菜',
                'priceMin': 88,
                'status': '4004',
              },
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return repo;
}

MyStore _store({required bool listed}) => MyStore(
  myStoreResponse(
    storeCode: 'FRESH88',
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
    'product share is disabled with a hint while the store cannot go public',
    (tester) async {
      final semantics = tester.ensureSemantics();
      final repo = await _pump(tester, _store(listed: false));

      expect(
        tester.getSemantics(find.byKey(_share)),
        isSemantics(
          label: '分享商品',
          hint: _disabledMessage,
          isButton: true,
          hasEnabledState: true,
          isEnabled: false,
        ),
      );
      await tester.tap(find.byKey(_share));
      await tester.pumpAndSettle();

      expect(find.text(_disabledMessage), findsOneWidget);
      verifyNever(
        () => repo.getShareUrl(
          type: any(named: 'type'),
          id: any(named: 'id'),
        ),
      );
      semantics.dispose();
    },
  );

  testWidgets(
    'product share is disabled without the hint while the store is unknown',
    (tester) async {
      final repo = await _pump(tester, null);

      await tester.tap(find.byKey(_share));
      await tester.pumpAndSettle();

      expect(find.text(_disabledMessage), findsNothing);
      verifyNever(
        () => repo.getShareUrl(
          type: any(named: 'type'),
          id: any(named: 'id'),
        ),
      );
    },
  );

  testWidgets('product share opens the share sheet with the server URL', (
    tester,
  ) async {
    final repo = await _pump(tester, _store(listed: true));
    when(() => repo.getShareUrl(type: 'product', id: 'p-1')).thenAnswer(
      (_) async => const ShareData(
        title: '青江菜',
        description: '',
        imageUrl: '',
        url: 'https://share.example.test/p/p-1',
      ),
    );

    await tester.tap(find.byKey(_share));
    await tester.pumpAndSettle();

    verify(() => repo.getShareUrl(type: 'product', id: 'p-1')).called(1);
    expect(find.text('https://share.example.test/p/p-1'), findsOneWidget);
  });

  testWidgets('a failed product share lookup shows the failure toast', (
    tester,
  ) async {
    await _pump(tester, _store(listed: true));

    await tester.tap(find.byKey(_share));
    await tester.pumpAndSettle();

    expect(find.text(kShareFailedMessage), findsOneWidget);
    expect(find.text('分享'), findsNothing);
    // 讓 toast 的自動關閉計時器跑完再卸載。
    await tester.pump(const Duration(seconds: 3));
  });
}

class _EmptyAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async => ResponseBody.fromString(
    jsonEncode({'code': 0, 'data': <String, dynamic>{}}),
    200,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );

  @override
  void close({bool force = false}) {}
}
