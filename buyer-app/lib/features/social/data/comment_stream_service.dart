import 'dart:convert';

import 'package:flutter/foundation.dart';

import '../../../core/sse/sse_client.dart';

/// 留言 SSE 業務層。
///
/// **此檔為 buyer / seller 共用模板,變更請同步 `hawkr_seller` 中同路徑檔案。**
///
/// 設計目標:
/// - 接 `postId` + `apiBaseUrl` 後組裝 public SSE 路徑、開啟連線
/// - 維護 `lastEventId` per-post in-memory cache(5min TTL),切換 post 後再回來能補帶
/// - `ready` 事件 `bufferMiss=true` 透過 [onBufferMiss] 通知 caller 觸發一次性 refresh
/// - 純模板:不讀 session、不知道 role / accountId / X-User-Id
class CommentStreamService {
  CommentStreamService({
    required this.apiBaseUrl,
    required this.postId,
    required this.onCommentEvent,
    this.onBufferMiss,
    this.onConnectionDegraded,
    this.onConnectionRecovered,
  });

  /// Gateway 公開 API base,通常為 `${flavor.gatewayHost}/api/v1`
  final String apiBaseUrl;
  final int postId;

  /// 收到留言領域事件時觸發。
  ///
  /// - [eventName] 為 `comment.created` / `comment.replied` / `comment.updated` /
  ///   `comment.deleted` / `comment.like_changed`(`comment.batch` 已由本層攤平)
  /// - [data] 為事件 payload 解析後的 Map
  final void Function(String eventName, Map<String, dynamic> data)
  onCommentEvent;

  /// `ready` 事件帶 `bufferMiss=true` 時觸發,caller 應觸發一次性 refresh。
  final void Function()? onBufferMiss;

  /// 持續 30s 連不上時觸發(僅一次,直到再次成功才 reset)。
  final void Function()? onConnectionDegraded;

  /// 重新連上時觸發。
  final void Function()? onConnectionRecovered;

  SseClient? _client;

  /// per-app 程序內共享 cache。同 session 內切離 post 又回來能延續 stream 位置。
  static final Map<int, _LastIdEntry> _lastEventIdCache = <int, _LastIdEntry>{};
  static const Duration _ttl = Duration(minutes: 5);

  static String? _getCachedLastId(int postId) {
    final entry = _lastEventIdCache[postId];
    if (entry == null) return null;
    if (DateTime.now().isAfter(entry.expireAt)) {
      _lastEventIdCache.remove(postId);
      return null;
    }
    return entry.id;
  }

  static void _setCachedLastId(int postId, String id) {
    _lastEventIdCache[postId] = _LastIdEntry(id, DateTime.now().add(_ttl));
  }

  void start() {
    debugPrint(
      '[SSE-DEBUG] CommentStreamService.start postId=$postId apiBaseUrl=$apiBaseUrl',
    );
    if (_client != null) return; // 已啟動
    final url = '$apiBaseUrl/public/posts/$postId/comments/stream';
    debugPrint('[SSE-DEBUG] CommentStreamService url=$url');
    _client = SseClient(
      url: url,
      initialLastEventId: _getCachedLastId(postId),
      onEvent: _handleEvent,
      onOpen: () {
        debugPrint('[CommentStream:$postId] connected');
        onConnectionRecovered?.call();
      },
      onError: (err) {
        if (kDebugMode) debugPrint('[CommentStream:$postId] error: $err');
      },
      onConnectionDegraded: () {
        if (kDebugMode) {
          debugPrint('[CommentStream:$postId] degraded — 30s without connect');
        }
        onConnectionDegraded?.call();
      },
    );
    _client!.connect();
  }

  void _handleEvent(SseEvent ev) {
    final eventName = ev.event ?? '';
    if (eventName.isEmpty) return;

    // 寫回 cache(SSE id 由 sse_client 自身追蹤;這裡額外存到 per-post cache)
    final id = ev.id;
    if (id != null && id.isNotEmpty) {
      _setCachedLastId(postId, id);
    }

    if (eventName == 'heartbeat') return;

    final payload = _decodePayload(ev.data);

    if (eventName == 'ready') {
      final bufferMiss = payload['bufferMiss'] == true;
      if (kDebugMode) {
        debugPrint(
          '[CommentStream:$postId] ready bufferMiss=$bufferMiss '
          'replayed=${payload['replayed']}',
        );
      }
      if (bufferMiss) onBufferMiss?.call();
      return;
    }

    if (eventName == 'comment.batch') {
      final events = payload['events'];
      if (events is List) {
        for (final e in events) {
          if (e is! Map) continue;
          final em = Map<String, dynamic>.from(e);
          final innerName = (em['event'] ?? '').toString();
          final innerRaw = em['data'];
          if (innerName.isEmpty || innerRaw == null) continue;
          final innerMap = _coerceMap(innerRaw);
          if (innerMap == null) continue;
          onCommentEvent(innerName, innerMap);
        }
      }
      return;
    }

    onCommentEvent(eventName, payload);
  }

  Map<String, dynamic> _decodePayload(String? raw) {
    if (raw == null || raw.isEmpty) return const <String, dynamic>{};
    try {
      final decoded = jsonDecode(raw);
      final coerced = _coerceMap(decoded);
      return coerced ?? const <String, dynamic>{};
    } catch (e) {
      if (kDebugMode) {
        debugPrint('[CommentStream:$postId] payload parse error: $e raw=$raw');
      }
      return const <String, dynamic>{};
    }
  }

  Map<String, dynamic>? _coerceMap(dynamic value) {
    if (value is Map<String, dynamic>) return value;
    if (value is Map) return Map<String, dynamic>.from(value);
    return null;
  }

  void dispose() {
    _client?.dispose();
    _client = null;
  }
}

class _LastIdEntry {
  _LastIdEntry(this.id, this.expireAt);
  final String id;
  final DateTime expireAt;
}
