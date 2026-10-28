import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'config/flavor.dart';
import 'config/app_constants.dart';
import 'config/media_image.dart';
import 'core/analytics/analytics_identity.dart';
import 'core/layout/app_media_query_policy.dart';
import 'core/layout/app_text_size.dart';
import 'core/router/app_router.dart';
import 'core/theme/app_colors.dart';
import 'core/theme/app_theme.dart';
import 'shared/utils/hk_log.dart';
import 'shared/utils/keyboard_unfocus_controller.dart';
import 'features/app_update/application/force_update_provider.dart';
import 'features/chat/data/active_chat_gate.dart';
import 'features/chat/data/chat_repository.dart';
import 'features/notification/data/push/manager.dart';
import 'shared/widgets/chat_bubble_icon.dart';
import 'shared/widgets/environment_banner.dart';
import 'shared/widgets/fcm_debug_overlay.dart';
import 'shared/widgets/optimized_image.dart';
import 'config/image_config.dart';

class MyApp extends ConsumerStatefulWidget {
  const MyApp({super.key, this.runtimeSideEffectsEnabled = true});

  /// Keeps the production widget tree testable without starting analytics,
  /// push lifecycle work, or a long-lived chat subscription.
  ///
  /// Production entrypoints must use the default `true`. Hermetic app tests
  /// set this to `false` while still rendering this exact app and router.
  final bool runtimeSideEffectsEnabled;

  /// 全域 navigatorKey，用來取得 Overlay
  static final GlobalKey<NavigatorState> navigatorKey =
      GlobalKey<NavigatorState>();

  @override
  ConsumerState<MyApp> createState() => _MyAppState();
}

class _MyAppState extends ConsumerState<MyApp> with WidgetsBindingObserver {
  StreamSubscription<ChatNotificationEvent>? _newMessageSub;

  @override
  void initState() {
    super.initState();
    if (!widget.runtimeSideEffectsEnabled) return;
    // App 回前景時節流重新註冊 device token（讓後端 updated_at 反映活躍度，供 staleness reaper）。
    WidgetsBinding.instance.addObserver(this);
    // GA4: 綁定 session → 身分（user_id/seller_id/role），fireImmediately 套用當前登入狀態
    registerAnalyticsIdentity(ref);
    HKLog.logD(
      '[MyApp] ====== initState: Setting up new message listener ======',
    );
    // 監聯新訊息事件；eager read → creates the keepAlive provider
    _newMessageSub = ref
        .read(chatRepositoryProvider)
        .onNewMessage
        .listen(
          (event) {
            HKLog.logD(
              '[MyApp] ★★★ Received new message event! sender=${event.senderId} ★★★',
            );
            _showNewMessageToast(event);
          },
          onError: (e) => HKLog.logD('[MyApp] Stream error: $e'),
          onDone: () => HKLog.logD('[MyApp] Stream done'),
        );
    HKLog.logD('[MyApp] ====== Listener setup completed ======');
  }

  @override
  void dispose() {
    _newMessageSub?.cancel();
    if (widget.runtimeSideEffectsEnabled) {
      WidgetsBinding.instance.removeObserver(this);
    }
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (!widget.runtimeSideEffectsEnabled) return;
    if (state == AppLifecycleState.resumed) {
      PushNotificationManager.instance.onAppResumed();
      // App 版本 gate 前景重檢（`packages/hawkr_app_update/docs/spec.md`）：節流 10 分鐘、以上次
      // 成功檢查為錨；結果變動經 SessionChangeNotifier 觸發 redirect 重判。
      unawaited(ref.read(forceUpdateGateProvider.notifier).checkOnResume());
    }
  }

  void _showNewMessageToast(ChatNotificationEvent event) {
    HKLog.logD(
      '[MyApp] _showNewMessageToast called from sender=${event.senderId}',
    );

    // 使用者正在跟這個人聊天 → 不彈 toast，跟 FCM 前景 banner 同邏輯
    // (manager.dart 已有同樣 gate)。
    // senderId / conversationId 任一比對到就跳過：participantId 與 senderId
    // 可能 namespace 不同對不上，conversationId 是 server 端唯一 ID 比對最穩。
    final gate = ref.read(activeChatGateProvider);
    final activePeer = gate.peerId;
    final activeConv = gate.conversationId;
    final senderId = event.senderId;
    final conversationId = event.conversationId ?? '';
    final matchesPeer =
        senderId.isNotEmpty && activePeer != null && activePeer == senderId;
    final matchesConv =
        conversationId.isNotEmpty &&
        activeConv != null &&
        activeConv == conversationId;
    if (matchesPeer || matchesConv) {
      HKLog.logD(
        '[MyApp] 跳過 WS toast：使用者已在聊天室內 '
        'senderId=$senderId conversationId=$conversationId '
        '(activePeer=$activePeer activeConv=$activeConv)',
      );
      return;
    }

    final navigatorState = MyApp.navigatorKey.currentState;
    if (navigatorState == null) {
      HKLog.logD('[MyApp] navigatorState is null!');
      return;
    }

    final overlay = navigatorState.overlay;
    if (overlay == null) {
      HKLog.logD('[MyApp] overlay is null!');
      return;
    }

    HKLog.logD('[MyApp] Showing toast overlay...');

    // 建立 OverlayEntry 顯示浮動 Toast
    late OverlayEntry overlayEntry;
    overlayEntry = OverlayEntry(
      builder: (context) => Positioned(
        bottom: 120.h,
        left: 0,
        right: 0,
        child: Center(
          child: Material(
            color: Colors.transparent,
            child: TweenAnimationBuilder<double>(
              tween: Tween(begin: 0.0, end: 1.0),
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeOut,
              builder: (context, value, child) {
                return Opacity(
                  opacity: value,
                  child: Transform.translate(
                    offset: Offset(0, 20.h * (1 - value)),
                    child: child,
                  ),
                );
              },
              child: Container(
                constraints: BoxConstraints(maxWidth: 280.w),
                padding: EdgeInsets.symmetric(horizontal: 20.w, vertical: 16.h),
                decoration: BoxDecoration(
                  color: AppColors.bgCard,
                  borderRadius: BorderRadius.circular(16.r),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 24,
                      offset: Offset(0, 8.h),
                    ),
                  ],
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _buildChatAvatar(event.senderAvatar),
                    SizedBox(width: 12.w),
                    Flexible(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            _senderTitle(event),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 16.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.textEmphasis,
                            ),
                          ),
                          if (event.preview.isNotEmpty) ...[
                            SizedBox(height: 3.h),
                            Text(
                              event.preview,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                fontSize: 14.sp,
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ],
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );

    overlay.insert(overlayEntry);

    // 3 秒後自動移除
    Future.delayed(const Duration(seconds: 3), () {
      if (overlayEntry.mounted) {
        overlayEntry.remove();
      }
    });
  }

  String _senderTitle(ChatNotificationEvent event) {
    final name = event.senderName?.trim() ?? '';
    if (name.isNotEmpty) return name;
    return '您有一則新訊息';
  }

  Widget _buildChatAvatar(String? rawAvatar) {
    final url = _resolveAvatarUrl(rawAvatar);
    if (url == null || url.isEmpty) {
      return _fallbackChatAvatar();
    }
    return ClipOval(
      child: OptimizedNetworkImage(
        imageUrl: url,
        scene: ImageScene.chatAvatar,
        width: 42.w,
        height: 42.w,
        fit: BoxFit.cover,
        errorWidget: _fallbackChatAvatar(),
      ),
    );
  }

  Widget _fallbackChatAvatar() {
    return Container(
      width: 42.w,
      height: 42.w,
      decoration: BoxDecoration(
        color: AppColors.primary.withValues(alpha: 0.12),
        shape: BoxShape.circle,
      ),
      child: ChatBubbleIcon(size: 20.sp, color: AppIconColors.secondary),
    );
  }

  /// WS `chat` / `notification` 推播的 `avatarKey` → 顯示 URL。
  /// （唯一來源是 WebSocket，見 `chat_repository.dart` 的 `_emitNewMessageEvent`
  /// 兩處呼叫點；FCM 那條路不經過這裡。）
  ///
  /// 值可能是 objectKey、本站顯示 URL，或外部（Google）URL；三種都交給
  /// `buildImageUrl`（唯一組裝入口）：本站 URL 還原成 key 後改用 avatar 尺寸重組，
  /// 外部 URL 原樣 passthrough。
  ///
  /// **為什麼這裡不需要「裸 `/api/` 路徑」與「`i/{preset}/` 前綴污染」的護欄**
  /// （#430 B3 移除舊分支時查證，2026-08-05）：
  /// - `buildImageUrl` 的 `/api/` passthrough 只對 http URL 生效，裸路徑會被當成
  ///   objectKey 組出 `{mediaBase}/i/avatar/api/...`（必 404）；`i/{preset}/` 前綴
  ///   也只有 `mediaObjectKey` 會剝除。兩者確實是 `buildImageUrl` 的盲區。
  /// - 但這個值到不了那兩種形狀：後端只放 objectKey。
  ///   `im-service` 的 WS payload 取 `AvatarKey` / `LogoKey`
  ///   （`internal/client/seller/client.go:66,78,80`、`buyer/client.go:58`，
  ///   emit 於 `internal/usecase/message.go:505-506`，49 §6.6 明載「WS payload 只帶
  ///   objectKey」），值就是 `sellers.logo` / `buyers.avatar` 欄位原文。
  /// - 兩環境全表普查（2026-08-05）：dev 114＋124 列、prod 17＋12 列，
  ///   **100% 以 `public/` 開頭**，`/api/`、`member`、`i/` 前綴各 0 列。
  /// 所以舊分支保護的是 49 號媒體遷移前的歷史形狀，現已不可達。
  /// 若哪天 WS 改帶完整 URL 或動態端點路徑，要回頭補護欄並加測試釘住。
  String? _resolveAvatarUrl(String? rawAvatar) {
    final value = rawAvatar?.trim() ?? '';
    if (value.isEmpty) return null;
    return buildImageUrl(value, ImagePreset.avatar);
  }

  @override
  Widget build(BuildContext context) {
    final goRouter = ref.watch(goRouterProvider);
    final appTextSize = ref.watch(appTextSizeProvider);
    return ScreenUtilInit(
      designSize: const Size(414, 896),
      // Keep logical dimensions and typography stable across physical phones.
      // Responsive screens must adapt from constraints instead of scaling the
      // entire simulator canvas to the device width.
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      splitScreenMode: true,
      builder: (context, child) => MaterialApp.router(
        debugShowCheckedModeBanner: false,
        routerConfig: goRouter,
        title: 'DaliyFresh Farmer',
        theme: AppTheme.light,
        localizationsDelegates: const [
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
        ],
        supportedLocales: const [
          Locale('zh', 'TW'),
          Locale('zh', 'CN'),
          Locale('en', 'US'),
        ],
        locale: const Locale('zh', 'TW'),
        builder: (context, child) {
          final mediaQuery = AppMediaQueryPolicy.apply(
            MediaQuery.of(context),
            textSize: appTextSize,
          );

          final keyboardDismissible = ValueListenableBuilder<int>(
            valueListenable:
                KeyboardUnfocusController.overrideCounterListenable,
            builder: (_, counter, __) {
              final allowUnfocus = counter == 0;
              return Listener(
                behavior: HitTestBehavior.deferToChild,
                onPointerDown: allowUnfocus
                    ? (_) => FocusManager.instance.primaryFocus?.unfocus()
                    : null,
                child: child ?? const SizedBox.shrink(),
              );
            },
          );

          final environmentFramed = EnvironmentBanner(
            child: keyboardDismissible,
          );

          final appSurface =
              F.appFlavor == Flavor.prod || !AppConstants.showFcmDebugOverlay
              ? environmentFramed
              // fit: StackFit.expand 讓內容填滿全螢幕，否則會靠左貼齊。
              : Stack(
                  fit: StackFit.expand,
                  children: [environmentFramed, const FcmDebugOverlay()],
                );

          // Keep routes, the environment banner, and debug controls under
          // the same user-selected typography policy.
          return MediaQuery(data: mediaQuery, child: appSurface);
        },
      ),
    );
  }
}
