import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app.dart';
import '../../../../config/firebase_config.dart';
import '../../../../config/flavor.dart';
import '../../../../core/networking/connect_provider.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/utils/secret_redaction.dart';
import '../../../../shared/widgets/fcm_debug_overlay.dart';
import '../../../chat/data/active_chat_gate.dart';
import '../deep_link.dart';
import '../../../dashboard/data/dashboard_notification_unread_provider.dart';
import '../../presentation/notification_controller.dart';
import '../state_refresh.dart';
import '../../presentation/notification_banner.dart';
import 'apns_token_wait.dart';
import 'device_registrar.dart';

/// 推播通知管理器（協調器 / SSOT）
///
/// 職責：
/// - 初始化 FCM
/// - 管理 FCM Token 生命週期與裝置註冊
/// - 前台可見推播（FCM `notification` 欄位）：Overlay Banner
/// - 點擊推播：交給伺服器給的 `deep_link`（[NotificationDeepLink]）
/// - 記錄 Debug 訊息總線
///
/// 推播一律是可見通知（APNs alert）；沒有 `notification` 欄位的訊息不顯示。狀態重讀由通知串流
/// （`NotificationSignalService`）負責，這裡不依通知型別分支。
class PushNotificationManager {
  static final PushNotificationManager _instance =
      PushNotificationManager._internal();
  factory PushNotificationManager() => _instance;
  PushNotificationManager._internal();

  static PushNotificationManager get instance => _instance;

  /// E2E / ui-verify 旗標：由 main.dart 在 `--dart-define=E2E_AUTH_TOKEN=...`
  /// 啟動時設為 true，本 manager 所有會觸發 iOS 通知權限 dialog 的入口全部 no-op。
  static bool e2eDisabled = false;

  // ── Provider container (injected from main.dart) ────────────
  ProviderContainer? _container;

  /// main.dart 在 runApp 前注入，讓 manager 能讀 activeChatGateProvider
  /// 並以中央 Dio 建立受保護的裝置註冊 gateway。
  void injectContainer(ProviderContainer container) {
    _container = container;
    _registration = DeviceRegistrationCoordinator(
      gateway: NotificationDeviceRegistrar(
        container.read(notificationClientProvider),
      ),
    );
  }

  // ── 依賴 ──────────────────────────────────────────────────

  /// 第一次使用時才取 [FirebaseMessaging.instance]：Firebase 沒有設定時它會拋
  /// `core/no-app`，所有入口先以 [isFirebaseInitialized] 擋掉。
  late final FirebaseMessaging _messaging = FirebaseMessaging.instance;

  // ── 狀態 ──────────────────────────────────────────────────

  String? _fcmToken;
  DeviceRegistrationCoordinator? _registration;
  bool _isUserLoggedIn = false;

  /// 回前景重新註冊的節流間隔：同一 token 成功後 24h 內跳過。
  /// 失敗與 token rotation 都不受此節流。
  static const _resumeReregisterThrottle = Duration(hours: 24);

  /// 登出時等待裝置註銷的上限。
  ///
  /// 註銷是伺服器端的盡力清理，不是登出的前置條件：逾時後請求（含 bounded
  /// retry）仍在背景跑完，只是不再擋著使用者。沒有這個上限時，單次請求可耗掉
  /// `apiTimeout`（prod 15s）、最壞連重試共 ~46s，全都是使用者盯著登出鍵發呆
  /// 的時間。
  ///
  /// 為什麼不是「完全不等」：中央 Dio 的 auth interceptor 是在 Dio 排定的
  /// event-loop turn 才去讀 session 取 token；若不等就清空 session，那個 DELETE
  /// 會裸奔成 401，正常登出反而註銷不掉。等待本身就是「請求已帶著憑證送出」
  /// 的保證，而 1 秒足以讓健康網路下的整趟往返（實測 ~200ms）照常走完。
  static const Duration logoutDeregistrationDeadline = Duration(seconds: 1);

  @visibleForTesting
  void seedContainerForTest(ProviderContainer? container) =>
      _container = container;

  @visibleForTesting
  void handleForegroundMessageForTest(RemoteMessage message) =>
      _handleForegroundMessage(message);

  @visibleForTesting
  void handleMessageOpenedAppForTest(RemoteMessage message) =>
      _handleMessageOpenedApp(message);

  /// 測試接縫：本類是硬 singleton，`_fcmToken` 只能由 FirebaseMessaging 給、
  /// `_registration` 只能由 [injectContainer] 以真 Dio 建立。沒有這個接縫，
  /// 「無憑證 / 未登入時不得送出裝置註銷」這道守衛（#106 迴圈的唯一防線）
  /// 就寫不出回歸測試。
  @visibleForTesting
  void seedLifecycleStateForTest({
    DeviceRegistrationCoordinator? registration,
    String? fcmToken,
    bool isUserLoggedIn = false,
  }) {
    _registration = registration;
    _fcmToken = fcmToken;
    _isUserLoggedIn = isUserLoggedIn;
  }

  /// 當前 FCM Token（init 完成後可讀）
  String? get fcmToken => _fcmToken;

  bool _initialized = false;

  /// 防止同時疊加多張前台 Banner
  OverlayEntry? _activeBannerEntry;

  /// Debug 訊息總線（僅非 prod，最多保留 50 條）
  final ValueNotifier<List<FcmDebugRecord>> debugMessagesBus = ValueNotifier(
    const [],
  );

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

      // 步驟 1：iOS 前台通知顯示選項
      // alert:false → 前台時由 Flutter Overlay Banner 接管，抑制 iOS 系統通知
      // 後台/終止狀態下，OS 依 notification 欄位顯示系統通知
      await _messaging.setForegroundNotificationPresentationOptions(
        alert: false,
        badge: true,
        sound: false,
      );

      // 步驟 2：設置監聽器
      _setupListeners();

      // 步驟 3：檢查冷啟動來源
      await _checkInitialLaunch();

      _initialized = true;
      HKLog.logI('[FCM] 初始化完成');

      if (requestPermissionOnInit) {
        // 非啟動流程可維持既有行為；App 冷啟動會在品牌動畫退場後才詢問，
        // 避免系統權限視窗蓋住 Hawkr 載入動畫。
        await requestPermission();
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
      unawaited(_registerDevice(newToken));
    }, onError: (err) => HKLog.logE('[FCM] Token 更新錯誤', err));

    // 前台訊息
    _onMessageSub = FirebaseMessaging.onMessage.listen((m) {
      HKLog.logD('[FCM] onMessage: ${m.notification?.title}');

      // 過濾 Firebase Auth 靜默推播（Phone Auth 專用，由原生 SDK 處理）
      if (m.data.containsKey('com.google.firebase.auth')) return;

      _recordDebugMessage(m, source: 'onMessage');
      _handleForegroundMessage(m);
    });

    // 背景點擊通知返回 App（App in background，非冷啟動）
    _onMessageOpenedSub = FirebaseMessaging.onMessageOpenedApp.listen((m) {
      HKLog.logD('[FCM] onMessageOpenedApp: ${m.messageId}');
      _recordDebugMessage(m, source: 'onMessageOpenedApp');
      _handleMessageOpenedApp(m);
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
        final apnsToken = await waitForApnsToken(_messaging.getAPNSToken);
        if (apnsToken == null) {
          HKLog.logW('[FCM] APNs Token 逾時未就緒，略過本次 getToken()');
          return null;
        }
      }
      final token = await _messaging.getToken();
      if (token != null) _fcmToken = token;
      HKLog.logI('[FCM] Token 已取得');
      return token;
    } catch (e, s) {
      HKLog.logE('[FCM] 獲取 Token 失敗', e, s);
      return null;
    }
  }

  // ── 裝置註冊 ──────────────────────────────────────────────

  /// 向後端註冊裝置 FCM Token
  ///
  /// 未登入時跳過，不拋出例外。相同 token 的並行呼叫由 coordinator 合併，
  /// retry 僅限 bounded transient backoff；只有成功才更新 24h 節流狀態。
  Future<DeviceMutationOutcome> _registerDevice(String fcmToken) async {
    if (!_isUserLoggedIn) {
      HKLog.logW('[FCM] 無登入狀態，跳過裝置註冊');
      return const DeviceMutationOutcome.skipped();
    }
    final registration = _registration;
    if (registration == null) {
      HKLog.logW('[FCM] 裝置註冊尚未初始化');
      return const DeviceMutationOutcome.skipped();
    }
    return registration.register(fcmToken);
  }

  // ── 訊息處理 ──────────────────────────────────────────────

  /// 前台訊息處理：顯示 Overlay Banner。文字、圖片與深連結都是伺服器給的；沒有 `notification` 欄位的訊息不顯示。
  void _handleForegroundMessage(RemoteMessage message) {
    // 串流不通時前景推播是唯一的訊號：先依 `state_refresh_hints` 重讀（和串流同一份對照），不論要不要顯示 banner。
    final container = _container;
    if (container != null) {
      container
          .read(stateRefreshActionsProvider)
          .applyInbox(
            parseStateRefreshHints(
              message.data['state_refresh_hints']?.toString(),
            ),
          );
    }
    final notification = message.notification;
    if (notification == null) return;
    final title = notification.title ?? '';
    final body = notification.body ?? '';
    if (title.isEmpty && body.isEmpty) return;

    final link = NotificationDeepLink.parse(
      message.data['deep_link']?.toString(),
    );

    // 使用者已經在那個聊天室裡：不再彈前景 banner，避免雙重通知。
    final activeChat = _container?.read(activeChatGateProvider);
    if (activeChat != null && link.isOpenIn(activeChat)) {
      HKLog.logD('[FCM] 跳過 banner：使用者已在該聊天室');
      return;
    }

    _showBanner(
      title: title,
      body: body,
      imageUrl: notification.apple?.imageUrl,
      link: link,
      inboxItemId: message.data['inbox_item_id']?.toString(),
    );
  }

  /// 點擊推播：把對應的收件匣項目標為已讀（`inbox_item_id`）；失敗由 `markAsRead` 吞掉，不影響導航。
  void _markInboxItemRead(String? inboxItemId) {
    final container = _container;
    if (container == null || inboxItemId == null || inboxItemId.isEmpty) {
      return;
    }
    unawaited(
      container
          .read(notificationListControllerProvider.notifier)
          .markAsRead(inboxItemId)
          .then(
            (_) => container.invalidate(dashboardNotificationUnreadProvider),
          ),
    );
  }

  /// 後台點擊 / 冷啟動訊息處理
  void _handleMessageOpenedApp(RemoteMessage message) {
    // 非 prod 下，含 "Test" 關鍵字的訊息直接彈 Dialog 展示 payload
    if (F.appFlavor != Flavor.prod && _isTestMessage(message)) {
      HKLog.logD('[FCM] Test message → debug dialog');
      _showTestDialog(message);
      return;
    }
    _markInboxItemRead(message.data['inbox_item_id']?.toString());
    NotificationDeepLink.parse(message.data['deep_link']?.toString()).open();
  }

  // ── Overlay Banner ────────────────────────────────────────

  /// 顯示 App 內前台通知 Banner（同時只顯示一張，舊的自動替換）
  void _showBanner({
    required String title,
    required String body,
    required NotificationDeepLink link,
    String? inboxItemId,
    String? imageUrl,
  }) {
    final overlay = MyApp.navigatorKey.currentState?.overlay;
    if (overlay == null) return;

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
          _markInboxItemRead(inboxItemId);
          link.open();
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
      data: Map<String, dynamic>.from(redactSensitiveData(message.data) as Map),
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
    final context = MyApp.navigatorKey.currentContext;
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
    final token = _fcmToken ?? await getToken();
    final registration = _registration;
    if (token == null || registration == null) return;
    if (registration.shouldThrottle(
      token,
      throttle: _resumeReregisterThrottle,
    )) {
      return;
    }
    await _registerDevice(token);
  }

  /// 用戶登出前呼叫：移除後端裝置記錄
  ///
  /// [hasCredential] 由呼叫端（session 層）給——manager 不持有 session，
  /// 也刻意不去拿全域 container 讀 token。沒有憑證時那個 DELETE 注定 401，
  /// 送出去只是白打一次後端，直接跳過。
  Future<void> onUserLoggedOut({required bool hasCredential}) async {
    final registration = _registration;
    final token = _fcmToken;
    try {
      if (registration != null) {
        if (_isUserLoggedIn && hasCredential && token != null) {
          await _deregisterWithinDeadline(registration, token);
        } else {
          HKLog.logD('[FCM] 跳過裝置註銷（未登入或無有效憑證）');
        }
        // 只重置註冊側狀態；in-flight 的註銷不受影響，逾時後仍在背景跑完。
        registration.reset();
      }
    } finally {
      // 不論註銷成敗都要清：留著 token 會讓下一次登出又送一次注定失敗的
      // DELETE，也讓 registrar 的空 token 保護永遠不生效。
      _isUserLoggedIn = false;
      _fcmToken = null;
    }
  }

  /// 送出裝置註銷，但最多擋住登出 [logoutDeregistrationDeadline]。
  ///
  /// 逾時不是取消：`remove` 的 future 繼續跑（含 bounded retry），只是不再被
  /// 等待。註銷結果對上層沒有可執行的分支——登出無論如何都必須完成——所以這裡
  /// 不把 outcome 往上傳，改成把「為什麼沒成功」記到 log 供事後對帳。
  Future<void> _deregisterWithinDeadline(
    DeviceRegistrationCoordinator registration,
    String token,
  ) async {
    final outcome = await registration
        .remove(token)
        .timeout(
          logoutDeregistrationDeadline,
          onTimeout: () {
            HKLog.logW(
              '[FCM] 裝置註銷未在 ${logoutDeregistrationDeadline.inMilliseconds}ms '
              '內完成，轉背景續跑，不擋登出',
            );
            return const DeviceMutationOutcome.skipped();
          },
        );
    if (!outcome.isSuccess) {
      HKLog.logW('[FCM] 裝置註銷未確認成功（disposition=${outcome.disposition.name}）');
    }
  }

  /// 清理資源
  void dispose() {
    _onMessageSub?.cancel();
    _onMessageOpenedSub?.cancel();
    _onTokenRefreshSub?.cancel();
    if (_activeBannerEntry?.mounted == true) _activeBannerEntry!.remove();
    _activeBannerEntry = null;
    _initialized = false;
  }
}
