import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../config/image_config.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/utils/date_format_utils.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../data/order_model.dart';
import '../../order_color_tokens.dart';
import 'pill_button.dart';

/// 訂單狀態資訊框 — 展開區域底部的狀態卡片
/// 顯示當前狀態標題、時間、描述文案，已送達/已完成時顯示收據照片或「顧客已取貨」
/// 已取消訂單會根據 cancelReason 顯示 7 種取消原因標籤
class StatusInfoBox extends StatelessWidget {
  const StatusInfoBox({
    super.key,
    required this.order,
    this.onReview,
    this.receiptPhotoUrl,
  });
  final Order order;
  final VoidCallback? onReview;
  final String? receiptPhotoUrl;

  String get _cancelReasonLabel {
    switch (order.cancelReason) {
      case '9101':
        return '付款逾時自動取消';
      case '9102':
        return '出貨逾時自動取消';
      case '9201':
        return '賣家取消 · 天災因素';
      case '9202':
        return '賣家取消 · 備貨不足';
      case '9203':
        return '賣家取消 · 商品品質問題';
      case '9204':
        return '賣家取消 · 建議買家拆單出貨';
      case '9299':
        final remark = order.cancelRemark.trim();
        return remark.isNotEmpty ? '賣家取消 · $remark' : '賣家取消 · 其他原因';
      default:
        return '訂單已取消';
    }
  }

  String get _cancelDescHint {
    switch (order.cancelReason) {
      case '9101':
        return '訂單已超過付款期限';
      case '9102':
        return '訂單已取消，款項將自動返回';
      case '9201':
      case '9202':
      case '9203':
      case '9204':
      case '9299':
        final amount = order.totalActualAmount;
        return amount.isNotEmpty ? '已退款 NT\$ $amount' : '已退款';
      default:
        return '';
    }
  }

  String get _statusTitle {
    switch (order.status) {
      case '10':
        return '等待商家出貨';
      case '20':
        return '已出貨';
      case '30':
        return '配送中';
      case '32':
        return '待取貨';
      case '40':
        return '已送達';
      case '50':
        return '訂單完成';
      case '60':
        return '部分退款';
      case '70':
        return '全額退款';
      case '80':
        return _cancelReasonLabel;
      default:
        return '處理中';
    }
  }

  String get _statusDesc {
    if (order.status == '80') {
      final hint = _cancelDescHint;
      if (hint.isNotEmpty) return hint;
      if (order.statusSummary.isNotEmpty) return order.statusSummary;
      return '訂單已取消';
    }
    if (order.statusSummary.isNotEmpty) return order.statusSummary;
    switch (order.status) {
      case '0':
        final deadline = DateFormatUtils.formatDateTime(order.statusTime);
        if (deadline.isNotEmpty) {
          return '請於 $deadline 前完成付款，逾時訂單將自動取消。';
        }
        return '請儘速完成付款，逾時訂單將自動取消。';
      case '10':
        return '您的訂單已付款，等待商家出貨';
      case '20':
        return '您的包裹已出貨';
      case '30':
        return '您的包裹正在配送途中';
      case '32':
        return '包裹已到店，請儘速前往取貨';
      case '40':
        return '您的包裹已送達，請確認收貨';
      case '50':
        return '訂單已完成，感謝您的購買';
      case '60':
        return '訂單部分商品已退款';
      case '70':
        return '訂單已全額退款';
      default:
        return '';
    }
  }

  @override
  Widget build(BuildContext context) {
    final timeStr = DateFormatUtils.formatDateTime(
      order.statusTime.isNotEmpty ? order.statusTime : order.createTime,
    );
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12.h),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(
          color: AppTypography.colorPrimary.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            alignment: WrapAlignment.spaceBetween,
            runSpacing: AppSpacing.xs,
            children: [
              Text(
                _statusTitle,
                style: AppTypography.bodyBold.copyWith(
                  color: AppColors.textEmphasis,
                ),
              ),
              if (timeStr.isNotEmpty)
                Text(
                  timeStr,
                  style: AppTypography.meta.copyWith(
                    color: AppTypography.colorDisabled,
                  ),
                ),
            ],
          ),
          if (order.status == '40' || order.status == '50') ...[
            SizedBox(height: 6.h),
            if (receiptPhotoUrl != null && receiptPhotoUrl!.isNotEmpty)
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Padding(
                      padding: EdgeInsets.only(top: 4.h),
                      child: Text(
                        '已送出商品照片',
                        style: AppTypography.meta.copyWith(
                          color: AppTypography.colorPrimary.withValues(
                            alpha: 0.8,
                          ),
                        ),
                      ),
                    ),
                  ),
                  Container(
                    width: 50.w,
                    height: 50.w,
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(12.r),
                      border: Border.all(color: AppTypography.colorDisabled),
                    ),
                    clipBehavior: Clip.antiAlias,
                    child: OssImage(
                      imageSource: receiptPhotoUrl!,
                      scene: ImageScene.reviewPhoto,
                      width: 50.w,
                      height: 50.w,
                      fit: BoxFit.cover,
                      // URL 有可能是 stale staged previewUrl（後端 finalize 後失效）；
                      // 失敗時用素淨灰塊，不要顯示醒目的破圖 icon 讓 list view 看起來都壞了
                      errorWidget: ColoredBox(color: AppColors.gray100),
                    ),
                  ),
                ],
              )
            else
              Text(
                '顧客已取貨',
                style: AppTypography.meta.copyWith(
                  color: AppTypography.colorPrimary.withValues(alpha: 0.8),
                ),
              ),
            if (onReview != null) ...[
              SizedBox(height: AppSpacing.sm),
              Align(
                alignment: Alignment.centerRight,
                child: PillButton(
                  label: '給我好評',
                  color: OrderColorTokens.primary,
                  onTap: onReview,
                ),
              ),
            ],
          ] else if (_statusDesc.isNotEmpty) ...[
            SizedBox(height: 6.h),
            Text(
              _statusDesc,
              style: AppTypography.meta.copyWith(
                color: AppTypography.colorPrimary.withValues(alpha: 0.8),
              ),
            ),
          ],
        ],
      ),
    );
  }
}
