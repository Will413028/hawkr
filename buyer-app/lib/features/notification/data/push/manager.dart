import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../config/firebase_config.dart';
import '../../../../config/flavor.dart';
import '../../../../core/networking/connect_provider.dart';
import '../../../../core/routing/app_router.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/widgets/fcm_debug_overlay.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../presentation/notification_banner.dart';
import '../../presentation/notification_controller.dart';
import '../../../chat/data/active_chat_gate.dart';
import '../../../chat/data/chat_repository.dart';
import '../notification_repository.dart';
import '../state_refresh.dart';
import 'destination.dart';
import 'deep_link.dart';
import 'device_registrar.dart';
import 'router.dart';

/// 推播通知管理器（協調器 / SSOT）
///
/// 職責：
/// - 初始化 FCM
/// - 管理 FCM Token 生命週期
/// - 分發前台訊息（Overlay Banner）
/// - 記錄 Debug 訊息總線
/// - 委派導航至 [NotificationRouter]
class PushNotificationManager {
  static final PushNotificationManager _instance =
      PushNotificationManager._internal();
  factory PushNotificationManager() => _instance;
  PushNotificationManager._internal();

  static PushNotificationManager get instance => _instance;

  /// E2E / ui-verify 旗標：由 main.dart 在 `--dart-define=E2E_AUTH_TOKEN=...`
  /// 啟動時設為 true，本 manager 所有會觸發 iOS 通知權限 dialog 的入口
  /// （init / requestPermission / getToken / onUserLoggedIn）全部 no-op。
  static bool e2eDisabled = false;

  // ── 依賴 ──────────────────────────────────────────────────

  /// 第一次使用時才取 [FirebaseMessaging.instance]：Firebase 沒有設定時它會拋
  /// `core/no-app`，所有入口先以 [isFirebaseInitialized] 擋掉。
  late final FirebaseMessaging _messaging = FirebaseMessaging.instance;

  /// 以 Connect client 送出的裝置註冊（[injectContainer] 建立）；憑證由 transport 的 auth interceptor 帶。
  NotificationDeviceRegistrar? _registrar;
  ProviderContainer? _container;

  // ── 狀態 ──────────────────────────────────────────────────

  /// session 層通知的登入狀態（[onUserLoggedIn] / [onUserLoggedOut]）。manager 不持有 session token。
  bool _isUserLoggedIn = false;

  String? _fcmToken;

  /// 上次成功註冊裝置的時間，供 [onAppResumed] 節流用。
  DateTime? _lastRegisteredAt;

  /// 回前景重新註冊的節流間隔：24h 內已註冊過就跳過，避免每次 resume 都打後端。
  static const _resumeReregisterThrottle = Duration(hours: 24);

  /// 當前 FCM Token（init 完成後可讀）
  String? get fcmToken => _fcmToken;

  void injectContainer(ProviderContainer container) {
    _container = container;
    _registrar = NotificationDeviceRegistrar(
      container.read(notificationAppClientProvider),
    );
  }

  /// 測試接縫：本類是硬 singleton，`_fcmToken` 只能由 FirebaseMessaging 給、登入狀態只能由
  /// session 流程設定。
  @visibleForTesting
  void seedLifecycleStateForTest({
    String? fcmToken,
    bool isUserLoggedIn = false,
  }) {
    _fcmToken = fcmToken;
    _isUserLoggedIn = isUserLoggedIn;
    _lastRegisteredAt = null;
  }

  bool _initialized = false;

  /// 防止同時疊加多張前台 Banner
  OverlayEntry? _activeBannerEntry;

  /// Debug 訊息總線（僅非 prod，最多保留 50 條）
  final ValueNotifier<List<FcmDebugRecord>> debugMessagesBus = ValueNotifier(
    const [],
  );

  // ── FCM 訊息總線（供 UI 層監聽刷新） ─────────────────────
  final StreamController<RemoteMessage> _inboxController =
      StreamController<RemoteMessage>.broadcast();

  /// 收到任何 FCM 前景訊息 / 點擊後台通知時的廣播 stream。
  /// UI 層（通知頁、訂單頁等）訂閱後可即時刷新列表。
  Stream<RemoteMessage> get onInboxMessage => _inboxController.stream;

  // ── StreamSubscription ────────────────────────────────────

  StreamSubscription<RemoteMessage>? _onMessageSub;
  StreamSubscription<RemoteMessage>? _onMessageOpenedSub;
  StreamSubscription<String>? _onTokenRefreshSub;

  // ── 初始化 ────────────────────────────────────────────────

  /// 初始化推播通知，在 main.dart runApp() 之前呼叫
  Future<void> init({bool requestPermissionOnInit = true}) async {
    if (_initialized) return;
    if (e2eDisabled) {
      HKLog.logI('[FCM] E2E mode - skipping FCM init');
      return;
    }
    if (kIsWeb) {
      HKLog.logI('[FCM] Web platform - skipping FCM init');
      return;
    }
    if (!isFirebaseInitialized) {
      HKLog.logI('[FCM] Firebase 未設定 - skipping FCM init');
      return;
    }

    try {
      HKLog.logI('[FCM] 開始初始化');

      // iOS 前台通知顯示選項
      // alert:false → 前台時由 Flutter Overlay Banner 接管，抑制 iOS 系統通知
      // 後台/終止狀態下，OS 依 notification 欄位顯示系統通知（可見通知，不經背景 isolate）
      await _messaging.setForegroundNotificationPresentationOptions(
        alert: false,
        badge: true,
        sound: false,
      );

      // 設置監聽器
      _setupListeners();

      // 檢查冷啟動來源
      await _checkInitialLaunch();

      _initialized = true;
      HKLog.logI('[FCM] 初始化完成');

      if (requestPermissionOnInit) {
        // 呼叫端可延後到啟動過場結束後再詢問，避免權限視窗打斷品牌動畫。
        await requestPermission();

        // 取得 FCM Token（供 Debug Overlay 顯示及裝置註冊使用）
        await getToken();
      }
    } catch (e, s) {
      HKLog.logE('[FCM] 初始化失敗', e, s);
    }
  }

  // ── 監聽器 ────────────────────────────────────────────────

  void _setupListeners() {
    // Token 更新
    _onTokenRefreshSub = _messaging.onTokenRefresh.listen((newToken) {
      HKLog.logI('[FCM] Token 已更新');
      _fcmToken = newToken;
      _registerDevice(newToken);
    }, onError: (err) => HKLog.logE('[FCM] Token 更新錯誤', err));

    // 前台訊息
    _onMessageSub = FirebaseMessaging.onMessage.listen((m) {
      HKLog.logD('[FCM] onMessage: ${m.notification?.title}');

      // 過濾 Firebase Auth 靜默推播（Phone Auth 專用，由原生 SDK 處理）
      if (m.data.containsKey('com.google.firebase.auth')) return;

      _recordDebugMessage(m, source: 'onMessage');
      _handleForegroundMessage(m);
      _inboxController.add(m);
    });

    // 後台點擊（App 在後台時，用戶點擊通知將 App 帶回前台）
    _onMessageOpenedSub = FirebaseMessaging.onMessageOpenedApp.listen((m) {
      HKLog.logD('[FCM] onMessageOpenedApp: ${m.notification?.title}');
      _recordDebugMessage(m, source: 'onMessageOpenedApp');
      _handleMessageOpenedApp(m);
      _inboxController.add(m);
    });

    HKLog.logD('[FCM] 監聽器設置完成');
  }

  // ── 冷啟動檢查 ────────────────────────────────────────────

  Future<void> _checkInitialLaunch() async {
    final initialMessage = await _messaging.getInitialMessage();
    if (initialMessage != null) {
      HKLog.logI('[FCM] 從 FCM 通知冷啟動');
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _handleMessageOpenedApp(initialMessage);
      });
    }
  }

  // ── 權限 & Token ──────────────────────────────────────────

  /// 請求通知權限
  Future<AuthorizationStatus> requestPermission({
    bool provisional = false,
  }) async {
    if (!isFirebaseInitialized) return AuthorizationStatus.notDetermined;
    try {
      final settings = await _messaging.requestPermission(
        alert: true,
        announcement: false,
        badge: true,
        carPlay: false,
        criticalAlert: false,
        provisional: provisional,
        sound: true,
      );
      HKLog.logI('[FCM] 權限狀態: ${settings.authorizationStatus}');
      return settings.authorizationStatus;
    } catch (e, s) {
      HKLog.logE('[FCM] 請求通知權限失敗', e, s);
      return AuthorizationStatus.notDetermined;
    }
  }

  /// 獲取 FCM Token
  Future<String?> getToken() async {
    if (!isFirebaseInitialized) return null;
    try {
      if (!kIsWeb) {
        final apnsToken = await _messaging.getAPNSToken();
        HKLog.logI('[FCM] APNs Token: ${apnsToken ?? "NULL ⚠️"}');
        if (apnsToken == null) {
          HKLog.logE(
            '[FCM] APNs Token 為 null — 可能原因：'
            '模擬器不支援 / 通知權限未授予 / APNs 配置缺失',
          );
        }
      }
      final token = await _messaging.getToken();
      if (token != null) _fcmToken = token;
      HKLog.logI('[FCM] Token: $_fcmToken');
      return token;
    } catch (e, s) {
      HKLog.logE('[FCM] 獲取 Token 失敗', e, s);
      return null;
    }
  }

  // ── 裝置註冊 ──────────────────────────────────────────────

  /// 向後端註冊裝置 FCM Token
  ///
  /// 未登入時跳過，不拋出例外。
  Future<void> _registerDevice(String fcmToken) async {
    if (!_isUserLoggedIn) {
      HKLog.logW('[FCM] 無登入狀態，跳過裝置註冊');
      return;
    }
    final registrar = _registrar;
    if (registrar == null) {
      HKLog.logW('[FCM] 裝置註冊尚未初始化');
      return;
    }
    if (await registrar.registerDevice(fcmToken: fcmToken)) {
      _lastRegisteredAt = DateTime.now();
    }
  }

  // ── 訊息處理 ──────────────────────────────────────────────

  /// 前台訊息處理：顯示 Overlay Banner。標題、內文與圖片都是伺服器組好的（FCM `notification` 欄位），
  /// 不在這裡組字。
  void _handleForegroundMessage(RemoteMessage message) {
    // 狀態重讀提示不依賴有沒有可見文字，先叫訂單、售後畫面重讀。
    dispatchStateRefreshHints(stateRefreshHintsFromPush(message.data));
    if (_isViewingChatOf(message.data)) {
      HKLog.logD(
        '[FCM] chat already open for this notification, banner skipped',
      );
      return;
    }
    final notification = message.notification;
    final title = notification?.title ?? '';
    final body = notification?.body ?? '';
    if (title.isEmpty && body.isEmpty) {
      HKLog.logW('[FCM] 前台訊息 title/body 均為空，略過 Banner');
      return;
    }
    _showBanner(
      title: title,
      body: body,
      imageUrl: notification?.apple?.imageUrl,
      data: message.data,
      messageId: message.messageId,
    );
  }

  /// 深連結指向的聊天室就是使用者正開著的那一間。
  bool _isViewingChatOf(Map<String, dynamic> data) {
    final container = _container;
    if (container == null) return false;
    final targetId = chatTargetOfDeepLink(
      data[notificationDeepLinkKey]?.toString() ?? '',
    );
    if (targetId == null) return false;
    final repo = container.read(chatRepositoryProvider);
    return activeChatIsStore(
      container.read(activeChatGateProvider),
      targetId,
      (conversationId) => repo.storeCards[conversationId]?.id,
    );
  }

  /// 點通知（banner、系統通知）：有 `inbox_item_id` 就標已讀，再導覽。
  void _onNotificationTapped(Map<String, dynamic> data, String? messageId) {
    final container = _container;
    if (container != null && _isUserLoggedIn) {
      unawaited(
        markTappedInboxItemRead(data, (id) async {
          await container.read(notificationRepositoryProvider).markRead(id);
          await container
              .read(notificationListControllerProvider.notifier)
              .silentRefresh();
        }),
      );
    }
    NotificationRouter.navigate(data, messageId: messageId);
  }

  /// 後台點擊 / 冷啟動訊息處理
  void _handleMessageOpenedApp(RemoteMessage message) {
    // 非 prod 下，含 "Test" 關鍵字的訊息直接彈 Dialog 展示 payload
    if (F.appFlavor != Flavor.prod && _isTestMessage(message)) {
      _showTestDialog(message);
      return;
    }
    _onNotificationTapped(message.data, message.messageId);
  }

  // ── Overlay Banner ────────────────────────────────────────

  /// 顯示 App 內前台通知 Banner（同時只顯示一張，舊的自動替換）
  void _showBanner({
    required String title,
    required String body,
    required Map<String, dynamic> data,
    String? imageUrl,
    String? messageId,
  }) {
    final overlay = rootNavigatorKey.currentState?.overlay;
    HKLog.logD('[FCM] _showBanner overlay=${overlay != null} title="$title"');
    if (overlay == null) {
      HKLog.logW('[FCM] overlay 為 null，Banner 無法顯示');
      return;
    }

    if (_activeBannerEntry?.mounted == true) {
      _activeBannerEntry!.remove();
    }
    _activeBannerEntry = null;

    late OverlayEntry entry;

    void removeSafely() {
      if (entry.mounted) {
        entry.remove();
        if (_activeBannerEntry == entry) _activeBannerEntry = null;
      }
    }

    entry = OverlayEntry(
      builder: (_) => NotificationBanner(
        title: title,
        body: body,
        imageUrl: imageUrl,
        onTap: () {
          removeSafely();
          _onNotificationTapped(data, messageId);
        },
        onClose: removeSafely,
      ),
    );

    _activeBannerEntry = entry;
    overlay.insert(entry);

    Future.delayed(const Duration(seconds: 5), removeSafely);
  }

  // ── Debug ─────────────────────────────────────────────────

  /// 記錄訊息到 debug bus（僅非 prod，最多保留 50 條）
  void _recordDebugMessage(RemoteMessage message, {required String source}) {
    if (F.appFlavor == Flavor.prod) return;
    final record = FcmDebugRecord(
      receivedAt: DateTime.now(),
      messageId: message.messageId,
      fcmTitle: message.notification?.title,
      fcmBody: message.notification?.body,
      data: Map<String, dynamic>.from(message.data),
      source: source,
    );
    final updated = [record, ...debugMessagesBus.value].take(50).toList();
    debugMessagesBus.value = updated;
  }

  /// 判斷是否為 Test 訊息（title / body / data 任一含 "Test"）
  bool _isTestMessage(RemoteMessage message) {
    if (message.notification?.title?.contains('Test') == true) return true;
    if (message.notification?.body?.contains('Test') == true) return true;
    return message.data.values.any(
      (v) => v?.toString().contains('Test') == true,
    );
  }

  /// 顯示 Test 訊息 Dialog（僅非 prod）
  void _showTestDialog(RemoteMessage message) {
    final context = rootNavigatorKey.currentContext;
    if (context == null) return;

    final title = message.notification?.title ?? '(無標題)';
    final body = message.notification?.body ?? '';
    final data = message.data;

    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: AppColors.textEmphasis,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: const Row(
          children: [
            Icon(
              AppIcons.scienceOutlined,
              color: AppIconColors.secondary,
              size: 20,
            ),
            SizedBox(width: 8),
            Text(
              'FCM Test Message',
              style: TextStyle(
                color: AppColors.bgCard,
                fontSize: 16,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
        content: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              if (title != '(無標題)' || body.isNotEmpty) ...[
                _dialogSection('Notification'),
                if (title != '(無標題)')
                  _dialogRow('title', title, highlight: true),
                if (body.isNotEmpty) _dialogRow('body', body),
                const SizedBox(height: 12),
              ],
              if (data.isNotEmpty) ...[
                _dialogSection('Data Payload'),
                ...data.entries.map(
                  (e) => _dialogRow(e.key, e.value?.toString() ?? 'null'),
                ),
              ],
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const HkButtonLabel(
              '關閉',
              style: TextStyle(color: AppColors.primary),
            ),
          ),
        ],
      ),
    );
  }

  Widget _dialogSection(String label) => Padding(
    padding: const EdgeInsets.only(bottom: 8),
    child: Text(
      '── $label ──────────────',
      style: const TextStyle(
        color: AppColors.textPrimary,
        fontSize: 12,
        fontFamily: 'monospace',
      ),
    ),
  );

  Widget _dialogRow(String key, String value, {bool highlight = false}) =>
      Padding(
        padding: const EdgeInsets.only(bottom: 4),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: 100,
              child: Text(
                key,
                style: const TextStyle(
                  color: AppColors.info,
                  fontSize: 12,
                  fontFamily: 'monospace',
                ),
              ),
            ),
            const Text(
              ': ',
              style: TextStyle(color: AppColors.textPrimary, fontSize: 12),
            ),
            Expanded(
              child: Text(
                value,
                style: TextStyle(
                  color: highlight ? AppColors.warning : AppColors.gray300,
                  fontSize: 12,
                  fontFamily: 'monospace',
                  fontWeight: highlight ? FontWeight.w700 : FontWeight.normal,
                ),
              ),
            ),
          ],
        ),
      );

  // ── 生命週期 ──────────────────────────────────────────────

  /// 用戶登入後呼叫：取得 Token 並向後端註冊裝置
  Future<void> onUserLoggedIn() async {
    if (e2eDisabled) return;
    _isUserLoggedIn = true;
    final token = await getToken();
    if (token != null) await _registerDevice(token);
  }

  /// App 回前景時呼叫：節流地重新註冊裝置，讓後端 device_tokens.updated_at 反映
  /// 「最近有在用 App」（供後端 staleness reaper 判斷活躍度）。冷啟動已在 session
  /// 還原時註冊；此處補「溫啟動回前景」這個缺口（app 未被 OS 殺、僅切到背景再回來）。
  /// 未登入則跳過（登入流程會註冊）；24h 內已註冊過則節流跳過。
  Future<void> onAppResumed() async {
    if (!_isUserLoggedIn) return;
    final last = _lastRegisteredAt;
    if (last != null &&
        DateTime.now().difference(last) < _resumeReregisterThrottle) {
      return;
    }
    final token = _fcmToken ?? await getToken();
    if (token != null) await _registerDevice(token);
  }

  /// 用戶登出前呼叫（session 還在，註銷請求才帶得出憑證）：移除後端裝置記錄
  ///
  /// 沒有登入就沒有可移除的裝置記錄——後端是以帳號為鍵刪除的，未帶憑證只會拿到 401，所以不送出。
  Future<void> onUserLoggedOut() async {
    if (!_isUserLoggedIn) {
      HKLog.logW(
        '[PushNotificationManager] onUserLoggedOut skipped: not logged in',
      );
      return;
    }
    _isUserLoggedIn = false;
    await _registrar?.removeDevice(fcmToken: _fcmToken ?? '');
  }

  /// 清理資源
  void dispose() {
    _onMessageSub?.cancel();
    _onMessageOpenedSub?.cancel();
    _onTokenRefreshSub?.cancel();
    _inboxController.close();
    if (_activeBannerEntry?.mounted == true) _activeBannerEntry!.remove();
    _activeBannerEntry = null;
    _initialized = false;
  }
}
