import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import 'hk_icon.dart';
import 'hk_glass_chrome_surface.dart';
import 'hk_tap_target.dart';

/// 舊名稱保留相容；實際為 iOS 26 cream glass chrome。
const Color kGreenHeaderColor = AppColors.topBarGlass;

// ============================================================================
// Header 圖示按鈕元件
// ============================================================================

/// 聊天圖示按鈕
class HeaderChatIcon extends StatelessWidget {
  const HeaderChatIcon({
    super.key,
    this.onPressed,
    this.color = AppColors.chromeForeground,
  });
  final VoidCallback? onPressed;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      onPressed: onPressed,
      icon: HkIcon(HkIconName.chat, size: 22, color: color),
      tooltip: '訊息',
    );
  }
}

/// 文字按鈕（如：儲存、完成）
class HeaderTextButton extends StatelessWidget {
  const HeaderTextButton({
    super.key,
    required this.text,
    this.onPressed,
    this.color = AppColors.actionPrimary,
  });
  final String text;
  final VoidCallback? onPressed;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return TextButton(
      onPressed: onPressed,
      style: TextButton.styleFrom(
        foregroundColor: color,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        minimumSize: const Size(48, 48),
      ),
      child: HkButtonLabel(
        text,
        style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
      ),
    );
  }
}

// ============================================================================
// 綠色頂部搜尋/導航欄
// ============================================================================

/// 綠色頂部搜尋/導航欄
///
/// 用於多個頁面的統一綠色 header，支援多種變體：
///
/// **變體 1: 搜尋欄 + 圖示**
/// ```dart
/// GreenHeaderBar.search(
///   searchController: controller,
///   trailing: [HeaderCartIcon(), HeaderChatIcon()],
/// )
/// ```
///
/// **變體 2: 返回 + 搜尋欄 + 圖示**
/// ```dart
/// GreenHeaderBar.backWithSearch(
///   searchController: controller,
///   trailing: [HeaderChatIcon()],
/// )
/// ```
///
/// **變體 3: 返回 + 標題 + 動作按鈕**
/// ```dart
/// GreenHeaderBar.backWithTitle(
///   title: '個人資訊',
///   actionText: '儲存',
///   onActionPressed: () {},
/// )
/// ```
///
/// **變體 4: 標題 + 副標題**
/// ```dart
/// GreenHeaderBar.titleWithSubtitle(
///   title: 'Hawkr',
///   subtitle: '桃園市 330',
///   trailing: [HeaderSettingsIcon()],
/// )
/// ```
class GreenHeaderBar extends StatelessWidget {
  const GreenHeaderBar({
    super.key,
    this.height = 56,
    this.showBackButton = true,
    this.onBackPressed,
    this.showSearchBar = false,
    this.searchController,
    this.searchHint = '搜尋',
    this.onSearchSubmitted,
    this.onSearchCleared,
    this.searchQuery = '',
    this.title,
    this.subtitle,
    this.titleWidget,
    this.leading,
    this.trailing,
    this.centerTitle = true,
    this.backgroundColor = kGreenHeaderColor,
    this.includeTopSafeArea = false,
  });

  /// 變體 1: 只有搜尋欄 + 圖示（無返回按鈕）
  factory GreenHeaderBar.search({
    Key? key,
    TextEditingController? searchController,
    String searchHint = '搜尋',
    ValueChanged<String>? onSearchSubmitted,
    VoidCallback? onSearchCleared,
    String searchQuery = '',
    List<Widget>? trailing,
  }) {
    return GreenHeaderBar(
      key: key,
      showBackButton: false,
      showSearchBar: true,
      searchController: searchController,
      searchHint: searchHint,
      onSearchSubmitted: onSearchSubmitted,
      onSearchCleared: onSearchCleared,
      searchQuery: searchQuery,
      trailing: trailing,
    );
  }

  /// 變體 2: 返回 + 搜尋欄 + 圖示
  factory GreenHeaderBar.backWithSearch({
    Key? key,
    VoidCallback? onBackPressed,
    TextEditingController? searchController,
    String searchHint = '搜尋',
    ValueChanged<String>? onSearchSubmitted,
    VoidCallback? onSearchCleared,
    String searchQuery = '',
    List<Widget>? trailing,
  }) {
    return GreenHeaderBar(
      key: key,
      showBackButton: true,
      onBackPressed: onBackPressed,
      showSearchBar: true,
      searchController: searchController,
      searchHint: searchHint,
      onSearchSubmitted: onSearchSubmitted,
      onSearchCleared: onSearchCleared,
      searchQuery: searchQuery,
      trailing: trailing,
    );
  }

  /// 變體 3: 返回 + 標題 + 動作按鈕
  factory GreenHeaderBar.backWithTitle({
    Key? key,
    VoidCallback? onBackPressed,
    required String title,
    String? actionText,
    VoidCallback? onActionPressed,
  }) {
    return GreenHeaderBar(
      key: key,
      showBackButton: true,
      onBackPressed: onBackPressed,
      title: title,
      centerTitle: true,
      trailing: actionText != null
          ? [HeaderTextButton(text: actionText, onPressed: onActionPressed)]
          : null,
    );
  }

  /// 變體 4: 標題 + 副標題 + 圖示
  factory GreenHeaderBar.titleWithSubtitle({
    Key? key,
    required String title,
    String? subtitle,
    bool showBackButton = false,
    VoidCallback? onBackPressed,
    List<Widget>? trailing,
  }) {
    return GreenHeaderBar(
      key: key,
      showBackButton: showBackButton,
      onBackPressed: onBackPressed,
      title: title,
      subtitle: subtitle,
      centerTitle: true,
      trailing: trailing,
    );
  }

  /// 變體 6: 返回 + 標題 + 多圖示
  factory GreenHeaderBar.backWithTitleAndIcons({
    Key? key,
    VoidCallback? onBackPressed,
    required String title,
    List<Widget>? trailing,
  }) {
    return GreenHeaderBar(
      key: key,
      showBackButton: true,
      onBackPressed: onBackPressed,
      title: title,
      centerTitle: true,
      trailing: trailing,
    );
  }

  /// Header 高度，預設 56
  final double height;

  /// 是否顯示返回按鈕
  final bool showBackButton;

  /// 返回按鈕點擊回調
  final VoidCallback? onBackPressed;

  /// 是否顯示搜尋欄
  final bool showSearchBar;

  /// 搜尋欄控制器
  final TextEditingController? searchController;

  /// 搜尋欄提示文字
  final String searchHint;

  /// 搜尋提交回調
  final ValueChanged<String>? onSearchSubmitted;

  /// 清除搜尋回調
  final VoidCallback? onSearchCleared;

  /// 當前搜尋內容（用於顯示清除按鈕）
  final String searchQuery;

  /// 標題文字
  final String? title;

  /// 副標題文字
  final String? subtitle;

  /// 自訂標題 Widget（優先於 title）
  final Widget? titleWidget;

  /// 左側自訂 Widget（優先於返回按鈕）
  final Widget? leading;

  /// 右側自訂 Widget 列表
  final List<Widget>? trailing;

  /// 是否置中標題
  final bool centerTitle;

  /// 背景顏色
  final Color backgroundColor;

  /// Paint the same glass behind the status bar and toolbar. The page's outer
  /// SafeArea must use top: false; the content inset is consumed inside here.
  final bool includeTopSafeArea;

  @override
  Widget build(BuildContext context) {
    final toolbar = SizedBox(
      height: subtitle == null ? height : height + HkSpace.lg,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
        child: centerTitle && !showSearchBar
            ? _buildCenteredLayout(context)
            : _buildRowLayout(context),
      ),
    );
    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: const SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: Brightness.dark,
        statusBarBrightness: Brightness.light,
      ),
      child: HkGlassChromeSurface(
        surfaceKey: const ValueKey('seller-glass-header-surface'),
        backgroundColor: backgroundColor,
        child: includeTopSafeArea
            ? SafeArea(bottom: false, child: toolbar)
            : toolbar,
      ),
    );
  }

  /// Measure the side controls before giving the title its available width.
  /// A Stack lets long titles paint underneath back/save buttons.
  Widget _buildCenteredLayout(BuildContext context) {
    return NavigationToolbar(
      centerMiddle: true,
      middleSpacing: 8,
      middle: _buildTitle(),
      leading:
          leading ??
          (showBackButton
              ? IconButton(
                  icon: const HkIcon(
                    HkIconName.back,
                    color: AppIconColors.defaultColor,
                  ),
                  onPressed:
                      onBackPressed ?? () => Navigator.of(context).maybePop(),
                  tooltip: '返回',
                )
              : null),
      trailing: trailing == null
          ? null
          : Row(mainAxisSize: MainAxisSize.min, children: trailing!),
    );
  }

  /// 橫向佈局（使用 Row）
  Widget _buildRowLayout(BuildContext context) {
    return Row(
      children: [
        // 左側區域
        if (leading != null)
          leading!
        else if (showBackButton)
          IconButton(
            icon: const HkIcon(
              HkIconName.back,
              color: AppIconColors.defaultColor,
            ),
            onPressed: onBackPressed ?? () => Navigator.of(context).maybePop(),
            tooltip: '返回',
          ),

        // 中間區域
        Expanded(
          child: showSearchBar ? _buildSearchBar(context) : _buildTitle(),
        ),

        // 右側區域
        if (trailing != null) ...trailing!,
      ],
    );
  }

  Widget _buildSearchBar(BuildContext context) {
    return Container(
      height: 44,
      margin: const EdgeInsets.symmetric(horizontal: 8),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(24),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 8,
            offset: Offset(0, 2),
          ),
        ],
      ),
      child: Row(
        children: [
          const SizedBox(width: 12),
          const HkIcon(HkIconName.search, size: 20, color: AppIconColors.muted),
          const SizedBox(width: 8),
          Expanded(
            child: TextField(
              controller: searchController,
              textInputAction: TextInputAction.search,
              textAlignVertical: TextAlignVertical.center,
              style: const TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w400,
                height: 1.0,
                color: AppColors.textEmphasis,
              ),
              decoration: InputDecoration(
                isCollapsed: true,
                hintText: searchHint,
                hintStyle: const TextStyle(
                  color: AppColors.textSecondary,
                  fontSize: 16,
                  fontWeight: FontWeight.w400,
                  height: 1.0,
                ),
                border: InputBorder.none,
                contentPadding: EdgeInsets.zero,
              ),
              onSubmitted: onSearchSubmitted,
            ),
          ),
          if (searchQuery.isNotEmpty)
            HkTapTarget(
              semanticLabel: '清除搜尋',
              onTap: () {
                searchController?.clear();
                onSearchCleared?.call();
              },
              child: HkIcon(
                HkIconName.close,
                size: 18,
                color: AppIconColors.muted,
              ),
            ),
          const SizedBox(width: 12),
        ],
      ),
    );
  }

  Widget _buildTitle() {
    if (titleWidget != null) {
      return DefaultTextStyle.merge(
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        child: titleWidget!,
      );
    }

    if (title != null) {
      if (subtitle != null) {
        return Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              title!,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                color: AppColors.chromeForeground,
                fontSize: 20,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 2),
            Text(
              subtitle!,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                color: AppColors.chromeMuted,
                fontSize: 12,
              ),
            ),
          ],
        );
      }
      return Text(
        title!,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(
          color: AppColors.chromeForeground,
          fontSize: 20,
          fontWeight: FontWeight.w700,
        ),
      );
    }
    return const SizedBox.shrink();
  }
}
