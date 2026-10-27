import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_client_sse/constants/sse_request_type_enum.dart';
import 'package:flutter_client_sse/flutter_client_sse.dart';

import '../networking/app_version_headers.dart';

/// 通用 SSE Client。
///
/// **此檔為 buyer / seller 共用模板,變更請同步 `hawkr_seller` 中同路徑檔案。**
///
/// 設計目標:
/// - 廢用 `flutter_client_sse` 內建固定 5s 重連 → 自做指數退避(2/4/8/16/30s 上限)
/// - 自動於重連時注入 `Last-Event-ID`,從上一筆事件 id 取得
/// - `dispose()` 釋放 listener、停止套件內建重連、取消 timer
/// - 不知道任何業務概念(post / role / accountId),純粹 URL + headers + callback
///
/// 已知限制(來自 `flutter_client_sse`):
/// - 內部使用 `static http.Client`,**整個 app 同時只能 1 條 SSE**
/// - 如未來需要同時多條 stream,須改換 package
class SseClient {
  SseClient({
    required this.url,
    this.extraHeaders = const {},
    required this.onEvent,
    this.onOpen,
    this.onError,
    this.onConnectionDegraded,
    this.initialLastEventId,
    this.maxBackoffSeconds = 30,
    this.degradedAfter = const Duration(seconds: 30),
  });

  final String url;
  final Map<String, String> extraHeaders;
  final void Function(SseEvent event) onEvent;
  final void Function()? onOpen;
  final void Function(Object error)? onError;

  /// 持續無法連上(超過 [degradedAfter])時觸發,僅觸發一次,直到下次成功連上才會 reset。
  final void Function()? onConnectionDegraded;

  final String? initialLastEventId;
  final int maxBackoffSeconds;
  final Duration degradedAfter;

  StreamSubscription<SSEModel>? _sub;
  Timer? _reconnectTimer;
  Timer? _degradedTimer;
  String? _lastEventId;
  int _retryAttempt = 0;
  bool _disposed = false;
  bool _degradedFired = false;

  String? get lastEventId => _lastEventId;

  Future<void> connect() async {
    if (_disposed) return;
    _lastEventId ??= initialLastEventId;
    _subscribe();
  }

  void _subscribe() {
    if (_disposed) return;
    // X-App-* 三欄供 Gateway access log 統計版本佔比；SSE 只在握手記一筆
    // access log，之後的訊息不進 log（54 號設計書 §6.5 / §8.2）。
    final headers = <String, String>{
      'Accept': 'text/event-stream',
      ...AppVersionHeaders.headers,
      ...extraHeaders,
    };
    if (_lastEventId != null && _lastEventId!.isNotEmpty) {
      headers['Last-Event-ID'] = _lastEventId!;
    }

    debugPrint('[SSE-DEBUG] SseClient._subscribe url=$url headers=$headers');
    try {
      final stream = SSEClient.subscribeToSSE(
        method: SSERequestType.GET,
        url: url,
        header: headers,
      );
      debugPrint('[SSE-DEBUG] SseClient stream obtained, attaching listener');

      _sub = stream.listen(
        (model) {
          debugPrint(
            '[SSE-DEBUG] event id=${model.id} event=${model.event} dataLen=${model.data?.length}',
          );
          if (_disposed) return;
          _onSuccess();
          if (model.id != null && model.id!.isNotEmpty) {
            _lastEventId = model.id;
          }
          onEvent(SseEvent(id: model.id, event: model.event, data: model.data));
        },
        onError: (err) {
          debugPrint('[SSE-DEBUG] stream onError err=$err');
          if (_disposed) return;
          onError?.call(err);
          _scheduleReconnect();
        },
        onDone: () {
          debugPrint('[SSE-DEBUG] stream onDone');
          if (_disposed) return;
          _scheduleReconnect();
        },
        cancelOnError: false,
      );
    } catch (e, st) {
      debugPrint('[SSE-DEBUG] subscribeToSSE threw: $e\n$st');
      _scheduleReconnect();
    }
  }

  void _onSuccess() {
    onOpen?.call();
    _retryAttempt = 0;
    _degradedTimer?.cancel();
    _degradedTimer = null;
    _degradedFired = false;
  }

  void _scheduleReconnect() {
    if (_disposed) return;
    if (_reconnectTimer != null) return; // 已排程,避免重複觸發

    _sub?.cancel();
    _sub = null;
    // 強制關閉套件內建的 5s 重連 loop,改由本 wrapper 自己重連
    SSEClient.unsubscribeFromSSE();

    if (_degradedTimer == null && !_degradedFired) {
      _degradedTimer = Timer(degradedAfter, () {
        if (_disposed) return;
        _degradedFired = true;
        onConnectionDegraded?.call();
      });
    }

    final delay = _computeBackoff();
    _reconnectTimer = Timer(delay, () {
      _reconnectTimer = null;
      _retryAttempt++;
      _subscribe();
    });
  }

  Duration _computeBackoff() {
    // 2, 2, 4, 8, 16, 30, 30, ... (clamp 上限)
    final raw = 1 << _retryAttempt;
    final secs = raw.clamp(2, maxBackoffSeconds);
    return Duration(seconds: secs);
  }

  void dispose() {
    if (_disposed) return;
    _disposed = true;
    _sub?.cancel();
    _sub = null;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    _degradedTimer?.cancel();
    _degradedTimer = null;
    SSEClient.unsubscribeFromSSE();
    if (kDebugMode) {
      debugPrint('[SseClient] disposed url=$url');
    }
  }
}

class SseEvent {
  SseEvent({this.id, this.event, this.data});
  final String? id;
  final String? event;
  final String? data;
}
