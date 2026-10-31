import 'dart:ui' show PointerDeviceKind, SemanticsAction, Tristate;

import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_auth/oauth.dart';
import 'package:hawkr_buyer/features/auth/presentation/auth_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/production_layout_test_harness.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

void main() {
  testWidgets(
    'OAuth pills keep complete single-line labels at 320dp and 1.4x text',
    (tester) async {
      final semantics = tester.ensureSemantics();
      var googleTaps = 0;
      var lineTaps = 0;

      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(320, 568),
        systemTextScale: 1.4,
        home: Scaffold(
          body: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                AuthOAuthPillButton(
                  provider: OAuthProvider.google,
                  text: '透過Google繼續',
                  isLoading: false,
                  isDisabled: false,
                  onTap: () => googleTaps += 1,
                  scale: 1,
                ),
                const SizedBox(height: 16),
                AuthOAuthPillButton(
                  provider: OAuthProvider.line,
                  text: '透過LINE繼續',
                  isLoading: false,
                  isDisabled: false,
                  onTap: () => lineTaps += 1,
                  scale: 1,
                ),
              ],
            ),
          ),
        ),
      );

      for (final provider in const ['google', 'line']) {
        final button = find.byKey(ValueKey('auth-oauth-$provider-button'));
        final icon = find.byKey(ValueKey('auth-oauth-$provider-icon'));
        final providerGlyph = find.byKey(
          ValueKey('auth-oauth-$provider-provider-icon'),
        );
        final label = find.byKey(ValueKey('auth-oauth-$provider-label'));
        final labelText = find.descendant(
          of: label,
          matching: find.byType(Text),
        );

        expect(button, findsOneWidget);
        expect(icon, findsOneWidget);
        expect(providerGlyph, findsOneWidget);
        expect(labelText, findsOneWidget);
        expect(
          find.byKey(ValueKey('auth-oauth-$provider-tap-hint')),
          findsNothing,
          reason: '1.4x 文字優先保留完整登入標籤，裝飾提示應讓位',
        );

        final buttonRect = tester.getRect(button);
        final iconRect = tester.getRect(icon);
        final providerGlyphRect = tester.getRect(providerGlyph);
        final labelRect = tester.getRect(label);
        expect(buttonRect.height, greaterThanOrEqualTo(48));
        expect(buttonRect.left, greaterThanOrEqualTo(0));
        expect(buttonRect.right, lessThanOrEqualTo(320));
        expect(
          iconRect.left - buttonRect.left,
          moreOrLessEquals(HkSpace.md, epsilon: 0.01),
          reason: '$provider 圖示在大字級仍須保留 12dp 左側安全內距',
        );
        expect(
          providerGlyphRect.left - buttonRect.left,
          moreOrLessEquals(HkSpace.md, epsilon: 0.01),
          reason: '$provider 可見圖示本體不可貼齊按鈕左緣',
        );
        expect(iconRect.right, lessThanOrEqualTo(labelRect.left));
        expect(iconRect.left, greaterThanOrEqualTo(buttonRect.left));
        expect(iconRect.top, greaterThanOrEqualTo(buttonRect.top));
        expect(iconRect.right, lessThanOrEqualTo(buttonRect.right));
        expect(iconRect.bottom, lessThanOrEqualTo(buttonRect.bottom));
        expect(labelRect.left, greaterThanOrEqualTo(buttonRect.left));
        expect(labelRect.top, greaterThanOrEqualTo(buttonRect.top));
        expect(labelRect.right, lessThanOrEqualTo(buttonRect.right));
        expect(labelRect.bottom, lessThanOrEqualTo(buttonRect.bottom));

        final text = tester.widget<Text>(labelText);
        final paragraph = tester.renderObject<RenderParagraph>(labelText);
        expect(text.maxLines, 1);
        expect(text.softWrap, isFalse);
        expect(text.overflow, TextOverflow.ellipsis);
        expect(
          paragraph.didExceedMaxLines,
          isFalse,
          reason:
              '$provider 的完整登入文字不可被省略 '
              '(button=${buttonRect.width}, label=${labelRect.width}, '
              'paragraph=${paragraph.size.width})',
        );

        final data = tester.getSemantics(button).getSemanticsData();
        expect(data.label, provider == 'google' ? '透過Google繼續' : '透過LINE繼續');
        expect(data.flagsCollection.isButton, isTrue);
        expect(data.flagsCollection.isEnabled, Tristate.isTrue);
        expect(data.hasAction(SemanticsAction.tap), isTrue);
      }

      await tester.tap(find.byKey(const ValueKey('auth-oauth-google-button')));
      await tester.tap(find.byKey(const ValueKey('auth-oauth-line-button')));
      expect(googleTaps, 1);
      expect(lineTaps, 1);
      expect(tester.takeException(), isNull);

      semantics.dispose();
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets('OAuth labels keep 16sp across the large-text threshold', (
    tester,
  ) async {
    for (final textScale in const [1.29, 1.30, 1.4]) {
      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(320, 568),
        systemTextScale: textScale,
        home: Scaffold(
          body: Center(
            child: SizedBox(
              width: 288,
              child: AuthOAuthPillButton(
                provider: OAuthProvider.google,
                text: '透過Google繼續',
                isLoading: false,
                isDisabled: false,
                onTap: _noop,
              ),
            ),
          ),
        ),
      );
      final button = find.byKey(const ValueKey('auth-oauth-google-button'));
      final labelText = find.descendant(
        of: find.byKey(const ValueKey('auth-oauth-google-label')),
        matching: find.byType(Text),
      );
      final icon = find.byKey(const ValueKey('auth-oauth-google-icon'));
      final label = tester.widget<Text>(labelText);
      final paragraph = tester.renderObject<RenderParagraph>(labelText);
      final buttonRect = tester.getRect(button);
      final iconRect = tester.getRect(icon);
      final labelRect = tester.getRect(labelText);

      expect(label.style?.fontSize, 16, reason: 'scale=$textScale');
      expect(label.maxLines, 1);
      expect(label.softWrap, isFalse);
      expect(paragraph.didExceedMaxLines, isFalse, reason: 'scale=$textScale');
      expect(
        iconRect.left - buttonRect.left,
        greaterThanOrEqualTo(HkSpace.md),
        reason: 'scale=$textScale 時圖示左側內距不可被大字壓縮',
      );
      expect(iconRect.right, lessThanOrEqualTo(labelRect.left));
      expect(buttonRect.left, greaterThanOrEqualTo(0));
      expect(buttonRect.right, lessThanOrEqualTo(320));
      expect(tester.takeException(), isNull);
    }

    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('production AuthScreen fits iOS labels across the text matrix', (
    tester,
  ) async {
    for (final textScale in const [1.0, 1.15, 1.4]) {
      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(320, 844),
        systemTextScale: textScale,
        home: ProviderScope(
          child: Theme(
            data: ThemeData(platform: TargetPlatform.iOS),
            child: const AuthScreen(),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 840));

      for (final provider in const ['google', 'line']) {
        final button = find.byKey(ValueKey('auth-oauth-$provider-button'));
        final icon = find.byKey(ValueKey('auth-oauth-$provider-icon'));
        final label = find.byKey(ValueKey('auth-oauth-$provider-label'));
        final labelText = find.descendant(
          of: label,
          matching: find.byType(Text),
        );

        expect(button, findsOneWidget);
        expect(icon, findsOneWidget);
        expect(labelText, findsOneWidget);
        if (textScale == 1.4) {
          expect(
            find.byKey(ValueKey('auth-oauth-$provider-tap-hint')),
            findsNothing,
          );
        }

        final buttonRect = tester.getRect(button);
        final iconRect = tester.getRect(icon);
        final labelRect = tester.getRect(label);
        expect(buttonRect.left, greaterThanOrEqualTo(0));
        expect(buttonRect.right, lessThanOrEqualTo(320));
        expect(buttonRect.height, greaterThanOrEqualTo(48));
        expect(
          iconRect.left - buttonRect.left,
          greaterThanOrEqualTo(HkSpace.md),
          reason: '$provider 在 ${textScale}x 必須維持 12dp 圖示左側安全內距',
        );
        expect(iconRect.right, lessThanOrEqualTo(labelRect.left));
        expect(labelRect.right, lessThanOrEqualTo(buttonRect.right));
        expect(
          tester.renderObject<RenderParagraph>(labelText).didExceedMaxLines,
          isFalse,
          reason:
              '$provider 在 ${textScale}x 的完整登入文字不可被省略 '
              '(button=${buttonRect.width}, label=${labelRect.width})',
        );
      }

      expect(tester.takeException(), isNull);
    }

    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('production auth surface fits representative device matrix', (
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
      await pumpProductionLayoutTestApp(
        tester,
        size: caseData.size,
        systemTextScale: caseData.scale,
        viewPadding: caseData.safeArea,
        home: ProviderScope(
          child: Theme(
            data: ThemeData(platform: caseData.platform),
            child: const AuthScreen(),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 840));

      expect(
        find.byKey(const ValueKey('auth-action-glass-panel')),
        findsOneWidget,
      );
      expect(find.byType(BackdropFilter), findsOneWidget);
      expect(find.byType(SingleChildScrollView), findsOneWidget);
      expect(find.byKey(const ValueKey('auth-footer-legal')), findsOneWidget);
      expect(find.byKey(const ValueKey('auth-footer-version')), findsOneWidget);

      for (final provider in caseData.providers) {
        final button = find.byKey(ValueKey('auth-oauth-$provider-button'));
        final icon = find.byKey(ValueKey('auth-oauth-$provider-icon'));
        await tester.ensureVisible(button);
        await tester.pump();
        final labelText = find.descendant(
          of: find.byKey(ValueKey('auth-oauth-$provider-label')),
          matching: find.byType(Text),
        );
        final rect = tester.getRect(button);
        final iconRect = tester.getRect(icon);
        expect(rect.left, greaterThanOrEqualTo(0));
        expect(rect.right, lessThanOrEqualTo(caseData.size.width));
        expect(rect.top, greaterThanOrEqualTo(caseData.safeArea.top));
        expect(
          rect.bottom,
          lessThanOrEqualTo(caseData.size.height - caseData.safeArea.bottom),
        );
        expect(rect.height, greaterThanOrEqualTo(48));
        expect(
          iconRect.left - rect.left,
          greaterThanOrEqualTo(HkSpace.md),
          reason:
              '$provider 在 ${caseData.size.width}dp／${caseData.scale}x '
              '仍須保留圖示左側安全內距',
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
      expect(
        find.byKey(const ValueKey('auth-oauth-google-tap-hint')),
        findsNothing,
      );
      expect(
        find.byKey(const ValueKey('auth-oauth-line-tap-hint')),
        findsNothing,
      );
      expect(tester.takeException(), isNull);
    }
  });

  testWidgets('OAuth supports hover press focus and keyboard activation', (
    tester,
  ) async {
    var activations = 0;
    await pumpProductionLayoutTestApp(
      tester,
      size: const Size(390, 844),
      systemTextScale: 1,
      home: Scaffold(
        body: Center(
          child: SizedBox(
            width: 320,
            child: AuthOAuthPillButton(
              provider: OAuthProvider.google,
              text: '透過Google繼續',
              isLoading: false,
              isDisabled: false,
              onTap: () => activations += 1,
            ),
          ),
        ),
      ),
    );

    final button = find.byKey(const ValueKey('auth-oauth-google-button'));
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await mouse.addPointer(location: Offset.zero);
    await mouse.moveTo(tester.getCenter(button));
    await tester.pump(const Duration(milliseconds: 160));
    expect(
      tester
          .widget<AnimatedSlide>(
            find.byKey(const ValueKey('auth-oauth-google-hover-motion')),
          )
          .offset
          .dy,
      lessThan(0),
    );

    final touch = await tester.startGesture(tester.getCenter(button));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 80));
    final pressTransform = tester.widget<Transform>(
      find.byKey(const ValueKey('auth-oauth-google-press-scale')),
    );
    expect(pressTransform.transform.storage[0], lessThan(1));
    expect(pressTransform.transform.storage[5], lessThan(1));
    await touch.up();
    await tester.pump();
    expect(activations, 1);
    expect(
      find.byKey(const ValueKey('auth-oauth-google-click-effect')),
      findsOneWidget,
    );

    await mouse.moveTo(Offset.zero);
    await tester.pump(const Duration(milliseconds: 220));
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    expect(
      find.byKey(const ValueKey('auth-oauth-google-focus-ring')),
      findsOneWidget,
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(activations, 2);

    await mouse.removePointer();
    await tester.pumpWidget(const SizedBox.shrink());
    expect(tester.takeException(), isNull);
  });

  testWidgets('reduced-motion loading keeps geometry and uses fixed progress', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final loading = ValueNotifier(false);
    addTearDown(loading.dispose);
    await pumpProductionLayoutTestApp(
      tester,
      size: const Size(320, 568),
      systemTextScale: 1.4,
      home: MediaQuery(
        data: const MediaQueryData(
          size: Size(320, 568),
          textScaler: TextScaler.linear(1.4),
          disableAnimations: true,
        ),
        child: Scaffold(
          body: Center(
            child: ValueListenableBuilder<bool>(
              valueListenable: loading,
              builder: (context, isLoading, _) => AuthOAuthPillButton(
                provider: OAuthProvider.google,
                text: '透過Google繼續',
                isLoading: isLoading,
                isDisabled: false,
                onTap: _noop,
              ),
            ),
          ),
        ),
      ),
    );

    final button = find.byKey(const ValueKey('auth-oauth-google-button'));
    final rect = tester.getRect(button);
    final iconRect = tester.getRect(
      find.byKey(const ValueKey('auth-oauth-google-icon')),
    );
    loading.value = true;
    await tester.pump();
    // 減少動態時 HkBrandLoader 自己停在完成態，不再靠呼叫端傳 value；
    // 「確實停住」由本測試最後的 hasScheduledFrame 斷言把關。
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('auth-oauth-loading-indicator')),
        matching: find.byType(HkBrandLoader),
      ),
      findsOneWidget,
    );
    expect(tester.getRect(button), rect);
    expect(
      tester.getRect(find.byKey(const ValueKey('auth-oauth-google-icon'))),
      iconRect,
    );
    final data = tester.getSemantics(button).getSemanticsData();
    expect(data.label, '透過Google繼續，處理中');
    expect(data.flagsCollection.isEnabled, Tristate.isFalse);
    expect(data.hasAction(SemanticsAction.tap), isFalse);
    await tester.pump(const Duration(seconds: 1));
    expect(tester.binding.hasScheduledFrame, isFalse);

    semantics.dispose();
    await tester.pumpWidget(const SizedBox.shrink());
  });
}

void _noop() {}
