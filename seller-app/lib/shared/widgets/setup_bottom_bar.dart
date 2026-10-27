import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 商品上架流程共用底部欄
///
/// 用法：
/// ```dart
/// SetupBottomBar(
///   onBack: () => context.pop(),
///   onNext: isValid ? onSubmit : null,
///   nextLabel: '完成且預覽',  // 預設 '下一步'
/// )
/// ```
class SetupBottomBar extends StatelessWidget {
  const SetupBottomBar({
    super.key,
    this.onBack,
    this.onNext,
    this.backLabel = '上一步',
    this.nextLabel = '下一步',
  });

  /// null = 不顯示上一步
  final VoidCallback? onBack;

  /// null = disabled 狀態（灰色按鈕）
  final VoidCallback? onNext;

  final String backLabel;
  final String nextLabel;

  @override
  Widget build(BuildContext context) {
    final bottomPadding = MediaQuery.of(context).padding.bottom;
    final enabled = onNext != null;

    return Container(
      width: double.infinity,
      padding: EdgeInsets.fromLTRB(
        16.w,
        12.h,
        16.w,
        bottomPadding > 0 ? bottomPadding : 16,
      ),
      decoration: const BoxDecoration(
        color: AppColors.bgCard,
        border: Border(
          top: BorderSide(color: AppColors.dividerLight, width: 0.5),
        ),
      ),
      child: onBack != null
          ? Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                // 上一步
                GestureDetector(
                  onTap: onBack,
                  child: Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 24.w,
                      vertical: 12.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.bgCard,
                      borderRadius: BorderRadius.circular(999),
                      border: Border.all(color: AppColors.primary, width: 1.5),
                    ),
                    child: Text(
                      backLabel,
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.primary,
                      ),
                    ),
                  ),
                ),
                // 下一步 / 完成
                GestureDetector(
                  onTap: onNext,
                  child: Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 24.w,
                      vertical: 12.h,
                    ),
                    decoration: BoxDecoration(
                      color: enabled ? AppColors.primary : AppColors.gray300,
                      borderRadius: BorderRadius.circular(999),
                    ),
                    child: Text(
                      nextLabel,
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.bgCard,
                      ),
                    ),
                  ),
                ),
              ],
            )
          : SizedBox(
              width: double.infinity,
              child: GestureDetector(
                onTap: onNext,
                child: Container(
                  alignment: Alignment.center,
                  padding: EdgeInsets.symmetric(vertical: 12.h),
                  decoration: BoxDecoration(
                    color: enabled ? AppColors.primary : AppColors.gray300,
                    borderRadius: BorderRadius.circular(999),
                  ),
                  child: Text(
                    nextLabel,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.bgCard,
                    ),
                  ),
                ),
              ),
            ),
    );
  }
}
