import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;

import 'package:dio/dio.dart';
import 'package:flutter/widgets.dart';
import 'package:rxdart/rxdart.dart';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../config/flavor.dart';
import '../../../core/networking/app_version_headers.dart';
import '../../../core/session/auth_session_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'chat_model.dart';
import '../../moderation/data/report_block_codes.dart';

part 'chat_repository.g.dart';

/// WebSocket 連線狀態
enum ChatConnectionState {
  disconnected, // 未連線
  connecting, // 連線中
  connected, // 已連線（WebSocket）
  reconnecting, // 重連中
  fallbackPolling, // 降級為 HTTP 輪詢
}

/// 對方在線狀態推送事件（WebSocket type=presence）
class PresenceEvent {
  final int peerId;
  final int peerRole; // 1=buyer, 2=seller
  final bool online;
  final int lastSeenAtMs; // 0 = 未知（online 時不帶）

  const PresenceEvent({
    required this.peerId,
    required this.peerRole,
    required this.online,
    required this.lastSeenAtMs,
  });
}

class ChatRepository with WidgetsBindingObserver {
  ChatRepository._(this._sessionToken, this._accountId, {Dio? dio})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              validateStatus: (_) => true,
              headers: {...AppVersionHeaders.headers},
            ),
          ) {
    WidgetsBinding.instance.addObserver(this);
  }

  /// 測試用：注入 [dio] 以檢查送出的 header。
  @visibleForTesting
  ChatRepository.withDio({
    required Future<String?> Function() sessionToken,
    required String? Function() accountId,
    required Dio dio,
  }) : this._(sessionToken, accountId, dio: dio);

  /// 目前的 session token；訪客回 null。
  final Future<String?> Function() _sessionToken;
  final String? Function() _accountId;

  // X-App-* 三欄供 Gateway access log 統計版本佔比（`packages/hawkr_app_update/docs/spec.md`）。
  final Dio _dio;

  // ============================================================
  // 訊息去重 (Deduplication via roomId + seqId)
  // ============================================================

  final Set<String> _seenMessages = {};

  /// 檢查是否為重複訊息。回傳 true 表示已經收過，應跳過。
  bool _isDuplicate(ChatMessage msg) {
    if (msg.conversationId == null || msg.conversationId!.isEmpty) return false;
    if (msg.seqId <= 0) return false; // 沒有 seqId 無法去重
    // 防止無限增長
    if (_seenMessages.length > 5000) {
      _seenMessages.clear();
    }
    final key = '${msg.conversationId}_${msg.seqId}';
    return !_seenMessages.add(key);
  }

  // ============================================================
  // WebSocket 連線管理
  // ============================================================

  WebSocket? _socket;
  StreamSubscription? _socketSub;
  int _connEpoch = 0; // socket 建立後 +1；辨識並忽略被取代的舊連線遲來的回呼
  int _connectGen = 0; // 連線「嘗試」代數；await 期間被取代/disconnect 就丟棄
  Timer? _reconnectTimer;
  Timer? _pingTimer;
  Timer? _livenessTimer;
  Timer? _pollTimer;
  int _reconnectAttempts = 0;
  String? _currentUserId; // 連線時快取；讓 ping 送出維持同步、不需 async getter
  static const int _maxReconnectAttempts = 5;
  // ponytail: carrier NAT (~30s idle) 的保守預設。心跳間隔與 2× liveness 視窗
  // 應在量測 CF/edge idle 後再調（IM 設計書 D6 / §11.2）——量測前別硬調更低。
  static const Duration _pingInterval = Duration(seconds: 15);
  static const Duration _livenessTimeout = Duration(seconds: 30);

  ChatConnectionState _connectionState = ChatConnectionState.disconnected;
  // BehaviorSubject.seeded 會把當前狀態 replay 給晚訂閱者（§6.6 emit-on-subscribe）——
  // 修掉那類「晚掛載的 widget（如未讀 badge）卡在初始 disconnected 而誤報
  // 網路連線異常」的 bug；純 broadcast Stream 不 replay last value。
  final BehaviorSubject<ChatConnectionState> _connectionStateController =
      BehaviorSubject<ChatConnectionState>.seeded(
        ChatConnectionState.disconnected,
      );
  Stream<ChatConnectionState> get connectionStateStream =>
      _connectionStateController.stream;
  ChatConnectionState get connectionState => _connectionState;

  // ============================================================
  // 訊息 Streams
  // ============================================================

  final StreamController<ChatMessage> _messageController =
      StreamController<ChatMessage>.broadcast();

  /// 即時收到的 WebSocket 訊息
  Stream<ChatMessage> get onMessage => _messageController.stream;

  final StreamController<ChatNotificationEvent> _newMessageController =
      StreamController<ChatNotificationEvent>.broadcast();

  /// 新訊息通知事件（全域，含寄件者資訊與預覽）
  Stream<ChatNotificationEvent> get onNewMessage =>
      _newMessageController.stream;

  final StreamController<PresenceEvent> _presenceController =
      StreamController<PresenceEvent>.broadcast();

  /// 對方在線狀態推送（取代 30 秒輪詢 /chat/online）
  Stream<PresenceEvent> get onPresence => _presenceController.stream;

  final BehaviorSubject<int> _unreadCountController =
      BehaviorSubject<int>.seeded(0);

  /// 後端推送的全域未讀數（unread_changed 事件）
  Stream<int> get onUnreadCountChange => _unreadCountController.stream;

  int _consecutive404 = 0;

  // ============================================================
  // Reference Counting 連線控制
  // ============================================================

  int _chatScreenRefCount = 0;

  /// 進入聊天畫面時呼叫，refCount +1，首次進入建立 WebSocket
  Future<void> connectForChat() async {
    _chatScreenRefCount++;
    HKLog.logD(
      '[ChatRepository] connectForChat: refCount=$_chatScreenRefCount',
    );
    if (_chatScreenRefCount == 1) {
      await connect();
    }
  }

  /// 離開聊天畫面時呼叫，refCount -1，最後一個畫面離開時斷線
  Future<void> disconnectFromChat() async {
    _chatScreenRefCount--;
    if (_chatScreenRefCount < 0) _chatScreenRefCount = 0;
    HKLog.logD(
      '[ChatRepository] disconnectFromChat: refCount=$_chatScreenRefCount',
    );
    if (_chatScreenRefCount == 0) {
      await disconnect();
    }
  }

  bool get hasChatScreenActive => _chatScreenRefCount > 0;

  Future<void> connect() async {
    if (_connectionState == ChatConnectionState.connected ||
        _connectionState == ChatConnectionState.connecting) {
      HKLog.logD('[ChatRepository] Already connected or connecting, skipping');
      return;
    }
    _setConnectionState(ChatConnectionState.connecting);
    await _connectWebSocket();
  }

  Future<void> disconnect() async {
    HKLog.logD('[ChatRepository] Disconnecting...');
    _chatScreenRefCount = 0;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    _pollTimer?.cancel();
    _pollTimer = null;
    _reconnectAttempts = 0;
    _seenMessages.clear();
    _teardownSocket();
    _setConnectionState(ChatConnectionState.disconnected);
  }

  /// Provider teardown — called by ref.onDispose when container is disposed.
  void dispose() {
    _reconnectTimer?.cancel();
    _pollTimer?.cancel();
    _chatScreenRefCount = 0;
    _reconnectAttempts = 0;
    _seenMessages.clear();
    _teardownSocket(); // sync: cancels subscription, closes WebSocket
    _messageController.close();
    _newMessageController.close();
    _presenceController.close();
    _connectionStateController.close();
    _unreadCountController.close();
    WidgetsBinding.instance.removeObserver(this);
  }

  void _setConnectionState(ChatConnectionState state) {
    if (_connectionState == state) return;
    _connectionState = state;
    _connectionStateController.add(state);
    HKLog.logD('[ChatRepository] Connection state: ${state.name}');
  }

  // ============================================================
  // WebSocket 連線核心
  // ============================================================

  Future<void> _connectWebSocket() async {
    final gen = ++_connectGen; // 認領這次連線嘗試

    // ws-ticket：authed POST 鑄單次短效票，WS URL 只帶 ?ticket=（不帶長命的 session token /
    // userId）。與 web（lib/chat/ws.ts buildWsUrl）對齊；終止層 GETDEL 消費後推導身份。
    _WsTicket? ticket;
    try {
      ticket = await _mintWsTicket();
    } catch (e) {
      // 暫時失敗（網路 / 5xx）→ 當作一次連線嘗試走重連 backoff，別直接死掉。
      HKLog.logD('[ChatRepository] ws-ticket mint failed (transient): $e');
      _handleConnectionFailure();
      return;
    }
    if (ticket == null) {
      // 持久失敗（未登入 / 401）→ 停止重連、清連線。
      HKLog.logD('[ChatRepository] ws-ticket unauthorized, not connecting');
      _setConnectionState(ChatConnectionState.disconnected);
      return;
    }
    _currentUserId = ticket.userId;

    try {
      final wsUrl =
          '$_wsBase?ticket=${Uri.encodeQueryComponent(ticket.ticket)}';
      HKLog.logD('[ChatRepository] Connecting to WebSocket (ws-ticket)');

      // 長連線握手同樣帶 X-App-*（只算 Dio 會漏；`packages/hawkr_app_update/docs/spec.md`）。
      final socket = await WebSocket.connect(
        wsUrl,
        headers: {...AppVersionHeaders.headers},
      ).timeout(const Duration(seconds: 10));

      // await 期間若被 disconnect() 或更新的 connect 取代，丟棄這個 socket，
      // 否則會留下沒人管的 zombie / leaked 連線（connect-boundary race）。
      if (gen != _connectGen ||
          _connectionState == ChatConnectionState.disconnected) {
        await socket.close();
        return;
      }
      _socket = socket;
      final epoch = ++_connEpoch;

      HKLog.logD('[ChatRepository] WebSocket connected successfully');

      // epoch 綁進 callback：被取代的舊 socket 遲來的 onDone/frame 會因 epoch
      // 不符被忽略，不會誤砍重連後的健康 socket（half-open orphan bug）。
      _socketSub = socket.listen(
        (data) => _onWebSocketMessage(epoch, data),
        onDone: () => _onWebSocketDisconnect(epoch),
        onError: (e) => _onWebSocketError(epoch, e),
        cancelOnError: false,
      );

      _reconnectAttempts = 0;
      _setConnectionState(ChatConnectionState.connected);
      _startHeartbeat();
    } on TimeoutException catch (_) {
      HKLog.logD('[ChatRepository] WebSocket connection timeout');
      _handleConnectionFailure();
    } catch (e) {
      HKLog.logD('[ChatRepository] WebSocket connection failed: $e');
      _handleConnectionFailure();
    }
  }

  void _onWebSocketMessage(int epoch, dynamic data) {
    if (epoch != _connEpoch) return; // 舊連線遲來的幀不該重設新連線的 liveness
    // 任何入站幀都是 liveness 訊號 → 重設讀取側期限（§6.3）。
    _armLiveness();
    try {
      final decoded = json.decode(data.toString());
      if (decoded is! Map<String, dynamic>) return;

      final type = decoded['type']?.toString() ?? 'text';
      HKLog.logD('[ChatRepository] Received WS message type: $type');

      switch (type) {
        case 'typing':
        case 'start_typing':
        case 'stop_typing':
          // 轉發給 UI 層（chat_thread_screen 偵測 senderId 顯示打字指示器）
          _messageController.add(
            ChatMessage(
              id: '',
              senderId: (decoded['from'] ?? decoded['fromUserId'] ?? '')
                  .toString(),
              receiverId: '',
              messageContent: '',
              messageType: type,
              timestamp: DateTime.now(),
            ),
          );
          break;
        case 'read_receipt':
          HKLog.logD('[ChatRepository] Read receipt from=${decoded['from']}');
          _messageController.add(
            ChatMessage(
              id: (decoded['messageId'] ?? '').toString(),
              senderId: (decoded['from'] ?? decoded['fromUserId'] ?? '')
                  .toString(),
              receiverId: (decoded['to'] ?? '').toString(),
              messageContent: '',
              messageType: 'read_receipt',
              timestamp: DateTime.now(),
            ),
          );
          break;
        case 'pong':
        case 'heartbeat':
          // Liveness ack — 期限已於進入時重設，這裡不需再做事。
          break;
        case 'unread_changed':
          // 後端推送的權威未讀數(任一裝置已讀/收到新訊息後)。一律 SET。
          final count =
              int.tryParse(
                (decoded['unreadCount'] ?? decoded['count'] ?? '').toString(),
              ) ??
              0;
          _unreadCountController.add(count);
          break;
        case 'chat_ack':
          // 處理 server 確認回覆
          final msg = _parseWebSocketMessage(decoded);
          if (msg != null) _messageController.add(msg);
          break;
        case 'presence':
          final peerId = int.tryParse((decoded['to'] ?? '').toString()) ?? 0;
          final peerRole =
              int.tryParse((decoded['peerRole'] ?? '').toString()) ?? 0;
          if (peerId > 0 && peerRole > 0) {
            _presenceController.add(
              PresenceEvent(
                peerId: peerId,
                peerRole: peerRole,
                online: decoded['online'] == true,
                lastSeenAtMs:
                    int.tryParse((decoded['lastSeenAt'] ?? '0').toString()) ??
                    0,
              ),
            );
          }
          break;
        default:
          final msg = _parseWebSocketMessage(decoded);
          if (msg != null && !_isDuplicate(msg)) {
            _messageController.add(msg);
            _newMessageController.add(
              ChatNotificationEvent(
                unreadCount: 0, // authoritative count comes via unread_changed
                senderId: msg.senderId,
                preview: msg.messageContent.length > 40
                    ? '${msg.messageContent.substring(0, 40)}...'
                    : msg.messageContent,
                senderName: msg.senderName,
                senderAvatar: msg.senderAvatar,
                conversationId: msg.conversationId,
              ),
            );
            HKLog.logD(
              '[ChatRepository] New message from ${msg.senderId} (seqId=${msg.seqId})',
            );
          }
      }
    } catch (e) {
      HKLog.logD('[ChatRepository] Parse WS message error: $e');
    }
  }

  ChatMessage? _parseWebSocketMessage(Map<String, dynamic> data) {
    try {
      return ChatMessage.fromJson(data);
    } catch (e) {
      HKLog.logD('[ChatRepository] Parse message error: $e');
      return null;
    }
  }

  /// 取消監聽 + 關閉當前 socket，讓被放棄的（半開）連線無法再觸發任何 callback。
  void _teardownSocket() {
    _socketSub?.cancel();
    _socketSub = null;
    final s = _socket;
    _socket = null;
    s?.close();
    _pingTimer?.cancel();
    _pingTimer = null;
    _livenessTimer?.cancel();
    _livenessTimer = null;
  }

  void _onWebSocketDisconnect([int? epoch]) {
    // Epoch guard：忽略已被取代的舊連線遲來的 onDone（半開 socket 的 onDone
    // 不能去砍掉後來重連的健康 socket）。liveness / ping-catch 不帶 epoch，作用
    // 在當前連線。
    if (epoch != null && epoch != _connEpoch) return;
    // 同一次斷線只處理一次（ping-catch + onDone 可能對同一 socket 雙觸發）。
    if (_socket == null) return;
    final closeCode = _socket?.closeCode;
    final closeReason = _socket?.closeReason;
    HKLog.logD(
      '[ChatRepository] WebSocket disconnected - code: $closeCode, reason: $closeReason',
    );
    _teardownSocket();
    if (_connectionState != ChatConnectionState.disconnected) {
      _handleConnectionFailure();
    }
  }

  void _onWebSocketError(int epoch, dynamic error) {
    if (epoch != _connEpoch) return;
    HKLog.logD('[ChatRepository] WebSocket error: $error');
  }

  // ============================================================
  // 心跳與重連
  // ============================================================

  /// 心跳：雙向 ping/pong + 偵測方 read-deadline（§6.3）。
  void _startHeartbeat() {
    _pingTimer?.cancel();
    _pingTimer = Timer.periodic(_pingInterval, (_) => _sendPing());
    _armLiveness();
  }

  void _sendPing() {
    if (_socket == null || _connectionState != ChatConnectionState.connected) {
      return;
    }
    // 同步送出：socket 已死時 add() 會在這裡同步 throw → 立即 reconnect。
    // 舊版把 add() 包在 _getUserId().then() 的 async callback，throw 落在 Future
    // 裡、外層 sync catch 永遠收不到 → buyer 唯一的 liveness 訊號形同壞掉。
    try {
      _socket!.add(
        json.encode({
          'type': 'ping',
          'from': _currentUserId,
          'timestamp': DateTime.now().millisecondsSinceEpoch,
        }),
      );
    } catch (e) {
      HKLog.logD('[ChatRepository] ping send failed: $e');
      _onWebSocketDisconnect();
    }
  }

  /// (重)設讀取側 liveness 期限。任何入站幀都會 reset；超過 _livenessTimeout
  /// 沒收到任何幀就視為半開（死）連線並重連。server 會對 ping 回 pong
  /// (im-service P1)，所以即使 idle 聊天也能持續刷新此期限。
  void _armLiveness() {
    if (_socket == null) return;
    _livenessTimer?.cancel();
    _livenessTimer = Timer(_livenessTimeout, () {
      HKLog.logD(
        '[ChatRepository] liveness timeout — ${_livenessTimeout.inSeconds}s 無入站幀，視為斷線',
      );
      _onWebSocketDisconnect();
    });
  }

  void _handleConnectionFailure() {
    if (_connectionState == ChatConnectionState.disconnected) return;

    if (_reconnectAttempts >= _maxReconnectAttempts) {
      _startFallbackPolling();
      return;
    }

    _setConnectionState(ChatConnectionState.reconnecting);
    _reconnectAttempts++;

    // Full Jitter (§6.2): random(0, min(cap, base * 2^(attempt-1))). Jitter stops a
    // fleet of clients reconnecting in lockstep after a single-replica restart.
    final ceilingMs = math.min(1000 * (1 << (_reconnectAttempts - 1)), 30000);
    final delayMs = (math.Random().nextDouble() * ceilingMs).round();
    HKLog.logD(
      '[ChatRepository] Reconnecting in ${delayMs}ms (attempt $_reconnectAttempts/$_maxReconnectAttempts)',
    );

    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(Duration(milliseconds: delayMs), () {
      if (_connectionState == ChatConnectionState.reconnecting) {
        _connectWebSocket();
      }
    });
  }

  // ============================================================
  // HTTP Fallback Polling (deprecated - FCM handles offline delivery)
  // ============================================================

  void _startFallbackPolling() {
    // FCM 負責離線訊息推送，不再需要 HTTP 輪詢降級
    HKLog.logD(
      '[ChatRepository] Max reconnect attempts reached, relying on FCM for offline delivery',
    );
    _setConnectionState(ChatConnectionState.disconnected);
  }

  // ============================================================
  // 全域輪詢（已棄用 - FCM 處理離線訊息推送）
  // ============================================================

  /// 已棄用 - FCM 負責離線訊息推送，不再需要輪詢
  @Deprecated('No longer needed - FCM handles offline delivery')
  void startPolling({Duration interval = const Duration(seconds: 5)}) {}

  /// 已棄用
  @Deprecated('No longer needed')
  void stopPolling() {}

  /// 已棄用
  @Deprecated('No longer needed')
  void resetLastUnreadCount() {}

  // ============================================================
  // AppLifecycle 自動重連
  // ============================================================

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && hasChatScreenActive) {
      if (_connectionState != ChatConnectionState.connected &&
          _connectionState != ChatConnectionState.connecting) {
        HKLog.logD('[ChatRepository] App resumed, reconnecting WebSocket...');
        _reconnectAttempts = 0;
        connect();
      }
    }
  }

  // Base hosts
  static String get _httpBase => '${F.appFlavor.apiBaseUrl}/app/chat';

  static String get _wsBase {
    final host = F.appFlavor.gatewayHost
        .replaceFirst('https://', 'wss://')
        .replaceFirst('http://', 'ws://');
    return '$host/api/v1/app/chat/ws';
  }

  Future<Map<String, String>?> _authHeaders({bool jsonBody = true}) async {
    final token = await _sessionToken();
    if (token == null || token.isEmpty) return null;

    return {
      if (jsonBody) 'Content-Type': 'application/json',
      'Accept': 'application/json',
      'Authorization': 'Bearer $token',
    };
  }

  /// 鑄一張 WS 連線票（authed POST /chat/ws-ticket，帶 Bearer）。
  /// 回傳 null = 持久失敗（未登入 / 401-403，呼叫端停止重連）；
  /// throw = 暫時失敗（網路 / 5xx，呼叫端走重連 backoff）。
  Future<_WsTicket?> _mintWsTicket() async {
    final headers = await _authHeaders();
    if (headers == null) return null; // 無 token = 未登入
    final resp = await _dio.post<dynamic>(
      '$_httpBase/ws-ticket',
      options: Options(headers: headers),
    );
    final code = resp.statusCode ?? 0;
    if (code == 401 || code == 403) return null; // 已登出 → 持久失敗
    if (code < 200 || code >= 300) {
      throw Exception('ws-ticket mint HTTP $code');
    }
    final data = resp.data;
    final m = (data is Map && data['data'] is Map) ? data['data'] as Map : null;
    final ticket = m?['ticket']?.toString();
    if (ticket == null || ticket.isEmpty) {
      throw Exception('ws-ticket mint returned empty ticket');
    }
    return _WsTicket(ticket, m?['userId']?.toString() ?? '');
  }

  // ============================================================
  // 訊息發送（一律走 HTTP 確保持久化；WebSocket 只用於即時接收）
  // ============================================================

  /// 發送已讀回執（via WebSocket）
  void sendReadReceipt({required String receiverId}) {
    if (_connectionState == ChatConnectionState.connected && _socket != null) {
      try {
        final fromId = _accountId() ?? '';
        if (fromId.isEmpty) return;
        _socket?.add(
          json.encode({
            'type': 'read_receipt',
            'from': fromId,
            'to': receiverId,
            'timestamp': DateTime.now().millisecondsSinceEpoch,
          }),
        );
      } catch (_) {}
    }
  }

  /// 發送打字指示器（via WebSocket）
  void sendTypingIndicator({required String receiverId, bool isTyping = true}) {
    if (_connectionState != ChatConnectionState.connected || _socket == null) {
      return;
    }
    try {
      final fromIdStr = _accountId() ?? '';
      if (fromIdStr.isEmpty) return;
      // from / to 都送 String：後端 im-service domain.Message 的 from/to
      // 用 Go `json:",string"` tag，WS frame 這些欄位必須是 JSON 字串
      // （18-19 位 TSID > 2^53，送 JSON number 會被後端 unmarshal 拒絕/skip）。
      _socket?.add(
        json.encode({
          'type': isTyping ? 'typing' : 'stop_typing',
          'from': fromIdStr,
          'to': receiverId,
          'timestamp': DateTime.now().millisecondsSinceEpoch,
        }),
      );
    } catch (_) {}
  }

  Future<Map<String, dynamic>?> sendMessage({
    required String receiverId,
    required String messageContent,
    String messageType = 'text',
    Map<String, String>? extra,
  }) async {
    return _sendMessageHttp(receiverId, messageContent, messageType, extra);
  }

  Future<Map<String, dynamic>?> _sendMessageHttp(
    String receiverId,
    String messageContent,
    String messageType, [
    Map<String, String>? extra,
  ]) async {
    final headers = await _authHeaders();
    if (headers == null) throw Exception('Unauthorized: missing JWT token');
    final uri = Uri.parse('$_httpBase/messages');
    final jsonBody = {
      'toUserId': receiverId,
      'content': messageContent,
      'type': messageType.isNotEmpty ? messageType : 'chat',
      if (extra != null && extra.isNotEmpty) 'extra': extra,
    };
    HKLog.logD('[ChatRepository] sendMessage HTTP - POST $uri');
    final resp = await _dio.post<dynamic>(
      uri.toString(),
      data: jsonBody,
      options: Options(headers: headers),
    );
    final respBody = json.encode(resp.data);
    HKLog.logD(
      '[ChatRepository] sendMessage HTTP - Response: ${resp.statusCode} $respBody',
    );
    if (resp.statusCode != null &&
        resp.statusCode! >= 200 &&
        resp.statusCode! < 300) {
      try {
        return resp.data as Map<String, dynamic>;
      } catch (_) {}
      return {'ok': true};
    }
    if (resp.statusCode == 401) throw const _ChatSendException('登入已過期，請重新登入');
    final bizCode = resp.data is Map ? (resp.data as Map)['code'] : null;
    final bizMsg = reportBlockErrorMessage(bizCode);
    if (bizMsg != null) throw _ChatSendException(bizMsg);
    final errMsg = _extractErrorMessage(resp.data);
    throw _ChatSendException(errMsg ?? '傳送失敗（${resp.statusCode}）');
  }

  /// 從 API error body 取出 message，並去掉 "[NNNN] " 業務碼前綴。
  static String? _extractErrorMessage(dynamic body) {
    if (body is! Map) return null;
    final raw = body['message']?.toString();
    if (raw == null || raw.isEmpty) return null;
    return raw.replaceFirst(RegExp(r'^\[\d+\]\s*'), '');
  }

  // GET /v1/app/chat/history/:targetId - 獲取與指定用戶的聊天記錄
  Future<List<ChatMessage>> getHistory({
    required String participantId,
    int page = 1,
    int pageSize = 50,
    String? beforeId,
    String? conversationId,
  }) async {
    final headers = await _authHeaders(jsonBody: false);
    if (headers == null) {
      HKLog.logW(
        'ChatRepository: getHistory - No headers, throwing Unauthorized',
      );
      throw Exception('Unauthorized: missing JWT token');
    }
    // OpenAPI: /app/chat/history/:targetId?page=&pageSize=
    final targetId = conversationId ?? participantId;
    final params = <String, String>{
      'page': '$page',
      'pageSize': '$pageSize',
      if (beforeId != null) 'beforeId': beforeId,
    };
    final uri = Uri.parse(
      '$_httpBase/history/$targetId',
    ).replace(queryParameters: params);
    HKLog.logD('ChatRepository: getHistory - GET $uri');
    HKLog.logD(
      'ChatRepository: getHistory - Headers: ${headers.keys.toList()}',
    );
    final resp = await _dio.get<dynamic>(
      uri.toString(),
      options: Options(headers: headers),
    );
    final respBody = json.encode(resp.data);
    HKLog.logD(
      'ChatRepository: getHistory - Response: ${resp.statusCode} ${respBody.length > 100 ? "${respBody.substring(0, 100)}..." : respBody}',
    );
    if (resp.statusCode == 404 || resp.statusCode == 204) {
      // No conversation/history yet — treat as empty instead of error
      return const <ChatMessage>[];
    }
    if (resp.statusCode != 200) {
      if (resp.statusCode == 401) throw Exception('Unauthorized (401)');
      throw Exception('History failed (${resp.statusCode})');
    }
    if (resp.data == null) return const <ChatMessage>[];
    final decoded = resp.data;
    // OpenAPI 格式: { code: 0, data: { messages: [...], total, hasMore } }
    List list;
    if (decoded is List) {
      list = decoded;
    } else if (decoded is Map) {
      final data = decoded['data'];
      if (data is Map && data['messages'] is List) {
        list = data['messages'] as List;
      } else if (data is List) {
        list = data;
      } else {
        list = const [];
      }
    } else {
      list = const [];
    }
    return List<ChatMessage>.from(
      list.map(
        (e) => ChatMessage.fromJson(Map<String, dynamic>.from(e as Map)),
      ),
    );
  }

  /// 取得聊天歷史及參與者資訊（含 participants，避免額外 API 呼叫）
  Future<
    ({List<ChatMessage> messages, List<Map<String, dynamic>> participants})
  >
  getHistoryWithParticipants({
    required String participantId,
    int pageSize = 50,
    String? conversationId,
  }) async {
    final headers = await _authHeaders(jsonBody: false);
    if (headers == null) {
      HKLog.logE(
        '[ChatRepository] getHistoryWithParticipants - _authHeaders returned null (no JWT token)',
      );
      throw Exception('Unauthorized: missing JWT token');
    }
    final targetId = conversationId ?? participantId;
    final params = <String, String>{'pageSize': '$pageSize'};
    final uri = Uri.parse(
      '$_httpBase/history/$targetId',
    ).replace(queryParameters: params);
    HKLog.logD('[ChatRepository] getHistoryWithParticipants - GET $uri');
    final resp = await _dio.get<dynamic>(
      uri.toString(),
      options: Options(headers: headers),
    );
    HKLog.logD(
      '[ChatRepository] getHistoryWithParticipants - Response: ${resp.statusCode}',
    );
    if (resp.statusCode == 404 || resp.statusCode == 204) {
      HKLog.logD(
        '[ChatRepository] getHistoryWithParticipants - No history (${resp.statusCode})',
      );
      return (
        messages: const <ChatMessage>[],
        participants: const <Map<String, dynamic>>[],
      );
    }
    if (resp.statusCode != 200) {
      if (resp.statusCode == 401) throw Exception('Unauthorized (401)');
      throw Exception('History failed (${resp.statusCode})');
    }
    if (resp.data == null) {
      return (
        messages: const <ChatMessage>[],
        participants: const <Map<String, dynamic>>[],
      );
    }
    final decoded = resp.data;
    List msgList = const [];
    List participantList = const [];
    if (decoded is Map) {
      final data = decoded['data'];
      if (data is Map) {
        msgList = (data['messages'] is List) ? data['messages'] : const [];
        if (data['participants'] is List) {
          participantList = data['participants'];
        }
      } else if (data is List) {
        msgList = data;
      }
    } else if (decoded is List) {
      msgList = decoded;
    }
    return (
      messages: List<ChatMessage>.from(
        msgList.map(
          (e) => ChatMessage.fromJson(Map<String, dynamic>.from(e as Map)),
        ),
      ),
      participants: List<Map<String, dynamic>>.from(
        participantList.map((e) => Map<String, dynamic>.from(e as Map)),
      ),
    );
  }

  // GET /v1/app/chat/conversations - 獲取會話列表
  Future<List<Map<String, dynamic>>> getConversations({
    int page = 1,
    int pageSize = 50,
  }) async {
    HKLog.logD('=== getConversations API Call ===');
    final headers = await _authHeaders(jsonBody: false);
    if (headers == null) throw Exception('Unauthorized: missing JWT token');
    final uri = Uri.parse(
      '$_httpBase/conversations',
    ).replace(queryParameters: {'page': '$page', 'pageSize': '$pageSize'});
    HKLog.logD('Request URL: $uri');
    HKLog.logD('Request Headers: $headers');
    final resp = await _dio.get<dynamic>(
      uri.toString(),
      options: Options(headers: headers),
    );
    final respBody = json.encode(resp.data);
    HKLog.logD('Response Status: ${resp.statusCode}');
    HKLog.logD('Response Body: $respBody');
    if (resp.statusCode != 200) {
      HKLog.logE('=== getConversations FAILED ===');
      if (resp.statusCode == 401) throw Exception('Unauthorized (401)');
      throw Exception('Conversations failed (${resp.statusCode})');
    }
    final decoded = resp.data;
    if (decoded is List) {
      return List<Map<String, dynamic>>.from(
        decoded.map((e) => Map<String, dynamic>.from(e as Map)),
      );
    }
    if (decoded is Map) {
      final data = decoded['data'];
      // Gateway 回傳 { data: { conversations: [...], total: N } }
      if (data is Map && data['conversations'] is List) {
        return List<Map<String, dynamic>>.from(
          (data['conversations'] as List).map(
            (e) => Map<String, dynamic>.from(e as Map),
          ),
        );
      }
      if (data is List) {
        return List<Map<String, dynamic>>.from(
          data.map((e) => Map<String, dynamic>.from(e as Map)),
        );
      }
    }
    return const [];
  }

  // GET /v1/app/chat/unread - 獲取未讀訊息總數
  // API 返回格式: { code: 0, data: { unread: N } }
  Future<int> getUnreadCount() async {
    // 如果連續多次 404，減少日誌輸出
    if (_consecutive404 > 3) {
      // 靜默模式：endpoint 尚未部署，不再重複輸出 log
      try {
        final headers = await _authHeaders(jsonBody: false);
        if (headers == null) return 0;
        final resp = await _dio.get<dynamic>(
          '$_httpBase/unread',
          options: Options(headers: headers),
        );
        if (resp.statusCode == 404) return 0;
        _consecutive404 = 0; // endpoint 恢復，重置計數器
        return _parseUnreadResponse(resp.data);
      } catch (_) {
        return 0;
      }
    }

    final headers = await _authHeaders(jsonBody: false);
    if (headers == null) throw Exception('Unauthorized: missing JWT token');
    final uri = Uri.parse('$_httpBase/unread');
    final resp = await _dio.get<dynamic>(
      uri.toString(),
      options: Options(headers: headers),
    );
    if (resp.statusCode == 404) {
      _consecutive404++;
      if (_consecutive404 == 1) {
        HKLog.logW(
          'ChatRepository: getUnreadCount - 404 (endpoint not deployed yet, suppressing further logs)',
        );
      }
      return 0;
    }
    if (resp.statusCode != 200) return 0;
    _consecutive404 = 0;
    return _parseUnreadResponse(resp.data);
  }

  int _parseUnreadResponse(dynamic decoded) {
    if (decoded is Map) {
      final data = decoded['data'];
      if (data is Map) {
        for (final k in [
          'unread',
          'count',
          'totalUnreadCount',
          'total',
          'unreadCount',
        ]) {
          final v = data[k];
          if (v is int) return v;
          final n = int.tryParse(v?.toString() ?? '');
          if (n != null) return n;
        }
      }
      for (final k in [
        'unread',
        'count',
        'totalUnreadCount',
        'total',
        'unreadCount',
      ]) {
        final v = decoded[k];
        if (v is int) return v;
        final n = int.tryParse(v?.toString() ?? '');
        if (n != null) return n;
      }
    }
    if (decoded is int) return decoded;
    return 0;
  }

  // PATCH /v1/app/chat/conversations/:targetId - 標記與指定用戶的會話為已讀（canonical）
  Future<bool> markConversationAsRead(String targetId) async {
    final headers = await _authHeaders(jsonBody: false);
    if (headers == null) throw Exception('Unauthorized: missing JWT token');
    final uri = Uri.parse('$_httpBase/conversations/$targetId');
    HKLog.logD('ChatRepository: markConversationAsRead - PATCH $uri');
    final resp = await _dio.patch<dynamic>(
      uri.toString(),
      options: Options(headers: headers),
    );
    HKLog.logD(
      'ChatRepository: markConversationAsRead - Response: ${resp.statusCode}',
    );
    return resp.statusCode != null &&
        resp.statusCode! >= 200 &&
        resp.statusCode! < 300;
  }

  // 標記所有訊息為已讀
  Future<bool> markAllRead() async {
    final headers = await _authHeaders(jsonBody: false);
    if (headers == null) throw Exception('Unauthorized: missing JWT token');
    final uri = Uri.parse('$_httpBase/conversations');
    HKLog.logD('ChatRepository: markAllRead - PATCH $uri');
    final resp = await _dio.patch<dynamic>(
      uri.toString(),
      options: Options(headers: headers),
    );
    HKLog.logD('ChatRepository: markAllRead - Response: ${resp.statusCode}');
    return resp.statusCode != null &&
        resp.statusCode! >= 200 &&
        resp.statusCode! < 300;
  }

  /// Mark all messages in a 1-1 chat as read by participant id（canonical）
  Future<bool> markAllMessagesRead({required String participantId}) async {
    return markConversationAsRead(participantId);
  }

  /// 訂閱對方（peerId + peerRole）的在線狀態，伺服器會立即推一次 snapshot。
  /// peerRole: 1=buyer, 2=seller。
  void subscribePresence(int peerId, int peerRole) {
    if (_socket == null || _connectionState != ChatConnectionState.connected) {
      return;
    }
    try {
      _socket!.add(
        json.encode({
          'type': 'presence_sub',
          'to': peerId.toString(),
          'peerRole': peerRole,
        }),
      );
    } catch (e) {
      HKLog.logD('[ChatRepository] subscribePresence failed: $e');
    }
  }

  /// 取消訂閱對方在線狀態。
  void unsubscribePresence(int peerId, int peerRole) {
    if (_socket == null || _connectionState != ChatConnectionState.connected) {
      return;
    }
    try {
      _socket!.add(
        json.encode({
          'type': 'presence_unsub',
          'to': peerId.toString(),
          'peerRole': peerRole,
        }),
      );
    } catch (e) {
      HKLog.logD('[ChatRepository] unsubscribePresence failed: $e');
    }
  }
}

/// 傳訊失敗的 Exception，toString() 直接回傳 message（避免 "Exception: " 前綴）。
class _ChatSendException implements Exception {
  const _ChatSendException(this.message);
  final String message;
  @override
  String toString() => message;
}

/// ws-ticket 鑄票結果：單次票 + 後端推導出的 userId（與 web buildWsUrl 對齊）。
class _WsTicket {
  const _WsTicket(this.ticket, this.userId);
  final String ticket;
  final String userId;
}

/// 新訊息通知事件——由 WebSocket default case 發出，供全域 notification layer 使用。
/// unreadCount 固定為 0（權威未讀數由 unread_changed 事件負責）。
class ChatNotificationEvent {
  const ChatNotificationEvent({
    required this.unreadCount,
    required this.senderId,
    required this.preview,
    this.senderName,
    this.senderAvatar,
    this.conversationId,
  });
  final int unreadCount;
  final String senderId;
  final String preview;
  final String? senderName;
  final String? senderAvatar;
  final String? conversationId;
}

@Riverpod(keepAlive: true)
ChatRepository chatRepository(Ref ref) {
  // keepAlive: one instance for the app lifetime, owned by the provider.
  // If ever invalidated, a fresh instance is created (no closed-controller reuse).
  final repo = ChatRepository._(
    () => currentSessionToken(ref),
    () => ref.read(sessionProvider).value?.userId,
  );
  ref.onDispose(repo.dispose);
  return repo;
}
