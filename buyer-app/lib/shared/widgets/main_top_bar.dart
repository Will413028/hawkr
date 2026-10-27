import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'cart_icon_with_badge.dart';
import 'category_top_bar.dart';
import 'chat_icon_with_badge.dart';
import 'common_top_bar.dart';
import 'hk_icon.dart';
import 'hk_tap_target.dart';
import 'logo_home_button.dart';
import 'text_size_top_bar_action.dart';
import 'top_navigation.dart';

/// 五個主要分頁共用的官網式 mobile header。
///
/// Header 本身只保留品牌與全域操作；首頁搜尋獨立成下一列。首頁可加入字級
/// action，窄螢幕則改用草標，所有 action 仍維持 48dp。
PreferredSize buildMainTopBar(
  BuildContext context, {
  bool showTextSize = false,
}) {
  return buildCommonAppBar(
    context,
    backgroundColor: AppColors.topBarBackground,
    systemOverlayStyle: AppColors.glassTopOverlay,
    frostedGlass: true,
    child: MainTopBarContent(showTextSize: showTextSize),
  );
}

class MainTopBarContent extends StatelessWidget {
  const MainTopBarContent({super.key, this.showTextSize = false});

  final bool showTextSize;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final gap = constraints.maxWidth < 336 ? HkSpace.xs : HkSpace.sm;
        final actionCount = showTextSize ? 4 : 3;
        final fixedActionWidth =
            actionCount * HkControl.minTap + (actionCount - 1) * gap;
        final useCompactBrand =
            constraints.maxWidth <
            HawkrWordmarkButton.wordmarkWidth + fixedActionWidth;
        return Row(
          children: [
            Expanded(
              child: Align(
                alignment: Alignment.centerLeft,
                child: HawkrWordmarkButton(compact: useCompactBrand),
              ),
            ),
            if (showTextSize) ...[
              const TextSizeTopBarAction(),
              SizedBox(width: gap),
            ],
            const CartIconWithBadge(compact: true, iconSize: HkControl.icon),
            SizedBox(width: gap),
            const NotificationBellButton(),
            SizedBox(width: gap),
            const ChatIconWithBadge(compact: true, iconSize: HkControl.icon),
          ],
        );
      },
    );
  }
}

/// 訂單、付款、物流等情境頁共用的「返回＋搜尋＋actions」Header variant。
/// 外框仍遵守相同 56 / 44 / 48 尺寸，不再讓舊頁各自使用 36、38、40px。
class HawkrSearchTopBar extends StatelessWidget {
  const HawkrSearchTopBar({
    super.key,
    this.hintText = '搜尋商品',
    this.onBack,
    this.onSearch,
    this.actions,
  });

  final String hintText;
  final VoidCallback? onBack;
  final VoidCallback? onSearch;
  final List<Widget>? actions;

  @override
  Widget build(BuildContext context) {
    return CategoryTopBar(
      backgroundColor: AppColors.topBarBackground,
      systemOverlayStyle: AppColors.glassTopOverlay,
      leading: HkTapTarget(
        semanticLabel: '返回',
        onTap: onBack ?? () => Navigator.maybePop(context),
        child: const HkIcon(
          HkIconName.back,
          size: HkControl.icon,
          color: AppIconColors.strong,
        ),
      ),
      middle: CategorySearchField(
        readOnly: true,
        hintText: hintText,
        onTap: onSearch,
      ),
      actions:
          actions ??
          const [
            CartIconWithBadge(compact: true, iconSize: HkControl.icon),
            ChatIconWithBadge(compact: true, iconSize: HkControl.icon),
          ],
    );
  }
}

/// 情境 Header 右側 icon 的 48dp 共用 action wrapper。
class HawkrTopBarAction extends StatelessWidget {
  const HawkrTopBarAction({
    super.key,
    required this.semanticLabel,
    required this.icon,
    this.onTap,
  });

  final String semanticLabel;
  final Widget icon;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return HkTapTarget(semanticLabel: semanticLabel, onTap: onTap, child: icon);
  }
}
