import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:flutter/foundation.dart' show visibleForTesting;
import 'refresh_coalescer.dart';

part 'order_status_notifier.g.dart';

/// 訂單狀態該重讀的訊號。伺服器只給「哪一類狀態變了」（`StateRefreshHint.ORDER`），不帶訂單內容，
/// 所以畫面收到就重讀自己顯示的訂單。
class OrderStatusEvent {
  OrderStatusEvent({DateTime? timestamp})
    : timestamp = timestamp ?? DateTime.now();

  final DateTime timestamp;
}

/// 訂單狀態通知器（Singleton），與 [AfterSaleStatusNotifier] 同架構。由通知串流的 inbox 訊號與前景推播的
/// `state_refresh_hints` 驅動（見 `state_refresh.dart`）。
class OrderStatusNotifier {
  OrderStatusNotifier._internal();

  static final OrderStatusNotifier _instance = OrderStatusNotifier._internal();
  static OrderStatusNotifier get instance => _instance;

  late final _coalescer = RefreshCoalescer(
    () => _eventController.add(OrderStatusEvent()),
  );

  final _eventController = StreamController<OrderStatusEvent>.broadcast();

  Stream<OrderStatusEvent> get eventStream => _eventController.stream;

  /// 通知訂單相關畫面重讀（合併窗內的多次要求只送一次，見 [RefreshCoalescer]）。
  void requestRefresh() => _coalescer.request();

  @visibleForTesting
  set coalesceWindow(Duration value) => _coalescer.window = value;

  void dispose() {
    _coalescer.dispose();
    _eventController.close();
  }
}

/// Riverpod bridge — `ref.watch(orderStatusStreamProvider)` 取得事件 Stream。
@Riverpod(keepAlive: true)
Stream<OrderStatusEvent> orderStatusStream(Ref ref) {
  return OrderStatusNotifier.instance.eventStream;
}
