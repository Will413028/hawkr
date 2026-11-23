import 'package:flutter/widgets.dart';
import 'package:hawkr_api/taipei_time.dart';

import '../../l10n/l10n.dart';
import '../../l10n/time_ago_text.dart';

export 'package:hawkr_api/taipei_time.dart' show TaipeiDisplay, TaipeiFormat;

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

/// 臺北時間的顯示（格式化與標籤）在 `hawkr_api` 的 [TaipeiDisplay]；App 只注入 ARB 的 `taipeiTimeLabel`。
extension TaipeiDisplayContext on BuildContext {
  TaipeiDisplay get taipeiDisplay => TaipeiDisplay(
    label: l10n.taipeiTimeLabel,
    deviceOffset:
        dependOnInheritedWidgetOfExactType<TaipeiDisplayScope>()?.deviceOffset,
  );
}

extension TaipeiDisplayL10n on TaipeiDisplay {
  /// [time] 的相對時間文字（ARB 的 [timeAgoText]），日期形時附標籤。
  String timeAgoIn(AppLocalizations l, DateTime time, {DateTime? now}) =>
      timeAgo(time, (ago) => timeAgoText(l, ago), now: now);
}
