import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/codemeta/code_meta.dart';
import '../../../../core/codemeta/code_meta_display.dart';
import '../../../../core/codemeta/shipping_presentation.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../config/image_config.dart';
import '../../../../shared/widgets/optimized_image.dart';
import '../../../../shared/widgets/step_indicator.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../data/after_sale_evidence.dart';
import '../../data/after_sale_model.dart';
import '../../data/after_sale_repository.dart';
import '../../data/after_sale_status_notifier.dart';
import '../../../../shared/time/taipei_display.dart';

/// 賣家訂單卡片「已送達」狀態下的售後展開區。
///
/// 從 [sellerAfterSaleLookupProvider] 拿到的 [AfterSaleCase] 作為摘要顯示；
/// 同時用 caseNo 向後端拉取完整 detail 補齊 resolutions / 物流 / 照片細節。
/// FCM Silent Push 同案件狀態變更自動刷新。
class AfterSaleExpandedSection extends HookConsumerWidget {
  const AfterSaleExpandedSection({super.key, required this.summary});

  /// 列表摘要資料（來自 sellerAfterSaleLookupProvider），提供 caseNo / status
  /// 做首屏渲染；詳細資料仍以 detail API 為準。
  final AfterSaleCase summary;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final detail = useState<AfterSaleCase?>(null);
    final loading = useState(true);
    final error = useState<String?>(null);

    Future<void> load() async {
      if (summary.caseNo.isEmpty) {
        loading.value = false;
        return;
      }
      loading.value = true;
      error.value = null;
      try {
        final result = await ref
            .read(afterSaleRepositoryProvider)
            .getAfterSaleDetail(summary.caseNo);
        if (context.mounted) detail.value = result;
      } catch (_) {
        if (context.mounted) error.value = '載入失敗';
      } finally {
        if (context.mounted) loading.value = false;
      }
    }

    useEffect(() {
      load();
      return null;
    }, [summary.caseNo, summary.status]);

    ref.listen(afterSaleStatusStreamProvider, (_, next) {
      next.whenData((event) {
        final current = detail.value ?? summary;
        if (event.refreshesAll ||
            event.caseNo == current.caseNo ||
            event.orderNo == current.orderNo) {
          load();
        }
      });
    });

    return Column(
      children: [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 20.w),
          child: Divider(
            color: AppTypography.colorPrimary.withValues(alpha: 0.2),
            height: 1,
          ),
        ),
        Padding(
          padding: EdgeInsets.fromLTRB(20.w, 16.h, 20.w, 12.h),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildHeader(),
              SizedBox(height: 12.h),
              _buildStepIndicator(),
              SizedBox(height: 12.h),
              _buildSummaryCard(),
              if (loading.value) ...[
                SizedBox(height: 12.h),
                const Center(
                  child: Padding(
                    padding: EdgeInsets.all(12),
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: AppColors.error,
                    ),
                  ),
                ),
              ] else if (error.value != null) ...[
                SizedBox(height: 12.h),
                _buildErrorState(error.value!, load),
              ] else if (detail.value != null) ...[
                SizedBox(height: 12.h),
                _buildDetailContent(context, detail.value!),
              ],
            ],
          ),
        ),
      ],
    );
  }


  Widget _buildHeader() {
    return Row(
      children: [
        HkIcon(HkIconName.swap, size: 20.w, color: AppIconColors.defaultColor),
        SizedBox(width: 4.w),
        Text('售後流程', style: AppTypography.bodyBold),
      ],
    );
  }

  Widget _buildStepIndicator() {
    final currentStep = StepIndicator.sellerAfterSaleStepFromStatus(
      summary.status,
    );
    return StepIndicator(
      steps: StepIndicator.sellerAfterSaleSteps,
      currentStep: currentStep,
      activeColor: AppColors.error,
    );
  }

  Widget _buildSummaryCard() {
    final providerMsg = summary.providerMessage.trim();
    String fallback(String f) => providerMsg.isNotEmpty ? providerMsg : f;

    final underAppeal = AfterSaleStatus.isUnderAppeal(summary.appealState);
    final statusDesc = underAppeal
        ? '案件已進入平台申訴，等待平台裁決'
        : switch (summary.status) {
            '0' => '等待賣家回覆中，請於期限內確認處理',
            '20' => '已同意退貨退款，等待買家寄回商品',
            '25' => fallback('退貨商品運送中'),
            '26' => fallback('退貨包裹已到門市，等待賣家取貨'),
            '27' => fallback('退貨已送達，等待賣家確認收貨'),
            '28' => fallback('退貨物流失敗，需 Admin 介入或申訴'),
            '30' => '已確認收貨，準備啟動退款',
            '31' => '超商待取逾時，系統已自動完成確認收貨',
            '32' => '確認收貨逾時，系統已自動完成確認收貨',
            '33' => '待寄回逾時自動關案，款項照常撥付',
            '40' => '退款處理中，預計 1-3 個工作天入帳',
            '50' => '售後案件已完結',
            '60' => '已婉拒，買家可重新申請或聯繫客服',
            '71' => '買家確認收貨，售後案件已自動取消',
            _ => fallback('處理中'),
          };

    final statusLabel = AfterSaleStatus.displayLabel(
      summary.status,
      summary.appealState,
    );

    return Container(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray1_30),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Flexible(
                child: Text(
                  statusLabel,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.body.copyWith(
                    color: AppTypography.colorEmphasis,
                  ),
                ),
              ),
              Flexible(
                child: Text(
                  summary.caseNo,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.meta.copyWith(color: AppColors.gray1_50),
                  textAlign: TextAlign.right,
                ),
              ),
            ],
          ),
          SizedBox(height: 6.h),
          Text(
            statusDesc,
            style: AppTypography.meta.copyWith(color: AppColors.textSecondary),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorState(String msg, Future<void> Function() reload) {
    return Center(
      child: Column(
        children: [
          Text(msg, style: AppTypography.meta.copyWith(color: AppColors.error)),
          SizedBox(height: 4.h),
          OutlinedButton(onPressed: reload, child: const HkButtonLabel('重試')),
        ],
      ),
    );
  }


  Widget _buildDetailContent(BuildContext context, AfterSaleCase c) {
    final shipTradeNo = _text(c.shipTradeNo);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (_isReturnFlow(c)) ...[
          _buildReturnLogisticsProgress(context, c),
          SizedBox(height: 10.h),
        ],
        _buildBasicInfo(context, c),
        SizedBox(height: 10.h),
        _buildReasonSection(c),
        if (c.resolutions.isNotEmpty) ...[
          SizedBox(height: 10.h),
          _buildResolutionsCard(c),
        ],
        if (c.returnShipmentNo.isNotEmpty ||
            shipTradeNo.isNotEmpty ||
            c.returnShippingMethod.isNotEmpty) ...[
          SizedBox(height: 10.h),
          _buildReturnShipmentSection(context, c),
        ],
      ],
    );
  }

  /// 共用分組卡容器：白底 + 1px 淡邊框 + 12.r 圓角 + 12w/10h padding，
  /// 對齊 OrderCard 主體的視覺語言（截圖風格）。
  Widget _groupCard({required Widget child}) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray1_20),
      ),
      child: child,
    );
  }

  /// 卡內行標題（icon + 文字），用於分組卡的 header
  Widget _cardHeader(HkIconName icon, String title) {
    return Row(
      children: [
        HkIcon(icon, size: 18.w, color: AppIconColors.defaultColor),
        SizedBox(width: 6.w),
        Text(title, style: AppTypography.bodyBold),
      ],
    );
  }

  /// 標籤 + 值，左右排佈（截圖「時間 + 倒計時」樣式）
  Widget _infoRow(String label, String value, {Color? valueColor}) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: 4.h),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 72.w,
            child: Text(
              label,
              style: AppTypography.meta.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: AppTypography.meta.copyWith(
                color: valueColor ?? AppTypography.colorPrimary,
                fontWeight: valueColor != null
                    ? FontWeight.w700
                    : FontWeight.w400,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBasicInfo(BuildContext context, AfterSaleCase c) {
    final hasRefund =
        c.refundableAmount.isNotEmpty && c.refundableAmount != '0';
    return _groupCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _cardHeader(HkIconName.receipt, '基本資訊'),
          SizedBox(height: 8.h),
          _infoRow('訂單編號', c.orderNo),
          _infoRow('申請時間', _formatTime(c.createdAt, context.taipeiDisplay)),
          if (hasRefund)
            _infoRow(
              '可退金額',
              'NT\$ ${c.refundableAmount}',
              valueColor: AppColors.primary,
            ),
        ],
      ),
    );
  }

  Widget _buildReasonSection(AfterSaleCase c) {
    final reasonLabel = AfterSaleReason.label(c.reason);
    final issuePhotos = buyerIssueEvidenceImageKeys(c.evidence);
    return _groupCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _cardHeader(HkIconName.document, '申請資訊'),
          SizedBox(height: 8.h),
          Row(
            children: [
              Text(
                '退貨原因',
                style: AppTypography.meta.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
              SizedBox(width: 8.w),
              Container(
                padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 4.h),
                decoration: BoxDecoration(
                  color: AppColors.warning.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text(
                  reasonLabel.isNotEmpty ? reasonLabel : '-',
                  style: AppTypography.meta.copyWith(
                    color: AppColors.warningText,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: 10.h),
          Text(
            '詳細說明',
            style: AppTypography.meta.copyWith(color: AppColors.textSecondary),
          ),
          SizedBox(height: 4.h),
          Container(
            width: double.infinity,
            padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
            decoration: BoxDecoration(
              color: AppColors.bgApp,
              borderRadius: BorderRadius.circular(8.r),
            ),
            child: Text(
              c.description.isNotEmpty ? c.description : '-',
              style: AppTypography.meta.copyWith(
                color: AppTypography.colorPrimary,
              ),
            ),
          ),
          if (issuePhotos.isNotEmpty) ...[
            SizedBox(height: 10.h),
            Text(
              '上傳的照片',
              style: AppTypography.meta.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
            SizedBox(height: 6.h),
            Wrap(
              spacing: 8.w,
              runSpacing: 8.h,
              children: [
                for (final url in issuePhotos.take(3))
                  _PhotoThumb(url: url, size: 72.w),
              ],
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildReturnLogisticsProgress(BuildContext context, AfterSaleCase c) {
    final status = _returnLogisticsStatus(context, c);
    return _groupCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _cardHeader(HkIconName.truck, '退貨物流進度'),
          SizedBox(height: 10.h),
          _InlineStepProgress(
            steps: const [
              _ProgressStep(label: '取貨中', icon: HkIconName.check),
              _ProgressStep(label: '配送中', icon: HkIconName.notification),
              _ProgressStep(label: '已送達', icon: HkIconName.truck),
              _ProgressStep(label: '驗貨完成', icon: HkIconName.check),
            ],
            currentStep: status.step,
            activeColor: AppColors.warning,
          ),
          SizedBox(height: 12.h),
          _processCard(
            title: status.title,
            time: _returnStatusTime(context, c),
            message: status.message,
          ),
        ],
      ),
    );
  }

  Widget _buildResolutionsCard(AfterSaleCase c) {
    return _groupCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _cardHeader(HkIconName.history, '處理紀錄'),
          SizedBox(height: 8.h),
          for (int i = 0; i < c.resolutions.length; i++) ...[
            if (i > 0) SizedBox(height: 8.h),
            _buildResolution(c.resolutions[i]),
          ],
        ],
      ),
    );
  }

  Widget _buildResolution(AfterSaleResolution r) {
    final typeLabel = CodeMetaDisplay.afterSaleResolutionType(r.resolutionType);
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
      decoration: BoxDecoration(
        color: AppColors.bgApp,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
                decoration: BoxDecoration(
                  color: AppColors.primaryDark.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text(
                  typeLabel,
                  style: AppTypography.meta.copyWith(
                    color: AppColors.primaryDark,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ],
          ),
          if (r.refundAmount.isNotEmpty && r.refundAmount != '0') ...[
            SizedBox(height: 6.h),
            _infoRow(
              '退款金額',
              'NT\$ ${r.refundAmount}',
              valueColor: AppColors.primary,
            ),
          ],
          if (r.couponAmount.isNotEmpty && r.couponAmount != '0')
            _infoRow('優惠券', 'NT\$ ${r.couponAmount}'),
          if (r.rejectReason.isNotEmpty) ...[
            SizedBox(height: 6.h),
            _infoRow(
              '拒絕原因',
              r.rejectMessage.isNotEmpty ? r.rejectMessage : r.rejectReason,
              valueColor: AppColors.error,
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildReturnShipmentSection(BuildContext context, AfterSaleCase c) {
    final returnShipping = shippingPresentationOf(c.returnShippingMethod);
    final shipTradeNo = _text(c.shipTradeNo);
    final returnShipmentNo = _text(c.returnShipmentNo);
    final providerStatus = _text(c.providerStatus);
    final providerMessage = _text(c.providerMessage);

    final (primaryNo, primaryLabel) = switch (returnShipping) {
      ShippingPresentation.cvsStorePickup => (returnShipmentNo, '7-11 退貨代碼'),
      ShippingPresentation.homeDelivery => (
        shipTradeNo.isNotEmpty ? shipTradeNo : returnShipmentNo,
        '退貨物流單',
      ),
      ShippingPresentation.unsupported => (
        shipTradeNo.isNotEmpty ? shipTradeNo : returnShipmentNo,
        '退貨單號',
      ),
    };
    final showSecondaryTradeNo =
        returnShipping == ShippingPresentation.cvsStorePickup &&
        shipTradeNo.isNotEmpty &&
        shipTradeNo != returnShipmentNo;

    final methodLabel = c.returnShippingMethod.isNotEmpty
        ? CodeMetaDisplay.shippingMethod(c.returnShippingMethod)
        : '';
    return _groupCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _cardHeader(HkIconName.truck, '退貨物流'),
          SizedBox(height: 8.h),
          if (methodLabel.isNotEmpty) _infoRow('退貨方式', methodLabel),
          if (providerStatus.isNotEmpty || providerMessage.isNotEmpty)
            _infoRow('物流狀態', _providerStatusText(c)),
          if (primaryNo.isNotEmpty) ...[
            SizedBox(height: 4.h),
            Text(
              primaryLabel,
              style: AppTypography.meta.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
            SizedBox(height: 4.h),
            Container(
              width: double.infinity,
              padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
              decoration: BoxDecoration(
                color: AppColors.bgApp,
                borderRadius: BorderRadius.circular(8.r),
              ),
              child: Text(
                primaryNo,
                style: AppTypography.bodyBold.copyWith(
                  color: AppColors.primaryDark,
                  letterSpacing: 0.5,
                ),
              ),
            ),
          ],
          if (showSecondaryTradeNo) ...[
            SizedBox(height: 8.h),
            _infoRow('退貨物流單號', shipTradeNo),
          ],
        ],
      ),
    );
  }

  Widget _processCard({
    required String title,
    required String time,
    required String message,
  }) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray1_20),
      ),
      child: Column(
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  title,
                  style: AppTypography.bodyBold.copyWith(
                    color: AppTypography.colorEmphasis,
                  ),
                ),
              ),
              if (time.isNotEmpty)
                Text(
                  time,
                  style: AppTypography.meta.copyWith(color: AppColors.gray1_50),
                ),
            ],
          ),
          if (message.isNotEmpty) ...[
            SizedBox(height: 6.h),
            Align(
              alignment: Alignment.centerLeft,
              child: Text(
                message,
                style: AppTypography.meta.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  bool _isReturnFlow(AfterSaleCase c) {
    final shipTradeNo = _text(c.shipTradeNo);
    if (c.returnShippingMethod.isNotEmpty ||
        c.returnShipmentNo.isNotEmpty ||
        shipTradeNo.isNotEmpty) {
      return true;
    }
    if (c.status == AfterSaleStatus.awaitingReturn ||
        c.status == AfterSaleStatus.returnInTransit ||
        AfterSaleStatus.returnCompletedSet.contains(c.status)) {
      return true;
    }
    return c.resolutions.any(
      (r) => r.resolutionType == AfterSaleResolutionType.returnRefund.value,
    );
  }

  _ReturnLogisticsStatus _returnLogisticsStatus(
    BuildContext context,
    AfterSaleCase c,
  ) {
    final code = _text(c.providerStatus);

    final refundTaskStatus = _text(c.refundTaskStatus);
    final refundStatus = refundTaskStatus.toUpperCase();
    final refundFailureMessage = _text(c.refundFailureMessage);
    if (c.status == AfterSaleStatus.closed) {
      return const _ReturnLogisticsStatus(
        step: 3,
        title: '驗貨完成',
        message: '退款已完成，售後案件結案',
      );
    }
    if (c.status == AfterSaleStatus.refundProcessing ||
        refundTaskStatus.isNotEmpty) {
      final msg = switch (refundStatus) {
        'FAILED' when refundFailureMessage.isNotEmpty =>
          '退款失敗：$refundFailureMessage',
        'FAILED' => '退款失敗，請聯繫客服處理',
        'MANUAL_INTERVENTION' => '退款需人工處理，請聯繫客服',
        'PENDING' => '退款排隊中',
        _ => '已完成驗貨，系統將依確認金額執行退款',
      };
      return _ReturnLogisticsStatus(step: 3, title: '驗貨完成', message: msg);
    }
    if (AfterSaleStatus.isAutoCompleted(c.status)) {
      return _ReturnLogisticsStatus(
        step: 3,
        title: '驗貨完成',
        message: '${AfterSaleStatus.label(c.status)}，系統將依確認金額執行退款',
      );
    }
    if (c.status == AfterSaleStatus.returnReceived) {
      return const _ReturnLogisticsStatus(
        step: 2,
        title: '已送達',
        message: '退貨包裹已送達，請確認收貨後觸發退款',
      );
    }

    switch (code) {
      case ReturnShipmentStatus.logisticsCheck:
        return const _ReturnLogisticsStatus(
          step: 1,
          title: '物流驗收中',
          message: '物流中心驗收退貨包裹中',
        );
      case ReturnShipmentStatus.inTransit:
        return const _ReturnLogisticsStatus(
          step: 1,
          title: '配送中',
          message: '物流商已從超商收件，運送中',
        );
      case ReturnShipmentStatus.returnGeneral:
      case ReturnShipmentStatus.returnByHomeDelivery:
      case ReturnShipmentStatus.tcatCalled:
        return const _ReturnLogisticsStatus(
          step: 1,
          title: '退貨運送中',
          message: '退貨包裹運送中',
        );
      case ReturnShipmentStatus.returnToWarehouse:
        return const _ReturnLogisticsStatus(
          step: 1,
          title: '退回物流中心',
          message: '退貨包裹回到物流中心，將續轉至您的取件門市',
        );

      case ReturnShipmentStatus.awaitingPickup:
        return const _ReturnLogisticsStatus(
          step: 2,
          title: '已送達',
          message: '退貨包裹已送達您的取件超商，請盡快前往取件',
        );
      case ReturnShipmentStatus.pickedUp:
        return const _ReturnLogisticsStatus(
          step: 2,
          title: '已取貨',
          message: '您已取件完成，請確認收貨後觸發退款',
        );

      case ReturnShipmentStatus.abnormal:
        return const _ReturnLogisticsStatus(
          step: 0,
          title: '物流異常',
          message: '退貨物流發生異常，請聯絡客服協助處理',
        );
      case ReturnShipmentStatus.cancelled:
        return const _ReturnLogisticsStatus(
          step: 0,
          title: '物流已取消',
          message: '退貨物流已取消',
        );
      case ReturnShipmentStatus.packageLost:
        return const _ReturnLogisticsStatus(
          step: 0,
          title: '包裹遺失',
          message: '退貨包裹遺失，請聯絡客服協助處理',
        );
      case ReturnShipmentStatus.sellerNotPicked:
        return const _ReturnLogisticsStatus(
          step: 2,
          title: '逾期未取',
          message: '退貨已配達門市但您未在期限內領取，請聯絡客服',
        );
      case ReturnShipmentStatus.pendingHomeReturn:
        return const _ReturnLogisticsStatus(
          step: 1,
          title: '待轉宅配',
          message: '逾期未取，物流中心將安排宅配退回',
        );
    }

    if (c.status == AfterSaleStatus.returnInTransit) {
      return const _ReturnLogisticsStatus(
        step: 0,
        title: '等待買家寄回',
        message: '已建立退貨物流，等待買家交付商品至超商',
      );
    }

    return _ReturnLogisticsStatus(
      step: 0,
      title: '等待買家寄回',
      message: switch (shippingPresentationOf(c.returnShippingMethod)) {
        ShippingPresentation.homeDelivery => '已建立黑貓退貨物流，等待買家交付退貨商品',
        ShippingPresentation.cvsStorePickup => '已建立退貨代碼，等待買家寄回商品',
        ShippingPresentation.unsupported => '已建立退貨物流，等待買家寄回商品',
      },
    );
  }

  String _returnStatusTime(BuildContext context, AfterSaleCase c) {
    for (final r in c.resolutions.reversed) {
      if (r.createdAt.isNotEmpty) {
        return _formatTime(r.createdAt, context.taipeiDisplay);
      }
    }
    return _formatTime(c.createdAt, context.taipeiDisplay);
  }

  String _providerStatusText(AfterSaleCase c) {
    final providerStatus = _text(c.providerStatus);
    final providerMessage = _text(c.providerMessage);
    final label = ShippingStatus.fromValue(providerStatus)?.label;
    if (providerMessage.isNotEmpty &&
        label != null &&
        label.isNotEmpty &&
        providerMessage.contains(label)) {
      return providerMessage;
    }
    final parts = <String>[
      if (label != null && label.isNotEmpty) label,
      if (providerMessage.isNotEmpty) providerMessage,
    ];
    if (parts.isNotEmpty) return parts.join('・');
    return providerStatus;
  }

  String _text(String? value) => value?.trim() ?? '';

  String _formatTime(String raw, TaipeiDisplay t) {
    return t.formatIso(raw);
  }
}

class _ReturnLogisticsStatus {
  const _ReturnLogisticsStatus({
    required this.step,
    required this.title,
    required this.message,
  });

  final int step;
  final String title;
  final String message;
}

class _ProgressStep {
  const _ProgressStep({required this.label, required this.icon});
  final String label;
  final HkIconName icon;
}

class _InlineStepProgress extends StatelessWidget {
  const _InlineStepProgress({
    required this.steps,
    required this.currentStep,
    required this.activeColor,
  });

  final List<_ProgressStep> steps;
  final int currentStep;
  final Color activeColor;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final lineWidth =
            ((constraints.maxWidth - steps.length * 54.w) / (steps.length - 1))
                .clamp(12.0, double.infinity);
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (var i = 0; i < steps.length; i++) ...[
              SizedBox(
                width: 54.w,
                child: Column(
                  children: [
                    Container(
                      width: 34.w,
                      height: 34.w,
                      decoration: BoxDecoration(
                        color: i <= currentStep
                            ? activeColor
                            : AppColors.gray1_30,
                        shape: BoxShape.circle,
                      ),
                      alignment: Alignment.center,
                      child: HkIcon(
                        steps[i].icon,
                        size: 18.sp,
                        color: AppIconColors.onBrand,
                      ),
                    ),
                    SizedBox(height: 5.h),
                    Text(
                      steps[i].label,
                      textAlign: TextAlign.center,
                      maxLines: 1,
                      style: AppTypography.meta.copyWith(
                        color: i <= currentStep
                            ? AppTypography.colorPrimary
                            : AppColors.textDisabled,
                      ),
                    ),
                  ],
                ),
              ),
              if (i < steps.length - 1)
                Container(
                  width: lineWidth,
                  height: 3.h,
                  margin: EdgeInsets.only(top: 16.h),
                  decoration: BoxDecoration(
                    color: i < currentStep ? activeColor : AppColors.gray1_30,
                    borderRadius: BorderRadius.circular(999),
                  ),
                ),
            ],
          ],
        );
      },
    );
  }
}

class _PhotoThumb extends StatelessWidget {
  const _PhotoThumb({required this.url, required this.size});

  final String url;
  final double size;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: AppColors.gray100,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray1_20),
      ),
      clipBehavior: Clip.antiAlias,
      child: OptimizedNetworkImage(
        imageUrl: url,
        scene: ImageScene.reviewPhoto,
        width: size,
        height: size,
        fit: BoxFit.cover,
        errorWidget: Icon(
          AppIcons.brokenImageOutlined,
          size: 24.sp,
          color: AppIconColors.disabled,
        ),
      ),
    );
  }
}
