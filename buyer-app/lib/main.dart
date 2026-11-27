import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:firebase_core/firebase_core.dart';
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
import 'core/l10n/ui_locale.dart';
import 'core/layout/app_text_size.dart';
import 'l10n/gen/app_localizations.dart';
import 'core/networking/app_version_headers.dart';
import 'core/routing/app_router.dart';
import 'core/routing/buyer_navigation_observer.dart';
import 'core/session/auth_session_provider.dart';
import 'package:hawkr_auth/logging.dart';
import 'core/session/force_logout_listener.dart';
import 'features/auth/presentation/account_restricted_screen.dart';
import 'core/session/suspended_action_listener.dart';
import 'core/session/session_provider.dart';
import 'core/theme/app_colors.dart';
import 'core/theme/app_theme.dart';
import 'features/app_update/application/force_update_provider.dart';
import 'features/chat/data/active_chat_gate.dart';
import 'features/chat/data/chat_display.dart';
import 'features/chat/data/chat_realtime.dart';
import 'features/chat/data/chat_repository.dart';
import 'features/seller/data/store_card_display.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as chat_pb;
import 'features/notification/data/notification_signals_provider.dart';
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
  }
}

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  GoogleFonts.config.allowRuntimeFetching = false;

  final flavor = _resolveFlavorFromEnv();
  flavor.requireCoreConfig();

  SentryFlutter.init(
    (options) {
      options.dsn = flavor.sentryDsn;
      options.environment = flavor.name;
      options.tracesSampleRate = 0;
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
  if (kDebugMode && _e2eMode) {
    PushNotificationManager.e2eDisabled = true;
  }
  if (e2eAuthEnabled) HKLog.logI('[E2E] auth token injected → bypass OAuth');

  if (!F.isInitialized) {
    F.init(_resolveFlavorFromEnv());
  }

  HKLog.logI('[Main] Flavor: ${F.appFlavor.name}');
  HKLog.logI('[Main] Gateway: ${F.appFlavor.gatewayHost}');
  HKLog.logI('[Main] API Base: ${F.appFlavor.apiBaseUrl}');

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

  try {
    await AppVersionHeaders.initialize();
    await container.read(forceUpdateProvider.notifier).check();
  } catch (e, s) {
    HKLog.logE('[Main] App 版本檢查失敗，fail-open 繼續啟動', e, s);
  }

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

  PushNotificationManager.instance.injectContainer(container);

  if (!_appStoreScreenshotMode && !_e2eMode && isFirebaseInitialized) {
    await _initializeFirebaseMessaging();
  }

  await container.read(sessionProvider.future);

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

  static GlobalKey<NavigatorState> get navigatorKey => rootNavigatorKey;
  static final GlobalKey<ScaffoldMessengerState> scaffoldMessengerKey =
      GlobalKey<ScaffoldMessengerState>();

  @override
  ConsumerState<MyApp> createState() => _MyAppState();
}

class _MyAppState extends ConsumerState<MyApp> with WidgetsBindingObserver {
  StreamSubscription<chat_pb.ChatEvent>? _messageSub;
  OverlayEntry? _activeToast;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    registerAnalyticsIdentity(ref);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      NotificationRouter.flushPending();
    });
    _messageSub = ref.read(chatRealtimeProvider).events.listen(_onChatEvent);
    ref.read(notificationSignalsProvider);
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
      unawaited(ref.read(forceUpdateProvider.notifier).checkThrottled());
    }
  }

  Future<void> _onChatEvent(chat_pb.ChatEvent event) async {
    if (event.whichEvent() != chat_pb.ChatEvent_Event.message) return;
    final msg = event.message.message;
    if (event.message.updated ||
        msg.sender != chat_pb.ChatParty.CHAT_PARTY_STORE) {
      return;
    }
    final viewing = await activeChatCovers(
      () => ref.read(activeChatGateProvider),
      ref.read(chatRepositoryProvider),
      msg.conversationId,
    );
    if (viewing || !mounted) return;

    HapticFeedback.mediumImpact();

    _showChatBanner(msg);
  }

  void _showChatBanner(chat_pb.Message msg) {
    final overlay = MyApp.navigatorKey.currentState?.overlay;
    if (overlay == null) return;

    _activeToast?.remove();

    final locale = ref.read(uiLocaleProvider);
    var preview = chatMessagePreview(msg, locale);
    if (preview.length > 40) preview = '${preview.substring(0, 40)}...';
    final card = ref
        .read(chatRepositoryProvider)
        .storeCards[msg.conversationId];

    late OverlayEntry entry;
    entry = OverlayEntry(
      builder: (context) {
        final top = MediaQuery.of(context).padding.top;
        return _ChatNotificationBanner(
          senderName: card?.displayName(locale) ?? '',
          senderAvatar: card?.logoUri,
          senderId: msg.conversationId,
          preview: preview,
          topPadding: top,
          onTap: () {
            entry.remove();
            _activeToast = null;
            final ctx = MyApp.navigatorKey.currentContext;
            if (ctx != null) {
              GoRouter.of(ctx).push(
                '/chat/room',
                extra: {'conversationId': msg.conversationId},
              );
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
          locale: ref.watch(uiLocaleProvider).flutterLocale,
          supportedLocales: AppLocalizations.supportedLocales,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
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
            final suspensionAware = SuspendedActionListener(
              navigatorKey: rootNavigatorKey,
              onViewDetails: () => AccountRestrictedScreen.open(router),
              child: guarded,
            );
            final keyboardDismissible = KeyboardDismissScope(
              child: suspensionAware,
            );

            final appSurface = _appStoreScreenshotMode || !F.showFcmDebugOverlay
                ? keyboardDismissible
                : Stack(
                    fit: StackFit.expand,
                    children: [keyboardDismissible, const FcmDebugOverlay()],
                  );

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
