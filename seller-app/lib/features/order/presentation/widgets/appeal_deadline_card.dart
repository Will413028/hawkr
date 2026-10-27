import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../shared/widgets/hk_icon.dart';

/// 售後申訴時效比對卡。
///
/// 顯示「商品送達時間」vs「買家申訴時間」，並根據 [freshWindowHours]
/// 動態計算是否超過生鮮申訴期（預設 60 小時），用顏色與文案提醒賣家
/// 婉拒前衡量案件時效依據。
///
/// 若 [deliveredAt] 為 null（後端尚未提供），則以中性文案呈現。
class AppealDeadlineCard extends StatelessWidget {
  const AppealDeadlineCard({
    super.key,
    required this.createdAt,
    this.deliveredAt,
    this.freshWindowHours = 60,
  });

  /// 買家申請售後的時間（= AfterSaleCase.createdAt）
  final DateTime createdAt;

  /// 商品送達時間（= Order.deliveredAt）。null 表示尚未提供。
  final DateTime? deliveredAt;

  /// 生鮮申訴期限（小時），預設 60
  final int freshWindowHours;

  @override
  Widget build(BuildContext context) {
    final double? hoursDiff = deliveredAt == null
        ? null
        : createdAt.difference(deliveredAt!).inMinutes / 60.0;

    final Color accent;
    final String hint;
    if (hoursDiff == null) {
      accent = AppColors.gray1_50;
      hint = '未取得商品送達時間，請先確認是否在 $freshWindowHours 小時申訴期內';
    } else if (hoursDiff > freshWindowHours) {
      accent = AppColors.error;
      hint =
          '已超過生鮮 $freshWindowHours 小時申訴期（超過 ${(hoursDiff - freshWindowHours).toStringAsFixed(1)} 小時）';
    } else {
      accent = AppColors.warning;
      hint =
          '仍在申訴期內（已 ${hoursDiff.toStringAsFixed(1)} 小時／剩 ${(freshWindowHours - hoursDiff).toStringAsFixed(1)} 小時），婉拒需謹慎';
    }

    return Container(
      padding: EdgeInsets.all(12.w),
      decoration: BoxDecoration(
        color: accent.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: accent.withValues(alpha: 0.35)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              HkIcon(HkIconName.clock, size: 16.sp, color: accent),
              SizedBox(width: 6.w),
              Text(
                '時效與爭議比對',
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w700,
                  color: accent,
                ),
              ),
            ],
          ),
          SizedBox(height: 10.h),
          Row(
            children: [
              Expanded(
                child: _TimeBlock(
                  label: '商品送達時間',
                  value: deliveredAt != null ? _fmt(deliveredAt!) : '尚未提供',
                  valueColor: deliveredAt != null
                      ? AppColors.gray1
                      : AppColors.gray1_50,
                ),
              ),
              Expanded(
                child: _TimeBlock(
                  label: '買家申訴時間',
                  value: _fmt(createdAt),
                  valueColor: accent,
                ),
              ),
            ],
          ),
          SizedBox(height: 6.h),
          Text(
            hint,
            style: TextStyle(
              fontSize: 12.sp,
              color: accent,
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }

  String _fmt(DateTime dt) {
    final l = dt.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${l.year}/${two(l.month)}/${two(l.day)} ${two(l.hour)}:${two(l.minute)}';
  }
}

class _TimeBlock extends StatelessWidget {
  const _TimeBlock({
    required this.label,
    required this.value,
    required this.valueColor,
  });

  final String label;
  final String value;
  final Color valueColor;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: TextStyle(fontSize: 12.sp, color: AppColors.gray1_50),
        ),
        SizedBox(height: 2.h),
        Text(
          value,
          style: TextStyle(
            fontSize: 14.sp,
            color: valueColor,
            fontWeight: FontWeight.w700,
          ),
        ),
      ],
    );
  }
}
