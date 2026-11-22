import '../../order/data/order_model.dart';
import 'customer_model.dart';

/// 店家訂單列表（`buyer_id` 篩選）的一列轉成客戶詳情的訂單列。
/// 狀態碼：50 已完成、60／70 退款歸已完成、80 已取消，其餘已付款以後都是配送中。
OrderRecord orderRecordFromSummary(OrderSummary o) {
  final code = o.statusCode ?? 0;
  final status = switch (code) {
    50 || 60 || 70 => OrderStatus.completed,
    80 => OrderStatus.cancelled,
    _ => OrderStatus.delivering,
  };
  return OrderRecord(
    orderId: o.orderSn.isNotEmpty ? o.orderSn : o.id,
    date: o.dateTime,
    items: o.productName,
    amount: o.totalAmount.round(),
    status: status,
  );
}
