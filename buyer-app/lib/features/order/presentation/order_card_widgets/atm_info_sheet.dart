import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../data/order_model.dart';

/// ATM 匯款資訊 BottomSheet — 顯示銀行代碼、虛擬帳號、匯款期限、應付金額
/// 由待付款 ATM 訂單的「匯款資訊」按鈕觸發
void showAtmInfoSheet(
  BuildContext context, {
  required Order order,
  Order? detailOrder,
}) {
  showModalBottomSheet(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: false,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (sheetContext) {
      final maxHeight = MediaQuery.sizeOf(sheetContext).height * 0.9;
      return SafeArea(
        top: false,
        child: Align(
          alignment: Alignment.bottomCenter,
          heightFactor: 1,
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: 560, maxHeight: maxHeight),
            child: SingleChildScrollView(
              key: const ValueKey('atm-info-scroll-view'),
              padding: EdgeInsets.fromLTRB(
                AppSpacing.xl,
                20.h,
                AppSpacing.xl,
                AppSpacing.xxl,
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Center(
                    child: Container(
                      width: 40.w,
                      height: 4.h,
                      decoration: BoxDecoration(
                        color: AppColors.dragHandle,
                        borderRadius: BorderRadius.circular(2.r),
                      ),
                    ),
                  ),
                  SizedBox(height: AppSpacing.lg),
                  Text(
                    'ATM 匯款資訊',
                    style: AppTypography.sectionTitle.copyWith(fontSize: 18),
                  ),
                  SizedBox(height: AppSpacing.lg),
                  _AtmInfoRow(
                    label: '銀行代碼',
                    value:
                        (detailOrder?.atmBankCode ?? order.atmBankCode).isEmpty
                        ? '---'
                        : (detailOrder?.atmBankCode ?? order.atmBankCode),
                  ),
                  SizedBox(height: AppSpacing.md),
                  _AtmInfoRow(
                    label: '虛擬帳號',
                    value:
                        (detailOrder?.atmVirtualAccount ??
                                order.atmVirtualAccount)
                            .isEmpty
                        ? '---'
                        : (detailOrder?.atmVirtualAccount ??
                              order.atmVirtualAccount),
                  ),
                  SizedBox(height: AppSpacing.md),
                  _AtmInfoRow(
                    label: '匯款期限',
                    value:
                        (detailOrder?.atmExpireDate ?? order.atmExpireDate)
                            .isEmpty
                        ? '---'
                        : (detailOrder?.atmExpireDate ?? order.atmExpireDate),
                  ),
                  SizedBox(height: AppSpacing.md),
                  _AtmInfoRow(
                    label: '應付金額',
                    value:
                        'NT\$${detailOrder?.totalActualAmount ?? order.totalActualAmount}',
                  ),
                  SizedBox(height: 20.h),
                  Text(
                    '請於期限內完成匯款，逾時訂單將自動取消。',
                    style: AppTypography.meta.copyWith(
                      fontSize: 14,
                      color: AppColors.primary,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
    },
  );
}

class _AtmInfoRow extends StatelessWidget {
  const _AtmInfoRow({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final labelWidget = Text(
      label,
      style: AppTypography.body.copyWith(color: AppTypography.colorMuted),
    );
    final valueWidget = SelectableText(
      value,
      key: ValueKey('atm-info-value-$label'),
      style: AppTypography.bodyBold.copyWith(fontWeight: FontWeight.w700),
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < 360) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              labelWidget,
              SizedBox(height: AppSpacing.xs),
              valueWidget,
            ],
          );
        }

        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            labelWidget,
            SizedBox(width: AppSpacing.lg),
            Expanded(
              child: Align(
                alignment: Alignment.centerRight,
                child: valueWidget,
              ),
            ),
          ],
        );
      },
    );
  }
}
