import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';

import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/glass_tab_bar.dart';
import 'page_header.dart';

/// 商家頁面固定置頂的 Tab 列，作為 SliverAppBar.bottom 使用
class PageTabBar extends StatelessWidget implements PreferredSizeWidget {
  const PageTabBar({super.key, required this.tabController});

  final TabController tabController;

  @override
  Size get preferredSize =>
      const Size.fromHeight(PageHeaderLayoutSpec.tabBarHeight);

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: tabController,
      builder: (context, _) => ColoredBox(
        color: AppColors.storefrontPageSurface,
        child: HkGlassTabBar(
          labels: const ['貼文', '商城', '留言'],
          icons: const [HkIconName.post, HkIconName.market, HkIconName.chat],
          selectedIndex: tabController.index,
          onTap: (index) => tabController.animateTo(
            index,
            duration: MediaQuery.maybeOf(context)?.disableAnimations ?? false
                ? Duration.zero
                : HkMotion.duration,
            curve: HkMotion.ease,
          ),
          height: PageHeaderLayoutSpec.tabBarHeight,
          labelKeyPrefix: 'seller_store_tab_',
        ),
      ),
    );
  }
}
