import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/widgets/step_indicator.dart';
import '../../data/order_model.dart';
import '../../order_color_tokens.dart';
import '../receipt_photo_controller.dart';
import '../order_status_presentation.dart';
import 'status_info_box.dart';

/// 訂單卡片展開區域 — 物流進度（4 步驟 StepIndicator + StatusInfoBox）
/// 顯示物流步驟指示器與當前狀態資訊框，已送達/已完成時可顯示收據照片
class OrderExpandedSection extends ConsumerWidget {
  const OrderExpandedSection({super.key, required this.order});
  final Order order;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final delivery = DeliveryStatusPresentation.fromOrder(order);
    final receiptState = ref.watch(
      receiptPhotoControllerProvider(
        orderSn: order.orderSn,
        backendUrls: order.receiptPhotoUrls,
      ),
    );
    final receiptPhotos = receiptState.value?.photos ?? const <String>[];
    final receiptPhotoUrl = receiptPhotos.isEmpty ? null : receiptPhotos.first;
    return Column(
      children: [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg),
          child: Divider(
            color: AppTypography.colorPrimary.withValues(alpha: 0.2),
            height: 1,
          ),
        ),
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.md,
            AppSpacing.lg,
            12.h,
          ),
          child: Column(
            children: [
              Row(
                children: [
                  SizedBox(
                    width: 20.w,
                    height: 20.w,
                    child: Icon(AppIcons.localShippingOutlined, size: 20.w),
                  ),
                  SizedBox(width: AppSpacing.xs),
                  Text('物流進度', style: AppTypography.bodyBold),
                ],
              ),
              SizedBox(height: AppSpacing.sm),
              // status=80 改用 cancelledSteps（已下單 ✓ → 已取消 ✗ → 配送中（灰）→ 已送達（灰））
              StepIndicator(
                steps: order.status == '80'
                    ? StepIndicator.cancelledSteps
                    : StepIndicator.logisticsSteps,
                currentStep: order.status == '80'
                    ? StepIndicator.cancelledStepIndex()
                    : delivery.progressStep,
                activeColor: order.status == '80'
                    ? OrderColorTokens.neutral
                    : OrderColorTokens.progress,
              ),
              SizedBox(height: AppSpacing.sm),
              StatusInfoBox(order: order, receiptPhotoUrl: receiptPhotoUrl),
            ],
          ),
        ),
      ],
    );
  }
}
