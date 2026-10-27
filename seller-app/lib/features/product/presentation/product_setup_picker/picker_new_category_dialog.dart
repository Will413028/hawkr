import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 顯示「新增分類」Dialog（L2 層 — L1 下的二級分類）。
///
/// 純 UI 收集：使用者輸入名稱 → 呼叫 [onSubmit]。後端儲存與狀態更新由呼叫方處理。
void showPickerNewCategoryDialog({
  required BuildContext context,
  required void Function(String name) onSubmit,
}) {
  final controller = TextEditingController();

  showDialog(
    context: context,
    barrierDismissible: true,
    barrierColor: AppColors.textPrimary,
    builder: (ctx) => StatefulBuilder(
      builder: (dialogContext, setDialogState) {
        final hasText = controller.text.trim().isNotEmpty;

        return Dialog(
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12.r),
          ),
          insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
          child: Container(
            width: 280.w,
            constraints: const BoxConstraints(maxWidth: 280),
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(12.r),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Padding(
                  padding: EdgeInsets.fromLTRB(16.w, 20.h, 16.w, 12.h),
                  child: Text(
                    '請填寫分類',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      color: AppColors.textPrimary,
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                Padding(
                  padding: EdgeInsets.symmetric(horizontal: 16.w),
                  child: TextField(
                    controller: controller,
                    onChanged: (_) => setDialogState(() {}),
                    decoration: InputDecoration(
                      hintText: '請填寫分類',
                      hintStyle: TextStyle(
                        color: AppColors.textSecondary,
                        fontSize: 14.sp,
                      ),
                      contentPadding: EdgeInsets.symmetric(
                        horizontal: 12.w,
                        vertical: 12.h,
                      ),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8.r),
                        borderSide: const BorderSide(
                          color: AppColors.cardBorder,
                        ),
                      ),
                      enabledBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8.r),
                        borderSide: const BorderSide(
                          color: AppColors.cardBorder,
                        ),
                      ),
                      focusedBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8.r),
                        borderSide: BorderSide(
                          color: AppColors.primary,
                          width: 1.5.w,
                        ),
                      ),
                    ),
                  ),
                ),
                SizedBox(height: 16.h),
                Container(
                  width: double.infinity,
                  height: 1,
                  color: AppColors.textSecondary,
                ),
                SizedBox(
                  height: 44.h,
                  child: Row(
                    children: [
                      Expanded(
                        child: GestureDetector(
                          onTap: () => Navigator.pop(ctx),
                          behavior: HitTestBehavior.opaque,
                          child: Center(
                            child: Text(
                              '取消',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                          ),
                        ),
                      ),
                      Container(
                        width: 1,
                        height: 44.h,
                        color: AppColors.textSecondary,
                      ),
                      Expanded(
                        child: GestureDetector(
                          onTap: hasText
                              ? () {
                                  final name = controller.text.trim();
                                  Navigator.pop(ctx);
                                  onSubmit(name);
                                }
                              : null,
                          behavior: HitTestBehavior.opaque,
                          child: Center(
                            child: Text(
                              '確定',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: hasText
                                    ? AppColors.primary
                                    : AppColors.textSecondary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        );
      },
    ),
  );
}
