import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

import '../../../shared/widgets/hk_button_label.dart';

/// 分頁清單第一次讀取失敗的畫面：與「真的沒資料」的空狀態分開，避免使用者誤以為沒有任何項目；提供重試。
class PagedErrorState extends StatelessWidget {
  const PagedErrorState({super.key, required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 32.w),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              AppIcons.errorOutline,
              color: AppIconColors.danger,
              size: 48.sp,
            ),
            SizedBox(height: 12.h),
            Text(
              '載入失敗，請稍後再試',
              textAlign: TextAlign.center,
              style: TextStyle(color: AppColors.textPrimary, fontSize: 14),
            ),
            SizedBox(height: 12.h),
            TextButton(onPressed: onRetry, child: const HkButtonLabel('重試')),
          ],
        ),
      ),
    );
  }
}

/// 分頁清單底部：載入更多中顯示 loader，否則是「查看更多」按鈕（[moreKey] 供測試定位）。
class PagedLoadMoreFooter extends StatelessWidget {
  const PagedLoadMoreFooter({
    super.key,
    required this.moreKey,
    required this.loading,
    required this.onMore,
  });

  final Key moreKey;
  final bool loading;
  final VoidCallback onMore;

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return Center(
        child: SizedBox(
          width: 20.w,
          height: 20.w,
          child: const HkBrandLoader(
            tone: HkBrandLoaderTone.mono,
            color: AppColors.primary,
          ),
        ),
      );
    }
    return TextButton(
      key: moreKey,
      onPressed: onMore,
      child: const HkButtonLabel('查看更多'),
    );
  }
}
