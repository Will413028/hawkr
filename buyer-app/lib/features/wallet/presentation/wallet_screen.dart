import 'dart:convert';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';

import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_dialog.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/main_top_bar.dart';
import '../../../shared/widgets/magnetic_tab_bar.dart';
import '../../../shared/widgets/top_bar_provider.dart';
import '../../member_card/presentation/member_card_controller.dart';
import 'wallet_controller.dart';
import 'widgets/coupon_status_tabs.dart';
import 'widgets/wallet_coupon_ticket.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/shared/widgets/fruit_refresh_indicator.dart';
import 'package:hawkr_buyer/shared/widgets/hk_skeleton_shimmer.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

// ===================== Skeleton Shimmer Components =====================

class _SkeletonBar extends StatelessWidget {
  const _SkeletonBar({
    required this.width,
    required this.height,
    this.radius = 8,
  });

  final double width;
  final double height;
  final double radius;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: AppColors.dividerLight,
        borderRadius: BorderRadius.circular(radius),
      ),
    );
  }
}

/// 會員卡骨架圖
class _MemberCardSkeleton extends StatelessWidget {
  const _MemberCardSkeleton();

  @override
  Widget build(BuildContext context) {
    final sw = MediaQuery.of(context).size.width;
    final cardWidth = (sw - 32).clamp(0.0, 360.0);
    final isSmallScreen = cardWidth < 300;
    final cardHeight = isSmallScreen ? 170.0 : 200.0;

    return HkSkeletonShimmer(
      child: Container(
        width: cardWidth,
        height: cardHeight,
        margin: EdgeInsets.only(top: 12.h),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(isSmallScreen ? 24 : 32),
          color: AppColors.dividerLight,
          border: Border.all(
            color: AppColors.bgCard,
            width: isSmallScreen ? 2 : 3,
          ),
          boxShadow: const [
            BoxShadow(
              blurRadius: 8,
              color: AppColors.borderSubtle,
              offset: Offset(0, 2),
            ),
          ],
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // 頭像骨架
            Container(
              width: isSmallScreen ? 65 : 85,
              height: isSmallScreen ? 65 : 85,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: AppColors.dividerLight,
                border: Border.all(color: AppColors.bgCard, width: 4.w),
              ),
            ),
            SizedBox(height: 8.h),
            // 商家名稱骨架
            _SkeletonBar(width: 120, height: 16.h, radius: 8),
            SizedBox(height: 8.h),
            // 分隔線骨架
            Container(width: 160, height: 2, color: AppColors.dividerLight),
            SizedBox(height: 8.h),
            // 累積消費骨架
            _SkeletonBar(width: 180, height: 14.h, radius: 6),
          ],
        ),
      ),
    );
  }
}

/// 會員卡列表骨架圖
class _MemberCardSkeletonList extends StatelessWidget {
  const _MemberCardSkeletonList({required this.count});

  final int count;

  @override
  Widget build(BuildContext context) {
    const double cardHeight = 200.0;
    const double collapsedGap = 45.0;
    final double stackHeight =
        16 + collapsedGap * (count - 1) + cardHeight + 16;

    return SingleChildScrollView(
      padding: EdgeInsets.fromLTRB(16.w, 0, 16.w, 16.h),
      child: SizedBox(
        height: stackHeight,
        width: double.infinity,
        child: Stack(
          clipBehavior: Clip.none,
          children: List.generate(count, (i) {
            final double top = 16 + collapsedGap * i;
            return Positioned(
              top: top,
              left: 0,
              right: 0,
              child: const Center(child: _MemberCardSkeleton()),
            );
          }),
        ),
      ),
    );
  }
}

/// 優惠券骨架圖
class _CouponSkeleton extends StatelessWidget {
  const _CouponSkeleton();

  @override
  Widget build(BuildContext context) {
    return HkSkeletonShimmer(
      child: Container(
        constraints: const BoxConstraints(minHeight: 168),
        margin: const EdgeInsets.only(bottom: HkSpace.xxl),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(HkRadius.lg),
          border: Border.all(color: AppColors.borderSubtle),
          boxShadow: HkShadows.card,
        ),
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            const Padding(
              padding: EdgeInsets.only(right: 96),
              child: Padding(
                padding: EdgeInsets.all(HkSpace.lg),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        DecoratedBox(
                          decoration: BoxDecoration(
                            color: AppColors.dividerLight,
                            shape: BoxShape.circle,
                          ),
                          child: SizedBox.square(dimension: HkSpace.large),
                        ),
                        SizedBox(width: HkSpace.md),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              _SkeletonBar(
                                width: double.infinity,
                                height: HkSpace.xl,
                              ),
                              SizedBox(height: HkSpace.sm),
                              _SkeletonBar(width: 80, height: HkSpace.md),
                            ],
                          ),
                        ),
                      ],
                    ),
                    SizedBox(height: HkSpace.lg),
                    _SkeletonBar(
                      width: 120,
                      height: HkSpace.xxl,
                      radius: HkRadius.pill,
                    ),
                    SizedBox(height: HkSpace.lg),
                    Row(
                      children: [
                        DecoratedBox(
                          decoration: BoxDecoration(
                            color: AppColors.dividerLight,
                            shape: BoxShape.circle,
                          ),
                          child: SizedBox.square(dimension: HkSpace.xl),
                        ),
                        SizedBox(width: HkSpace.sm),
                        Expanded(
                          child: _SkeletonBar(
                            width: double.infinity,
                            height: HkSpace.lg,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
            const Positioned(
              top: 0,
              right: 0,
              bottom: 0,
              width: 96,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.dividerLight,
                  borderRadius: BorderRadius.horizontal(
                    right: Radius.circular(HkRadius.lg),
                  ),
                ),
                child: Padding(
                  padding: EdgeInsets.symmetric(
                    horizontal: HkSpace.md,
                    vertical: HkSpace.lg,
                  ),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      _SkeletonBar(width: 56, height: HkSpace.lg),
                      _SkeletonBar(width: 64, height: HkSpace.xxl),
                      _SkeletonBar(width: 48, height: HkSpace.lg),
                    ],
                  ),
                ),
              ),
            ),
            const Positioned(
              top: -HkSpace.md,
              right: 86,
              child: SizedBox.square(
                dimension: HkSpace.xl,
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    color: AppColors.bgPrimary,
                    shape: BoxShape.circle,
                  ),
                ),
              ),
            ),
            const Positioned(
              bottom: -HkSpace.md,
              right: 86,
              child: SizedBox.square(
                dimension: HkSpace.xl,
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    color: AppColors.bgPrimary,
                    shape: BoxShape.circle,
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

/// 優惠券列表骨架圖
class _CouponSkeletonList extends StatelessWidget {
  const _CouponSkeletonList({required this.count});

  final int count;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: List.generate(count, (_) => const _CouponSkeleton()),
    );
  }
}

// ===================== End Skeleton Components =====================

/// Wallet 入口：上方搜尋列、Tab（My wallet / My coupon）、底部導覽
class WalletScreen extends ConsumerStatefulWidget {
  const WalletScreen({
    super.key,
    this.showBottomNav = true,
    this.showTopBar = false,
    this.initialTab = 0,
  });
  final bool showBottomNav;
  final bool showTopBar;
  final int initialTab;
  @override
  ConsumerState<WalletScreen> createState() => _WalletScreenState();
}

class _WalletScreenState extends ConsumerState<WalletScreen>
    with SingleTickerProviderStateMixin, TopBarProvider {
  late final TabController _tabController;
  String _userName = 'User';

  @override
  void initState() {
    super.initState();
    int initIndex = widget.initialTab;
    if (initIndex < 0) initIndex = 0;
    if (initIndex > 1) initIndex = 1;
    _tabController = TabController(
      length: 2,
      vsync: this,
      initialIndex: initIndex,
    );
    _loadUserName();
  }

  Future<void> _loadUserName() async {
    final session = ref.read(sessionProvider).value;
    if (hasUsableSession(session) && mounted) {
      final activeSession = session!;
      setState(() {
        _userName = activeSession.nickname ?? 'User';
      });
    }
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // When embedded in MainNavigationScreen (showTopBar: false), skip the
    // inner Scaffold entirely to avoid duplicate app-bars.
    final body = Container(
      color: AppColors.bgPrimary,
      child: Column(
        children: [
          SizedBox(height: 6.h),
          _WalletTabs(controller: _tabController),
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: [
                _MyWalletTab(userName: _userName),
                const _MyCouponTab(),
              ],
            ),
          ),
        ],
      ),
    );

    if (!widget.showTopBar) return body;

    // Standalone mode: wrap in Scaffold with its own app bar
    return PopScope(
      canPop: Navigator.of(context).canPop(),
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop && widget.showBottomNav) {
          context.pop();
        }
      },
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        appBar: buildMainTopBar(context),
        body: body,
      ),
    );
  }

  @override
  PreferredSizeWidget buildTopBar(BuildContext context) {
    return buildMainTopBar(context);
  }
}

class _WalletTabs extends StatelessWidget {
  const _WalletTabs({required this.controller});
  final TabController controller;

  @override
  Widget build(BuildContext context) {
    return MagneticTabBar(
      controller: controller,
      labels: const ['會員卡', '優惠券'],
      height: 52,
      maxWidth: 560,
      indicatorKey: const ValueKey('wallet-tab-indicator'),
      labelKeyPrefix: 'wallet_tab_label_',
      iconBuilders: [
        (selected) => HkIcon(
          HkIconName.walletMember,
          key: const ValueKey('wallet-member-tab-icon'),
          selected: selected,
          size: 24,
          color: selected ? AppIconColors.strong : AppIconColors.muted,
        ),
        (selected) => HkIcon(
          HkIconName.walletCoupon,
          key: const ValueKey('wallet-coupon-tab-icon'),
          selected: selected,
          size: 24,
          color: selected ? AppIconColors.strong : AppIconColors.muted,
        ),
      ],
    );
  }
}

class _MyWalletTab extends ConsumerStatefulWidget {
  const _MyWalletTab({required this.userName});
  final String userName;
  @override
  ConsumerState<_MyWalletTab> createState() => _MyWalletTabState();
}

class _MyWalletTabState extends ConsumerState<_MyWalletTab> {
  final Duration _animDur = const Duration(milliseconds: 260);
  bool _animating = false;
  bool _showSkeleton = false; // 延遲顯示 skeleton，避免快速載入閃爍
  bool _isExpanded = false;
  final ScrollController _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    // 300ms 後才顯示 skeleton，如果載入夠快就不會閃
    Future.delayed(const Duration(milliseconds: 300), () {
      if (mounted) setState(() => _showSkeleton = true);
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _toggleExpand() {
    if (_animating) return;
    _animating = true;
    setState(() => _isExpanded = !_isExpanded);
    // 動畫結束後只更新 _animating 狀態，不觸發 setState 避免閃爍
    Future.delayed(_animDur, () {
      if (mounted) _animating = false;
    });
  }

  // 收起時點擊卡片，展開並滾動到該卡片位置
  void _onCardTap(int index) {
    if (_animating) return;

    if (_isExpanded) {
      // 已展開，點擊則收起
      _toggleExpand();
    } else {
      // 收起狀態，點擊展開並滾動到對應卡片
      _animating = true;

      // 計算該卡片展開後的位置
      const double expandedGap = 220.0;
      final double targetOffset = expandedGap * index;

      // 展開卡片
      setState(() => _isExpanded = true);

      // 等待動畫完成後，再等一幀確保佈局更新，然後滾動
      Future.delayed(_animDur, () {
        if (!mounted) {
          _animating = false;
          return;
        }
        // 使用 addPostFrameCallback 確保佈局完成後再滾動
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted || !_scrollController.hasClients) {
            _animating = false;
            return;
          }
          final maxScroll = _scrollController.position.maxScrollExtent;
          final clampedOffset = targetOffset.clamp(0.0, maxScroll);
          // 直接跳到目標位置（不用動畫，避免二次滾動感）
          _scrollController.jumpTo(clampedOffset);
          _animating = false;
        });
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final cardsAsync = ref.watch(memberCardsControllerProvider);

    const double cardHeight = 200.0;
    const double collapsedGap = 45.0;
    const double expandedGap = 220.0;

    if (cardsAsync.isLoading) {
      return _showSkeleton
          ? const _MemberCardSkeletonList(count: 3)
          : const SizedBox.shrink();
    }
    if (cardsAsync.hasError) {
      return const Center(child: Text('載入失敗，請稍後再試'));
    }
    final records = cardsAsync.value?.records ?? [];
    if (records.isEmpty) {
      return const HkEmptyState(
        key: ValueKey('member-card-empty-state'),
        iconKey: ValueKey('member-card-empty-state-icon'),
        icon: HkIconName.emptyMemberCard,
        message: '暫無會員卡',
      );
    }

    final double gap = _isExpanded ? expandedGap : collapsedGap;
    final double stackHeight =
        16 + gap * (records.length - 1) + cardHeight + 16;

    final List<Widget> layers = <Widget>[];
    for (int i = 0; i < records.length; i++) {
      final card = records[i];
      final double top = 16 + gap * i;
      final int totalSpending =
          int.tryParse(card.totalSpending.replaceAll(',', '')) ?? 0;
      layers.add(
        AnimatedPositioned(
          key: ValueKey(card.sellerId),
          duration: _animDur,
          curve: Curves.easeOutCubic,
          top: top,
          left: 0,
          right: 0,
          child: Center(
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: () => _onCardTap(i),
              child: _NewLoyaltyCard(
                merchantId: card.sellerId,
                storeCode: card.storeCode,
                statusLabel: card.statusLabel,
                merchantName: card.sellerName,
                merchantAvatarRaw: card.sellerLogoKey.isNotEmpty
                    ? card.sellerLogoKey
                    : null,
                coverUrl: card.sellerCoverKey.isNotEmpty
                    ? card.sellerCoverKey
                    : null,
                totalSpending: totalSpending,
                isExpanded: _isExpanded,
              ),
            ),
          ),
        ),
      );
    }

    return SingleChildScrollView(
      controller: _scrollController,
      padding: EdgeInsets.fromLTRB(16.w, 0, 16.w, 16.h),
      child: Column(
        children: [
          AnimatedContainer(
            duration: _animDur,
            curve: Curves.easeOutCubic,
            height: stackHeight,
            width: double.infinity,
            child: Stack(clipBehavior: Clip.none, children: layers),
          ),
          if (cardsAsync.value?.hasMore ?? false)
            TextButton(
              style: TextButton.styleFrom(
                foregroundColor: AppColors.textPrimary,
              ),
              onPressed: () => ref
                  .read(memberCardsControllerProvider.notifier)
                  .loadMore()
                  .catchError((Object _) {
                    if (mounted) {
                      showCommonSnackBar(this.context, '載入失敗，請稍後再試');
                    }
                  }),
              child: const HkButtonLabel('載入更多'),
            ),
        ],
      ),
    );
  }
}

// 新版會員卡樣式：背景圖片、商家頭貼、商家名稱、累積消費、點數
class _NewLoyaltyCard extends StatefulWidget {
  const _NewLoyaltyCard({
    required this.merchantId,
    required this.storeCode,
    this.statusLabel = '',
    required this.merchantName,
    this.merchantAvatarRaw,
    this.coverUrl,
    required this.totalSpending,
    this.isExpanded = false,
  });
  final String merchantId;

  /// 店面網址的路徑段；店家不是營業中時是空字串，不能進店。
  final String storeCode;

  /// 店家暫停營業或已關閉時的標籤（B11）；營業中是空字串。
  final String statusLabel;
  final String merchantName;
  final String? merchantAvatarRaw;
  final String? coverUrl;
  final int totalSpending;
  final bool isExpanded; // 是否展開狀態

  @override
  State<_NewLoyaltyCard> createState() => _NewLoyaltyCardState();
}

class _NewLoyaltyCardState extends State<_NewLoyaltyCard> {
  ImageProvider? _avatarImage;

  @override
  void initState() {
    super.initState();
    _avatarImage = _resolveImage(widget.merchantAvatarRaw);
  }

  @override
  void didUpdateWidget(covariant _NewLoyaltyCard oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.merchantAvatarRaw != widget.merchantAvatarRaw) {
      _avatarImage = _resolveImage(widget.merchantAvatarRaw);
    }
  }

  ImageProvider? _resolveImage(String? raw) {
    if (raw == null || raw.trim().isEmpty) return null;
    final value = raw.trim();
    // `data:` URI 是唯一需要走 MemoryImage 的形式。原本還有一條「不是 http 且
    // 長度 > 60 就當成 base64」的猜測——但 objectKey 本來就有 70~90 字元
    // （`public/user_avatar/<storeId>/<yyyymmdd>/<uuid>.png` 是 78 字元），
    // 會被誤判成 base64、decode 失敗後靜默退回預設頭像。objectKey 與完整 URL
    // 一律交給 buildImageUrl 處理（49 §4.1 唯一入口）。
    if (value.startsWith('data:image/')) {
      try {
        final bytes = base64Decode(
          value.contains(',') ? value.split(',').last : value,
        );
        return bytes.isEmpty ? null : MemoryImage(bytes);
      } catch (_) {
        return null;
      }
    }
    return NetworkImage(ImageConfig.urlFor(value, ImageScene.merchantAvatar));
  }

  @override
  Widget build(BuildContext context) {
    final sw = MediaQuery.of(context).size.width;
    final cardWidth = (sw - 32).clamp(0.0, 360.0);

    // 響應式尺寸
    final isSmallScreen = cardWidth < 300;
    final avatarSize = isSmallScreen ? 65.0 : 85.0;
    final nameFontSize = isSmallScreen ? 14.0 : 16.0;
    final amountFontSize = isSmallScreen ? 14.0 : 16.0;
    final dividerWidth = isSmallScreen ? 120.0 : 160.0;
    final tagWidth = isSmallScreen ? 40.0 : 50.0;
    final tagHeight = isSmallScreen ? 56.0 : 70.0;
    final cardHeight = isSmallScreen ? 170.0 : 200.0;
    final bodyHeight = isSmallScreen ? 160.0 : 190.0;

    return RepaintBoundary(
      child: SizedBox(
        width: cardWidth,
        height: cardHeight,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            // 卡片主體（往下偏移讓右上角圖片突出）
            Positioned(
              top: 10,
              left: 0,
              right: 0,
              child: Container(
                height: bodyHeight,
                decoration: BoxDecoration(
                  color: AppColors.primary.withValues(alpha: 0.8),
                  borderRadius: BorderRadius.circular(16),
                  border: Border.all(color: AppColors.bgCard, width: 2),
                  boxShadow: const [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 2,
                      offset: Offset(0, 1),
                    ),
                  ],
                ),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(12),
                  child: Stack(
                    fit: StackFit.expand,
                    children: [
                      // 背景圖片（商家 cover 或預設）
                      if (widget.coverUrl != null &&
                          widget.coverUrl!.isNotEmpty)
                        OssImage(
                          imageSource: widget.coverUrl!,
                          scene: ImageScene.merchantBanner,
                          fit: BoxFit.cover,
                          width: double.infinity,
                          height: double.infinity,
                          errorWidget: const HkPlaceholder(
                            HkPlaceholderKind.storeCover,
                          ),
                        )
                      else
                        const HkPlaceholder(HkPlaceholderKind.storeCover),
                      // 半透明遮罩
                      Container(
                        color: AppColors.textEmphasis.withValues(alpha: 0.25),
                      ),
                      // 內容：置中排列 - 頭貼在上，名稱在中，累積消費在下
                      Center(
                        child: Padding(
                          padding: EdgeInsets.symmetric(
                            horizontal: isSmallScreen ? 8 : 16,
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.center,
                            mainAxisAlignment: MainAxisAlignment.center,
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              // 商家頭貼 - 展開時可點擊到商家頁
                              GestureDetector(
                                onTap:
                                    widget.isExpanded &&
                                        widget.storeCode.isNotEmpty
                                    ? () {
                                        context.push(
                                          '/merchant/${widget.storeCode}',
                                        );
                                      }
                                    : null,
                                child: Container(
                                  width: avatarSize + (isSmallScreen ? 6 : 8),
                                  height: avatarSize + (isSmallScreen ? 6 : 8),
                                  decoration: BoxDecoration(
                                    shape: BoxShape.circle,
                                    color: AppColors.bgCard,
                                    boxShadow: const [
                                      BoxShadow(
                                        blurRadius: 6,
                                        color: AppColors.textDisabled,
                                      ),
                                    ],
                                  ),
                                  padding: EdgeInsets.all(
                                    isSmallScreen ? 4 : 4,
                                  ),
                                  child: Container(
                                    width: avatarSize,
                                    height: avatarSize,
                                    decoration: const BoxDecoration(
                                      shape: BoxShape.circle,
                                    ),
                                    clipBehavior: Clip.antiAlias,
                                    child: _avatarImage != null
                                        ? Image(
                                            image: _avatarImage!,
                                            fit: BoxFit.cover,
                                            gaplessPlayback: true,
                                            errorBuilder: (_, __, ___) =>
                                                const HkPlaceholder(
                                                  HkPlaceholderKind
                                                      .merchantAvatar,
                                                ),
                                          )
                                        : const HkPlaceholder(
                                            HkPlaceholderKind.merchantAvatar,
                                          ),
                                  ),
                                ),
                              ),
                              SizedBox(height: isSmallScreen ? 2 : 4),
                              // 商家名稱
                              Text(
                                widget.merchantName,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                textAlign: TextAlign.center,
                                style: TextStyle(
                                  color: AppColors.bgCard,
                                  fontSize: nameFontSize,
                                  fontWeight: FontWeight.w900,
                                  shadows: const [
                                    Shadow(
                                      blurRadius: 4,
                                      color: AppColors.textSecondary,
                                    ),
                                  ],
                                ),
                              ),
                              if (widget.statusLabel.isNotEmpty)
                                Text(
                                  widget.statusLabel,
                                  key: const ValueKey('member-card-status'),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  textAlign: TextAlign.center,
                                  style: TextStyle(
                                    color: AppColors.bgCard,
                                    fontSize: 12,
                                    shadows: const [
                                      Shadow(
                                        blurRadius: 4,
                                        color: AppColors.textSecondary,
                                      ),
                                    ],
                                  ),
                                ),
                              SizedBox(height: isSmallScreen ? 2 : 4),
                              // 白色分隔線
                              Container(
                                width: dividerWidth,
                                height: 2,
                                color: AppColors.bgCard,
                              ),
                              SizedBox(height: isSmallScreen ? 2 : 4),
                              // 累積消費（點數轉換成金額）
                              _buildAmountText(fontSize: amountFontSize),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
            // 右上角浮動的紀錄按鈕圖片（突出卡片上方）- 展開時才能點擊跳轉到消費紀錄頁
            Positioned(
              top: -5,
              right: isSmallScreen ? 4 : 8,
              child: GestureDetector(
                onTap: widget.isExpanded
                    ? () {
                        context.push(
                          '/member-cards/${widget.merchantId}/records',
                          extra: widget.merchantName,
                        );
                      }
                    : null,
                child: _RecordsTag(width: tagWidth, height: tagHeight),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildAmountText({double fontSize = 16.0}) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '累計消費',
          style: TextStyle(
            color: AppColors.bgCard,
            fontSize: fontSize,
            fontWeight: FontWeight.w900,
            shadows: const [
              Shadow(blurRadius: 2, color: AppColors.textSecondary),
            ],
          ),
        ),
        SizedBox(width: 8.w),
        Text(
          'NT\$ ${widget.totalSpending}',
          style: TextStyle(
            color: AppColors.bgCard,
            fontSize: fontSize,
            fontWeight: FontWeight.w900,
            shadows: const [
              Shadow(blurRadius: 2, color: AppColors.textSecondary),
            ],
          ),
        ),
      ],
    );
  }
}

/// 會員卡右上角突出的「紀錄」標籤。
class _RecordsTag extends StatelessWidget {
  const _RecordsTag({required this.width, required this.height});

  final double width;
  final double height;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: AppColors.brandPrimaryStrong,
        borderRadius: BorderRadius.vertical(bottom: Radius.circular(width / 2)),
      ),
      child: ConstrainedBox(
        constraints: BoxConstraints(minWidth: width, minHeight: height),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(
                AppIcons.receiptLongOutlined,
                size: 20,
                color: AppIconColors.onBrand,
              ),
              const Text(
                '紀錄',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  color: AppColors.bgCard,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MyCouponTab extends ConsumerStatefulWidget {
  const _MyCouponTab();
  @override
  ConsumerState<_MyCouponTab> createState() => _MyCouponTabState();
}

class _MyCouponTabState extends ConsumerState<_MyCouponTab> {
  bool _showSkeleton = false;
  int _selectedChipIndex = 0;
  int _skeletonRequest = 0;

  @override
  void initState() {
    super.initState();
    _scheduleSkeleton();
  }

  void _scheduleSkeleton() {
    final request = ++_skeletonRequest;
    Future.delayed(const Duration(milliseconds: 300), () {
      if (!mounted || request != _skeletonRequest) return;
      final loading = ref
          .read(couponControllerProvider(status: _selectedChipIndex))
          .isLoading;
      if (loading) setState(() => _showSkeleton = true);
    });
  }

  Future<void> _refresh() {
    return ref
        .read(couponControllerProvider(status: _selectedChipIndex).notifier)
        .refresh();
  }

  @override
  Widget build(BuildContext context) {
    final couponsAsync = ref.watch(
      couponControllerProvider(status: _selectedChipIndex),
    );
    final coupons = couponsAsync.value ?? const <WalletCouponCardState>[];
    final loading = couponsAsync.isLoading && !couponsAsync.hasValue;
    final loadFailed = couponsAsync.hasError && !couponsAsync.hasValue;
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;

    return LayoutBuilder(
      builder: (context, constraints) {
        final gutter = HkBreakpoints.pageGutter(constraints.maxWidth);
        final frameWidth = 560 + gutter * 2;
        final outerInset = ((constraints.maxWidth - frameWidth) / 2)
            .clamp(0, double.infinity)
            .toDouble();
        final bottomInset = MediaQuery.paddingOf(context).bottom + HkSpace.lg;
        final switchDuration = reduceMotion ? Duration.zero : HkMotion.duration;

        return Padding(
          padding: EdgeInsets.symmetric(horizontal: outerInset),
          child: Column(
            children: [
              Padding(
                padding: EdgeInsets.fromLTRB(gutter, HkSpace.sm, gutter, 0),
                child: CouponStatusTabs(
                  selectedIndex: _selectedChipIndex,
                  onSelected: (index) {
                    if (_selectedChipIndex == index) return;
                    setState(() {
                      _selectedChipIndex = index;
                      _showSkeleton = false;
                    });
                    _scheduleSkeleton();
                  },
                ),
              ),
              const SizedBox(height: HkSpace.sm),
              Expanded(
                child: AnimatedSwitcher(
                  duration: switchDuration,
                  switchInCurve: HkMotion.ease,
                  switchOutCurve: Curves.easeInCubic,
                  child: _buildCouponContent(
                    context,
                    coupons: coupons,
                    loading: loading,
                    loadFailed: loadFailed,
                    gutter: gutter,
                    bottomInset: bottomInset,
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _buildCouponContent(
    BuildContext context, {
    required List<WalletCouponCardState> coupons,
    required bool loading,
    required bool loadFailed,
    required double gutter,
    required double bottomInset,
  }) {
    if (loading) {
      return KeyedSubtree(
        key: ValueKey('coupon-loading-$_selectedChipIndex-$_showSkeleton'),
        child: _showSkeleton
            ? ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                padding: EdgeInsets.fromLTRB(gutter, 0, gutter, bottomInset),
                children: const [_CouponSkeletonList(count: 3)],
              )
            : const SizedBox.shrink(),
      );
    }

    if (loadFailed) {
      return FruitRefreshIndicator(
        key: ValueKey('coupon-error-$_selectedChipIndex'),
        onRefresh: _refresh,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: EdgeInsets.fromLTRB(gutter, 0, gutter, bottomInset),
          children: [_CouponLoadError(onRetry: _refresh)],
        ),
      );
    }

    if (coupons.isEmpty) {
      return FruitRefreshIndicator(
        key: ValueKey('coupon-empty-$_selectedChipIndex'),
        onRefresh: _refresh,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: EdgeInsets.fromLTRB(gutter, 0, gutter, bottomInset),
          children: [
            HkEmptyState(
              key: const ValueKey('coupon-empty-state'),
              iconKey: const ValueKey('coupon-empty-state-icon'),
              icon: HkIconName.emptyCoupon,
              message: _emptyMessage,
              description: _emptyDescription,
            ),
          ],
        ),
      );
    }

    return FruitRefreshIndicator(
      key: ValueKey('coupon-list-$_selectedChipIndex'),
      onRefresh: _refresh,
      child: ListView.separated(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.fromLTRB(gutter, 0, gutter, bottomInset),
        itemCount: coupons.length + 1,
        separatorBuilder: (_, index) =>
            SizedBox(height: index == 0 ? HkSpace.sm : HkSpace.xxl),
        itemBuilder: (context, index) {
          if (index == 0) {
            return _CouponListSummary(label: _listLabel, count: coupons.length);
          }

          final coupon = coupons[index - 1];
          return _CouponTile(
            key: ValueKey('wallet-coupon-ticket-${coupon.id}'),
            headerLabel: coupon.title,
            title: coupon.title,
            content: coupon.content,
            rule: coupon.rule,
            expire: coupon.expire,
            discountLabel: coupon.discountLabel,
            visualStatus: _visualStatus(),
            sellerName: coupon.sellerName,
            storeId: coupon.storeId,
            sellerAvatarUrl: coupon.sellerAvatarUrl,
            onShowRule: () => showCommonDialog(
              context: context,
              title: coupon.title,
              content: coupon.ruleDescription,
              confirmText: '關閉',
            ),
          );
        },
      ),
    );
  }

  /// 券的可用／已用／過期由頁籤（＝送給後端的 `status` 查詢值）決定。
  /// 曾經還 or 上 `coupon.isUsed`，但那個旗標來自 wire 上不存在的 `usedAt`，
  /// 恆為 false —— 判斷實際上一直只由頁籤決定，旗標已移除。
  WalletCouponVisualStatus _visualStatus() {
    if (_selectedChipIndex == 2) return WalletCouponVisualStatus.expired;
    if (_selectedChipIndex == 1) return WalletCouponVisualStatus.used;
    return WalletCouponVisualStatus.available;
  }

  String get _listLabel => switch (_selectedChipIndex) {
    1 => '使用紀錄',
    2 => '過期紀錄',
    _ => '可使用優惠券',
  };

  String get _emptyMessage => switch (_selectedChipIndex) {
    1 => '還沒有使用紀錄',
    2 => '目前沒有已過期優惠券',
    _ => '目前沒有可用優惠券',
  };

  String get _emptyDescription => switch (_selectedChipIndex) {
    1 => '使用過的優惠券會保留在這裡',
    2 => '過期紀錄會顯示在這裡',
    _ => '到市集看看商家最新優惠',
  };
}

class _CouponListSummary extends StatelessWidget {
  const _CouponListSummary({required this.label, required this.count});

  final String label;
  final int count;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      liveRegion: true,
      label: '$label，共 $count 張',
      excludeSemantics: true,
      child: Row(
        children: [
          Expanded(child: Text(label, style: AppTypography.bodyBold)),
          DecoratedBox(
            decoration: BoxDecoration(
              color: AppColors.primarySubtle,
              borderRadius: BorderRadius.circular(HkRadius.pill),
              border: Border.all(color: AppColors.borderSubtle),
            ),
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: HkSpace.sm,
                vertical: HkSpace.xs,
              ),
              child: Text(
                '$count 張',
                style: AppTypography.caption.copyWith(
                  color: AppColors.brandPrimaryStrong,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _CouponLoadError extends StatelessWidget {
  const _CouponLoadError({required this.onRetry});

  final Future<void> Function() onRetry;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        const HkEmptyState(
          key: ValueKey('coupon-error-state'),
          iconKey: ValueKey('coupon-error-state-icon'),
          icon: HkIconName.emptyCoupon,
          message: '優惠券載入失敗',
          description: '請檢查網路後再試一次',
        ),
        const SizedBox(height: HkSpace.lg),
        FilledButton.icon(
          key: const ValueKey('coupon-retry-button'),
          onPressed: onRetry,
          icon: const HkIcon(
            HkIconName.refresh,
            size: HkSpace.xl,
            color: AppIconColors.onBrand,
          ),
          label: const HkButtonLabel('重新載入'),
        ),
      ],
    );
  }
}

class _CouponTile extends StatelessWidget {
  const _CouponTile({
    super.key,
    required this.headerLabel,
    required this.title,
    required this.content,
    required this.rule,
    required this.expire,
    required this.visualStatus,
    this.discountLabel = '',
    this.sellerName = '',
    this.storeId = '',
    this.sellerAvatarUrl = '',
    this.onShowRule,
  });
  final String headerLabel;
  final String title;
  final String content;
  final String rule;
  final String expire;
  final String discountLabel;
  final WalletCouponVisualStatus visualStatus;

  /// 發放者商家名稱（CouponInstanceResponse.sellerName）。空字串=全平台券。
  final String sellerName;

  /// 發放者商家 store_id（CouponInstanceResponse.applicableStoreId）
  /// 用來撈 /public/sellers/{id} 取真頭像
  final String storeId;

  /// 發放者頭像 URL（spec 待補；有就用，否則撈 storeId）。
  final String sellerAvatarUrl;
  final VoidCallback? onShowRule;

  @override
  Widget build(BuildContext context) {
    final issuer = sellerName.trim();
    final normalizedTitle = title.trim().isEmpty
        ? headerLabel.trim()
        : title.trim();
    final issuerLabel = issuer.isEmpty
        ? 'Hawkr 全站優惠'
        : issuer == normalizedTitle
        ? '商家專屬優惠'
        : '由 $issuer 提供';

    return WalletCouponTicket(
      title: normalizedTitle,
      content: content,
      rule: rule,
      expiry: expire,
      discountLabel: discountLabel,
      issuerLabel: issuerLabel,
      issuerAvatar: _CouponIssuerAvatar(
        size: HkSpace.large,
        storeId: storeId,
        sellerName: sellerName,
        sellerAvatarUrl: sellerAvatarUrl,
      ),
      status: visualStatus,
      onPressed: onShowRule,
    );
  }
}

/// 發放者頭像 — 顯示優先順序：
/// 1. 券本身的 `logoKey` / `logoUrl`（gateway `instanceInfoToMap` /
///    `templateInfoToMap` 成對回傳；賣家券必有值、全平台券為空）
/// 2. 商家名稱首字（淺綠底色 + 綠字）
/// 3. 全平台券（粉紅底色 + tag icon）
///
/// 曾經有第 2 順位「用 storeId 撈 `/public/sellers/{id}` 取頭像」——每張券一次
/// 請求（N+1），而且該端點回的是 `logoKey` / `storeName`，舊 `Seller` model 讀
/// `avatar` / `name`，解析必定落空。券 payload 已成對帶 logo，整段移除。
class _CouponIssuerAvatar extends StatelessWidget {
  const _CouponIssuerAvatar({
    required this.size,
    required this.storeId,
    required this.sellerName,
    required this.sellerAvatarUrl,
  });
  final double size;
  final String storeId;
  final String sellerName;
  final String sellerAvatarUrl;

  @override
  Widget build(BuildContext context) {
    final url = sellerAvatarUrl.trim();

    if (url.isNotEmpty) {
      return ClipOval(
        child: SizedBox(
          width: size,
          height: size,
          child: AppCachedImage(
            imageUrl: url,
            // logoKey 是裸 objectKey，必須帶 scene 才會過 buildImageUrl 組 URL。
            scene: ImageScene.merchantAvatar,
            fit: BoxFit.cover,
            errorWidget: _fallback(),
          ),
        ),
      );
    }
    return _fallback();
  }

  Widget _fallback() {
    // 商家券沒有頭像 URL → 用預設商家頭像（不再用首字 fallback）
    if (sellerName.isNotEmpty || storeId.isNotEmpty) {
      return ClipOval(
        child: SizedBox(
          width: size,
          height: size,
          child: HkPlaceholder(HkPlaceholderKind.merchantAvatar),
        ),
      );
    }
    // 全平台券：保留原粉紅 + tag icon 設計
    return Container(
      width: size,
      height: size,
      decoration: const BoxDecoration(
        shape: BoxShape.circle,
        color: AppColors.bgPrimary,
      ),
      child: Icon(
        AppIcons.localOffer,
        size: size * 0.5,
        color: AppIconColors.danger,
      ),
    );
  }
}
