import 'dart:async';
import 'dart:math' as math;

import 'package:hawkr_api/errors.dart' show ErrorFacts;
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;

import 'chat_model.dart';

/// 串流為什麼停下來不再重連。
enum ChatStreamStopReason {
  /// 呼叫了 [ChatStreamClient.stop]（登出）。
  stopped,

  /// session 失效或帳號刪除（unauthenticated）；登出由 transport 的 interceptor 處理。
  unauthenticated,

  /// 呼叫者沒有店，或店家關閉之後沒有權限（not_found）。
  noStore,

  /// 串流進行中關店（failed_precondition 與 STORE_CLOSED）。
  storeClosed,

  /// cursors 或請求不合法（invalid_argument）；重連也會得到同一個結果。
  invalidRequest,
}

/// 串流的連線狀態。
sealed class ChatStreamStatus {
  const ChatStreamStatus();
}

final class ChatStreamConnecting extends ChatStreamStatus {
  const ChatStreamConnecting();
}

final class ChatStreamLive extends ChatStreamStatus {
  const ChatStreamLive();
}

/// 斷線等待重連。
final class ChatStreamReconnecting extends ChatStreamStatus {
  const ChatStreamReconnecting(this.attempt, this.delay);
  final int attempt;
  final Duration delay;
}

final class ChatStreamStopped extends ChatStreamStatus {
  const ChatStreamStopped(this.reason, [this.error]);
  final ChatStreamStopReason reason;
  final Object? error;
}

/// 串流上的事件，已轉成 client 形狀。
sealed class ChatStreamEvent {
  const ChatStreamEvent();
}

/// 補洞完成的第一個 frame。
final class ChatReady extends ChatStreamEvent {
  const ChatReady(this.unreadCount);
  final int unreadCount;
}

final class ChatMessageArrived extends ChatStreamEvent {
  const ChatMessageArrived(this.message, {required this.updated});
  final ChatMessage message;

  /// 為真是同一個 seq 的可見度改變，以 seq 取代手上的那則。
  final bool updated;
}

final class ChatUnreadChanged extends ChatStreamEvent {
  const ChatUnreadChanged(this.unreadCount);
  final int unreadCount;
}

final class ChatPeerRead extends ChatStreamEvent {
  const ChatPeerRead(this.conversationId, this.peerReadSeq);
  final String conversationId;
  final int peerReadSeq;
}

/// 要求重新取列表與缺口；[conversationId] 是 null 表示全部對話。
final class ChatResync extends ChatStreamEvent {
  const ChatResync(this.conversationId);
  final String? conversationId;
}

typedef ChatStreamOpener =
    Stream<pb.ChatEvent> Function(Map<String, int> cursors);

/// `StreamStoreEvents` 的 client：以每個對話最後的 seq 當 cursors 連線，斷線依原因重連。
///
/// - 伺服器以 `max_lifetime` 正常結束：立刻以最新的 seq 重連，不計失敗。
/// - 超過 `heartbeat_interval` 兩倍沒有任何 frame：視為已斷線。
/// - unavailable、其他連線錯誤：Full Jitter 的指數退避（上限 [maxBackoff]）。
/// - resource_exhausted（並行串流超過上限）：退避的下限與上限都拉長（[exhaustedBackoff]）。
/// - unauthenticated、not_found、STORE_CLOSED、invalid_argument：停止，不重連。
class ChatStreamClient {
  ChatStreamClient({
    required this.open,
    math.Random? random,
    this.baseBackoff = const Duration(seconds: 1),
    this.maxBackoff = const Duration(seconds: 30),
    this.exhaustedBackoff = const Duration(minutes: 1),
    this.initialLiveness = const Duration(seconds: 30),
    this.maxCursors = 50,
  }) : _random = random ?? math.Random();

  final ChatStreamOpener open;
  final Duration baseBackoff;
  final Duration maxBackoff;
  final Duration exhaustedBackoff;

  /// 收到 Ready 之前（還不知道心跳間隔）的等待上限。
  final Duration initialLiveness;
  final int maxCursors;
  final math.Random _random;

  final _events = StreamController<ChatStreamEvent>.broadcast();
  final _status = StreamController<ChatStreamStatus>.broadcast();
  final Map<String, int> _cursors = {};
  ChatStreamStatus _current = const ChatStreamStopped(
    ChatStreamStopReason.stopped,
  );
  bool _running = false;
  int _generation = 0;
  Completer<void>? _wake;
  StreamSubscription<pb.ChatEvent>? _sub;
  Timer? _liveness;
  Completer<_Outcome>? _attempt;

  Stream<ChatStreamEvent> get events => _events.stream;
  Stream<ChatStreamStatus> get statuses => _status.stream;
  ChatStreamStatus get status => _current;
  bool get isRunning => _running;

  /// 目前的 cursors（對話 ID 到最後的 seq）。
  Map<String, int> get cursors => Map.unmodifiable(_cursors);

  /// 記下一個對話已有的最後一個 seq（只前進）。載入歷史時也呼叫，讓重連的補洞起點不比畫面舊。
  void noteSeq(String conversationId, int seq) {
    if (seq <= 0) return;
    final prev = _cursors[conversationId] ?? 0;
    if (seq > prev) _cursors[conversationId] = seq;
  }

  void start() {
    if (_running) return;
    _running = true;
    final gen = ++_generation;
    unawaited(_run(gen));
  }

  void stop() {
    if (!_running && _current is ChatStreamStopped) return;
    _running = false;
    _generation++;
    _teardown();
    _wake?.complete();
    _setStatus(const ChatStreamStopped(ChatStreamStopReason.stopped));
  }

  /// 回前景等「立刻重連」的時機：正在等退避就跳過等待，連線看起來還活著就不動。
  void reconnectNow() {
    if (!_running) return;
    final w = _wake;
    if (w != null && !w.isCompleted) {
      w.complete();
    }
  }

  Future<void> dispose() async {
    stop();
    await _events.close();
    await _status.close();
  }

  Future<void> _run(int gen) async {
    var failures = 0;
    while (_running && gen == _generation) {
      _setStatus(const ChatStreamConnecting());
      final outcome = await _connectOnce(gen);
      if (!_running || gen != _generation) return;
      switch (outcome) {
        case _Ended(:final sawReady):
          // 伺服器正常結束（壽命到）：立刻重連；沒收到 Ready 就結束算失敗，避免緊密迴圈。
          if (sawReady) {
            failures = 0;
            continue;
          }
          failures++;
        case _Dropped(:final sawReady):
          failures = sawReady ? 1 : failures + 1;
        case _Failed(:final error):
          final stop = _terminalReason(error);
          if (stop != null) {
            _running = false;
            _setStatus(ChatStreamStopped(stop, error));
            return;
          }
          failures++;
          if (_isExhausted(error)) {
            await _waitBackoff(
              gen,
              failures,
              floor: exhaustedBackoff,
              cap: exhaustedBackoff * 2,
            );
            continue;
          }
      }
      await _waitBackoff(gen, failures, floor: Duration.zero, cap: maxBackoff);
    }
  }

  Future<void> _waitBackoff(
    int gen,
    int failures, {
    required Duration floor,
    required Duration cap,
  }) async {
    final exp = baseBackoff.inMilliseconds * math.pow(2, failures - 1);
    final floorMs = floor.inMilliseconds.toDouble();
    final ceiling = math.max(
      floorMs,
      math.min(exp, cap.inMilliseconds.toDouble()),
    );
    // Full Jitter：在 [0, ceiling] 內均勻取值；有下限時落在 [floor, ceiling]。
    final ms = floorMs + _random.nextDouble() * (ceiling - floorMs);
    final delay = Duration(milliseconds: ms.round());
    _setStatus(ChatStreamReconnecting(failures, delay));
    final wake = _wake = Completer<void>();
    final timer = Timer(delay, () {
      if (!wake.isCompleted) wake.complete();
    });
    await wake.future;
    timer.cancel();
    _wake = null;
  }

  Future<_Outcome> _connectOnce(int gen) async {
    final attempt = _attempt = Completer<_Outcome>();
    var sawReady = false;
    var liveFor = initialLiveness;
    final snapshot = _snapshotCursors();

    void armLiveness(Duration d) {
      _liveness?.cancel();
      _liveness = Timer(d, () {
        if (!attempt.isCompleted) attempt.complete(_Dropped(sawReady));
      });
    }

    armLiveness(initialLiveness);
    try {
      _sub = open(snapshot).listen(
        (frame) {
          if (gen != _generation) return;
          final e = frame;
          if (e.hasReady()) {
            sawReady = true;
            final hb = e.ready.heartbeatInterval;
            final d = Duration(
              seconds: hb.seconds.toInt(),
              microseconds: hb.nanos ~/ 1000,
            );
            liveFor = d > Duration.zero ? d * 2 : initialLiveness;
            armLiveness(liveFor);
            _setStatus(const ChatStreamLive());
            _events.add(ChatReady(e.ready.unreadCount.toInt()));
            return;
          }
          // 任何 frame 都重置存活計時：用 Ready 的間隔。
          armLiveness(liveFor);
          _dispatch(e);
        },
        onError: (Object err) {
          if (!attempt.isCompleted) attempt.complete(_Failed(err));
        },
        onDone: () {
          if (!attempt.isCompleted) attempt.complete(_Ended(sawReady));
        },
        cancelOnError: true,
      );
    } on Object catch (err) {
      if (!attempt.isCompleted) attempt.complete(_Failed(err));
    }
    final result = await attempt.future;
    _teardown();
    return result;
  }

  Map<String, int> _snapshotCursors() {
    if (_cursors.length <= maxCursors) return Map.of(_cursors);
    // 超過上限只送最近有動靜的（seq 最大者無法代表最近，保守取 map 尾端的）。
    final entries = _cursors.entries.toList();
    return {
      for (final e in entries.sublist(entries.length - maxCursors))
        e.key: e.value,
    };
  }

  void _dispatch(pb.ChatEvent e) {
    if (e.hasMessage()) {
      final m = ChatMessage.fromProto(e.message.message);
      noteSeq(m.conversationId, m.seq);
      _events.add(ChatMessageArrived(m, updated: e.message.updated));
    } else if (e.hasUnread()) {
      _events.add(ChatUnreadChanged(e.unread.unreadCount.toInt()));
    } else if (e.hasPeerRead()) {
      _events.add(
        ChatPeerRead(e.peerRead.conversationId, e.peerRead.peerReadSeq.toInt()),
      );
    } else if (e.hasResync()) {
      final r = e.resync;
      _events.add(ChatResync(r.hasConversationId() ? r.conversationId : null));
    }
    // Heartbeat：只用來重置存活計時。
  }

  void _teardown() {
    _liveness?.cancel();
    _liveness = null;
    final sub = _sub;
    _sub = null;
    if (sub != null) unawaited(sub.cancel());
    final a = _attempt;
    _attempt = null;
    if (a != null && !a.isCompleted) a.complete(const _Ended(false));
  }

  void _setStatus(ChatStreamStatus s) {
    _current = s;
    if (!_status.isClosed) _status.add(s);
  }

  static ChatStreamStopReason? _terminalReason(Object error) {
    final e = ConnectException.from(error);
    switch (e.code) {
      case Code.unauthenticated:
        return ChatStreamStopReason.unauthenticated;
      case Code.notFound:
        return ChatStreamStopReason.noStore;
      case Code.invalidArgument:
        return ChatStreamStopReason.invalidRequest;
      case Code.failedPrecondition:
        final reason = ErrorFacts.fromException(e).reason;
        if (reason == ErrorReason.STORE_CLOSED.name) {
          return ChatStreamStopReason.storeClosed;
        }
        return null;
      default:
        return null;
    }
  }

  static bool _isExhausted(Object error) =>
      ConnectException.from(error).code == Code.resourceExhausted;
}

sealed class _Outcome {
  const _Outcome();
}

final class _Ended extends _Outcome {
  const _Ended(this.sawReady);
  final bool sawReady;
}

final class _Dropped extends _Outcome {
  const _Dropped(this.sawReady);
  final bool sawReady;
}

final class _Failed extends _Outcome {
  const _Failed(this.error);
  final Object error;
}
