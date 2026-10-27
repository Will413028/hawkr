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
