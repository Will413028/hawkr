import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'after_sale_status_notifier.g.dart';

/// 售後狀態該重讀的訊號（`StateRefreshHint.AFTER_SALE`）：不帶案件內容，畫面收到就重讀自己顯示的案件。
class AfterSaleStatusEvent {
  AfterSaleStatusEvent({DateTime? timestamp})
    : timestamp = timestamp ?? DateTime.now();

  final DateTime timestamp;
}

/// 售後狀態通知器（Singleton）。由通知串流的 inbox 訊號與前景推播的 `state_refresh_hints` 驅動
/// （見 `state_refresh.dart`）。
///
/// ```dart
/// ref.listen(afterSaleStatusStreamProvider, (_, next) {
///   next.whenData((_) => ref.invalidate(afterSaleListProvider));
/// });
/// ```
class AfterSaleStatusNotifier {
  AfterSaleStatusNotifier._internal();

  static final AfterSaleStatusNotifier _instance =
      AfterSaleStatusNotifier._internal();
  static AfterSaleStatusNotifier get instance => _instance;

  final _eventController = StreamController<AfterSaleStatusEvent>.broadcast();

  Stream<AfterSaleStatusEvent> get eventStream => _eventController.stream;

  /// 通知售後相關畫面重讀。
  void requestRefresh() => _eventController.add(AfterSaleStatusEvent());

  void dispose() => _eventController.close();
}

/// Riverpod bridge — `ref.watch(afterSaleStatusStreamProvider)` 取得事件 Stream。
@Riverpod(keepAlive: true)
Stream<AfterSaleStatusEvent> afterSaleStatusStream(Ref ref) {
  return AfterSaleStatusNotifier.instance.eventStream;
}
