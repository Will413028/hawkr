import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 拖拉把手寬度（sidebar 與右側面板之間的分隔區域）
const double kPickerHandleWidth = 14;

/// L3 分類卡片（綠色垂直條 + 分類名稱）
class PickerL3CategoryCard extends StatelessWidget {
  const PickerL3CategoryCard({
    super.key,
    required this.name,
    required this.itemCount,
    required this.onTap,
    this.isSelected = false,
  });

  final String name;
  final int itemCount;
  final VoidCallback onTap;
  final bool isSelected;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: EdgeInsets.only(bottom: 8.h),
        padding: EdgeInsets.symmetric(vertical: 12.h, horizontal: 12.w),
        decoration: BoxDecoration(
          color: isSelected ? AppColors.primarySoft : AppColors.bgCard,
          borderRadius: BorderRadius.circular(8.r),
          border: Border.all(
            color: isSelected ? AppColors.primary : AppColors.cardBorder,
            width: isSelected ? 2 : 1,
          ),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 4,
              offset: Offset(0, 2.h),
            ),
          ],
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Container(
              width: 4.w,
              height: 20.h,
              decoration: BoxDecoration(
                color: AppColors.primary,
                borderRadius: BorderRadius.circular(2.r),
              ),
            ),
            SizedBox(width: 12.w),
            Expanded(
              child: Text(
                name,
                style: TextStyle(
                  fontSize: 16.sp,
                  fontWeight: isSelected ? FontWeight.w700 : FontWeight.w400,
                  color: AppColors.textPrimary,
                  height: 1.5,
                ),
              ),
            ),
            if (itemCount > 0)
              Text(
                '$itemCount 項',
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textSecondary,
                  height: 1.5,
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// 骨架列表（載入中佔位）
class PickerSkeletonList extends StatelessWidget {
  const PickerSkeletonList({super.key, required this.count});
  final int count;

  @override
  Widget build(BuildContext context) {
    return ListView.separated(
      padding: EdgeInsets.symmetric(horizontal: 8.w),
      itemCount: count,
      separatorBuilder: (_, __) => SizedBox(height: 12.h),
      itemBuilder: (_, __) => const PickerSkeletonCard(),
    );
  }
}

/// 骨架卡片
class PickerSkeletonCard extends StatelessWidget {
  const PickerSkeletonCard({super.key});

  @override
  Widget build(BuildContext context) {
    return PickerShimmer(
      child: Container(
        height: 110.h,
        decoration: BoxDecoration(
          color: AppColors.bgElevated,
          borderRadius: BorderRadius.circular(12.r),
          border: Border.all(color: AppColors.neutralPanel, width: 1),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 6,
              offset: Offset(0, 2.h),
            ),
          ],
        ),
        child: Row(
          children: [
            Container(
              width: 110.w,
              height: 110.h,
              decoration: BoxDecoration(
                color: AppColors.dividerLight,
                borderRadius: BorderRadius.only(
                  topLeft: Radius.circular(12.r),
                  bottomLeft: Radius.circular(12.r),
                ),
              ),
            ),
            Expanded(
              child: Padding(
                padding: EdgeInsets.all(8.w),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    Container(
                      width: 100.w,
                      height: 14.h,
                      decoration: BoxDecoration(
                        color: AppColors.dividerLight,
                        borderRadius: BorderRadius.circular(8.r),
                      ),
                    ),
                    Container(
                      width: 60.w,
                      height: 12.h,
                      decoration: BoxDecoration(
                        color: AppColors.dividerLight,
                        borderRadius: BorderRadius.circular(8.r),
                      ),
                    ),
                    Container(
                      width: 80.w,
                      height: 14.h,
                      decoration: BoxDecoration(
                        color: AppColors.dividerLight,
                        borderRadius: BorderRadius.circular(8.r),
                      ),
                    ),
                    Align(
                      alignment: Alignment.centerRight,
                      child: Container(
                        width: 50.w,
                        height: 22.h,
                        decoration: BoxDecoration(
                          color: AppColors.dividerLight,
                          borderRadius: BorderRadius.circular(16.r),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 閃爍載入動畫
class PickerShimmer extends StatefulWidget {
  const PickerShimmer({super.key, required this.child});
  final Widget child;

  @override
  State<PickerShimmer> createState() => _PickerShimmerState();
}

class _PickerShimmerState extends State<PickerShimmer>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1100),
    )..repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final double width = constraints.maxWidth.isFinite
            ? constraints.maxWidth
            : MediaQuery.of(context).size.width;
        final double shimmerWidth = width * 0.45;
        final double start = -shimmerWidth;
        final double end = width;

        return AnimatedBuilder(
          animation: _controller,
          builder: (_, __) {
            final double dx = start + (end - start) * _controller.value;
            return Stack(
              children: [
                widget.child,
                Positioned(
                  left: dx,
                  top: 0,
                  bottom: 0,
                  child: IgnorePointer(
                    child: Container(
                      width: shimmerWidth,
                      decoration: const BoxDecoration(
                        gradient: LinearGradient(
                          begin: Alignment.centerLeft,
                          end: Alignment.centerRight,
                          colors: [
                            Color(0x00FFFFFF),
                            Color(0xCCFFFFFF),
                            Color(0x00FFFFFF),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            );
          },
        );
      },
    );
  }
}
