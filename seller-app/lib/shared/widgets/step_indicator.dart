import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../core/codemeta/shipping_presentation.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import 'hk_icon.dart';

/// 步驟定義
class StepDef {
  const StepDef({required this.label, required this.icon});
  final String label;
  final HkIconName icon;
}

/// 通用水平 step indicator（售後流程 / 物流進度 / 退貨流程共用）
///
/// 步驟容器 60w、圓圈 32×32 居中、icon 16×16、label 12sp、line 3h
/// LayoutBuilder 依父寬度自動計算線段長度。
///
/// ```dart
/// // 售後流程（4 步）
/// StepIndicator(
///   steps: StepIndicator.afterSaleSteps,
///   currentStep: StepIndicator.afterSaleStepFromStatus(case.status),
/// )
///
/// // 退貨物流進度（4 步，seller）
/// StepIndicator(
///   steps: StepIndicator.returnShippingSteps(
///     shipping: ShippingPresentation.cvsStorePickup,
///   ),
///   currentStep: StepIndicator.returnShippingStepFromStatus(case.status),
/// )
/// ```
class StepIndicator extends StatelessWidget {
  const StepIndicator({
    super.key,
    required this.steps,
    required this.currentStep,
    this.activeColor,
  });

  final List<StepDef> steps;
  final int currentStep;

  /// 已完成與目前步驟的圓圈與連線顏色；沒給時用 [AppColors.warning]。
  final Color? activeColor;

  static const double _circleSize = 32;
  static const double _iconSize = 16;
  static const double _stepW = 60;
  static const double _lineH = 3;

  // ═══════════════════════════════════════════════════════════════
  // 預設步驟組
  // ═══════════════════════════════════════════════════════════════

  /// 售後流程 4 步
  static const afterSaleSteps = [
    StepDef(label: '售後申請', icon: HkIconName.document),
    StepDef(label: '審核中', icon: HkIconName.clock),
    StepDef(label: '審核結果', icon: HkIconName.info),
    StepDef(label: '完成', icon: HkIconName.check),
  ];

  /// 賣家售後流程 4 步（退貨退款處理視角）
  static const sellerAfterSaleSteps = [
    StepDef(label: '收到申請', icon: HkIconName.document),
    StepDef(label: '審核中', icon: HkIconName.clock),
    StepDef(label: '回覆結果', icon: HkIconName.info),
    StepDef(label: '結案歸檔', icon: HkIconName.check),
  ];

  /// 物流進度 4 步（賣家視角：已下單 / 待出貨 / 配送中 / 已送達）
  static const logisticsSteps = [
    StepDef(label: '已下單', icon: HkIconName.check),
    StepDef(label: '待出貨', icon: HkIconName.notification),
    StepDef(label: '配送中', icon: HkIconName.truck),
    StepDef(label: '已送達', icon: HkIconName.location),
  ];

  /// 物流進度 4 步（已取消變體：第 2 步改為「已取消」叉號）
  static const cancelledLogisticsSteps = [
    StepDef(label: '已下單', icon: HkIconName.check),
    StepDef(label: '已取消', icon: HkIconName.close),
    StepDef(label: '配送中', icon: HkIconName.truck),
    StepDef(label: '已送達', icon: HkIconName.location),
  ];

  /// 退貨物流進度 4 步（賣家端售後頁面）。
  ///
  /// 🔴 參數是 [ShippingPresentation] 不是 `bool`：`bool is711` 在型別層就把
  /// 五元的 `ShippingMethod` 壓成二元，呼叫端只能在「門市收件」與「到府取件」
  /// 之間二選一，未知 / 本版未支援的退貨方式必然被標成其中一家。
  static List<StepDef> returnShippingSteps({
    required ShippingPresentation shipping,
  }) => [
    StepDef(
      label: switch (shipping) {
        ShippingPresentation.cvsStorePickup => '門市收件',
        ShippingPresentation.homeDelivery => '到府取件',
        ShippingPresentation.unsupported => '已寄出',
      },
      icon: HkIconName.check,
    ),
    const StepDef(label: '配送中', icon: HkIconName.truck),
    const StepDef(label: '已送達', icon: HkIconName.location),
    const StepDef(label: '退款進度', icon: HkIconName.returnOrder),
  ];

  /// 賣家退貨物流進度 4 步
  static const sellerReturnShippingSteps = [
    StepDef(label: '取貨中', icon: HkIconName.check),
    StepDef(label: '配送中', icon: HkIconName.notification),
    StepDef(label: '已送達', icon: HkIconName.truck),
    StepDef(label: '驗貨完成', icon: HkIconName.location),
  ];

  // ═══════════════════════════════════════════════════════════════
  // status → step 轉換
  // ═══════════════════════════════════════════════════════════════

  static int afterSaleStepFromStatus(String status) => switch (status) {
    '0' => 1,
    '20' || '25' => 2,
    '30' || '40' => 2,
    '50' => 3,
    '60' => 2,
    _ => 0,
  };

  static int sellerAfterSaleStepFromStatus(String status) => switch (status) {
    '0' => 1,
    '20' || '25' || '30' || '40' => 2,
    '50' => 3,
    '60' || '70' || '71' => 2,
    _ => 0,
  };

  static int logisticsStepFromStatus(String status) => switch (status) {
    '10' => 1,
    '20' => 2,
    '30' || '32' => 2,
    '40' => 3,
    '50' || '60' || '70' || '80' => 3,
    _ => 0,
  };

  /// 退貨物流 status → step
  /// 20 待收件 / 25 運送中 / 30 已收退貨 / 40 退款中 / 50 已退款
  static int returnShippingStepFromStatus(String status) => switch (status) {
    '20' => 0,
    '25' => 1,
    '30' => 2,
    '40' => 3,
    '50' => 4,
    _ => 0,
  };

  // ═══════════════════════════════════════════════════════════════
  // Build
  // ═══════════════════════════════════════════════════════════════

  @override
  Widget build(BuildContext context) {
    const double labelSize = 12;
    final stepCount = steps.length;
    final lineCount = stepCount - 1;

    return LayoutBuilder(
      builder: (context, constraints) {
        final totalStepW = _stepW.w * stepCount;
        final remaining = constraints.maxWidth - totalStepW;
        final lineWidth = lineCount > 0
            ? (remaining / lineCount).clamp(0.0, double.infinity)
            : 0.0;

        return Row(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (int i = 0; i < stepCount; i++) ...[
              _buildStep(i, labelSize),
              if (i < lineCount) _buildLine(i, lineWidth),
            ],
          ],
        );
      },
    );
  }

  Color get _activeColor => activeColor ?? AppColors.warning;

  Widget _buildStep(int i, double labelSize) {
    final isDone = i < currentStep;
    final isCurrent = i == currentStep;
    final isActive = isDone || isCurrent;

    return SizedBox(
      width: _stepW.w,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: _circleSize.w,
            height: _circleSize.w,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: isActive ? _activeColor : AppColors.gray1_30,
              shape: BoxShape.circle,
            ),
            child: HkIcon(
              steps[i].icon,
              size: _iconSize.w,
              color: AppIconColors.onBrand,
            ),
          ),
          SizedBox(height: 4.h),
          Text(
            steps[i].label,
            style: TextStyle(
              fontFamily: AppTypography.family,
              fontSize: labelSize.sp,
              color: isActive
                  ? AppTypography.colorPrimary
                  : AppTypography.colorDisabled,
            ),
            textAlign: TextAlign.center,
            maxLines: 1,
          ),
        ],
      ),
    );
  }

  Widget _buildLine(int i, double lineWidth) {
    final active = i < currentStep;
    return SizedBox(
      width: lineWidth,
      child: Padding(
        padding: EdgeInsets.only(top: ((_circleSize - _lineH) / 2).w),
        child: SizedBox(
          height: _lineH.h,
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: active ? _activeColor : AppColors.gray1_30,
              borderRadius: BorderRadius.circular(HkRadius.pill),
            ),
          ),
        ),
      ),
    );
  }
}
