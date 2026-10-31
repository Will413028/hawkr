import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/features/auth/presentation/widgets/seller_auth_surface.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Future<void> pumpSurface(
    WidgetTester tester, {
    required Size size,
    required double textScale,
    required TargetPlatform platform,
    EdgeInsets safeArea = EdgeInsets.zero,
    EdgeInsets viewInsets = EdgeInsets.zero,
  }) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = size;
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MaterialApp(
        key: ValueKey(
          '${size.width}x${size.height}-$textScale-$platform-'
          '${safeArea.top}-${safeArea.bottom}-${viewInsets.bottom}',
        ),
        theme: ThemeData(platform: platform, useMaterial3: true),
        builder: (context, child) {
          return MediaQuery(
            data: MediaQuery.of(context).copyWith(
              size: size,
              padding: safeArea,
              viewPadding: safeArea,
              viewInsets: viewInsets,
              textScaler: TextScaler.linear(textScale),
              disableAnimations: true,
            ),
            child: child!,
          );
        },
        home: SellerAuthSurface(
          loadingProvider: null,
          onSignIn: (_) {},
          appVersionFuture: Future.value('1.0.5(42)'),
          wordmarkBuilder: (width) => SizedBox(
            key: const ValueKey('test-seller-wordmark'),
            width: width,
            height: width * 66 / 255,
          ),
        ),
      ),
    );
    await tester.pump();
  }

  testWidgets('seller auth matches buyer surface across the device matrix', (
    tester,
  ) async {
    final matrix = [
      for (final size in const [
        Size(320, 568),
        Size(390, 844),
        Size(768, 1024),
      ])
        for (final scale in const [1.0, 1.15, 1.4])
          (
            size: size,
            scale: scale,
            platform: TargetPlatform.iOS,
            providers: const ['google', 'line'],
            safeArea: size.width == 390 && scale == 1.4
                ? const EdgeInsets.only(top: 44, bottom: 34)
                : EdgeInsets.zero,
          ),
    ];

    for (final caseData in matrix) {
      await pumpSurface(
        tester,
        size: caseData.size,
        textScale: caseData.scale,
        platform: caseData.platform,
        safeArea: caseData.safeArea,
      );

      expect(
        find.byKey(const ValueKey('auth-action-glass-panel')),
        findsOneWidget,
      );
      expect(find.byType(BackdropFilter), findsOneWidget);
      expect(find.byType(SingleChildScrollView), findsOneWidget);
      expect(
        find.byKey(const ValueKey('seller-auth-wordmark')),
        findsOneWidget,
      );

      for (final provider in caseData.providers) {
        final button = find.byKey(ValueKey('auth-oauth-$provider-button'));
        final icon = find.byKey(ValueKey('auth-oauth-$provider-icon'));
        final label = find.byKey(ValueKey('auth-oauth-$provider-label'));
        await tester.ensureVisible(button);
        await tester.pump();

        final buttonRect = tester.getRect(button);
        final iconRect = tester.getRect(icon);
        final labelText = find.descendant(
          of: label,
          matching: find.byType(RichText),
        );
        expect(buttonRect.left, greaterThanOrEqualTo(0));
        expect(buttonRect.right, lessThanOrEqualTo(caseData.size.width));
        expect(buttonRect.top, greaterThanOrEqualTo(caseData.safeArea.top));
        expect(
          buttonRect.bottom,
          lessThanOrEqualTo(caseData.size.height - caseData.safeArea.bottom),
        );
        expect(buttonRect.height, greaterThanOrEqualTo(HkControl.minTap));
        expect(
          iconRect.left - buttonRect.left,
          greaterThanOrEqualTo(HkSpace.md),
          reason:
              '$provider at ${caseData.size.width}dp/${caseData.scale}x '
              'must keep the buyer 12dp leading inset',
        );
        expect(
          tester.renderObject<RenderParagraph>(labelText).didExceedMaxLines,
          isFalse,
        );
      }

      expect(
        find.byKey(const ValueKey('auth-oauth-line-button')),
        caseData.platform == TargetPlatform.iOS ? findsOneWidget : findsNothing,
      );

      final footer = find.byKey(const ValueKey('auth-footer-legal'));
      await tester.ensureVisible(footer);
      await tester.pump();
      final footerRect = tester.getRect(footer);
      expect(footerRect.left, greaterThanOrEqualTo(0));
      expect(footerRect.right, lessThanOrEqualTo(caseData.size.width));
      expect(
        footerRect.bottom,
        lessThanOrEqualTo(caseData.size.height - caseData.safeArea.bottom),
      );
      expect(find.byKey(const ValueKey('auth-footer-version')), findsOneWidget);
      expect(tester.takeException(), isNull);
    }
  });

  testWidgets('keyboard inset keeps login actions and footer reachable', (
    tester,
  ) async {
    await pumpSurface(
      tester,
      size: const Size(320, 568),
      textScale: 1.4,
      platform: TargetPlatform.iOS,
      safeArea: const EdgeInsets.only(top: 24, bottom: 20),
      viewInsets: const EdgeInsets.only(bottom: 260),
    );

    for (final key in const [
      ValueKey('auth-oauth-google-button'),
      ValueKey('auth-oauth-line-button'),
      ValueKey('auth-footer-legal'),
    ]) {
      final target = find.byKey(key);
      await tester.ensureVisible(target);
      await tester.pump();
      final rect = tester.getRect(target);
      expect(rect.left, greaterThanOrEqualTo(0));
      expect(rect.right, lessThanOrEqualTo(320));
      expect(rect.top, greaterThanOrEqualTo(24));
      expect(rect.bottom, lessThanOrEqualTo(548));
    }
    expect(tester.takeException(), isNull);
  });

  test('formats seller auth version exactly like buyer auth', () {
    expect(formatSellerAuthVersion('1.0.5(1.0.5)'), 'v1.0.5');
    expect(formatSellerAuthVersion('1.0.5(7)'), 'v1.0.5(7)');
    expect(formatSellerAuthVersion(''), 'v—');
    expect(formatSellerAuthVersion(null), 'v—');
  });
}
