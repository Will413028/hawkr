/// 臺北時間的格式化，與 website `lib/date/taipei.ts` 同形。
///
/// 臺北是固定的 UTC+8，沒有日光節約時間，所以只加固定位移，不看裝置時區。顯示 Timestamp 一律走這裡，
/// 不用 `toLocal`（輸出會跟著手機時區跑）。
library;

import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';

/// 顯示層的 helper 以 [Timestamp] 當參數型別，從這裡取，各 App 不必直接依賴 protobuf。
export 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    show Timestamp;

const Duration _taipeiUtcOffset = Duration(hours: 8);

String _pad(int n) => n.toString().padLeft(2, '0');

/// `2026/07/30 15:05`（24 小時制、不含秒）；[timestamp] 為 null 時回空字串。
///
/// proto3 的訊息欄位沒設時讀到的是預設值（epoch），呼叫端要先用 `has*()` 確認有設，才傳進來。
String formatTaipeiTimestamp(Timestamp? timestamp) {
  if (timestamp == null) return '';
  return formatTaipeiDateTime(timestamp.toDateTime());
}

/// 同 [formatTaipeiTimestamp]，輸入是 [DateTime]（local 或 UTC 都行，以時間點計）。
String formatTaipeiDateTime(DateTime dateTime) {
  final t = dateTime.toUtc().add(_taipeiUtcOffset);
  return '${t.year}/${_pad(t.month)}/${_pad(t.day)} '
      '${_pad(t.hour)}:${_pad(t.minute)}';
}

/// `2026/07/30`（臺北的日曆日）；輸入是 [DateTime]（local 或 UTC 都行，以時間點計）。
String formatTaipeiDate(DateTime dateTime) {
  final t = dateTime.toUtc().add(_taipeiUtcOffset);
  return '${t.year}/${_pad(t.month)}/${_pad(t.day)}';
}

/// 臺北牆上時間的視圖：回傳的 [DateTime] 是 UTC 旗標、欄位（`year`、`month`、`day`、`hour`、`minute`、`weekday`）讀起來就是臺北時間。
///
/// 取代顯示用的 `toLocal()`（欄位會跟著手機時區跑）。只用來讀欄位與組字串，不要拿它跟 `DateTime.now()`、
/// 另一個時間點比先後或相減（旗標與位移都被改過）；算時間差照舊用原本的時間點。
extension TaipeiWallClock on DateTime {
  DateTime get taipei => toUtc().add(_taipeiUtcOffset);

  /// 字串解析出來的時間（`DateTime.parse` 與 `tryParse`）：帶時區標記的是時間點，換成臺北牆上時間；沒有時區標記的（`isUtc` 為
  /// false）本來就是臺北牆上時間（業務時間字串的約定，與 website `taipei.ts` 的 `WALL_CLOCK_RE` 相同），原樣回傳。
  /// 不用 [taipei]：它對沒有時區標記的值會把裝置時區算進去。
  DateTime get taipeiWall => isUtc ? taipei : this;
}

/// [time] 與 [now] 在臺北日曆日上差幾天（同一天為 0）；只看日曆日，不看經過的小時數。
int taipeiCalendarDaysAgo(DateTime time, {DateTime? now}) {
  final a = time.taipei;
  final b = (now ?? DateTime.now()).taipei;
  return DateTime.utc(
    b.year,
    b.month,
    b.day,
  ).difference(DateTime.utc(a.year, a.month, a.day)).inDays;
}

/// 相對時間的結構值；文字由各 App 的 ARB 組出。
sealed class TimeAgo {
  const TimeAgo();
}

/// 不到一分鐘（含時間在未來）。
final class TimeAgoJustNow extends TimeAgo {
  const TimeAgoJustNow();
  @override
  bool operator ==(Object other) => other is TimeAgoJustNow;
  @override
  int get hashCode => 1;
}

/// 經過的整分鐘，1 至 59。
final class TimeAgoMinutes extends TimeAgo {
  const TimeAgoMinutes(this.count);
  final int count;
  @override
  bool operator ==(Object other) =>
      other is TimeAgoMinutes && other.count == count;
  @override
  int get hashCode => Object.hash(2, count);
}

/// 經過的整小時，1 至 23。
final class TimeAgoHours extends TimeAgo {
  const TimeAgoHours(this.count);
  final int count;
  @override
  bool operator ==(Object other) =>
      other is TimeAgoHours && other.count == count;
  @override
  int get hashCode => Object.hash(3, count);
}

/// 臺北日曆日相差的天數，1 至 6。
final class TimeAgoDays extends TimeAgo {
  const TimeAgoDays(this.count);
  final int count;
  @override
  bool operator ==(Object other) =>
      other is TimeAgoDays && other.count == count;
  @override
  int get hashCode => Object.hash(4, count);
}

/// 七天以上、與 [now] 同一個臺北年份的臺北日期。
final class TimeAgoMonthDay extends TimeAgo {
  const TimeAgoMonthDay(this.month, this.day);
  final int month;
  final int day;
  @override
  bool operator ==(Object other) =>
      other is TimeAgoMonthDay && other.month == month && other.day == day;
  @override
  int get hashCode => Object.hash(5, month, day);
}

/// 七天以上、不同臺北年份的臺北日期。
final class TimeAgoFullDate extends TimeAgo {
  const TimeAgoFullDate(this.year, this.month, this.day);
  final int year;
  final int month;
  final int day;
  @override
  bool operator ==(Object other) =>
      other is TimeAgoFullDate &&
      other.year == year &&
      other.month == month &&
      other.day == day;
  @override
  int get hashCode => Object.hash(6, year, month, day);
}

/// [time] 相對 [now] 的結構值；一小時內與一天內用經過的時間，一天以上的天數與跨年判斷用臺北日曆日，不看裝置時區。
TimeAgo taipeiTimeAgo(DateTime time, {DateTime? now}) {
  final current = now ?? DateTime.now();
  final diff = current.difference(time);
  if (diff.isNegative || diff.inSeconds < 60) return const TimeAgoJustNow();
  if (diff.inMinutes < 60) return TimeAgoMinutes(diff.inMinutes);
  if (diff.inHours < 24) return TimeAgoHours(diff.inHours);
  final days = taipeiCalendarDaysAgo(time, now: current);
  if (days < 7) return TimeAgoDays(days);
  final t = time.taipei;
  if (t.year == current.taipei.year) return TimeAgoMonthDay(t.month, t.day);
  return TimeAgoFullDate(t.year, t.month, t.day);
}

/// 解析時間字串成時間點：帶時區標記的照標記；沒有標記的當臺北牆上時間（業務時間字串的約定），換成對應的 UTC 時間點。
/// 無法解析回 null。結果與裝置時區無關。
DateTime? tryParseTaipeiInstant(String raw) {
  final parsed = DateTime.tryParse(raw);
  if (parsed == null) return null;
  if (parsed.isUtc) return parsed;
  return DateTime.utc(
    parsed.year,
    parsed.month,
    parsed.day,
    parsed.hour,
    parsed.minute,
    parsed.second,
    parsed.millisecond,
    parsed.microsecond,
  ).subtract(_taipeiUtcOffset);
}

/// [offset] 是否為臺北的 +8 時。
bool isTaipeiOffset(Duration offset) => offset == _taipeiUtcOffset;

/// 裝置目前的時區偏移是 +8 時 true；false 時顯示臺北時間處要標「台北時間」（ADR 多語系架構 D5）。
bool isDeviceInTaipeiOffset({DateTime? now}) =>
    isTaipeiOffset((now ?? DateTime.now()).timeZoneOffset);

/// 臺北時間字串 [formatted] 的顯示形：裝置偏移是 +8 時原樣回傳，否則附上 [label]（「台北時間」，取自各 App 的 ARB）成
/// `2026/07/30 15:05 (台北時間)`。空字串不附標籤。是否附標籤只在這裡決定（ADR 多語系架構 D5）。
///
/// 裝置偏移取自 [now]（預設 `DateTime.now()`）；要指定任意偏移（Dart 造不出非裝置時區的 local 時間）用 [withTaipeiLabelAtOffset]。
String withTaipeiLabel(String formatted, String label, {DateTime? now}) =>
    withTaipeiLabelAtOffset(
      formatted,
      label,
      (now ?? DateTime.now()).timeZoneOffset,
    );

/// 同 [withTaipeiLabel]，裝置的 UTC 偏移由 [deviceOffset] 直接給。
String withTaipeiLabelAtOffset(
  String formatted,
  String label,
  Duration deviceOffset,
) {
  if (formatted.isEmpty || isTaipeiOffset(deviceOffset)) return formatted;
  return '$formatted ($label)';
}
