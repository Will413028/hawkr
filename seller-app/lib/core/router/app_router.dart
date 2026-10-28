import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'dart:typed_data';

import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show SystemUiOverlayStyle;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'redirect_logic.dart';
import '../../app.dart';
import '../../config/firebase_config.dart';
import '../../features/ai_assistant/presentation/ai_assistant_screen.dart';
import '../../features/app_update/application/force_update_provider.dart';
import '../../features/app_update/presentation/force_update_screen.dart';
import '../../features/auth/presentation/auth_screen.dart';
import '../../features/auth/presentation/welcome_screen.dart';
import '../../features/chat/presentation/chat_list_screen.dart';
import '../../features/chat/presentation/chat_thread_screen.dart';
import '../../features/coupon/data/coupon_model.dart';
import '../../features/coupon/data/coupon_repository.dart';
import '../../features/coupon/presentation/add_coupon_screen.dart';
import '../../features/coupon/presentation/coupon_detail_screen.dart';
import '../../features/coupon/presentation/coupon_editor_screen.dart';
import '../../features/customer/data/customer_model.dart';
import '../../features/customer/data/customer_repository.dart';
import '../../features/customer/presentation/customer_detail_screen.dart';
import '../../features/customer/presentation/customer_management_screen.dart';
import '../../features/customer/presentation/customer_order_detail_screen.dart';
import '../../features/customer/presentation/customer_spending_history_screen.dart';
import '../../features/customer/presentation/subscriber_list_screen.dart';
import '../../features/dashboard/presentation/dashboard_screen.dart';
import '../../features/notification/presentation/notification_settings_screen.dart';
import '../../features/notification/presentation/notifications_screen.dart';
import '../../features/order/data/order_model.dart';
import '../../features/order/presentation/after_sales_detail_screen.dart';
import '../../features/order/presentation/logistics_tracking_screen.dart';
import '../../features/order/presentation/order_detail_screen.dart';
import '../../features/order/presentation/order_details_receipts_screen.dart';
import '../../features/order/presentation/order_list_screen.dart';
import '../../features/order/presentation/order_payment_details_screen.dart';
import '../../features/order/presentation/order_receipts_screen.dart';
import '../../features/photo_editor/presentation/image_editor_screen.dart';
import '../../features/photo_editor/presentation/photo_editor_args.dart';
import '../../features/post/presentation/page_home_feed_screen.dart';
import '../../features/post/presentation/page_screen.dart';
import '../../features/post/presentation/post_compose_screen.dart';
import '../../features/post/presentation/post_edit_screen.dart';
import '../../features/post/presentation/post_media_picker_screen.dart';
import '../../features/procurement/presentation/seller_publication_list_screen.dart';
import '../../features/procurement/presentation/seller_quote_form_screen.dart';
import '../../features/product/presentation/product_preview_screen.dart';
import '../../features/product/presentation/product_reviews_screen.dart';
import '../../features/product/presentation/product_setup_image_screen.dart';
import '../../features/product/presentation/product_setup_info_screen.dart';
import '../../features/product/presentation/product_setup_media_picker_screen.dart';
import '../../features/product/presentation/product_setup_picker/product_setup_picker_screen.dart';
import '../../features/product/presentation/product_setup_spec_screen.dart';
import '../../features/report/presentation/block_list_screen.dart';
import '../../features/report/presentation/my_reports_screen.dart';
import '../../features/shipment_anomaly/presentation/shipment_anomaly_dashboard_screen.dart';
import '../../features/seller/presentation/profile_screen.dart';
import '../../features/seller/presentation/seller_measure_request_form_screen.dart';
import '../../features/seller/presentation/seller_appeal_screen.dart';
import '../../features/seller/presentation/seller_certification_upload_screen.dart';
import '../../features/seller/presentation/seller_info_screen.dart';
import '../../features/seller/presentation/seller_photo_tools_screen.dart';
import '../../features/seller/data/current_store.dart';
import '../../features/seller/data/measures.dart' show MeasureRequestAction;
import '../../features/seller/presentation/store_unavailable_screen.dart';
import '../../features/seller/presentation/suspension_notice_screen.dart';
import '../../features/settings/presentation/help_center_screen.dart';
import '../../features/settings/presentation/shipping_settings/shipping_settings_screen.dart';
import '../../features/wallet/presentation/wallet_screen.dart';
import '../../shared/widgets/app_bottom_navigation_bar.dart';
import '../../shared/widgets/seller_floating_navigation_layout.dart';
import '../../shared/widgets/rich_text_editor.dart';
import '../session/session_provider.dart';
import 'route_payload.dart';
import 'session_listenable.dart';
import 'tab_refresh_provider.dart';

// ══════════════════════════════════════════════════════════════════════════════
// 路由路徑常量
// ══════════════════════════════════════════════════════════════════════════════

/// 認證相關路徑
class AuthRoutes {
  static const welcome = '/welcome';
  static const auth = '/auth';
  static const onboard = '/onboard';
}

/// 主導航路徑
class MainRoutes {
  static const dashboard = '/main/dashboard';
  static const category = '/main/category';
  static const wallet = '/main/wallet';
  static const orders = '/main/orders';
  static const settings = '/main/settings';
}

/// 客戶相關路徑
class CustomerRoutes {
  static const customers = '/customers';
  static const subscribers = '/subscribers';
  static const detail = '/customer/detail';
  static const spendingHistory = '/customer/spending-history';
  static const orderDetail = '/customer/order-detail';
}

/// 訂單相關路徑
class OrderRoutes {
  static const details = '/order/details';
  static const matched = '/order/matched';
  static const paymentDetails = '/order/payment-details';
  static const receipts = '/order/receipts';
  static const receiptsDetail = '/order/receipts-detail';
  static const dispute = '/order/dispute';
  static const logistics = '/order/logistics';
  static const view = '/orders/view';
  static const matchedDetail = '/orders/matched/detail';
}

/// 商品相關路徑
class ProductRoutes {
  static const picker = '/product/picker';
  static const info = '/product/info';
  static const images = '/product/images';
  static const specs = '/product/specs';
  static const mediaPicker = '/product/media-picker';
  static const preview = '/product/preview';
  static const reviews = '/product/reviews';
}

/// 貼文相關路徑
class PostRoutes {
  static const mediaPicker = '/post/media-picker';
  static const compose = '/post/compose';
  static const edit = '/post/edit';
  static const feed = '/post/feed';
}

/// 措施請求表單（[SellerRoutes.measureRequest]）依查詢參數 `mode` 決定是申覆還是請求解除：沒有參數是申覆、
/// `mode=remediation` 是請求解除，其他值不是有效的表單網址，回 null。
MeasureRequestAction? measureFormKindOf(Uri uri) {
  final mode = uri.queryParameters[SellerRoutes.measureFormModeParam];
  if (mode == null) return MeasureRequestAction.appeal;
  if (mode == SellerRoutes.measureFormModeRemediation) {
    return MeasureRequestAction.remediation;
  }
  return null;
}

/// [SellerRoutes.measureRequest] 的路由層導向：`mode` 不是有效值時回到措施列表（和舊網址導向同一種做法），有效時 null。
String? measureRequestRedirect(Uri uri) =>
    measureFormKindOf(uri) == null ? SellerRoutes.measures : null;

/// 賣家相關路徑
class SellerRoutes {
  static const certificationUpload = '/seller/certification/upload';
  static const photoTools = '/seller/photo-tools';

  /// 停權公告頁（46 號設計書「賣家能力權限層」§7.2）
  static const suspensionNotice = '/seller/suspension-notice';

  /// 措施與申覆列表（停權公告頁的入口）
  static const measures = '/seller/measures';

  /// 申覆與請求解除共用的全螢幕表單；路徑參數是措施 id，用 [measureRequestFor] 組網址。
  static const measureRequest = '/seller/measures/:measureId';

  /// [measureId] 這一筆措施的申覆表單網址（請求解除用 [measureRemediationFor]）。
  static String measureRequestFor(String measureId) =>
      '/seller/measures/${Uri.encodeComponent(measureId)}';

  /// 「已改善，請求解除」：同一個表單畫面（[measureRequest] 的路由），以查詢參數 `mode=remediation` 切到請求解除，不另開路由
  /// （路由能力與無店家放行都沿用 [measureRequest] 的宣告）。
  static String measureRemediationFor(String measureId) =>
      '${measureRequestFor(measureId)}?$measureFormModeParam=$measureFormModeRemediation';

  /// 表單模式的查詢參數名稱與請求解除的值；沒有參數是申覆，其他值無效（路由導回 [measures]）。
  static const measureFormModeParam = 'mode';
  static const measureFormModeRemediation = 'remediation';

  /// 讀不到店家狀態的重試頁（GetMyStore 失敗而且沒有上一次的快照）
  static const storeUnavailable = '/seller/store-unavailable';

  /// App 強制更新攔截頁（`packages/hawkr_app_update/docs/spec.md`）
  static const forceUpdate = '/force-update';
}

/// 優惠券相關路徑
class CouponRoutes {
  static const editor = '/coupon/editor';
  static const add = '/coupon/add';
  static const detail = '/coupon/detail';
}

/// 設定相關路徑
class SettingsRoutes {
  static const profile = '/profile';
  static const chat = '/chat';
  static const chatAiAssistant = '/chat/ai-assistant';
  static const chatThread = '/chat/thread';
  static const shipping = '/settings/shipping';
  static const notifications = '/notifications';
  static const notificationSettings = '/notifications/settings';
  static const help = '/settings/help';

  /// 我的檢舉列表（backend-payuni#645）
  static const myReports = '/reports/me';

  /// 封鎖名單（backend-payuni#645）
  static const blockList = '/blocks';
}

/// 工具相關路徑
class ToolRoutes {
  static const photoEditor = '/photo-editor';
  static const richTextEditor = '/rich-text-editor';
  static const afterSales = '/after-sales';
  static const afterSalesDetail = '/after-sales/detail';
}

/// 配送異常相關路徑
class ShipmentAnomalyRoutes {
  static const dashboard = '/shipment-anomalies';
}

/// Hawkr Agent 採購線（賣家側）相關路徑
class ProcurementRoutes {
  /// 待回覆詢價清單——回價表單的唯一入口。
  static const sellerPublications = '/procurement/publications';

  /// 平台賣家回價表單。`?publicationId=` 指定要回覆哪一則詢價。
  static const sellerQuote = '/procurement/seller-quote';
}

// ══════════════════════════════════════════════════════════════════════════════
// 導航鍵
// ══════════════════════════════════════════════════════════════════════════════

/// 根導航鍵 — 與 MyApp.navigatorKey 共用，保持 Toast Overlay 正常運作
final rootNavigatorKey = MyApp.navigatorKey;

/// 分支導航鍵 — StatefulShellRoute 各 tab 獨立的 navigator
final _dashboardNavigatorKey = GlobalKey<NavigatorState>(
  debugLabel: 'dashboard',
);
final _categoryNavigatorKey = GlobalKey<NavigatorState>(debugLabel: 'category');
final _walletNavigatorKey = GlobalKey<NavigatorState>(debugLabel: 'wallet');
final _ordersNavigatorKey = GlobalKey<NavigatorState>(debugLabel: 'orders');
final _settingsNavigatorKey = GlobalKey<NavigatorState>(debugLabel: 'settings');

/// 從 extra 中安全地提取 `Map<String, dynamic>`
Map<String, dynamic> _extractExtraMap(GoRouterState state) {
  return stringKeyedRouteMap(state.extra);
}

/// 從 extra 中安全地提取指定類型的值，帶默認值
T _extractExtraValueWithDefault<T>(
  GoRouterState state,
  String key,
  T defaultValue,
) {
  final value = _extractExtraMap(state)[key];
  return value is T ? value : defaultValue;
}

T? _extractExtraAs<T>(GoRouterState state) {
  final extra = state.extra;
  return extra is T ? extra : null;
}

T? _extraMapValue<T>(Map<String, dynamic> extra, String key) {
  return routeValue<T>(extra, key);
}

Map<String, dynamic>? _extraNestedMap(Map<String, dynamic> extra, String key) {
  return routeNestedMap(extra, key);
}

List<T>? _extraTypedList<T>(Map<String, dynamic> extra, String key) {
  return routeTypedList<T>(extra, key);
}

List<Map<String, dynamic>>? _extraMapList(
  Map<String, dynamic> extra,
  String key,
) {
  return routeMapList(extra, key);
}

Widget _invalidRouteData(String label) {
  return Scaffold(
    key: const ValueKey('invalid-route-data'),
    body: Center(child: Text('$label資料無效，請返回上一頁重試')),
  );
}

/// 新舊訂單入口共用的 route adapter。將歷史列表的 Map payload
/// 收旂成 [OrderSummary]，確保所有狀態都進同一套新版詳情頁。
OrderSummary? _orderSummaryFromRouteExtra(
  Object? extra, {
  String? fallbackOrderSn,
}) {
  return orderSummaryFromRoutePayload(extra, fallbackOrderSn: fallbackOrderSn);
}

OrderSummary? _orderSummaryFromRouteState(GoRouterState state) {
  return _orderSummaryFromRouteExtra(
    state.extra,
    fallbackOrderSn:
        state.uri.queryParameters['orderSn'] ??
        state.uri.queryParameters['orderNo'],
  );
}

final _routeCustomerProvider = FutureProvider.autoDispose
    .family<Customer?, String>(
      (ref, accountId) => ref
          .read(customerRepositoryProvider)
          .getCustomerByAccountId(accountId),
    );

final _routeCouponProvider = FutureProvider.autoDispose.family<Coupon, String>((
  ref,
  templateId,
) async {
  final detail = await ref
      .read(couponRepositoryProvider)
      .getCouponDetail(templateId);
  return detail.template;
});

class _CustomerDetailRoute extends ConsumerWidget {
  const _CustomerDetailRoute({this.initial, this.accountId});

  final Customer? initial;
  final String? accountId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (initial != null) return CustomerDetailScreen(customer: initial!);
    final id = accountId?.trim() ?? '';
    if (id.isEmpty) return _invalidRouteData('顧客');
    return ref
        .watch(_routeCustomerProvider(id))
        .when(
          loading: () =>
              const Scaffold(body: Center(child: CircularProgressIndicator())),
          error: (_, __) => _invalidRouteData('顧客'),
          data: (customer) => customer == null
              ? _invalidRouteData('顧客')
              : CustomerDetailScreen(customer: customer),
        );
  }
}

class _CouponDetailRoute extends ConsumerWidget {
  const _CouponDetailRoute({this.initial, this.templateId});

  final Coupon? initial;
  final String? templateId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (initial != null) return CouponDetailScreen(coupon: initial!);
    final id = templateId?.trim() ?? '';
    if (id.isEmpty) return _invalidRouteData('優惠券');
    return ref
        .watch(_routeCouponProvider(id))
        .when(
          loading: () =>
              const Scaffold(body: Center(child: CircularProgressIndicator())),
          error: (_, __) => _invalidRouteData('優惠券'),
          data: (coupon) => CouponDetailScreen(coupon: coupon),
        );
  }
}

// ══════════════════════════════════════════════════════════════════════════════
// GoRouter Provider
// ══════════════════════════════════════════════════════════════════════════════

/// Navigator observers are injectable so hermetic app tests can exercise the
/// production router without booting the Firebase native plugin.
final appRouterObserversProvider = Provider<List<NavigatorObserver>>(
  (ref) => <NavigatorObserver>[
    if (isFirebaseInitialized)
      FirebaseAnalyticsObserver(analytics: FirebaseAnalytics.instance),
  ],
);

final goRouterProvider = Provider<GoRouter>((ref) {
  final sessionListenable = SessionChangeNotifier(ref);
  ref.onDispose(() => sessionListenable.dispose());

  return GoRouter(
    navigatorKey: rootNavigatorKey,
    initialLocation: AuthRoutes.welcome,
    debugLogDiagnostics: true,
    refreshListenable: sessionListenable,
    onException: (context, state, router) {
      debugPrint('[Router:onException] Unknown route: ${state.uri}, ignoring');
    },
    redirect: (context, state) {
      final session = ref.read(sessionProvider);
      final isLoggedIn = session.isLoggedIn;
      final location = redirectLocationOf(state);

      // onboarding gating 的單一真相來源（純函式，見 redirect_logic.dart）。
      // 認得 needsOnboarding：刪除帳號後重登停在 /auth 會被導去 /onboard，
      // 不再被「已登入即進 dashboard」的舊規則蓋過。
      final needsForceUpdate = ref.read(forceUpdateGateProvider).updateRequired;
      final target = resolveAuthRedirect(
        isLoggedIn: isLoggedIn,
        needsOnboarding: ref.read(needsOnboardingProvider),
        myStore: ref.read(myStoreSnapshotProvider),
        location: location,
        needsForceUpdate: needsForceUpdate,
        storeUnavailable: ref.read(storeUnavailableProvider),
      );

      debugPrint(
        '[Router:redirect] location=$location isLoggedIn=$isLoggedIn '
        'needsOnboarding=${ref.read(needsOnboardingProvider)} '
        'storeUnavailable=${ref.read(storeUnavailableProvider)} '
        'needsForceUpdate=$needsForceUpdate → ${target ?? '(stay)'}',
      );

      return target;
    },
    // GA4: 自動送 screen_view（root navigator）
    observers: ref.watch(appRouterObserversProvider),
    routes: [
      // ── 啟動 / 認證 ──
      GoRoute(
        path: AuthRoutes.welcome,
        builder: (context, state) => const WelcomeScreen(),
      ),
      GoRoute(
        path: AuthRoutes.auth,
        builder: (context, state) => const AuthScreen(),
      ),
      GoRoute(
        path: AuthRoutes.onboard,
        builder: (context, state) => const SellerInformationScreen(),
      ),

      // ── 主導航 (StatefulShellRoute) ──
      StatefulShellRoute.indexedStack(
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state, navigationShell) {
          return _MainShellScaffold(navigationShell: navigationShell);
        },
        branches: [
          // Tab 0: Dashboard
          StatefulShellBranch(
            navigatorKey: _dashboardNavigatorKey,
            routes: [
              GoRoute(
                path: MainRoutes.dashboard,
                builder: (context, state) => const DashboardScreen(),
              ),
            ],
          ),
          // Tab 1: Category / Page
          StatefulShellBranch(
            navigatorKey: _categoryNavigatorKey,
            routes: [
              GoRoute(
                path: MainRoutes.category,
                builder: (context, state) =>
                    const PageScreen(variant: PageEmptyVariant.activityEmpty),
              ),
            ],
          ),
          // Tab 2: Wallet
          StatefulShellBranch(
            navigatorKey: _walletNavigatorKey,
            routes: [
              GoRoute(
                path: MainRoutes.wallet,
                builder: (context, state) => const WalletScreen(),
              ),
            ],
          ),
          // Tab 3: Orders
          StatefulShellBranch(
            navigatorKey: _ordersNavigatorKey,
            routes: [
              GoRoute(
                path: MainRoutes.orders,
                builder: (context, state) => OrderListScreen(
                  initialAfterSaleFilter:
                      state.uri.queryParameters['filter'] == 'after_sale',
                ),
              ),
            ],
          ),
          // Tab 4: Settings / Profile
          StatefulShellBranch(
            navigatorKey: _settingsNavigatorKey,
            routes: [
              GoRoute(
                path: MainRoutes.settings,
                builder: (context, state) =>
                    const ProfileScreen(showSellerSection: true),
              ),
            ],
          ),
        ],
      ),

      // ── 子頁面 (parentNavigatorKey = root，覆蓋整個畫面) ──

      // -- Dashboard 子頁面 --
      GoRoute(
        path: SettingsRoutes.profile,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const ProfileScreen(),
      ),
      GoRoute(
        path: SettingsRoutes.chat,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const ChatListScreen(),
      ),
      GoRoute(
        path: SettingsRoutes.chatAiAssistant,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const AiAssistantScreen(),
      ),
      GoRoute(
        path: SettingsRoutes.chatThread,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          return ChatThreadScreen(
            title: _extraMapValue<String>(extra, 'title') ?? '',
            conversationId: _extraMapValue<String>(extra, 'conversationId'),
            participantId: _extraMapValue<String>(extra, 'participantId'),
          );
        },
      ),
      GoRoute(
        path: SettingsRoutes.shipping,
        parentNavigatorKey: rootNavigatorKey,
        // builder: (context, state) => const ShippingSettingsScreen(),
        builder: (context, state) => const ShippingSettingsScreen(),
      ),
      GoRoute(
        path: CustomerRoutes.customers,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const CustomerManagementScreen(),
      ),
      GoRoute(
        path: CustomerRoutes.subscribers,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const SubscriberListScreen(),
      ),
      GoRoute(
        path: CustomerRoutes.detail,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          return _CustomerDetailRoute(
            initial: _extractExtraAs<Customer>(state),
            accountId:
                state.uri.queryParameters['accountId'] ??
                state.uri.queryParameters['customerId'],
          );
        },
      ),
      GoRoute(
        path: CustomerRoutes.spendingHistory,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          final customerId = _extraMapValue<String>(extra, 'customerId');
          final customerName = _extraMapValue<String>(extra, 'customerName');
          if (customerId == null || customerName == null) {
            return _invalidRouteData('顧客消費紀錄');
          }
          return CustomerSpendingHistoryScreen(
            customerName: customerName,
            customerId: customerId,
            customerPhone: _extraMapValue<String>(extra, 'customerPhone') ?? '',
            totalSpent: _extraMapValue<int>(extra, 'totalSpent') ?? 0,
            totalRefund: _extraMapValue<int>(extra, 'totalRefund') ?? 0,
            records:
                _extraMapValue<List<SpendingRecord>>(extra, 'records') ??
                const [],
            orders:
                _extraMapValue<List<OrderRecord>>(extra, 'orders') ?? const [],
            coupons: _extraMapValue<List<Coupon>>(extra, 'coupons') ?? const [],
            initialTab: _extractExtraValueWithDefault(state, 'initialTab', 0),
          );
        },
      ),
      GoRoute(
        path: CustomerRoutes.orderDetail,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          final order = _extraMapValue<OrderRecord>(extra, 'order');
          final customerName = _extraMapValue<String>(extra, 'customerName');
          if (order == null || customerName == null) {
            return _invalidRouteData('顧客訂單');
          }
          return CustomerOrderDetailScreen(
            order: order,
            customerName: customerName,
          );
        },
      ),
      GoRoute(
        path: ToolRoutes.afterSales,
        parentNavigatorKey: rootNavigatorKey,
        redirect: (context, state) => '${MainRoutes.orders}?filter=after_sale',
      ),
      GoRoute(
        path: ToolRoutes.afterSalesDetail,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final caseNo =
              _extractExtraAs<String>(state)?.trim() ??
              state.uri.queryParameters['caseNo']?.trim() ??
              '';
          return caseNo.isEmpty
              ? _invalidRouteData('售後案件')
              : AfterSalesDetailScreen(caseNo: caseNo);
        },
      ),

      // -- Order 子頁面 --
      GoRoute(
        path: OrderRoutes.details,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final order = _orderSummaryFromRouteState(state);
          return order == null
              ? const Scaffold(body: Center(child: Text('訂單資料無效')))
              : OrderDetailScreen(order: order);
        },
      ),
      GoRoute(
        path: OrderRoutes.matched,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) =>
            const OrderListScreen(showBackButton: true),
      ),
      GoRoute(
        path: OrderRoutes.paymentDetails,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => OrderPaymentDetailsScreen(
          order: _orderSummaryFromRouteState(state),
        ),
      ),
      GoRoute(
        path: OrderRoutes.receipts,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) =>
            OrderReceiptsScreen(order: _orderSummaryFromRouteState(state)),
      ),
      GoRoute(
        path: OrderRoutes.receiptsDetail,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const OrderDetailsReceiptsScreen(),
      ),
      GoRoute(
        path: OrderRoutes.dispute,
        parentNavigatorKey: rootNavigatorKey,
        // Retired static dispute mock. Seller after-sales cases and decisions
        // must come from the server-backed after-sales workflow.
        redirect: (context, state) => ToolRoutes.afterSales,
      ),
      GoRoute(
        path: OrderRoutes.logistics,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          // 🔴 原本 `?? 'HOME'`：沒帶 lgsType 的導航會把超商單當宅配去查 PayUni。
          // 改成空字串，畫面走「未知物流類型」的 fail-safe 呈現。
          final lgsType =
              _extraMapValue<String>(extra, 'lgsType') ??
              state.uri.queryParameters['lgsType'] ??
              '';
          return LogisticsTrackingScreen(
            shipTradeNo:
                _extraMapValue<String>(extra, 'shipTradeNo') ??
                state.uri.queryParameters['shipTradeNo'],
            lgsType: lgsType,
            tradeType:
                _extraMapValue<int>(extra, 'tradeType') ??
                int.tryParse(state.uri.queryParameters['tradeType'] ?? ''),
            returnOdno:
                _extraMapValue<String>(extra, 'returnOdno') ??
                state.uri.queryParameters['returnOdno'],
            orderSn:
                _extraMapValue<String>(extra, 'orderSn') ??
                state.uri.queryParameters['orderSn'],
          );
        },
      ),
      GoRoute(
        path: OrderRoutes.view,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) =>
            const OrderListScreen(showBackButton: true),
      ),
      GoRoute(
        path: OrderRoutes.matchedDetail,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final order = _orderSummaryFromRouteState(state);
          if (order == null) {
            return const Scaffold(body: Center(child: Text('訂單資料無效')));
          }
          return OrderDetailScreen(order: order);
        },
      ),

      // -- Product 子頁面 (wizard chain) --
      GoRoute(
        path: ProductRoutes.picker,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          return ProductSetupPickerScreen(
            initialType: _extraMapValue<String>(extra, 'initialType'),
            isEditMode: _extractExtraValueWithDefault(
              state,
              'isEditMode',
              false,
            ),
            existingProduct: _extraNestedMap(extra, 'existingProduct'),
          );
        },
      ),
      GoRoute(
        path: ProductRoutes.info,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          return ProductSetupInfoScreen(
            skuId: extra['skuId'],
            initialProductName: _extraMapValue<String>(
              extra,
              'initialProductName',
            ),
            initialOrigin: _extraMapValue<String>(extra, 'initialOrigin'),
            initialDescription: _extraMapValue<String>(
              extra,
              'initialDescription',
            ),
            initialCategory: _extraMapValue<String>(extra, 'initialCategory'),
            initialSubCategory: _extraMapValue<String>(
              extra,
              'initialSubCategory',
            ),
            initialCatalogId: _extraMapValue<int>(extra, 'initialCatalogId'),
            isCandidateCategory: _extractExtraValueWithDefault(
              state,
              'isCandidateCategory',
              false,
            ),
            l1Id: _extraMapValue<int>(extra, 'l1Id'),
            l1Name: _extraMapValue<String>(extra, 'l1Name'),
            l2Id: _extraMapValue<int>(extra, 'l2Id'),
            l2Name: _extraMapValue<String>(extra, 'l2Name'),
            l3Name: _extraMapValue<String>(extra, 'l3Name'),
            isEditMode: _extractExtraValueWithDefault(
              state,
              'isEditMode',
              false,
            ),
            existingProduct: _extraNestedMap(extra, 'existingProduct'),
          );
        },
      ),
      GoRoute(
        path: ProductRoutes.images,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          final productName = _extraMapValue<String>(extra, 'productName');
          final category = _extraMapValue<String>(extra, 'category');
          final subCategory = _extraMapValue<String>(extra, 'subCategory');
          if (productName == null || category == null || subCategory == null) {
            return _invalidRouteData('商品圖片');
          }
          return ProductSetupImageScreen(
            skuId: extra['skuId'],
            productName: productName,
            category: category,
            subCategory: subCategory,
            catalogId: _extraMapValue<int>(extra, 'catalogId'),
            isCandidateCategory: _extractExtraValueWithDefault(
              state,
              'isCandidateCategory',
              false,
            ),
            l1Id: _extraMapValue<int>(extra, 'l1Id'),
            l1Name: _extraMapValue<String>(extra, 'l1Name'),
            l2Id: _extraMapValue<int>(extra, 'l2Id'),
            l2Name: _extraMapValue<String>(extra, 'l2Name'),
            l3Name: _extraMapValue<String>(extra, 'l3Name'),
            origin: _extraMapValue<String>(extra, 'origin'),
            description: _extraMapValue<String>(extra, 'description'),
            descriptionBlocks: _extraMapList(extra, 'descriptionBlocks'),
            descriptionStagedTokens:
                _extraTypedList<String>(extra, 'descriptionStagedTokens') ??
                const [],
            isEditMode: _extractExtraValueWithDefault(
              state,
              'isEditMode',
              false,
            ),
            existingProduct: _extraNestedMap(extra, 'existingProduct'),
          );
        },
      ),
      GoRoute(
        path: ProductRoutes.specs,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          final images = _extraTypedList<Uint8List>(extra, 'images');
          final productName = _extraMapValue<String>(extra, 'productName');
          final category = _extraMapValue<String>(extra, 'category');
          final subCategory = _extraMapValue<String>(extra, 'subCategory');
          if (images == null ||
              productName == null ||
              category == null ||
              subCategory == null) {
            return _invalidRouteData('商品規格');
          }
          return ProductSetupSpecScreen(
            images: images,
            skuId: extra['skuId'],
            productName: productName,
            category: category,
            subCategory: subCategory,
            catalogId: _extraMapValue<int>(extra, 'catalogId'),
            isCandidateCategory: _extractExtraValueWithDefault(
              state,
              'isCandidateCategory',
              false,
            ),
            l1Id: _extraMapValue<int>(extra, 'l1Id'),
            l1Name: _extraMapValue<String>(extra, 'l1Name'),
            l2Id: _extraMapValue<int>(extra, 'l2Id'),
            l2Name: _extraMapValue<String>(extra, 'l2Name'),
            l3Name: _extraMapValue<String>(extra, 'l3Name'),
            origin: _extraMapValue<String>(extra, 'origin'),
            description: _extraMapValue<String>(extra, 'description'),
            descriptionBlocks: _extraMapList(extra, 'descriptionBlocks'),
            descriptionStagedTokens:
                _extraTypedList<String>(extra, 'descriptionStagedTokens') ??
                const [],
            isEditMode: _extractExtraValueWithDefault(
              state,
              'isEditMode',
              false,
            ),
            existingProduct: _extraNestedMap(extra, 'existingProduct'),
          );
        },
      ),
      GoRoute(
        path: ProductRoutes.mediaPicker,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const ProductSetupMediaPickerScreen(),
      ),
      GoRoute(
        path: ProductRoutes.preview,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          final product = _extraMapValue<Map<String, dynamic>>(
            extra,
            'product',
          );
          if (product == null) return _invalidRouteData('商品預覽');
          return ProductPreviewScreen(product: product);
        },
      ),
      GoRoute(
        path: ProductRoutes.reviews,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          final sellerId =
              _extraMapValue<String>(extra, 'sellerId') ??
              state.uri.queryParameters['sellerId'];
          final skuId =
              _extraMapValue<String>(extra, 'skuId') ??
              state.uri.queryParameters['skuId'];
          if (sellerId == null || skuId == null) {
            return _invalidRouteData('商品評價');
          }
          return ProductReviewsScreen(
            sellerId: sellerId,
            skuId: skuId,
            productName:
                _extraMapValue<String>(extra, 'productName') ??
                state.uri.queryParameters['productName'] ??
                '',
          );
        },
      ),
      // -- Post 子頁面 --
      GoRoute(
        path: PostRoutes.mediaPicker,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          return PostMediaPickerScreen(
            title: _extractExtraValueWithDefault(state, 'title', 'New Post'),
          );
        },
      ),
      GoRoute(
        path: PostRoutes.compose,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          final contentType = _extraMapValue<String>(extra, 'contentType');
          if (contentType == null) return _invalidRouteData('貼文媒體');
          return PostComposeScreen(
            imagePreview: _extraMapValue<Uint8List>(extra, 'imagePreview'),
            mediaBytes: _extraMapValue<Uint8List>(extra, 'mediaBytes'),
            contentType: contentType,
          );
        },
      ),
      GoRoute(
        path: PostRoutes.edit,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          return PostEditScreen(post: extra['post']);
        },
      ),
      GoRoute(
        path: PostRoutes.feed,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          final initialIndex = _extraMapValue<int>(extra, 'initialIndex') ?? 0;
          return PageHomeFeedScreen(
            initialIndex: initialIndex,
            initialPostId: _extraMapValue<int>(extra, 'postId'),
            sellerName: _extraMapValue<String>(extra, 'sellerName') ?? '',
          );
        },
      ),

      // -- Seller 子頁面 --
      GoRoute(
        path: SellerRoutes.certificationUpload,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          return SellerCertificationUploadScreen(
            sellerType: _extraMapValue<String>(extra, 'sellerType'),
          );
        },
      ),
      GoRoute(
        path: SellerRoutes.photoTools,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          return SellerPhotoToolsScreen(
            imagePreview: _extraMapValue<Uint8List>(extra, 'imagePreview'),
            mediaBytes: _extraMapValue<Uint8List>(extra, 'mediaBytes'),
            contentType: _extractExtraValueWithDefault(
              state,
              'contentType',
              'image',
            ),
          );
        },
      ),
      GoRoute(
        path: SellerRoutes.suspensionNotice,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const SuspensionNoticeScreen(),
      ),
      GoRoute(
        path: SellerRoutes.storeUnavailable,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const StoreUnavailableScreen(),
      ),
      // parentNavigatorKey 是全螢幕攔截的關鍵：少了它會被 ShellRoute 的
      // bottom nav 包住，強更畫面下方還留著可點的分頁（`packages/hawkr_app_update/docs/spec.md`）。
      GoRoute(
        path: SellerRoutes.forceUpdate,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const ForceUpdateScreen(),
      ),
      GoRoute(
        path: SellerRoutes.measures,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const SellerAppealScreen(),
      ),
      GoRoute(
        path: SellerRoutes.measureRequest,
        parentNavigatorKey: rootNavigatorKey,
        redirect: (context, state) => measureRequestRedirect(state.uri),
        builder: (context, state) => SellerMeasureRequestFormScreen(
          measureId: state.pathParameters['measureId']!,
          kind: measureFormKindOf(state.uri)!,
        ),
      ),

      // -- Wallet 子頁面 --
      GoRoute(
        path: CouponRoutes.editor,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final coupon = _extractExtraAs<Coupon>(state);
          return CouponEditorScreen(coupon: coupon);
        },
      ),
      GoRoute(
        path: CouponRoutes.detail,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => _CouponDetailRoute(
          initial: _extractExtraAs<Coupon>(state),
          templateId:
              state.uri.queryParameters['templateId'] ??
              state.uri.queryParameters['id'],
        ),
      ),
      GoRoute(
        path: CouponRoutes.add,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          return AddCouponScreen(
            preselectedUser: _extraMapValue<PreselectedUser>(
              extra,
              'preselectedUser',
            ),
          );
        },
      ),

      // -- Settings 子頁面 --
      GoRoute(
        path: SettingsRoutes.notifications,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const NotificationsScreen(),
      ),
      GoRoute(
        path: SettingsRoutes.notificationSettings,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const NotificationSettingsScreen(),
      ),
      GoRoute(
        path: SettingsRoutes.myReports,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const MyReportsScreen(),
      ),
      GoRoute(
        path: SettingsRoutes.blockList,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const BlockListScreen(),
      ),
      GoRoute(
        path: SettingsRoutes.help,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const HelpCenterScreen(),
      ),

      // -- 配送異常 dashboard --
      GoRoute(
        path: ShipmentAnomalyRoutes.dashboard,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const ShipmentAnomalyDashboardScreen(),
      ),

      // -- Hawkr Agent 採購線：待回覆詢價清單（回價表單的入口）--
      GoRoute(
        path: ProcurementRoutes.sellerPublications,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => const SellerPublicationListScreen(),
      ),

      // -- Hawkr Agent 採購線：平台賣家回價表單 --
      GoRoute(
        path: ProcurementRoutes.sellerQuote,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) => SellerQuoteFormScreen(
          publicationId: state.uri.queryParameters['publicationId'] ?? '',
        ),
      ),

      // -- 共用工具頁面 --
      GoRoute(
        path: ToolRoutes.photoEditor,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final args = _extractExtraAs<PhotoEditorArgs>(state);
          return ImageEditorScreen(
            initialImage: args?.file,
            category: args?.category ?? 'product',
          );
        },
      ),
      GoRoute(
        path: ToolRoutes.richTextEditor,
        parentNavigatorKey: rootNavigatorKey,
        builder: (context, state) {
          final extra = _extractExtraMap(state);
          return RichTextEditor(
            initialContent: _extraMapValue<String>(extra, 'initialContent'),
            initialBlocks: _extraMapList(extra, 'initialBlocks'),
            maxLength: _extractExtraValueWithDefault(state, 'maxLength', 3000),
          );
        },
      ),
    ],
  );
});

// ══════════════════════════════════════════════════════════════════════════════
// 主導航 Shell Scaffold
// ══════════════════════════════════════════════════════════════════════════════

/// 主導航 Shell Scaffold — 取代原本的 MainNavigationScreen
class _MainShellScaffold extends ConsumerStatefulWidget {
  const _MainShellScaffold({required this.navigationShell});

  final StatefulNavigationShell navigationShell;

  @override
  ConsumerState<_MainShellScaffold> createState() => _MainShellScaffoldState();
}

class _MainShellScaffoldState extends ConsumerState<_MainShellScaffold> {
  @override
  Widget build(BuildContext context) {
    final currentIndex = widget.navigationShell.currentIndex;
    final navigationLocation = GoRouterState.of(context).uri.toString();
    final useLightStatusBar = currentIndex == 1;
    final overlayStyle =
        (useLightStatusBar
                ? SystemUiOverlayStyle.light
                : SystemUiOverlayStyle.dark)
            .copyWith(
              statusBarColor: Colors.transparent,
              systemNavigationBarColor: AppColors.bgPrimary,
              systemNavigationBarIconBrightness: Brightness.dark,
            );

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: overlayStyle,
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: SellerFloatingNavigationLayout(
          visibilityResetKey: '$navigationLocation#$currentIndex',
          page: widget.navigationShell,
          navigation: AppBottomNavigationBar(
            currentIndex: NavIndex.values[currentIndex],
            onTap: (index) => _onTap(index),
          ),
        ),
      ),
    );
  }

  void _onTap(int index) {
    // 切換到不同 tab 時觸發刷新
    if (index != widget.navigationShell.currentIndex) {
      if (index == 1) {
        ref.read(pageTabRefreshProvider.notifier).increment();
      }
      if (index == 3) {
        ref.read(ordersTabRefreshProvider.notifier).increment();
      }
      if (index == 4) {
        ref.read(settingsTabRefreshProvider.notifier).increment();
      }
    }
    widget.navigationShell.goBranch(
      index,
      // 點擊當前 tab 時 pop 到該 tab 的根頁面
      initialLocation: index == widget.navigationShell.currentIndex,
    );
  }
}
