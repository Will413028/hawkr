import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';

/// 一行規格：左邊是名稱，右邊是限制值。
class ShippingSpecLine {
  const ShippingSpecLine(this.label, this.value);

  final String label;
  final String value;
}

/// 規格群組：一個圖示加一到多行限制，或一組可選尺寸。
class ShippingSpecGroup {
  const ShippingSpecGroup({
    required this.icon,
    this.lines = const [],
    this.heading,
    this.options = const [],
    this.note,
  });

  final IconData icon;
  final List<ShippingSpecLine> lines;
  final String? heading;
  final List<String> options;
  final String? note;
}

/// 物流規格卡：標題前有一個色點，下面是規格群組。
class ShippingSpecCard extends StatelessWidget {
  const ShippingSpecCard({
    super.key,
    required this.title,
    required this.dotColor,
    required this.groups,
  });

  final String title;
  final Color dotColor;
  final List<ShippingSpecGroup> groups;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            ExcludeSemantics(
              child: Container(
                width: 8.w,
                height: 8.w,
                decoration: BoxDecoration(
                  color: dotColor,
                  shape: BoxShape.circle,
                ),
              ),
            ),
            SizedBox(width: 8.w),
            Text(
              title,
              style: TextStyle(
                fontSize: 14.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
          ],
        ),
        for (final group in groups) ...[
          SizedBox(height: 8.h),
          _SpecGroup(group),
        ],
      ],
    );
  }
}

class _SpecGroup extends StatelessWidget {
  const _SpecGroup(this.group);

  final ShippingSpecGroup group;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(12.w),
      decoration: BoxDecoration(
        color: AppColors.gray50,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ExcludeSemantics(
            child: Container(
              width: 32.w,
              height: 32.w,
              decoration: BoxDecoration(
                color: AppColors.success,
                borderRadius: BorderRadius.circular(8.r),
              ),
              child: Icon(group.icon, size: 18.w, color: AppIconColors.onBrand),
            ),
          ),
          SizedBox(width: 12.w),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (group.heading != null)
                  Text(
                    group.heading!,
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                for (final line in group.lines)
                  Padding(
                    padding: EdgeInsets.symmetric(vertical: 4.h),
                    child: Row(
                      children: [
                        Expanded(
                          child: Text(
                            line.label,
                            style: TextStyle(
                              fontSize: 14.sp,
                              color: AppColors.textSecondary,
                            ),
                          ),
                        ),
                        _ValuePill(line.value),
                      ],
                    ),
                  ),
                if (group.options.isNotEmpty)
                  Padding(
                    padding: EdgeInsets.only(top: 4.h),
                    child: Wrap(
                      spacing: 8.w,
                      runSpacing: 4.h,
                      children: [
                        for (final option in group.options) _ValuePill(option),
                      ],
                    ),
                  ),
                if (group.note != null)
                  Padding(
                    padding: EdgeInsets.only(top: 4.h),
                    child: Text(
                      '・${group.note}',
                      style: TextStyle(
                        fontSize: 12.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ValuePill extends StatelessWidget {
  const _ValuePill(this.value);

  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
      decoration: BoxDecoration(
        color: AppColors.successSoft,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        value,
        style: TextStyle(
          fontSize: 14.sp,
          fontWeight: FontWeight.w500,
          color: AppColors.success,
        ),
      ),
    );
  }
}
