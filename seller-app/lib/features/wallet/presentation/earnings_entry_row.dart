import 'package:flutter/material.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart';
import 'package:hawkr_api/money.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/time/taipei_display.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../data/earnings.dart';

/// 入帳紀錄的一列：種類與狀態的文字取自共用文案目錄（`payment.EarningsEntryType`、`payment.EarningsEntryStatus`），
/// 金額帶正負（收回是負數），時間是這一列的記錄時間。[showOrderNo] 在錢包的跨訂單列表顯示訂單編號，訂單詳情裡不顯示。
class EarningsEntryRow extends StatelessWidget {
  const EarningsEntryRow({
    super.key,
    required this.entry,
    this.showOrderNo = false,
  });

  final EarningsEntry entry;
  final bool showOrderNo;

  @override
  Widget build(BuildContext context) {
    final l = context.uiLocale;
    final type = entry.type.uiText(l) ?? commonUnknown(l);
    final status = entry.status.uiText(l) ?? commonUnknown(l);
    final tone = earningsStatusTone(entry.status) ?? HkStatusTone.neutral;
    final meta = [
      if (showOrderNo && entry.orderNo.isNotEmpty) entry.orderNo,
      if (entry.hasCreateTime())
        context.taipeiDisplay.timestamp(entry.createTime),
    ].join('・');
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Flexible(child: Text(type, style: AppTypography.bodyBold)),
                  const SizedBox(width: HkSpace.sm),
                  HkStatusBadge(label: status, tone: tone),
                ],
              ),
              if (meta.isNotEmpty) ...[
                const SizedBox(height: HkSpace.xs),
                Text(
                  meta,
                  style: AppTypography.caption.copyWith(
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ],
          ),
        ),
        const SizedBox(width: HkSpace.sm),
        Text(formatMoney(entry.amount.toInt()), style: AppTypography.bodyBold),
      ],
    );
  }
}
