import '../../../core/session/my_store.dart';
import '../../../features/seller/data/current_store.dart';
import '../../../features/seller/data/seller_status_refresh.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show SystemUiOverlayStyle;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/router/tab_refresh_provider.dart';
import '../../../shared/widgets/seller_gradient_canvas.dart';
import 'widgets/page_echo_tab.dart';
import 'widgets/page_header.dart';
import 'widgets/page_home_tab.dart';
import 'widgets/page_shop_tab.dart';
import 'widgets/page_tab_bar.dart';
import 'widgets/storefront_tab_glass.dart';

/// 商家頁面空狀態的顯示變體，用於 GoRouter `extra` 傳入控制初始佈局。
enum PageEmptyVariant {
  activityEmpty,
  gridPlaceholder,
  shopEmpty,
  homeEmptyProduct,
  homeRich,
}

/// 商家主頁面（分頁式佈局），顯示當前登入賣家自己的店面。
///
/// 包含三個商店管理 Tab：
/// - Tab 0 [PageHomeTab]：動態牆 / 貼文 grid
/// - Tab 1 [PageShopTab]：商品上架與認證狀態
/// - Tab 2 [PageEchoTab]：互動回聲 (評論/按讚)
///
/// 店家資料與統計來自 `GetMyStore`（`myStoreSnapshotProvider`）。
class PageScreen extends ConsumerStatefulWidget {
  const PageScreen({
    super.key,
    this.variant = PageEmptyVariant.activityEmpty,
    this.onBackPressed,
  });

  final PageEmptyVariant variant;
  final VoidCallback? onBackPressed;

  @override
  ConsumerState<PageScreen> createState() => _PageScreenState();
}

class _PageScreenState extends ConsumerState<PageScreen>
    with TickerProviderStateMixin, WidgetsBindingObserver {
  late TabController _tabController;
  int _lastHandledTabIndex =
      1; // tracks last processed tab to avoid re-firing during animation
  final _shopTabKey = GlobalKey<PageShopTabState>();
  static const Color _pageBackgroundColor = AppColors.bgPrimary;

  /// 與買家商家頁共用相同的 Hero、資訊面板與 Tab 高度計算。
  double _expandedHeight(BuildContext context, MyStore? mine) {
    final spec = PageHeaderLayoutSpec.of(context);
    final location = mine?.locationText ?? '';
    final prize = mine?.store?.prize ?? '';
    return spec.expandedAppBarHeightFor(
      awardText: prize,
      fansCount: mine?.followerCount,
      hasAward: prize.trim().isNotEmpty,
      displayedLocation: location.isEmpty ? '尚未設定店舖位置' : location,
      description: (mine?.store?.description ?? '').trim(),
    );
  }

  double get _collapsedHeight => PageHeaderLayoutSpec.topBarHeight;

  void _handleTabChanged() {
    if (!mounted) return;
    final newIndex = _tabController.index;
    if (newIndex == _lastHandledTabIndex) return;
    _lastHandledTabIndex = newIndex;
    // 切到 Shop tab 時重新載入認證 / 物流狀態
    if (newIndex == 1) {
      _shopTabKey.currentState?.reload();
    }
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _tabController = TabController(
      length: 3,
      vsync: this,
      initialIndex: 1,
    ); // Shop tab selected
    _tabController.addListener(_handleTabChanged);
    _refreshStore();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    super.didChangeAppLifecycleState(state);
    if (state == AppLifecycleState.resumed) _refreshStore();
  }

  void _refreshStore() {
    unawaited(ref.read(sellerStatusRefreshProvider.notifier).refresh());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _tabController.removeListener(_handleTabChanged);
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final mine = ref.watch(myStoreSnapshotProvider);
    final storeId = mine?.storeId ?? '';
    final storeName = mine?.storeName;

    // Tab 切換時刷新店家
    ref.listen<int>(pageTabRefreshProvider, (_, _) => _refreshStore());

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle.light.copyWith(
        statusBarColor: Colors.transparent,
      ),
      child: Scaffold(
        backgroundColor: Colors.transparent,
        body: SellerGradientCanvas(
          child: NestedScrollView(
            headerSliverBuilder: (context, _) => [
              SliverOverlapAbsorber(
                handle: NestedScrollView.sliverOverlapAbsorberHandleFor(
                  context,
                ),
                sliver: SliverAppBar(
                  pinned: true,
                  expandedHeight: _expandedHeight(context, mine),
                  collapsedHeight: _collapsedHeight,
                  toolbarHeight: _collapsedHeight,
                  systemOverlayStyle: SystemUiOverlayStyle.light.copyWith(
                    statusBarColor: Colors.transparent,
                  ),
                  automaticallyImplyLeading: false,
                  backgroundColor: _pageBackgroundColor,
                  elevation: 0,
                  scrolledUnderElevation: 0,
                  flexibleSpace: PageHeader(
                    coverUrl: mine?.coverUrl,
                    sellerName:
                        storeName ?? (storeId.isEmpty ? '商家' : '商家$storeId'),
                    fansCount: mine?.followerCount,
                    ratingAverage: mine?.ratingAverage,
                    reviewCount: mine?.reviewCount,
                    location: mine?.locationText,
                    prize: mine?.store?.prize,
                    description: (mine?.store?.description ?? '').trim(),
                    avatarUrl: mine?.logoUrl,
                    onBackPressed: widget.onBackPressed,
                  ),
                  bottom: PageTabBar(tabController: _tabController),
                ),
              ),
            ],
            body: Builder(
              builder: (outerContext) {
                final handle = NestedScrollView.sliverOverlapAbsorberHandleFor(
                  outerContext,
                );
                return TabBarView(
                  controller: _tabController,
                  physics: const NeverScrollableScrollPhysics(),
                  children: [
                    StorefrontTabCanvas(
                      child: PageHomeTab(
                        outerHandle: handle,
                        sellerName: storeName,
                        onReloadSeller: () async {
                          await ref
                              .read(sellerStatusRefreshProvider.notifier)
                              .refresh();
                        },
                      ),
                    ),
                    StorefrontTabCanvas(
                      child: PageShopTab(
                        key: _shopTabKey,
                        outerHandle: handle,
                        sellerId: storeId,
                      ),
                    ),
                    StorefrontTabCanvas(
                      child: PageEchoTab(
                        outerHandle: handle,
                        sellerId: storeId,
                        sellerDisplayName: storeName ?? '商家',
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
        ),
      ),
    );
  }
}
