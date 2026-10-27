import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../config/image_config.dart';
import '../../../../core/codemeta/code_meta.dart';
import '../../../../core/codemeta/shipping_presentation.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/widgets/optimized_image.dart';
import '../../../../shared/widgets/step_indicator.dart';
import '../../../media/data/media_repository.dart';
import '../../data/order_model.dart';
import '../order_status_presentation.dart';
import 'pill_button.dart';

// ── 上傳出貨照片，回傳 stagedToken（失敗或未上傳回傳 null） ─────────────────
Future<String?> uploadShipmentPhoto(
  WidgetRef ref,
  BuildContext context,
  File photo,
  String orderNo,
) async {
  FloatingToast.show(context, '正在上傳照片...', icon: AppIcons.cloudUpload);
  final bytes = await photo.readAsBytes();
  if (!context.mounted) return null;
  final mediaResult = await ref
      .read(mediaRepositoryProvider)
      .uploadWithSession(
        bytes: bytes,
        filename:
            'shipment_${orderNo}_${DateTime.now().millisecondsSinceEpoch}.jpg',
      );
  if (!context.mounted) return null;
  final token = mediaResult.stagedToken;
  return (token != null && token.isNotEmpty) ? token : null;
}

// ── 計算 progressStage（0~3） ──────────────────────────────────────────────
int calcProgressStage(OrderSummary order) {
  return DeliveryStatusPresentation.fromOrder(order).progressStep;
}

/// 訂單卡片展開區域 — 物流進度（標題 + StepIndicator + 狀態資訊框）
///
/// 接收 [onCancelShipment] / [onDownloadPdf] 回調以保留業務操作入口。
class OrderExpandedSection extends StatelessWidget {
  const OrderExpandedSection({
    super.key,
    required this.order,
    required this.onCancelShipment,
    required this.onDownloadPdf,
  });

  final OrderSummary order;
  final VoidCallback onCancelShipment;
  final VoidCallback onDownloadPdf;

  @override
  Widget build(BuildContext context) {
    final isCancelled = (order.statusCode ?? 0) == 80;

    return Column(
      children: [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 20.w),
          child: Divider(
            color: AppTypography.colorPrimary.withValues(alpha: 0.2),
            height: 1,
          ),
        ),
        Padding(
          padding: EdgeInsets.fromLTRB(20.w, 16.h, 20.w, 12.h),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  SizedBox(
                    width: 24.w,
                    height: 24.w,
                    child: FittedBox(
                      child: Icon(
                        AppIcons.localShippingOutlined,
                        color: AppIconColors.defaultColor,
                      ),
                    ),
                  ),
                  SizedBox(width: 8.w),
                  Text('物流進度', style: AppTypography.sectionTitle),
                ],
              ),
              SizedBox(height: 8.h),
              StepIndicator(
                steps: isCancelled
                    ? StepIndicator.cancelledLogisticsSteps
                    : StepIndicator.logisticsSteps,
                currentStep: calcProgressStage(order),
              ),
              SizedBox(height: 8.h),
              _StatusInfoBox(
                order: order,
                onCancelShipment: onCancelShipment,
                onDownloadPdf: onDownloadPdf,
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// 物流狀態資訊框 — 標題 + 時間 + 分支內容（文案／PDF／取消出貨按鈕／照片縮圖）
class _StatusInfoBox extends StatelessWidget {
  const _StatusInfoBox({
    required this.order,
    required this.onCancelShipment,
    required this.onDownloadPdf,
  });

  final OrderSummary order;
  final VoidCallback onCancelShipment;
  final VoidCallback onDownloadPdf;

  bool _isLabelExpired(String expireAt) {
    if (expireAt.isEmpty) return false;
    final dt = DateTime.tryParse(expireAt);
    if (dt == null) return false;
    return DateTime.now().isAfter(dt);
  }

  String _formatDate(String raw) {
    if (raw.isEmpty) return '';
    DateTime? dt = DateTime.tryParse(raw);
    if (dt == null) {
      final m = RegExp(r'\d{10,13}').firstMatch(raw);
      if (m != null) {
        final n = int.tryParse(m.group(0)!);
        if (n != null) {
          dt = (m.group(0)!.length >= 13)
              ? DateTime.fromMillisecondsSinceEpoch(n)
              : DateTime.fromMillisecondsSinceEpoch(n * 1000);
        }
      }
    }
    if (dt == null) return raw;
    final local = dt.toLocal();
    String pad(int v) => v.toString().padLeft(2, '0');
    return '${local.year}/${pad(local.month)}/${pad(local.day)} ${pad(local.hour)}:${pad(local.minute)}';
  }

  bool _isShipmentUnprocessed(String ss) =>
      ss.isEmpty || ss == ShippingStatus.unprocessed.value;

  String _statusLabel(int code) {
    const map = {
      10: '已付款',
      20: '待出貨',
      30: '配送中',
      32: '待取貨',
      35: '配送異常',
      40: '已送達',
      50: '已完成',
      60: '退款中',
      70: '已退款',
      80: '已取消',
    };
    return map[code] ?? '未知狀態';
  }

  /// 對應後端 OrderCancelReason enum（91xx 系統 / 92xx 賣家）
  String _cancelReasonLabel() {
    switch (order.cancelReason) {
      case '9101':
        return '付款逾時自動取消';
      case '9102':
        return '出貨逾時自動取消';
      case '9201':
        return '天災因素無法配送';
      case '9202':
        return '商品缺貨';
      case '9203':
        return '商品品質問題';
      case '9204':
        return '建議買家拆單出貨';
      case '9299':
        final remark = order.cancelRemark.trim();
        return remark.isNotEmpty ? '其他原因 · $remark' : '其他原因';
      default:
        return '訂單已取消';
    }
  }

  String _cancelDescHint() {
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
        return '已通知顧客，款項已退款';
      default:
        return order.statusSummary.isNotEmpty ? order.statusSummary : '訂單已取消';
    }
  }

  String _statusTitle(int statusCode, bool isCancelled) {
    if (isCancelled) return _cancelReasonLabel();
    final isTcatCalledTaqbin =
        order.shipmentStatus == ShippingStatus.tcatCalled.value &&
        order.shippingMethod == ShippingMethod.taqbin.value;
    if (statusCode >= 30 || isTcatCalledTaqbin) return '商品配送中';
    if (order.odno.isNotEmpty) return '711取號成功';
    if (order.shippingStatus.isNotEmpty) return '商品準備中';
    return _statusLabel(statusCode);
  }

  @override
  Widget build(BuildContext context) {
    final statusCode = order.statusCode ?? 0;
    final isCancelled = statusCode == 80;
    final statusTitle = _statusTitle(statusCode, isCancelled);
    final statusTime = _formatDate(order.dateTime);

    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray1_30),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            alignment: WrapAlignment.spaceBetween,
            runSpacing: 4.h,
            children: [
              Text(
                statusTitle,
                style: AppTypography.body.copyWith(
                  color: AppTypography.colorEmphasis,
                ),
              ),
              Text(
                statusTime,
                style: AppTypography.meta.copyWith(color: AppColors.gray1_50),
              ),
            ],
          ),
          SizedBox(height: 6.h),
          if (isCancelled)
            Text(
              _cancelDescHint(),
              style: AppTypography.meta.copyWith(
                color: AppColors.textSecondary,
              ),
            )
          else
            ..._buildInfoContent(context),
        ],
      ),
    );
  }

  List<Widget> _buildInfoContent(BuildContext context) {
    final ss = order.shipmentStatus;
    // 🔴 依配送方式**明確分派**。原本是 `if (isTaqbin) 宅配段` ＋ `if (!isTaqbin) 超商段`，
    // 於是未知 / 本版未支援的配送方式會走進超商段，顯示「出貨便代碼」與「取消出貨」。
    final shipping = shippingPresentationOf(order.shippingMethod);
    final firstPhoto = order.shipmentPhotoUrls.isNotEmpty
        ? order.shipmentPhotoUrls.first
        : '';

    final infoStyle = AppTypography.meta.copyWith(
      color: AppColors.textSecondary,
    );

    // ── 宅配 ──
    if (shipping == ShippingPresentation.homeDelivery) {
      if (ss == ShippingStatus.tcatCalled.value && order.labelUrl.isNotEmpty) {
        return [
          Text(
            order.shipStatusDesc.isNotEmpty ? order.shipStatusDesc : '訂單正在配送中',
            style: infoStyle,
          ),
        ];
      }
      if (order.labelUrl.isNotEmpty) {
        final expireAt = order.labelExpireAt;
        final isExpired = _isLabelExpired(expireAt);
        return [
          Row(
            children: [
              Expanded(
                child: Text(
                  '下載pdf檔案期限：\n${expireAt.isNotEmpty ? _formatDate(expireAt) : ''}',
                  style: infoStyle,
                ),
              ),
              if (!isExpired)
                PillButton(
                  label: '下載 PDF',
                  color: AppColors.warning,
                  onTap: onDownloadPdf,
                ),
              if (firstPhoto.isNotEmpty) ...[
                SizedBox(width: 8.w),
                _photoThumb(firstPhoto),
              ],
            ],
          ),
        ];
      }
      if (_isShipmentUnprocessed(ss) && order.labelUrl.isEmpty) {
        return [
          Text('訂單處理中', style: infoStyle),
          SizedBox(height: 6.h),
          Align(
            alignment: Alignment.centerRight,
            child: PillButton(
              label: '取消出貨',
              color: AppColors.primary,
              outlined: true,
              onTap: onCancelShipment,
            ),
          ),
        ];
      }
    }

    // ── 超商 ──
    if (shipping == ShippingPresentation.cvsStorePickup) {
      if (order.odno.isNotEmpty && (order.statusCode ?? 0) >= 30) {
        return [
          Text(
            order.shipStatusDesc.isNotEmpty ? order.shipStatusDesc : '訂單正在配送中',
            style: infoStyle,
          ),
        ];
      }
      if (order.odno.isNotEmpty) {
        return [
          Row(
            children: [
              Expanded(
                child: Text(
                  '出貨便代碼：${order.odno}${order.validationNo}',
                  style: infoStyle,
                ),
              ),
              if (firstPhoto.isNotEmpty) ...[
                SizedBox(width: 8.w),
                _photoThumb(firstPhoto),
              ],
            ],
          ),
        ];
      }
      if (_isShipmentUnprocessed(ss) && order.odno.isEmpty) {
        return [
          Text('訂單處理中', style: infoStyle),
          SizedBox(height: 6.h),
          Align(
            alignment: Alignment.centerRight,
            child: PillButton(
              label: '取消出貨',
              color: AppColors.primary,
              outlined: true,
              onTap: onCancelShipment,
            ),
          ),
        ];
      }
    }

    // 兩段都不中（含未知 / 本版未支援的配送方式）→ provider-neutral 文案，
    // 不出現任何物流商專屬欄位或操作。
    return [Text('訂單處理中', style: infoStyle)];
  }

  Widget _photoThumb(String url) {
    return ClipRRect(
      borderRadius: BorderRadius.circular(8.r),
      child: OptimizedNetworkImage(
        imageUrl: url,
        scene: ImageScene.orderItem,
        width: 50.w,
        height: 50.w,
        fit: BoxFit.cover,
        errorWidget: SizedBox(width: 50.w, height: 50.w),
      ),
    );
  }
}
