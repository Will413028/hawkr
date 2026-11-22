/// 臺北時間的格式化，與 website `lib/date/taipei.ts` 同形。
///
/// 臺北是固定的 UTC+8，沒有日光節約時間，所以只加固定位移，不看裝置時區。顯示 Timestamp 一律走這裡，
/// 不用 `toLocal`（輸出會跟著手機時區跑）。
library;

import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';

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
