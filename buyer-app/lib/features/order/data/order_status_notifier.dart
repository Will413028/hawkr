import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../shared/utils/hk_log.dart';

part 'order_status_notifier.g.dart';

/// 訂單狀態變更事件（FCM Silent Push 驅動）。
class OrderStatusEvent {
  OrderStatusEvent({
    this.orderId,
    this.orderSn,
    required this.eventType,
    this.status,
    this.previousStatus,
    this.data,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  final String? orderId;
  final String? orderSn;

  /// 事件類型，例如 ORDER_PAID / ORDER_SHIPPED / ORDER_COMPLETED / ORDER_CANCELLED
  final String eventType;

  /// 新狀態值（OrderStatus 對應）
  final String? status;
  final String? previousStatus;
  final Map<String, dynamic>? data;
  final DateTime timestamp;

  @override
  String toString() =>
      'OrderStatusEvent(type=$eventType, orderSn=$orderSn, status=$previousStatus→$status)';
}

/// 訂單狀態通知器（Singleton），與 [AfterSaleStatusNotifier] 同架構。
///
/// 後端 FCM payload 範例：
/// ```json
/// {
///   "type": "ORDER_STATUS",
///   "eventType": "ORDER_SHIPPED",
///   "orderSn": "O306817753",
///   "status": "30",
///   "previousStatus": "20"
/// }
/// ```
class OrderStatusNotifier {
  OrderStatusNotifier._internal();

  static final OrderStatusNotifier _instance = OrderStatusNotifier._internal();
  static OrderStatusNotifier get instance => _instance;

  final _eventController = StreamController<OrderStatusEvent>.broadcast();

  Stream<OrderStatusEvent> get eventStream => _eventController.stream;

  /// 處理來自 FCM 的訂單狀態更新。由 [NotificationRouter] 呼叫。
  void handlePushNotification(Map<String, dynamic> data) {
    HKLog.logI('[OrderStatusNotifier] Received push: $data');
    try {
      final event = OrderStatusEvent(
        orderId: data['orderId']?.toString() ?? data['order_id']?.toString(),
        orderSn:
            data['orderSn']?.toString() ??
            data['order_sn']?.toString() ??
            data['refId']?.toString(),
        eventType:
            data['eventType']?.toString() ??
            data['event']?.toString() ??
            'UNKNOWN',
        status: data['status']?.toString(),
        previousStatus:
            data['previousStatus']?.toString() ??
            data['previous_status']?.toString(),
        data: data,
      );
      _eventController.add(event);
      HKLog.logI('[OrderStatusNotifier] Emit $event');
    } catch (e) {
      HKLog.logE('[OrderStatusNotifier] parse error', e);
    }
  }

  void dispose() => _eventController.close();
}

/// Riverpod bridge — `ref.watch(orderStatusStreamProvider)` 取得事件 Stream。
@Riverpod(keepAlive: true)
Stream<OrderStatusEvent> orderStatusStream(Ref ref) {
  return OrderStatusNotifier.instance.eventStream;
}
