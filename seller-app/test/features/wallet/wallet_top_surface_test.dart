import 'dart:async';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/coupon/data/coupon_model.dart';
import 'package:hawkr_seller/features/coupon/presentation/coupon_controller.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/wallet/presentation/wallet_screen.dart';
import 'package:hawkr_seller/shared/widgets/green_header_bar.dart';
import 'package:hawkr_seller/shared/widgets/truck_loading_overlay.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart' show SystemChrome;
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/kyc_fixtures.dart';
import '../../support/my_store_fixtures.dart';

typedef _Scenario = ({
  String name,
  KYCFlowStatus status,
  SellerPayUni? payuni,
  bool certified,
});

final _scenarios = <_Scenario>[
  (
    name: 'approved',
    status: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
    payuni: SellerPayUni(state: PayUniState.PAY_UNI_STATE_APPROVED),
    certified: true,
  ),
  (
    name: 'not started',
    status: KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
    payuni: null,
    certified: false,
  ),
  (
    name: 'submitted',
    status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
    payuni: null,
    certified: false,
  ),
  (
    name: 'rejected',
    status: KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
    payuni: null,
    certified: false,
  ),
  (
    name: 'payuni pending info',
    status: KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO,
    payuni: SellerPayUni(
      state: PayUniState.PAY_UNI_STATE_PENDING_INFO,
      explanation: LocalizedMessage(locale: 'zh-TW', message: '請至信箱查看補件通知。'),
    ),
    certified: false,
  ),
];

void main() {
  for (final scenario in _scenarios) {
    for (final device in const [
      (width: 320.0, top: 20.0),
      (width: 393.0, top: 59.0),
      (width: 402.0, top: 62.0),
    ]) {
      testWidgets(
        '${scenario.name} wallet paints one header through the top inset at ${device.width}pt',
        (tester) async {
          await _pumpWallet(
            tester,
            scenario,
            width: device.width,
            top: device.top,
            textScale: 1.4,
          );

          final header = find.byType(GreenHeaderBar);
          final surface = find.byKey(
            const ValueKey('seller-glass-header-surface'),
          );
          expect(find.text('營運錢包'), findsOneWidget);
          expect(
            tester.widget<GreenHeaderBar>(header).includeTopSafeArea,
            isTrue,
          );
          expect(tester.getRect(surface).top, 0);
          expect(tester.getRect(surface).bottom, device.top + 56);
          expect(tester.getRect(surface).width, device.width);
          expect(
            tester.getRect(find.text('營運錢包')).top,
            greaterThanOrEqualTo(device.top),
          );
          expect(
            SystemChrome.latestStyle?.statusBarBrightness,
            Brightness.light,
            reason: 'The cream wallet chrome needs dark iOS status icons.',
          );
          final boundary = tester.renderObject<RenderRepaintBoundary>(
            find.byKey(const ValueKey('wallet-capture')),
          );
          final pixels = await tester.runAsync(() async {
            final image = await boundary.toImage(pixelRatio: 1);
            final data = await image.toByteData(
              format: ui.ImageByteFormat.rawRgba,
            );
            final bytes = Uint8List.fromList(data!.buffer.asUint8List());
            image.dispose();
            return bytes;
          });
          final x = (device.width / 2).floor();
          final above =
              ((device.top.toInt() - 1) * device.width.toInt() + x) * 4;
          final below = (device.top.toInt() * device.width.toInt() + x) * 4;
          for (var channel = 0; channel < 3; channel++) {
            expect(
              (pixels![above + channel] - pixels[below + channel]).abs(),
              lessThanOrEqualTo(1),
              reason:
                  'One continuous painted surface across the former inset seam',
            );
          }
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox.shrink());
        },
        variant: const TargetPlatformVariant({TargetPlatform.iOS}),
      );
    }
  }

  testWidgets(
    'wallet loading keeps a full cream background and dark status icons',
    (tester) async {
      await _pumpWallet(tester, null);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);
      final scaffold = tester.widget<Scaffold>(find.byType(Scaffold));
      expect(scaffold.backgroundColor, AppColors.bgPrimary);
      expect(
        tester.getRect(find.byType(Scaffold)),
        const Rect.fromLTWH(0, 0, 402, 874),
      );
      expect(SystemChrome.latestStyle?.statusBarBrightness, Brightness.light);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: const TargetPlatformVariant({TargetPlatform.iOS}),
  );
}

Future<void> _pumpWallet(
  WidgetTester tester,
  _Scenario? scenario, {
  double width = 402,
  double top = 62,
  double textScale = 1,
}) async {
  addTearDown(tester.view.reset);
  tester.view
    ..devicePixelRatio = 3
    ..physicalSize = Size(width * 3, 874 * 3)
    ..padding = FakeViewPadding(top: top * 3, bottom: 34 * 3)
    ..viewPadding = FakeViewPadding(top: top * 3, bottom: 34 * 3);

  final kyc = scenario == null
      ? const AsyncLoading<MyKyc>()
      : AsyncData(
          MyKyc.fromApplication(
            individualApplication(
              status: scenario.status,
              payuni: scenario.payuni,
            ),
          ),
        );
  final store = scenario == null
      ? null
      : myStore(
          publicListing: publicListingMissing([
            if (!scenario.certified)
              SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
          ]),
        );

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        currentKycProvider.overrideWithValue(kyc),
        myStoreSnapshotProvider.overrideWithValue(store),
        couponListControllerProvider.overrideWith(_CouponFixture.new),
      ],
      child: ScreenUtilInit(
        designSize: const Size(414, 896),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(context).copyWith(
              disableAnimations: true,
              textScaler: TextScaler.linear(textScale),
            ),
            child: child!,
          ),
          home: const RepaintBoundary(
            key: ValueKey('wallet-capture'),
            child: WalletScreen(),
          ),
        ),
      ),
    ),
  );
  for (var frame = 0; frame < 5; frame++) {
    await tester.pump(const Duration(milliseconds: 16));
  }
}

class _CouponFixture extends CouponListController {
  @override
  FutureOr<CouponListState> build() => const CouponListState();
}
