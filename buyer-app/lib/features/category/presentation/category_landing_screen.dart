import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../config/media_image.dart';
import '../../../core/networking/dio_provider.dart';
import '../../auth/data/auth_repository.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/widgets/hk_placeholder.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/main_top_bar.dart';
import '../../../shared/widgets/top_bar_provider.dart';
import '../../address/data/address_model.dart';
import '../../address/presentation/address_controller.dart';
import '../../member_card/presentation/member_card_controller.dart';
import '../../product/presentation/merchant_item_detail_screen.dart';
import '../data/category_marketplace_model.dart';
import 'category_marketplace_controller.dart';
import '../../../shared/utils/price_display.dart';
import 'package:hawkr_buyer/shared/widgets/hk_skeleton_shimmer.dart';

/// Category Landing (Dashboard-style)
/// - Mirrors Home dashboard layout
/// - Middle card is "Reserve"; tapping navigates to the dedicated Quick Category screen
class CategoryLandingScreen extends ConsumerStatefulWidget {
  const CategoryLandingScreen({super.key, this.showTopBar = true});
  final bool showTopBar;

  @override
  ConsumerState<CategoryLandingScreen> createState() =>
      _CategoryLandingScreenState();
}

class _CategoryLandingScreenState extends ConsumerState<CategoryLandingScreen>
    with SingleTickerProviderStateMixin, TopBarProvider {
  String _userName = 'User';
  String _userAvatar = '';
  int _couponCount = 0;

  late final AnimationController _marketProgress;
  final ScrollController _marketScrollController = ScrollController();
  final ScrollController _categoryScrollController = ScrollController();
  final TextEditingController _marketSearchController = TextEditingController();
  double _bodyDragExtent = 1;
  bool _isAtBottom = false;

  @override
  void initState() {
    super.initState();
    _marketProgress = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 260),
    )..addListener(() => setState(() {}));
    _categoryScrollController.addListener(_onCategoryScroll);
    _loadUserName();
    _loadCouponCount();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadCouponCount();
    });
  }

  @override
  void dispose() {
    _marketProgress.dispose();
    _marketScrollController.dispose();
    _categoryScrollController.removeListener(_onCategoryScroll);
    _categoryScrollController.dispose();
    _marketSearchController.dispose();
    super.dispose();
  }

  void _onCategoryScroll() {
    if (!_categoryScrollController.hasClients) return;
    final pos = _categoryScrollController.position;
    final atBottom = pos.pixels >= pos.maxScrollExtent - 10;
    if (atBottom != _isAtBottom) {
      setState(() => _isAtBottom = atBottom);
    }
  }

  bool _handleScrollNotification(ScrollNotification notification) {
    if (notification is OverscrollNotification) {
      if (notification.overscroll > 0) {
        if (_bodyDragExtent > 1) {
          final double next =
              (_marketProgress.value +
                      (notification.overscroll / _bodyDragExtent))
                  .clamp(0.0, 1.0);
          _marketProgress.value = next;
        }
      }
    } else if (notification is ScrollEndNotification) {
      if (_marketProgress.value > 0 && _marketProgress.value < 1) {
        _animateMarketplaceTo(_marketProgress.value >= 0.2 ? 1.0 : 0.0);
      }
    }
    return false;
  }

  Future<void> _loadUserName() async {
    final session = ref.read(sessionProvider).value;
    String name = _userName;
    String avatar = _userAvatar;
    String userId = hasUsableSession(session) ? session!.userId : '';

    if (hasUsableSession(session)) {
      final activeSession = session!;
      name = activeSession.nickname ?? name;
    }

    if (userId.isNotEmpty) {
      try {
        final info = await ref.read(authRepositoryProvider).getProfile();
        final remoteName = info.displayName.trim();
        if (remoteName.isNotEmpty) {
          name = remoteName;
        }
        final remoteAvatar = info.avatarUrl.trim();
        if (remoteAvatar.isNotEmpty) {
          avatar = remoteAvatar;
        }
      } catch (_) {}
    }

    if (!mounted) return;
    setState(() {
      _userName = name;
      _userAvatar = avatar.trim();
    });
  }

  Future<void> _loadCouponCount() async {
    try {
      final dio = ref.read(dioProvider);
      final session = ref.read(sessionProvider).value;
      if (!hasUsableSession(session)) return;
      final response = await dio.get('/app/coupons');
      final data = response.data;
      final list = (data is Map ? (data['data'] ?? []) : data);
      if (!mounted) return;
      setState(() {
        _couponCount = list is List ? list.length : 0;
      });
    } catch (_) {}
  }

  void _animateMarketplaceTo(double target) {
    final double clamped = target.clamp(0.0, 1.0);
    if ((_marketProgress.value - clamped).abs() < 0.001) {
      _marketProgress.value = clamped;
      return;
    }
    _marketProgress.animateTo(
      clamped,
      duration: const Duration(milliseconds: 260),
      curve: Curves.easeOutCubic,
    );
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(sessionProvider, (_, __) => _loadUserName());

    final media = MediaQuery.of(context);
    final double status = media.padding.top;
    final double topChromeHeight =
        status + (widget.showTopBar ? kTopBarHeight : 0);
    final double bottomInset = media.padding.bottom;
    final double navHeight = HkControl.navHeight + bottomInset;
    final double progress = _marketProgress.value.clamp(0.0, 1.0);
    final double currentBottomHeight = navHeight * progress;
    final double bodyHeight =
        (media.size.height - topChromeHeight - currentBottomHeight).clamp(
          1,
          double.infinity,
        );
    _bodyDragExtent = bodyHeight;

    final Widget categoryBody = _buildCategoryBody(
      context,
      16 + currentBottomHeight,
    );

    return SafeArea(
      top: false,
      child: Stack(
        children: [
          Column(
            children: [
              SizedBox(height: topChromeHeight),
              Expanded(
                child: NotificationListener<ScrollNotification>(
                  onNotification: _handleScrollNotification,
                  child: categoryBody,
                ),
              ),
              SizedBox(height: currentBottomHeight),
            ],
          ),
          if (widget.showTopBar)
            Positioned(
              top: 0,
              left: 0,
              right: 0,
              child: const _TopBar(key: ValueKey('category-top')),
            ),
        ],
      ),
    );
  }

  Widget _buildCategoryBody(BuildContext context, double bottomPadding) {
    return ColoredBox(
      color: AppColors.bgPrimary,
      child: SingleChildScrollView(
        controller: _categoryScrollController,
        padding: EdgeInsets.only(bottom: bottomPadding),
        physics: const BouncingScrollPhysics(),
        clipBehavior: Clip.hardEdge,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const _AddressBar(),
            const _HeroBannerNew(),
            SizedBox(height: 12.h),
            Container(
              height: 140.h,
              margin: EdgeInsets.symmetric(horizontal: 16.w),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(12.r),
                child: Stack(
                  children: [
                    const Positioned.fill(child: _MemberCardBackdrop()),
                    _MemberCardNew(
                      userName: _userName,
                      couponCount: _couponCount,
                      avatarUrl: _userAvatar,
                    ),
                  ],
                ),
              ),
            ),
            SizedBox(height: 20.h),
            const CategoryMarketplaceProductSection(),
            SizedBox(height: 16.h),
          ],
        ),
      ),
    );
  }

  @override
  PreferredSizeWidget buildTopBar(BuildContext context) {
    return buildMainTopBar(context);
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({super.key});
  @override
  Widget build(BuildContext context) {
    return commonTopBar(
      context: context,
      backgroundColor: AppColors.topBarBackground,
      systemOverlayStyle: AppColors.glassTopOverlay,
      frostedGlass: true,
      child: const MainTopBarContent(),
    );
  }
}

class _AddressBar extends ConsumerStatefulWidget {
  const _AddressBar();

  @override
  ConsumerState<_AddressBar> createState() => _AddressBarState();
}

class _AddressBarState extends ConsumerState<_AddressBar> {
  void _openAddressSettings() {
    context.push('/settings/address');
  }

  @override
  Widget build(BuildContext context) {
    final addressesAsync = ref.watch(addressControllerProvider);

    String address = '\u5c1a\u672a\u8a2d\u5b9a\u5730\u5740';

    addressesAsync.whenData((addresses) {
      if (addresses.isNotEmpty) {
        final defaultAddr =
            addresses.where((a) => a.isDefault).firstOrNull ?? addresses.first;
        final display = defaultAddr.fullAddress;
        if (display.isNotEmpty) {
          address = display;
        }
      }
    });

    return Semantics(
      label: address == '\u5c1a\u672a\u8a2d\u5b9a\u5730\u5740'
          ? '設定收貨地址'
          : '更改收貨地址，$address',
      button: true,
      onTap: _openAddressSettings,
      child: ExcludeSemantics(
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: _openAddressSettings,
          child: Container(
            color: AppColors.bgPrimary,
            padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
            child: Row(
              children: [
                HkIcon(HkIconName.truck, size: 40),
                SizedBox(width: 8.w),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '\u6536\u8ca8\u5730\u5740',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.textSecondary,
                        ),
                      ),
                      SizedBox(height: 2.h),
                      Text(
                        address,
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w500,
                          color: AppColors.textEmphasis,
                        ),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
                Text(
                  '\u9ede\u64ca\u66f4\u6539',
                  style: TextStyle(fontSize: 14, color: AppColors.primary),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _HeroBannerNew extends StatefulWidget {
  const _HeroBannerNew();
  @override
  State<_HeroBannerNew> createState() => _HeroBannerNewState();
}

class _HeroBannerNewState extends State<_HeroBannerNew> {
  final PageController _pageController = PageController();
  int _currentPage = 0;

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        SizedBox(
          height: 180.h,
          child: PageView(
            controller: _pageController,
            onPageChanged: (index) => setState(() => _currentPage = index),
            children: [
              _buildBannerCard(context),
              _buildBannerCard(context),
              _buildBannerCard(context),
            ],
          ),
        ),
        SizedBox(height: 8.h),
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: List.generate(3, (i) {
            return Container(
              margin: EdgeInsets.symmetric(horizontal: 4.w),
              width: _currentPage == i ? 8 : 6,
              height: _currentPage == i ? 8 : 6,
              decoration: BoxDecoration(
                color: _currentPage == i
                    ? AppColors.textEmphasis
                    : AppColors.gray300,
                shape: BoxShape.circle,
              ),
            );
          }),
        ),
      ],
    );
  }

  Widget _buildBannerCard(BuildContext context) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: 16.w),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(16.r),
        child: const HkPlaceholder(HkPlaceholderKind.photo),
      ),
    );
  }
}

/// 會員卡的上下兩段底色，中間隔一條細線。
class _MemberCardBackdrop extends StatelessWidget {
  const _MemberCardBackdrop();

  @override
  Widget build(BuildContext context) {
    return const Column(
      children: [
        Expanded(
          flex: 60,
          child: ColoredBox(
            color: AppColors.brandPrimaryStrong,
            child: SizedBox.expand(),
          ),
        ),
        SizedBox(
          height: 1,
          child: ColoredBox(color: AppColors.bgCard, child: SizedBox.expand()),
        ),
        Expanded(
          flex: 40,
          child: ColoredBox(
            color: AppColors.brandPrimaryDeep,
            child: SizedBox.expand(),
          ),
        ),
      ],
    );
  }
}

class _MemberCardNew extends ConsumerStatefulWidget {
  const _MemberCardNew({
    required this.userName,
    required this.couponCount,
    required this.avatarUrl,
  });
  final String userName;
  final int couponCount;
  final String avatarUrl;

  @override
  ConsumerState<_MemberCardNew> createState() => _MemberCardNewState();
}

class _MemberCardNewState extends ConsumerState<_MemberCardNew> {
  List<String> _merchantAvatars = [];
  int _memberCardCount = 0;

  /// 從 memberCardsController 取得會員卡數量與商家頭像。
  void _syncMemberCards() {
    final cardsAsync = ref.read(memberCardsControllerProvider);
    final records = cardsAsync.value?.records ?? [];
    _memberCardCount = records.length;
    _merchantAvatars = records
        .where((c) => c.sellerLogoKey.isNotEmpty)
        .map((c) => c.sellerLogoKey)
        .toList();
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(memberCardsControllerProvider, (_, __) {
      if (mounted) setState(() => _syncMemberCards());
    });
    _syncMemberCards();

    return GestureDetector(
      onTap: () => context.go('/?tab=3'),
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 12.w),
        child: Column(
          children: [
            Expanded(
              flex: 50,
              child: Padding(
                padding: EdgeInsets.only(top: 16.h),
                child: Row(
                  children: [
                    _MemberAvatarNew(avatarUrl: widget.avatarUrl),
                    SizedBox(width: 8.w),
                    Expanded(
                      flex: 4,
                      child: Text(
                        'Hi, ${widget.userName}',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                          color: AppColors.bgCard,
                        ),
                        overflow: TextOverflow.ellipsis,
                        maxLines: 1,
                      ),
                    ),
                    SizedBox(width: 4.w),
                    Flexible(
                      flex: 5,
                      child: Text(
                        '\u53ef\u7528${widget.couponCount}\u5f35\u512a\u60e0\u5238',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        textAlign: TextAlign.end,
                        style: TextStyle(fontSize: 12, color: AppColors.bgCard),
                      ),
                    ),
                    Icon(
                      AppIcons.chevronRight,
                      color: AppIconColors.onBrand,
                      size: 16.sp,
                    ),
                  ],
                ),
              ),
            ),
            Expanded(
              flex: 50,
              child: Align(
                alignment: const Alignment(0, 0.3),
                child: CategoryMemberCardSummary(
                  merchantAvatars: _merchantAvatars,
                  memberCardCount: _memberCardCount,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Responsive lower row used by the category membership card.
///
/// Public so the production layout can be exercised directly in the shared
/// device/text-size matrix without booting the network-backed category page.
class CategoryMemberCardSummary extends StatelessWidget {
  const CategoryMemberCardSummary({
    super.key,
    required this.merchantAvatars,
    required this.memberCardCount,
  });

  final List<String> merchantAvatars;
  final int memberCardCount;

  @override
  Widget build(BuildContext context) {
    const labelStyle = TextStyle(
      fontSize: 14,
      fontWeight: FontWeight.w500,
      color: AppColors.bgCard,
    );
    final label = '$memberCardCount\u5f35\u6703\u54e1\u5361';

    return LayoutBuilder(
      builder: (context, constraints) {
        final labelPainter = TextPainter(
          text: TextSpan(text: label, style: labelStyle),
          maxLines: 1,
          textDirection: Directionality.of(context),
          textScaler: MediaQuery.textScalerOf(context),
        )..layout();
        const iconWidth = 18.0;
        const itemGap = 8.0;
        const firstAvatarWidth = 40.0;
        final labelReservation = labelPainter.width
            .clamp(
              0,
              (constraints.maxWidth -
                      iconWidth -
                      itemGap * 2 -
                      firstAvatarWidth)
                  .clamp(0, double.infinity),
            )
            .toDouble();
        final avatarBudget =
            constraints.maxWidth - labelReservation - iconWidth - itemGap * 2;
        final visibleAvatarCount = _visibleAvatarCount(avatarBudget);

        return Row(
          key: const ValueKey('category-member-card-summary'),
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            if (visibleAvatarCount > 0) ...[
              SizedBox(
                key: const ValueKey('category-member-avatar-stack'),
                width: _calculateAvatarsWidth(visibleAvatarCount),
                height: 40,
                child: Stack(
                  clipBehavior: Clip.hardEdge,
                  children: _buildAvatarStack(visibleAvatarCount),
                ),
              ),
              const SizedBox(width: itemGap),
            ],
            Expanded(
              child: Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                textAlign: TextAlign.end,
                style: labelStyle,
              ),
            ),
            const SizedBox(width: itemGap),
            const Icon(
              AppIcons.chevronRight,
              color: AppIconColors.onBrand,
              size: iconWidth,
            ),
          ],
        );
      },
    );
  }

  int _visibleAvatarCount(double availableWidth) {
    if (merchantAvatars.isEmpty || availableWidth < 40) return 0;
    final capacity = 1 + ((availableWidth - 40) / 28).floor();
    return capacity.clamp(1, merchantAvatars.length.clamp(0, 10));
  }

  double _calculateAvatarsWidth(int count) {
    return 40 + (count - 1) * 28.0;
  }

  List<Widget> _buildAvatarStack(int count) {
    final List<Widget> avatars = [];

    for (int i = 0; i < count; i++) {
      avatars.add(
        Positioned(
          left: i * 28.0,
          child: Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              border: Border.all(color: AppColors.primaryLight, width: 2),
              color: AppColors.bgCard,
            ),
            child: ClipOval(child: _buildMerchantAvatar(merchantAvatars[i])),
          ),
        ),
      );
    }
    return avatars;
  }

  Widget _buildMerchantAvatar(String avatarUrl) {
    final trimmed = avatarUrl.trim();
    if (trimmed.isEmpty) {
      return Icon(AppIcons.store, size: 20.sp, color: AppIconColors.secondary);
    }

    if (isInlineImageData(trimmed)) {
      try {
        final bytes = base64Decode(
          trimmed.contains(',') ? trimmed.split(',').last : trimmed,
        );
        return Image.memory(
          bytes,
          fit: BoxFit.cover,
          width: 36.w,
          height: 36.h,
        );
      } catch (_) {
        return Icon(
          AppIcons.store,
          size: 20.sp,
          color: AppIconColors.secondary,
        );
      }
    }

    return OssImage(
      imageSource: trimmed,
      scene: ImageScene.merchantAvatar,
      fit: BoxFit.cover,
      width: 36.w,
      height: 36.h,
      errorWidget: Icon(
        AppIcons.store,
        size: 20.sp,
        color: AppIconColors.secondary,
      ),
    );
  }
}

class _MemberAvatarNew extends StatelessWidget {
  const _MemberAvatarNew({required this.avatarUrl});
  final String avatarUrl;

  @override
  Widget build(BuildContext context) {
    final placeholder = Container(
      width: 40.w,
      height: 40.h,
      decoration: const BoxDecoration(
        color: AppColors.bgCard,
        shape: BoxShape.circle,
      ),
      child: Icon(AppIcons.person, size: 24.sp, color: AppIconColors.secondary),
    );

    final trimmed = avatarUrl.trim();
    if (trimmed.isEmpty) return placeholder;

    if (isInlineImageData(trimmed)) {
      try {
        final bytes = base64Decode(
          trimmed.contains(',') ? trimmed.split(',').last : trimmed,
        );
        return Container(
          width: 40.w,
          height: 40.h,
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            shape: BoxShape.circle,
            image: DecorationImage(
              image: MemoryImage(bytes),
              fit: BoxFit.cover,
            ),
          ),
        );
      } catch (_) {
        return placeholder;
      }
    }

    return ClipOval(
      child: SizedBox(
        width: 40.w,
        height: 40.h,
        child: OssImage(
          imageSource: trimmed,
          scene: ImageScene.avatarSmall,
          width: 40.w,
          height: 40.h,
          fit: BoxFit.cover,
          errorWidget: Container(
            width: 40.w,
            height: 40.h,
            color: AppColors.bgCard,
            child: Icon(
              AppIcons.person,
              size: 24.sp,
              color: AppIconColors.secondary,
            ),
          ),
        ),
      ),
    );
  }
}

class CategoryMarketplaceProductSection extends ConsumerWidget {
  const CategoryMarketplaceProductSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final recommendations = ref.watch(categoryMarketplaceControllerProvider);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 16.w),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                '\u5546\u54c1\u63a8\u85a6',
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                  color: AppColors.textPrimary,
                ),
              ),
              GestureDetector(
                onTap: () => context.push('/?tab=1'),
                child: Text(
                  '\u67e5\u770b\u66f4\u591a',
                  style: TextStyle(fontSize: 14, color: Color(0xB28F3315)),
                ),
              ),
            ],
          ),
        ),
        SizedBox(height: 12.h),
        recommendations.when(
          loading: () => SizedBox(
            key: const ValueKey('category-marketplace-loading'),
            height: 220.h,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              itemCount: 4,
              separatorBuilder: (_, __) => SizedBox(width: 12.w),
              itemBuilder: (_, __) => const _ProductCardSkeleton(),
            ),
          ),
          error: (_, __) => Container(
            key: const ValueKey('category-marketplace-error'),
            height: 200.h,
            padding: EdgeInsets.symmetric(horizontal: 24.w),
            alignment: Alignment.center,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  '推薦商品載入失敗',
                  style: TextStyle(color: AppColors.textSecondary),
                ),
                SizedBox(height: 12.h),
                OutlinedButton(
                  key: const ValueKey('category-marketplace-retry'),
                  onPressed: () => ref
                      .read(categoryMarketplaceControllerProvider.notifier)
                      .refresh(),
                  style: OutlinedButton.styleFrom(
                    minimumSize: const Size(96, 48),
                  ),
                  child: const HkButtonLabel('重新載入'),
                ),
              ],
            ),
          ),
          data: (items) => items.isEmpty
              ? Container(
                  key: const ValueKey('category-marketplace-empty'),
                  height: 200.h,
                  alignment: Alignment.center,
                  child: Text(
                    '\u66ab\u7121\u63a8\u85a6',
                    style: TextStyle(color: AppColors.textSecondary),
                  ),
                )
              : SizedBox(
                  key: const ValueKey('category-marketplace-products'),
                  height: 220.h,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    padding: EdgeInsets.symmetric(horizontal: 16.w),
                    itemCount: items.length,
                    separatorBuilder: (_, __) => SizedBox(width: 12.w),
                    itemBuilder: (_, i) => _ProductCard(product: items[i]),
                  ),
                ),
        ),
      ],
    );
  }
}

class _ProductCard extends StatelessWidget {
  const _ProductCard({required this.product});

  final CategoryMarketplaceProduct product;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      key: ValueKey('category-marketplace-product-${product.id}'),
      onTap: () {
        context.push(
          '/merchant-product',
          extra: MerchantItemDetailScreen(
            title: product.name,
            imageUrl: product.imageUrl.isNotEmpty ? product.imageUrl : null,
            price: product.navigationPrice,
            skuId: product.id,
            merchantName: product.merchantName,
            merchantId: product.merchantId.isEmpty ? null : product.merchantId,
          ),
        );
      },
      child: Container(
        width: 150.w,
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 8,
              offset: const Offset(0, 2),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Stack(
              children: [
                ClipRRect(
                  borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(16),
                  ),
                  child: Container(
                    height: 130.h,
                    width: double.infinity,
                    color: AppColors.gray100,
                    child: product.imageUrl.isEmpty
                        ? Icon(
                            AppIcons.imageOutlined,
                            size: 40.sp,
                            color: AppIconColors.disabled,
                          )
                        : OssImage(
                            imageSource: product.imageUrl,
                            scene: ImageScene.homeProductCard,
                            fit: BoxFit.cover,
                          ),
                  ),
                ),
                Positioned(
                  bottom: 8,
                  right: 8,
                  child: DecoratedBox(
                    decoration: const BoxDecoration(
                      color: AppColors.white,
                      shape: BoxShape.circle,
                    ),
                    child: SizedBox(
                      width: 36.w,
                      height: 36.h,
                      child: Icon(
                        AppIcons.shoppingCartOutlined,
                        size: 20.sp,
                        color: AppIconColors.defaultColor,
                      ),
                    ),
                  ),
                ),
              ],
            ),
            Padding(
              padding: EdgeInsets.all(12.w),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    product.name,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.bold,
                      color: AppColors.textPrimary,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  SizedBox(height: 4.h),
                  Text(
                    switch (PriceDisplay.formatPrice(
                      product.priceMin,
                      prefix: 'NT\$ ',
                    )) {
                      '' => '',
                      final label => '$label / \u65a4',
                    },
                    style: TextStyle(
                      fontSize: 14,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ProductCardSkeleton extends StatelessWidget {
  const _ProductCardSkeleton();

  @override
  Widget build(BuildContext context) {
    return HkSkeletonShimmer(
      child: Container(
        width: 150.w,
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 8,
              offset: const Offset(0, 2),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            ClipRRect(
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(16),
              ),
              child: Container(
                height: 130.h,
                width: double.infinity,
                color: AppColors.dividerLight,
              ),
            ),
            Padding(
              padding: EdgeInsets.all(12.w),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    height: 14.h,
                    width: 100.w,
                    decoration: BoxDecoration(
                      color: AppColors.dividerLight,
                      borderRadius: BorderRadius.circular(6.r),
                    ),
                  ),
                  SizedBox(height: 8.h),
                  Container(
                    height: 12.h,
                    width: 70.w,
                    decoration: BoxDecoration(
                      color: AppColors.dividerLight,
                      borderRadius: BorderRadius.circular(6.r),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
