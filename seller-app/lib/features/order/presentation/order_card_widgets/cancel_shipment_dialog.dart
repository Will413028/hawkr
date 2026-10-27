import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// Figma: 取消出貨原因 dialog (270w, rounded-15, radio buttons + textarea)
class CancelShipmentDialog extends StatefulWidget {
  const CancelShipmentDialog({
    super.key,
    required this.orderNo,
    required this.onConfirm,
  });

  final String orderNo;
  final ValueChanged<String> onConfirm;

  @override
  State<CancelShipmentDialog> createState() => _CancelShipmentDialogState();
}

class _CancelShipmentDialogState extends State<CancelShipmentDialog> {
  static const _reasons = ['天災因素', '商品缺貨', '品質問題', '其他'];
  static const _reasonCodes = ['9201', '9202', '9203', '9299'];

  int? _selectedIndex;
  final _otherCtrl = TextEditingController();
  bool _submitting = false;

  String get _reason =>
      _selectedIndex == null ? '' : _reasonCodes[_selectedIndex!];

  bool get _canSubmit => _selectedIndex != null;

  @override
  void dispose() {
    _otherCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Material(
        color: Colors.transparent,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Container(
              width: 270.w,
              padding: EdgeInsets.all(16.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16.r),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: double.infinity,
                    child: Text(
                      '請勾選取消出貨的原因',
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        fontSize: 18.sp,
                        fontWeight: FontWeight.w700,
                        height: 1.23,
                        color: AppColors.textPrimary,
                      ),
                    ),
                  ),
                  SizedBox(height: 12.h),
                  ...List.generate(_reasons.length, (i) {
                    final selected = _selectedIndex == i;
                    return GestureDetector(
                      onTap: () => setState(() => _selectedIndex = i),
                      behavior: HitTestBehavior.opaque,
                      child: Padding(
                        padding: EdgeInsets.symmetric(vertical: 8.h),
                        child: Row(
                          children: [
                            Container(
                              width: 16.w,
                              height: 16.w,
                              decoration: BoxDecoration(
                                shape: BoxShape.circle,
                                color: selected
                                    ? AppColors.primary
                                    : AppColors.bgCard,
                                border: Border.all(
                                  color: selected
                                      ? AppColors.primary
                                      : const Color(0x33000000),
                                ),
                                boxShadow: const [
                                  BoxShadow(
                                    color: AppColors.cardShadow,
                                    blurRadius: 2,
                                    offset: Offset(0, 1),
                                  ),
                                ],
                              ),
                              child: selected
                                  ? Icon(
                                      AppIcons.check,
                                      size: 12.sp,
                                      color: AppIconColors.onBrand,
                                    )
                                  : null,
                            ),
                            SizedBox(width: 10.w),
                            Text(
                              _reasons[i],
                              style: TextStyle(
                                fontSize: 16.sp,
                                color: AppColors.textEmphasis,
                              ),
                            ),
                          ],
                        ),
                      ),
                    );
                  }),
                  SizedBox(height: 4.h),
                  Container(
                    height: 60.h,
                    width: double.infinity,
                    padding: EdgeInsets.symmetric(
                      horizontal: 8.w,
                      vertical: 8.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.gray100,
                      borderRadius: BorderRadius.circular(12.r),
                    ),
                    child: TextField(
                      controller: _otherCtrl,
                      maxLines: 3,
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textPrimary,
                      ),
                      decoration: InputDecoration(
                        hintText: '填寫原因',
                        hintStyle: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.textPrimary.withValues(alpha: 0.5),
                        ),
                        border: InputBorder.none,
                        isCollapsed: true,
                      ),
                      onChanged: (_) => setState(() {}),
                    ),
                  ),
                  SizedBox(height: 24.h),
                  GestureDetector(
                    onTap: _canSubmit && !_submitting
                        ? () {
                            setState(() => _submitting = true);
                            widget.onConfirm(_reason);
                          }
                        : null,
                    child: Opacity(
                      opacity: _canSubmit ? 1.0 : 0.5,
                      child: Container(
                        width: double.infinity,
                        padding: EdgeInsets.symmetric(vertical: 8.h),
                        decoration: BoxDecoration(
                          color: AppColors.primary,
                          borderRadius: BorderRadius.circular(999),
                          border: Border.all(
                            color: AppColors.primary,
                            width: 2,
                          ),
                        ),
                        child: _submitting
                            ? Center(
                                child: SizedBox(
                                  width: 20.w,
                                  height: 20.w,
                                  child: const CircularProgressIndicator(
                                    strokeWidth: 2,
                                    color: AppColors.bgCard,
                                  ),
                                ),
                              )
                            : Text(
                                '取消出貨',
                                textAlign: TextAlign.center,
                                style: TextStyle(
                                  fontSize: 16.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.bgCard,
                                ),
                              ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            Positioned(
              top: -12.h,
              right: -12.w,
              child: GestureDetector(
                onTap: () => Navigator.of(context).pop(),
                child: Container(
                  width: 28.w,
                  height: 28.w,
                  decoration: const BoxDecoration(
                    color: AppColors.bgCard,
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    AppIcons.close,
                    size: 18.sp,
                    color: AppIconColors.defaultColor,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
