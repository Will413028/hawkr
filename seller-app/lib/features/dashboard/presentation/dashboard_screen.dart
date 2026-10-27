import 'dart:async';

import '../../seller/presentation/seller_store_avatar.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart'
    show Dashboard, OrderStatusCounts;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart'
    show MeasureState;
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/layout/app_text_size.dart';
import '../../../core/router/tab_refresh_provider.dart';
import '../../../core/router/app_router.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/use_on_became_visible.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../core/session/session_provider.dart';
import '../../seller/data/account_scope.dart';
import '../../seller/data/current_store.dart';
import '../../seller/data/measures.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_glass_chrome_surface.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_page_section.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/seller_gradient_canvas.dart';
import '../../order/data/order_status_notifier.dart';
import '../../order/data/order_tab_mapper.dart';
import '../../seller/data/seller_status_refresh.dart';
import '../../seller/presentation/seller_certification_gate.dart';
import '../../settings/presentation/text_size_settings_sheet.dart';
import '../../shipment_anomaly/data/shipment_anomaly_providers.dart';
import '../data/dashboard_chat_unread_provider.dart';
import '../data/dashboard_display.dart';
import '../data/dashboard_notification_unread_provider.dart';
import '../data/dashboard_provider.dart';
import 'shipment_anomaly_alert_banner.dart';
import 'termination_notice_banner.dart';

/// 固定導覽 chrome 在大字級時只增加必要高度，不縮小系統文字。
double dashboardHeaderTextScaleExtra(TextScaler textScaler) {
  final extra = (textScaler.scale(1) - 1).clamp(0.0, 0.4) * 72;
  return extra.clamp(0.0, 32.0).toDouble();
}

/// 下拉重新整理時儀表板讀取失敗的提示。
const dashboardRefreshFailedNotice = '重新整理失敗，請稍後再試';

int dashboardMetricColumnCount(double width) =>
    width >= HkBreakpoints.tablet ? 4 : 2;

int dashboardQuickActionColumnCount(double width) => width >= 330 ? 3 : 2;

class DashboardScreen extends HookConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final storeId = ref.watch(sellerStoreIdProvider);

    /// 重讀儀表板一輪（見 [StoreDashboard.refresh]）；還沒有店家時沒有東西可讀，回 null（不是讀取失敗）。
    Future<bool?> reloadDashboard({bool fresh = false, Duration? maxAge}) {
      if (storeId == null) return Future<bool?>.value(null);
      return ref
          .read(storeDashboardProvider(storeId).notifier)
          .refresh(fresh: fresh, maxAge: maxAge);
    }

    // 畫面是否看得到由 widget 樹決定（主導航的分頁被隱藏、或被不透明頁面蓋住時看不到）。第一次開啟與從看不到變成
    // 看得到時重讀，30 秒內讀過就不讀（同錢包的 statusStaleTime）。
    // 隱藏期間收到訂單推播時記下「儀表板已過期」，變成看得到時就讀最新的（不套用 30 秒略過）。
    final staleWhileHidden = useRef(false);
    final visible = useOnBecameVisible(() {
      final stale = staleWhileHidden.value;
      staleWhileHidden.value = false;
      unawaited(
        reloadDashboard(fresh: stale, maxAge: stale ? null : statusStaleTime),
      );
    }, includeInitial: true);
    final visibleNow = useRef(visible)..value = visible;
    final latestReload = useRef(reloadDashboard)..value = reloadDashboard;

    useEffect(() {
      final subscription = OrderStatusNotifier.instance.orderEventStream.listen(
        (event) {
          debugPrint('[Dashboard] Order event received: ${event.eventType}');
          ref.invalidate(dashboardChatUnreadProvider);
          // 看得到就立刻讀最新的；被隱藏的分頁（IndexedStack 仍在）不讀，等變成看得到時再讀。
          if (visibleNow.value) {
            unawaited(latestReload.value(fresh: true));
          } else {
            staleWhileHidden.value = true;
          }
        },
      );

      return () {
        subscription.cancel();
      };
    }, const []);

    useOnAppLifecycleStateChange((_, state) {
      if (state == AppLifecycleState.resumed) {
        unawaited(ref.read(sellerStatusRefreshProvider.notifier).refresh());
        if (visibleNow.value) unawaited(latestReload.value());
      }
    });

    final dashboardAsync = ref.watch(currentDashboardProvider);
    final dashboard = dashboardAsync?.value;
    final chatUnread = ref.watch(dashboardChatUnreadProvider).value ?? 0;
    final mine = ref.watch(myStoreSnapshotProvider);
    final notificationUnread =
        ref.watch(dashboardNotificationUnreadProvider).value ?? 0;
    final sellerDisplayName = mine?.storeName ?? 'Hawkr';
    final headerHeight =
        92 + dashboardHeaderTextScaleExtra(MediaQuery.textScalerOf(context));

    Future<void> refresh() async {
      ref.invalidate(dashboardChatUnreadProvider);
      ref.invalidate(dashboardNotificationUnreadProvider);
      ref.invalidate(shipmentAnomalyStatsProvider);
      // 終止預告卡讀預定生效的措施（帳號層級）：下拉時一併重讀，伺服器剛撤銷或生效的預告才會消失。
      final accountId = signedInAccountId(ref.read(sessionProvider));
      final scheduledProvider = accountId == null
          ? null
          : myMeasuresProvider(accountId, MeasureState.MEASURE_STATE_SCHEDULED);
      if (scheduledProvider != null) ref.invalidate(scheduledProvider);
      final dashboardRead = reloadDashboard().catchError((Object error) {
        HKLog.logW('[Dashboard] pull refresh: dashboard read threw: $error');
        return false;
      });
      // 其他資料讀不到不影響儀表板的提示，也不讓 refresh() 丟出例外（下拉只要等它們讀完）。
      try {
        await Future.wait([
          ref.read(sellerStatusRefreshProvider.notifier).refresh(),
          ref.read(dashboardChatUnreadProvider.future),
          ref.read(dashboardNotificationUnreadProvider.future),
          ref.read(shipmentAnomalyStatsProvider.future),
          if (scheduledProvider != null) ref.read(scheduledProvider.future),
        ]);
      } catch (error) {
        HKLog.logW('[Dashboard] pull refresh: other reads failed: $error');
      }
      // 下拉讀取失敗才提示（null 是店家還沒讀到、沒有東西可讀，不是失敗）；背景重讀與第一次讀取失敗另有頂端的錯誤區塊，不提示。
      if (await dashboardRead == false && context.mounted) {
        FloatingToast.error(context, dashboardRefreshFailedNotice);
      }
    }

    Future<void> openProductCreation() async {
      await context.push(ProductRoutes.picker);
    }

    // 店家還沒讀到（沒有 storeId）時儀表板也還沒開始讀：和第一次讀取一樣顯示讀取中，不是一頁「—」。
    final Widget? status = dashboard != null
        ? null
        : dashboardAsync != null &&
              dashboardAsync.hasError &&
              !dashboardAsync.isLoading
        ? _DashboardLoadFailure(onRetry: () => unawaited(reloadDashboard()))
        : const _DashboardLoading();

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: const SystemUiOverlayStyle(
        statusBarColor: AppColors.topBarBackground,
        statusBarIconBrightness: Brightness.dark,
        statusBarBrightness: Brightness.light,
      ),
      child: SellerGradientCanvas(
        child: SafeArea(
          top: false,
          child: Column(
            children: [
              HkGlassChromeSurface(
                surfaceKey: const ValueKey('seller-dashboard-glass-header'),
                child: SafeArea(
                  bottom: false,
                  child: SizedBox(
                    height: headerHeight,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: HkSpace.lg,
                      ),
                      child: DashboardHeader(
                        sellerName: sellerDisplayName,
                        storeCode: mine?.storeCode,
                        notificationUnread: notificationUnread,
                        chatUnread: chatUnread,
                        onAvatarTap: () => context.go('/main/settings'),
                        onNotificationTap: () async {
                          await context.push('/notifications');
                          ref.invalidate(dashboardNotificationUnreadProvider);
                        },
                        onChatTap: () async {
                          await context.push('/chat');
                          ref.invalidate(dashboardChatUnreadProvider);
                        },
                      ),
                    ),
                  ),
                ),
              ),
              Expanded(
                child: FruitRefreshIndicator(
                  onRefresh: refresh,
                  child: SingleChildScrollView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    padding: const EdgeInsets.fromLTRB(
                      HkSpace.lg,
                      HkSpace.lg,
                      HkSpace.lg,
                      120,
                    ),
                    child: Center(
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 720),
                        child: DashboardOverviewContent(
                          dashboard: dashboard,
                          status: status,
                          onAddProduct: openProductCreation,
                          onOpenShipping: () =>
                              openShippingSettings(context, ref),
                          onOpenOrderTab: (index) {
                            ref
                                .read(ordersInitialTabProvider.notifier)
                                .request(index);
                            context.go('/main/orders');
                          },
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class DashboardHeader extends StatelessWidget {
  const DashboardHeader({
    super.key,
    required this.sellerName,
    required this.storeCode,
    required this.notificationUnread,
    required this.chatUnread,
    required this.onAvatarTap,
    required this.onNotificationTap,
    required this.onChatTap,
  });

  final String sellerName;
  final String? storeCode;
  final int notificationUnread;
  final int chatUnread;
  final VoidCallback onAvatarTap;
  final VoidCallback onNotificationTap;
  final VoidCallback onChatTap;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Semantics(
          button: true,
          label: '前往設定',
          child: Material(
            type: MaterialType.transparency,
            child: InkWell(
              onTap: onAvatarTap,
              customBorder: const CircleBorder(),
              child: Container(
                width: HkSpace.control,
                height: HkSpace.control,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  border: Border.all(
                    color: AppColors.neutralWhite.withValues(alpha: 0.92),
                    width: 1.5,
                  ),
                  boxShadow: HkShadows.sm,
                ),
                child: const ClipOval(child: SellerStoreAvatar(size: 48)),
              ),
            ),
          ),
        ),
        const SizedBox(width: HkSpace.md),
        Expanded(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                sellerName,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.h1,
              ),
              if (storeCode != null)
                Text(
                  'ID $storeCode',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.metaMuted,
                ),
            ],
          ),
        ),
        const SizedBox(width: HkSpace.sm),
        const DashboardTextSizeAction(),
        const SizedBox(width: HkSpace.xs),
        _HeaderAction(
          icon: HkIconName.notification,
          tooltip: '通知',
          unread: notificationUnread,
          onTap: onNotificationTap,
        ),
        const SizedBox(width: HkSpace.xs),
        _HeaderAction(
          icon: HkIconName.chat,
          tooltip: '訊息',
          unread: chatUnread,
          onTap: onChatTap,
        ),
      ],
    );
  }
}

/// Seller-home shortcut for the device-wide Small / Medium / Large text choice.
///
/// The selected value stays in [appTextSizeProvider]; tapping the shortcut
/// opens the existing settings sheet.
class DashboardTextSizeAction extends ConsumerWidget {
  const DashboardTextSizeAction({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final textSize = ref.watch(appTextSizeProvider);
    final semanticLabel = '字體大小，目前為${textSize.label}';

    return _HeaderAction(
      key: const ValueKey('dashboard-text-size-action'),
      icon: HkIconName.textSize,
      iconKey: const ValueKey('dashboard-text-size-icon'),
      tooltip: semanticLabel,
      unread: 0,
      onTap: () => showTextSizeSettingsSheet(context),
    );
  }
}

class _HeaderAction extends StatelessWidget {
  const _HeaderAction({
    super.key,
    required this.icon,
    this.iconKey,
    required this.tooltip,
    required this.unread,
    required this.onTap,
  });

  final HkIconName icon;
  final Key? iconKey;
  final String tooltip;
  final int unread;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: unread > 0 ? '$tooltip，$unread 則未讀' : tooltip,
      child: IconButton(
        onPressed: onTap,
        tooltip: tooltip,
        constraints: const BoxConstraints.tightFor(
          width: HkSpace.control,
          height: HkSpace.control,
        ),
        icon: Stack(
          clipBehavior: Clip.none,
          children: [
            HkIcon(
              icon,
              key: iconKey,
              size: HkControl.icon,
              color: AppIconColors.active,
            ),
            if (unread > 0)
              Positioned(
                top: -7,
                right: -9,
                child: Container(
                  constraints: const BoxConstraints(
                    minWidth: 18,
                    minHeight: 18,
                  ),
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  decoration: BoxDecoration(
                    color: AppColors.error,
                    borderRadius: BorderRadius.circular(HkRadius.pill),
                    border: Border.all(color: AppColors.neutralWhite, width: 1),
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    unread > 99 ? '99+' : '$unread',
                    style: const TextStyle(
                      color: AppColors.neutralWhite,
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                      height: 1,
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// 儀表板的內容。[dashboard] 是 null（還沒讀到、或第一次讀取失敗）時，今日待辦、營運快覽、訂單進度的每一塊都顯示
/// 「—」，[status] 放在最上方說明原因（讀取中、或失敗加重試）；「常用功能」不受影響，一直可用。
class DashboardOverviewContent extends StatelessWidget {
  const DashboardOverviewContent({
    super.key,
    required this.dashboard,
    required this.onOpenOrderTab,
    required this.onAddProduct,
    required this.onOpenShipping,
    this.status,
    this.showAnomalyBanner = true,
  });

  final Dashboard? dashboard;
  final Widget? status;
  final ValueChanged<int> onOpenOrderTab;
  final VoidCallback onAddProduct;
  final VoidCallback onOpenShipping;
  final bool showAnomalyBanner;

  @override
  Widget build(BuildContext context) {
    final counts = orderStatusCountsOf(dashboard);
    final revenue = revenueStat(dashboard);
    final orders = ordersStat(dashboard);
    final customers = customersStat(dashboard);
    final views = productViewsStat(dashboard);
    final followers = newFollowersStat(dashboard);
    String count(int Function(OrderStatusCounts counts) pick) =>
        orderCountText(counts, pick);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (status != null) ...[status!, const SizedBox(height: HkSpace.lg)],
        _TodayFocusCard(
          dashboard: dashboard,
          onOpenPending: () => onOpenOrderTab(0),
          onOpenAfterSales: () => context.go('/main/orders?filter=after_sale'),
        ),
        if (showAnomalyBanner) ...[
          const SizedBox(height: HkSpace.lg),
          const TerminationNoticeBanner(),
          const ShipmentAnomalyAlertBanner(),
        ],
        const SizedBox(height: HkSpace.section),
        HkPageSection(
          title: '營運快覽',
          description: '關鍵數據與較前 7 日變化',
          child: Column(
            children: [
              _RevenueCard(stat: revenue),
              const SizedBox(height: HkSpace.md),
              _MetricGrid(
                items: [
                  _MetricData(
                    label: '近 7 日訂單',
                    stat: orders,
                    icon: HkIconName.receipt,
                    onTap: null,
                  ),
                  _MetricData(
                    label: '近 7 日顧客',
                    stat: customers,
                    icon: HkIconName.users,
                    onTap: () => context.push('/customers'),
                  ),
                  _MetricData(
                    label: '商品瀏覽',
                    stat: views,
                    icon: HkIconName.chart,
                    onTap: null,
                  ),
                  _MetricData(
                    label: '新追蹤者',
                    stat: followers,
                    icon: HkIconName.notification,
                    onTap: () => context.push('/subscribers'),
                  ),
                ],
              ),
            ],
          ),
        ),
        const SizedBox(height: HkSpace.section),
        HkPageSection(
          title: '訂單進度',
          description: '點選狀態即可進入對應訂單',
          child: _OrderProgressCard(
            items: [
              _OrderData(
                label: sellerOrderTabLabels[0],
                value: count((c) => c.pendingShipmentCount),
                icon: HkIconName.box,
                onTap: () => onOpenOrderTab(0),
              ),
              _OrderData(
                label: sellerOrderTabLabels[1],
                value: count((c) => c.inProgressCount),
                icon: HkIconName.truck,
                onTap: () => onOpenOrderTab(1),
              ),
              // 「已結束」不是「已完成」：這個數字含已取消、全額退款，不是成交數。
              // 顯示名與訂單列表 tab 共用 sellerOrderTabLabels（理由見 order_tab_mapper.dart）。
              _OrderData(
                label: sellerOrderTabLabels[2],
                value: count((c) => c.closedCount),
                icon: HkIconName.check,
                onTap: () => onOpenOrderTab(2),
              ),
              _OrderData(
                label: '售後服務',
                value: count((c) => c.aftersalesCount),
                icon: HkIconName.returnOrder,
                onTap: () => context.go('/main/orders?filter=after_sale'),
              ),
            ],
          ),
        ),
        const SizedBox(height: HkSpace.section),
        HkPageSection(
          title: '常用功能',
          description: '快速開始今天最常使用的工作',
          child: _QuickActionsCard(
            actions: [
              _QuickActionData(
                label: '商品管理',
                icon: HkIconName.goods,
                onTap: () => context.go('/main/category'),
              ),
              _QuickActionData(
                label: '新增商品',
                icon: HkIconName.add,
                onTap: onAddProduct,
              ),
              _QuickActionData(
                label: '發佈貼文',
                icon: HkIconName.post,
                onTap: () => context.push('/post/media-picker'),
              ),
              _QuickActionData(
                label: '客戶管理',
                icon: HkIconName.users,
                onTap: () => context.push('/customers'),
              ),
              _QuickActionData(
                label: '優惠券',
                icon: HkIconName.coupon,
                onTap: () => context.push('/coupon/editor'),
              ),
              _QuickActionData(
                label: '配送設定',
                icon: HkIconName.truck,
                onTap: onOpenShipping,
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _TodayFocusCard extends StatelessWidget {
  const _TodayFocusCard({
    required this.dashboard,
    required this.onOpenPending,
    required this.onOpenAfterSales,
  });

  /// 目前讀到的儀表板。還沒有（第一次讀取中或失敗，null）時，狀況由頂端的狀態區塊說明，這張卡只顯示「—」，不說
  /// 「暫時無法取得待辦」：那句只在讀到儀表板、但伺服器沒設訂單狀態時才成立。訂單狀態沒設時不是 0，不能說「待辦已清空」。
  final Dashboard? dashboard;
  final VoidCallback onOpenPending;
  final VoidCallback onOpenAfterSales;

  @override
  Widget build(BuildContext context) {
    final known = orderStatusCountsOf(dashboard);
    final openingText = orderCountText(known, (c) => c.pendingShipmentCount);
    final afterSalesText = orderCountText(known, (c) => c.aftersalesCount);
    final total = known == null
        ? 0
        : known.pendingShipmentCount + known.aftersalesCount;
    final (title, body) = dashboard == null
        ? ('今日待辦', null)
        : known == null
        ? ('暫時無法取得待辦', '下拉重新整理，或到訂單頁查看')
        : total == 0
        ? ('今日待辦已清空', '營運狀態良好，可繼續整理商品與貼文。')
        : ('今天有 $total 件事待處理', '先完成出貨與售後回覆，降低買家等待時間。');
    return HkFrostedCardSurface(
      radius: HkRadius.xl,
      shadows: HkShadows.lg,
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.xxl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: HkSpace.control,
                  height: HkSpace.control,
                  decoration: BoxDecoration(
                    color: known != null && total == 0
                        ? AppColors.brandPrimaryStrong.withValues(alpha: 0.14)
                        : AppColors.brandSecondary.withValues(alpha: 0.20),
                    shape: BoxShape.circle,
                  ),
                  child: Center(
                    child: HkIcon(
                      known != null && total == 0
                          ? HkIconName.check
                          : HkIconName.clock,
                      size: HkControl.icon,
                      color: AppIconColors.active,
                    ),
                  ),
                ),
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(title, style: AppTypography.h1),
                      const SizedBox(height: HkSpace.xs),
                      if (body != null)
                        Text(body, style: AppTypography.metaMuted),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: HkSpace.xl),
            LayoutBuilder(
              builder: (context, constraints) {
                final stack = constraints.maxWidth < 360;
                final pendingButton = FilledButton.icon(
                  onPressed: onOpenPending,
                  icon: const HkIcon(
                    HkIconName.box,
                    size: 18,
                    color: AppIconColors.onBrand,
                  ),
                  label: HkButtonLabel('待出貨 $openingText'),
                );
                final afterSalesButton = OutlinedButton.icon(
                  onPressed: onOpenAfterSales,
                  icon: const HkIcon(HkIconName.returnOrder, size: 18),
                  label: HkButtonLabel('售後 $afterSalesText'),
                );
                if (stack) {
                  return Column(
                    children: [
                      SizedBox(width: double.infinity, child: pendingButton),
                      const SizedBox(height: HkSpace.sm),
                      SizedBox(width: double.infinity, child: afterSalesButton),
                    ],
                  );
                }
                return Row(
                  children: [
                    Expanded(child: pendingButton),
                    const SizedBox(width: HkSpace.sm),
                    Expanded(child: afterSalesButton),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _RevenueCard extends StatelessWidget {
  const _RevenueCard({required this.stat});

  final DashboardStat stat;

  @override
  Widget build(BuildContext context) {
    return HkFrostedCardSurface(
      radius: HkRadius.lg,
      shadows: HkShadows.card,
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.xl),
        child: LayoutBuilder(
          builder: (context, constraints) {
            final icon = Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: AppColors.brandPrimaryStrong,
                borderRadius: BorderRadius.circular(HkRadius.md),
              ),
              child: const Center(
                child: HkIcon(
                  HkIconName.wallet,
                  size: 22,
                  color: AppIconColors.onBrand,
                ),
              ),
            );
            final value = stat.value;
            final trend = stat.change;
            final valueStyle = AppTypography.display;
            final valuePainter = TextPainter(
              text: TextSpan(text: value, style: valueStyle),
              textDirection: Directionality.of(context),
              textScaler: MediaQuery.textScalerOf(context),
            )..layout();
            final trendPainter = TextPainter(
              text: TextSpan(
                text: trend,
                style: AppTypography.caption.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              textDirection: Directionality.of(context),
              textScaler: MediaQuery.textScalerOf(context),
            )..layout();
            // The amount and weekly comparison must each retain their natural
            // width. A phone, large text, or a longer amount can trigger reflow.
            final inlineWidth =
                44 +
                HkSpace.md +
                valuePainter.width +
                HkSpace.md +
                trendPainter.width +
                HkSpace.sm * 2;
            valuePainter.dispose();
            trendPainter.dispose();

            final label = Text('近 7 日營收', style: AppTypography.metaMuted);
            final amount = Text(
              value,
              key: const ValueKey('dashboard-revenue-amount'),
              style: valueStyle,
            );
            final caption = stat.caption;
            final captionText = caption == null
                ? null
                : Text(
                    caption,
                    key: const ValueKey('dashboard-revenue-caption'),
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textMuted,
                    ),
                  );
            if (inlineWidth > constraints.maxWidth) {
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      icon,
                      const SizedBox(width: HkSpace.md),
                      Expanded(child: label),
                    ],
                  ),
                  const SizedBox(height: HkSpace.md),
                  amount,
                  ?captionText,
                  const SizedBox(height: HkSpace.sm),
                  _TrendPill(text: trend, growth: stat.growth),
                ],
              );
            }
            return Row(
              children: [
                icon,
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [label, amount, ?captionText],
                  ),
                ),
                const SizedBox(width: HkSpace.md),
                _TrendPill(text: trend, growth: stat.growth),
              ],
            );
          },
        ),
      ),
    );
  }
}

class _MetricData {
  const _MetricData({
    required this.label,
    required this.stat,
    required this.icon,
    this.onTap,
  });

  final String label;
  final DashboardStat stat;
  final HkIconName icon;
  final VoidCallback? onTap;
}

class _MetricGrid extends StatelessWidget {
  const _MetricGrid({required this.items});

  final List<_MetricData> items;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        const gap = HkSpace.md;
        final columns = dashboardMetricColumnCount(constraints.maxWidth);
        final width = (constraints.maxWidth - gap * (columns - 1)) / columns;
        return Wrap(
          spacing: gap,
          runSpacing: gap,
          children: [
            for (final item in items)
              SizedBox(
                width: width,
                child: _MetricTile(item: item),
              ),
          ],
        );
      },
    );
  }
}

class _MetricTile extends StatelessWidget {
  const _MetricTile({required this.item});

  final _MetricData item;

  @override
  Widget build(BuildContext context) {
    final content = HkFrostedCardSurface(
      radius: HkRadius.lg,
      shadows: HkShadows.sm,
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                HkIcon(item.icon, size: 20, color: AppIconColors.active),
                const SizedBox(width: HkSpace.sm),
                Expanded(
                  child: Text(
                    item.label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.meta,
                  ),
                ),
              ],
            ),
            const SizedBox(height: HkSpace.md),
            Text(item.stat.value, style: AppTypography.h1),
            const SizedBox(height: HkSpace.xs),
            Text(
              item.stat.change,
              style: AppTypography.caption.copyWith(
                color: _trendColor(item.stat.growth),
              ),
            ),
            if (item.stat.caption case final caption?)
              Text(
                caption,
                key: ValueKey('dashboard-metric-caption-${item.label}'),
                style: AppTypography.caption,
              ),
          ],
        ),
      ),
    );
    if (item.onTap == null) return content;
    return Semantics(
      button: true,
      label: '${item.label} ${item.stat.value}',
      child: Material(
        type: MaterialType.transparency,
        child: InkWell(
          onTap: item.onTap,
          borderRadius: BorderRadius.circular(HkRadius.lg),
          child: content,
        ),
      ),
    );
  }
}

class _OrderData {
  const _OrderData({
    required this.label,
    required this.value,
    required this.icon,
    required this.onTap,
  });

  final String label;

  /// 件數；訂單狀態讀不到時是 [dashboardUnset]。
  final String value;
  final HkIconName icon;
  final VoidCallback onTap;
}

class _OrderProgressCard extends StatelessWidget {
  const _OrderProgressCard({required this.items});

  final List<_OrderData> items;

  @override
  Widget build(BuildContext context) {
    return HkFrostedCardSurface(
      radius: HkRadius.xl,
      shadows: HkShadows.card,
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.md),
        child: LayoutBuilder(
          builder: (context, constraints) {
            final columns = dashboardMetricColumnCount(constraints.maxWidth);
            const gap = HkSpace.sm;
            final width =
                (constraints.maxWidth - gap * (columns - 1)) / columns;
            return Wrap(
              spacing: gap,
              runSpacing: gap,
              children: [
                for (final item in items)
                  SizedBox(
                    width: width,
                    child: _OrderProgressItem(item: item),
                  ),
              ],
            );
          },
        ),
      ),
    );
  }
}

class _OrderProgressItem extends StatelessWidget {
  const _OrderProgressItem({required this.item});

  final _OrderData item;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: item.value == dashboardUnset
          ? '${item.label} 暫時無法取得'
          : '${item.label} ${item.value} 筆',
      child: Material(
        type: MaterialType.transparency,
        child: InkWell(
          onTap: item.onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: 88),
            child: Padding(
              padding: const EdgeInsets.all(HkSpace.md),
              child: Row(
                children: [
                  Container(
                    width: 36,
                    height: 36,
                    decoration: BoxDecoration(
                      color: AppColors.brandPrimaryStrong.withValues(
                        alpha: 0.12,
                      ),
                      shape: BoxShape.circle,
                    ),
                    child: Center(
                      child: HkIcon(
                        item.icon,
                        size: 19,
                        color: AppIconColors.active,
                      ),
                    ),
                  ),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(item.value, style: AppTypography.h1),
                        Text(
                          item.label,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.caption,
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _QuickActionData {
  const _QuickActionData({
    required this.label,
    required this.icon,
    required this.onTap,
  });

  final String label;
  final HkIconName icon;
  final VoidCallback onTap;
}

class _QuickActionsCard extends StatelessWidget {
  const _QuickActionsCard({required this.actions});

  final List<_QuickActionData> actions;

  @override
  Widget build(BuildContext context) {
    return HkFrostedCardSurface(
      radius: HkRadius.xl,
      shadows: HkShadows.card,
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.md),
        child: LayoutBuilder(
          builder: (context, constraints) {
            final columns = dashboardQuickActionColumnCount(
              constraints.maxWidth,
            );
            const gap = HkSpace.sm;
            final width =
                (constraints.maxWidth - gap * (columns - 1)) / columns;
            return Wrap(
              spacing: gap,
              runSpacing: gap,
              children: [
                for (final action in actions)
                  SizedBox(
                    width: width,
                    child: _QuickAction(action: action),
                  ),
              ],
            );
          },
        ),
      ),
    );
  }
}

class _QuickAction extends StatelessWidget {
  const _QuickAction({required this.action});

  final _QuickActionData action;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: action.label,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: action.onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: 86),
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: HkSpace.sm,
                vertical: HkSpace.md,
              ),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  HkIcon(
                    action.icon,
                    size: HkControl.icon,
                    color: AppIconColors.active,
                  ),
                  const SizedBox(height: HkSpace.sm),
                  Text(
                    action.label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.meta.copyWith(
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _TrendPill extends StatelessWidget {
  const _TrendPill({required this.text, required this.growth});

  final String text;

  /// 成長百分比；null（沒有可比的基準）畫中性色。
  final int? growth;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: HkSpace.sm,
        vertical: HkSpace.xs,
      ),
      decoration: BoxDecoration(
        color: _trendColor(growth).withValues(alpha: 0.10),
        borderRadius: BorderRadius.circular(HkRadius.pill),
      ),
      child: Text(
        text,
        style: AppTypography.caption.copyWith(
          color: _trendColor(growth),
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}

class _DashboardLoading extends StatelessWidget {
  const _DashboardLoading();

  @override
  Widget build(BuildContext context) {
    return HkFrostedCardSurface(
      radius: HkRadius.xl,
      shadows: HkShadows.sm,
      child: const SizedBox(
        height: 180,
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              SizedBox(
                width: 24,
                height: 24,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: AppColors.brandPrimaryStrong,
                ),
              ),
              SizedBox(height: HkSpace.md),
              Text('正在整理今日營運資料'),
            ],
          ),
        ),
      ),
    );
  }
}

/// 第一次讀取失敗：頂端的錯誤區塊（沿用錢包的 [HkAsyncStatePanel]）加重試，下方每一塊顯示「—」。
class _DashboardLoadFailure extends StatelessWidget {
  const _DashboardLoadFailure({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return HkAsyncStatePanel(
      kind: HkAsyncStateKind.error,
      title: '暫時無法載入總覽',
      description: '請檢查網路後再試一次',
      actionLabel: '重新載入',
      onAction: onRetry,
    );
  }
}

Color _trendColor(int? growth) {
  if (growth == null || growth == 0) return AppColors.textSecondary;
  if (growth < 0) return AppColors.dangerStrong;
  return AppColors.brandPrimaryStrong;
}
