import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/suspension_signal.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../media/data/media_repository.dart';
import '../data/store_report_repository.dart';
import 'report_form_parts.dart';
import 'rune_limit_formatter.dart';

String storeReportSupplementRoute(String reportId) =>
    '/settings/reports/${Uri.encodeComponent(reportId)}/supplement';

/// 補充被伺服器以前置條件拒絕（例如檢舉已有結果）時的關閉結果：詳情頁據此重讀。
const storeReportSupplementStale = 'stale';

const storeReportSupplementSuccessText = '已收到補充說明';

/// 對自己的店家檢舉補充說明（`AddReportSupplement`）：成功後以 `AddReportSupplementResponse` 關閉，前置條件失敗以 [storeReportSupplementStale] 關閉。
class StoreReportSupplementScreen extends ConsumerStatefulWidget {
  const StoreReportSupplementScreen({super.key, required this.reportId});

  final String reportId;

  @override
  ConsumerState<StoreReportSupplementScreen> createState() =>
      _StoreReportSupplementScreenState();
}

class _StoreReportSupplementScreenState
    extends ConsumerState<StoreReportSupplementScreen> {
  final _description = TextEditingController();
  final List<File> _photos = [];
  final Map<String, String> _stagedByPath = {};
  bool _submitting = false;

  bool get _canSubmit => !_submitting && _description.text.trim().isNotEmpty;

  @override
  void dispose() {
    _description.dispose();
    super.dispose();
  }

  Future<void> _pickPhoto() async {
    if (_photos.length >= storeReportMaxEvidence) return;
    final file = await pickReportPhoto();
    if (file != null && mounted) setState(() => _photos.add(file));
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() => _submitting = true);
    try {
      final tokens = await stageReportEvidence(
        ref.read(mediaRepositoryProvider),
        _photos,
        _stagedByPath,
      );
      final response = await ref
          .read(storeReportRepositoryProvider)
          .addSupplement(
            reportId: widget.reportId,
            description: _description.text.trim(),
            evidenceStagedTokens: tokens,
          );
      if (!mounted) return;
      showFloatingToast(context, storeReportSupplementSuccessText);
      context.pop(response);
    } catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      if (isAccountSuspendedError(e)) return;
      showFloatingToast(
        context,
        errorText(e, defaults: ref.read(defaultErrorTextProvider)),
        isSuccess: false,
      );
      if (e is ConnectException && e.code == Code.failedPrecondition) {
        context.pop(storeReportSupplementStale);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: !_submitting,
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: Column(
          children: [
            commonTitleTopBar(
              context,
              title: '補充說明',
              backIcon: AppIcons.chevronLeft,
              onBack: () {
                if (!_submitting) context.pop();
              },
            ),
            Expanded(
              child: SingleChildScrollView(
                keyboardDismissBehavior:
                    ScrollViewKeyboardDismissBehavior.onDrag,
                padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 32.h),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    reportFieldLabel('說明', required: true),
                    SizedBox(height: 8.h),
                    TextField(
                      key: const ValueKey(
                        'store-report-supplement-description',
                      ),
                      controller: _description,
                      minLines: 4,
                      maxLines: 8,
                      inputFormatters: [
                        RuneLimitFormatter(storeReportDescriptionMaxRunes),
                      ],
                      buildCounter:
                          (
                            _, {
                            required currentLength,
                            required isFocused,
                            maxLength,
                          }) => Text(
                            '${_description.text.runes.length}/$storeReportDescriptionMaxRunes',
                            style: const TextStyle(fontSize: 12),
                          ),
                      onChanged: (_) => setState(() {}),
                      decoration: reportFieldDecoration('請補充檢舉的原因與經過'),
                    ),
                    SizedBox(height: 20.h),
                    reportFieldLabel('佐證圖片'),
                    SizedBox(height: 8.h),
                    ReportPhotoRow(
                      photos: _photos,
                      maxCount: storeReportMaxEvidence,
                      disabled: _submitting,
                      addKey: const ValueKey(
                        'store-report-supplement-add-photo',
                      ),
                      onAdd: _pickPhoto,
                      onRemove: (i) => setState(() => _photos.removeAt(i)),
                    ),
                    SizedBox(height: 28.h),
                    SizedBox(
                      width: double.infinity,
                      child: ElevatedButton(
                        key: const ValueKey('store-report-supplement-submit'),
                        onPressed: _canSubmit ? _submit : null,
                        style: ElevatedButton.styleFrom(
                          backgroundColor: AppColors.danger,
                          disabledBackgroundColor: AppColors.gray300,
                          minimumSize: const Size(0, HkControl.minTap),
                          padding: const EdgeInsets.symmetric(
                            horizontal: HkSpace.lg,
                          ),
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
                            : const HkButtonLabel(
                                '送出補充說明',
                                style: TextStyle(
                                  fontSize: 16,
                                  fontWeight: FontWeight.w700,
                                  color: Colors.white,
                                ),
                              ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
