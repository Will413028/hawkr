import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart'
    show Measure;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart'
    show MeasureKind, MeasureState, SellerCapability;
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/router/route_capabilities.dart';
import '../../../core/session/my_store.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/icons/app_icons.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../data/current_store.dart';
import '../data/measures.dart';
import 'profile_other_options_card.dart' show ProfileLogoutButton;
import 'seller_appeal_screen.dart' show MeasureCard;

/// 等待刪除的提示文字。
const pendingDeletionNotice = '帳號刪除處理中，等待未結訂單完成；期間只能處理訂單與售後';

/// App 沒有稅務身分與買家發票申訴的頁面，改以一行說明取代 website 的兩張連結卡。
const invoiceTaxNotice = '發票與稅務相關事項請至網站處理';

const _suspendedTitle = '商店已暫停營業';
const _pendingDeletionTitle = '帳號刪除處理中';
const _suspendedSubtitle =
    '你的商家帳號目前處於停權狀態，商品已從市集下架、無法新增或變更上架商品。已成立的訂單仍可依下方白名單功能繼續處理。';
const _terminatedSubtitle =
    '店家已暫停營業，已成立的訂單仍需完成；訂單完成後店家會關閉。終止生效後 6 個月內申覆成立，可恢復同一家店。';
const _loadingText = '載入中…';
const _accountStatusText = '帳號狀態';
const _supportHeading = '對停權決定有疑義？';
const _terminatedSupportHeading = '對終止決定有疑義？';
const _supportBody = '到措施與申覆查看措施內容與期間，並可對仍可申覆的措施提出申覆。';
const _supportAction = '措施與申覆';

class NoticeEntry {
  const NoticeEntry(this.route, this.title, this.description);

  final String route;
  final String title;
  final String description;
}

const _entries = [
  NoticeEntry(MainRoutes.settings, '商店設定', '檢視商店資料；需要時可以在這裡關閉商店。'),
  NoticeEntry(MainRoutes.orders, '訂單管理', '既有訂單的出貨、狀態更新與售後處理，仍可正常使用。'),
  NoticeEntry(SettingsRoutes.notifications, '通知', '查看訂單與平台通知，掌握必要動態。'),
];

/// 限制中店家的公告頁文案與入口，全由 `GetMyStore` 的狀態與生效中的措施（`ListMyMeasures` 的 `state=ACTIVE`）推導
/// （語意與文案照 website 的 `SellerSuspendedNotice`）。
class SuspensionNoticeContent {
  SuspensionNoticeContent(
    MyStore? store,
    this._locale, {
    bool? pendingOnly,
    AsyncValue<List<Measure>>? measures,
  }) : loading = store == null,
       pendingOnly =
           pendingOnly ??
           (store != null && store.isPendingDeletion && !store.isSuspended),
       isSuspended = store?.isSuspended ?? false,
       _measures = measures,
       _capabilities = store?.capabilities ?? const <SellerCapability>[];

  final bool loading;
  final UiLocale _locale;

  /// 只有等待刪除、沒有停權。
  final bool pendingOnly;

  /// 帳號此刻停權（`GetMyStore.suspended`）。
  final bool isSuspended;
  final AsyncValue<List<Measure>>? _measures;
  final List<SellerCapability> _capabilities;

  /// 生效中的措施；還沒讀到、讀取失敗或沒有時是空的。
  List<Measure> get activeMeasures => _measures?.value ?? const [];

  /// 生效中的措施裡有終止合作：公告頁是「合作已終止」（種類由伺服器給，不從事由或時間推導）。
  bool get terminated =>
      !pendingOnly &&
      activeMeasures.any((m) => m.kind == MeasureKind.MEASURE_KIND_TERMINATION);

  /// 停權中、生效中的措施還沒讀到：還不知道是停權還是終止合作（`GetMyStore` 只給 `suspended`），標題與說明不能先用停權的字樣
  /// （終止合作的賣家會先看到「停權」）。
  bool get kindPending {
    final measures = _measures;
    return !pendingOnly &&
        isSuspended &&
        measures != null &&
        measures.isLoading &&
        !measures.hasValue;
  }

  /// 停權中、生效中的措施讀取失敗：同樣不知道種類，只用中性文案（與 website 的 `SellerSuspendedNotice` 一致）。
  bool get kindFailed {
    final measures = _measures;
    return !pendingOnly &&
        isSuspended &&
        measures != null &&
        measures.hasError &&
        !measures.hasValue;
  }

  bool get _kindUnknown => kindPending || kindFailed;

  String get title => pendingOnly
      ? _pendingDeletionTitle
      : kindPending
      ? _suspendedTitle
      : terminated
      ? MeasureState.MEASURE_STATE_ACTIVE.uiText(
              _locale,
              kind: MeasureKind.MEASURE_KIND_TERMINATION,
            ) ??
            commonUnknown(_locale)
      : _suspendedTitle;

  String? get subtitle => pendingOnly || _kindUnknown
      ? null
      : terminated
      ? _terminatedSubtitle
      : _suspendedSubtitle;

  String get reasonHeading => pendingOnly || _kindUnknown
      ? _accountStatusText
      : terminated
      ? '終止原因'
      : '停權原因';

  /// 用措施卡片取代單行的原因文字。
  bool get showsMeasures => !pendingOnly && activeMeasures.isNotEmpty;

  /// 單行的原因文字：等待刪除的說明；停權但沒有（讀到）措施時是通用說明，措施還在讀取時是讀取中。
  String get reasonBody {
    if (pendingOnly) return pendingDeletionNotice;
    final measures = _measures;
    if (loading ||
        (measures != null && measures.isLoading && !measures.hasValue)) {
      return _loadingText;
    }
    if (kindFailed) return _suspendedTitle;
    return MyStore.genericSuspensionLabel;
  }

  String get entriesHeading =>
      pendingOnly || _kindUnknown || terminated ? '目前仍可使用' : '停權期間仍可使用';

  /// 目前仍可使用的 App 頁面（依能力）。
  List<NoticeEntry> get entries => [
    for (final entry in _entries)
      if (isRouteAvailable(entry.route, _capabilities)) entry,
  ];

  /// 停權的疑義入口：到措施與申覆；等待刪除與讀取中沒有。
  SuspensionSupport? get support {
    if (pendingOnly || !isSuspended) return null;
    return SuspensionSupport(
      heading: _kindUnknown
          ? null
          : terminated
          ? _terminatedSupportHeading
          : _supportHeading,
      body: _supportBody,
      actionLabel: _supportAction,
      route: SellerRoutes.measures,
    );
  }
}

/// 停權公告頁的疑義入口：說明、按鈕文字與目的地。
class SuspensionSupport {
  const SuspensionSupport({
    this.heading,
    required this.body,
    required this.actionLabel,
    required this.route,
  });

  /// 還不知道措施種類（讀取中或失敗）時沒有標題。
  final String? heading;
  final String body;
  final String actionLabel;
  final String route;
}

void _open(BuildContext context, String route) {
  if (route.startsWith('/main/')) {
    context.go(route);
  } else {
    context.push(route);
  }
}

/// 賣家停權／等待刪除公告頁（46 號設計書「賣家能力權限層」§7.2）。
///
/// 賣家收 `SELLER_SUSPENDED`／9240，或 `GetMyStore` 讀到停權或等待刪除、而目前的頁面需要的能力被收回時，
/// router gating（見 `redirect_logic.dart`、`route_capabilities.dart`）導來此頁；仍可用的頁面由能力決定，
/// 賣家從這裡繼續處理。
///
/// 沒有 leading 返回鍵——這是攔截頁，離開只能透過下方的入口。
class SuspensionNoticeScreen extends ConsumerWidget {
  const SuspensionNoticeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final store = ref.watch(myStoreSnapshotProvider);
    final content = SuspensionNoticeContent(
      store,
      context.uiLocale,
      measures: store != null && store.isSuspended
          ? ref.watch(
              currentMyMeasuresProvider(MeasureState.MEASURE_STATE_ACTIVE),
            )
          : null,
    );

    return Scaffold(
      backgroundColor: AppColors.bgApp,
      appBar: HkGlassAppBar(title: Text(content.title), showBackButton: false),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(20.w, 32.h, 20.w, 32.h),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Center(
                child: Container(
                  width: 72.w,
                  height: 72.w,
                  decoration: BoxDecoration(
                    color: AppColors.dangerStrong.withValues(alpha: 0.12),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    AppIcons.warningAmberRounded,
                    color: AppIconColors.warning,
                    size: 40.sp,
                  ),
                ),
              ),
              if (content.subtitle != null) ...[
                SizedBox(height: 20.h),
                Text(
                  content.subtitle!,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
              SizedBox(height: 24.h),
              if (content.showsMeasures)
                _MeasuresSection(
                  heading: content.reasonHeading,
                  measures: content.activeMeasures,
                )
              else
                SuspensionReasonCard(
                  heading: content.reasonHeading,
                  body: content.reasonBody,
                ),
              SizedBox(height: 20.h),
              _EntriesCard(
                heading: content.entriesHeading,
                entries: content.entries,
              ),
              SizedBox(height: 32.h),
              if (content.support case final support?) ...[
                _SupportCard(
                  support: support,
                  onPressed: () => context.push(support.route),
                ),
                SizedBox(height: 12.h),
              ],
              const ProfileLogoutButton(),
            ],
          ),
        ),
      ),
    );
  }
}

/// 生效中的措施：每一項一張卡（可申覆的帶「提出申覆」）。
class _MeasuresSection extends StatelessWidget {
  const _MeasuresSection({required this.heading, required this.measures});

  final String heading;
  final List<Measure> measures;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          heading,
          style: TextStyle(
            fontSize: 14.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.gray1,
          ),
        ),
        SizedBox(height: 8.h),
        for (final measure in measures) ...[
          MeasureCard(
            measure: measure,
            onAppeal: () =>
                context.push(SellerRoutes.measureRequestFor(measure.id)),
            onRemediate: () =>
                context.push(SellerRoutes.measureRemediationFor(measure.id)),
          ),
          SizedBox(height: 12.h),
        ],
      ],
    );
  }
}

/// 停權原因（或等待刪除的狀態）卡片：純展示元件，不依賴 Riverpod，方便獨立單元測試。
class SuspensionReasonCard extends StatelessWidget {
  const SuspensionReasonCard({
    super.key,
    required this.heading,
    required this.body,
  });

  final String heading;
  final String body;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.dangerStrong.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(
          color: AppColors.dangerStrong.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            heading,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
          SizedBox(height: 4.h),
          Text(
            body,
            style: TextStyle(
              fontSize: 16.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.dangerStrong,
            ),
          ),
        ],
      ),
    );
  }
}

class _EntriesCard extends StatelessWidget {
  const _EntriesCard({required this.heading, required this.entries});

  final String heading;
  final List<NoticeEntry> entries;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.border),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 12,
            offset: Offset(0, 4.h),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            heading,
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.gray1,
            ),
          ),
          SizedBox(height: 8.h),
          ...entries.map(
            (entry) => InkWell(
              onTap: () => _open(context, entry.route),
              child: Padding(
                padding: EdgeInsets.symmetric(vertical: 6.h),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      AppIcons.checkCircleOutline,
                      size: 16.sp,
                      color: AppIconColors.success,
                    ),
                    SizedBox(width: 8.w),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            entry.title,
                            style: TextStyle(
                              fontSize: 14.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.gray1,
                            ),
                          ),
                          Text(
                            entry.description,
                            style: TextStyle(
                              fontSize: 12.sp,
                              color: AppColors.textSecondary,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          SizedBox(height: 8.h),
          Text(
            invoiceTaxNotice,
            style: TextStyle(fontSize: 12.sp, color: AppColors.textSecondary),
          ),
        ],
      ),
    );
  }
}

class _SupportCard extends StatelessWidget {
  const _SupportCard({required this.support, required this.onPressed});

  final SuspensionSupport support;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (support.heading case final heading?) ...[
            Text(
              heading,
              style: TextStyle(
                fontSize: 14.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.gray1,
              ),
            ),
            SizedBox(height: 8.h),
          ],
          Text(
            support.body,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
          SizedBox(height: 12.h),
          OutlinedButton(
            onPressed: onPressed,
            style: OutlinedButton.styleFrom(
              foregroundColor: AppColors.gray1,
              side: BorderSide(color: AppColors.border),
              padding: EdgeInsets.symmetric(vertical: 14.h),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8.r),
              ),
            ),
            child: HkButtonLabel(
              support.actionLabel,
              style: TextStyle(fontSize: 16.sp, fontWeight: FontWeight.w700),
            ),
          ),
        ],
      ),
    );
  }
}
