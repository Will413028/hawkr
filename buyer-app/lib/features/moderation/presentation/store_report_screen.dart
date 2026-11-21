import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as pb;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
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
import 'store_report_entry.dart';

/// 成功後的提示：補充時用伺服器的 notice（有文字才用），否則附上「我的檢舉」的指引。
String storeReportSuccessText(pb.CreateReportResponse response) {
  if (response.supplemented &&
      response.hasNotice() &&
      response.notice.message.trim().isNotEmpty) {
    return response.notice.message;
  }
  return '已收到檢舉，處理結果可在「我的檢舉」查看';
}

/// 檢舉店家的整頁表單（`SellerReportAppService.CreateReport`）。
class StoreReportScreen extends ConsumerStatefulWidget {
  const StoreReportScreen({super.key, required this.args});

  final StoreReportArgs args;

  @override
  ConsumerState<StoreReportScreen> createState() => _StoreReportScreenState();
}

class _StoreReportScreenState extends ConsumerState<StoreReportScreen> {
  PolicyCategory? _category;
  final _description = TextEditingController();
  final _orderNo = TextEditingController();
  final List<File> _photos = [];
  final Map<String, String> _stagedByPath = {};
  bool _submitting = false;

  bool get _canSubmit =>
      !_submitting &&
      _category != null &&
      _description.text.trim().isNotEmpty &&
      widget.args.storeId.isNotEmpty;

  @override
  void dispose() {
    _description.dispose();
    _orderNo.dispose();
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
          .create(
            storeId: widget.args.storeId,
            category: _category!,
            description: _description.text.trim(),
            productId: widget.args.productId,
            orderNo: _orderNo.text.trim(),
            evidenceStagedTokens: tokens,
          );
      if (!mounted) return;
      final message = storeReportSuccessText(response);
      showFloatingToast(context, message, isSuccess: true);
      context.pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      if (isAccountSuspendedError(e)) return;
      showFloatingToast(
        context,
        errorText(e, defaults: ref.read(defaultErrorTextProvider)),
        isSuccess: false,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final locale = ref.watch(uiLocaleProvider);
    final args = widget.args;
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          commonTitleTopBar(
            context,
            title: args.forProduct ? '檢舉商品' : '檢舉店家',
            backIcon: AppIcons.chevronLeft,
            onBack: () => context.pop(),
          ),
          Expanded(
            child: SingleChildScrollView(
              key: const ValueKey('store-report-scroll'),
              keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
              padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 32.h),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (args.forProduct && args.productName.isNotEmpty) ...[
                    Text(
                      '檢舉商品：${args.productName}',
                      key: const ValueKey('store-report-product'),
                      style: const TextStyle(
                        fontSize: 14,
                        color: AppColors.textSecondary,
                      ),
                    ),
                    SizedBox(height: 16.h),
                  ],
                  reportFieldLabel('類別', required: true),
                  SizedBox(height: 8.h),
                  for (final (category, label) in storeReportCategoryOptions(
                    locale,
                  ))
                    _CategoryTile(
                      label: label,
                      selected: _category == category,
                      onTap: () => setState(() => _category = category),
                    ),
                  SizedBox(height: 20.h),
                  reportFieldLabel('說明', required: true),
                  SizedBox(height: 8.h),
                  TextField(
                    key: const ValueKey('store-report-description'),
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
                    decoration: reportFieldDecoration('請描述檢舉的原因與經過'),
                  ),
                  SizedBox(height: 20.h),
                  reportFieldLabel('訂單編號'),
                  SizedBox(height: 8.h),
                  TextField(
                    key: const ValueKey('store-report-order-no'),
                    controller: _orderNo,
                    inputFormatters: [
                      RuneLimitFormatter(storeReportOrderNoMaxRunes),
                    ],
                    decoration: reportFieldDecoration('選填'),
                  ),
                  SizedBox(height: 20.h),
                  reportFieldLabel('佐證圖片'),
                  SizedBox(height: 8.h),
                  ReportPhotoRow(
                    photos: _photos,
                    maxCount: storeReportMaxEvidence,
                    disabled: _submitting,
                    addKey: const ValueKey('store-report-add-photo'),
                    onAdd: _pickPhoto,
                    onRemove: (i) => setState(() => _photos.removeAt(i)),
                  ),
                  SizedBox(height: 28.h),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      key: const ValueKey('store-report-submit'),
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
                              '送出檢舉',
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
    );
  }
}

class _CategoryTile extends StatelessWidget {
  const _CategoryTile({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      label: label,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8.r),
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          child: Row(
            children: [
              Icon(
                selected
                    ? AppIcons.radioButtonChecked
                    : AppIcons.radioButtonUnchecked,
                size: 22.sp,
                color: selected ? AppIconColors.danger : AppIconColors.muted,
              ),
              SizedBox(width: 12.w),
              Expanded(
                child: Text(
                  label,
                  style: const TextStyle(
                    fontSize: 16,
                    color: AppColors.textPrimary,
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
