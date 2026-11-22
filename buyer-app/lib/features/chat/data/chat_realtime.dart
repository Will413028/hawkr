import 'dart:async';
import 'dart:math' as math;

import 'package:fixnum/fixnum.dart';
import 'package:flutter/widgets.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:rxdart/rxdart.dart';

import '../../../shared/utils/hk_log.dart';
import 'chat_repository.dart';

part 'chat_realtime.g.dart';

enum ChatConnectionState { disconnected, connecting, connected, reconnecting }

@Riverpod(keepAlive: true)
ChatRealtime chatRealtime(Ref ref) {
  final realtime = ChatRealtime(ref.watch(chatRepositoryProvider));
  ref.onDispose(realtime.dispose);
  return realtime;
}

/// 重連的等待時間：full jitter，上限 30 秒。[attempt] 從 0 起。
Duration chatReconnectDelay(int attempt, math.Random random) {
  final ceilingMs = math.min(1000 * (1 << math.min(attempt, 10)), 30000);
  return Duration(milliseconds: (random.nextDouble() * ceilingMs).round());
}

enum _End { ended, failed, terminal }

/// 聊天的即時事件（`ChatAppService.StreamEvents`）。畫面進出以 [acquire]／[release] 計數，第一個進來時連線，
/// 最後一個離開時斷線。串流活到伺服器的 `max_lifetime` 正常結束就以最新的 cursors 立即重連；暫時性的結束
/// （unavailable、resource_exhausted、網路中斷、超過 heartbeat 兩倍沒有任何 frame）以 backoff 無限重連；
/// 只有 unauthenticated（session 失效，由 transport 處理登出）與 invalid_argument（client 的 bug）停止。
class ChatRealtime with WidgetsBindingObserver {
  ChatRealtime(
    this._repo, {
    math.Random? random,
    Duration Function(int attempt)? reconnectDelay,
    Duration Function(Duration heartbeat)? livenessWindow,
  }) : _random = random ?? math.Random(),
       _reconnectDelay = reconnectDelay,
       _livenessWindow = livenessWindow ?? ((hb) => hb * 2) {
    WidgetsBinding.instance.addObserver(this);
  }

  final ChatRepository _repo;
  final math.Random _random;
  final Duration Function(int attempt)? _reconnectDelay;
  final Duration Function(Duration heartbeat) _livenessWindow;

  static const _readyTimeout = Duration(seconds: 60);

  final _events = StreamController<pb.ChatEvent>.broadcast();
  final _state = BehaviorSubject<ChatConnectionState>.seeded(
    ChatConnectionState.disconnected,
  );
  final _unread = BehaviorSubject<int>();

  /// 串流收到的 `Ready`、`Message`、`PeerRead`、`Resync`（`Unread` 走 [unreadCount]，`Heartbeat` 不外送）。
  Stream<pb.ChatEvent> get events => _events.stream;

  Stream<ChatConnectionState> get connectionState => _state.stream;
  ChatConnectionState get currentState => _state.value;

  /// 總未讀數：`Ready` 與 `Unread` 事件的值，晚訂閱者拿到最後一個。
  Stream<int> get unreadCount => _unread.stream;

  /// 每個對話已收到的最後一個 seq；最近更新的在最後。
  final _cursors = <String, Int64>{};

  int _refCount = 0;
  int _generation = 0;
  void Function()? _abortCurrent;
  Timer? _backoffTimer;
  Completer<void>? _backoffWake;
  bool _disposed = false;

  bool get isActive => _refCount > 0;

  /// 記下 [conversationId] 已有的最後一個 seq（只前進）；下次連線的 `cursors` 帶它。
  void noteSeq(String conversationId, Int64 seq) {
    final current = _cursors[conversationId];
    if (current != null && current >= seq) return;
    _cursors.remove(conversationId);
    _cursors[conversationId] = seq;
  }

  /// 這次連線送的 cursors：最近更新的最多 [ChatRepository.maxCursors] 個。
  Map<String, Int64> requestCursors() {
    final entries = _cursors.entries.toList();
    final start = math.max(0, entries.length - ChatRepository.maxCursors);
    return {for (final e in entries.skip(start)) e.key: e.value};
  }

  void acquire() {
    _refCount++;
    if (_refCount == 1) _start();
  }

  void release() {
    if (_refCount == 0) return;
    _refCount--;
    if (_refCount == 0) _stop();
  }

  void _start() {
    final gen = ++_generation;
    _setState(ChatConnectionState.connecting);
    unawaited(_loop(gen));
  }

  void _stop() {
    _generation++;
    _abortCurrent?.call();
    _abortCurrent = null;
    _backoffTimer?.cancel();
    _backoffTimer = null;
    _backoffWake?.complete();
    _backoffWake = null;
    _setState(ChatConnectionState.disconnected);
  }

  void dispose() {
    if (_disposed) return;
    _disposed = true;
    _stop();
    WidgetsBinding.instance.removeObserver(this);
    _events.close();
    _state.close();
    _unread.close();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed || !isActive) return;
    if (currentState == ChatConnectionState.connected) return;
    // 回前景時不等 backoff，立即重連。
    final wake = _backoffWake;
    if (wake != null && !wake.isCompleted) wake.complete();
  }

  void _setState(ChatConnectionState s) {
    if (_disposed || _state.value == s) return;
    _state.add(s);
  }

  bool _current(int gen) => !_disposed && gen == _generation;

  Future<void> _loop(int gen) async {
    var attempt = 0;
    while (_current(gen)) {
      final result = await _connectOnce(gen);
      if (!_current(gen)) return;
      switch (result.end) {
        case _End.terminal:
          _setState(ChatConnectionState.disconnected);
          return;
        case _End.ended:
          attempt = 0;
          _setState(ChatConnectionState.reconnecting);
        case _End.failed:
          if (result.sawReady) attempt = 0;
          _setState(ChatConnectionState.reconnecting);
          final delay =
              _reconnectDelay?.call(attempt) ??
              chatReconnectDelay(attempt, _random);
          attempt++;
          await _wait(delay);
      }
    }
  }

  Future<void> _wait(Duration delay) {
    final wake = Completer<void>();
    _backoffWake = wake;
    _backoffTimer = Timer(delay, () {
      if (!wake.isCompleted) wake.complete();
    });
    return wake.future.whenComplete(() {
      _backoffTimer?.cancel();
      _backoffTimer = null;
      if (identical(_backoffWake, wake)) _backoffWake = null;
    });
  }

  Future<({_End end, bool sawReady})> _connectOnce(int gen) {
    final done = Completer<({_End end, bool sawReady})>();
    var sawReady = false;
    Duration? heartbeat;
    Timer? liveness;
    StreamSubscription<pb.ChatEvent>? sub;
    late final void Function() abortFn;

    void finish(_End end) {
      if (done.isCompleted) return;
      liveness?.cancel();
      unawaited(sub?.cancel());
      if (identical(_abortCurrent, abortFn)) _abortCurrent = null;
      done.complete((end: end, sawReady: sawReady));
    }

    void arm() {
      liveness?.cancel();
      final window = heartbeat == null
          ? _readyTimeout
          : _livenessWindow(heartbeat!);
      liveness = Timer(window, () => finish(_End.failed));
    }

    abortFn = () => finish(_End.ended);
    _abortCurrent = abortFn;
    arm();

    try {
      sub = _repo
          .streamEvents(requestCursors())
          .listen(
            (event) {
              if (!_current(gen)) return finish(_End.ended);
              arm();
              switch (event.whichEvent()) {
                case pb.ChatEvent_Event.ready:
                  sawReady = true;
                  heartbeat = event.ready.hasHeartbeatInterval()
                      ? Duration(
                          seconds: event.ready.heartbeatInterval.seconds
                              .toInt(),
                          microseconds:
                              event.ready.heartbeatInterval.nanos ~/ 1000,
                        )
                      : null;
                  arm();
                  _setState(ChatConnectionState.connected);
                  _unread.add(event.ready.unreadCount.toInt());
                  _events.add(event);
                case pb.ChatEvent_Event.message:
                  noteSeq(
                    event.message.message.conversationId,
                    event.message.message.seq,
                  );
                  _events.add(event);
                case pb.ChatEvent_Event.unread:
                  _unread.add(event.unread.unreadCount.toInt());
                case pb.ChatEvent_Event.peerRead:
                case pb.ChatEvent_Event.resync:
                  _events.add(event);
                case pb.ChatEvent_Event.heartbeat:
                case pb.ChatEvent_Event.notSet:
                  break;
              }
            },
            onError: (Object error) {
              final code = ConnectException.from(error).code;
              HKLog.logD('[ChatRealtime] stream error: ${code.name}');
              finish(
                code == Code.unauthenticated || code == Code.invalidArgument
                    ? _End.terminal
                    : _End.failed,
              );
            },
            onDone: () => finish(_End.ended),
            cancelOnError: true,
          );
    } catch (error) {
      final code = ConnectException.from(error).code;
      finish(
        code == Code.unauthenticated || code == Code.invalidArgument
            ? _End.terminal
            : _End.failed,
      );
    }
    return done.future;
  }
}
