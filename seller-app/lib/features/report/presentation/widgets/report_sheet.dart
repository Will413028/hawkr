import 'package:hawkr_seller/l10n/l10n.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show MaxLengthEnforcement;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:uuid/uuid.dart';

import '../../../../core/l10n/ui_locale.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../data/moderation_repository.dart';

/// 檢舉說明的上限（`moderation.proto` SubmitReportRequest.description，以 rune 計）。
const reportDescriptionMaxRunes = 500;

/// 顯示檢舉聊天訊息的底部選單；[messageId] 是被檢舉訊息的 ID，檢舉人由伺服器依訊息所屬的對話推導。
/// 回傳 true 代表已成功送出檢舉。
Future<bool> showReportSheet(
  BuildContext context,
  WidgetRef ref, {
  required String messageId,
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
    builder: (_) => _ReportSheetContent(messageId: messageId),
  );
  return result ?? false;
}

class _ReportSheetContent extends ConsumerStatefulWidget {
  const _ReportSheetContent({required this.messageId});

  final String messageId;

  @override
  ConsumerState<_ReportSheetContent> createState() =>
      _ReportSheetContentState();
}

class _ReportSheetContentState extends ConsumerState<_ReportSheetContent> {
  static const _uuid = Uuid();

  PolicyCategory? _selected;
  ({PolicyCategory category, String description, String requestId})? _attempt;
  final _descController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _descController.dispose();
    super.dispose();
  }

  bool get _canSubmit {
    final selected = _selected;
    if (_submitting || selected == null) return false;
    if (selected == PolicyCategory.POLICY_CATEGORY_OTHER) {
      return _descController.text.trim().isNotEmpty;
    }
    return true;
  }

  /// 內容（原因與說明）沒變的重試帶同一個 request_id；內容變了就是新的嘗試，換新的 request_id。
  String _requestIdFor(PolicyCategory category, String description) {
    final previous = _attempt;
    if (previous != null &&
        previous.category == category &&
        previous.description == description) {
      return previous.requestId;
    }
    final next = (
      category: category,
      description: description,
      requestId: _uuid.v4(),
    );
    _attempt = next;
    return next.requestId;
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    final category = _selected!;
    final description = _descController.text.trim();
    setState(() => _submitting = true);
    try {
      await ref
          .read(moderationRepositoryProvider)
          .reportChatMessage(
            messageId: widget.messageId,
            category: category,
            description: description,
            requestId: _requestIdFor(category, description),
          );
      if (!mounted) return;
      Navigator.of(context).pop(true);
      FloatingToast.success(context, '已送出檢舉，感謝你的回報');
    } catch (e) {
      if (!mounted) return;
      FloatingToast.error(
        context,
        errorText(e, defaults: ref.read(defaultErrorTextProvider)),
      );
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
                context.l10n.reportMessageTitle,
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
                  for (final r in moderationSubmitReportRequestCategory)
                    ChoiceChip(
                      label: Text(r.uiText(ref.watch(uiLocaleProvider)) ?? ''),
                      selected: _selected == r,
                      onSelected: (_) => setState(() => _selected = r),
                      selectedColor: AppColors.primarySoft,
                      backgroundColor: AppColors.bgPrimary,
                      labelStyle: TextStyle(
                        fontSize: 14.sp,
                        color: _selected == r
                            ? AppColors.primary
                            : AppColors.textPrimary,
                        fontWeight: _selected == r
                            ? FontWeight.w700
                            : FontWeight.w400,
                      ),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(999.r),
                        side: BorderSide(
                          color: _selected == r
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
                maxLength: reportDescriptionMaxRunes,
                maxLengthEnforcement: MaxLengthEnforcement.enforced,
                onChanged: (_) => setState(() {}),
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
                  onPressed: _canSubmit ? _submit : null,
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
