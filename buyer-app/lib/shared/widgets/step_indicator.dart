import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';

/// 步驟定義
class StepDef {
  const StepDef({required this.label, required this.icon});
  final String label;
  final IconData icon;
}

/// 通用水平 step indicator（售後流程 / 物流進度共用）
///
/// Figma 規格：圓圈 32×32 居中、icon 16×16；各步驟依父寬平均分配，
/// 窄版與大字級允許標籤自然換成兩行，連接線維持在圓圈之間。
///
/// ```dart
/// // 售後流程（steps 自動依 status 切換「完成」/「拒絕處理」）
/// StepIndicator(
///   steps: StepIndicator.afterSaleStepsFor(case.status),
///   currentStep: StepIndicator.afterSaleStepFromStatus(case.status),
/// )
///
/// // 物流進度（卡片展開）
/// StepIndicator(
///   steps: StepIndicator.logisticsSteps,
///   currentStep: StepIndicator.logisticsStepFromStatus(order.status),
///   compact: true,
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
  final Color? activeColor;

  // ── Figma 常量 ──
  static const double _circleSize = 32;
  static const double _iconSize = 16;
  static const double _lineH = 3;
  static const Color _inactiveColor = AppIconColors.disabled;

  // ═══════════════════════════════════════════════════════════════
  // 預設步驟組
  // ═══════════════════════════════════════════════════════════════

  /// 售後流程 4 步（Figma 10108:94700）— 正常完成分支
  static const afterSaleSteps = [
    StepDef(label: '售後申請', icon: AppIcons.assignmentOutlined),
    StepDef(label: '審核中', icon: AppIcons.hourglassTopOutlined),
    StepDef(label: '審核結果', icon: AppIcons.factCheckOutlined),
    StepDef(label: '完成', icon: AppIcons.taskAltOutlined),
  ];

  /// 售後流程 4 步 — 拒絕分支（最後一步改為「拒絕處理」，改用 block icon）
  static const afterSaleStepsRejected = [
    StepDef(label: '售後申請', icon: AppIcons.assignmentOutlined),
    StepDef(label: '審核中', icon: AppIcons.hourglassTopOutlined),
    StepDef(label: '審核結果', icon: AppIcons.factCheckOutlined),
    StepDef(label: '拒絕處理', icon: AppIcons.blockOutlined),
  ];

  /// 依後端 status 取得對應的售後步驟組（完成 vs 拒絕互斥）。
  ///
  /// - REJECTED('60') → 拒絕分支，最後一步顯示「拒絕處理」
  /// - 其他狀態 → 正常分支，最後一步顯示「完成」
  ///
  /// 配合 [afterSaleStepFromStatus] 使用：
  /// ```dart
  /// StepIndicator(
  ///   steps: StepIndicator.afterSaleStepsFor(case.status),
  ///   currentStep: StepIndicator.afterSaleStepFromStatus(case.status),
  /// )
  /// ```
  static List<StepDef> afterSaleStepsFor(String status) {
    return status == '60' ? afterSaleStepsRejected : afterSaleSteps;
  }

  /// 物流進度 4 步（Figma 10092:104047）
  static const logisticsSteps = [
    StepDef(label: '已下單', icon: AppIcons.check),
    StepDef(label: '準備中', icon: AppIcons.notificationsOutlined),
    StepDef(label: '配送中', icon: AppIcons.localShippingOutlined),
    StepDef(label: '已送達', icon: AppIcons.location),
  ];

  /// 已取消訂單 4 步（Figma 10258:117571 / 117972 / 118592 / 118908 / 119204）
  /// 第 2 步把「準備中」替換成「已取消」（X icon），3-4 步保留但呈灰色非啟用狀態。
  static const cancelledSteps = [
    StepDef(label: '已下單', icon: AppIcons.check),
    StepDef(label: '已取消', icon: AppIcons.close),
    StepDef(label: '配送中', icon: AppIcons.localShippingOutlined),
    StepDef(label: '已送達', icon: AppIcons.location),
  ];

  // ═══════════════════════════════════════════════════════════════
  // status → step 轉換
  // ═══════════════════════════════════════════════════════════════

  /// 售後狀態碼 → 進度步驟 index（0-3）
  ///
  /// 對齊後端 AfterSaleCaseStatus 9 種狀態：
  /// - step 0「售後申請」— 剛提交（短暫，通常直接進 step 1）
  /// - step 1「審核中」    — PENDING(0)：等賣家回覆
  /// - step 2「審核結果」  — RETURN_*(20,25,30) 退貨流程 / REFUND_PROCESSING(40) 退款中
  /// - step 3「完成/拒絕處理」— CLOSED(50) / CANCELLED(70) → 完成
  ///                            / REJECTED(60) → 拒絕處理（搭配 [afterSaleStepsFor] 切換 label）
  static int afterSaleStepFromStatus(String status) => switch (status) {
    '0' => 1, // PENDING → 審核中
    '20' || '25' || '30' => 2, // RETURN_PENDING / SHIPPING / RECEIVED → 審核結果
    '40' => 2, // REFUND_PROCESSING → 審核結果
    '50' ||
    '70' ||
    '71' => 3, // CLOSED / CANCELLED / CANCELLED_BY_CONFIRM_RECEIPT → 完成
    '60' => 3, // REJECTED → 拒絕處理（最後一步，搭配 rejected 步驟組）
    _ => 0,
  };

  static int logisticsStepFromStatus(String status) => switch (status) {
    '10' => 1,
    '20' => 2,
    '30' || '32' => 2,
    '40' => 3,
    // 80 已取消改用 cancelledSteps 顯示，這裡的值僅在誤傳時 fallback。
    '50' || '60' || '70' || '80' => 3,
    _ => 0,
  };

  /// 已取消訂單 step：固定停在「已取消」（index 1）。
  static int cancelledStepIndex() => 1;

  // ═══════════════════════════════════════════════════════════════
  // Build
  // ═══════════════════════════════════════════════════════════════

  @override
  Widget build(BuildContext context) {
    const double labelSize = 12; // 統一 12sp（卡片展開 + 詳情頁）
    final stepCount = steps.length;
    final lineCount = stepCount - 1;
    if (stepCount == 0) return const SizedBox.shrink();

    return LayoutBuilder(
      builder: (context, constraints) {
        final slotWidth = constraints.maxWidth / stepCount;

        return Stack(
          clipBehavior: Clip.none,
          children: [
            for (int i = 0; i < lineCount; i++)
              Positioned(
                top: (_circleSize - _lineH) / 2,
                left: slotWidth * (i + 0.5) + _circleSize / 2,
                width: (slotWidth - _circleSize).clamp(0, double.infinity),
                child: _buildLine(i),
              ),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (int i = 0; i < stepCount; i++)
                  Expanded(child: _buildStep(i, labelSize)),
              ],
            ),
          ],
        );
      },
    );
  }

  Widget _buildStep(int i, double labelSize) {
    final isDone = i < currentStep;
    final isCurrent = i == currentStep;
    final isActive = isDone || isCurrent;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: _circleSize,
          height: _circleSize,
          child: DecoratedBox(
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: isActive
                  ? (activeColor ?? AppColors.warning)
                  : _inactiveColor,
            ),
            child: Icon(
              steps[i].icon,
              size: _iconSize,
              color: AppIconColors.onBrand,
            ),
          ),
        ),
        const SizedBox(height: 4),
        Text(
          steps[i].label,
          style: TextStyle(
            fontFamily: AppTypography.family,
            fontSize: labelSize,
            height: 1.15,
            color: isActive
                ? AppTypography.colorPrimary
                : AppTypography.colorDisabled,
          ),
          textAlign: TextAlign.center,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
        ),
      ],
    );
  }

  Widget _buildLine(int i) {
    final active = i < currentStep;
    return SizedBox(
      height: _lineH,
      child: DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(_lineH / 2),
          color: active ? (activeColor ?? AppColors.warning) : _inactiveColor,
        ),
      ),
    );
  }
}
