import 'package:flutter/widgets.dart';
import 'package:hawkr_api/taipei_time.dart';

import '../../l10n/l10n.dart';
import '../../l10n/time_ago_text.dart';

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

  /// 相對時間文字 [text]（由 [ago] 產生）；只有一天以上的臺北日期（月日、年月日）附標籤，「N 分鐘前」這類經過時間不附。
  String agoText(TimeAgo ago, String text) =>
      ago is TimeAgoMonthDay || ago is TimeAgoFullDate ? labeled(text) : text;

  /// [time] 的相對時間（[taipeiTimeAgo]）與 ARB 文字，日期形時附標籤。
  String timeAgo(AppLocalizations l, DateTime time, {DateTime? now}) {
    final ago = taipeiTimeAgo(time, now: now);
    return agoText(ago, timeAgoText(l, ago));
  }
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

extension TaipeiDisplayContext on BuildContext {
  TaipeiDisplay get taipeiDisplay => TaipeiDisplay(
    label: l10n.taipeiTimeLabel,
    deviceOffset:
        dependOnInheritedWidgetOfExactType<TaipeiDisplayScope>()?.deviceOffset,
  );
}
