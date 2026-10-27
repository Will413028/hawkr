import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/magnetic_tab_bar.dart';
import 'package:flutter/material.dart';

import 'merchant_layout.dart';

/// Storefront tabs share the Web-style magnetic interaction used by the
/// buyer bottom navigation: drag across labels, release to commit, or swipe
/// the content and keep the selection treatment attached to the page.
class MerchantTabBar extends StatelessWidget implements PreferredSizeWidget {
  const MerchantTabBar({super.key, required this.tabController});

  final TabController tabController;

  static const List<String> _tabs = ['貼文', '商城', '留言'];

  @override
  Size get preferredSize =>
      const Size.fromHeight(MerchantLayoutSpec.tabBarHeight);

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: AppColors.storefrontPageSurface,
      child: MagneticTabBar(
        controller: tabController,
        labels: _tabs,
        height: MerchantLayoutSpec.tabBarHeight,
        maxWidth: 560,
        indicatorKey: const Key('merchant_tab_indicator'),
        labelKeyPrefix: 'merchant_tab_label_',
        showMaterialTapEffect: false,
        iconBuilders: [
          (selected) => HkIcon(
            HkIconName.merchantPosts,
            key: const ValueKey('merchant-posts-tab-icon'),
            selected: selected,
            size: 22,
            color: selected ? AppIconColors.strong : AppIconColors.muted,
          ),
          (selected) => HkIcon(
            HkIconName.merchantShop,
            key: const ValueKey('merchant-shop-tab-icon'),
            selected: selected,
            size: 22,
            color: selected ? AppIconColors.strong : AppIconColors.muted,
          ),
          (selected) => HkIcon(
            HkIconName.chat,
            key: const ValueKey('merchant-messages-tab-icon'),
            selected: selected,
            size: 22,
            color: selected ? AppIconColors.strong : AppIconColors.muted,
          ),
        ],
      ),
    );
  }
}

class MerchantTabBarSliverDelegate extends SliverPersistentHeaderDelegate {
  const MerchantTabBarSliverDelegate({required this.tabController});

  final TabController tabController;

  @override
  double get minExtent => MerchantLayoutSpec.tabBarHeight;

  @override
  double get maxExtent => MerchantLayoutSpec.tabBarHeight;

  @override
  Widget build(
    BuildContext context,
    double shrinkOffset,
    bool overlapsContent,
  ) {
    return MerchantTabBar(tabController: tabController);
  }

  @override
  bool shouldRebuild(covariant MerchantTabBarSliverDelegate oldDelegate) {
    return oldDelegate.tabController != tabController;
  }
}
