import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../core/exceptions/app_exception.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../data/report_model.dart';
import '../../data/report_repository.dart';
import '../report_block_error_dispatch.dart';

/// 顯示檢舉底部選單（backend-payuni#645）。
///
/// [targetType] 決定 admin 裁決時會打到哪張表，選值規則見 [ReportTargetType]；
/// [targetId] 是對應實體的 string id（TSID）。
/// 回傳 true 代表已成功送出檢舉。
Future<bool> showReportSheet(
  BuildContext context,
  WidgetRef ref, {
  required ReportTargetType targetType,
  required String targetId,
}) async {
  final result = await showModalBottomSheet<bool>(
    context: context,
    // 必須掛根 Navigator。本 sheet 有三個開啟點，其中留言 tab（`PageEchoTab`，
    // `StatefulShellRoute` 的分支路由）位於浮動導覽列**之下**的分支 Navigator：
    // 不掛根 Navigator 時 sheet 底部的「送出檢舉」鈕會整顆被導覽列蓋住，
    // 該路徑的檢舉永遠送不出去（hawkr_seller#132）。另兩個開啟點
    // （`page_home_feed_screen` / `chat_thread_screen`）是 `rootNavigatorKey`
    // 的根路由所以一直正常——這個層級差就是本 bug 的交叉驗證。
    useRootNavigator: true,
    isScrollControlled: true,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (_) =>
        _ReportSheetContent(targetType: targetType, targetId: targetId),
  );
  return result ?? false;
}

class _ReportSheetContent extends ConsumerStatefulWidget {
  const _ReportSheetContent({required this.targetType, required this.targetId});

  final ReportTargetType targetType;
  final String targetId;

  @override
  ConsumerState<_ReportSheetContent> createState() =>
      _ReportSheetContentState();
}

class _ReportSheetContentState extends ConsumerState<_ReportSheetContent> {
  String? _selectedReason;
  final _descController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _descController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final reason = _selectedReason;
    if (reason == null || _submitting) return;
    setState(() => _submitting = true);
    try {
      await ref
          .read(reportRepositoryProvider)
          .submitReport(
            targetType: widget.targetType.value,
            targetId: widget.targetId,
            reasonCode: reason,
            description: _descController.text.trim().isEmpty
                ? null
                : _descController.text.trim(),
          );
      if (!mounted) return;
      Navigator.of(context).pop(true);
      FloatingToast.success(context, '已送出檢舉，感謝你的回報');
    } catch (e) {
      if (!mounted) return;
      final handled = handleReportBlockApiError(context, e);
      if (!handled) {
        FloatingToast.error(
          context,
          e is AppException ? e.message : '檢舉送出失敗，請稍後再試',
        );
      }
      setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.only(
          left: 20.w,
          right: 20.w,
          top: 16.h,
          bottom: MediaQuery.of(context).viewInsets.bottom + 16.h,
        ),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(
                child: Container(
                  width: 36.w,
                  height: 4.h,
                  decoration: BoxDecoration(
                    color: AppColors.gray200,
                    borderRadius: BorderRadius.circular(2.r),
                  ),
                ),
              ),
              SizedBox(height: 16.h),
              Text(
                '檢舉${widget.targetType.label}',
                style: TextStyle(
                  fontSize: 18.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textEmphasis,
                ),
              ),
              SizedBox(height: 4.h),
              Text(
                '請選擇檢舉原因，我們會盡快處理',
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textSecondary,
                ),
              ),
              SizedBox(height: 12.h),
              Wrap(
                spacing: 8.w,
                runSpacing: 8.h,
                children: [
                  for (final r in reportReasonsFor(widget.targetType))
                    ChoiceChip(
                      label: Text(r.label),
                      selected: _selectedReason == r.code,
                      onSelected: (_) =>
                          setState(() => _selectedReason = r.code),
                      selectedColor: AppColors.primarySoft,
                      backgroundColor: AppColors.bgPrimary,
                      labelStyle: TextStyle(
                        fontSize: 14.sp,
                        color: _selectedReason == r.code
                            ? AppColors.primary
                            : AppColors.textPrimary,
                        fontWeight: _selectedReason == r.code
                            ? FontWeight.w700
                            : FontWeight.w400,
                      ),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(999.r),
                        side: BorderSide(
                          color: _selectedReason == r.code
                              ? AppColors.primary
                              : AppColors.dividerLight,
                        ),
                      ),
                    ),
                ],
              ),
              SizedBox(height: 12.h),
              TextField(
                controller: _descController,
                maxLines: 3,
                maxLength: 200,
                decoration: InputDecoration(
                  hintText: '補充說明（選填）',
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(12.r),
                  ),
                ),
              ),
              SizedBox(height: 8.h),
              SizedBox(
                width: double.infinity,
                height: HkControl.minTap,
                child: ElevatedButton(
                  onPressed: (_selectedReason == null || _submitting)
                      ? null
                      : _submit,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primary,
                    disabledBackgroundColor: AppColors.dividerLight,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12.r),
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
                          '送出檢舉',
                          style: TextStyle(
                            color: AppColors.bgCard,
                            fontSize: 16.sp,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
