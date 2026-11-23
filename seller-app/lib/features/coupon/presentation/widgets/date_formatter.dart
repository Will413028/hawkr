import 'package:hawkr_api/taipei_time.dart';

/// 日期格式化工具类
///
/// 提供优惠券日期的格式化功能
class DateFormatter {
  /// 将 ISO 8601 日期字符串转换为 XXXX/XX/XX 格式
  ///
  /// 输入示例：2026-04-06T00:00:00Z
  /// 输出示例：2026/04/06
  ///
  /// 返回格式化后的日期字符串，如果解析失败返回原字符串（此时不是臺北日期，不附「台北時間」标签；要标签用 [tryFormatToYYYYMMDD]）
  static String formatToYYYYMMDD(String? dateStr) =>
      tryFormatToYYYYMMDD(dateStr) ?? dateStr ?? '';

  /// 同 [formatToYYYYMMDD]，空字符串或解析失败回 null。
  static String? tryFormatToYYYYMMDD(String? dateStr) {
    if (dateStr == null || dateStr.isEmpty) return null;
    final utc = DateTime.tryParse(dateStr);
    if (utc == null) return null;
    final local = utc.taipeiWall;
    final month = local.month.toString().padLeft(2, '0');
    final day = local.day.toString().padLeft(2, '0');
    return '${local.year}/$month/$day';
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
      final local = utc.taipeiWall;
      final now = DateTime.now().taipei;

      // 比较日期（忽略时间）
      final expireDate = DateTime(local.year, local.month, local.day);
      final today = DateTime(now.year, now.month, now.day);

      return expireDate.isBefore(today);
    } catch (e) {
      return false;
    }
  }
}
