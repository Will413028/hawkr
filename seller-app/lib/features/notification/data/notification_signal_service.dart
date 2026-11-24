import 'dart:async';
import 'dart:math' as math;

import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    show WatchNotificationsResponse_Event;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../shared/utils/hk_log.dart';
import 'notification_repository.dart';
import 'state_refresh.dart';

part 'notification_signal_service.g.dart';

/// `WatchNotifications`（STORE 受眾）的 client：只收「該重讀了」的訊號，收到就重讀對應的資料；
/// 這條串流同時承載賣家工作區事件（訂單、售後）。
///
/// - 每次連線（第一次連線與斷線重連）都先全部重讀一次再收訊號：串流沒有續傳，連線前的變化只能靠重讀補上
///   （冷啟動、回前景重建串流時同理）。
/// - 伺服器正常結束：立刻重連；沒收到任何訊號就結束算失敗，走退避。
/// - unauthenticated、permission_denied：停止（登出由 transport 的 interceptor 處理）。
/// - resource_exhausted、not_found（還沒有店）：退避拉長到 [slowBackoff]；其他錯誤指數退避加抖動。
class NotificationSignalService {
  NotificationSignalService({
    required this.open,
    required this.onInbox,
    required this.onWorkspace,
    required this.onResync,
    math.Random? random,
    this.baseBackoff = const Duration(seconds: 1),
    this.maxBackoff = const Duration(seconds: 30),
    this.slowBackoff = const Duration(minutes: 1),
    Future<void> Function(Duration)? delay,
  }) : _random = random ?? math.Random(),
       _delay = delay ?? Future<void>.delayed;

  final Stream<WatchNotificationsResponse> Function() open;
  final void Function(InboxSignal) onInbox;
  final void Function(SellerWorkspaceChange) onWorkspace;

  /// 每次連線（含第一次）：全部重讀。
  final void Function() onResync;
  final Duration baseBackoff;
  final Duration maxBackoff;
  final Duration slowBackoff;
  final math.Random _random;
  final Future<void> Function(Duration) _delay;

  bool _running = false;
  int _generation = 0;
  StreamSubscription<WatchNotificationsResponse>? _sub;
  Completer<void>? _closed;

  bool get isRunning => _running;

  void start() {
    if (_running) return;
    _running = true;
    unawaited(_run(++_generation));
  }

  void stop() {
    _running = false;
    _generation++;
    unawaited(_sub?.cancel());
    _sub = null;
    final closed = _closed;
    if (closed != null && !closed.isCompleted) closed.complete();
  }

  Future<void> _run(int gen) async {
    var failures = 0;
    bool live() => _running && gen == _generation;
    while (live()) {
      onResync();
      var sawSignal = false;
      Object? error;
      final closed = _closed = Completer<void>();
      _sub = open().listen(
        (r) {
          if (!live()) return;
          sawSignal = true;
          failures = 0;
          switch (r.whichEvent()) {
            case WatchNotificationsResponse_Event.inbox:
              onInbox(r.inbox);
            case WatchNotificationsResponse_Event.sellerWorkspace:
              onWorkspace(r.sellerWorkspace.change);
            case WatchNotificationsResponse_Event.notSet:
              break;
          }
        },
        onError: (Object e) {
          error = e;
          if (!closed.isCompleted) closed.complete();
        },
        onDone: () {
          if (!closed.isCompleted) closed.complete();
        },
        cancelOnError: true,
      );
      await closed.future;
      if (!live()) return;
      final e = error;
      if (e != null) {
        final code = ConnectException.from(e).code;
        if (code == Code.unauthenticated || code == Code.permissionDenied) {
          HKLog.logW('[NotificationSignal] 串流停止（$code）');
          _running = false;
          return;
        }
        failures++;
        await _delay(_backoff(failures, slow: _isSlow(code)));
      } else if (!sawSignal) {
        failures++;
        await _delay(_backoff(failures, slow: false));
      }
    }
  }

  bool _isSlow(Code code) =>
      code == Code.resourceExhausted || code == Code.notFound;

  /// 慢路徑固定至少等 [slowBackoff]（再加最多 25% 的抖動）；一般路徑在
  /// `[baseBackoff, min(maxBackoff, baseBackoff * 2^(failures-1))]` 之間隨機（抖動）。
  Duration _backoff(int failures, {required bool slow}) {
    if (slow) {
      final jitter = slowBackoff.inMilliseconds ~/ 4;
      return slowBackoff +
          Duration(milliseconds: jitter == 0 ? 0 : _random.nextInt(jitter));
    }
    final exp = baseBackoff * math.pow(2, math.min(failures, 10) - 1).toInt();
    final ceiling = exp > maxBackoff ? maxBackoff : exp;
    final span = (ceiling - baseBackoff).inMilliseconds;
    return baseBackoff +
        Duration(milliseconds: span <= 0 ? 0 : _random.nextInt(span));
  }
}

@Riverpod(keepAlive: true)
NotificationSignalService notificationSignalService(Ref ref) {
  final actions = ref.read(stateRefreshActionsProvider);
  final service = NotificationSignalService(
    open: ref.read(notificationRepositoryProvider).watch,
    onInbox: (signal) => actions.applyInbox(signal.stateRefreshHints),
    onWorkspace: actions.applyWorkspaceChange,
    onResync: actions.applyAll,
  );
  ref.onDispose(service.stop);
  return service;
}
