/// 共用文案目錄（契約值到使用者面文字）。生成的 API 在 `src/ui_text.g.dart`，這裡只放手寫的 [UiLocale]。
///
/// 已發佈語系是各介面自己的清單（ADR 2026-10-17-multi-locale-architecture D1），每支 App 在 `lib/core/l10n/ui_locale.dart`
/// 宣告；解析規則（四端相同）在 [resolveUiLocale]。
library;

export 'src/ui_text.g.dart';

/// UI 語系。成員與 `catalog/config.json` 的 `locales` 一一對應；生成碼的 switch 不留萬用分支，
/// 兩邊不一致時 `dart analyze` 會報「switch 沒有涵蓋所有成員」。
enum UiLocale {
  zhTW('zh-TW'),
  en('en');

  const UiLocale(this.tag);

  /// BCP 47 標籤，與目錄的語系檔名相同。
  final String tag;
}

/// 依裝置語言選 UI 語系（ADR 2026-10-17-multi-locale-architecture D1 的解析規則，四端相同）：中文一律 zh-TW；其他語言在 en 已發佈時用 en，
/// 否則 zh-TW。[deviceLanguageCode] 是裝置語系的語言碼（`Locale.languageCode`），沒有語系是 null；[published] 是這個介面已發佈的語系，
/// 未發佈的語系在解析時視為不存在。
UiLocale resolveUiLocale(String? deviceLanguageCode, List<UiLocale> published) {
  if (deviceLanguageCode == null || deviceLanguageCode == 'zh') {
    return UiLocale.zhTW;
  }
  return published.contains(UiLocale.en) ? UiLocale.en : UiLocale.zhTW;
}
