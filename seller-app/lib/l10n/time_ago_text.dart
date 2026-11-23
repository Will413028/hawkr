import 'package:hawkr_api/taipei_time.dart';

import 'gen/app_localizations.dart';

/// [TimeAgo] 的 ARB 文字。
String timeAgoText(AppLocalizations l, TimeAgo ago) => switch (ago) {
  TimeAgoJustNow() => l.timeAgoJustNow,
  TimeAgoMinutes(:final count) => l.timeAgoMinutes(count),
  TimeAgoHours(:final count) => l.timeAgoHours(count),
  TimeAgoDays(:final count) => l.timeAgoDays(count),
  TimeAgoMonthDay(:final month, :final day) => l.timeAgoMonthDay(month, day),
  TimeAgoFullDate(:final year, :final month, :final day) => l.timeAgoFullDate(
    year,
    month,
    day,
  ),
};
