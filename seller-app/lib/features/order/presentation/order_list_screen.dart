import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/router/tab_refresh_provider.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/glass_tab_bar.dart';
import '../../../shared/widgets/green_header_bar.dart';
import '../../../shared/widgets/logo_home_button.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/after_sale_status_notifier.dart';
import '../data/after_sale_model.dart';
import '../data/order_model.dart';
import '../data/order_repository.dart';
import '../data/order_status_notifier.dart';
import '../data/order_tab_mapper.dart';
import '../data/seller_after_sale_lookup.dart';
import 'order_card.dart';

// tab 顯示名的單一來源在 order_tab_mapper.dart（與各 tab 的 status 集合同檔），
// 不在這裡另寫字面值 —— 第三個是「已結束」不是「已完成」，理由見該處說明。
const _kTabLabels = sellerOrderTabLabels;

enum _OrderQuickFilter { all, pendingAction, afterSale }

class OrderListScreen extends ConsumerStatefulWidget {
  const OrderListScreen({
    super.key,
    this.showBackButton = false,
    this.initialAfterSaleFilter = false,
  });

  /// true = 從其他頁面 push 進來（顯示返回箭頭）
  /// false = 從底部 tab 進入（顯示 logo）
  final bool showBackButton;

  /// true = 從首頁售後服務或通知導入配送中 tab，並預設套用售後處理篩選。
  final bool initialAfterSaleFilter;

  @override
  ConsumerState<OrderListScreen> createState() => _OrderListScreenState();
}

class _OrderListScreenState extends ConsumerState<OrderListScreen>
    with WidgetsBindingObserver {
  late final OrderRepository _repo;

  int _tabIndex = 0;
  _OrderQuickFilter _quickFilter = _OrderQuickFilter.all;

  List<OrderSummary> _allOrders = [];
  final Set<String> _loadingOrderIds = {};

  bool _initialLoading = true;
  bool _loadingMore = false;
  bool _pendingInitialReload = false;
  bool _hasMore = true;
  int _page = 1;
  static const int _pageSize = 20;

  // 直接訂閱 notifier 的 broadcast stream，獨立於 Riverpod StreamProvider 的
  // build-cycle 綁定時序，確保 FCM silent push 事件不漏單。
  StreamSubscription<OrderStatusEvent>? _orderStatusSub;
  StreamSubscription<AfterSaleStatusEvent>? _afterSaleStatusSub;

  void _applyInitialTabRequest(int request, {bool rebuild = false}) {
    void apply() {
      if (request == ordersAfterSaleInTransitTabRequest) {
        _tabIndex = 1;
        _quickFilter = _OrderQuickFilter.afterSale;
        return;
      }
      _tabIndex = request;
      _quickFilter = _OrderQuickFilter.all;
    }

    if (rebuild && mounted) {
      setState(apply);
    } else {
      apply();
    }
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _repo = ref.read(orderRepositoryProvider);
    final requestedTab = ref.read(ordersInitialTabProvider);
    if (widget.initialAfterSaleFilter) {
      _applyInitialTabRequest(ordersAfterSaleInTransitTabRequest);
    } else if (requestedTab != null) {
      _applyInitialTabRequest(requestedTab);
      ref.read(ordersInitialTabProvider.notifier).consume();
    }

    _orderStatusSub = OrderStatusNotifier.instance.orderEventStream.listen((
      event,
    ) {
      HKLog.logI('[OrderListScreen] 收到訂單事件，觸發刷新: $event');
      if (mounted) _requestReloadOrders();
    });
    _afterSaleStatusSub = AfterSaleStatusNotifier.instance.eventStream.listen((
      event,
    ) {
      HKLog.logI('[OrderListScreen] 收到售後事件，觸發刷新: $event');
      if (mounted) {
        ref.invalidate(sellerAfterSaleLookupProvider);
        _requestReloadOrders();
      }
    });

    _loadOrders(initial: true);
  }

  @override
  void didUpdateWidget(covariant OrderListScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!oldWidget.initialAfterSaleFilter && widget.initialAfterSaleFilter) {
      _applyInitialTabRequest(ordersAfterSaleInTransitTabRequest);
      _requestReloadOrders();
    }
  }

  @override
  void dispose() {
    _orderStatusSub?.cancel();
    _afterSaleStatusSub?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && mounted) {
      _requestReloadOrders();
    }
  }

  @override
  void activate() {
    super.activate();
    if (!_initialLoading && _allOrders.isEmpty) {
      _loadOrders(initial: true);
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_initialLoading && _allOrders.isEmpty && !_loadingMore) {
      _loadOrders(initial: true);
    }
  }

  Future<void> _requestReloadOrders() async {
    if (_loadingMore) {
      _pendingInitialReload = true;
      return;
    }
    await _loadOrders(initial: true);
  }

  void _flushPendingReloadIfNeeded() {
    if (!_pendingInitialReload || !mounted) return;
    _pendingInitialReload = false;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _loadOrders(initial: true);
    });
  }

  Future<void> _loadOrders({
    bool initial = false,
    bool showLoading = true,
  }) async {
    if (_loadingMore) return;
    if (!_hasMore && !initial) return;

    setState(() {
      if (initial) {
        if (showLoading) _initialLoading = true;
        if (showLoading) _allOrders = [];
        _page = 1;
        _hasMore = true;
      }
      _loadingMore = true;
    });

    try {
      final statusFilter = statusFilterForTab(_tabIndex);
      final list = await _repo.fetchMatchedOrders(
        page: _page,
        pageSize: _pageSize,
        status: statusFilter,
      );
      if (!mounted) return;
      setState(() {
        if (initial) {
          _allOrders = List<OrderSummary>.of(list);
        } else {
          _allOrders.addAll(list);
        }
        _hasMore = list.length >= _pageSize;
        if (_hasMore) _page++;
        _loadingMore = false;
        _initialLoading = false;
      });
      _flushPendingReloadIfNeeded();
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _loadingMore = false;
        _initialLoading = false;
      });
      _flushPendingReloadIfNeeded();
    }
  }

  String _orderIdOf(OrderSummary order) =>
      order.orderSn.isNotEmpty ? order.orderSn : order.id;

  List<OrderSummary> _tabOrders() =>
      _allOrders.where((o) => tabForOrder(o) == _tabIndex).toList();

  List<OrderSummary> _visibleOrders(Map<String, AfterSaleCase> afterSales) {
    final orders = _tabOrders();
    return switch (_quickFilter) {
      _OrderQuickFilter.all => orders,
      _OrderQuickFilter.pendingAction =>
        orders
            .where(
              (order) =>
                  _needsSellerAction(order, afterSales[_orderIdOf(order)]),
            )
            .toList(),
      _OrderQuickFilter.afterSale =>
        orders
            .where((order) => afterSales.containsKey(_orderIdOf(order)))
            .toList(),
    };
  }

  bool _needsSellerAction(OrderSummary order, AfterSaleCase? afterSale) {
    if (afterSale != null &&
        (afterSale.status == AfterSaleCaseStatus.pending.value ||
            afterSale.status == AfterSaleCaseStatus.returnReceived.value)) {
      return true;
    }

    final statusCode = order.statusCode ?? int.tryParse(order.subtitle) ?? -1;
    if (!pendingOrderStatuses.contains(statusCode)) return false;

    if (order.shippingMethod == ShippingMethod.taqbin.value) {
      if (order.shipmentStatus == ShippingStatus.tcatCalled.value) {
        return order.labelUrl.isNotEmpty;
      }
      return true;
    }

    return true;
  }

  Future<void> _refresh() async {
    await Future.wait([
      _loadOrders(initial: true, showLoading: false),
      ref.read(sellerAfterSaleLookupProvider.notifier).refresh(),
    ]);
  }

  void _handleTabChanged(int index) {
    if (_tabIndex == index) return;
    setState(() {
      _tabIndex = index;
      if (_quickFilter == _OrderQuickFilter.afterSale && index != 1) {
        _quickFilter = _OrderQuickFilter.all;
      }
    });
    _requestReloadOrders();
  }

  void _setOrderLoading(String orderId, bool loading) {
    setState(() {
      if (loading) {
        _loadingOrderIds.add(orderId);
      } else {
        _loadingOrderIds.remove(orderId);
      }
    });
  }

  bool _onScrollNotification(ScrollNotification n) {
    if (n is ScrollUpdateNotification) {
      final remaining = n.metrics.maxScrollExtent - n.metrics.pixels;
      if (remaining < 200 && _hasMore) _loadOrders();
    }
    return false;
  }

  // ══════════════════════════════════════════════════════════════════════════
  @override
  Widget build(BuildContext context) {
    ref.listen<int?>(ordersInitialTabProvider, (prev, next) {
      if (next == null) return;
      _applyInitialTabRequest(next, rebuild: true);
      ref.read(ordersInitialTabProvider.notifier).consume();
      _requestReloadOrders();
    });

    if (!widget.showBackButton) {
      ref.listen<int>(ordersTabRefreshProvider, (prev, next) {
        if (prev != next) _requestReloadOrders();
      });
    }
    // orderStatusStream / afterSaleStatusStream 已在 initState 直接訂閱 notifier，
    // 不再經 Riverpod StreamProvider 橋接，避免 keepAlive/build-cycle 時序衝突。

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            GreenHeaderBar(
              includeTopSafeArea: true,
              title: '我的訂單',
              centerTitle: true,
              showBackButton: widget.showBackButton,
              onBackPressed: widget.showBackButton
                  ? () => Navigator.of(context).maybePop()
                  : null,
              leading: widget.showBackButton ? null : const LogoHomeButton(),
            ),
            _buildTabBar(),
            _buildFilterBar(),
            Expanded(
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 720),
                  child: _buildBody(),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterBar() {
    final afterSaleLookup = ref.watch(sellerAfterSaleLookupProvider);
    final afterSales = afterSaleLookup.value ?? const <String, AfterSaleCase>{};
    final tabOrders = _tabOrders();
    final pendingCount = tabOrders
        .where(
          (order) => _needsSellerAction(order, afterSales[_orderIdOf(order)]),
        )
        .length;
    final afterSaleCount = tabOrders
        .where((order) => afterSales.containsKey(_orderIdOf(order)))
        .length;
    final showAfterSaleFilter = _tabIndex == 1;

    return ColoredBox(
      color: AppColors.bgPrimary,
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: SizedBox(
              width: double.infinity,
              child: Wrap(
                spacing: HkSpace.sm,
                runSpacing: HkSpace.sm,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  HkIcon(
                    HkIconName.filter,
                    size: 18.sp,
                    color: AppIconColors.disabled,
                  ),
                  _OrderFilterChip(
                    label: '全部',
                    count: tabOrders.length,
                    selected: _quickFilter == _OrderQuickFilter.all,
                    onTap: () {
                      if (_quickFilter == _OrderQuickFilter.all) return;
                      setState(() => _quickFilter = _OrderQuickFilter.all);
                    },
                  ),
                  _OrderFilterChip(
                    label: '待處理',
                    count: pendingCount,
                    selected: _quickFilter == _OrderQuickFilter.pendingAction,
                    activeColor: AppColors.warning,
                    onTap: () {
                      if (_quickFilter == _OrderQuickFilter.pendingAction) {
                        return;
                      }
                      setState(
                        () => _quickFilter = _OrderQuickFilter.pendingAction,
                      );
                    },
                  ),
                  if (showAfterSaleFilter) ...[
                    _OrderFilterChip(
                      label: '售後處理',
                      count: afterSaleCount,
                      selected: _quickFilter == _OrderQuickFilter.afterSale,
                      activeColor: AppColors.error,
                      onTap: () {
                        if (_quickFilter == _OrderQuickFilter.afterSale) return;
                        setState(
                          () => _quickFilter = _OrderQuickFilter.afterSale,
                        );
                      },
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildTabBar() {
    return ColoredBox(
      color: AppColors.bgPrimary,
      child: Padding(
        padding: const EdgeInsets.only(top: HkSpace.xs),
        child: HkGlassTabBar(
          labels: _kTabLabels,
          selectedIndex: _tabIndex,
          onTap: _handleTabChanged,
          labelKeyPrefix: 'seller_order_tab_',
        ),
      ),
    );
  }

  Widget _buildBody() {
    if (_initialLoading) {
      return const TruckLoadingOverlay(message: '載入中...');
    }

    final afterSaleLookup = ref.watch(sellerAfterSaleLookupProvider);
    final afterSales = afterSaleLookup.value ?? const <String, AfterSaleCase>{};
    final orders = _visibleOrders(afterSales);
    if (orders.isEmpty) {
      return HkEmptyState(
        key: const ValueKey('seller-orders-empty-state'),
        iconKey: const ValueKey('seller-orders-empty-state-icon'),
        icon: HkIconName.box,
        message: switch (_quickFilter) {
          _OrderQuickFilter.pendingAction => '暫無待處理訂單',
          _OrderQuickFilter.afterSale => '暫無售後處理訂單',
          _OrderQuickFilter.all => '暫無訂單',
        },
        description: '買家下單後會自動出現在這裡。',
        topPadding: 40,
      );
    }

    return NotificationListener<ScrollNotification>(
      onNotification: _onScrollNotification,
      child: FruitRefreshIndicator(
        onRefresh: _refresh,
        child: ListView.builder(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 120),
          itemCount: orders.length,
          itemBuilder: (_, i) {
            final order = orders[i];
            final orderId = order.orderSn.isNotEmpty ? order.orderSn : order.id;
            final isPending = pendingOrderStatuses.contains(order.statusCode);
            return Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: OrderCard(
                key: ValueKey(orderId),
                order: order,
                isPending: isPending,
                isLoading: _loadingOrderIds.contains(orderId),
                onSetLoading: _setOrderLoading,
                onReload: () => _loadOrders(initial: true, showLoading: false),
                onReloadAndSwitchTab: (tab) {
                  if (mounted) setState(() => _tabIndex = tab);
                  _loadOrders(initial: true, showLoading: false);
                },
              ),
            );
          },
        ),
      ),
    );
  }
}

class _OrderFilterChip extends StatelessWidget {
  const _OrderFilterChip({
    required this.label,
    required this.count,
    required this.selected,
    required this.onTap,
    this.activeColor = AppColors.primary,
  });

  final String label;
  final int count;
  final bool selected;
  final VoidCallback onTap;
  final Color activeColor;

  @override
  Widget build(BuildContext context) {
    final foreground = selected ? AppColors.bgCard : AppTypography.colorPrimary;
    final borderColor = selected
        ? activeColor
        : AppTypography.colorDisabled.withValues(alpha: 0.25);

    return Material(
      color: selected ? activeColor : AppColors.bgCard,
      borderRadius: BorderRadius.circular(999.r),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(999.r),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          curve: Curves.easeOut,
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 8.h),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(999.r),
            border: Border.all(color: borderColor, width: 1),
            boxShadow: selected
                ? const [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 6,
                      offset: Offset(0, 2),
                    ),
                  ]
                : null,
          ),
          child: Center(
            widthFactor: 1,
            heightFactor: 1,
            child: Text(
              '$label ($count)',
              style: AppTypography.meta.copyWith(
                color: foreground,
                fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                height: 1.2,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
