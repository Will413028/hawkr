import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/widgets/hk_secondary_cta_tabs.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/main_top_bar.dart';
import '../../../shared/widgets/top_bar_provider.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../../core/theme/app_colors.dart';
import '../data/after_sale_order_lookup.dart';
import '../data/after_sale_status_notifier.dart';
import '../data/order_model.dart';
import '../data/order_status.dart';
import '../data/order_status_notifier.dart';
import '../../../shared/widgets/tab_switch_provider.dart';
import '../../../shared/widgets/glass_tab_bar.dart';
import 'order_card.dart';
import 'order_list_controller.dart';

enum _OrderQuickFilter { all, pendingAction }

class OrderListScreen extends ConsumerStatefulWidget {
  const OrderListScreen({
    super.key,
    this.initialTabIndex = 0,
    this.initialPendingAction = false,
  });
  final int initialTabIndex;
  final bool initialPendingAction;

  @override
  ConsumerState<OrderListScreen> createState() => _OrderListScreenState();
}

class _OrderListScreenState extends ConsumerState<OrderListScreen>
    with TopBarProvider, WidgetsBindingObserver {
  late int _selectedTab;
  _OrderQuickFilter _quickFilter = _OrderQuickFilter.all;

  void _refreshCurrentTab() {
    ref
        .read(orderListControllerProvider.notifier)
        .switchTab(OrderTabFilter.tabFilters[_selectedTab]);
    // 售後快照（卡片展開區依賴）背景重撈 — 保留舊資料，避免 UI 短暫看到 null
    ref.read(afterSaleOrderLookupProvider.notifier).refresh();
  }

  Future<void> _refreshOrders() async {
    await Future.wait([
      ref.read(orderListControllerProvider.notifier).refresh(),
      ref.read(afterSaleOrderLookupProvider.notifier).refresh(),
    ]);
  }

  List<Order> _visibleOrders(
    List<Order> orders,
    Map<String, AfterSaleCaseSnapshot> afterSales,
  ) {
    if (_quickFilter == _OrderQuickFilter.all) return orders;
    return orders
        .where((order) => _needsBuyerAction(order, afterSales[order.orderSn]))
        .toList();
  }

  bool _needsBuyerAction(Order order, AfterSaleCaseSnapshot? afterSale) {
    if (afterSale?.needRefundPage == true) return true;
    if (order.needRefundPage) return true;
    if (order.status == OrderStatus.pendingPayment.value) return true;
    if (order.status == OrderStatus.delivered.value && afterSale == null) {
      return true;
    }
    if (order.status == OrderStatus.completed.value &&
        !order.isReviewed &&
        order.canReview) {
      return true;
    }
    return false;
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _selectedTab = widget.initialTabIndex;
    _quickFilter = widget.initialPendingAction
        ? _OrderQuickFilter.pendingAction
        : _OrderQuickFilter.all;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _refreshCurrentTab();
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && mounted) {
      _refreshCurrentTab();
    }
  }

  @override
  PreferredSizeWidget buildTopBar(BuildContext context) {
    return buildMainTopBar(context);
  }

  @override
  Widget build(BuildContext context) {
    final ordersAsync = ref.watch(orderListControllerProvider);
    final controller = ref.read(orderListControllerProvider.notifier);
    final afterSaleLookup = ref.watch(afterSaleOrderLookupProvider);
    final afterSales =
        afterSaleLookup.value ?? const <String, AfterSaleCaseSnapshot>{};

    // Tab 切換回訂單頁 → 強制刷新
    ref.listen(currentTabProvider, (prev, next) {
      if (next == 3 && prev != 3 && mounted) {
        _refreshCurrentTab();
      }
    });

    // FCM Silent Push: 訂單狀態變更（賣家出貨、後端推進狀態等）→ 自動刷新列表
    ref.listen(orderStatusStreamProvider, (_, next) {
      next.whenData((_) {
        if (mounted) _refreshCurrentTab();
      });
    });

    // FCM Silent Push: 售後案件狀態變更（賣家回覆售後等）→ 自動刷新列表
    ref.listen(afterSaleStatusStreamProvider, (_, next) {
      next.whenData((_) {
        if (mounted) _refreshCurrentTab();
      });
    });

    return Container(
      color: AppColors.bgPrimary,
      child: Column(
        children: [
          SizedBox(height: 6.h),
          // Tab filter
          _OrderTabs(
            selectedIndex: _selectedTab,
            onTap: (i) {
              setState(() => _selectedTab = i);
              controller.switchTab(OrderTabFilter.tabFilters[i]);
            },
          ),
          _OrderFilterBar(
            orders: ordersAsync.value ?? const <Order>[],
            afterSales: afterSales,
            selectedFilter: _quickFilter,
            needsBuyerAction: _needsBuyerAction,
            onChanged: (filter) {
              if (_quickFilter == filter) return;
              setState(() => _quickFilter = filter);
            },
          ),
          // Order list
          Expanded(
            child: ordersAsync.when(
              loading: () => const TruckLoadingOverlay(),
              error: (err, _) => Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(
                      AppIcons.receiptLong,
                      size: 48.sp,
                      color: AppIconColors.disabled,
                    ),
                    SizedBox(height: 12.h),
                    Text(
                      '載入訂單失敗',
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textSecondary,
                      ),
                    ),
                    SizedBox(height: 12.h),
                    GestureDetector(
                      onTap: () => controller.refresh(),
                      child: Container(
                        padding: EdgeInsets.symmetric(
                          horizontal: 20.w,
                          vertical: 8.h,
                        ),
                        decoration: BoxDecoration(
                          color: AppColors.primary,
                          borderRadius: BorderRadius.circular(999),
                        ),
                        child: Text(
                          '重試',
                          style: TextStyle(
                            fontSize: 14,
                            color: AppColors.bgCard,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              data: (orders) {
                final visibleOrders = _visibleOrders(orders, afterSales);
                return visibleOrders.isEmpty
                    ? HkEmptyState(
                        key: const ValueKey('order-empty-state'),
                        iconKey: const ValueKey('order-empty-state-icon'),
                        icon: HkIconName.emptyOrders,
                        message: _quickFilter == _OrderQuickFilter.pendingAction
                            ? '暫無待處理訂單'
                            : '暫無訂單',
                      )
                    : FruitRefreshListView(
                        onRefresh: _refreshOrders,
                        onLoadMore: controller.loadMore,
                        hasMore: controller.hasMore,
                        padding: EdgeInsets.fromLTRB(20.w, 12.h, 20.w, 12.h),
                        loadMoreWidget: const TruckLoadingOverlay(
                          message: '載入中...',
                        ),
                        itemCount: visibleOrders.length,
                        itemBuilder: (context, index) => Padding(
                          padding: EdgeInsets.only(bottom: 12.h),
                          child: OrderCard(order: visibleOrders[index]),
                        ),
                      );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _OrderFilterBar extends StatelessWidget {
  const _OrderFilterBar({
    required this.orders,
    required this.afterSales,
    required this.selectedFilter,
    required this.needsBuyerAction,
    required this.onChanged,
  });

  final List<Order> orders;
  final Map<String, AfterSaleCaseSnapshot> afterSales;
  final _OrderQuickFilter selectedFilter;
  final bool Function(Order order, AfterSaleCaseSnapshot? afterSale)
  needsBuyerAction;
  final ValueChanged<_OrderQuickFilter> onChanged;

  @override
  Widget build(BuildContext context) {
    final pendingCount = orders
        .where((order) => needsBuyerAction(order, afterSales[order.orderSn]))
        .length;

    return ColoredBox(
      color: AppColors.bgPrimary,
      child: Padding(
        padding: EdgeInsets.fromLTRB(24.w, 8.h, 24.w, 0),
        child: HkSecondaryCtaTabs(
          key: const ValueKey('order-quick-filter-tabs'),
          trackKey: const ValueKey('order-quick-filter-track'),
          itemKeyPrefix: 'order-quick-filter-tab',
          labels: ['全部 (${orders.length})', '待處理 ($pendingCount)'],
          selectedIndex: selectedFilter == _OrderQuickFilter.all ? 0 : 1,
          onSelected: (index) => onChanged(
            index == 0
                ? _OrderQuickFilter.all
                : _OrderQuickFilter.pendingAction,
          ),
        ),
      ),
    );
  }
}

/// Tab 列 — 待付款 / 待出貨 / 配送中 / 已完成。
class _OrderTabs extends StatelessWidget {
  const _OrderTabs({required this.selectedIndex, required this.onTap});
  final int selectedIndex;
  final void Function(int) onTap;

  @override
  Widget build(BuildContext context) {
    return HkGlassTabBar(
      labels: OrderTabFilter.tabLabels,
      selectedIndex: selectedIndex,
      onTap: onTap,
      height: 48.h,
      labelKeyPrefix: 'order_tab_label_',
    );
  }
}
