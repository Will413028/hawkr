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

/// 瀏覽紀錄的相對時間標籤；一天以上的「昨天」「N 天前」與日期以臺北日曆日判斷。
String historyViewedText(AppLocalizations l, DateTime time, {DateTime? now}) =>
    switch (taipeiTimeAgo(time, now: now)) {
      TimeAgoJustNow() => l.viewedJustNow,
      TimeAgoMinutes(:final count) => l.viewedMinutes(count),
      TimeAgoHours(:final count) => l.viewedHours(count),
      TimeAgoDays(count: <= 1) => l.viewedYesterday,
      TimeAgoDays(:final count) => l.viewedDays(count),
      TimeAgoMonthDay(:final month, :final day) => l.viewedDate(month, day),
      TimeAgoFullDate(:final month, :final day) => l.viewedDate(month, day),
    };
