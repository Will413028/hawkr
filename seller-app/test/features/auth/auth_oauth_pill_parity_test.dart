import 'dart:ui' show PointerDeviceKind, SemanticsAction, Tristate;

import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_auth/oauth.dart';
import 'package:hawkr_seller/features/auth/presentation/widgets/auth_oauth_pill_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Future<void> pumpPill(
    WidgetTester tester, {
    required Widget child,
    double textScale = 1,
    bool disableAnimations = false,
  }) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 568);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MaterialApp(
        home: MediaQuery(
          data: MediaQueryData(
            size: const Size(320, 568),
            textScaler: TextScaler.linear(textScale),
            disableAnimations: disableAnimations,
          ),
          child: Scaffold(body: Center(child: child)),
        ),
      ),
    );
  }

  testWidgets(
    'buyer-parity OAuth pills keep full labels and a 12dp icon inset at 1.4x',
    (tester) async {
      final semantics = tester.ensureSemantics();
      var googleTaps = 0;
      var lineTaps = 0;

      await pumpPill(
        tester,
        textScale: 1.4,
        child: SizedBox(
          width: 288,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              AuthOAuthPillButton(
                provider: OAuthProvider.google,
                text: '透過Google繼續',
                isLoading: false,
                isDisabled: false,
                onTap: () => googleTaps += 1,
              ),
              const SizedBox(height: HkSpace.lg),
              AuthOAuthPillButton(
                provider: OAuthProvider.line,
                text: '透過LINE繼續',
                isLoading: false,
                isDisabled: false,
                onTap: () => lineTaps += 1,
              ),
            ],
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

        final buttonRect = tester.getRect(button);
        final iconRect = tester.getRect(icon);
        final glyphRect = tester.getRect(providerGlyph);
        final labelRect = tester.getRect(label);
        expect(buttonRect.height, greaterThanOrEqualTo(HkControl.minTap));
        expect(
          iconRect.left - buttonRect.left,
          moreOrLessEquals(HkSpace.md, epsilon: 0.01),
        );
        expect(
          glyphRect.left - buttonRect.left,
          moreOrLessEquals(HkSpace.md, epsilon: 0.01),
        );
        expect(iconRect.right, lessThanOrEqualTo(labelRect.left));
        expect(labelRect.right, lessThanOrEqualTo(buttonRect.right));

        final text = tester.widget<Text>(labelText);
        final paragraph = tester.renderObject<RenderParagraph>(
          find.descendant(of: label, matching: find.byType(RichText)),
        );
        expect(text.maxLines, 1);
        expect(text.softWrap, isFalse);
        expect(text.overflow, TextOverflow.ellipsis);
        expect(paragraph.didExceedMaxLines, isFalse);

        final semanticsData = tester.getSemantics(button).getSemanticsData();
        expect(semanticsData.flagsCollection.isButton, isTrue);
        expect(semanticsData.flagsCollection.isEnabled, Tristate.isTrue);
        expect(semanticsData.hasAction(SemanticsAction.tap), isTrue);
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

  testWidgets('OAuth exposes buyer-parity hover press focus and keyboard', (
    tester,
  ) async {
    var activations = 0;
    await pumpPill(
      tester,
      child: SizedBox(
        width: 288,
        child: AuthOAuthPillButton(
          provider: OAuthProvider.google,
          text: '透過Google繼續',
          isLoading: false,
          isDisabled: false,
          onTap: () => activations += 1,
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
    final press = tester.widget<Transform>(
      find.byKey(const ValueKey('auth-oauth-google-press-scale')),
    );
    expect(press.transform.storage[0], lessThan(1));
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

  testWidgets('reduced-motion loading keeps geometry and stops animation', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final loading = ValueNotifier(false);
    addTearDown(loading.dispose);
    await pumpPill(
      tester,
      textScale: 1.4,
      disableAnimations: true,
      child: SizedBox(
        width: 288,
        child: ValueListenableBuilder<bool>(
          valueListenable: loading,
          builder: (context, isLoading, _) => AuthOAuthPillButton(
            provider: OAuthProvider.google,
            text: '透過Google繼續',
            isLoading: isLoading,
            isDisabled: false,
            onTap: () {},
          ),
        ),
      ),
    );

    final button = find.byKey(const ValueKey('auth-oauth-google-button'));
    final before = tester.getRect(button);
    loading.value = true;
    await tester.pump();
    final progress = tester.widget<CircularProgressIndicator>(
      find.descendant(
        of: find.byKey(const ValueKey('auth-oauth-loading-indicator')),
        matching: find.byType(CircularProgressIndicator),
      ),
    );
    expect(progress.value, 0.72);
    expect(tester.getRect(button), before);

    final data = tester.getSemantics(button).getSemanticsData();
    expect(data.label, '透過Google繼續，處理中');
    expect(data.flagsCollection.isEnabled, Tristate.isFalse);
    expect(data.hasAction(SemanticsAction.tap), isFalse);
    await tester.pump(const Duration(seconds: 1));
    expect(tester.binding.hasScheduledFrame, isFalse);
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });
}
