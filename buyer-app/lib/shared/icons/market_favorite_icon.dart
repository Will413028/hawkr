import 'package:flutter/material.dart';

import 'app_icons.dart';

/// 市集商品卡的收藏愛心：已收藏畫 filled，未收藏畫 outlined。
class MarketFavoriteIcon extends StatelessWidget {
  const MarketFavoriteIcon({
    super.key,
    required this.isFavorite,
    required this.color,
    this.size = 24,
  });

  final bool isFavorite;
  final Color color;
  final double size;

  IconData get iconData =>
      isFavorite ? AppIcons.favorite : AppIcons.favoriteBorder;

  @override
  Widget build(BuildContext context) {
    return ExcludeSemantics(
      child: Icon(iconData, size: size, color: color),
    );
  }
}
