import '../../order/data/order_model.dart';
import 'customer_model.dart';

/// 店家訂單列表（`buyer_id` 篩選）的一列轉成客戶詳情的訂單列。
/// 狀態碼（訂單狀態契約）：0 待付款；10／20／30／32／40 履約中歸配送中；35 物流異常；
/// 50／54／60／70 已結案（完成、配送失敗未退款、部分與全額退款）歸已完成；80 已取消。
OrderRecord orderRecordFromSummary(OrderSummary o) {
  final code = o.statusCode ?? 0;
  final status = switch (code) {
    0 => OrderStatus.pendingPayment,
    35 => OrderStatus.issue,
    50 || 54 || 60 || 70 => OrderStatus.completed,
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
