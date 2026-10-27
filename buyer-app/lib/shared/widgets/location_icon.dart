import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import '../icons/app_icons.dart';

class HawkrLocationIcon extends StatelessWidget {
  const HawkrLocationIcon({
    super.key,
    this.size = 20,
    this.color = AppIconColors.defaultColor,
  });

  final double size;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return Icon(AppIcons.location, size: size, color: color);
  }
}
