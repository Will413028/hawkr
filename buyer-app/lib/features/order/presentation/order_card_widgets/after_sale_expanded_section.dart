import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/core/codemeta/shipping_presentation.dart';
import '../../../../config/image_config.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../../../shared/widgets/step_indicator.dart';
import '../../data/after_sale_model.dart';
import '../../data/after_sale_order_lookup.dart';
import '../../data/after_sale_repository.dart';
import '../../data/after_sale_status_notifier.dart';
import '../../data/order_model.dart';
import '../widgets/return_logistics_timeline.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 售後案件展開區塊 — 對應 Figma 10092:103928 / 10103:93051
///
/// 展開後顯示完整售後詳情：
/// - 4 步驟進度指示器 + 狀態資訊卡（摘要）
/// - 案件基本資訊（案件編號 / 訂單編號 / 時間 / 可退金額）
/// - 退貨原因 + 詳細說明 + 照片
/// - 處理結果（resolutions）
/// - 物流進度（退貨 timeline + 退款 timeline）
/// - 操作按鈕（取消售後 / 重新申請）
///
/// 資料由 caseNo 調用 [afterSaleRepositoryProvider.getAfterSaleDetail] 異步載入，
/// 並訂閱 [afterSaleStatusStreamProvider] 實現 FCM Silent Push 自動刷新。
class AfterSaleExpandedSection extends HookConsumerWidget {
  const AfterSaleExpandedSection({
    super.key,
    required this.snapshot,
    this.order,
  });
  final AfterSaleCaseSnapshot snapshot;
  final Order? order;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final detail = useState<AfterSaleCase?>(null);
    final loading = useState(true);
    final error = useState<String?>(null);

    // 監聽父層 lookup provider — 父層 invalidate / refresh 時
    // lookupAsync 實例會變化，觸發下方 useEffect 重新 load()
    final lookupAsync = ref.watch(afterSaleOrderLookupProvider);

    Future<void> load() async {
      if (snapshot.caseNo.isEmpty) {
        loading.value = false;
        return;
      }
      loading.value = true;
      error.value = null;
      try {
        final result = await ref
            .read(afterSaleRepositoryProvider)
            .getAfterSaleDetail(snapshot.caseNo);
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
    }, [snapshot.caseNo, lookupAsync]);

    // FCM silent push: 本案件狀態變更自動刷新
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

    return Column(
      children: [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg),
          child: Divider(
            color: AppTypography.colorPrimary.withValues(alpha: 0.2),
            height: 1,
          ),
        ),
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.md,
            AppSpacing.lg,
            12.h,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildHeader(),
              SizedBox(height: AppSpacing.md),
              _buildStepIndicator(),
              SizedBox(height: AppSpacing.md),
              _buildSummaryCard(detail: detail.value),
              if (loading.value) ...[
                SizedBox(height: AppSpacing.md),
                const Center(
                  child: Padding(
                    padding: EdgeInsets.all(12),
                    child: HkBrandLoader(
                      size: HkBrandLoader.inline,
                      tone: HkBrandLoaderTone.mono,
                      color: AppColors.danger,
                    ),
                  ),
                ),
              ] else if (error.value != null) ...[
                SizedBox(height: AppSpacing.md),
                _buildErrorState(error.value!, load),
              ] else if (detail.value != null) ...[
                SizedBox(height: AppSpacing.md),
                _buildDetailContent(context, ref, detail.value!, load),
              ],
            ],
          ),
        ),
      ],
    );
  }

  // ─── 摘要區（不依賴詳情數據）────────────────────────

  Widget _buildHeader() {
    return Row(
      children: [
        Icon(
          AppIcons.errorOutline,
          size: 20.w,
          color: AppIconColors.defaultColor,
        ),
        SizedBox(width: AppSpacing.xs),
        Text('售後申請進度', style: AppTypography.bodyBold),
      ],
    );
  }

  // 步驟指示器
  Widget _buildStepIndicator() {
    return StepIndicator(
      steps: StepIndicator.afterSaleStepsFor(snapshot.status),
      currentStep: StepIndicator.afterSaleStepFromStatus(snapshot.status),
      activeColor: AppColors.danger,
    );
  }

  // 摘要卡片
  ///
  /// 文案優先級（B-114 動態文案 > status-based 靜態文案）：
  /// 1. detail.refundTaskStatus 非空 → 用 [RefundTaskStatus.buyerProgressLabel]
  ///    回傳的動態文案（含失敗重試 / 銀行處理中預計日期 / 入帳完成日期等）
  /// 2. fallback 到 status 對應的靜態文案
  Widget _buildSummaryCard({AfterSaleCase? detail}) {
    if (detail != null && BlackcatAfterSaleStatusCard.shouldShowFor(detail)) {
      return BlackcatAfterSaleStatusCard(caseData: detail);
    }

    // 嘗試取動態文案（僅當後端回傳 refundTaskStatus 時）
    String? dynamicDesc;
    if (detail != null && detail.refundTaskStatus.isNotEmpty) {
      final label = RefundTaskStatus.buyerProgressLabel(
        taskStatus: detail.refundTaskStatus,
        failureMessage: detail.refundFailureMessage,
        estimatedRefundDate: detail.estimatedRefundDate,
        bankDate: detail.bankDate,
      );
      if (label.isNotEmpty) dynamicDesc = label;
    }

    // 申訴 / 平台仲裁中覆寫文案（與詳情頁仲裁 banner 一致），優先於 status 靜態文案。
    final appealDesc = snapshot.isUnderAppeal ? '申訴中，等待平台裁決' : null;
    final statusDesc =
        dynamicDesc ??
        appealDesc ??
        switch (snapshot.status) {
          // 後端 #1241：REOPENED(65) 已淘汰（被拒後改走平台申訴），僅剩 0=待回覆
          '0' => '等待賣家回覆中，預計 1-3 個工作天',
          '20' => '請填寫退款銀行資訊',
          '25' => '退貨商品運送中',
          '30' => '賣家已收到退貨，退款處理中',
          // PR #983：待寄回逾時自動關案（status=33）
          '33' => '待寄回逾時自動關案，款項照常撥付',
          '40' => '平台正在處理退款，1-3 個工作天內入帳',
          '50' => '售後案件已完結',
          '60' => '收貨有誤，賣家拒絕退費',
          '70' => '售後案件已取消',
          '71' => '買家確認收貨，售後案件已自動取消',
          _ => '處理中',
        };

    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12.h),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(
          color: AppTypography.colorPrimary.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                snapshot.statusLabel,
                style: AppTypography.bodyBold.copyWith(
                  color: AppColors.textEmphasis,
                ),
              ),
              Text(
                snapshot.caseNo,
                style: AppTypography.meta.copyWith(
                  color: AppTypography.colorDisabled,
                ),
                textAlign: TextAlign.right,
              ),
            ],
          ),
          SizedBox(height: 6.h),
          Text(
            statusDesc,
            style: AppTypography.meta.copyWith(
              color: AppTypography.colorPrimary.withValues(alpha: 0.8),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorState(String msg, Future<void> Function() reload) {
    return Center(
      child: Column(
        children: [
          Text(
            msg,
            style: AppTypography.meta.copyWith(color: AppColors.danger),
          ),
          SizedBox(height: AppSpacing.xs),
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

  // ─── 詳情區（異步載入完成後顯示）──────────────────

  Widget _buildDetailContent(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
    Future<void> Function() reload,
  ) {
    /// 是否有物流追蹤 — 只有「退貨退款」（caseResolutionType=1）才會走實體寄回流程；
    /// 部分退款 (2) / 全額退款免退貨 (3) 沒有物流，不能秀「物流進度」誤導買家。
    /// caseResolutionType 為空（案件尚未決議）也保守不顯示。
    final hasLogistics =
        c.caseResolutionType == '1' &&
        (c.status == '20' ||
            c.status == '25' ||
            c.status == '40' ||
            c.status == '50');
    final showBlackcatReturn = BlackcatReturnLogisticsCard.shouldShowFor(c);

    if (showBlackcatReturn) {
      // 黑貓宅配退貨：照舊渲染 BlackcatReturnLogisticsCard，但在它「上方」加上
      // 跟一般物流進度同款的水平步驟條，讓 UI 一致（買家先看到 4 步進度，
      // 再看到取件地址 / 預約出車 / 收件流程明細）。
      // 黑貓宅配進入此 path 隱含 caseResolutionType=1，故步驟條無需額外條件。
      final steps = ['宅配取貨', '配送中', '已送達', '驗貨完成'];
      final currentStep = _returnLogisticsCurrentStep(c);
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _infoCard(
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _sectionTitle(AppIcons.localShipping, '物流進度'),
                SizedBox(height: AppSpacing.md),
                _buildProgressSteps(steps, currentStep),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          BlackcatReturnLogisticsCard(
            caseData: c,
            pickupAddress: order?.shippingAddress ?? '',
            contactPhone: order?.recipientPhone ?? '',
            temperatureCode: order?.deliveryTemp ?? '',
          ),
          if (c.needRefundPage) ...[
            SizedBox(height: AppSpacing.sm),
            _buildRefundFormBanner(context, ref, c),
          ],
          _buildActionButtons(context, ref, c),
        ],
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 1. 案件基本資訊（卡片化）
        _infoCard(_buildBasicInfo(context, c)),
        // 2. B-93 7-11 三段碼（高優先級，條件渲染）
        if (c.shouldShow711ReturnCode) ...[
          SizedBox(height: AppSpacing.sm),
          _build711ReturnCodeBox(c),
        ],
        // 3. B-104 ATM 退款填表（高優先級，條件渲染）
        if (c.needRefundPage) ...[
          SizedBox(height: AppSpacing.sm),
          _buildRefundFormBanner(context, ref, c),
        ],
        // 4. 申請原因 + 說明 + 照片
        SizedBox(height: AppSpacing.sm),
        _infoCard(_buildReasonSection(c)),
        // 5. 物流進度（條件渲染）
        if (hasLogistics) ...[
          SizedBox(height: AppSpacing.sm),
          _infoCard(_buildLogisticsSection(c)),
        ],
        _buildActionButtons(context, ref, c),
      ],
    );
  }

  /// 統一卡片包裝 — 白底 + 淡灰邊 + 圓角 10
  ///
  /// 所有 detail sections 用此工具統一視覺層級，取代舊的 `_sectionDivider`
  /// 扁平化分組方式。
  Widget _infoCard(Widget child) {
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

  /// 區塊標題 — icon + 文字
  Widget _sectionTitle(IconData icon, String label) {
    return Row(
      children: [
        Icon(icon, size: 16.w, color: AppIconColors.defaultColor),
        SizedBox(width: AppSpacing.xs),
        Text(label, style: AppTypography.bodyBold),
      ],
    );
  }

  // ─── B-93 7-11 三段碼展示區 ────────────────────
  ///
  /// 顯示條件：resolutionType=同意退貨 + returnShippingMethod=7-11 + status≥20
  /// （由 [AfterSale711CodeX.shouldShow711ReturnCode] 判斷）
  Widget _build711ReturnCodeBox(AfterSaleCase c) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.warning.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(
          color: AppColors.warning.withValues(alpha: 0.5),
          width: 1,
        ),
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
          // 三段碼橫向展示
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
              color: AppTypography.colorPrimary.withValues(alpha: 0.8),
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
            code.isNotEmpty ? code : '-',
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

  // ─── B-104 ATM 退款填表 Banner ────────────────
  ///
  /// 顯示條件：case.needRefundPage=true（ATM 退款需買家提供銀行帳戶）
  /// 點擊呼叫 getRefundPageUrl 取 PayUni 填單 URL，推 webview
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
        border: Border.all(color: AppColors.danger, width: 1),
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
              color: AppTypography.colorPrimary.withValues(alpha: 0.8),
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

  // ─── 1. 案件基本資訊 ──────────────────────────

  Widget _buildBasicInfo(BuildContext context, AfterSaleCase c) {
    final hasRefund =
        c.refundableAmount.isNotEmpty && c.refundableAmount != '0';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 訂單編號行：icon + 單號（可複製）+ 右側可退金額 chip
        Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Icon(
              AppIcons.receiptLong,
              size: 16.w,
              color: AppIconColors.defaultColor,
            ),
            SizedBox(width: AppSpacing.xs),
            Expanded(
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: () {
                  Clipboard.setData(ClipboardData(text: c.orderNo));
                  showFloatingToast(context, '已複製訂單編號');
                },
                child: Row(
                  children: [
                    Text(
                      c.orderNo,
                      style: AppTypography.bodyBold.copyWith(
                        color: AppColors.primaryDark,
                      ),
                    ),
                    SizedBox(width: AppSpacing.xs),
                    Icon(
                      AppIcons.copy,
                      size: 12.w,
                      color: AppIconColors.active,
                    ),
                  ],
                ),
              ),
            ),
            if (hasRefund) _refundAmountChip(c.refundableAmount),
          ],
        ),
        SizedBox(height: AppSpacing.xs),
        // 建立時間
        Row(
          children: [
            Icon(
              AppIcons.accessTime,
              size: 14.w,
              color: AppIconColors.disabled,
            ),
            SizedBox(width: AppSpacing.xs),
            Text(
              _formatTime(c.createdAt),
              style: AppTypography.meta.copyWith(
                color: AppTypography.colorDisabled,
              ),
            ),
          ],
        ),
      ],
    );
  }

  /// 可退金額膠囊（橙色高亮）
  Widget _refundAmountChip(String amount) {
    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.sm,
        vertical: AppSpacing.xs,
      ),
      decoration: BoxDecoration(
        color: AppColors.warning.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: AppColors.warning.withValues(alpha: 0.35)),
      ),
      child: Text.rich(
        TextSpan(
          children: [
            TextSpan(
              text: 'NT\$ ',
              style: AppTypography.meta.copyWith(color: AppColors.warning),
            ),
            TextSpan(
              text: amount,
              style: AppTypography.bodyBold.copyWith(color: AppColors.warning),
            ),
          ],
        ),
      ),
    );
  }

  // ─── 2. 退貨原因 + 說明 + 照片 ─────────────────

  Widget _buildReasonSection(AfterSaleCase c) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 標題：📝 申請資訊
        _sectionTitle(AppIcons.assignmentOutlined, '申請資訊'),
        SizedBox(height: AppSpacing.md),

        // 退貨原因 — 橙色 outline pill
        Row(
          children: [
            Text(
              '退貨原因',
              style: AppTypography.meta.copyWith(
                color: AppTypography.colorDisabled,
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            _reasonPill(AfterSaleReason.label(c.reason)),
          ],
        ),
        SizedBox(height: AppSpacing.sm),

        // 詳細說明 — 浅灰底 box，长文更易读
        Text(
          '詳細說明',
          style: AppTypography.meta.copyWith(
            color: AppTypography.colorDisabled,
          ),
        ),
        SizedBox(height: AppSpacing.xs),
        _descriptionBox(c.description.isNotEmpty ? c.description : '-'),
        SizedBox(height: AppSpacing.sm),

        // 上傳的照片
        Text(
          '上傳的照片',
          style: AppTypography.meta.copyWith(
            color: AppTypography.colorDisabled,
          ),
        ),
        SizedBox(height: AppSpacing.xs),
        Builder(
          builder: (_) {
            // 優先使用 evidence（新契約）中買家舉證的 imageUrl，
            // fallback 至舊欄位 shipmentPhotoUrls 保持向後相容
            final buyerPhotos = c.evidence
                .where(
                  (e) =>
                      e.evidenceType == EvidenceType.buyerProof &&
                      e.imageUrl.isNotEmpty,
                )
                .map((e) => e.imageUrl)
                .toList();
            final photos = buyerPhotos.isNotEmpty
                ? buyerPhotos
                : c.shipmentPhotoUrls;
            if (photos.isEmpty) return Text('-', style: AppTypography.meta);
            return Row(
              children: photos.take(3).map((url) {
                return Padding(
                  padding: EdgeInsets.only(right: AppSpacing.sm),
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(8.r),
                    child: OssImage(
                      imageSource: url,
                      scene: ImageScene.reviewPhoto,
                      width: 72.w,
                      height: 72.w,
                      fit: BoxFit.cover,
                    ),
                  ),
                );
              }).toList(),
            );
          },
        ),

        // 退貨代碼 — 高亮 box（和 7-11 三段碼視覺呼應）
        if (c.returnShipmentNo.isNotEmpty) ...[
          SizedBox(height: AppSpacing.md),
          _returnShipmentNoBox(c.returnShipmentNo),
        ],
      ],
    );
  }

  /// 退貨原因膠囊（橙色 outline，和頂部金額 chip 視覺呼應）
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

  /// 詳細說明 box — 淺灰底呈現長文
  Widget _descriptionBox(String text) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.sm),
      decoration: BoxDecoration(
        color: AppColors.bgPrimary,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Text(text, style: AppTypography.meta),
    );
  }

  /// 退貨代碼 box — 綠色高亮 + 可複製（業務關鍵資訊）
  Widget _returnShipmentNoBox(String code) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.sm),
      decoration: BoxDecoration(
        color: AppColors.primary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(8.r),
        border: Border.all(color: AppColors.primary.withValues(alpha: 0.35)),
      ),
      child: Row(
        children: [
          Icon(AppIcons.localShipping, size: 16.w, color: AppIconColors.active),
          SizedBox(width: AppSpacing.xs),
          Text(
            '退貨代碼',
            style: AppTypography.meta.copyWith(color: AppColors.primaryDark),
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              code,
              style: AppTypography.bodyBold.copyWith(
                color: AppColors.primaryDark,
                letterSpacing: 1.2,
              ),
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }

  // ─── 3. 物流進度 ───────────────────────────

  /// 物流進度 currentStep 對應（對齊賣家 退貨物流進度 `_returnLogisticsStatus`）：
  /// - status 50 已結案 → 3 驗貨完成
  /// - status 40 退款處理中 → 3 驗貨完成（賣家驗貨完成才會推進到 40）
  /// - refundTaskStatus 非空 → 3（賣家剛按完確認收貨、status 還沒推進的空窗期）
  /// - status 30 已收退貨 → 2 已送達
  /// - providerStatus 11/32（已取貨/待取貨）→ 2 已送達
  /// - status 25 退貨運送中 → 1 配送中
  /// - 其他（status 20 / 異常 / 空）→ 0
  static int _returnLogisticsCurrentStep(AfterSaleCase c) {
    if (c.status == AfterSaleStatus.closed) return 3;
    if (c.status == AfterSaleStatus.refundProcessing) return 3;
    if (c.refundTaskStatus.trim().isNotEmpty) return 3;
    if (c.status == '30') return 2;
    final provider = c.providerStatus.trim();
    if (provider == ReturnShipmentStatus.pickedUp ||
        provider == ReturnShipmentStatus.awaitingPickup) {
      return 2;
    }
    if (c.status == AfterSaleStatus.returnInTransit) return 1;
    return 0;
  }

  Widget _buildLogisticsSection(AfterSaleCase c) {
    // 第 4 步用「驗貨完成」對齊賣家 退貨物流進度（語意一致：物流結束於驗貨）。
    // 第 1 步依退貨方式**明確分派**；未知 / 本版未支援者用 provider-neutral 的
    // 「已寄出」，不得沿用宅配（到府取件）或超商（門市收件）的語意。
    final steps = switch (shippingPresentationOf(c.returnShippingMethod)) {
      ShippingPresentation.cvsStorePickup => const [
        '門市收件',
        '配送中',
        '已送達',
        '驗貨完成',
      ],
      ShippingPresentation.homeDelivery => const ['宅配取貨', '配送中', '已送達', '驗貨完成'],
      ShippingPresentation.unsupported => const ['已寄出', '配送中', '已送達', '驗貨完成'],
    };
    final currentStep = _returnLogisticsCurrentStep(c);

    final showReturnTimeline =
        c.status == AfterSaleStatus.awaitingReturn ||
        c.status == AfterSaleStatus.returnInTransit ||
        c.status == '30';

    // 退款進度區塊已併入上方 _buildProgressSteps 第 4 步「退款進度」+ 對應狀態
    // 由訂單卡的售後流程列代為呈現；此處不再渲染冗餘的下方退款 timeline。
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.localShipping, '物流進度'),
        SizedBox(height: AppSpacing.md),
        _buildProgressSteps(steps, currentStep),
        // 退貨物流（買家寄回賣家）
        if (showReturnTimeline) ...[
          SizedBox(height: AppSpacing.md),
          ReturnLogisticsTimeline(caseData: c),
        ],
      ],
    );
  }

  /// 物流進度 horizontal step indicator — 對齊賣家 退貨物流進度 `_InlineStepProgress` 規格。
  ///
  /// 4 步：
  /// 0 宅配取貨/門市收件（package）
  /// 1 配送中（truck）
  /// 2 已送達（check_circle_outline）
  /// 3 驗貨完成（fact_check_outlined，與賣家退貨物流進度同步）
  Widget _buildProgressSteps(List<String> steps, int currentStep) {
    final activeColor = AppColors.warning;
    final inactiveColor = AppColors.gray300;
    final activeText = AppColors.textPrimary;
    final inactiveText = AppColors.textSecondary;

    final icons = <IconData>[
      AppIcons.inventory2Outlined,
      AppIcons.localShippingOutlined,
      AppIcons.checkCircleOutline,
      AppIcons.factCheckOutlined,
    ];

    return LayoutBuilder(
      builder: (context, constraints) {
        final lineWidth =
            ((constraints.maxWidth - steps.length * 54.w) / (steps.length - 1))
                .clamp(8.0, double.infinity);
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
                        color: i <= currentStep ? activeColor : inactiveColor,
                        shape: BoxShape.circle,
                      ),
                      alignment: Alignment.center,
                      child: Icon(
                        icons[i.clamp(0, icons.length - 1)],
                        size: 18.sp,
                        color: AppIconColors.onBrand,
                      ),
                    ),
                    SizedBox(height: 5.h),
                    Text(
                      steps[i],
                      textAlign: TextAlign.center,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12,
                        color: i <= currentStep ? activeText : inactiveText,
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
                    color: i < currentStep ? activeColor : inactiveColor,
                    borderRadius: BorderRadius.circular(999),
                  ),
                ),
            ],
          ],
        );
      },
    );
  }

  // ─── 4. 操作按鈕 ──────────────────────────

  Widget _buildActionButtons(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
  ) {
    // 待處理（'0'）允許取消售後
    final canCancel = c.status == AfterSaleStatus.pending;

    if (!canCancel) return const SizedBox.shrink();

    return Padding(
      padding: EdgeInsets.only(top: AppSpacing.md),
      child: SizedBox(
        width: double.infinity,
        height: HkControl.minTap,
        child: OutlinedButton(
          onPressed: () => _onCancel(context, ref, c),
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

  Future<void> _onCancel(
    BuildContext context,
    WidgetRef ref,
    AfterSaleCase c,
  ) async {
    try {
      await ref.read(afterSaleRepositoryProvider).cancelAfterSale(c.caseNo);
      if (!context.mounted) return;
      ref.read(afterSaleOrderLookupProvider.notifier).removeOrderNo(c.orderNo);
      showFloatingToast(context, '已取消售後');
    } catch (_) {
      if (context.mounted) {
        showFloatingToast(context, '取消失敗', isSuccess: false);
      }
    }
  }

  String _formatTime(String raw) {
    if (raw.isEmpty) return '';
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final local = dt.toLocal();
    return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }
}
