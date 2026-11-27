import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart'
    show CheckAppVersionResponse;
import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart' show Provider;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../config/firebase_config.dart';
import '../session/session_model.dart';
import '../session/session_provider.dart';
import 'buyer_navigation_observer.dart';
import 'floating_detail_page.dart';
import 'market_route.dart';
import 'redirect_logic.dart';
import '../../features/app_update/application/force_update_provider.dart';
import '../../features/app_update/presentation/force_update_screen.dart';
import '../../features/auth/presentation/account_restricted_screen.dart';
import '../../features/auth/presentation/auth_screen.dart';
import '../../features/home/presentation/main_navigation_screen.dart';
import '../../features/product/presentation/search_screen.dart';
import '../../features/product/presentation/search_results_screen.dart';
import '../../features/product/presentation/merchant_item_detail_screen.dart';
import '../../features/moderation/presentation/store_report_entry.dart';
import '../../features/moderation/presentation/store_report_screen.dart';
import '../../features/seller/presentation/merchant_screen.dart';
import '../../features/cart/presentation/cart_screen.dart';
import '../../features/order/presentation/order_detail_screen.dart';
import '../../features/order/data/order_summary.dart';
import '../../features/chat/presentation/chat_list_screen.dart';
import '../../features/chat/presentation/chat_thread_screen.dart';
import '../../features/ai_assistant/presentation/ai_assistant_screen.dart';
import '../../features/settings/presentation/profile_edit_screen.dart';
import '../../features/settings/presentation/settings_screen.dart';
import '../../features/settings/presentation/my_subscription_screen.dart';
import '../../features/address/presentation/address_management_screen.dart';
import '../../features/wallet/presentation/spending_history_screen.dart';
import '../../features/wallet/presentation/wallet_screen.dart';
import '../../features/notification/presentation/notifications_screen.dart';
import '../../features/notification/presentation/notification_settings_screen.dart';
import '../../features/order/presentation/after_widgets/after_sale_screen.dart';
import '../../features/order/presentation/after_widgets/after_sale_list_screen.dart';
import '../../features/order/presentation/after_widgets/after_sale_detail_screen.dart';
import '../../features/order/presentation/after_widgets/after_sale_view_screen.dart';
import '../../features/order/presentation/after_widgets/arbitration_view_screen.dart';
import '../../features/order/presentation/after_widgets/submit_appeal_screen.dart';
import '../../features/review/presentation/rate_screen.dart';
import '../../features/review/presentation/order_review_screen.dart';
import '../../features/review/presentation/product_reviews_screen.dart';
import '../../features/social/presentation/merchant_posts_feed_screen.dart';
import '../../features/social/presentation/post_detail_screen.dart';
import '../../features/category/presentation/category_landing_screen.dart';
import '../../features/member_card/presentation/spending_records_screen.dart';
import '../../features/moderation/presentation/my_reports_screen.dart';
import '../../features/moderation/presentation/store_report_detail_screen.dart';
import '../../features/moderation/presentation/store_report_supplement_screen.dart';
import '../../features/moderation/presentation/blocked_users_screen.dart';

part 'app_router.g.dart';

final rootNavigatorKey = GlobalKey<NavigatorState>();

const String _appStoreScreenshotRoute = String.fromEnvironment(
  'APP_STORE_SCREENSHOT_ROUTE',
  defaultValue: '',
);

const bool _appStoreScreenshotMode = bool.fromEnvironment(
  'APP_STORE_SCREENSHOT',
  defaultValue: false,
);

/// Router observers are injectable so hermetic tests can render the production
/// route graph without initializing Firebase. Production keeps analytics and
/// the buyer navigation observer enabled by default.
final appRouterObserversProvider = Provider<List<NavigatorObserver>>(
  (ref) => [
    if (isFirebaseInitialized)
      FirebaseAnalyticsObserver(analytics: FirebaseAnalytics.instance),
    buyerNavigationObserver,
  ],
);

@Riverpod(keepAlive: true)
GoRouter appRouter(Ref ref) {
  final notifier = _RouterNotifier(ref);
  ref.onDispose(notifier.dispose);

  return GoRouter(
    navigatorKey: rootNavigatorKey,
    refreshListenable: notifier,
    initialLocation: _appStoreScreenshotRoute.isEmpty
        ? '/'
        : _appStoreScreenshotRoute,
    observers: ref.watch(appRouterObserversProvider),
    redirect: (context, state) {
      final session = ref.read(sessionProvider).value;
      return resolveBuyerRedirect(
        location: state.matchedLocation,
        isLoggedIn: hasUsableSession(session),
        needsForceUpdate: ref.read(forceUpdateProvider).updateRequired,
        isScreenshotMode: _appStoreScreenshotMode,
        loginContinueParam: state.uri.queryParameters['continue'],
        currentUri: state.uri.toString(),
      );
    },
    routes: [
      GoRoute(
        path: '/force-update',
        builder: (_, __) => const ForceUpdateScreen(),
      ),

      GoRoute(
        path: AccountRestrictedScreen.path,
        builder: (_, __) => const AccountRestrictedScreen(),
      ),

      GoRoute(path: '/welcome', redirect: (_, __) => '/'),
      GoRoute(
        path: '/login',
        builder: (_, state) =>
            AuthScreen(continueUrl: state.uri.queryParameters['continue']),
      ),

      GoRoute(
        path: '/',
        builder: (_, state) {
          final tab = int.tryParse(state.uri.queryParameters['tab'] ?? '');
          final marketIntent = tab == marketTabIndex
              ? MarketRouteIntent.fromUri(state.uri)
              : const MarketRouteIntent();
          final walletTab =
              int.tryParse(state.uri.queryParameters['walletTab'] ?? '') ?? 0;
          final orderTab =
              int.tryParse(state.uri.queryParameters['orderTab'] ?? '') ?? 0;
          final orderPendingAction =
              state.uri.queryParameters['orderPending'] == '1';
          return MainNavigationScreen(
            initialIndex: tab,
            marketKeyword: marketIntent.keyword,
            marketSort: marketIntent.sort,
            walletTab: walletTab,
            orderTab: orderTab,
            orderPendingAction: orderPendingAction,
          );
        },
      ),

      GoRoute(path: '/search', builder: (_, __) => const SearchScreen()),

      GoRoute(
        path: '/merchant-product',
        redirect: (_, state) =>
            state.extra is MerchantItemDetailScreen ? null : '/?tab=1',
        pageBuilder: (context, state) {
          return floatingDetailPage<Map<String, dynamic>?>(
            context: context,
            key: state.pageKey,
            child: state.extra as MerchantItemDetailScreen,
            barrierLabel: '關閉商品詳情',
          );
        },
      ),

      GoRoute(
        path: '/merchant/:storeCode',
        builder: (_, state) =>
            MerchantScreen(storeCode: state.pathParameters['storeCode'] ?? ''),
      ),

      GoRoute(
        path: '/category',
        redirect: (_, state) => canonicalMarketLocation(state.uri),
      ),

      GoRoute(
        path: '/cart',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>?;
          final preSelectedSkuId = extra?['preSelectedSkuId'] as int?;
          return CartScreen(preSelectedSkuId: preSelectedSkuId);
        },
      ),

      GoRoute(path: '/orders', redirect: (_, __) => '/?tab=3'),
      GoRoute(
        path: '/orders/detail',
        pageBuilder: (context, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return floatingDetailPage<void>(
            context: context,
            key: state.pageKey,
            child: OrderDetailScreen(
              orderSn: extra['orderSn'] as String?,
              fromCheckout: extra['fromCheckout'] as bool? ?? false,
              heroTag: extra['heroTag'],
            ),
            barrierLabel: '關閉訂單詳情',
          );
        },
      ),
      GoRoute(
        path: '/orders/matched',
        redirect: (_, __) => '/?tab=3',
      ),
      GoRoute(
        path: '/orders/order-dispute',
        redirect: (_, __) => '/settings/after-sale',
      ),
      GoRoute(
        path: '/orders/after-sale',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          final rawItems = extra['items'];
          final items = rawItems is List
              ? rawItems
                    .whereType<Map>()
                    .map((m) => Map<String, dynamic>.from(m))
                    .toList()
              : <Map<String, dynamic>>[];
          return AfterSaleScreen(
            orderSn: extra['orderSn'] as String? ?? '',
            items: items,
          );
        },
      ),
      GoRoute(
        path: '/orders/after-sale/view',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return AfterSaleViewScreen(
            orderSn: extra['orderSn'] as String? ?? '',
          );
        },
      ),
      GoRoute(
        path: '/orders/delivering',
        redirect: (_, state) {
          final extra = state.extra;
          return extra is Map<String, dynamic> && extra['order'] != null
              ? null
              : '/?tab=3';
        },
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          final order = extra['order'] as OrderSummary?;
          final orderSn = order?.orderSn.trim().isNotEmpty == true
              ? order!.orderSn.trim()
              : order?.id.trim();
          return OrderDetailScreen(orderSn: orderSn);
        },
      ),

      GoRoute(path: '/chat', builder: (_, __) => const ChatListScreen()),
      GoRoute(
        path: '/chat/room',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return ChatThreadScreen(
            storeId: extra['merchantId'] as String?,
            storeName: extra['merchantName'] as String?,
            conversationId: extra['conversationId'] as String?,
          );
        },
      ),
      GoRoute(
        path: '/chat/ai-assistant',
        builder: (_, __) => const AiAssistantScreen(),
      ),

      GoRoute(
        path: '/settings/profile',
        builder: (_, __) => const ProfileEditScreen(),
      ),
      GoRoute(
        path: '/settings/address',
        builder: (_, __) => const AddressManagementScreen(),
      ),
      GoRoute(
        path: '/settings/help',
        builder: (_, __) => const HelpCenterScreen(),
      ),
      GoRoute(
        path: '/settings/notifications',
        builder: (_, __) => const NotificationSettingsScreen(),
      ),
      GoRoute(
        path: '/settings/after-sale',
        builder: (_, __) => const AfterSaleListScreen(),
      ),
      GoRoute(
        path: '/settings/after-sale/detail',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return AfterSaleDetailScreen(
            caseNo: extra['caseNo'] as String?,
            orderSn: extra['orderSn'] as String?,
          );
        },
      ),
      GoRoute(
        path: '/settings/after-sale/appeal',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return SubmitAppealScreen(caseNo: (extra['caseNo'] as String?) ?? '');
        },
      ),
      GoRoute(
        path: '/settings/after-sale/arbitration',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return ArbitrationViewScreen(
            caseNo: (extra['caseNo'] as String?) ?? '',
          );
        },
      ),
      GoRoute(
        path: '/settings/subscription',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return MySubscriptionScreen(
            initialTab: extra['initialTab'] as int? ?? 0,
          );
        },
      ),
      GoRoute(
        path: '/settings/reports',
        builder: (_, __) => const MyReportsScreen(),
        routes: [
          GoRoute(
            path: ':reportId',
            builder: (_, state) => StoreReportDetailScreen(
              reportId: state.pathParameters['reportId']!,
            ),
            routes: [
              GoRoute(
                path: 'supplement',
                builder: (_, state) => StoreReportSupplementScreen(
                  reportId: state.pathParameters['reportId']!,
                ),
              ),
            ],
          ),
        ],
      ),
      GoRoute(
        path: storeReportRoute,
        redirect: (_, state) => storeReportRedirect(state.extra),
        builder: (_, state) =>
            StoreReportScreen(args: state.extra! as StoreReportArgs),
      ),
      GoRoute(
        path: '/settings/blocks',
        builder: (_, __) => const BlockedUsersScreen(),
      ),

      GoRoute(
        path: '/notifications',
        builder: (_, __) => const NotificationsScreen(),
      ),

      GoRoute(
        path: '/account/orders/:orderNo',
        pageBuilder: (context, state) => floatingDetailPage<void>(
          context: context,
          key: state.pageKey,
          child: OrderDetailScreen(orderSn: state.pathParameters['orderNo']),
        ),
      ),
      GoRoute(
        path: '/account/after-sales/:caseNo',
        builder: (_, state) =>
            AfterSaleDetailScreen(caseNo: state.pathParameters['caseNo']),
      ),
      GoRoute(
        path: '/account/messages/:targetId',
        builder: (_, state) =>
            ChatThreadScreen(storeId: state.pathParameters['targetId']),
      ),
      GoRoute(
        path: '/products/:id',
        pageBuilder: (context, state) =>
            floatingDetailPage<Map<String, dynamic>?>(
              context: context,
              key: state.pageKey,
              child: MerchantItemDetailScreen(
                skuId: state.pathParameters['id'],
              ),
            ),
      ),

      GoRoute(
        path: '/wallet/spending',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return SpendingHistoryScreen(
            merchantId: extra['merchantId'] as String? ?? '',
            merchantName: extra['merchantName'] as String? ?? '',
          );
        },
      ),
      GoRoute(
        path: '/wallet/coupons',
        builder: (_, __) => const WalletScreen(showTopBar: true, initialTab: 1),
      ),

      GoRoute(
        path: '/review',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return RateScreen(
            orderItemId: (extra['orderItemId'] ?? '').toString(),
            orderNo: (extra['orderNo'] ?? '').toString(),
          );
        },
      ),
      GoRoute(
        path: '/review/order',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          final rawItems = extra['items'];
          final items = rawItems is List
              ? rawItems
                    .whereType<Map>()
                    .map((m) => Map<String, dynamic>.from(m))
                    .toList()
              : <Map<String, dynamic>>[];
          return OrderReviewScreen(
            orderSn: extra['orderSn'] as String? ?? '',
            items: items,
            skuId: extra['skuId'] as int? ?? 0,
          );
        },
      ),
      GoRoute(
        path: '/review/product',
        builder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return ProductReviewsScreen(
            merchantId: extra['merchantId'] as String? ?? '',
            skuId: extra['skuId'] as String? ?? '',
            productName: extra['productName'] as String? ?? '商品評價',
          );
        },
      ),

      GoRoute(
        path: '/marketplace',
        redirect: (_, state) => canonicalMarketLocation(state.uri),
      ),

      GoRoute(
        path: '/member-cards',
        builder: (_, __) => const WalletScreen(showTopBar: true, initialTab: 0),
      ),
      GoRoute(
        path: '/member-cards/:seller_id/records',
        builder: (_, state) => SpendingRecordsScreen(
          sellerId: state.pathParameters['seller_id']!,
          sellerName: state.extra as String? ?? '',
        ),
      ),

      GoRoute(
        path: '/social/posts',
        pageBuilder: (_, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return NoTransitionPage<void>(
            key: state.pageKey,
            child: MerchantPostsFeedScreen(
              merchantId: extra['merchantId'] as String? ?? '',
              merchantName: extra['merchantName'] as String?,
              merchantAvatar: extra['merchantAvatar'] as String?,
              initialIndex: extra['initialIndex'] as int? ?? 0,
              initialPostId: extra['initialPostId'] as String?,
              includeGlobalFeed: extra['includeGlobalFeed'] as bool? ?? false,
            ),
          );
        },
      ),
      GoRoute(
        path: '/social/post/:postId',
        pageBuilder: (context, state) {
          final extra = state.extra as Map<String, dynamic>? ?? {};
          return floatingDetailPage<void>(
            context: context,
            key: state.pageKey,
            child: PostDetailScreen(
              postId: int.tryParse(state.pathParameters['postId'] ?? '') ?? 0,
              merchantId: extra['merchantId'] as String?,
              merchantName: extra['merchantName'] as String?,
              merchantAvatar: extra['merchantAvatar'] as String?,
            ),
            barrierLabel: '關閉貼文詳情',
          );
        },
      ),

      ShellRoute(
        builder: (_, __, child) => child,
        routes: [
          GoRoute(
            path: '/search/results',
            builder: (_, state) => SearchResultsScreen(
              query: state.uri.queryParameters['q'] ?? '',
            ),
          ),
          GoRoute(
            path: '/category-landing',
            builder: (_, __) => const CategoryLandingScreen(),
          ),
          GoRoute(
            path: '/wallet/coupon-detail',
            redirect: (_, __) => '/wallet/coupons',
          ),
          GoRoute(
            path: '/orders/reserve',
            redirect: (_, __) => '/?tab=1',
          ),
          GoRoute(
            path: '/orders/payment-details',
            redirect: (_, __) => '/?tab=3',
          ),
          GoRoute(
            path: '/orders/tracking',
            redirect: (_, __) => '/?tab=3',
          ),
          GoRoute(
            path: '/orders/dispute',
            redirect: (_, __) => '/settings/after-sale',
          ),
          GoRoute(
            path: '/orders/payment',
            redirect: (_, __) => '/?tab=3',
          ),
        ],
      ),
    ],
  );
}

/// Apple App Review 合規 — 帳號相關路由才需要登入；其他路由（首頁、商品列表、
/// 商品詳情、搜尋、賣家頁、社群貼文）皆允許訪客瀏覽。
/// 新增帳號相關路由時記得加進這個清單。
/// 監聽 session 狀態，任何變化都觸發 GoRouter 重新執行 redirect。
/// 確保 401 logout 後立即跳轉到 /login，而非等待下次路由事件。
class _RouterNotifier extends ChangeNotifier {
  _RouterNotifier(Ref ref) {
    ref.listen<AsyncValue<SessionModel?>>(
      sessionProvider,
      (_, __) => notifyListeners(),
    );
    ref.listen<CheckAppVersionResponse>(
      forceUpdateProvider,
      (_, __) => notifyListeners(),
    );
  }
}
