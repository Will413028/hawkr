import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

class IconInfoRow extends StatelessWidget {
  const IconInfoRow({
    super.key,
    required this.icon,
    required this.text,
    this.textStyle,
  });

  final IconData icon;
  final String text;
  final TextStyle? textStyle;

  static TextStyle get defaultTextStyle => TextStyle(
    fontFamily: 'Roboto',
    fontSize: 16.sp,
    fontWeight: FontWeight.w400,
    height: 24 / 16,
    letterSpacing: -0.5,
    color: AppColors.textPrimary,
  );

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 24.w,
          height: 24.w,
          child: FittedBox(
            child: Icon(icon, color: AppIconColors.defaultColor),
          ),
        ),
        SizedBox(width: 5.w),
        Expanded(
          child: Text(
            text,
            style: textStyle ?? defaultTextStyle,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }
}
