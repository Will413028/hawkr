import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'dart:async';

import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart' as api;
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/networking/dio_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/buyer_navigation_clearance_sliver.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/main_top_bar.dart';
import '../../../shared/widgets/top_bar_provider.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../address/presentation/address_controller.dart';
import '../../address/presentation/address_management_screen.dart';
import '../../address/presentation/store_pickup_controller.dart';
import '../../address/presentation/widgets/address_selection_widgets.dart';
import '../../cart/presentation/store_pickup_form_screen.dart';
import '../../member_card/presentation/member_card_controller.dart';
import '../data/banner_repository.dart';
import 'home_product_controller.dart';
import 'home_widget/home_address_section.dart';
import 'home_widget/home_banner_section.dart';
import 'home_widget/home_latest_posts_section.dart';
import 'home_widget/home_member_card_section.dart';
import 'home_widget/home_product_section.dart';

/// 使用者型別：一般用戶或商家用戶
enum UserKind { user, business }

/// Home 畫面 - 符合 Figma 設計
class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({
    super.key,
    this.userKind = UserKind.user,
    this.onSwitchTab,
  });

  final UserKind userKind;
  final void Function(int mainTab, {int walletTab})? onSwitchTab;

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> with TopBarProvider {
  // 優惠券與會員卡計數
  int _couponCount = 0;
  int _memberCardCount = 0;
  List<String> _merchantAvatars = [];

  // Banner 資料
  List<api.Banner> _banners = const [];

  @override
  void initState() {
    super.initState();
    _loadCouponCount();
    _loadBanners();
    // Riverpod rejects provider mutations while the first widget tree is
    // mounting. Starting immediately can leave the controller in loading
    // before the request is sent, which turns the product skeleton permanent.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      unawaited(ref.read(homeProductControllerProvider.notifier).loadInitial());
    });
  }

  /// 載入 Banner 資料。失敗時不顯示 banner（transport 已依錯誤模型重試過）。
  Future<void> _loadBanners() async {
    try {
      final banners = await ref.read(bannerRepositoryProvider).liveBanners();
      if (!mounted) return;
      setState(() {
        _banners = banners;
      });
      HKLog.logD('[HomeScreen] Loaded ${banners.length} banners');
    } catch (e) {
      HKLog.logD('[HomeScreen] _loadBanners error: $e');
    }
  }

  /// 載入優惠券數量 — GET /app/coupons
  Future<void> _loadCouponCount() async {
    // Apple App Review 合規：訪客（無 token）打 /app/coupons 必 401，直接略過。
    final session = ref.read(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return;
    }
    try {
      final dio = ref.read(dioProvider);
      final resp = await dio.get<Map<String, dynamic>>(
        '/app/coupons',
        queryParameters: {'status': 0},
      );
      final body = resp.data!;
      HKLog.logD('[HomeScreen] _loadCouponCount body: $body');
      if (body['code'] != 0) return;
      final data = body['data'];
      final List list;
      if (data is List) {
        list = data;
      } else if (data is Map) {
        list = (data['records'] ?? data['list'] ?? data['items'] ?? []) as List;
      } else {
        list = [];
      }
      if (!mounted) return;
      setState(() {
        _couponCount = list.length;
      });
    } catch (e) {
      HKLog.logD('[HomeScreen] _loadCouponCount error: $e');
    }
  }

  /// 從 memberCardsController 取得會員卡數量與商家頭像。
  void _syncMemberCards() {
    final cardsAsync = ref.read(memberCardsControllerProvider);
    final records = cardsAsync.value?.records ?? [];
    _memberCardCount = records.length;
    _merchantAvatars = records.map((c) => c.sellerLogoKey).toList();
  }

  Future<void> _showAddressBottomSheet() async {
    final addresses = ref.read(addressControllerProvider).value ?? [];
    final defaultAddress = addresses.where((a) => a.isDefault).firstOrNull;
    final storePickups = ref.read(storePickupControllerProvider).value ?? [];
    final defaultStorePickup = storePickups
        .where((pickup) => pickup.isDefault)
        .firstOrNull;

    if (!mounted) return;
    final scheme = Theme.of(context).colorScheme;
    await showModalBottomSheet<void>(
      context: context,
      backgroundColor: Colors.transparent,
      barrierColor: scheme.scrim.withValues(alpha: 0.40),
      elevation: 0,
      isScrollControlled: true,
      showDragHandle: false,
      constraints: const BoxConstraints(maxWidth: 520),
      builder: (ctx) => AddressPickerPanel(
        addresses: addresses,
        selectedId: defaultAddress?.id,
        onClose: () => Navigator.of(ctx).pop(),
        onAdd: _showAddAddressBottomSheet,
        storePickups: storePickups,
        selectedStorePickupId: defaultStorePickup?.id,
        onAddStorePickup: _showAddStorePickup,
        onSelect: (address) async {
          try {
            await ref
                .read(addressControllerProvider.notifier)
                .setDefault(address.id);
            return true;
          } catch (e) {
            HKLog.logD('[HomeScreen] setDefault error: $e');
            if (ctx.mounted) {
              showFloatingToast(ctx, '設定預設地址失敗：$e', isSuccess: false);
            }
            return false;
          }
        },
        onStorePickupSelect: (pickup) async {
          try {
            await ref
                .read(storePickupControllerProvider.notifier)
                .setDefault(pickup);
            return true;
          } catch (e) {
            HKLog.logD('[HomeScreen] setDefaultStorePickup error: $e');
            if (ctx.mounted) {
              showFloatingToast(ctx, '設定預設門市失敗：$e', isSuccess: false);
            }
            return false;
          }
        },
      ),
    );
  }

  Future<void> _showAddStorePickup() async {
    await showStorePickupFormSheet(context);
  }

  Future<void> _showAddAddressBottomSheet() async {
    final result = await showAddressFormSheet(context);
    if (result != null) {
      if (mounted) TruckLoadingOverlay.show(context);
      try {
        await ref
            .read(addressControllerProvider.notifier)
            .add(
              receiverName: result.receiverName,
              receiverPhone: result.receiverPhone,
              townCode: result.townCode,
              addressDetail: result.addressDetail,
              isDefault: result.isDefault,
            );
        if (mounted) {
          TruckLoadingOverlay.hide(context);
          showFloatingToast(context, '地址新增成功');
        }
      } catch (e) {
        HKLog.logD('[HomeScreen] addAddress error: $e');
        if (mounted) {
          TruckLoadingOverlay.hide(context);
          showFloatingToast(context, '新增地址失敗：$e', isSuccess: false);
        }
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(memberCardsControllerProvider, (_, __) {
      if (mounted) setState(() => _syncMemberCards());
    });
    _syncMemberCards();

    final session = ref.watch(sessionProvider).value;
    // Apple App Review 合規：未登入顯示「訪客」；已登入用 nickname，空字串再退回「會員」。
    final bool isGuest = !hasUsableSession(session);
    final activeSession = isGuest ? null : session!;
    final String userName = isGuest ? '訪客' : (activeSession!.nickname ?? '會員');
    final productsState = ref.watch(homeProductControllerProvider);
    final productsController = ref.read(homeProductControllerProvider.notifier);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: CustomScrollView(
        key: const PageStorageKey<String>('home-primary-scroll-view'),
        slivers: [
          SliverToBoxAdapter(
            child: HomeAddressSection(onTap: _showAddressBottomSheet),
          ),
          const SliverToBoxAdapter(child: SizedBox(height: HkSpace.xs)),
          SliverToBoxAdapter(child: HomeBannerSection(banners: _banners)),
          const SliverToBoxAdapter(child: SizedBox(height: HkSpace.sm)),
          SliverToBoxAdapter(
            child: HomeMemberCardSection(
              userName: userName,
              couponCount: _couponCount,
              memberCardCount: _memberCardCount,
              merchantAvatars: _merchantAvatars,
              onCouponTap: () => widget.onSwitchTab?.call(2, walletTab: 1),
              onMemberCardTap: () => widget.onSwitchTab?.call(2, walletTab: 0),
            ),
          ),
          const SliverToBoxAdapter(child: SizedBox(height: HkSpace.sm)),
          const HomeLatestPostsSection(),
          const SliverToBoxAdapter(child: SizedBox(height: HkSpace.sm)),
          HomeProductSection(
            isLoading: productsState.isInitialLoading,
            products: productsState.products,
            initialErrorMessage: productsState.initialError?.toString(),
            onRetryInitial: () {
              unawaited(productsController.retryInitial());
            },
          ),
          const SliverToBoxAdapter(child: SizedBox(height: HkSpace.md)),
          const BuyerNavigationClearanceSliver(
            key: ValueKey('home-navigation-clearance'),
          ),
        ],
      ),
    );
  }

  /// 實作 TopBarProvider，提供共用頂部導航欄
  /// 使用 buildCommonAppBar 讓 widget tree 與其他主 tab 完全一致，避免切換 tab 時抖動
  @override
  PreferredSizeWidget buildTopBar(BuildContext context) {
    return buildMainTopBar(context, showTextSize: true);
  }
}
