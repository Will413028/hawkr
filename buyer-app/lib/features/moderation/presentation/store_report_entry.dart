import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/utils/require_login.dart';

/// 檢舉店家表單的對象：[storeId] 是店家卡片的 id（不是 store_code）；從商品頁進來時帶 [productId] 與 [productName]。
class StoreReportArgs {
  const StoreReportArgs({
    required this.storeId,
    this.productId = '',
    this.productName = '',
  });

  final String storeId;
  final String productId;
  final String productName;

  bool get forProduct => productId.isNotEmpty;
}

const storeReportRoute = '/report/store';

/// 店面與商品頁「⋯」的共用入口：選單只有「檢舉」，訪客點了走登入閘門 [requireLogin]，登入後才進表單頁。
Future<void> showStoreReportMenu(
  BuildContext context,
  WidgetRef ref, {
  required StoreReportArgs args,
}) async {
  final picked = await showModalBottomSheet<bool>(
    context: context,
    showDragHandle: false,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (ctx) => SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(height: 8.h),
          Container(
            width: 36.w,
            height: 4.h,
            decoration: BoxDecoration(
              color: AppColors.gray300,
              borderRadius: BorderRadius.circular(2.r),
            ),
          ),
          SizedBox(height: 12.h),
          ListTile(
            key: const ValueKey('store-report-menu-report'),
            leading: Icon(
              AppIcons.reportOutlined,
              color: AppIconColors.warning,
              size: 22.sp,
            ),
            title: const Text('檢舉', style: TextStyle(fontSize: 16)),
            onTap: () => Navigator.pop(ctx, true),
          ),
          SizedBox(height: 8.h),
        ],
      ),
    ),
  );
  if (picked != true || !context.mounted) return;
  if (!await requireLogin(context, ref)) return;
  if (!context.mounted) return;
  await context.push(storeReportRoute, extra: args);
}
