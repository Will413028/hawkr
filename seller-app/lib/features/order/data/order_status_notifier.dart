import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../shared/utils/hk_log.dart';

part 'order_status_notifier.g.dart';

/// 訂單狀態變更事件
class OrderStatusEvent {
  final String? orderId;
  final String? orderSn;
  final String
  eventType;
  final int? status;
  final Map<String, dynamic>? data;
  final DateTime timestamp;

  OrderStatusEvent({
    this.orderId,
    this.orderSn,
    required this.eventType,
    this.status,
    this.data,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  /// 是否為新訂單事件
  bool get isNewOrder => eventType == 'ORDER_PAID';

  /// 是否為出貨事件
  bool get isShipped => eventType == 'ORDER_SHIPPED';

  /// 是否為完成事件
  bool get isCompleted => eventType == 'ORDER_COMPLETED';

  /// 是否為取消/退款事件
  bool get isCancelledOrRefund =>
      eventType == 'ORDER_CANCELLED' || eventType == 'ORDER_REFUND_REQUESTED';

  @override
  String toString() =>
      'OrderStatusEvent(type=$eventType, orderId=$orderId, orderSn=$orderSn)';
}

/// 訂單狀態通知器
///
/// 功能：
/// 1. 轉發通知串流的訂單重讀訊號（`NotificationSignalService`）
/// 2. 作為備用機制的低頻率 polling（每 60 秒，僅在需要時）
/// 3. 提供 Stream 讓 UI 監聽訂單狀態變更
///
/// 架構：
/// - 主要機制: 通知串流的重讀訊號
/// - 備用機制: Low-frequency Polling (確保送達)
///
/// 使用方式：
/// ```dart
/// // 開始監聽
/// OrderStatusNotifier.instance.startListening();
///
/// // 訂閱狀態變更
/// OrderStatusNotifier.instance.orderEventStream.listen((event) {
///   // 更新 UI
/// });
///
/// // 停止監聽
/// OrderStatusNotifier.instance.stopListening();
/// ```
class OrderStatusNotifier {
  static final OrderStatusNotifier _instance = OrderStatusNotifier._internal();
  factory OrderStatusNotifier() => _instance;
  OrderStatusNotifier._internal();

  static OrderStatusNotifier get instance => _instance;

  /// Polling 間隔（備用機制）
  /// 降低頻率為 60 秒，因為主要依賴 Silent Push
  static const Duration _pollingInterval = Duration(seconds: 60);

  /// 最大 polling 次數（防止無限 polling）
  static const int _maxPollingCount = 60;

  final _orderEventController = StreamController<OrderStatusEvent>.broadcast();
  Timer? _pollingTimer;
  int _pollingCount = 0;
  bool _isListening = false;

  /// 訂單事件 Stream
  Stream<OrderStatusEvent> get orderEventStream => _orderEventController.stream;

  /// 是否正在監聽
  bool get isListening => _isListening;

  /// 開始監聯聽狀態變更
  ///
  /// [enablePolling] 是否啟用備用 polling（預設 false，依賴 Silent Push）
  Future<void> startListening({bool enablePolling = false}) async {
    if (_isListening) {
      HKLog.logD('[OrderStatusNotifier] Already listening');
      return;
    }

    _isListening = true;
    _pollingCount = 0;
    HKLog.logI('[OrderStatusNotifier] Start listening');

    if (enablePolling) {
      _startPolling();
    }
  }

  /// 停止監聽
  void stopListening() {
    _isListening = false;
    _stopPolling();
    HKLog.logI('[OrderStatusNotifier] Stopped listening');
  }

  /// 伺服器訊號（通知串流的狀態重讀提示、賣家工作區訊號、斷線重連）：訂單可能變了，各畫面重讀。
  void emitRefresh() =>
      _emitEvent(OrderStatusEvent(eventType: refreshEventType));

  /// [emitRefresh] 發出的事件型別。
  static const refreshEventType = 'REFRESH';

  /// 手動觸發訂單事件（用於 WebSocket 遷移期間的相容性）
  void emitOrderEvent(Map<String, dynamic> data) {
    final eventType =
        data['event']?.toString() ??
        data['eventType']?.toString() ??
        (data['data'] is Map ? data['data']['event']?.toString() : null) ??
        'UNKNOWN';

    final event = OrderStatusEvent(
      orderId:
          data['orderId']?.toString() ??
          (data['data'] is Map ? data['data']['orderId']?.toString() : null),
      orderSn:
          data['orderSn']?.toString() ??
          (data['data'] is Map ? data['data']['orderSn']?.toString() : null),
      eventType: eventType,
      data: data,
    );

    _emitEvent(event);
  }

  /// 發送事件
  void _emitEvent(OrderStatusEvent event) {
    HKLog.logI('[OrderStatusNotifier] Emitting event: $event');
    _orderEventController.add(event);
  }

  /// 手動獲取訂單列表（可選的 polling 方法）
  Future<void> fetchOrders() async {
    try {
      HKLog.logD('[OrderStatusNotifier] Fetch orders (backup polling)');
    } catch (e) {
      HKLog.logE('[OrderStatusNotifier] Fetch orders error', e);
    }
  }

  /// 開始 polling（備用機制）
  void _startPolling() {
    if (_pollingTimer != null) return;

    HKLog.logD('[OrderStatusNotifier] Start backup polling');
    _pollingTimer = Timer.periodic(_pollingInterval, (_) {
      _pollingCount++;

      if (_pollingCount >= _maxPollingCount) {
        HKLog.logW('[OrderStatusNotifier] Max polling count reached, stopping');
        _stopPolling();
        return;
      }

      if (_isListening) {
        HKLog.logD('[OrderStatusNotifier] Polling #$_pollingCount');
        fetchOrders();
      }
    });
  }

  /// 停止 polling
  void _stopPolling() {
    _pollingTimer?.cancel();
    _pollingTimer = null;
    _pollingCount = 0;
    HKLog.logD('[OrderStatusNotifier] Stopped polling');
  }

  /// 清理資源
  void dispose() {
    stopListening();
    _orderEventController.close();
  }
}

/// Riverpod bridge — 讓 UI 用 `ref.watch(orderStatusStreamProvider)` 訂閱訂單事件，
/// 享受 AsyncValue / `.when()` 的便利。
@Riverpod(keepAlive: true)
Stream<OrderStatusEvent> orderStatusStream(Ref ref) {
  return OrderStatusNotifier.instance.orderEventStream;
}
