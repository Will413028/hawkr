import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'after_sale_status_notifier.g.dart';

/// 售後案件狀態變更事件。
class AfterSaleStatusEvent {
  AfterSaleStatusEvent({
    required this.caseNo,
    this.orderNo,
    this.status,
    this.previousStatus,
    this.actor,
    this.data,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  /// 不指定案件、要求全部重讀的事件。
  AfterSaleStatusEvent.refreshAll()
    : caseNo = '',
      orderNo = null,
      status = null,
      previousStatus = null,
      actor = null,
      data = null,
      timestamp = DateTime.now();

  final String caseNo;

  bool get refreshesAll => caseNo.isEmpty;
  final String? orderNo;
  final String? status;
  final String? previousStatus;

  /// `'buyer' | 'seller' | 'system' | 'platform'`
  final String? actor;

  final Map<String, dynamic>? data;
  final DateTime timestamp;

  @override
  String toString() =>
      'AfterSaleStatusEvent(case=$caseNo, status=$previousStatus→$status, actor=$actor)';
}

/// 售後狀態通知器（Singleton），與 [OrderStatusNotifier] 同架構。
///
/// 後端 FCM payload 參見 backend-payuni B-98。賣家端用途：
/// - 買家取消/補件 時，重新整理 待處理 tab
/// - 平台裁決完成（status → 50/90）時，將案件移到 已結案 tab
class AfterSaleStatusNotifier {
  AfterSaleStatusNotifier._internal();

  static final AfterSaleStatusNotifier _instance =
      AfterSaleStatusNotifier._internal();
  static AfterSaleStatusNotifier get instance => _instance;

  final _eventController = StreamController<AfterSaleStatusEvent>.broadcast();

  Stream<AfterSaleStatusEvent> get eventStream => _eventController.stream;

  /// 伺服器訊號（通知串流的售後重讀提示、賣家工作區訊號、斷線重連）：不知道是哪一案，各畫面都重讀。
  void emitRefreshAll() =>
      _eventController.add(AfterSaleStatusEvent.refreshAll());

  void dispose() => _eventController.close();
}

@Riverpod(keepAlive: true)
Stream<AfterSaleStatusEvent> afterSaleStatusStream(Ref ref) {
  return AfterSaleStatusNotifier.instance.eventStream;
}
