import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as pb;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/suspension_signal.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/store_report_repository.dart';
import 'report_view_parts.dart';
import 'store_report_detail_provider.dart';
import 'paged_section_widgets.dart';
import 'store_report_supplement_screen.dart';

String storeReportDetailRoute(String reportId) =>
    '/settings/reports/${Uri.encodeComponent(reportId)}';

/// 佐證在檢舉本身的張數：不屬於任何補充說明的那些。
int reportOwnEvidenceCount(pb.GetMyReportResponse detail) {
  final inSupplements = {
    for (final s in detail.supplements) ...s.evidencePositions,
  };
  return detail.evidence
      .where((e) => !inSupplements.contains(e.position))
      .length;
}

/// 我的店家檢舉詳情（`GetMyReport`）。
class StoreReportDetailScreen extends ConsumerWidget {
  const StoreReportDetailScreen({super.key, required this.reportId});

  final String reportId;

  Future<void> _supplement(BuildContext context, WidgetRef ref) async {
    final result = await context.push<Object>(
      storeReportSupplementRoute(reportId),
    );
    final provider = storeReportDetailProvider(reportId);
    if (result is pb.AddReportSupplementResponse) {
      ref.read(provider.notifier).applySupplement(result);
    } else if (result == storeReportSupplementStale) {
      ref.invalidate(provider);
    }
  }

  Widget _error(WidgetRef ref, Object e) {
    if (isAccountSuspendedError(e)) return const SizedBox.shrink();
    final cause = e is StoreReportFailure ? e.cause : e;
    final message = errorText(
      cause,
      defaults: ref.read(defaultErrorTextProvider),
    );
    if (e is StoreReportFailure && e.kind == StoreReportFailureKind.notFound) {
      return HkEmptyState(
        key: const ValueKey('store-report-not-found'),
        icon: HkIconName.emptyAfterSale,
        message: message,
      );
    }
    return PagedErrorState(
      message: message,
      onRetry: () => ref.invalidate(storeReportDetailProvider(reportId)),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final detail = ref.watch(storeReportDetailProvider(reportId));
    final locale = ref.watch(uiLocaleProvider);
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          commonTitleTopBar(
            context,
            title: '檢舉詳情',
            onBack: () => context.pop(),
          ),
          Expanded(
            child: detail.when(
              loading: () => const TruckLoadingOverlay(message: '載入中...'),
              error: (e, _) => _error(ref, e),
              data: (d) => _body(context, ref, d, locale),
            ),
          ),
        ],
      ),
    );
  }

  Widget _body(
    BuildContext context,
    WidgetRef ref,
    pb.GetMyReportResponse d,
    UiLocale locale,
  ) {
    final r = d.report;
    final outcome = storeReportOutcomeText(r, locale);
    final category = r.category.uiText(locale);
    final ownEvidence = reportOwnEvidenceCount(d);
    return Column(
      children: [
        Expanded(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 20.h),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: StoreNameText(
                        r.storeId,
                        style: const TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textPrimary,
                        ),
                      ),
                    ),
                    if (outcome != null) ReportOutcomeChip(outcome),
                  ],
                ),
                if (category != null) ...[
                  SizedBox(height: 8.h),
                  _line('原因：$category'),
                ],
                if (r.orderNo.isNotEmpty) ...[
                  SizedBox(height: 4.h),
                  _line('訂單編號：${r.orderNo}'),
                ],
                if (r.resolutionNote.isNotEmpty) ...[
                  SizedBox(height: 20.h),
                  _title('平台說明'),
                  SizedBox(height: 8.h),
                  _line(r.resolutionNote, key: 'store-report-resolution-note'),
                ],
                SizedBox(height: 20.h),
                _title('你的檢舉'),
                SizedBox(height: 8.h),
                if (r.hasCreateTime())
                  _meta(formatReportTime(r.createTime.toDateTime())),
                if (r.description.isNotEmpty) _line(r.description),
                if (ownEvidence > 0) _meta('已附 $ownEvidence 張佐證'),
                SizedBox(height: 20.h),
                _title('補充說明（${d.supplements.length}/${d.supplementMax}）'),
                SizedBox(height: 8.h),
                for (final s in d.supplements) ...[
                  _meta(
                    s.hasCreateTime()
                        ? formatReportTime(s.createTime.toDateTime())
                        : '',
                  ),
                  _line(s.description),
                  if (s.evidencePositions.isNotEmpty)
                    _meta('已附 ${s.evidencePositions.length} 張佐證'),
                  SizedBox(height: 12.h),
                ],
              ],
            ),
          ),
        ),
        if (d.canSupplement)
          SafeArea(
            top: false,
            child: Padding(
              padding: EdgeInsets.fromLTRB(20.w, 8.h, 20.w, 12.h),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  key: const ValueKey('store-report-supplement-button'),
                  onPressed: () => _supplement(context, ref),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.danger,
                    minimumSize: const Size(0, HkControl.minTap),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12.r),
                    ),
                  ),
                  child: const HkButtonLabel(
                    '補充說明',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                      color: Colors.white,
                    ),
                  ),
                ),
              ),
            ),
          ),
      ],
    );
  }

  Widget _title(String text) => Text(
    text,
    style: const TextStyle(
      fontSize: 16,
      fontWeight: FontWeight.w700,
      color: AppColors.textPrimary,
    ),
  );

  Widget _line(String text, {String? key}) => Text(
    text,
    key: key == null ? null : ValueKey(key),
    style: const TextStyle(fontSize: 14, color: AppColors.textPrimary),
  );

  Widget _meta(String text) => Padding(
    padding: EdgeInsets.only(bottom: 4.h),
    child: Text(
      text,
      style: const TextStyle(fontSize: 12, color: AppColors.textSecondary),
    ),
  );
}
