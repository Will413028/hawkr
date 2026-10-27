import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../data/order_repository.dart';

class CancelOrderSuccessDialog extends StatelessWidget {
  const CancelOrderSuccessDialog({super.key, required this.result});

  final CancelOrderResult result;

  @override
  Widget build(BuildContext context) {
    final hasRefund = result.refundAmountInt > 0;
    return Dialog(
      backgroundColor: Colors.transparent,
      elevation: 0,
      child: Container(
        width: 280.w,
        padding: EdgeInsets.symmetric(horizontal: 24.w, vertical: 24.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 56.w,
              height: 56.w,
              decoration: const BoxDecoration(
                color: AppColors.primarySoft,
                shape: BoxShape.circle,
              ),
              child: Icon(
                AppIcons.checkRounded,
                size: 30.sp,
                color: AppIconColors.active,
              ),
            ),
            SizedBox(height: 16.h),
            Text(
              '訂單已取消',
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 6.h),
            Text(
              '已通知顧客，訂單 ${result.orderNo} 已取消',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            ),
            if (hasRefund) ...[
              SizedBox(height: 14.h),
              Container(
                width: double.infinity,
                padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
                decoration: BoxDecoration(
                  color: AppColors.bgPrimary,
                  borderRadius: BorderRadius.circular(12.r),
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      '退款金額',
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textPrimary,
                      ),
                    ),
                    Text(
                      'NT\$${result.refundAmount}',
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.brandPrimaryStrong,
                      ),
                    ),
                  ],
                ),
              ),
              if (result.needRefundPage) ...[
                SizedBox(height: 10.h),
                Text(
                  'ATM 付款需買家填寫銀行資訊，已通知買家完成退款',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.warningText,
                  ),
                ),
              ],
            ],
            SizedBox(height: 22.h),
            SizedBox(
              width: double.infinity,
              height: HkControl.minTap,
              child: ElevatedButton(
                onPressed: () => Navigator.of(context).pop(),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primary,
                  elevation: 0,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(999.r),
                  ),
                ),
                child: HkButtonLabel(
                  '確定',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.bgCard,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
