import 'dart:ui' show PlatformDispatcher;

import 'package:hawkr_api/errors.dart' show DefaultErrorText;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

/// 這支 App 已發佈的 UI 語系（ADR 2026-10-17-multi-locale-architecture D1）：未發佈的語系在解析時視為不存在。
const publishedUiLocales = [UiLocale.zhTW];

final uiLocaleProvider = Provider<UiLocale>(
  (ref) => resolveUiLocale(
    PlatformDispatcher.instance.locale.languageCode,
    publishedUiLocales,
  ),
);

/// Connect code 的預設文字（共用文案目錄）：`hawkr_api` 的錯誤 helper 不能依賴目錄，由這裡注入。
final defaultErrorTextProvider = Provider<DefaultErrorText>((ref) {
  final locale = ref.watch(uiLocaleProvider);
  return (code) => commonErrorText(code, locale)!;
});
