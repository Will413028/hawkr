import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/order/presentation/order_status_presentation.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const labels = <int, String>{
    0: '待付款',
    10: '已付款',
    20: '待出貨',
    30: '配送中',
    32: '待取貨',
    35: '物流異常',
    40: '已送達',
    50: '已完成',
    60: '部分退款',
    70: '全額退款',
    80: '已取消',
  };

  test('11 種訂單主狀態都有賣家視角名稱與下一步', () {
    for (final entry in labels.entries) {
      final visual = OrderStatusPresentation.fromOrder(
        _order(statusCode: entry.key),
      );
      expect(visual.label, entry.value, reason: 'status=${entry.key}');
      expect(visual.description, isNotEmpty, reason: 'status=${entry.key}');
    }
  });

  test('所有已知物流碼與建單失敗碼都有明確顯示', () {
    const known = <String>{
      '91',
      '92',
      '98',
      '21',
      '22',
      '31',
      '32',
      '33',
      '11',
      '41',
      '43',
      '44',
      '46',
      '51',
      '52',
      '53',
      '55',
      '56',
      '81',
      '82',
      '210',
      '920',
      '921',
      '990',
    };
    const abnormal = <String>{
      '33',
      '41',
      '43',
      '44',
      '46',
      '52',
      '55',
      '81',
      '82',
      '990',
    };

    for (final value in known) {
      final visual = DeliveryStatusPresentation.fromOrder(
        _order(statusCode: 30, shippingStatus: value),
      );
      expect(visual.label, isNotEmpty, reason: 'shippingStatus=$value');
      expect(
        visual.isAbnormal,
        abnormal.contains(value),
        reason: 'shippingStatus=$value',
      );
    }
  });

  test('未知狀態與賣家自訂取消原因安全呈現', () {
    final unknown = OrderStatusPresentation.fromOrder(_order(statusCode: 999));
    final cancelled = OrderStatusPresentation.fromOrder(
      _order(statusCode: 80, cancelReason: '9299', cancelRemark: '產地臨時停班'),
    );

    expect(unknown.label, '狀態更新中');
    expect(cancelled.description, contains('產地臨時停班'));
  });
}

OrderSummary _order({
  required int statusCode,
  String shippingStatus = '',
  String cancelReason = '',
  String cancelRemark = '',
}) => OrderSummary(
  id: 'O125369895',
  orderSn: 'O125369895',
  statusCode: statusCode,
  title: '訂單',
  subtitle: statusCode.toString(),
  dateTime: '2026-07-16T06:38:00+08:00',
  productName: '蟹味棒',
  price: '250',
  shipmentStatus: shippingStatus,
  shippingStatus: shippingStatus,
  cancelReason: cancelReason,
  cancelRemark: cancelRemark,
);
