import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter_test/flutter_test.dart';
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
}
