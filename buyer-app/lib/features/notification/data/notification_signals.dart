import 'dart:async';
import 'dart:math' as math;

import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;

import '../../../shared/utils/hk_log.dart';

/// 重連的等待時間：full jitter，上限 30 秒。[attempt] 從 0 起。
Duration notificationReconnectDelay(int attempt, math.Random random) {
  final ceilingMs = math.min(1000 * (1 << math.min(attempt, 10)), 30000);
  return Duration(milliseconds: (random.nextDouble() * ceilingMs).round());
}

enum _End { ended, failed, terminal }

/// 買家的通知即時訊號（`WatchNotifications(BUYER)`）。訊號只說「該重讀了」，內容以列表為準：
/// - inbox 訊號 → [onInbox]（重讀收件匣與未讀數）與其中的 `state_refresh_hints` → [onHints]；
/// - 斷線重連後訊號可能遺漏，先呼叫 [onResync]（全部重讀）；
/// - 暫時性的失敗與伺服器端正常結束以 backoff 無限重連；`unauthenticated`（session 失效，登出由 transport 處理）
///   與 `permission_denied`（停權以外的拒絕，例如管理員）停止，不重試；
/// - 同時只有一條串流：已啟動時 [start] 不做事。登入或還原 session 時 [start]，登出時 [stop]。
class NotificationSignals {
  NotificationSignals({
    required Stream<pb.WatchNotificationsResponse> Function() open,
    required void Function(pb.InboxSignal signal) onInbox,
    required void Function(List<StateRefreshHint> hints) onHints,
    required void Function() onResync,
    math.Random? random,
    Future<void> Function(Duration delay)? wait,
  }) : _open = open,
       _onInbox = onInbox,
       _onHints = onHints,
       _onResync = onResync,
       _random = random ?? math.Random(),
       _wait = wait ?? Future<void>.delayed;

  final Stream<pb.WatchNotificationsResponse> Function() _open;
  final void Function(pb.InboxSignal signal) _onInbox;
  final void Function(List<StateRefreshHint> hints) _onHints;
  final void Function() _onResync;
  final math.Random _random;
  final Future<void> Function(Duration delay) _wait;

  int _generation = 0;
  bool _running = false;
  void Function()? _abortCurrent;

  bool get isRunning => _running;

  void start() {
    if (_running) return;
    _running = true;
    final gen = ++_generation;
    unawaited(_loop(gen));
  }

  void stop() {
    _running = false;
    _generation++;
    _abortCurrent?.call();
    _abortCurrent = null;
  }

  bool _current(int gen) => gen == _generation;

  Future<void> _loop(int gen) async {
    var attempt = 0;
    var reconnect = false;
    while (_current(gen)) {
      if (reconnect) _onResync();
      reconnect = true;
      final result = await _connectOnce(gen);
      if (!_current(gen)) return;
      if (result.end == _End.terminal) {
        _running = false;
        return;
      }
      if (result.sawSignal) attempt = 0;
      final delay = notificationReconnectDelay(attempt, _random);
      attempt++;
      await _wait(delay);
    }
  }

  Future<({_End end, bool sawSignal})> _connectOnce(int gen) {
    final done = Completer<({_End end, bool sawSignal})>();
    var sawSignal = false;
    StreamSubscription<pb.WatchNotificationsResponse>? sub;
    late final void Function() abortFn;

    void finish(_End end) {
      if (done.isCompleted) return;
      unawaited(sub?.cancel());
      if (identical(_abortCurrent, abortFn)) _abortCurrent = null;
      done.complete((end: end, sawSignal: sawSignal));
    }

    _End classify(Object error) {
      final code = ConnectException.from(error).code;
      HKLog.logD('[NotificationSignals] stream error: ${code.name}');
      return code == Code.unauthenticated || code == Code.permissionDenied
          ? _End.terminal
          : _End.failed;
    }

    abortFn = () => finish(_End.ended);
    _abortCurrent = abortFn;

    try {
      sub = _open().listen(
        (response) {
          if (!_current(gen)) return finish(_End.ended);
          if (!response.hasInbox()) return;
          sawSignal = true;
          _onInbox(response.inbox);
          if (response.inbox.stateRefreshHints.isNotEmpty) {
            _onHints(response.inbox.stateRefreshHints);
          }
        },
        onError: (Object error) => finish(classify(error)),
        onDone: () => finish(_End.ended),
        cancelOnError: true,
      );
    } catch (error) {
      finish(classify(error));
    }
    return done.future;
  }
}
