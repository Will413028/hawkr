import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../shared/widgets/app_toggle_switch.dart';
import '../../../shared/widgets/dialog_close_button.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_icon.dart';

/// 呼叫黑貓取件 dialog
/// 包含：電話聯絡 toggle、需要推車 toggle、備註欄位
class CallTcatDialog extends StatefulWidget {
  const CallTcatDialog({super.key});

  @override
  State<CallTcatDialog> createState() => _CallTcatDialogState();
}

class _CallTcatDialogState extends State<CallTcatDialog> {
  bool _isContact = false;
  bool _isTrolley = false;
  final _memoCtrl = TextEditingController();
  bool _submitting = false;

  static const _green = AppColors.primary;
  static const _gray = AppColors.textPrimary;
  static const _lightGray = AppColors.gray100;

  @override
  void dispose() {
    _memoCtrl.dispose();
    super.dispose();
  }

  void _submit() {
    if (_submitting) return;
    setState(() => _submitting = true);
    Navigator.pop(
      context,
      CallTcatParams(
        isContact: _isContact ? 'Y' : 'N',
        isTrolley: _isTrolley ? 'Y' : 'N',
        memo: _memoCtrl.text.trim(),
      ),
    );
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
              width: 304.w,
              padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 20.h),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16.r),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // ── Header ──────────────────────────────────────────
                  Row(
                    children: [
                      Container(
                        width: 40.w,
                        height: 40.w,
                        decoration: BoxDecoration(
                          color: AppColors.bgCard,
                          borderRadius: BorderRadius.circular(12.r),
                          border: Border.all(
                            color: _gray.withValues(alpha: 0.25),
                          ),
                        ),
                        child: Center(
                          child: HkIcon(
                            HkIconName.truck,
                            size: 24.sp,
                            color: AppIconColors.defaultColor,
                          ),
                        ),
                      ),
                      SizedBox(width: 10.w),
                      Text(
                        '呼叫黑貓取件',
                        style: TextStyle(
                          fontSize: 18.sp,
                          fontWeight: FontWeight.w700,
                          color: _gray,
                        ),
                      ),
                    ],
                  ),
                  SizedBox(height: 16.h),

                  // ── Toggle 選項區塊 ──────────────────────────────────
                  Container(
                    decoration: BoxDecoration(
                      color: _lightGray,
                      borderRadius: BorderRadius.circular(12.r),
                    ),
                    child: Column(
                      children: [
                        _buildToggleRow(
                          icon: HkIconName.phone,
                          label: '電話聯絡',
                          value: _isContact,
                          onChanged: (v) => setState(() => _isContact = v),
                          showDivider: true,
                        ),
                        _buildToggleRow(
                          icon: HkIconName.cart,
                          label: '需要推車',
                          value: _isTrolley,
                          onChanged: (v) => setState(() => _isTrolley = v),
                          showDivider: false,
                        ),
                      ],
                    ),
                  ),
                  SizedBox(height: 14.h),

                  // ── 備註 label ───────────────────────────────────────
                  Row(
                    children: [
                      HkIcon(
                        HkIconName.edit,
                        size: 18.sp,
                        color: _gray.withValues(alpha: 0.6),
                      ),
                      SizedBox(width: 4.w),
                      Text(
                        '給宅配人員的備註',
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: _gray.withValues(alpha: 0.6),
                        ),
                      ),
                    ],
                  ),
                  SizedBox(height: 6.h),

                  // ── 備註 textarea ────────────────────────────────────
                  Container(
                    height: 80.h,
                    decoration: BoxDecoration(
                      color: _lightGray,
                      borderRadius: BorderRadius.circular(8.r),
                      border: Border.all(color: _gray.withValues(alpha: 0.15)),
                    ),
                    child: TextField(
                      controller: _memoCtrl,
                      maxLines: null,
                      expands: true,
                      textAlignVertical: TextAlignVertical.top,
                      style: TextStyle(fontSize: 14.sp, color: _gray),
                      decoration: InputDecoration(
                        hintText: '例如：請小心輕放',
                        hintStyle: TextStyle(
                          fontSize: 14.sp,
                          color: _gray.withValues(alpha: 0.4),
                        ),
                        contentPadding: EdgeInsets.all(12.w),
                        border: InputBorder.none,
                      ),
                    ),
                  ),
                  SizedBox(height: 18.h),

                  // ── 呼叫按鈕 ─────────────────────────────────────────
                  SizedBox(
                    width: double.infinity,
                    height: HkControl.minTap,
                    child: ElevatedButton(
                      onPressed: _submitting ? null : _submit,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: _green,
                        disabledBackgroundColor: _green.withValues(alpha: 0.45),
                        elevation: 0,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(999.r),
                        ),
                      ),
                      child: _submitting
                          ? SizedBox(
                              width: 20.w,
                              height: 20.w,
                              child: const CircularProgressIndicator(
                                strokeWidth: 2,
                                color: AppColors.bgCard,
                              ),
                            )
                          : HkButtonLabel(
                              '呼叫黑貓取件',
                              style: TextStyle(
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                                color: AppColors.bgCard,
                              ),
                            ),
                    ),
                  ),
                ],
              ),
            ),
            Positioned(
              top: -8.h,
              right: -8.w,
              child: const DialogCloseButton(
                visualAlignment: Alignment.topRight,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildToggleRow({
    required HkIconName icon,
    required String label,
    required bool value,
    required ValueChanged<bool> onChanged,
    required bool showDivider,
  }) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
          child: Row(
            children: [
              HkIcon(icon, size: 20.sp, color: _gray.withValues(alpha: 0.7)),
              SizedBox(width: 10.w),
              Expanded(
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: _gray,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
              AppToggleSwitch(value: value, onChanged: onChanged),
            ],
          ),
        ),
        if (showDivider)
          Divider(
            height: 1,
            thickness: 0.5,
            indent: 14.w,
            endIndent: 14.w,
            color: _gray.withValues(alpha: 0.12),
          ),
      ],
    );
  }
}

/// Dialog 回傳參數
class CallTcatParams {
  const CallTcatParams({
    required this.isContact,
    required this.isTrolley,
    required this.memo,
  });
  final String isContact; // Y/N
  final String isTrolley; // Y/N
  final String memo;
}
