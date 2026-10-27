/// `google.type.Date` 與 `YYYY-MM-DD` 字串的轉換。
library;

import 'gen/google/type/date.pb.dart';

final _isoDate = RegExp(r'^(\d{4})-(\d{2})-(\d{2})$');

/// `YYYY-MM-DD` 且是存在的日期（年份 1–9999）才回傳；其他一律 null。
Date? parseDate(String value) {
  final match = _isoDate.firstMatch(value);
  if (match == null) return null;
  final year = int.parse(match[1]!);
  if (year < 1) return null; // google.type.Date 的 year 0 是「沒有年份」，不是 0000 年
  final month = int.parse(match[2]!);
  final day = int.parse(match[3]!);
  final check = DateTime.utc(year, month, day);
  if (check.year != year || check.month != month || check.day != day) {
    return null;
  }
  return Date(year: year, month: month, day: day);
}

/// [date] 不是 null 而且年月日都不是 0（`google.type.Date` 的 0 表示該部分未設定）。
bool isDateSet(Date? date) =>
    date != null && date.year != 0 && date.month != 0 && date.day != 0;

/// 完整的日期轉成當地的 [DateTime]（時分秒為 0）；沒有設定（[isDateSet] 為假）回 null。
DateTime? dateToDateTime(Date? date) =>
    isDateSet(date) ? DateTime(date!.year, date.month, date.day) : null;

/// [DateTime] 的年月日（當地）轉成 `google.type.Date`。
Date dateFromDateTime(DateTime value) =>
    Date(year: value.year, month: value.month, day: value.day);

/// 格式化成 `YYYY-MM-DD`；[date] 為 null 或年月日有任何一個是 0（未設定）時回空字串。
String formatDate(Date? date) {
  if (date == null || !isDateSet(date)) {
    return '';
  }
  final year = date.year.toString().padLeft(4, '0');
  final month = date.month.toString().padLeft(2, '0');
  final day = date.day.toString().padLeft(2, '0');
  return '$year-$month-$day';
}
