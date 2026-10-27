import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/physics.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'picker_shared_widgets.dart';

/// 左側子分類抽屜 + 拖拉把手
///
/// 使用 [Positioned.fill] 鋪滿父 Stack，內部再以 Stack + Positioned 管理：
/// - 遮罩層（抽屜展開時，點擊右側收起）
/// - 左側子分類清單（可拖拉寬度）
/// - 把手（三點圓形，跟隨抽屜寬度）
class PickerSubcatDrawer extends StatelessWidget {
  const PickerSubcatDrawer({
    super.key,
    required this.sidebarAnim,
    required this.sidebarWidth,
    required this.loading,
    required this.loadFinished,
    required this.subcats,
    required this.activeSubcat,
    required this.leftListController,
    required this.onSubcatSelected,
    required this.onAddCategory,
  });

  final AnimationController sidebarAnim;
  final double sidebarWidth;
  final bool loading;
  final bool loadFinished;
  final List<String> subcats;
  final String? activeSubcat;
  final ScrollController leftListController;
  final void Function(String subcat) onSubcatSelected;
  final VoidCallback onAddCategory;

  void _handleDragUpdate(DragUpdateDetails details) {
    sidebarAnim.stop();
    sidebarAnim.value = (sidebarAnim.value + details.delta.dx).clamp(
      90.0.w,
      160.0.w,
    );
  }

  void _handleDragEnd(DragEndDetails details) {
    final mid = (90.0.w + 160.0.w) / 2;
    final target = sidebarAnim.value > mid ? 160.0.w : 90.0.w;
    sidebarAnim.animateWith(
      SpringSimulation(
        const SpringDescription(mass: 0.8, stiffness: 500.0, damping: 28.0),
        sidebarAnim.value,
        target,
        details.velocity.pixelsPerSecond.dx,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        // 遮罩：抽屜展開時覆蓋右側，點擊收起
        if (sidebarWidth > 90.0.w)
          Positioned(
            left: sidebarWidth + kPickerHandleWidth,
            top: 0,
            right: 0,
            bottom: 0,
            child: GestureDetector(
              onTap: () => sidebarAnim.animateWith(
                SpringSimulation(
                  const SpringDescription(
                    mass: 0.8,
                    stiffness: 500.0,
                    damping: 28.0,
                  ),
                  sidebarAnim.value,
                  90.0.w,
                  0.0,
                ),
              ),
              behavior: HitTestBehavior.opaque,
              child: const SizedBox.expand(),
            ),
          ),

        // 左側子分類清單
        Positioned(
          left: 0,
          top: 0,
          bottom: 0,
          width: sidebarWidth,
          child: GestureDetector(
            onHorizontalDragUpdate: _handleDragUpdate,
            onHorizontalDragEnd: _handleDragEnd,
            child: loading || (subcats.isEmpty && !loadFinished)
                ? ListView.builder(
                    padding: EdgeInsets.zero,
                    itemCount: 10,
                    itemBuilder: (_, __) => PickerShimmer(
                      child: Container(
                        height: 36.h,
                        color: AppColors.dividerLight,
                      ),
                    ),
                  )
                : ListView.builder(
                    controller: leftListController,
                    padding: EdgeInsets.zero,
                    itemCount: subcats.length + 1,
                    itemBuilder: (_, i) {
                      if (i == 0) {
                        return _AddCategoryEntry(onTap: onAddCategory);
                      }
                      final s = subcats[i - 1];
                      final sel = s == activeSubcat;
                      return GestureDetector(
                        onTap: () => onSubcatSelected(s),
                        child: Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 12.w,
                            vertical: 8.h,
                          ),
                          decoration: sel
                              ? BoxDecoration(
                                  color: AppColors.brandPrimaryStrong,
                                  borderRadius: BorderRadius.only(
                                    topRight: Radius.circular(24.r),
                                    bottomRight: Radius.circular(24.r),
                                  ),
                                )
                              : const BoxDecoration(color: AppColors.bgCard),
                          alignment: Alignment.centerLeft,
                          child: Text(
                            s,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 16.sp,
                              color: sel
                                  ? AppColors.bgCard
                                  : AppColors.textSecondary,
                              fontWeight: sel
                                  ? FontWeight.bold
                                  : FontWeight.w400,
                              height: 1.5,
                            ),
                          ),
                        ),
                      );
                    },
                  ),
          ),
        ),

        // 拖拉把手
        Positioned(
          left: sidebarWidth,
          top: 0,
          bottom: 0,
          width: kPickerHandleWidth,
          child: GestureDetector(
            onHorizontalDragUpdate: _handleDragUpdate,
            onHorizontalDragEnd: _handleDragEnd,
            behavior: HitTestBehavior.opaque,
            child: SizedBox(
              width: kPickerHandleWidth,
              child: Center(
                child: Container(
                  padding: EdgeInsets.symmetric(horizontal: 4.w, vertical: 8.h),
                  decoration: BoxDecoration(
                    color: AppColors.bgPrimary,
                    borderRadius: const BorderRadius.only(
                      topRight: Radius.circular(6),
                      bottomRight: Radius.circular(6),
                    ),
                    boxShadow: [
                      BoxShadow(
                        color: AppColors.cardShadow,
                        blurRadius: 4,
                        offset: const Offset(0, 4),
                      ),
                    ],
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: List.generate(
                      3,
                      (i) => Padding(
                        padding: EdgeInsets.only(top: i == 0 ? 0 : 4),
                        child: Container(
                          width: 6,
                          height: 6,
                          decoration: const BoxDecoration(
                            color: AppColors.brandSecondary,
                            shape: BoxShape.circle,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

/// L2 抽屜頂部「新增」入口：綠色圓形 + icon 搭配「新增」文字。
/// 點擊會呼叫 [onTap]，由上層開啟新增分類 dialog。
class _AddCategoryEntry extends StatelessWidget {
  const _AddCategoryEntry({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Container(
        color: AppColors.bgCard,
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
        child: Wrap(
          spacing: 6.w,
          runSpacing: 4.h,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Container(
              width: 24.w,
              height: 24.w,
              decoration: const BoxDecoration(
                color: AppColors.primary,
                shape: BoxShape.circle,
              ),
              alignment: Alignment.center,
              child: Icon(
                AppIcons.add,
                color: AppIconColors.onBrand,
                size: 16.sp,
              ),
            ),
            Text(
              '新增',
              style: TextStyle(
                fontSize: 16.sp,
                color: AppColors.textSecondary,
                fontWeight: FontWeight.w400,
                height: 1.5,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
