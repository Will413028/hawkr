import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'picker_shared_widgets.dart';

/// 右側 L3 分類清單 + 新增子分類按鈕
///
/// 顯示目前選中 L2 下的所有 L3 分類，以及頂部「新增子分類」入口。
class PickerL3ContentPanel extends StatelessWidget {
  const PickerL3ContentPanel({
    super.key,
    required this.l3List,
    required this.activeSubcat,
    required this.selectedL3,
    required this.loading,
    required this.loadFinished,
    required this.scrollController,
    required this.onL3Selected,
    required this.onAddSubcategory,
  });

  final List<String> l3List;

  /// 當前選中的 L2 名稱（用於判斷是否顯示新增按鈕）
  final String? activeSubcat;

  /// 當前已選中的 L3 名稱（用於高亮顯示）
  final String? selectedL3;

  final bool loading;
  final bool loadFinished;
  final ScrollController scrollController;

  final void Function(String l3Name) onL3Selected;
  final VoidCallback onAddSubcategory;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // 頂部統計
        Padding(
          padding: EdgeInsets.all(8.w),
          child: Text(
            '共 ${l3List.length} 項分類',
            style: TextStyle(fontSize: 14.sp, color: AppColors.textEmphasis),
          ),
        ),
        // 清單
        Expanded(
          child: Builder(
            builder: (_) {
              if (loading && l3List.isEmpty && !loadFinished) {
                return PickerSkeletonList(count: 6);
              }

              final sections = <Widget>[];

              // 新增子分類按鈕
              if (activeSubcat != null && activeSubcat!.isNotEmpty) {
                sections.add(
                  Padding(
                    padding: EdgeInsets.only(bottom: 12.h),
                    child: GestureDetector(
                      onTap: onAddSubcategory,
                      child: Container(
                        width: 255.w,
                        padding: EdgeInsets.symmetric(
                          horizontal: 20.w,
                          vertical: 8.h,
                        ),
                        decoration: BoxDecoration(
                          color: AppColors.bgCard,
                          borderRadius: BorderRadius.circular(16.r),
                          border: Border.all(color: AppColors.cardBorder),
                          boxShadow: [
                            BoxShadow(
                              color: AppColors.cardShadow,
                              blurRadius: 4,
                              offset: Offset(0, 4.h),
                            ),
                          ],
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Container(
                              width: 24.w,
                              height: 24.h,
                              decoration: BoxDecoration(
                                color: AppColors.primary,
                                borderRadius: BorderRadius.circular(999.r),
                              ),
                              child: Icon(
                                AppIcons.add,
                                color: AppIconColors.onBrand,
                                size: 16.sp,
                              ),
                            ),
                            SizedBox(width: 6.w),
                            Text(
                              '新增子分類',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w400,
                                height: 1.5,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                );
              }

              for (final l3 in l3List) {
                sections.add(
                  PickerL3CategoryCard(
                    name: l3,
                    itemCount: 0,
                    isSelected: selectedL3 == l3,
                    onTap: () => onL3Selected(l3),
                  ),
                );
              }

              return ListView(
                controller: scrollController,
                padding: EdgeInsets.symmetric(horizontal: 8.w),
                children: sections,
              );
            },
          ),
        ),
      ],
    );
  }
}
