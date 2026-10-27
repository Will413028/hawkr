import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../social/data/social_repository.dart';
import '../data/report_target_type.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 通用檢舉底部彈窗 — 取代舊版僅支援貼文的 `ReportPostDialog`，支援全部
/// 6 種 target_type。
/// 使用：`await showReportSheet(context, targetType: ReportTargetType.post, targetId: '123')`
class ReportSheetWidget extends ConsumerStatefulWidget {
  const ReportSheetWidget({
    super.key,
    required this.targetType,
    required this.targetId,
  });

  final ReportTargetType targetType;
  final String targetId;

  @override
  ConsumerState<ReportSheetWidget> createState() => _ReportSheetWidgetState();
}

class _ReportSheetWidgetState extends ConsumerState<ReportSheetWidget> {
  ReportReasonCode? _selected;
  final TextEditingController _detailCtrl = TextEditingController();
  bool _submitting = false;

  bool get _canSubmit {
    if (_submitting || _selected == null) return false;
    if (_selected == ReportReasonCode.other) {
      return _detailCtrl.text.trim().isNotEmpty;
    }
    return true;
  }

  @override
  void dispose() {
    _detailCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() => _submitting = true);
    try {
      final detail = _detailCtrl.text.trim();
      await ref
          .read(socialRepositoryProvider)
          .submitReport(
            targetType: widget.targetType.value,
            targetId: widget.targetId,
            reasonCode: _selected!.code,
            description: detail.isNotEmpty ? detail : null,
          );
      if (!mounted) return;
      Navigator.of(context).pop(true);
      showFloatingToast(context, '舉報成功，我們會盡快審核', isSuccess: true);
    } catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      showFloatingToast(
        context,
        e.toString().replaceAll('Exception: ', ''),
        isSuccess: false,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final reasons = reportReasonsFor(widget.targetType);
    return Padding(
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom,
      ),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(height: 8.h),
            Container(
              width: 36.w,
              height: 4.h,
              decoration: BoxDecoration(
                color: AppColors.gray300,
                borderRadius: BorderRadius.circular(2.r),
              ),
            ),
            SizedBox(height: 16.h),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  AppIcons.reportOutlined,
                  size: 22.sp,
                  color: AppIconColors.warning,
                ),
                SizedBox(width: 8.w),
                Text(
                  '檢舉',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
              ],
            ),
            SizedBox(height: 8.h),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 24.w),
              child: Text(
                '請選擇檢舉原因，我們會盡快審核處理',
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 14,
                  color: AppColors.textSecondary,
                  height: 1.4,
                ),
              ),
            ),
            SizedBox(height: 12.h),
            Flexible(
              child: SingleChildScrollView(
                key: const ValueKey('report-reasons-scroll'),
                padding: EdgeInsets.symmetric(horizontal: 12.w),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    for (final reason in reasons)
                      _ReasonTile(
                        reason: reason,
                        selected: _selected == reason,
                        onTap: () => setState(() => _selected = reason),
                      ),
                    if (_selected == ReportReasonCode.other)
                      Padding(
                        padding: EdgeInsets.fromLTRB(12.w, 4.h, 12.w, 8.h),
                        child: TextField(
                          controller: _detailCtrl,
                          maxLines: 3,
                          maxLength: 200,
                          onChanged: (_) => setState(() {}),
                          decoration: InputDecoration(
                            hintText: '請描述原因（必填）',
                            hintStyle: TextStyle(
                              fontSize: 14,
                              color: AppColors.textDisabled,
                            ),
                            filled: true,
                            fillColor: AppColors.gray100,
                            contentPadding: EdgeInsets.symmetric(
                              horizontal: 12.w,
                              vertical: 12.h,
                            ),
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(8.r),
                              borderSide: BorderSide.none,
                            ),
                            counterStyle: TextStyle(fontSize: 12),
                          ),
                          style: TextStyle(fontSize: 14),
                        ),
                      ),
                  ],
                ),
              ),
            ),
            SizedBox(height: 12.h),
            Padding(
              padding: EdgeInsets.fromLTRB(16.w, 0, 16.w, 16.h),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  key: const ValueKey('report-submit'),
                  onPressed: _canSubmit ? _submit : null,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.danger,
                    disabledBackgroundColor: AppColors.gray300,
                    minimumSize: const Size(0, HkControl.minTap),
                    padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12.r),
                    ),
                  ),
                  child: _submitting
                      ? SizedBox(
                          width: 20.w,
                          height: 20.w,
                          child: const HkBrandLoader(
                            tone: HkBrandLoaderTone.mono,
                            color: Colors.white,
                          ),
                        )
                      : HkButtonLabel(
                          '送出檢舉',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            color: Colors.white,
                          ),
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

class _ReasonTile extends StatelessWidget {
  const _ReasonTile({
    required this.reason,
    required this.selected,
    required this.onTap,
  });
  final ReportReasonCode reason;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8.r),
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
        child: Row(
          children: [
            Icon(
              selected
                  ? AppIcons.radioButtonChecked
                  : AppIcons.radioButtonUnchecked,
              size: 20.sp,
              color: selected ? AppIconColors.danger : AppIconColors.disabled,
            ),
            SizedBox(width: 10.w),
            Expanded(
              child: Text(
                reportReasonOptionLabel(reason),
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: selected ? FontWeight.w700 : FontWeight.w400,
                  color: selected
                      ? AppColors.textEmphasis
                      : AppColors.textSecondary,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 顯示通用檢舉底部彈窗。回傳 true 表示已成功送出檢舉。
Future<bool?> showReportSheet(
  BuildContext context, {
  required ReportTargetType targetType,
  required String targetId,
}) {
  return showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    showDragHandle: false,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (_) =>
        ReportSheetWidget(targetType: targetType, targetId: targetId),
  );
}
