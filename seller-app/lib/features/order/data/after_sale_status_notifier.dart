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

  final String caseNo;
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

  void handlePushNotification(Map<String, dynamic> data) {
    HKLog.logI('[AfterSaleStatusNotifier] Received push: $data');
    try {
      final caseNo = data['caseNo']?.toString() ?? data['refId']?.toString();
      if (caseNo == null || caseNo.isEmpty) {
        HKLog.logW('[AfterSaleStatusNotifier] No caseNo, drop event');
        return;
      }
      final event = AfterSaleStatusEvent(
        caseNo: caseNo,
        orderNo: data['orderNo']?.toString(),
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

@Riverpod(keepAlive: true)
Stream<AfterSaleStatusEvent> afterSaleStatusStream(Ref ref) {
  return AfterSaleStatusNotifier.instance.eventStream;
}
