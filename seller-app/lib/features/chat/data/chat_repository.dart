import '../../../features/seller/data/current_store.dart';
import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:rxdart/rxdart.dart';

import '../../../config/flavor.dart';
import '../../../core/networking/app_version_headers.dart';
import '../../../core/networking/dio_provider.dart';
import '../../order/data/order_status_notifier.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/platform_io.dart';
import 'chat_model.dart';

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

/// 從聊天室 roomId 解析「對方」的數字 accountId。
///
/// 支援兩種 roomId 格式：
/// - 新版 `b{buyerId}_s{sellerId}`（WS payload / 部分 API 回傳）
/// - 舊版 `m{sellerId}_{buyerId}`
///
/// `/app/chat/history/{targetId}` 依 spec 只收「對方使用者 ID」（純數字），
/// 直接把 roomId 丟過去會 400 `invalid target_id`（2026-07-03 實測）。
/// 傳入純數字時原樣回傳；[ownId] 用來排除自己那一段。
String resolveChatPeerId(String raw, {String? ownId}) {
  final s = raw.trim();
  if (s.isEmpty || !s.contains('_')) return s;
  final candidates = <String>[];
  for (final part in s.split('_')) {
    final digits = part.replaceFirst(RegExp(r'^[a-zA-Z]+'), '');
    if (digits.isEmpty || int.tryParse(digits) == null) continue;
    candidates.add(digits);
  }
  if (candidates.isEmpty) return s;
  if (ownId != null && ownId.isNotEmpty) {
    for (final c in candidates) {
      if (c != ownId) return c;
    }
  }
  return candidates.first;
}

class ChatRepository with WidgetsBindingObserver {
  // ============================================================
  // Constructor
  // ============================================================

  ChatRepository(this._httpDio, [this._ref, bool observeLifecycle = false]) {
    if (observeLifecycle) WidgetsBinding.instance.addObserver(this);
  }

  Dio _httpDio;
  final Ref? _ref;

  /// 注入帶 interceptor 的 dioProvider Dio（Auth 注入 + 401 登出 + tracing），
  /// 取代啟動時的 bare client。登入 / session 還原時由 session_provider 呼叫。
  void useHttpClient(Dio dio) => _httpDio = dio;

  bool get _isLoggedIn => _ref?.read(sessionProvider).isLoggedIn ?? false;
  String? get _sellerId => _ref?.read(sellerStoreIdProvider);

  /// 本 app 在 chat wire 上的自己身份（seller-flow-verify flow 51：client
  /// 自己的 chat id 永遠是 active role 的 store_id，絕不是 account_id——
  /// 兩者是不同身份空間）。不 fallback 到 accountId：塞進去只會讓下游（排
  /// 除自己算 peer、echo 去重、read_receipt/typing 的 from）比對成一個保
  /// 證錯的值。sellerId 尚未同步完成時寧可回傳 null 讓呼叫端走 null-safe
  /// 分支，也不要用錯的 id 冒充（issue #114：accountId 已不再被
  /// 悄悄覆寫成 store_id，兩者語意不再相等）。
  String? get ownAccountId => _sellerId;

  // ============================================================
  // 環境設定
  // ============================================================

  static String get _wsBase {
    final host = F.appFlavor.gatewayHost;
    if (host.startsWith('https://')) {
      return '${host.replaceFirst('https://', 'wss://')}/api/v1/app/chat/ws';
    } else {
      return '${host.replaceFirst('http://', 'ws://')}/api/v1/app/chat/ws';
    }
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
  int _reconnectAttempts = 0;
  String? _currentUserId; // 連線時快取；讓 ping 送出維持同步
  static const int _maxReconnectAttempts = 5;
  // ponytail: carrier NAT (~30s idle) 的保守預設。心跳間隔與 2× liveness 視窗
  // 應在量測 CF/edge idle 後再調（IM 設計書 D6 / §11.2）——量測前別硬調更低。
  static const Duration _pingInterval = Duration(seconds: 15);
  static const Duration _livenessTimeout = Duration(seconds: 30);

  ChatConnectionState _connectionState = ChatConnectionState.disconnected;
  // BehaviorSubject.seeded 會把當前狀態 replay 給晚訂閱者（§6.6 emit-on-subscribe）——
  // 修掉那類「晚掛載的 widget 卡在初始 disconnected 而誤報網路連線異常」的 bug；
  // 純 broadcast Stream 不 replay last value。
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
  Stream<ChatMessage> get onMessage => _messageController.stream;

  final BehaviorSubject<int> _unreadCountController =
      BehaviorSubject<int>.seeded(0);
  Stream<int> get onUnreadCountChange => _unreadCountController.stream;

  final StreamController<ChatNotificationEvent> _newMessageController =
      StreamController<ChatNotificationEvent>.broadcast();
  Stream<ChatNotificationEvent> get onNewMessage =>
      _newMessageController.stream;

  final StreamController<PresenceEvent> _presenceController =
      StreamController<PresenceEvent>.broadcast();

  /// 對方在線狀態推送（取代 30 秒輪詢 /chat/online）
  Stream<PresenceEvent> get onPresence => _presenceController.stream;

  int _lastUnreadCount = -1;
  Timer? _pollTimer;

  /// Deduplication set for WS + FCM dual-delivery.
  /// Key format: "${conversationId}_${seqId}"
  final Set<String> _seenMessages = {};

  /// Returns true if this message has already been seen (duplicate).
  bool _isDuplicate(ChatMessage msg) {
    if (msg.conversationId == null || msg.conversationId!.isEmpty) return false;
    if (msg.seqId <= 0) return false;
    final key = '${msg.conversationId}_${msg.seqId}';
    // Prevent unbounded growth
    if (_seenMessages.length > 5000) _seenMessages.clear();
    return !_seenMessages.add(key);
  }

  // ============================================================
  // 連線控制
  // ============================================================

  int _chatScreenRefCount = 0;

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
    _lastUnreadCount = -1;
    _seenMessages.clear();
    _teardownSocket();
    _setConnectionState(ChatConnectionState.disconnected);
  }

  // ============================================================
  // 懶加載連線
  // ============================================================

  Future<void> connectForChat() async {
    _chatScreenRefCount++;
    HKLog.logD(
      '[ChatRepository] connectForChat: refCount=$_chatScreenRefCount',
    );
    if (_chatScreenRefCount == 1) {
      await connect();
    }
  }

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

  void _setConnectionState(ChatConnectionState state) {
    if (_connectionState == state) return;
    _connectionState = state;
    _connectionStateController.add(state);
    HKLog.logD('[ChatRepository] Connection state: ${state.name}');
  }

  Future<void> _connectWebSocket() async {
    final gen = ++_connectGen; // 認領這次連線嘗試

    // ws-ticket：authed POST 鑄單次短效票，WS URL 只帶 ?ticket=（不帶長命 JWT /
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
    final fallbackId = ownAccountId ?? '';
    _currentUserId = ticket.userId.isNotEmpty ? ticket.userId : fallbackId;

    try {
      final wsUrl =
          '$_wsBase?ticket=${Uri.encodeQueryComponent(ticket.ticket)}';
      HKLog.logD('[ChatRepository] Connecting to WebSocket (ws-ticket)');

      // WS 握手也是打 Gateway 的請求，版本 header 同樣要帶（54 號設計書 §6.5）。
      final socket = await WebSocket.connect(
        wsUrl,
        headers: appVersionHeaders(),
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
      _fetchUnreadCount();
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
    _armLiveness(); // 任何入站幀都是 liveness 訊號 → 重設讀取側期限（§6.3）
    try {
      final decoded = json.decode(data.toString());
      if (decoded is! Map<String, dynamic>) return;

      final type = decoded['type']?.toString() ?? 'text';
      HKLog.logD(
        '[ChatRepository] Received WS raw: type=$type from=${decoded['from']} to=${decoded['to']} content=${(decoded['content']?.toString() ?? '').length > 50 ? '${decoded['content'].toString().substring(0, 50)}...' : decoded['content']}',
      );

      switch (type) {
        case 'typing':
        case 'start_typing':
        case 'stop_typing':
          // 轉發給 UI（chat_thread_screen 偵測 senderId 顯示打字指示器）
          _messageController.add(
            ChatMessage(
              id: '',
              senderId: (decoded['from'] ?? '').toString(),
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
              senderId: (decoded['from'] ?? '').toString(),
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
          // Authoritative unread count pushed by the server on every send/mark-read.
          final c =
              int.tryParse(
                (decoded['unreadCount'] ?? decoded['count'] ?? '').toString(),
              ) ??
              0;
          _lastUnreadCount = c;
          _unreadCountController.add(c);
          break;
        case 'chat_ack':
          // Acknowledgement for a sent message (contains server-assigned id/seqId).
          final msg = _parseBackendMessage(decoded);
          if (msg != null) _messageController.add(msg);
          break;
        case 'system':
        case 'notification':
          final msg = _parseBackendMessage(decoded);
          if (msg != null) {
            _messageController.add(msg);
            _emitNewMessageEvent(decoded, msg);
          }
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
          final msg = _parseBackendMessage(decoded);
          if (msg == null) {
            HKLog.logD(
              '[ChatRepository] ⚠ _parseBackendMessage returned null for type=$type',
            );
          } else if (_isDuplicate(msg)) {
            HKLog.logD(
              '[ChatRepository] ⚠ Duplicate message skipped: ${msg.id}',
            );
          } else {
            _messageController.add(msg);
            _emitNewMessageEvent(decoded, msg);
            HKLog.logD(
              '[ChatRepository] ✅ New message from ${msg.senderId} to ${msg.receiverId}: ${msg.messageContent.length > 30 ? '${msg.messageContent.substring(0, 30)}...' : msg.messageContent}',
            );
          }
      }
    } catch (e) {
      HKLog.logD('[ChatRepository] Parse WS message error: $e');
    }
  }

  ChatMessage? _parseBackendMessage(Map<String, dynamic> data) {
    try {
      final fromId = data['from'];
      final toId = data['to'];
      final content = data['content']?.toString() ?? '';
      final messageType = data['type']?.toString() ?? 'text';
      final timestamp = data['timestamp'];
      final roomId = data['roomId']?.toString();
      final seqId = data['seqId'];

      return ChatMessage.fromJson({
        // fromJson 讀 'fromUserId' 或 'from'，不是 'senderId'
        'from': fromId,
        'to': toId,
        'content': content,
        'type': messageType,
        'timestamp': timestamp,
        'conversationId': roomId,
        'seqId': seqId,
        'id': data['messageId'] ?? data['id'] ?? '',
      });
    } catch (e) {
      HKLog.logD('[ChatRepository] Parse message error: $e');
      return null;
    }
  }

  void _emitNewMessageEvent(Map<String, dynamic> data, ChatMessage msg) {
    final senderId = msg.senderId.trim();
    if (senderId.isEmpty) return;

    final ownId = ownAccountId;
    if (ownId != null && ownId == senderId) return;

    _newMessageController.add(
      ChatNotificationEvent(
        unreadCount: _lastUnreadCount < 0 ? 0 : _lastUnreadCount,
        senderId: senderId,
        senderName: _firstNonEmpty(data, [
          'senderName',
          'sender_name',
          'fromName',
          'from_name',
          'displayName',
        ]),
        // WS frame 的頭像唯一契約欄位＝`avatarKey`（裸 objectKey，49 §6.7 S4）。
        // `data` 只可能來自 WebSocket（本方法兩個呼叫點都在 _handleMessage 內），
        // FCM 走 push/manager.dart 自己的 key list，不經過這裡。
        senderAvatar: _firstNonEmpty(data, ['avatarKey']),
        preview: _previewFromContent(msg.messageContent),
        conversationId: msg.conversationId,
      ),
    );
  }

  String? _firstNonEmpty(Map<String, dynamic> data, List<String> keys) {
    for (final key in keys) {
      final value = data[key]?.toString().trim();
      if (value != null && value.isNotEmpty) return value;
    }
    return null;
  }

  String _previewFromContent(String content) {
    var preview = content.trim();
    if (preview.startsWith('{') && preview.endsWith('}')) {
      try {
        final decoded = json.decode(preview);
        if (decoded is Map) {
          final type = (decoded['messageType'] ?? decoded['type'])?.toString();
          if (type == 'image') return '[圖片]';
          if (type == 'product') return '[商品]';
          final text = (decoded['text'] ?? decoded['content'])?.toString();
          if (text != null && text.trim().isNotEmpty) {
            preview = text.trim();
          }
        }
      } catch (_) {}
    }
    if (preview.length > 40) return '${preview.substring(0, 40)}...';
    return preview;
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
    if (error is WebSocketException) {
      HKLog.logD(
        '[ChatRepository] WebSocket exception message: ${error.message}',
      );
    }
  }

  void _handleConnectionFailure() {
    if (_connectionState == ChatConnectionState.disconnected) return;

    if (_reconnectAttempts >= _maxReconnectAttempts) {
      HKLog.logD(
        '[ChatRepository] Max reconnect attempts reached, falling back to HTTP polling',
      );
      _setConnectionState(ChatConnectionState.fallbackPolling);
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

  // ============================================================
  // HTTP Fallback
  // ============================================================

  void _startFallbackPolling() {
    // No longer polling via HTTP -- FCM handles offline delivery.
    HKLog.logD(
      '[ChatRepository] Max reconnect attempts reached, relying on FCM for offline delivery',
    );
    _setConnectionState(ChatConnectionState.disconnected);
  }

  Future<void> _fetchUnreadCount() async {
    try {
      final count = await getUnreadCount();
      _lastUnreadCount = count;
      _unreadCountController.add(count);
    } catch (e) {
      HKLog.logD('[ChatRepository] Fetch unread count error: $e');
    }
  }

  // ============================================================
  // 舊 API 相容（startPolling/stopPolling）
  // ============================================================

  @Deprecated('No longer needed - FCM handles offline delivery')
  void startPolling({Duration interval = const Duration(seconds: 1)}) {
    HKLog.logD('[ChatRepository] startPolling -> starting OrderStatusNotifier');
    OrderStatusNotifier.instance.startListening();
  }

  @Deprecated('No longer needed - FCM handles offline delivery')
  void stopPolling() {
    HKLog.logD(
      '[ChatRepository] stopPolling -> stopping OrderStatusNotifier and disconnecting',
    );
    OrderStatusNotifier.instance.stopListening();
    disconnect();
  }

  // ============================================================
  // HTTP API
  // ============================================================

  /// dioProvider 以外的 header；身分由 dioProvider 的 auth interceptor 帶。沒有登入時回
  /// null（呼叫端視為未登入）。
  Map<String, dynamic>? _extraHeaders() {
    if (!_isLoggedIn) return null;
    return const {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
    };
  }

  /// 鑄一張 WS 連線票（authed POST /chat/ws-ticket，帶 Bearer + X-Active-Role）。
  /// 回傳 null = 持久失敗（未登入 / 401-403，呼叫端停止重連）；
  /// throw = 暫時失敗（網路 / 5xx，呼叫端走重連 backoff）。
  Future<_WsTicket?> _mintWsTicket() async {
    final headers = _extraHeaders();
    if (headers == null) return null; // 未登入
    final resp = await _httpDio.post(
      '/api/v1/app/chat/ws-ticket',
      options: Options(
        validateStatus: (_) => true,
        headers: {
          ...headers,
          // 賣家固定 seller 視角；只是視角，不是授權。
          kActiveRoleHeader: kAppActiveRole,
        },
      ),
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

  Future<Map<String, dynamic>?> sendMessage({
    required String receiverId,
    required String messageContent,
    String messageType = 'text',
    Map<String, String>? extra,
  }) async {
    // 一律走 HTTP 確保訊息持久化到 DB；WebSocket 只用於即時接收
    return _sendMessageHttp(receiverId, messageContent, messageType, extra);
  }

  Future<Map<String, dynamic>?> _sendMessageHttp(
    String receiverId,
    String messageContent,
    String messageType, [
    Map<String, String>? extra,
  ]) async {
    final headers = _extraHeaders();
    if (headers == null) throw Exception('No auth token');

    HKLog.logD('[ChatRepository] POST /chat/messages');

    final resp = await _httpDio.post(
      '/api/v1/app/chat/messages',
      data: {
        'toUserId': receiverId,
        'content': messageContent,
        'type': messageType,
        if (extra != null && extra.isNotEmpty) 'extra': extra,
      },
      options: Options(validateStatus: (s) => true, headers: headers),
    );
    if ((resp.statusCode ?? 0) >= 200 && (resp.statusCode ?? 0) < 300) {
      final result = resp.data is Map<String, dynamic>
          ? Map<String, dynamic>.from(resp.data as Map)
          : <String, dynamic>{'ok': true};
      result['via'] = 'http';
      return result;
    }
    final errMsg = _extractErrorMessage(resp.data);
    final errCode = _extractErrorCode(resp.data, resp.statusCode);
    throw ChatSendException(
      errMsg ?? '傳送失敗（${resp.statusCode}）',
      code: errCode,
    );
  }

  static String? _extractErrorMessage(dynamic body) {
    if (body is! Map) return null;
    final raw = (body['messageZh'] ?? body['message'])?.toString();
    if (raw == null || raw.isEmpty) return null;
    return raw;
  }

  /// 取出統一回應 envelope 的業務碼（`code` 欄位），用於 call-site 層識別
  /// 9202（BlockedMessageNotAllowed）/ 9210（ModerationRejected）等
  /// 檢舉／封鎖相關業務碼（backend-payuni#645）；沒有的話 fallback 用 HTTP status。
  static int _extractErrorCode(dynamic body, int? statusCode) {
    if (body is Map) {
      final c = body['code'];
      if (c is int) return c;
      if (c is num) return c.toInt();
      final parsed = int.tryParse(c?.toString() ?? '');
      if (parsed != null) return parsed;
    }
    return statusCode ?? -1;
  }

  void sendReadReceipt({required String receiverId}) {
    if (_connectionState == ChatConnectionState.connected && _socket != null) {
      try {
        _socket!.add(
          json.encode({
            'type': 'read_receipt',
            'from': ownAccountId,
            'to': receiverId,
            'timestamp': DateTime.now().millisecondsSinceEpoch,
          }),
        );
      } catch (e) {
        HKLog.logD('[ChatRepository] Send read receipt failed: $e');
      }
    }
  }

  void sendTypingIndicator({required String receiverId, bool isTyping = true}) {
    if (_connectionState == ChatConnectionState.connected && _socket != null) {
      try {
        _socket!.add(
          json.encode({
            'type': isTyping ? 'typing' : 'stop_typing',
            'from': ownAccountId,
            'to': receiverId,
            'timestamp': DateTime.now().millisecondsSinceEpoch,
          }),
        );
      } catch (e) {
        HKLog.logD('[ChatRepository] Send typing indicator failed: $e');
      }
    }
  }

  Future<List<ChatMessage>> getHistory({
    required String participantId,
    int? beforeId,
    int pageSize = 20,
    String? conversationId,
  }) async {
    final result = await getHistoryWithParticipants(
      participantId: participantId,
      beforeId: beforeId,
      pageSize: pageSize,
      conversationId: conversationId,
    );
    return result.messages;
  }

  /// 取得聊天歷史及參與者資訊（含 participants 陣列，避免額外 API 呼叫）
  Future<ChatHistoryResult> getHistoryWithParticipants({
    required String participantId,
    int? beforeId,
    int pageSize = 20,
    String? conversationId,
  }) async {
    final headers = _extraHeaders();
    if (headers == null) throw Exception('No auth token');

    final params = <String, dynamic>{'pageSize': '$pageSize'};
    if (beforeId != null) params['beforeId'] = '$beforeId';

    // spec：targetId = 對方使用者 ID（純數字）。conversationId 可能是 roomId
    // 格式（b..._s...），先正規化，否則後端回 400 invalid target_id。
    final rawTarget = ((conversationId ?? '').isNotEmpty)
        ? conversationId!
        : participantId;
    final targetId = resolveChatPeerId(rawTarget, ownId: ownAccountId);
    final resp = await _httpDio.get(
      '/api/v1/app/chat/history/$targetId',
      queryParameters: params,
      options: Options(validateStatus: (s) => true, headers: headers),
    );
    if (resp.statusCode == 404 || resp.statusCode == 204) {
      return const ChatHistoryResult(messages: [], participants: []);
    }
    if ((resp.statusCode ?? 0) != 200) {
      throw Exception('History failed (${resp.statusCode})');
    }
    final decoded = resp.data;
    List msgList;
    List participantList = const [];
    if (decoded is List) {
      msgList = decoded;
    } else if (decoded is Map) {
      final data = decoded['data'];
      if (data is List) {
        msgList = data;
      } else if (data is Map) {
        msgList = (data['messages'] is List) ? data['messages'] : const [];
        if (data['participants'] is List) {
          participantList = data['participants'];
        }
      } else {
        msgList = const [];
      }
    } else {
      msgList = const [];
    }
    return ChatHistoryResult(
      messages: List<ChatMessage>.from(
        msgList.map(
          (e) => ChatMessage.fromJson(Map<String, dynamic>.from(e as Map)),
        ),
      ),
      participants: List<ChatParticipant>.from(
        participantList.map(
          (e) => ChatParticipant.fromJson(Map<String, dynamic>.from(e as Map)),
        ),
      ),
    );
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

  Future<List<Map<String, dynamic>>> getConversations({
    int page = 1,
    int pageSize = 50,
  }) async {
    final headers = _extraHeaders();
    if (headers == null) throw Exception('Unauthorized: missing JWT token');

    HKLog.logD('[ChatRepository] GET /chat/conversations');
    final resp = await _httpDio.get(
      '/api/v1/app/chat/conversations',
      queryParameters: {'page': '$page', 'pageSize': '$pageSize'},
      options: Options(validateStatus: (s) => true, headers: headers),
    );
    HKLog.logD('[ChatRepository] Response ${resp.statusCode}');

    if ((resp.statusCode ?? 0) != 200) {
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

  Future<bool> markMessageRead(String targetId, {String? messageId}) async {
    if (messageId == null) {
      return markConversationAsRead(targetId);
    }
    final headers = _extraHeaders();
    if (headers == null) throw Exception('Unauthorized: missing JWT token');

    final resp = await _httpDio.patch(
      '/api/v1/app/chat/messages',
      data: {
        'messageIds': [messageId],
      },
      options: Options(validateStatus: (s) => true, headers: headers),
    );
    return (resp.statusCode ?? 0) >= 200 && (resp.statusCode ?? 0) < 300;
  }

  Future<int> getUnreadCount() async {
    final headers = _extraHeaders();
    if (headers == null) throw Exception('Unauthorized: missing JWT token');

    final resp = await _httpDio.get(
      '/api/v1/app/chat/unread',
      options: Options(validateStatus: (s) => true, headers: headers),
    );
    if ((resp.statusCode ?? 0) != 200) return 0;

    final rawData = resp.data;
    if (rawData is int) return rawData;
    final body = rawData?.toString().trim() ?? '';
    final direct = int.tryParse(body);
    if (direct != null) return direct;

    try {
      final dynamic decoded = rawData;
      if (decoded == null) return 0;
      if (decoded is int) return decoded;
      if (decoded is Map) {
        final total = decoded['totalUnreadCount'];
        if (total is num) return total.toInt();
        if (total is String) {
          final n = int.tryParse(total);
          if (n != null) return n;
        }
        for (final k in ['count', 'unreadCount', 'total', 'value']) {
          final v = decoded[k];
          if (v is num) return v.toInt();
          if (v is String) {
            final n = int.tryParse(v);
            if (n != null) return n;
          }
        }
        final data = decoded['data'];
        if (data is int) return data;
        if (data is Map) {
          final t2 = data['totalUnreadCount'];
          if (t2 is num) return t2.toInt();
          if (t2 is String) {
            final n = int.tryParse(t2);
            if (n != null) return n;
          }
          for (final k in [
            'unread',
            'count',
            'unreadCount',
            'total',
            'value',
          ]) {
            final v = data[k];
            if (v is num) return v.toInt();
            if (v is String) {
              final n = int.tryParse(v);
              if (n != null) return n;
            }
          }
        }
      }
    } catch (_) {}
    return 0;
  }

  Future<bool> markConversationAsRead(String conversationId) async {
    final headers = _extraHeaders();
    if (headers == null) throw Exception('Unauthorized: missing JWT token');

    final resp = await _httpDio.patch(
      '/api/v1/app/chat/conversations/$conversationId',
      options: Options(validateStatus: (s) => true, headers: headers),
    );
    return (resp.statusCode ?? 0) >= 200 && (resp.statusCode ?? 0) < 300;
  }

  Future<bool> markAllMessagesRead({required String participantId}) async {
    final headers = _extraHeaders();
    if (headers == null) throw Exception('Unauthorized: missing JWT token');

    if (kDebugMode) {
      HKLog.logD('[ChatRepository] PATCH /chat/conversations (read-all)');
    }
    final resp = await _httpDio.patch(
      '/api/v1/app/chat/conversations',
      data: {'targetId': participantId},
      options: Options(validateStatus: (s) => true, headers: headers),
    );
    return (resp.statusCode ?? 0) >= 200 && (resp.statusCode ?? 0) < 300;
  }

  @Deprecated('Use connect() instead')
  Stream<ChatMessage> connectAndSubscribe({required String accountId}) {
    connect();
    return onMessage;
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && hasChatScreenActive) {
      if (_connectionState == ChatConnectionState.disconnected ||
          _connectionState == ChatConnectionState.reconnecting) {
        HKLog.logD('[ChatRepository] App resumed, reconnecting WebSocket...');
        _reconnectAttempts = 0;
        connect();
      }
    }
  }

  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    disconnect();
    _messageController.close();
    _unreadCountController.close();
    _newMessageController.close();
    _connectionStateController.close();
  }
}

@Riverpod(keepAlive: true)
ChatRepository chatRepository(Ref ref) {
  final repo = ChatRepository(ref.read(dioProvider), ref, true);
  ref.onDispose(repo.dispose);
  return repo;
}

/// 傳訊失敗的 Exception，toString() 直接回傳 message（避免 "Exception: " 前綴）。
///
/// [code] 攜帶統一回應 envelope 的業務碼（見 [ChatRepository._extractErrorCode]），
/// 讓 call-site 層可識別檢舉／封鎖相關業務碼（backend-payuni#645，9202/9210）。
class ChatSendException implements Exception {
  const ChatSendException(this.message, {this.code = -1});
  final String message;
  final int code;
  @override
  String toString() => message;
}

/// ws-ticket 鑄票結果：單次票 + 後端推導出的 userId（與 web buildWsUrl 對齊）。
class _WsTicket {
  const _WsTicket(this.ticket, this.userId);
  final String ticket;
  final String userId;
}
