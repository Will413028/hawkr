import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 儲存使用者輸入的 [value]：成功回傳 null（對話框關閉），失敗回傳要顯示在對話框裡的文字（輸入保留，可以改了再存）。
typedef ProfileEditSave = Future<String?> Function(String value);

/// 顯示純文字編輯對話框。按「保存」時呼叫 [onSave]；失敗時對話框留著，顯示錯誤文字，使用者的輸入不動。
///
/// 返回 `true` 表示已儲存；取消或關閉則返回 `null`。
Future<bool?> showProfileEditDialog(
  BuildContext context, {
  required String title,
  required String initialValue,
  required ProfileEditSave onSave,
  TextInputType keyboardType = TextInputType.text,
  int maxLines = 1,
  int? maxLength,
  String? placeholder,
  String? hint,
  List<TextInputFormatter> inputFormatters = const [],
}) {
  return showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (ctx) => _ProfileEditDialog(
      title: title,
      initialValue: initialValue,
      onSave: onSave,
      keyboardType: maxLines > 1 ? TextInputType.multiline : keyboardType,
      maxLines: maxLines,
      maxLength: maxLength,
      placeholder: placeholder,
      hint: hint,
      inputFormatters: inputFormatters,
    ),
  );
}

class _ProfileEditDialog extends StatefulWidget {
  const _ProfileEditDialog({
    required this.title,
    required this.initialValue,
    required this.onSave,
    required this.keyboardType,
    required this.maxLines,
    required this.maxLength,
    required this.placeholder,
    required this.hint,
    required this.inputFormatters,
  });

  final String title;
  final String initialValue;
  final ProfileEditSave onSave;
  final TextInputType keyboardType;
  final int maxLines;
  final int? maxLength;
  final String? placeholder;
  final String? hint;
  final List<TextInputFormatter> inputFormatters;

  @override
  State<_ProfileEditDialog> createState() => _ProfileEditDialogState();
}

class _ProfileEditDialogState extends State<_ProfileEditDialog> {
  late final TextEditingController controller = TextEditingController(
    text: widget.initialValue,
  );
  String? error;
  bool saving = false;

  @override
  void dispose() {
    controller.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (saving) return;
    setState(() {
      saving = true;
      error = null;
    });
    final failure = await widget.onSave(controller.text);
    if (!mounted) return;
    if (failure == null) {
      Navigator.of(context).pop(true);
      return;
    }
    setState(() {
      saving = false;
      error = failure;
    });
  }

  @override
  Widget build(BuildContext context) {
    final title = widget.title;
    final maxLines = widget.maxLines;
    final keyboardType = widget.keyboardType;
    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.dividerLight,
          borderRadius: BorderRadius.circular(24.r),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: double.infinity,
              padding: EdgeInsets.symmetric(vertical: 20.h, horizontal: 16.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.vertical(top: Radius.circular(24.r)),
              ),
              child: Column(
                children: [
                  Text(
                    '編輯$title',
                    style: TextStyle(
                      fontSize: 18.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  SizedBox(height: 16.h),
                  TextField(
                    controller: controller,
                    keyboardType: keyboardType,
                    minLines: 1,
                    maxLines: maxLines > 1 ? maxLines : 1,
                    maxLength: widget.maxLength,
                    inputFormatters: widget.inputFormatters,
                    autofocus: true,
                    decoration: InputDecoration(
                      hintText: widget.placeholder ?? '請輸入$title',
                      hintStyle: TextStyle(color: AppColors.textSecondary),
                      counterText: '',
                      filled: true,
                      fillColor: AppColors.gray100,
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(12.r),
                        borderSide: BorderSide.none,
                      ),
                      contentPadding: EdgeInsets.all(12.w),
                    ),
                  ),
                  if (widget.hint != null) ...[
                    SizedBox(height: 8.h),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: Text(
                        widget.hint!,
                        style: TextStyle(
                          fontSize: 12.sp,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ),
                  ],
                  if (error != null) ...[
                    SizedBox(height: 8.h),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: Text(
                        error!,
                        key: const Key('profile_edit_error'),
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.danger,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const Divider(height: 0, thickness: 1, color: AppColors.gray300),
            // ── 按鈕列 ───────────────────────────────
            Row(
              children: [
                Expanded(
                  child: Material(
                    color: AppColors.bgCard,
                    borderRadius: BorderRadius.only(
                      bottomLeft: Radius.circular(24.r),
                    ),
                    child: InkWell(
                      borderRadius: BorderRadius.only(
                        bottomLeft: Radius.circular(24.r),
                      ),
                      onTap: saving ? null : () => Navigator.of(context).pop(),
                      child: SizedBox(
                        height: 54.h,
                        child: Center(
                          child: Text(
                            '取消',
                            style: TextStyle(
                              fontSize: 20.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.textEmphasis,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
                Container(width: 1, height: 54.h, color: AppColors.gray300),
                Expanded(
                  child: Material(
                    color: AppColors.bgCard,
                    borderRadius: BorderRadius.only(
                      bottomRight: Radius.circular(24.r),
                    ),
                    child: InkWell(
                      borderRadius: BorderRadius.only(
                        bottomRight: Radius.circular(24.r),
                      ),
                      onTap: saving ? null : _save,
                      child: SizedBox(
                        height: 54.h,
                        child: Center(
                          child: Text(
                            '保存',
                            style: TextStyle(
                              fontSize: 20.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.textEmphasis,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
