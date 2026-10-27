/// TSID 主鍵是 64-bit BIGINT。後端正逐步把「推給前端」的 id 從 JSON number 改成
/// string（見 notification #960/#961 與同期 TSID 稽核）。為了讓 client 在後端翻字串
/// 前後都不會 crash（json_serializable 產出的 `(json['id'] as num)` 對 String 會丟
/// 例外），凡 TSID id 欄位一律經此容錯解析：number 與 string 皆接受。
///
/// Dart 的 int 為 64-bit，string→int 無精度損失，故內部仍以 int 保存、不影響下游。
library;

/// 容錯解析必填 TSID：接受 JSON number 或 string。
int tsidToInt(Object? value) {
  if (value is int) return value;
  if (value is String) return int.parse(value);
  if (value is num) return value.toInt();
  throw FormatException('tsidToInt: 預期 int/num/String，實得 ${value.runtimeType}');
}

/// 容錯解析可選 TSID：null 維持 null，其餘同 [tsidToInt]。
int? tsidToIntOrNull(Object? value) {
  if (value == null) return null;
  return tsidToInt(value);
}
