import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/app_colors.dart';
import '../../shipment_anomaly/data/shipment_anomaly_providers.dart';

/// 首頁配送異常 banner（賣家端）。
///
/// total > 0 時顯示，點擊跳轉到 `/shipment-anomalies` dashboard。
/// 沿用既有 card 樣式（卡片邊框 + 陰影），紅色語意色標示警告。
class ShipmentAnomalyAlertBanner extends ConsumerWidget {
  const ShipmentAnomalyAlertBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final stats = ref.watch(shipmentAnomalyStatsProvider).value;
    final total = stats?.total ?? 0;
    if (total <= 0) return const SizedBox.shrink();

    return Padding(
      padding: EdgeInsets.only(bottom: 16.h),
      child: Material(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: () => context.push('/shipment-anomalies'),
          child: Container(
            decoration: BoxDecoration(
              border: Border.all(color: AppColors.error, width: 1),
              borderRadius: BorderRadius.circular(16.r),
              boxShadow: const [
                BoxShadow(
                  color: AppColors.cardShadow,
                  blurRadius: 2,
                  offset: Offset(0, 1),
                ),
              ],
            ),
            padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
            child: Row(
              children: [
                Icon(
                  AppIcons.warningAmberRounded,
                  size: 24.sp,
                  color: AppIconColors.danger,
                ),
                SizedBox(width: 8.w),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        '您有 $total 筆配送異常',
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w700,
                          color: AppColors.error,
                          height: 1.3,
                        ),
                      ),
                      SizedBox(height: 2.h),
                      Text(
                        '點此查看並處理',
                        style: TextStyle(
                          fontSize: 12.sp,
                          fontWeight: FontWeight.w400,
                          color: AppColors.textPrimary,
                          height: 1.3,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
                  AppIcons.chevronRight,
                  size: 22.sp,
                  color: AppIconColors.muted,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
