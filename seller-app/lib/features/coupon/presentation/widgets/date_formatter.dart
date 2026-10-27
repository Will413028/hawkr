/// 日期格式化工具类
///
/// 提供优惠券日期的格式化功能
class DateFormatter {
  /// 将 ISO 8601 日期字符串转换为 XXXX/XX/XX 格式
  ///
  /// 输入示例：2026-04-06T00:00:00Z
  /// 输出示例：2026/04/06
  ///
  /// [dateStr] ISO 8601 格式的日期字符串
  /// 返回格式化后的日期字符串，如果解析失败返回原字符串
  static String formatToYYYYMMDD(String? dateStr) {
    if (dateStr == null || dateStr.isEmpty) return '';

    try {
      // 解析 ISO 8601 格式
      final utc = DateTime.parse(dateStr);

      // 转换为本地时区
      final local = utc.toLocal();

      // 格式化为 XXXX/XX/XX
      final year = local.year;
      final month = local.month.toString().padLeft(2, '0');
      final day = local.day.toString().padLeft(2, '0');

      return '$year/$month/$day';
    } catch (e) {
      // 解析失败时返回原字符串
      return dateStr;
    }
  }

  /// 将 DateTime 对象转换为 XXXX/XX/XX 格式
  ///
  /// [date] DateTime 对象
  /// 返回格式化后的日期字符串
  static String formatDateTimeToYYYYMMDD(DateTime? date) {
    if (date == null) return '';

    final year = date.year;
    final month = date.month.toString().padLeft(2, '0');
    final day = date.day.toString().padLeft(2, '0');

    return '$year/$month/$day';
  }

  /// 检查日期是否已过期
  ///
  /// [dateStr] ISO 8601 格式的日期字符串
  /// 返回 true 表示已过期，false 表示未过期
  static bool isExpired(String? dateStr) {
    if (dateStr == null || dateStr.isEmpty) return false;

    try {
      final utc = DateTime.parse(dateStr);
      final local = utc.toLocal();
      final now = DateTime.now();

      // 比较日期（忽略时间）
      final expireDate = DateTime(local.year, local.month, local.day);
      final today = DateTime(now.year, now.month, now.day);

      return expireDate.isBefore(today);
    } catch (e) {
      return false;
    }
  }
}
