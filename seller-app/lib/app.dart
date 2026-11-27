import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/core/l10n/ui_locale.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'config/flavor.dart';
import 'config/app_constants.dart';
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
import 'features/chat/data/chat_session.dart';
import 'features/seller/data/current_store.dart';
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
    WidgetsBinding.instance.addObserver(this);
    registerAnalyticsIdentity(ref);
    HKLog.logD(
      '[MyApp] ====== initState: Setting up new message listener ======',
    );
    _newMessageSub = ref
        .read(chatSessionProvider)
        .onNewMessage
        .listen(
          (event) {
            HKLog.logD(
              '[MyApp] ★★★ Received new message event! conversation=${event.conversationId} ★★★',
            );
            _showNewMessageToast(event);
          },
          onError: (e) => HKLog.logD('[MyApp] Stream error: $e'),
          onDone: () => HKLog.logD('[MyApp] Stream done'),
        );
    ref.listenManual(
      sellerStoreIdProvider,
      (prev, next) =>
          onChatStoreIdChanged(ref.read(chatSessionProvider), prev, next),
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
      unawaited(ref.read(forceUpdateGateProvider.notifier).checkOnResume());
    }
  }

  void _showNewMessageToast(ChatNotificationEvent event) {
    HKLog.logD(
      '[MyApp] _showNewMessageToast called from conversation=${event.conversationId}',
    );

    final gate = ref.read(activeChatGateProvider);
    final activePeer = gate.peerId;
    final activeConv = gate.conversationId;
    final senderId = event.buyerId;
    final conversationId = event.conversationId;
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

  /// 通知的頭像網址：伺服器回的圖片已經是可顯示的網址（`Image.uri`），空值回 null。
  String? _resolveAvatarUrl(String? uri) {
    final value = uri?.trim() ?? '';
    return value.isEmpty ? null : value;
  }

  @override
  Widget build(BuildContext context) {
    final goRouter = ref.watch(goRouterProvider);
    final appTextSize = ref.watch(appTextSizeProvider);
    return ScreenUtilInit(
      designSize: const Size(414, 896),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      splitScreenMode: true,
      builder: (context, child) => MaterialApp.router(
        debugShowCheckedModeBanner: false,
        routerConfig: goRouter,
        title: 'DaliyFresh Farmer',
        theme: AppTheme.light,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: ref.watch(uiLocaleProvider).flutterLocale,
        builder: (context, child) {
          final mediaQuery = AppMediaQueryPolicy.apply(
            MediaQuery.of(context),
            textSize: appTextSize,
          );

          final keyboardDismissible = ValueListenableBuilder<int>(
            valueListenable:
                KeyboardUnfocusController.overrideCounterListenable,
            builder: (_, counter, _) {
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
              : Stack(
                  fit: StackFit.expand,
                  children: [environmentFramed, const FcmDebugOverlay()],
                );

          return MediaQuery(data: mediaQuery, child: appSurface);
        },
      ),
    );
  }
}
