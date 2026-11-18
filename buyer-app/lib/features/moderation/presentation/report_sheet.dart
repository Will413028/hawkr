import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:uuid/uuid.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/suspension_signal.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../data/legacy_report_repository.dart';
import '../data/moderation_repository.dart';
import '../data/report_target.dart';
import '../data/report_target_type.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 一個檢舉原因選項：社群對象帶 [category]，聊天與評價（舊 REST）帶 [legacy]。
class _ReasonOption {
  const _ReasonOption({
    required this.label,
    required this.requiresDescription,
    this.category,
    this.legacy,
  });

  final String label;

  /// 選了這項要補說明（「其他」）。
  final bool requiresDescription;
  final PolicyCategory? category;
  final ReportReasonCode? legacy;

  bool sameAs(_ReasonOption other) =>
      category == other.category && legacy == other.legacy;
}

/// 一次送出嘗試：內容（原因與說明）沒變的重試帶同一個 request_id，伺服器回第一次的結果；
/// 內容變了就是新的嘗試，換新的 request_id（同一個 request_id 帶不同內容會被拒絕）。
class _SubmitAttempt {
  const _SubmitAttempt(this.category, this.description, this.requestId);

  final PolicyCategory category;
  final String description;
  final String requestId;
}

/// 通用檢舉底部彈窗。社群內容（貼文、留言、討論區留言）送 `ModerationAppService`，
/// 聊天與評價送舊 `/app/reports`；用哪一條由 [target] 的型別決定。
/// 使用：`await showReportSheet(context, target: const SocialReportTarget.post('123'))`
class ReportSheetWidget extends ConsumerStatefulWidget {
  const ReportSheetWidget({super.key, required this.target});

  final ReportTarget target;

  @override
  ConsumerState<ReportSheetWidget> createState() => _ReportSheetWidgetState();
}

class _ReportSheetWidgetState extends ConsumerState<ReportSheetWidget> {
  static const _uuid = Uuid();

  _ReasonOption? _selected;
  _SubmitAttempt? _attempt;
  final TextEditingController _detailCtrl = TextEditingController();
  bool _submitting = false;

  bool get _canSubmit {
    final selected = _selected;
    if (_submitting || selected == null) return false;
    if (selected.requiresDescription) {
      return _detailCtrl.text.trim().isNotEmpty;
    }
    return true;
  }

  List<_ReasonOption> _options(UiLocale locale) => switch (widget.target) {
    SocialReportTarget() => [
      for (final category in communityReportCategories)
        _socialOption(category, locale),
    ],
    LegacyReportTarget(:final type) => [
      for (final reason in reportReasonsFor(type))
        _ReasonOption(
          label: reason.label,
          requiresDescription: reason == ReportReasonCode.other,
          legacy: reason,
        ),
    ],
  };

  _ReasonOption _socialOption(PolicyCategory category, UiLocale locale) {
    return _ReasonOption(
      label: category.uiText(locale) ?? '',
      requiresDescription: category == PolicyCategory.POLICY_CATEGORY_OTHER,
      category: category,
    );
  }

  _SubmitAttempt _attemptFor(PolicyCategory category, String description) {
    final previous = _attempt;
    if (previous != null &&
        previous.category == category &&
        previous.description == description) {
      return previous;
    }
    return _attempt = _SubmitAttempt(category, description, _uuid.v4());
  }

  @override
  void dispose() {
    _detailCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    final selected = _selected!;
    setState(() => _submitting = true);
    try {
      final detail = _detailCtrl.text.trim();
      switch (widget.target) {
        case final SocialReportTarget target:
          await ref
              .read(moderationRepositoryProvider)
              .submitReport(
                target: target,
                category: selected.category!,
                description: detail,
                requestId: _attemptFor(selected.category!, detail).requestId,
              );
        case final LegacyReportTarget target:
          await ref
              .read(legacyReportRepositoryProvider)
              .submitReport(
                targetType: target.type.value,
                targetId: target.id,
                reasonCode: selected.legacy!.code,
                description: detail.isNotEmpty ? detail : null,
              );
      }
      if (!mounted) return;
      Navigator.of(context).pop(true);
      showFloatingToast(context, '舉報成功，我們會盡快審核', isSuccess: true);
    } catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      // 停權由 transport 的 interceptor 提示，這裡不再疊一則失敗訊息。
      if (isAccountSuspendedError(e)) return;
      showFloatingToast(
        context,
        widget.target is SocialReportTarget
            ? '檢舉失敗，請稍後再試'
            : e.toString().replaceAll('Exception: ', ''),
        isSuccess: false,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final reasons = _options(ref.watch(uiLocaleProvider));
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
                        selected: _selected?.sameAs(reason) ?? false,
                        onTap: () => setState(() => _selected = reason),
                      ),
                    if (_selected?.requiresDescription ?? false)
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
  final _ReasonOption reason;
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
                // 「其他」要補說明，選項後面加提示（送出鈕在說明填好前鎖住）。
                reason.requiresDescription
                    ? '${reason.label}（請說明）'
                    : reason.label,
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
  required ReportTarget target,
}) {
  return showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    showDragHandle: false,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (_) => ReportSheetWidget(target: target),
  );
}
