/// 日期時間格式化工具
class DateFormatUtils {
  DateFormatUtils._();

  /// 將 ISO 8601 時間字串轉為本地顯示格式 yyyy/MM/dd HH:mm
  ///
  /// [raw] 後端回傳的 UTC 時間字串（如 "2026-04-10T08:30:00Z"）
  /// 回傳格式範例：2026/04/10 16:30
  /// 若解析失敗則原樣回傳，空字串直接回傳空
  static String formatDateTime(String raw) {
    if (raw.isEmpty) return '';
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final local = dt.toLocal();
    return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }
}
