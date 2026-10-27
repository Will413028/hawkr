import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_status_notifier.dart';

import '../../../../config/image_config.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/common_top_bar.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/live_countdown_line.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../../../shared/widgets/step_indicator.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../data/after_sale_model.dart';
import '../../data/after_sale_order_lookup.dart';
import '../../data/after_sale_repository.dart';
import '../../data/order_model.dart';
import '../../data/order_repository.dart';
import '../widgets/return_logistics_timeline.dart';
import 'package:hawkr_buyer/shared/widgets/fruit_refresh_indicator.dart';

/// 售後服務詳情頁 — 使用 OpenAPI 定義的 /app/after_sales/{case_no}
/// 與 /app/orders/{order_no} 欄位；空值不渲染。
class AfterSaleDetailScreen extends HookConsumerWidget {
  const AfterSaleDetailScreen({super.key, this.caseNo, this.orderSn});
  final String? caseNo;
  final String? orderSn;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final loading = useState(true);
    final detail = useState<AfterSaleCase?>(null);
    final order = useState<Order?>(null);
    final error = useState<String?>(null);

    Future<void> load() async {
      loading.value = true;
      error.value = null;
      detail.value = null;
      order.value = null;

      try {
        final repo = ref.read(afterSaleRepositoryProvider);
        AfterSaleCase? loaded;

        if ((caseNo ?? '').isNotEmpty) {
          loaded = await repo.getAfterSaleDetail(caseNo!);
        } else if ((orderSn ?? '').isNotEmpty) {
          final list = await repo.getAfterSaleList(pageSize: 100);
          final matches = list.records.where((c) => c.orderNo == orderSn);
          if (matches.isNotEmpty && matches.first.caseNo.isNotEmpty) {
            loaded = await repo.getAfterSaleDetail(matches.first.caseNo);
          }
        }

        if (!context.mounted) return;
        detail.value = loaded;

        if ((loaded?.orderNo ?? '').isNotEmpty) {
          try {
            order.value = await ref
                .read(orderRepositoryProvider)
                .getOrderDetail(loaded!.orderNo);
          } catch (_) {
            order.value = null;
          }
        }
      } catch (_) {
        if (context.mounted) error.value = '載入失敗';
      } finally {
        if (context.mounted) loading.value = false;
      }
    }

    useEffect(() {
      load();
      return null;
    }, [caseNo, orderSn]);

    ref.listen(afterSaleStatusStreamProvider, (_, next) {
      next.whenData((event) {
        final current = detail.value;
        if (current == null) return;
        if (event.caseNo == current.caseNo ||
            event.orderNo == current.orderNo) {
          load();
        }
      });
    });

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          _buildAppBar(context),
          Expanded(
            child: loading.value
                ? const TruckLoadingOverlay(message: '載入中...')
                : error.value != null
                ? _buildErrorState(error.value!, load)
                : detail.value == null
                ? Center(
                    child: Text(
                      '尚未提出售後申請',
                      style: AppTypography.body.copyWith(
                        color: AppTypography.colorMuted,
                      ),
                    ),
                  )
                : FruitRefreshIndicator(
                    onRefresh: load,
                    child: SingleChildScrollView(
                      physics: const AlwaysScrollableScrollPhysics(),
                      padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 32.h),
                      child: _buildCard(
                        context,
                        ref,
                        detail.value!,
                        order.value,
                        load,
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildCard(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
    Order? order,
    Future<void> Function() reload,
  ) {
    final canShowLogistics = _hasReturnLogistics(c);
    final showBlackcatReturn = BlackcatReturnLogisticsCard.shouldShowFor(c);

    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(
          color: AppTypography.colorPrimary.withValues(alpha: 0.18),
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 8,
            offset: Offset(0, 3.h),
          ),
        ],
      ),
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildProgressSection(c),
            if (showBlackcatReturn) ...[
              SizedBox(height: AppSpacing.sm),
              BlackcatReturnLogisticsCard(
                caseData: c,
                pickupAddress: order?.shippingAddress ?? '',
                contactPhone: order?.recipientPhone ?? '',
                temperatureCode: order?.deliveryTemp ?? '',
              ),
            ],
            SizedBox(height: AppSpacing.md),
            _buildSectionCard(_buildBasicInfo(context, c)),
            if (_hasApplicationInfo(c)) ...[
              SizedBox(height: AppSpacing.sm),
              _buildSectionCard(_buildApplicationInfo(c)),
            ],
            if (c.shouldShow711ReturnCode) ...[
              SizedBox(height: AppSpacing.sm),
              _build711ReturnCodeBox(c),
            ],
            // PR #983 D-1 警示：寄回截止 < 24h 時提醒，逾時將解除退款資格
            if (_showShipBackUrgentBanner(c)) ...[
              SizedBox(height: AppSpacing.sm),
              _buildShipBackUrgentBanner(c),
            ],
            if (c.needRefundPage) ...[
              SizedBox(height: AppSpacing.sm),
              _buildRefundFormBanner(context, ref, c),
            ],
            if (_photoGroups(c, order).isNotEmpty) ...[
              SizedBox(height: AppSpacing.sm),
              _buildSectionCard(_buildPhotoComparison(c, order)),
            ],
            if (c.resolutions.isNotEmpty) ...[
              SizedBox(height: AppSpacing.sm),
              _buildSectionCard(_buildResolutionSection(c.resolutions)),
            ],
            if (canShowLogistics && !showBlackcatReturn) ...[
              SizedBox(height: AppSpacing.sm),
              _buildSectionCard(_buildLogisticsSection(c)),
            ],
            if (showBlackcatReturn && _hasRefundTimeline(c)) ...[
              SizedBox(height: AppSpacing.sm),
              _buildSectionCard(RefundProgressTimeline(caseData: c)),
            ],
            _buildActionButtons(context, ref, c, reload),
          ],
        ),
      ),
    );
  }

  Widget _buildProgressSection(AfterSaleCase c) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.errorOutline, '售後申請進度'),
        SizedBox(height: AppSpacing.md),
        StepIndicator(
          steps: StepIndicator.afterSaleStepsFor(c.status),
          currentStep: StepIndicator.afterSaleStepFromStatus(c.status),
          activeColor: AppColors.danger,
        ),
        SizedBox(height: AppSpacing.md),
        if (BlackcatAfterSaleStatusCard.shouldShowFor(c))
          BlackcatAfterSaleStatusCard(caseData: c)
        else
          Container(
            width: double.infinity,
            padding: EdgeInsets.symmetric(
              horizontal: AppSpacing.lg,
              vertical: 12.h,
            ),
            decoration: BoxDecoration(
              color: _statusColor(c.status).withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(12.r),
              border: Border.all(
                color: _statusColor(c.status).withValues(alpha: 0.24),
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        AfterSaleStatus.label(c.status),
                        style: AppTypography.bodyBold.copyWith(
                          color: _statusColor(c.status),
                        ),
                      ),
                    ),
                    if (c.caseNo.isNotEmpty)
                      Text(
                        c.caseNo,
                        style: AppTypography.meta.copyWith(
                          color: AppTypography.colorDisabled,
                        ),
                      ),
                  ],
                ),
                SizedBox(height: 6.h),
                Text(
                  _statusMessage(c),
                  style: AppTypography.meta.copyWith(
                    color: AppTypography.colorPrimary.withValues(alpha: 0.82),
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }

  Widget _buildBasicInfo(BuildContext context, AfterSaleCase c) {
    final rows = <Widget>[
      if (c.caseNo.isNotEmpty)
        _infoRow(AppIcons.confirmationNumberOutlined, '案件', c.caseNo),
      if (c.orderNo.isNotEmpty) _infoRow(AppIcons.receiptLong, '訂單', c.orderNo),
      if (c.createdAt.isNotEmpty)
        _infoRow(AppIcons.accessTime, '申請時間', _formatTime(c.createdAt)),
      if (c.closedAt.isNotEmpty)
        _infoRow(AppIcons.taskAltOutlined, '結案時間', _formatTime(c.closedAt)),
      if (c.refundableAmount.isNotEmpty && c.refundableAmount != '0')
        _infoRow(
          AppIcons.paymentsOutlined,
          '可退金額',
          'NT\$ ${c.refundableAmount}',
          valueColor: AppColors.warning,
          valueStyle: AppTypography.bodyBold,
        ),
      if (c.returnShippingMethod.isNotEmpty)
        _infoRow(
          AppIcons.localShippingOutlined,
          '退貨方式',
          _returnShippingMethodLabel(c.returnShippingMethod),
        ),
      if (c.returnShipmentNo.isNotEmpty)
        _infoRow(
          AppIcons.confirmationNumberOutlined,
          '退貨代碼',
          c.returnShipmentNo,
        ),
      // 物流單號只在買家「實際已寄出」之後才顯示。
      // 賣家同意退貨（status=20 awaitingReturn）後端會預配寄件單號，但在
      // 買家還沒寄之前顯示「物流單號」會讓人誤以為已寄出 — 等到 status
      // 進入 25 returnInTransit / 30 returnReceived / 40 refundProcessing /
      // 50 closed 才把它列出來，並改名「退貨物流單號」避免跟原訂單出貨混淆。
      if (c.shipTradeNo.isNotEmpty &&
          const {'25', '30', '40', '50'}.contains(c.status))
        _infoRow(AppIcons.localOfferOutlined, '退貨物流單號', c.shipTradeNo),
      if (c.canAppeal && c.appealDeadline.isNotEmpty)
        _infoRow(AppIcons.gavelOutlined, '申訴期限', _formatTime(c.appealDeadline)),
      // PR #983 待寄回 7 天倒數：賣家已同意（status=20）且後端回填 deadline 才顯示
      if (c.status == AfterSaleStatus.awaitingReturn &&
          c.shipBackDeadlineAt.isNotEmpty)
        LiveCountdownLine(
          label: '剩餘寄回時間',
          deadlineIso: c.shipBackDeadlineAt,
          expiredText: '已逾時，等待系統關案',
          urgentThreshold: const Duration(hours: 24),
        ),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.descriptionOutlined, '案件資訊'),
        SizedBox(height: AppSpacing.md),
        for (var i = 0; i < rows.length; i++) ...[
          if (i > 0) SizedBox(height: AppSpacing.sm),
          rows[i],
        ],
      ],
    );
  }

  Widget _buildApplicationInfo(AfterSaleCase c) {
    final buyerPhotos = _buyerEvidencePhotos(c);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.assignmentOutlined, '申請資訊'),
        SizedBox(height: AppSpacing.md),
        if (c.reason.isNotEmpty) ...[
          _label('退貨原因'),
          SizedBox(height: AppSpacing.xs),
          _reasonPill(AfterSaleReason.label(c.reason)),
        ],
        if (c.description.isNotEmpty) ...[
          if (c.reason.isNotEmpty) SizedBox(height: AppSpacing.md),
          _label('詳細說明'),
          SizedBox(height: AppSpacing.xs),
          _descriptionBox(c.description),
        ],
        if (buyerPhotos.isNotEmpty) ...[
          if (c.reason.isNotEmpty || c.description.isNotEmpty)
            SizedBox(height: AppSpacing.md),
          _label('上傳照片'),
          SizedBox(height: AppSpacing.xs),
          _photoStrip(buyerPhotos),
        ],
      ],
    );
  }

  Widget _buildPhotoComparison(AfterSaleCase c, Order? order) {
    final groups = _photoGroups(c, order);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.compareOutlined, '照片對照'),
        SizedBox(height: AppSpacing.md),
        for (var i = 0; i < groups.length; i++) ...[
          if (i > 0) SizedBox(height: AppSpacing.md),
          _photoGroup(groups[i]),
        ],
      ],
    );
  }

  Widget _buildResolutionSection(List<AfterSaleResolution> resolutions) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.factCheckOutlined, '處理結果'),
        SizedBox(height: AppSpacing.md),
        for (var i = 0; i < resolutions.length; i++) ...[
          if (i > 0) SizedBox(height: AppSpacing.sm),
          _resolutionCard(resolutions[i]),
        ],
      ],
    );
  }

  Widget _buildLogisticsSection(AfterSaleCase c) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.localShippingOutlined, '退貨與退款進度'),
        SizedBox(height: AppSpacing.md),
        ReturnLogisticsTimeline(caseData: c),
        if (_hasRefundTimeline(c)) ...[
          SizedBox(height: AppSpacing.sm),
          RefundProgressTimeline(caseData: c),
        ],
      ],
    );
  }

  Widget _build711ReturnCodeBox(AfterSaleCase c) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.warning.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.warning.withValues(alpha: 0.5)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(AppIcons.qrCode2, size: 20.w, color: AppIconColors.warning),
              SizedBox(width: AppSpacing.xs),
              Text(
                '7-11 寄件代碼',
                style: AppTypography.bodyBold.copyWith(
                  color: AppColors.warning,
                ),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Expanded(child: _segmentCode(c.partnerId, '母代碼')),
              SizedBox(width: AppSpacing.xs),
              Expanded(child: _segmentCode(c.odno, '出貨單號')),
              SizedBox(width: AppSpacing.xs),
              Expanded(child: _segmentCode(c.validationNo, '驗證碼')),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            '請至 7-11 ibon 輸入上方代碼列印託運單後交給門市人員',
            style: AppTypography.meta.copyWith(
              color: AppTypography.colorPrimary.withValues(alpha: 0.82),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildRefundFormBanner(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
  ) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.danger.withValues(alpha: 0.06),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.danger),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                AppIcons.priorityHigh,
                size: 20.w,
                color: AppIconColors.danger,
              ),
              SizedBox(width: AppSpacing.xs),
              Expanded(
                child: Text(
                  '請填寫退款銀行帳戶',
                  style: AppTypography.bodyBold.copyWith(
                    color: AppColors.danger,
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: 6.h),
          Text(
            'ATM 訂單退款需買家提供銀行帳戶才能完成匯款。',
            style: AppTypography.meta.copyWith(
              color: AppTypography.colorPrimary.withValues(alpha: 0.82),
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          SizedBox(
            width: double.infinity,
            height: HkControl.minTap,
            child: ElevatedButton(
              onPressed: () => _openRefundPage(context, ref, c),
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.danger,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8.r),
                ),
              ),
              child: HkButtonLabel(
                '前往填寫',
                style: AppTypography.bodyBold.copyWith(color: AppColors.bgCard),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildActionButtons(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
    Future<void> Function() reload,
  ) {
    final canCancel = c.status == AfterSaleStatus.pending;
    // PR #983：status=33（待寄回逾時自動關案）／status=60（賣家拒絕）
    // 兩種情境後端皆已開放申訴 API，appealState='0' 表示尚未申訴。
    final isAppealableStatus =
        c.status == AfterSaleStatus.autoCompletedReturnPendingTimeout ||
        c.status == AfterSaleStatus.rejected;
    final canAppeal = isAppealableStatus && c.appealState == '0' && c.canAppeal;
    // 已提交過申訴 → 顯示查看仲裁進度按鈕（status 33/60 + appealState != '0'）
    final hasAppealed =
        isAppealableStatus && c.appealState.isNotEmpty && c.appealState != '0';

    if (!canCancel && !canAppeal && !hasAppealed) {
      return const SizedBox.shrink();
    }

    if (canAppeal) {
      return Padding(
        padding: EdgeInsets.only(top: AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (c.appealDeadline.isNotEmpty)
              Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.xs),
                child: Text(
                  '申訴期限至 ${_formatTime(c.appealDeadline)}',
                  textAlign: TextAlign.center,
                  style: AppTypography.meta.copyWith(color: AppColors.gray600),
                ),
              ),
            SizedBox(
              height: HkControl.minTap,
              child: ElevatedButton(
                onPressed: () => _onSubmitAppeal(context, ref, c, reload),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primary,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(999.r),
                  ),
                ),
                child: HkButtonLabel(
                  '提交申訴',
                  style: AppTypography.bodyBold.copyWith(
                    color: AppColors.bgCard,
                  ),
                ),
              ),
            ),
          ],
        ),
      );
    }

    if (hasAppealed) {
      return Padding(
        padding: EdgeInsets.only(top: AppSpacing.md),
        child: SizedBox(
          width: double.infinity,
          height: HkControl.minTap,
          child: OutlinedButton(
            onPressed: () => _onViewArbitration(context, c),
            style: OutlinedButton.styleFrom(
              side: BorderSide(color: AppColors.primary),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(999.r),
              ),
            ),
            child: HkButtonLabel(
              '查看仲裁進度',
              style: AppTypography.bodyBold.copyWith(color: AppColors.primary),
            ),
          ),
        ),
      );
    }

    return Padding(
      padding: EdgeInsets.only(top: AppSpacing.md),
      child: SizedBox(
        width: double.infinity,
        height: HkControl.minTap,
        child: OutlinedButton(
          onPressed: () async {
            try {
              await ref
                  .read(afterSaleRepositoryProvider)
                  .cancelAfterSale(c.caseNo);
              if (!context.mounted) return;
              ref
                  .read(afterSaleOrderLookupProvider.notifier)
                  .removeOrderNo(c.orderNo);
              showFloatingToast(context, '已取消售後');
              context.pop();
            } catch (_) {
              if (context.mounted) {
                showFloatingToast(context, '取消失敗', isSuccess: false);
              }
            }
          },
          style: OutlinedButton.styleFrom(
            side: const BorderSide(color: AppColors.danger),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(999.r),
            ),
          ),
          child: HkButtonLabel(
            '取消售後',
            style: AppTypography.bodyBold.copyWith(color: AppColors.danger),
          ),
        ),
      ),
    );
  }

  /// PR #983：申訴提交入口。
  ///
  /// 後端 endpoint 為 `POST /app/after_sales/{case_no}/appeal`，前端走
  /// [SubmitAppealScreen] 表單頁。提交成功後重新載入詳情以更新 appealState。
  Future<void> _onSubmitAppeal(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
    Future<void> Function() reload,
  ) async {
    if (c.caseNo.isEmpty) return;
    await context.push(
      '/settings/after-sale/appeal',
      extra: {'caseNo': c.caseNo},
    );
    if (context.mounted) {
      reload();
    }
  }

  /// PR #983：仲裁進度查詢入口。
  void _onViewArbitration(BuildContext context, AfterSaleCase c) {
    if (c.caseNo.isEmpty) return;
    context.push(
      '/settings/after-sale/arbitration',
      extra: {'caseNo': c.caseNo},
    );
  }

  /// PR #983 D-1 警示：寄回截止 < 24h 才秀紅色 banner。
  bool _showShipBackUrgentBanner(AfterSaleCase c) {
    if (c.status != AfterSaleStatus.awaitingReturn) return false;
    if (c.shipBackDeadlineAt.isEmpty) return false;
    final dt = DateTime.tryParse(c.shipBackDeadlineAt);
    if (dt == null) return false;
    final remaining = dt.toLocal().difference(DateTime.now());
    if (remaining.isNegative) return false;
    return remaining < const Duration(hours: 24);
  }

  /// PR #983 D-1 警示 banner — 複用既有 `_buildRefundFormBanner` 樣板配色。
  Widget _buildShipBackUrgentBanner(AfterSaleCase c) {
    final dt = DateTime.tryParse(c.shipBackDeadlineAt);
    final remaining = dt == null
        ? Duration.zero
        : dt.toLocal().difference(DateTime.now());
    final hoursLeft = remaining.inHours.clamp(0, 23);

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.danger.withValues(alpha: 0.06),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.danger),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                AppIcons.priorityHigh,
                size: 20.w,
                color: AppIconColors.danger,
              ),
              SizedBox(width: AppSpacing.xs),
              Expanded(
                child: Text(
                  '剩 $hoursLeft 小時將解除退款資格',
                  style: AppTypography.bodyBold.copyWith(
                    color: AppColors.danger,
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: 6.h),
          Text(
            '請盡快寄回商品以保留退款資格',
            style: AppTypography.meta.copyWith(
              color: AppTypography.colorPrimary.withValues(alpha: 0.82),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSectionCard(Widget child) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(
          color: AppTypography.colorPrimary.withValues(alpha: 0.15),
        ),
      ),
      child: child,
    );
  }

  Widget _sectionTitle(IconData icon, String label) {
    return Row(
      children: [
        Icon(icon, size: 16.w, color: AppIconColors.defaultColor),
        SizedBox(width: AppSpacing.xs),
        Text(label, style: AppTypography.bodyBold),
      ],
    );
  }

  Widget _infoRow(
    IconData icon,
    String label,
    String value, {
    Color? valueColor,
    TextStyle? valueStyle,
  }) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 16.w, color: AppIconColors.disabled),
        SizedBox(width: AppSpacing.sm),
        SizedBox(
          width: 64.w,
          child: Text(
            label,
            style: AppTypography.meta.copyWith(
              color: AppTypography.colorDisabled,
            ),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: (valueStyle ?? AppTypography.body).copyWith(
              color: valueColor ?? AppTypography.colorPrimary,
            ),
          ),
        ),
      ],
    );
  }

  Widget _label(String text) {
    return Text(
      text,
      style: AppTypography.meta.copyWith(color: AppTypography.colorDisabled),
    );
  }

  Widget _reasonPill(String text) {
    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.sm,
        vertical: AppSpacing.xs,
      ),
      decoration: BoxDecoration(
        color: AppColors.warning.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: AppColors.warning.withValues(alpha: 0.3)),
      ),
      child: Text(
        text,
        style: AppTypography.meta.copyWith(color: AppColors.warning),
      ),
    );
  }

  Widget _descriptionBox(String text) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.sm),
      decoration: BoxDecoration(
        color: AppColors.bgPrimary,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Text(text, style: AppTypography.body),
    );
  }

  Widget _photoGroup(_PhotoGroup group) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _label(group.label),
        SizedBox(height: AppSpacing.xs),
        _photoStrip(group.urls),
      ],
    );
  }

  Widget _photoStrip(List<String> urls) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          for (var i = 0; i < urls.length; i++) ...[
            if (i > 0) SizedBox(width: AppSpacing.sm),
            _photoTile(urls[i]),
          ],
        ],
      ),
    );
  }

  Widget _photoTile(String url) {
    final size = 76.w;
    return SizedBox.square(
      dimension: size,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: AppColors.bgPrimary,
          borderRadius: BorderRadius.circular(12.r),
          border: Border.all(
            color: AppTypography.colorPrimary.withValues(alpha: 0.15),
          ),
        ),
        child: Padding(
          padding: EdgeInsets.all(4.w),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(8.r),
            child: OssImage(
              imageSource: url,
              scene: ImageScene.reviewPhoto,
              width: size,
              height: size,
              fit: BoxFit.cover,
            ),
          ),
        ),
      ),
    );
  }

  Widget _resolutionCard(AfterSaleResolution r) {
    final label =
        AfterSaleResolutionType.fromValue(r.resolutionType)?.label ??
        r.resolutionType;
    final rows = <Widget>[
      if (r.refundAmount.isNotEmpty && r.refundAmount != '0')
        _miniLine('退款金額', 'NT\$ ${r.refundAmount}'),
      if (r.couponAmount.isNotEmpty && r.couponAmount != '0')
        _miniLine('優惠券', 'NT\$ ${r.couponAmount}'),
      if (r.couponMinSpend.isNotEmpty && r.couponMinSpend != '0')
        _miniLine('優惠券門檻', 'NT\$ ${r.couponMinSpend}'),
      if (r.couponMessage.isNotEmpty) _miniLine('優惠券說明', r.couponMessage),
      if (r.rejectReason.isNotEmpty)
        _miniLine(
          '拒絕原因',
          r.rejectMessage.isNotEmpty ? r.rejectMessage : r.rejectReason,
          color: AppColors.danger,
        ),
      if (r.negotiationReason.isNotEmpty)
        _miniLine('協商原因', r.negotiationReason),
      if (r.deductionReason.isNotEmpty) _miniLine('扣款原因', r.deductionReason),
      if (r.inspectionNote.isNotEmpty) _miniLine('驗貨備註', r.inspectionNote),
      if (r.createdAt.isNotEmpty) _miniLine('處理時間', _formatTime(r.createdAt)),
    ];

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.sm),
      decoration: BoxDecoration(
        color: AppColors.bgPrimary,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (label.isNotEmpty)
            Text(
              label,
              style: AppTypography.bodyBold.copyWith(color: AppColors.primary),
            ),
          if (rows.isNotEmpty) ...[
            if (label.isNotEmpty) SizedBox(height: AppSpacing.xs),
            for (var i = 0; i < rows.length; i++) ...[
              if (i > 0) SizedBox(height: 2.h),
              rows[i],
            ],
          ],
        ],
      ),
    );
  }

  Widget _miniLine(String label, String value, {Color? color}) {
    return Text.rich(
      TextSpan(
        children: [
          TextSpan(
            text: '$label：',
            style: AppTypography.meta.copyWith(
              color: AppTypography.colorDisabled,
            ),
          ),
          TextSpan(
            text: value,
            style: AppTypography.meta.copyWith(
              color: color ?? AppTypography.colorPrimary,
            ),
          ),
        ],
      ),
    );
  }

  Widget _segmentCode(String code, String label) {
    return Column(
      children: [
        Container(
          width: double.infinity,
          padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            borderRadius: BorderRadius.circular(6.r),
            border: Border.all(color: AppColors.warning.withValues(alpha: 0.3)),
          ),
          alignment: Alignment.center,
          child: Text(
            code,
            style: AppTypography.bodyBold.copyWith(
              color: AppTypography.colorEmphasis,
              letterSpacing: 1.5,
            ),
          ),
        ),
        SizedBox(height: 2.h),
        Text(
          label,
          style: AppTypography.caption.copyWith(
            color: AppTypography.colorDisabled,
          ),
        ),
      ],
    );
  }

  Widget _buildErrorState(String msg, Future<void> Function() reload) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            msg,
            style: AppTypography.body.copyWith(color: AppColors.danger),
          ),
          SizedBox(height: AppSpacing.sm),
          OutlinedButton(
            onPressed: reload,
            style: OutlinedButton.styleFrom(
              minimumSize: const Size(0, HkControl.minTap),
            ),
            child: const HkButtonLabel('重試'),
          ),
        ],
      ),
    );
  }

  Widget _buildAppBar(BuildContext context) {
    return commonTitleTopBar(
      context,
      title: '查看售後',
      backIcon: AppIcons.chevronLeft,
      onBack: () => context.pop(),
    );
  }

  Future<void> _openRefundPage(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
  ) async {
    try {
      final url = await ref
          .read(afterSaleRepositoryProvider)
          .getRefundPageUrl(c.orderNo);
      if (!context.mounted) return;
      await context.push(
        '/refund/webview',
        extra: {'redirectUrl': url, 'orderNo': c.orderNo},
      );
    } catch (e) {
      if (context.mounted) {
        showFloatingToast(context, '載入退款頁面失敗：$e', isSuccess: false);
      }
    }
  }

  bool _hasApplicationInfo(AfterSaleCase c) {
    return c.reason.isNotEmpty ||
        c.description.isNotEmpty ||
        _buyerEvidencePhotos(c).isNotEmpty;
  }

  bool _hasReturnLogistics(AfterSaleCase c) {
    return c.status == AfterSaleStatus.awaitingReturn ||
        c.status == AfterSaleStatus.returnInTransit ||
        c.status == AfterSaleStatus.returnReceived ||
        c.status == AfterSaleStatus.refundProcessing ||
        c.status == AfterSaleStatus.closed;
  }

  bool _hasRefundTimeline(AfterSaleCase c) {
    return c.status == AfterSaleStatus.returnReceived ||
        c.status == AfterSaleStatus.refundProcessing ||
        c.status == AfterSaleStatus.closed;
  }

  String _statusMessage(AfterSaleCase c) {
    if (c.refundTaskStatus.isNotEmpty) {
      final label = RefundTaskStatus.buyerProgressLabel(
        taskStatus: c.refundTaskStatus,
        failureMessage: c.refundFailureMessage,
        estimatedRefundDate: c.estimatedRefundDate,
        bankDate: c.bankDate,
      );
      if (label.isNotEmpty) return label;
    }

    if (c.providerMessage.isNotEmpty) return c.providerMessage;

    if (c.status == AfterSaleStatus.pending && c.processDeadline.isNotEmpty) {
      return '等待賣家回覆中，處理期限 ${_formatTime(c.processDeadline)}';
    }

    return switch (c.status) {
      AfterSaleStatus.pending => '等待賣家回覆中',
      AfterSaleStatus.awaitingReturn => '賣家已同意退貨，請依退貨方式寄回商品',
      AfterSaleStatus.returnInTransit => '退貨商品運送中',
      AfterSaleStatus.returnReceived => '賣家已收到退貨，商品驗貨中',
      AfterSaleStatus.autoCompletedReturnPendingTimeout =>
        '逾時未寄回退貨商品，退款資格已解除；若有特殊情況可於申訴期內提出申訴',
      AfterSaleStatus.refundProcessing => '平台正在處理退款',
      AfterSaleStatus.closed => '售後案件已完結',
      AfterSaleStatus.rejected => '賣家已拒絕此售後申請',
      AfterSaleStatus.cancelled => '售後案件已取消',
      AfterSaleStatus.cancelledByConfirmReceipt => '您已確認收貨，售後案件已自動取消',
      _ => '處理中',
    };
  }

  List<String> _buyerEvidencePhotos(AfterSaleCase c) {
    return c.evidence
        .where((e) => _isBuyerEvidence(e) && e.imageUrl.isNotEmpty)
        .map((e) => e.imageUrl)
        .toList();
  }

  List<String> _sellerEvidencePhotos(AfterSaleCase c) {
    return c.evidence
        .where((e) => _isSellerEvidence(e) && e.imageUrl.isNotEmpty)
        .map((e) => e.imageUrl)
        .toList();
  }

  bool _isBuyerEvidence(AfterSaleEvidence e) {
    final type = e.evidenceType.trim().toUpperCase();
    return type == 'BUYER' || type == EvidenceType.buyerProof;
  }

  bool _isSellerEvidence(AfterSaleEvidence e) {
    final type = e.evidenceType.trim().toUpperCase();
    return type == 'SELLER' || type == EvidenceType.sellerInspection;
  }

  List<_PhotoGroup> _photoGroups(AfterSaleCase c, Order? order) {
    return [
      if ((order?.shipmentPhotoUrls ?? const <String>[]).isNotEmpty)
        _PhotoGroup('賣家出貨照片', order!.shipmentPhotoUrls),
      if ((order?.receiptPhotoUrls ?? const <String>[]).isNotEmpty)
        _PhotoGroup('買家收貨照片', order!.receiptPhotoUrls),
      if (c.shipmentPhotoUrls.isNotEmpty)
        _PhotoGroup('售後相關照片', c.shipmentPhotoUrls),
      if (_sellerEvidencePhotos(c).isNotEmpty)
        _PhotoGroup('賣家售後舉證', _sellerEvidencePhotos(c)),
    ];
  }

  String _returnShippingMethodLabel(String value) {
    return ShippingMethod.fromValue(value)?.label ?? value;
  }

  Color _statusColor(String status) => switch (status) {
    AfterSaleStatus.pending => AppColors.warning,
    AfterSaleStatus.awaitingReturn ||
    AfterSaleStatus.returnInTransit ||
    AfterSaleStatus.returnReceived => AppColors.primary,
    AfterSaleStatus.refundProcessing => AppColors.primaryDark,
    AfterSaleStatus.closed ||
    AfterSaleStatus.autoCompletedReturnPendingTimeout => AppColors.gray600,
    AfterSaleStatus.rejected ||
    AfterSaleStatus.cancelled ||
    AfterSaleStatus.cancelledByConfirmReceipt => AppColors.danger,
    _ => AppColors.gray500,
  };

  String _formatTime(String raw) {
    if (raw.isEmpty) return '';
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final local = dt.toLocal();
    return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }
}

class _PhotoGroup {
  const _PhotoGroup(this.label, this.urls);
  final String label;
  final List<String> urls;
}
