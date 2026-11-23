import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/l10n/l10n.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test('已發佈的 UI 語系：en 未發佈前解析結果一律是 zh-TW', () {
    expect(publishedUiLocales, [UiLocale.zhTW]);
    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(container.read(uiLocaleProvider), UiLocale.zhTW);
  });

  test('defaultErrorTextProvider：Connect code 的預設文字取自目錄', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final defaults = container.read(defaultErrorTextProvider);
    expect(defaults(Code.invalidArgument), '參數錯誤');
    expect(defaults(Code.internal), '系統錯誤');
    expect(defaults(Code.resourceExhausted), '操作過於頻繁，請稍後再試');
  });

  test('flutterLocale：zh-TW 是 zh_Hant_TW，en 是 en（ADR D1）', () {
    expect(UiLocale.zhTW.flutterLocale.toString(), 'zh_Hant_TW');
    expect(UiLocale.en.flutterLocale.toString(), 'en');
  });

  test('ARB 的語系與 UiLocale 一一對應：多或少一邊都紅', () {
    final arb = AppLocalizations.supportedLocales
        .map((l) => l.languageCode)
        .toSet();
    final ui = UiLocale.values.map((l) => l.flutterLocale.languageCode).toSet();
    expect(arb, ui);
  });

  testWidgets('context.l10n 依 MaterialApp 的 locale 取文字', (tester) async {
    for (final (locale, expected) in [
      (UiLocale.zhTW.flutterLocale, '繁體中文'),
      (UiLocale.en.flutterLocale, 'English'),
    ]) {
      late String name;
      await tester.pumpWidget(
        MaterialApp(
          locale: locale,
          supportedLocales: AppLocalizations.supportedLocales,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          home: Builder(
            builder: (context) {
              name = context.l10n.languageName;
              return const SizedBox();
            },
          ),
        ),
      );
      expect(name, expected);
    }
  });
}
