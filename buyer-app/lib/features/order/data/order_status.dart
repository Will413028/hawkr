import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'dart:ui' show Color;
import 'package:hawkr_buyer/core/codemeta/code_meta.dart';

/// 後端訂單狀態值對應
enum OrderStatus {
  pendingPayment('0', '待付款'),
  paid('10', '已付款'),
  pendingShipment('20', '待出貨'),
  inTransit('30', '配送中'),
  waitingPickup('32', '待取貨'),
  shippingAbnormal('35', '配送異常'),
  delivered('40', '已送達'),
  completed('50', '已完成'),
  partialRefund('60', '部分退款'),
  fullRefund('70', '全額退款'),
  cancelled('80', '已取消');

  const OrderStatus(this.value, this.label);

  /// 後端 API 回傳的 status 值
  final String value;

  /// 中文顯示名稱
  final String label;

  static OrderStatus? fromValue(String value) {
    for (final s in OrderStatus.values) {
      if (s.value == value) return s;
    }
    return null;
  }

  static String labelFor(String value) {
    return fromValue(value)?.label ?? '未知狀態';
  }
}

/// Tab 對應的 status 篩選值
class OrderTabFilter {
  static const String pendingPayment = '0';
  static const String pendingShipment = '10,20';
  static const String inTransit = '30,32,35,40';
  static const String completed = '50,60,70,80';

  static const List<String> tabLabels = ['待付款', '待出貨', '配送中', '已完成'];
  static const List<String> tabFilters = [
    pendingPayment,
    pendingShipment,
    inTransit,
    completed,
  ];
}

/// 物流方式 value → 顯示名稱
class ShippingMethodLabel {
  static String labelFor(String value) =>
      ShippingMethod.fromValue(value)?.label ??
      (value.isNotEmpty ? value : '未指定');
}

/// 溫層 value → 顯示名稱 & 顏色
class DeliveryTempLabel {
  static String labelFor(String value) {
    final normalized = value.trim().toUpperCase();
    for (final temperature in ShippingTemperature.values) {
      if (normalized == temperature.value ||
          normalized == temperature.code ||
          normalized == temperature.label) {
        return temperature.label;
      }
    }
    return value;
  }

  static Color colorFor(String value) => switch (labelFor(value)) {
    '冷藏' => AppColors.temperatureChilled,
    '冷凍' => AppColors.temperatureFrozen,
    _ => AppColors.temperatureAmbient,
  };
}
