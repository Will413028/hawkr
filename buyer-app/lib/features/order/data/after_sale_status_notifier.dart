import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../shared/utils/hk_log.dart';

part 'after_sale_status_notifier.g.dart';

/// 售後案件狀態變更事件（FCM Silent Push 驅動）。
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

  /// 案件編號（必填）
  final String caseNo;
  final String? orderNo;

  /// 新狀態（AfterSaleStatus 值：`0 / 20 / 25 / 30 / 40 / 50 / 60 / 70` …；
  /// 65=REOPENED 已於後端 #1241 淘汰）
  final String? status;

  /// 前一個狀態，供 UI 判斷是「賣家剛回覆」還是「退款剛完成」
  final String? previousStatus;

  /// 觸發者 (`'buyer' | 'seller' | 'system' | 'platform'`)
  final String? actor;

  /// 原始 payload，除錯用
  final Map<String, dynamic>? data;

  final DateTime timestamp;

  @override
  String toString() =>
      'AfterSaleStatusEvent(case=$caseNo, status=$previousStatus→$status, actor=$actor)';
}

/// 售後狀態通知器（Singleton）。
///
/// 後端 FCM payload 範例（見 backend-payuni B-98）：
/// ```json
/// {
///   "type": "AFTER_SALE_STATUS",
///   "caseNo": "A123456789",
///   "orderNo": "E2025110511",
///   "status": "20",
///   "previousStatus": "0",
///   "actor": "seller"
/// }
/// ```
///
/// UI 使用方式：
/// ```dart
/// ref.listen(afterSaleStatusStreamProvider, (_, next) {
///   next.whenData((event) {
///     ref.invalidate(afterSaleListProvider);
///     if (watchingCaseNo == event.caseNo) ref.invalidate(afterSaleDetailProvider);
///   });
/// });
/// ```
class AfterSaleStatusNotifier {
  AfterSaleStatusNotifier._internal();

  static final AfterSaleStatusNotifier _instance =
      AfterSaleStatusNotifier._internal();
  static AfterSaleStatusNotifier get instance => _instance;

  final _eventController = StreamController<AfterSaleStatusEvent>.broadcast();

  Stream<AfterSaleStatusEvent> get eventStream => _eventController.stream;

  /// 處理來自 FCM 的售後狀態更新。由 [NotificationRouter] 呼叫。
  void handlePushNotification(Map<String, dynamic> data) {
    HKLog.logI('[AfterSaleStatusNotifier] Received push: $data');
    try {
      final caseNo =
          data['caseNo']?.toString() ??
          data['case_no']?.toString() ??
          data['refId']?.toString();
      if (caseNo == null || caseNo.isEmpty) {
        HKLog.logW('[AfterSaleStatusNotifier] No caseNo, drop event');
        return;
      }
      final event = AfterSaleStatusEvent(
        caseNo: caseNo,
        orderNo: data['orderNo']?.toString() ?? data['order_no']?.toString(),
        status: data['status']?.toString(),
        previousStatus:
            data['previousStatus']?.toString() ??
            data['previous_status']?.toString(),
        actor: data['actor']?.toString(),
        data: data,
      );
      _eventController.add(event);
      HKLog.logI('[AfterSaleStatusNotifier] Emit $event');
    } catch (e) {
      HKLog.logE('[AfterSaleStatusNotifier] parse error', e);
    }
  }

  void dispose() => _eventController.close();
}

/// Riverpod bridge — `ref.watch(afterSaleStatusStreamProvider)` 取得事件 Stream。
@Riverpod(keepAlive: true)
Stream<AfterSaleStatusEvent> afterSaleStatusStream(Ref ref) {
  return AfterSaleStatusNotifier.instance.eventStream;
}
