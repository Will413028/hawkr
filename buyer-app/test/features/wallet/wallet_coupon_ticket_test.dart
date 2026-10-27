import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/wallet/presentation/widgets/wallet_coupon_ticket.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in <double>[320, 375, 390, 430, 768]) {
    for (final textScale in <double>[1, 1.15, 1.25, 1.4]) {
      testWidgets(
        'coupon ticket stays readable at ${width}px and ${textScale}x text',
        (tester) async {
          await _pumpTicket(
            tester,
            width: width,
            textScale: textScale,
            status: WalletCouponVisualStatus.available,
          );

          expect(tester.takeException(), isNull);
          expect(find.text('可使用'), findsOneWidget);
          expect(find.text('3,000'), findsOneWidget);
          expect(find.text(r'NT$'), findsOneWidget);
          expect(find.text('消費滿 NT\$ 12,000 可用'), findsOneWidget);
          expect(find.text('每人限用 1 次'), findsOneWidget);

          final ticketSize = tester.getSize(
            find.byKey(const ValueKey('coupon-ticket')),
          );
          expect(ticketSize.width, closeTo(width - 32, 0.01));
          expect(ticketSize.height, greaterThanOrEqualTo(160));

          final railSize = tester.getSize(
            find.byKey(const ValueKey('coupon-ticket-value-rail')),
          );
          expect(railSize.width, width == 320 ? 88 : 96);
          expect(
            find.bySemanticsLabel(RegExp('可使用.*夏日產地直送回饋券.*NT\\\$ 3,000')),
            findsOneWidget,
          );
        },
      );
    }
  }

  testWidgets('coupon ticket exposes status and opens rules as one button', (
    tester,
  ) async {
    var tapped = false;
    await _pumpTicket(
      tester,
      width: 390,
      textScale: 1,
      status: WalletCouponVisualStatus.expired,
      onPressed: () => tapped = true,
    );

    expect(find.text('已過期'), findsOneWidget);
    final semanticTicket = find.bySemanticsLabel(RegExp('已過期.*'));
    final semantics = tester.getSemantics(semanticTicket).getSemanticsData();
    expect(semantics.flagsCollection.isButton, isTrue);
    expect(semantics.hint, contains('查看使用範圍與規則'));

    await tester.tap(find.byKey(const ValueKey('coupon-ticket')));
    await tester.pump();

    expect(tapped, isTrue);
  });

  testWidgets('inactive coupon statuses use muted surfaces without opacity', (
    tester,
  ) async {
    for (final entry in <(WalletCouponVisualStatus, String, Color, Color)>[
      (
        WalletCouponVisualStatus.used,
        '已使用',
        AppColors.surfaceMuted,
        AppColors.textPrimary,
      ),
      (
        WalletCouponVisualStatus.expired,
        '已過期',
        AppColors.neutralWhisper,
        AppColors.textPrimary,
      ),
    ]) {
      await _pumpTicket(tester, width: 390, textScale: 1, status: entry.$1);

      expect(find.text(entry.$2), findsOneWidget);
      final surface = tester.widget<AnimatedContainer>(
        find.byKey(const ValueKey('coupon-ticket-surface')),
      );
      final rail = tester.widget<AnimatedContainer>(
        find.byKey(const ValueKey('coupon-ticket-value-rail')),
      );
      expect((surface.decoration as BoxDecoration).color, entry.$3);
      expect((rail.decoration as BoxDecoration).color, entry.$4);
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('coupon-ticket')),
          matching: find.byType(Opacity),
        ),
        findsNothing,
      );
    }
  });
}

Future<void> _pumpTicket(
  WidgetTester tester, {
  required double width,
  required double textScale,
  required WalletCouponVisualStatus status,
  VoidCallback? onPressed,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 700);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData(
        useMaterial3: true,
        textTheme: AppTypography.textTheme,
        splashFactory: NoSplash.splashFactory,
      ),
      home: MediaQuery(
        data: MediaQueryData(
          size: Size(width, 700),
          textScaler: TextScaler.linear(textScale),
          disableAnimations: true,
        ),
        child: Scaffold(
          backgroundColor: AppColors.bgPrimary,
          body: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: WalletCouponTicket(
              key: const ValueKey('coupon-ticket'),
              title: '夏日產地直送回饋券，限定會員專屬優惠',
              content: '消費滿 NT\$ 12,000 可折抵 NT\$ 3,000',
              rule: '消費滿 NT\$ 12,000 可用  每人限用 1 次',
              expiry: '有效期至 2026/08/01',
              discountLabel: 'NT\$ 3,000',
              issuerLabel: '由 呼你爽新鮮農產直送商店 提供',
              issuerAvatar: const ColoredBox(color: AppColors.primarySoft),
              status: status,
              onPressed: onPressed ?? () {},
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}
