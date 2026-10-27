import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../shared/widgets/app_dropdown.dart';

/// 分類下拉選單 + 搜尋欄列
///
/// 顯示 L1 類別下拉（[AppDropdown]）與關鍵字搜尋輸入框。
class PickerCategorySearchRow extends StatelessWidget {
  const PickerCategorySearchRow({
    super.key,
    required this.majors,
    required this.topTabLabels,
    required this.activeTop,
    required this.onTopChanged,
    required this.searchCtrl,
    required this.onSearch,
  });

  /// L1 類別 key 列表（Vegetables / Fruits / ...）
  final List<String> majors;

  /// L1 key → 顯示標籤 map
  final Map<String, String> topTabLabels;

  /// 當前選中的 L1 key
  final String activeTop;

  /// 選擇新 L1 時回傳對應 key
  final void Function(String key) onTopChanged;

  final TextEditingController searchCtrl;
  final VoidCallback onSearch;

  @override
  Widget build(BuildContext context) {
    final category = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '商品分類',
          style: TextStyle(
            fontSize: 14.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.textPrimary,
          ),
        ),
        SizedBox(width: 8.w),
        AppDropdown(
          items: majors.map((t) => topTabLabels[t] ?? t).toList(),
          selectedItem: topTabLabels[activeTop] ?? activeTop,
          onSelected: (label) {
            String? key;
            for (final entry in topTabLabels.entries) {
              if (entry.value == label) {
                key = entry.key;
                break;
              }
            }
            key ??= label;
            if (key != activeTop) onTopChanged(key);
          },
          width: 107.w,
          showSearch: true,
        ),
      ],
    );
    final search = Container(
      constraints: const BoxConstraints(minHeight: 48),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.textPrimary, width: 1),
      ),
      padding: EdgeInsets.symmetric(horizontal: 8.w),
      child: Row(
        children: [
          Icon(AppIcons.search, color: AppIconColors.muted, size: 20.sp),
          SizedBox(width: 6.w),
          Expanded(
            child: TextField(
              controller: searchCtrl,
              decoration: InputDecoration(
                hintText: '搜尋商品...',
                hintStyle: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textSecondary,
                ),
                filled: false,
                border: InputBorder.none,
                enabledBorder: InputBorder.none,
                focusedBorder: InputBorder.none,
                disabledBorder: InputBorder.none,
                errorBorder: InputBorder.none,
                focusedErrorBorder: InputBorder.none,
                isDense: true,
                contentPadding: EdgeInsets.zero,
              ),
              style: TextStyle(fontSize: 14.sp),
              onSubmitted: (_) => onSearch(),
            ),
          ),
        ],
      ),
    );
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 16.w),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final labelWidth = TextPainter(
            text: TextSpan(
              text: '商品分類',
              style: TextStyle(fontSize: 14.sp, fontWeight: FontWeight.w700),
            ),
            textScaler: MediaQuery.textScalerOf(context),
            textDirection: Directionality.of(context),
          )..layout();
          final categoryWidth = labelWidth.width + 8.w + 107.w;
          labelWidth.dispose();
          if (constraints.maxWidth < categoryWidth + 16.w + 170.w) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                category,
                SizedBox(height: 8.h),
                search,
              ],
            );
          }
          return Row(
            children: [
              category,
              SizedBox(width: 16.w),
              Expanded(child: search),
            ],
          );
        },
      ),
    );
  }
}
