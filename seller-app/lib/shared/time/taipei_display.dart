import 'package:flutter/widgets.dart';
import 'package:hawkr_api/taipei_time.dart';

import '../../l10n/l10n.dart';

/// 臺北時間的顯示：格式化與「台北時間」標籤都只走這裡（ADR 多語系架構 D5）。
///
/// 裝置偏移不是 +8 時，顯示出來的臺北時間後面附標籤；一行裡有多個時間時，用 [plainTimestamp] 等組好整行，再用 [labeled] 附一次。
/// 直接呼叫 `formatTaipei*` 與 `toLocal` 由 CI 的 `scripts/check-taipei-display.py` 擋。
class TaipeiDisplay {
  const TaipeiDisplay({required this.label, this.deviceOffset});

  /// ARB 的 `taipeiTimeLabel`。
  final String label;

  /// 裝置的 UTC 偏移；null 取 `DateTime.now()` 的偏移。測試用 [TaipeiDisplayScope] 注入。
  final Duration? deviceOffset;

  Duration get _offset => deviceOffset ?? DateTime.now().timeZoneOffset;

  /// 偏移不是 +8 時是 [label]，否則 null（給輸入框的 helperText 這類獨立顯示的位置）。
  String? get labelIfNeeded => isTaipeiOffset(_offset) ? null : label;

  /// 附標籤（偏移是 +8 或 [formatted] 為空時原樣）。
  String labeled(String formatted) =>
      withTaipeiLabelAtOffset(formatted, label, _offset);

  String plainTimestamp(Timestamp? timestamp) =>
      formatTaipeiTimestamp(timestamp);
  String plainDateTime(DateTime dateTime) => formatTaipeiDateTime(dateTime);
  String plainDate(DateTime dateTime) => formatTaipeiDate(dateTime);

  /// `2026/07/30 15:05`，必要時附標籤；[timestamp] 為 null 時空字串。
  String timestamp(Timestamp? timestamp) => labeled(plainTimestamp(timestamp));
  String dateTime(DateTime dateTime) => labeled(plainDateTime(dateTime));
  String date(DateTime dateTime) => labeled(plainDate(dateTime));
}

/// 測試用：在 widget 樹上方指定裝置偏移；沒有它時取 `DateTime.now()` 的偏移。
class TaipeiDisplayScope extends InheritedWidget {
  const TaipeiDisplayScope({
    super.key,
    required this.deviceOffset,
    required super.child,
  });

  final Duration deviceOffset;

  @override
  bool updateShouldNotify(TaipeiDisplayScope oldWidget) =>
      deviceOffset != oldWidget.deviceOffset;
}

/// 樹上沒有 App 的 localizations（只在沒掛 delegate 的測試樹出現）時用預設語系 zh 的標籤，不丟例外。
String _label(BuildContext context) =>
    (Localizations.of<AppLocalizations>(context, AppLocalizations) ??
            lookupAppLocalizations(const Locale('zh')))
        .taipeiTimeLabel;

extension TaipeiDisplayContext on BuildContext {
  TaipeiDisplay get taipeiDisplay => TaipeiDisplay(
    label: _label(this),
    deviceOffset:
        dependOnInheritedWidgetOfExactType<TaipeiDisplayScope>()?.deviceOffset,
  );
}
