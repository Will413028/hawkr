import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../icons/app_icons.dart';

/// [HkPlaceholder] 的用途。每個用途固定一個圖示與一組底色／圖示色。
enum HkPlaceholderKind {
  buyerAvatar(
    AppIcons.personOutline,
    AppColors.brandPrimarySoft,
    AppColors.brandPrimary,
  ),
  merchantAvatar(
    AppIcons.storefrontOutlined,
    AppColors.brandSecondarySoft,
    AppColors.brandSecondary,
  ),
  storeCover(
    AppIcons.storefrontOutlined,
    AppColors.brandSecondarySoft,
    AppColors.brandSecondary,
  ),
  photo(
    AppIcons.imageOutlined,
    AppColors.brandPrimarySoft,
    AppColors.brandPrimary,
  ),
  hero(
    AppIcons.nightlightOutlined,
    AppColors.brandSecondarySoft,
    AppColors.brandSecondary,
  );

  const HkPlaceholderKind(this.icon, this.background, this.foreground);

  final IconData icon;
  final Color background;
  final Color foreground;
}

/// 沒有圖片時的佔位：單色底加一個 Material 圖示，填滿父層給的框。
///
/// 形狀由呼叫端決定（圓形頭像外面包 `ClipOval`）；需要固定尺寸時由呼叫端包
/// `SizedBox`。某個方向沒有限制時不撐開，交給圖示的大小。圖示大小是框短邊的
/// [iconScale]，短邊沒有限制時用 `IconTheme` 的預設。佔位只是裝飾，不進語意樹。
class HkPlaceholder extends StatelessWidget {
  const HkPlaceholder(this.kind, {super.key});

  static const double iconScale = 0.4;

  final HkPlaceholderKind kind;

  @override
  Widget build(BuildContext context) {
    return ExcludeSemantics(
      child: LayoutBuilder(
        builder: (context, constraints) {
          final shortest = math.min(
            constraints.maxWidth,
            constraints.maxHeight,
          );
          return ColoredBox(
            color: kind.background,
            child: Center(
              child: Icon(
                kind.icon,
                color: kind.foreground,
                size: shortest.isFinite ? shortest * iconScale : null,
              ),
            ),
          );
        },
      ),
    );
  }
}
