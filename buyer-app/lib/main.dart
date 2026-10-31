import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';
import 'dart:convert';

import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

import 'config/firebase_config.dart';
import 'config/flavor.dart';
import 'core/observability/sentry_scrub.dart';
import 'core/analytics/analytics_identity.dart';
import 'core/analytics/analytics_providers.dart';
import 'core/layout/app_media_query_policy.dart';
import 'core/layout/app_text_size.dart';
import 'core/networking/app_version_headers.dart';
import 'core/routing/app_router.dart';
import 'core/routing/buyer_navigation_observer.dart';
import 'core/session/auth_session_provider.dart';
import 'package:hawkr_auth/logging.dart';
import 'core/session/force_logout_listener.dart';
import 'core/session/session_provider.dart';
import 'core/theme/app_colors.dart';
import 'core/theme/app_theme.dart';
import 'features/app_update/application/force_update_provider.dart';
import 'features/chat/data/active_chat_gate.dart';
import 'features/chat/data/chat_model.dart';
import 'features/chat/data/chat_repository.dart';
import 'features/notification/data/push/background_handler.dart';
import 'features/notification/data/push/manager.dart';
import 'features/notification/data/push/router.dart';
import 'shared/utils/hk_log.dart';
import 'shared/widgets/chat_bubble_icon.dart';
import 'shared/widgets/brand_startup_splash.dart';
import 'shared/widgets/keyboard_dismiss_scope.dart';
import 'shared/widgets/buyer_navigation_frame.dart';
import 'shared/widgets/environment_banner.dart';
import 'shared/widgets/fcm_debug_overlay.dart';
import 'shared/widgets/app_cached_image.dart';
import 'config/image_config.dart';

/// 讀取 flavor：native 用 FLUTTER_APP_FLAVOR（由 --flavor 自動設定）
const String _flavorFromEnv = String.fromEnvironment(
  'FLUTTER_APP_FLAVOR',
  defaultValue: String.fromEnvironment('APP_FLAVOR', defaultValue: 'dev'),
);

/// 解析 --flavor 對應的 [Flavor]，main() 與 _bootstrapApplication 共用同一份邏輯。
Flavor _resolveFlavorFromEnv() => Flavor.values.firstWhere(
  (e) => e.name == _flavorFromEnv,
  orElse: () => Flavor.dev,
);

/// App Store screenshot mode avoids transient OS prompts/overlays that should
/// never appear in marketing screenshots.
const bool _appStoreScreenshotMode = bool.fromEnvironment(
  'APP_STORE_SCREENSHOT',
  defaultValue: false,
);

/// ui-verify / E2E 自動測試模式：跳過會出系統權限 dialog 的初始化（推播權限等）。
/// 觸發條件：`--dart-define=E2E_AUTH_TOKEN=...`（[kE2EAuthToken]）或 `--dart-define=E2E_SUPPRESS_PROMPTS=true`。
const bool _e2eSuppressPrompts = bool.fromEnvironment(
  'E2E_SUPPRESS_PROMPTS',
  defaultValue: false,
);
bool get _e2eMode => _e2eSuppressPrompts || kE2EAuthToken.isNotEmpty;

/// 初始化 Firebase Messaging（在 runApp 之前呼叫，確保監聽器提前就緒）
Future<void> _initializeFirebaseMessaging() async {
  try {
    await PushNotificationManager.instance.init(requestPermissionOnInit: false);
    HKLog.logI('Firebase Messaging 初始化成功');
  } catch (e, stackTrace) {
    HKLog.logE('Firebase Messaging 初始化失敗', e, stackTrace);
    // Messaging 失敗不阻止 app 運行
  }
}

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  // 禁止 GoogleFonts 於執行時抓遠端字型，避免桌面環境封網導致閃退
  GoogleFonts.config.allowRuntimeFetching = false;

  final flavor = _resolveFlavorFromEnv();
  // API 主機是必要設定：stage 與 prod 缺少 GATEWAY_HOST 時在 runApp 前直接失敗。
  flavor.requireCoreConfig();

  SentryFlutter.init(
    (options) {
      options.dsn = flavor.sentryDsn;
      options.environment = flavor.name;
      options.tracesSampleRate = 0; // error-only rollout；不開 performance tracing
      options.sendDefaultPii = false;
      options.beforeSend = (event, hint) => scrubSentryEvent(event, hint);
    },
    appRunner: () => runApp(
      BrandStartupGate(
        bootstrap: _bootstrapApplication,
        onComplete: _finishNotificationStartup,
      ),
    ),
  );
}

void _finishNotificationStartup() {
  if (_appStoreScreenshotMode || _e2eMode) return;
  unawaited(_requestNotificationPermissionAndToken());
}

Future<void> _requestNotificationPermissionAndToken() async {
  final manager = PushNotificationManager.instance;
  await manager.requestPermission();
  await manager.getToken();
}

Future<Widget> _bootstrapApplication() async {
  // ============ E2E auth inject（debug build only）============
  // `--dart-define=E2E_AUTH_TOKEN=<token>` 以固定的 Bearer 取代 identity 的 session token（見
  // auth_session_provider.dart），GetSession 讀取對應的帳號。release build 不讀它。
  if (kDebugMode && _e2eMode) {
    PushNotificationManager.e2eDisabled = true;
  }
  if (e2eAuthEnabled) HKLog.logI('[E2E] auth token injected → bypass OAuth');

  // ============ Flavor 配置 ============
  // 優先使用程式碼設定的 Flavor（如測試時直接指定），
  // 若未設定則從 --flavor 參數（FLUTTER_APP_FLAVOR）讀取
  if (!F.isInitialized) {
    F.init(_resolveFlavorFromEnv());
  }

  HKLog.logI('[Main] Flavor: ${F.appFlavor.name}');
  HKLog.logI('[Main] Gateway: ${F.appFlavor.gatewayHost}');
  HKLog.logI('[Main] API Base: ${F.appFlavor.apiBaseUrl}');

  // 建立共享 container — 必須在 PushNotificationManager.init() 之前，
  // 讓 manager 可注入後在 _handleForegroundMessage 讀 activeChatGateProvider。
  final container = ProviderContainer(
    overrides: [
      authLoggerProvider.overrideWithValue((level, message, error, stack) {
        if (level == AuthLogLevel.error) {
          HKLog.logE(message, error, stack);
        } else {
          HKLog.logW(message, error, stack);
        }
      }),
    ],
  );
  await container.read(appTextSizeProvider.notifier).restore();
  NotificationRouter.configureSessionReadiness(
    () => !container.read(sessionProvider).isLoading,
  );

  // ============ App 版本 gate（`packages/hawkr_app_update/docs/spec.md`）============
  // 位置刻意排在 bootstrap 最前段：此處早於推播初始化，`getInitialMessage()`
  // 的導航還沒 arm，不需要另建抑制機制。
  //
  // ⚠️ 例外必須在此自行吃掉：`BrandStartupGate._runBootstrap` 的 catch 只
  // reportError、不 setState、不解除 splash（brand_startup_splash.dart:75-84），
  // 例外一旦冒上去就是**永遠卡在啟動畫面**。這是 fail-open 的落地點。
  try {
    await AppVersionHeaders.initialize();
    await container.read(forceUpdateProvider.notifier).check();
  } catch (e, s) {
    HKLog.logE('[Main] App 版本檢查失敗，fail-open 繼續啟動', e, s);
  }

  // ============ Firebase 初始化 ============
  // 沒有設定（缺 FIREBASE_* define）時整體停用：不初始化、不註冊推播與分析，
  // App 仍可啟動與登入。熱重啟的 duplicate-app 由 initializeFirebase 處理。
  final firebaseReady = await initializeFirebase(
    options: F.appFlavor.firebaseOptions,
    appName: F.appFlavor.firebaseAppName,
  );
  if (firebaseReady) {
    HKLog.logI(
      '[Main] Firebase initialized - Project: ${Firebase.app().options.projectId}',
    );
  } else {
    HKLog.logW('[Main] Firebase 未設定，推播與分析停用');
  }

  // 裝置註冊走 container 的中央 Dio，session 還原（下面的 sessionProvider.future）之前就要注入。
  PushNotificationManager.instance.injectContainer(container);

  if (!_appStoreScreenshotMode && !_e2eMode && isFirebaseInitialized) {
    // 註冊背景推播處理（須在 Firebase 初始化之後、runApp 之前呼叫）
    FirebaseMessaging.onBackgroundMessage(firebaseMessagingBackgroundHandler);

    // 初始化推播通知管理器（在 bootstrap 內完成，確保監聽器在 UI 就緒前已註冊）
    await _initializeFirebaseMessaging();
  }

  // Session restoration belongs to the single branded startup gate. Waiting
  // here prevents a second welcome/loading screen and avoids a guest-state
  // flash before the first real application frame.
  await container.read(sessionProvider.future);

  // ============ Analytics collection gate ============
  // prod + dev emit (each flavor's Firebase project routes to its own GA4
  // property, so dev never pollutes prod); stage stays off. Reuse the single
  // predicate so this SDK master switch and analyticsServiceProvider agree.
  if (isFirebaseInitialized) {
    await FirebaseAnalytics.instance.setAnalyticsCollectionEnabled(
      analyticsEnabledForFlavor(F.appFlavor),
    );
  }

  SystemChrome.setSystemUIOverlayStyle(AppColors.topBarOverlay);
  return UncontrolledProviderScope(container: container, child: const MyApp());
}

class MyApp extends ConsumerStatefulWidget {
  const MyApp({super.key});

  // 全域 keys，用於在任何地方顯示 toast
  static GlobalKey<NavigatorState> get navigatorKey => rootNavigatorKey;
  static final GlobalKey<ScaffoldMessengerState> scaffoldMessengerKey =
      GlobalKey<ScaffoldMessengerState>();

  @override
  ConsumerState<MyApp> createState() => _MyAppState();
}

class _MyAppState extends ConsumerState<MyApp> with WidgetsBindingObserver {
  StreamSubscription<ChatMessage>? _messageSub;
  OverlayEntry? _activeToast;

  @override
  void initState() {
    super.initState();
    // App 回前景時節流重新註冊 device token（讓後端 updated_at 反映活躍度，供 staleness reaper）。
    WidgetsBinding.instance.addObserver(this);
    registerAnalyticsIdentity(ref);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      NotificationRouter.flushPending();
    });
    // 監聽 WebSocket 新訊息（完整 ChatMessage）
    // Eagerly create chatRepository (registers WidgetsBindingObserver via constructor).
    _messageSub = ref
        .read(chatRepositoryProvider)
        .onMessage
        .listen(_onNewChatMessage);
  }

  @override
  void dispose() {
    _messageSub?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      NotificationRouter.flushPending();
      PushNotificationManager.instance.onAppResumed();
      // 版本 gate 前景恢復重檢（`packages/hawkr_app_update/docs/spec.md`）；節流錨點是「上次成功
      // 檢查」，門檻 10 分鐘，判斷在 provider 內，失敗 fail-open。
      unawaited(ref.read(forceUpdateProvider.notifier).checkThrottled());
    }
  }

  void _onNewChatMessage(ChatMessage msg) {
    // 只通知「收到的訊息」，自己發的不通知
    // senderId == 對方才觸發通知
    if (msg.senderId.isEmpty) return;
    // 如果使用者正在跟這個人聊天，不彈通知。
    // 雙重比對 senderId / conversationId — merchantId 與 senderId 可能因 namespace
    // 不同對不上（例：merchantId 為 shop_id，senderId 為 account_id），conversationId
    // 是 server 端唯一識別，比對最穩。
    final gate = ref.read(activeChatGateProvider);
    if (gate.peerId.isNotEmpty && gate.peerId == msg.senderId) return;
    final convId = msg.conversationId ?? '';
    if (convId.isNotEmpty && gate.conversationId == convId) return;

    // 震動反饋
    HapticFeedback.mediumImpact();

    // 顯示頂部滑入式通知橫幅
    _showChatBanner(msg);
  }

  void _showChatBanner(ChatMessage msg) {
    final overlay = MyApp.navigatorKey.currentState?.overlay;
    if (overlay == null) return;

    // 移除上一個通知
    _activeToast?.remove();

    // 解析訊息預覽文字
    String preview = msg.messageContent;
    if (preview.startsWith('{') && preview.endsWith('}')) {
      try {
        final m = Map<String, dynamic>.from(
          const JsonCodec().decode(preview) as Map,
        );
        final t = (m['text'] ?? m['content'] ?? '').toString();
        if (t.isNotEmpty) preview = t;
        final type = (m['messageType'] ?? m['type'] ?? '').toString();
        if (type == 'image') preview = '[圖片]';
        if (type == 'product') preview = '[商品]';
      } catch (_) {}
    }
    if (preview.length > 40) preview = '${preview.substring(0, 40)}...';

    late OverlayEntry entry;
    entry = OverlayEntry(
      builder: (context) {
        final top = MediaQuery.of(context).padding.top;
        return _ChatNotificationBanner(
          senderName: msg.senderName ?? '',
          senderAvatar: msg.senderAvatar,
          senderId: msg.senderId,
          preview: preview,
          topPadding: top,
          onTap: () {
            entry.remove();
            _activeToast = null;
            // 導航到聊天室
            final ctx = MyApp.navigatorKey.currentContext;
            if (ctx != null) {
              GoRouter.of(
                ctx,
              ).push('/chat/room', extra: {'merchantId': msg.senderId});
            }
          },
          onDismiss: () {
            entry.remove();
            _activeToast = null;
          },
        );
      },
    );

    _activeToast = entry;
    overlay.insert(entry);

    // 4 秒後自動消失
    Future.delayed(const Duration(seconds: 4), () {
      if (entry.mounted) {
        entry.remove();
        if (_activeToast == entry) _activeToast = null;
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final router = ref.watch(appRouterProvider);
    final appTextSize = ref.watch(appTextSizeProvider);
    // ScreenUtil 僅保留給仍在遷移中的寬高尺寸；文字固定使用 logical px。
    // 版面透過 LayoutBuilder / breakpoint 重排，不能隨螢幕寬度把字縮小。
    return ScreenUtilInit(
      designSize: const Size(414, 896),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      splitScreenMode: true,
      builder: (context, child) {
        return MaterialApp.router(
          routerConfig: router,
          scaffoldMessengerKey: MyApp.scaffoldMessengerKey,
          title: 'Hawkr',
          debugShowCheckedModeBanner: false,
          theme: AppTheme.light,
          builder: (context, child) {
            final mq = AppMediaQueryPolicy.apply(
              MediaQuery.of(context),
              textSize: appTextSize,
            );
            final inner = _appStoreScreenshotMode
                ? child ?? const SizedBox()
                : EnvironmentBanner(child: child ?? const SizedBox());

            final navigationFramed = BuyerNavigationFrame(
              router: router,
              navigatorKey: rootNavigatorKey,
              navigationObserver: buyerNavigationObserver,
              child: inner,
            );

            // Token 過期提醒 + 強制登出；對話框透過 rootNavigatorKey 顯示。
            final guarded = ForceLogoutListener(
              navigatorKey: rootNavigatorKey,
              currentPath: () =>
                  router.routerDelegate.currentConfiguration.uri.toString(),
              onLoginRequired: (continuePath) {
                final encoded = Uri.encodeComponent(continuePath);
                router.push('/login?continue=$encoded');
              },
              child: navigationFramed,
            );
            final keyboardDismissible = KeyboardDismissScope(child: guarded);

            // 非 prod 環境注入 FCM debug 浮動按鈕
            final appSurface = _appStoreScreenshotMode || !F.showFcmDebugOverlay
                ? keyboardDismissible
                // fit: StackFit.expand 讓 guarded 填滿全螢幕，否則會靠左貼齊導致整個 app 偏左
                : Stack(
                    fit: StackFit.expand,
                    children: [keyboardDismissible, const FcmDebugOverlay()],
                  );

            // Keep the router, global navigation, environment banner and
            // debug overlay under one typography policy. Wrapping only the
            // router child would leave bottom-navigation labels at a
            // different user-selected size.
            return MediaQuery(data: mq, child: appSurface);
          },
        );
      },
    );
  }
}

/// 頂部滑入式聊天通知橫幅 — 與主題風格一致
class _ChatNotificationBanner extends StatefulWidget {
  const _ChatNotificationBanner({
    required this.senderName,
    this.senderAvatar,
    required this.senderId,
    required this.preview,
    required this.topPadding,
    this.onTap,
    this.onDismiss,
  });
  final String senderName;
  final String? senderAvatar;
  final String senderId;
  final String preview;
  final double topPadding;
  final VoidCallback? onTap;
  final VoidCallback? onDismiss;

  @override
  State<_ChatNotificationBanner> createState() =>
      _ChatNotificationBannerState();
}

class _ChatNotificationBannerState extends State<_ChatNotificationBanner>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<Offset> _slideAnim;
  late final Animation<double> _fadeAnim;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 350),
    );
    _slideAnim = Tween<Offset>(
      begin: const Offset(0, -1),
      end: Offset.zero,
    ).animate(CurvedAnimation(parent: _ctrl, curve: Curves.easeOutCubic));
    _fadeAnim = Tween<double>(
      begin: 0,
      end: 1,
    ).animate(CurvedAnimation(parent: _ctrl, curve: Curves.easeOut));
    _ctrl.forward();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final title = widget.senderName.trim().isNotEmpty
        ? widget.senderName.trim()
        : '新訊息';

    return Positioned(
      top: 0,
      left: 0,
      right: 0,
      child: SlideTransition(
        position: _slideAnim,
        child: FadeTransition(
          opacity: _fadeAnim,
          child: GestureDetector(
            onTap: widget.onTap,
            onVerticalDragEnd: (d) {
              if (d.velocity.pixelsPerSecond.dy < -100) {
                widget.onDismiss?.call();
              }
            },
            child: Container(
              padding: EdgeInsets.fromLTRB(16, widget.topPadding + 8, 16, 12),
              decoration: const BoxDecoration(
                color: AppColors.primary,
                borderRadius: BorderRadius.only(
                  bottomLeft: Radius.circular(16),
                  bottomRight: Radius.circular(16),
                ),
                boxShadow: [
                  BoxShadow(
                    color: AppColors.cardShadow,
                    blurRadius: 12,
                    offset: Offset(0, 4),
                  ),
                ],
              ),
              child: Row(
                children: [
                  _buildAvatar(),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          title,
                          style: const TextStyle(
                            color: AppColors.bgCard,
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          widget.preview,
                          style: TextStyle(
                            color: AppColors.bgCard.withValues(alpha: 0.85),
                            fontSize: 14,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                    ),
                  ),
                  Icon(
                    AppIcons.chevronRight,
                    color: AppIconColors.onBrand.withValues(alpha: 0.7),
                    size: 20,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildAvatar() {
    final rawAvatar = widget.senderAvatar?.trim() ?? '';
    if (rawAvatar.isEmpty) return _fallbackAvatar();

    return ClipOval(
      child: AppCachedImage(
        imageUrl: rawAvatar,
        scene: ImageScene.chatAvatar,
        width: 40,
        height: 40,
        fit: BoxFit.cover,
        errorWidget: _fallbackAvatar(),
      ),
    );
  }

  Widget _fallbackAvatar() {
    return Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        color: AppColors.bgCard.withValues(alpha: 0.2),
        shape: BoxShape.circle,
      ),
      child: const ChatBubbleIcon(color: AppIconColors.onBrand, size: 20),
    );
  }
}
