import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../config/image_config.dart';
import '../../../core/codemeta/code_meta.dart';
import '../../../core/codemeta/code_meta_display.dart';
import '../../../core/codemeta/shipping_presentation.dart';
import '../../../core/theme/app_typography.dart';
import '../../../shared/widgets/app_dropdown.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../data/after_sale_evidence.dart';
import '../data/after_sale_model.dart';
import 'widgets/appeal_deadline_card.dart';

// ══════════════════════════════════════════════════════════════
// 售後處理 Entry Sheet (Figma 4820:44588)
// ══════════════════════════════════════════════════════════════

/// 從售後卡片的「售後申請」按鈕呼叫。
/// [afterSaleCase] 帶入退貨原因 / 詳細說明 / 照片。
/// [orderNo] 用於顯示原訂單編號。
/// [refundableAmount] 退款金額上限。
/// [onSubmit] 提交決策後的 callback。
void showAfterSalesResolveSheet({
  required BuildContext context,
  required AfterSaleCase afterSaleCase,
  required String orderNo,
  required String refundableAmount,
  String? allowedReturnShippingMethod,
  List<String> sellerShipmentPhotoUrls = const [],
  List<String> buyerReceiptPhotoUrls = const [],
  required Future<void> Function(Map<String, dynamic> data) onSubmit,
}) {
  showModalBottomSheet<void>(
    context: context,
    // 這個 sheet 有兩個開啟點：售後詳情頁（根路由，本來就正常）與訂單列表卡片的
    // `OrderBottomRow`（訂單 tab，位於 `StatefulShellRoute` 分支底下）。後者的分支
    // Navigator 在浮動導覽列之下，不掛根 Navigator 底部決策鈕會被蓋住（#132 同型）。
    useRootNavigator: true,
    isScrollControlled: true,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(24.r)),
    ),
    builder: (ctx) => _ResolveEntrySheet(
      afterSaleCase: afterSaleCase,
      orderNo: orderNo,
      refundableAmount: refundableAmount,
      allowedReturnShippingMethod: allowedReturnShippingMethod,
      sellerShipmentPhotoUrls: sellerShipmentPhotoUrls,
      buyerReceiptPhotoUrls: buyerReceiptPhotoUrls,
      onSubmit: (data) async {
        Navigator.of(ctx).pop();
        await onSubmit(data);
      },
    ),
  );
}

// ── Entry Sheet ───────────────────────────────────────────────

class _ResolveEntrySheet extends StatelessWidget {
  const _ResolveEntrySheet({
    required this.afterSaleCase,
    required this.orderNo,
    required this.refundableAmount,
    required this.allowedReturnShippingMethod,
    required this.sellerShipmentPhotoUrls,
    required this.buyerReceiptPhotoUrls,
    required this.onSubmit,
  });
  final AfterSaleCase afterSaleCase;
  final String orderNo;
  final String refundableAmount;
  final String? allowedReturnShippingMethod;
  final List<String> sellerShipmentPhotoUrls;
  final List<String> buyerReceiptPhotoUrls;
  final Future<void> Function(Map<String, dynamic> data) onSubmit;

  @override
  Widget build(BuildContext context) {
    final c = afterSaleCase;
    final reasonLabel = AfterSaleReason.label(c.reason);
    final description = c.description.trim();
    final buyerIssuePhotos = _buyerIssuePhotoSources(c);
    final sellerShippingPhotos = uniqueNonEmptyPhotoSources(
      sellerShipmentPhotoUrls,
    );
    final buyerReceivingPhotos = buyerReceiptPhotoUrls.isNotEmpty
        ? uniqueNonEmptyPhotoSources(buyerReceiptPhotoUrls)
        : buyerIssuePhotos;
    final hasApplicationInfo = reasonLabel.isNotEmpty || description.isNotEmpty;
    final hasDeliveryPhotos =
        sellerShippingPhotos.isNotEmpty || buyerReceivingPhotos.isNotEmpty;

    return DraggableScrollableSheet(
      expand: false,
      initialChildSize: 0.86,
      maxChildSize: 0.94,
      minChildSize: 0.5,
      builder: (_, scrollCtrl) => SingleChildScrollView(
        controller: scrollCtrl,
        padding: EdgeInsets.fromLTRB(
          40.w,
          24.h,
          40.w,
          MediaQuery.of(context).padding.bottom + 24.h,
        ),
        child: Column(
          children: [
            // Title
            Text(
              '選擇處理方式',
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textPrimary,
              ),
            ),
            SizedBox(height: 8.h),
            Text(
              '根據與買家的溝通結果，選擇最合適的解決方案',
              style: TextStyle(
                fontSize: 14.sp,
                color: AppColors.textPrimary,
                height: 1.4,
              ),
              textAlign: TextAlign.center,
            ),
            SizedBox(height: 8.h),

            if (hasApplicationInfo) ...[
              _AfterSaleApplicationSummary(
                reasonLabel: reasonLabel,
                description: description,
                issuePhotos: hasDeliveryPhotos ? const [] : buyerIssuePhotos,
              ),
              SizedBox(height: 8.h),
            ],

            if (hasDeliveryPhotos) ...[
              const Divider(
                height: 1,
                thickness: 1,
                color: AppColors.cardBorder,
              ),
              SizedBox(height: 8.h),
              _DeliveryPhotoComparison(
                sellerPhotos: sellerShippingPhotos,
                buyerPhotos: buyerReceivingPhotos,
              ),
              SizedBox(height: 8.h),
              const Divider(
                height: 1,
                thickness: 1,
                color: AppColors.cardBorder,
              ),
              SizedBox(height: 8.h),
            ],

            // 5 option buttons (2x2 grid + 1)
            Row(
              children: [
                Expanded(
                  child: _OptionCard(
                    icon: AppIcons.assignmentReturnOutlined,
                    title: '同意退貨退款',
                    subtitle: '買家寄回商品後退款',
                    onTap: () => _showReturnRefundDialog(context),
                  ),
                ),
                SizedBox(width: 8.w),
                Expanded(
                  child: _OptionCard(
                    icon: AppIcons.creditCardOutlined,
                    title: '部分退款',
                    subtitle: '買家留貨，退部分金額',
                    onTap: () => _showPartialRefundDialog(context),
                  ),
                ),
              ],
            ),
            SizedBox(height: 8.h),
            Row(
              children: [
                Expanded(
                  child: _OptionCard(
                    icon: AppIcons.creditCardOutlined,
                    title: '全額退款 (不退貨)',
                    subtitle: '直接退款，商品送給買家',
                    onTap: () => _showFullRefundDialog(context),
                  ),
                ),
                SizedBox(width: 8.w),
                Expanded(
                  child: _OptionCard(
                    icon: AppIcons.couponOutlined,
                    title: '發放優惠券',
                    subtitle: '安撫補償，不退現金',
                    onTap: () => _showCouponDialog(context),
                  ),
                ),
              ],
            ),
            SizedBox(height: 8.h),
            Align(
              alignment: Alignment.centerLeft,
              child: SizedBox(
                width: (MediaQuery.of(context).size.width - 80.w - 8.w) / 2,
                child: _OptionCard(
                  icon: AppIcons.blockOutlined,
                  title: '婉拒退款退貨',
                  subtitle: '不符合退款退貨條件',
                  onTap: () => _showRejectDialog(context),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ── Sub-dialogs ──

  void _showReturnRefundDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (_) => _ReturnRefundDialog(
        orderNo: orderNo,
        refundableAmount: refundableAmount,
        allowedReturnShippingMethod: allowedReturnShippingMethod,
        onSubmit: onSubmit,
      ),
    );
  }

  void _showPartialRefundDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (_) =>
          _PartialRefundDialog(maxAmount: refundableAmount, onSubmit: onSubmit),
    );
  }

  void _showFullRefundDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (_) =>
          _FullRefundDialog(refundAmount: refundableAmount, onSubmit: onSubmit),
    );
  }

  void _showCouponDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (_) => _CouponDialog(onSubmit: onSubmit),
    );
  }

  void _showRejectDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (_) =>
          _RejectDialog(afterSaleCase: afterSaleCase, onSubmit: onSubmit),
    );
  }
}

// ── Shared Widgets ────────────────────────────────────────────

/// 買家舉證照片 + 出貨照片。前者是 objectKey、後者是 Gateway 組好的顯示 URL，
/// 兩者都直接餵給 `OptimizedNetworkImage`（內部走 buildImageUrl 唯一入口）。
List<String> _buyerIssuePhotoSources(AfterSaleCase c) {
  return uniqueNonEmptyPhotoSources([
    ...buyerIssueEvidenceImageKeys(c.evidence),
    ...c.shipmentPhotoUrls,
  ]);
}

class _AfterSaleApplicationSummary extends StatelessWidget {
  const _AfterSaleApplicationSummary({
    required this.reasonLabel,
    required this.description,
    required this.issuePhotos,
  });
  final String reasonLabel;
  final String description;
  final List<String> issuePhotos;

  @override
  Widget build(BuildContext context) {
    final blocks = <Widget>[
      if (reasonLabel.isNotEmpty) _InfoBlock(title: '退貨原因', body: reasonLabel),
      if (description.isNotEmpty) _InfoBlock(title: '詳細說明', body: description),
      if (issuePhotos.isNotEmpty)
        Column(
          children: [
            Text(
              '上傳照片',
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textPrimary,
              ),
            ),
            SizedBox(height: 8.h),
            _PhotoStrip(
              photos: issuePhotos,
              alignment: MainAxisAlignment.center,
            ),
          ],
        ),
    ];

    return Column(
      children: [
        for (var i = 0; i < blocks.length; i++) ...[
          if (i > 0) SizedBox(height: 8.h),
          blocks[i],
        ],
      ],
    );
  }
}

class _InfoBlock extends StatelessWidget {
  const _InfoBlock({required this.title, required this.body});
  final String title;
  final String body;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Text(
          title,
          style: TextStyle(
            fontSize: 16.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.textPrimary,
          ),
        ),
        SizedBox(height: 8.h),
        Text(
          body,
          style: TextStyle(
            fontSize: 14.sp,
            color: AppColors.textPrimary,
            height: 1.35,
          ),
          textAlign: TextAlign.center,
        ),
      ],
    );
  }
}

class _DeliveryPhotoComparison extends StatelessWidget {
  const _DeliveryPhotoComparison({
    required this.sellerPhotos,
    required this.buyerPhotos,
  });
  final List<String> sellerPhotos;
  final List<String> buyerPhotos;

  @override
  Widget build(BuildContext context) {
    final hasSeller = sellerPhotos.isNotEmpty;
    final hasBuyer = buyerPhotos.isNotEmpty;

    if (hasSeller && hasBuyer) {
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: _PhotoColumn(label: '賣家出貨照片', photos: sellerPhotos),
          ),
          Padding(
            padding: EdgeInsets.only(top: 48.h),
            child: _VsBadge(),
          ),
          Expanded(
            child: _PhotoColumn(label: '買家收貨照片', photos: buyerPhotos),
          ),
        ],
      );
    }

    return _PhotoGroup(
      title: hasSeller ? '賣家出貨照片' : '買家收貨照片',
      photos: hasSeller ? sellerPhotos : buyerPhotos,
    );
  }
}

class _PhotoGroup extends StatelessWidget {
  const _PhotoGroup({required this.title, required this.photos});
  final String title;
  final List<String> photos;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Text(
          title,
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 16.sp,
            color: AppColors.textEmphasis,
            fontWeight: FontWeight.w700,
          ),
        ),
        SizedBox(height: 8.h),
        _PhotoStrip(photos: photos, alignment: MainAxisAlignment.center),
      ],
    );
  }
}

class _PhotoColumn extends StatelessWidget {
  const _PhotoColumn({required this.label, required this.photos});
  final String label;
  final List<String> photos;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        SizedBox(
          width: 133.w,
          child: Text(
            label,
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 16.sp,
              color: AppColors.textEmphasis,
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
        SizedBox(height: 8.h),
        _PhotoThumb(url: photos.first, overflowCount: photos.length - 1),
      ],
    );
  }
}

class _PhotoStrip extends StatelessWidget {
  const _PhotoStrip({required this.photos, required this.alignment});
  final List<String> photos;
  final MainAxisAlignment alignment;

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        mainAxisAlignment: alignment,
        children: [
          for (var i = 0; i < photos.length; i++) ...[
            _PhotoThumb(url: photos[i]),
            if (i != photos.length - 1) SizedBox(width: 8.w),
          ],
        ],
      ),
    );
  }
}

class _PhotoThumb extends StatelessWidget {
  const _PhotoThumb({required this.url, this.overflowCount = 0});
  final String url;
  final int overflowCount;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 82.w,
      height: 82.w,
      decoration: BoxDecoration(
        color: AppColors.bgPrimary,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.textSecondary),
      ),
      clipBehavior: Clip.antiAlias,
      child: Stack(
        fit: StackFit.expand,
        children: [
          OptimizedNetworkImage(
            // objectKey 或後端顯示 URL 皆直接交給 OptimizedNetworkImage，內部走
            // buildImageUrl（唯一組裝入口）依 reviewPhoto 場景組 URL。
            imageUrl: url,
            scene: ImageScene.reviewPhoto,
            fit: BoxFit.cover,
            errorWidget: Icon(
              AppIcons.brokenImageOutlined,
              size: 28.sp,
              color: AppIconColors.disabled,
            ),
          ),
          if (overflowCount > 0)
            Positioned(
              right: 0,
              bottom: 0,
              child: Container(
                padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
                decoration: BoxDecoration(
                  color: AppColors.textEmphasis.withValues(alpha: 0.6),
                  borderRadius: BorderRadius.only(
                    topLeft: Radius.circular(8.r),
                  ),
                ),
                child: Text(
                  '+$overflowCount',
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.bgCard,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _VsBadge extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      width: 36.w,
      height: 36.w,
      decoration: const BoxDecoration(
        color: AppColors.primary,
        shape: BoxShape.circle,
      ),
      alignment: Alignment.center,
      child: Text(
        'vs',
        style: TextStyle(
          fontSize: 18.sp,
          fontWeight: FontWeight.w700,
          color: AppColors.bgCard,
        ),
      ),
    );
  }
}

class _OptionCard extends StatelessWidget {
  const _OptionCard({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
  });
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        height: 96.h,
        padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 8.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(8.r),
          border: Border.all(color: AppColors.textSecondary, width: 1),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 26.sp, color: AppIconColors.defaultColor),
            SizedBox(height: 4.h),
            Text(
              title,
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textPrimary,
                height: 1.15,
              ),
              textAlign: TextAlign.center,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            SizedBox(height: 2.h),
            Text(
              subtitle,
              style: TextStyle(
                fontSize: 12.sp,
                color: AppColors.textSecondary,
                height: 1.25,
              ),
              textAlign: TextAlign.center,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}

// ── Dialog shell ──────────────────────────────────────────────

class _ResolveDialog extends StatelessWidget {
  const _ResolveDialog({
    required this.title,
    this.subtitle,
    required this.confirmLabel,
    required this.confirmColor,
    required this.onConfirm,
    required this.body,
    this.submitting = false,
  });
  final String title;
  final String? subtitle;
  final String confirmLabel;
  final Color confirmColor;
  final VoidCallback? onConfirm;
  final Widget body;
  final bool submitting;

  @override
  Widget build(BuildContext context) {
    // 視覺對齊「同意退貨退款」彈窗（_ReturnRefundDialog）：細灰邊 + 15r 卡片，
    // 標題 18sp、副標 14sp，按鈕 40h / 8r 圓角 + 微陰影。
    // 依 confirmColor 亮度挑前景色：淺色（如金 #E0C44C）用深墨字，深色用白字，
    // 確保按鈕文字/載入圈與底色對比足夠。
    final fg = confirmColor.computeLuminance() > 0.45
        ? AppColors.textEmphasis
        : AppColors.bgCard;
    return Dialog(
      insetPadding: EdgeInsets.symmetric(horizontal: 24.w, vertical: 40.h),
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(
        side: const BorderSide(width: 1, color: AppColors.cardBorder),
        borderRadius: BorderRadius.circular(16.r),
      ),
      child: SingleChildScrollView(
        padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 20.h),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // Title
            Text(
              title,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: AppTypography.colorEmphasis,
                fontSize: 18.sp,
                fontWeight: FontWeight.w700,
                height: 1.23,
              ),
            ),
            if (subtitle != null) ...[
              SizedBox(height: 8.h),
              Text(
                subtitle!,
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: AppColors.textSecondary,
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w400,
                  height: 1.43,
                ),
              ),
            ],
            SizedBox(height: 16.h),
            body,
            SizedBox(height: 16.h),
            // Bottom buttons: 取消 + confirm
            Row(
              children: [
                Expanded(
                  child: SizedBox(
                    height: HkControl.minTap,
                    child: OutlinedButton(
                      onPressed: () => Navigator.of(context).pop(),
                      style: OutlinedButton.styleFrom(
                        padding: EdgeInsets.symmetric(
                          horizontal: 16.w,
                          vertical: 8.h,
                        ),
                        foregroundColor: AppColors.textSecondary,
                        side: const BorderSide(
                          width: 1,
                          color: AppColors.cardBorder,
                        ),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8.r),
                        ),
                      ),
                      child: HkButtonLabel(
                        '取消',
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w400,
                          height: 1.5,
                        ),
                      ),
                    ),
                  ),
                ),
                SizedBox(width: 18.w),
                Expanded(
                  child: SizedBox(
                    height: HkControl.minTap,
                    child: ElevatedButton(
                      onPressed: submitting ? null : onConfirm,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: confirmColor,
                        disabledBackgroundColor: confirmColor.withValues(
                          alpha: 0.45,
                        ),
                        padding: EdgeInsets.symmetric(
                          horizontal: 16.w,
                          vertical: 8.h,
                        ),
                        elevation: 2,
                        shadowColor: AppColors.cardShadow,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8.r),
                        ),
                      ),
                      child: submitting
                          ? SizedBox(
                              width: 18.sp,
                              height: 18.sp,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: fg,
                              ),
                            )
                          : HkButtonLabel(
                              confirmLabel,
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: fg,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                                height: 1.5,
                              ),
                            ),
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

// ── 1. 同意退貨退款 Dialog (Figma 6260:116436) ─────────────────

class _ReturnRefundDialog extends StatefulWidget {
  const _ReturnRefundDialog({
    required this.orderNo,
    required this.refundableAmount,
    required this.allowedReturnShippingMethod,
    required this.onSubmit,
  });
  final String orderNo;
  final String refundableAmount;
  final String? allowedReturnShippingMethod;
  final Future<void> Function(Map<String, dynamic>) onSubmit;

  @override
  State<_ReturnRefundDialog> createState() => _ReturnRefundDialogState();
}

class _ReturnRefundDialogState extends State<_ReturnRefundDialog> {
  late String _shippingMethod;
  bool _submitting = false;

  @override
  void initState() {
    super.initState();
    final allowed = widget.allowedReturnShippingMethod;
    // 🔴 原本是 `allowed == 3002 ? 3002 : 3001` —— 後端指定了一個本 App 不支援的
    // 退貨方式（3003/3004/3005 或任何新碼值）時會**預設選取黑貓並一鍵建出黑貓
    // 逆物流單**。改為：只有兩個支援值才預選，其餘留空並由 build 擋住送出。
    final allowedPresentation = shippingPresentationOf(allowed);
    _shippingMethod = switch (allowedPresentation) {
      ShippingPresentation.cvsStorePickup =>
        ShippingMethod.sevenElevenStorePickup.value,
      ShippingPresentation.homeDelivery => ShippingMethod.taqbin.value,
      // allowed 為空 = 後端未限定 → 沿用既有預設（黑貓，兩張卡都給選）；
      // allowed 非空但不支援 → 留空，_availableMethods() 會回空清單。
      ShippingPresentation.unsupported =>
        (allowed == null || allowed.trim().isEmpty)
            ? ShippingMethod.taqbin.value
            : '',
    };
  }

  @override
  Widget build(BuildContext context) {
    final availableMethods = _availableMethods();
    final selected = shippingPresentationOf(_shippingMethod);
    final unsupportedAllowed = availableMethods.isEmpty;
    final methodLabel = _methodTitle(_shippingMethod);
    // 🔴 CTA 文案依所選退貨方式**明確分派**；未支援時不得顯示「預約黑貓派車」。
    final confirmLabel = switch (selected) {
      ShippingPresentation.cvsStorePickup => '產生7-11退貨代碼',
      ShippingPresentation.homeDelivery => '預約黑貓派車',
      ShippingPresentation.unsupported => '此退貨方式暫不支援',
    };

    return Dialog(
      insetPadding: EdgeInsets.symmetric(horizontal: 24.w, vertical: 40.h),
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(
        side: const BorderSide(width: 1, color: AppColors.cardBorder),
        borderRadius: BorderRadius.circular(16.r),
      ),
      child: SingleChildScrollView(
        padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 20.h),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              '建立退貨物流單 (同意退貨退款)',
              textAlign: TextAlign.center,
              style: TextStyle(
                color: AppColors.textEmphasis,
                fontSize: 18.sp,
                fontWeight: FontWeight.w700,
                height: 1.23,
              ),
            ),
            SizedBox(height: 8.h),
            Text(
              '適用於乾貨、未開封商品。系統將建立逆物流單號。',
              textAlign: TextAlign.center,
              style: TextStyle(
                color: AppColors.textSecondary,
                fontSize: 14.sp,
                fontWeight: FontWeight.w400,
                height: 1.43,
              ),
            ),
            SizedBox(height: 16.h),
            Container(
              padding: EdgeInsets.all(12.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(12.r),
                border: Border.all(color: AppColors.cardBorder),
                boxShadow: const [
                  BoxShadow(
                    color: AppColors.cardShadow,
                    blurRadius: 2,
                    offset: Offset(0, 1),
                  ),
                ],
              ),
              child: Column(
                children: [
                  _summaryRow(
                    '原訂單編號',
                    '#${widget.orderNo}',
                    labelColor: AppColors.textSecondary,
                    valueColor: AppColors.textSecondary,
                    fontWeight: FontWeight.w400,
                  ),
                  SizedBox(height: 4.h),
                  _summaryRow(
                    '退款金額預告',
                    'NT\$ ${widget.refundableAmount}',
                    labelColor: AppColors.textEmphasis,
                    valueColor: AppColors.danger,
                    fontWeight: FontWeight.w500,
                  ),
                ],
              ),
            ),
            SizedBox(height: 16.h),
            Text(
              '選擇退貨物流方式',
              style: TextStyle(
                color: AppColors.textEmphasis,
                fontSize: 18.sp,
                fontWeight: FontWeight.w400,
                height: 1.62,
                letterSpacing: 0.77,
              ),
            ),
            SizedBox(height: 10.h),
            Container(
              width: double.infinity,
              padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
              decoration: BoxDecoration(
                color: AppColors.bgPrimary,
                borderRadius: BorderRadius.circular(8.r),
              ),
              child: Text(
                unsupportedAllowed
                    ? '此訂單指定的退貨方式（$methodLabel）本版 App 尚未支援，'
                          '請改由後台或客服處理，避免建立錯誤的退貨物流單。'
                    : '此訂單需使用 $methodLabel 建立退貨物流，系統會依原寄送方式處理。',
                style: TextStyle(
                  fontSize: 14.sp,
                  height: 1.43,
                  color: AppColors.textSecondary,
                ),
                textAlign: TextAlign.center,
              ),
            ),
            SizedBox(height: 10.h),
            _buildLogisticsChoices(availableMethods),
            SizedBox(height: 16.h),
            Row(
              children: [
                Expanded(
                  child: SizedBox(
                    height: HkControl.minTap,
                    child: OutlinedButton(
                      onPressed: () => Navigator.of(context).pop(),
                      style: OutlinedButton.styleFrom(
                        padding: EdgeInsets.symmetric(
                          horizontal: 16.w,
                          vertical: 8.h,
                        ),
                        foregroundColor: AppColors.textSecondary,
                        side: const BorderSide(
                          width: 1,
                          color: AppColors.cardBorder,
                        ),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8.r),
                        ),
                      ),
                      child: HkButtonLabel(
                        '取消',
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w400,
                          height: 1.5,
                        ),
                      ),
                    ),
                  ),
                ),
                SizedBox(width: 18.w),
                Expanded(
                  child: SizedBox(
                    height: HkControl.minTap,
                    child: ElevatedButton(
                      onPressed: (_submitting || unsupportedAllowed)
                          ? null
                          : _submit,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.primary,
                        disabledBackgroundColor: AppColors.primaryLight,
                        padding: EdgeInsets.symmetric(
                          horizontal: 16.w,
                          vertical: 8.h,
                        ),
                        elevation: 2,
                        shadowColor: AppColors.cardShadow,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8.r),
                        ),
                      ),
                      child: _submitting
                          ? SizedBox(
                              width: 18.sp,
                              height: 18.sp,
                              child: const CircularProgressIndicator(
                                strokeWidth: 2,
                                color: AppColors.bgCard,
                              ),
                            )
                          : HkButtonLabel(
                              confirmLabel,
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: AppColors.bgCard,
                                fontSize:
                                    selected ==
                                        ShippingPresentation.cvsStorePickup
                                    ? 15.sp
                                    : 16.sp,
                                fontWeight: FontWeight.w700,
                                height: 1.5,
                              ),
                            ),
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  /// 可選的退貨物流方式。
  ///
  /// - 後端指定且本 App 支援 → 只給那一張卡
  /// - 後端未指定（空） → 兩張都給
  /// - 🔴 後端指定但本 App 不支援 → **空清單**（不得退回「兩張都給」，
  ///   那等於無視後端的限定、讓賣家建出錯誤的逆物流單）
  List<String> _availableMethods() {
    final allowed = widget.allowedReturnShippingMethod;
    switch (shippingPresentationOf(allowed)) {
      case ShippingPresentation.cvsStorePickup:
        return [ShippingMethod.sevenElevenStorePickup.value];
      case ShippingPresentation.homeDelivery:
        return [ShippingMethod.taqbin.value];
      case ShippingPresentation.unsupported:
        if (allowed == null || allowed.trim().isEmpty) {
          return [
            ShippingMethod.sevenElevenStorePickup.value,
            ShippingMethod.taqbin.value,
          ];
        }
        return const [];
    }
  }

  Widget _buildLogisticsChoices(List<String> methods) {
    final cards = methods
        .map(
          (method) => _LogisticsCard(
            icon: switch (shippingPresentationOf(method)) {
              ShippingPresentation.cvsStorePickup => HkIconName.store,
              ShippingPresentation.homeDelivery ||
              ShippingPresentation.unsupported => HkIconName.truck,
            },
            title: _methodTitle(method),
            lines: switch (shippingPresentationOf(method)) {
              ShippingPresentation.cvsStorePickup => const ['產生物流代碼', '買家自行寄回'],
              ShippingPresentation.homeDelivery => const ['預約上門取件', '運費賣家負擔'],
              ShippingPresentation.unsupported => const ['本版 App 不支援'],
            },
            selected: _shippingMethod == method,
            accentColor: AppColors.primary,
            onTap: () {
              if (_shippingMethod == method) return;
              setState(() => _shippingMethod = method);
            },
          ),
        )
        .toList();

    if (cards.isEmpty) return const SizedBox.shrink();
    if (cards.length == 1) return cards.first;

    return Row(
      children: [
        Expanded(child: cards[0]),
        SizedBox(width: 18.w),
        Expanded(child: cards[1]),
      ],
    );
  }

  String _methodTitle(String method) =>
      switch (shippingPresentationOf(method)) {
        ShippingPresentation.cvsStorePickup => '7-11 退貨便',
        ShippingPresentation.homeDelivery => '黑貓宅急便',
        // 未知 / 本版未支援：顯示 CodeMeta label（查不到就顯示原碼），
        // 不得冒名成「黑貓宅急便」。
        ShippingPresentation.unsupported => CodeMetaDisplay.shippingMethod(
          method,
          fallback: '未指定',
        ),
      };

  Widget _summaryRow(
    String label,
    String value, {
    required Color labelColor,
    required Color valueColor,
    required FontWeight fontWeight,
  }) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Flexible(
          child: Text(
            label,
            style: TextStyle(
              color: labelColor,
              fontSize: 14.sp,
              fontWeight: fontWeight,
              height: 1.43,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        SizedBox(width: 8.w),
        Flexible(
          child: Text(
            value,
            textAlign: TextAlign.right,
            style: TextStyle(
              color: valueColor,
              fontSize: 14.sp,
              fontWeight: fontWeight,
              height: 1.43,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }

  Future<void> _submit() async {
    setState(() => _submitting = true);
    await widget.onSubmit({
      'resolutionType': AfterSaleResolutionType.returnRefund.value,
      'returnShippingMethod': _shippingMethod,
    });
    if (mounted) {
      setState(() => _submitting = false);
      Navigator.of(context).pop();
    }
  }
}

class _LogisticsCard extends StatelessWidget {
  const _LogisticsCard({
    required this.icon,
    required this.title,
    required this.lines,
    required this.selected,
    required this.accentColor,
    required this.onTap,
  });
  final HkIconName icon;
  final String title;
  final List<String> lines;
  final bool selected;
  final Color accentColor;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: double.infinity,
        height: 133.h,
        padding: EdgeInsets.symmetric(vertical: 8.h, horizontal: 8.w),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(
            color: selected ? accentColor : AppColors.cardBorder,
            width: selected ? 3 : 1,
          ),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 45.w,
              height: 45.w,
              padding: EdgeInsets.all(12.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(12.r),
                border: Border.all(color: AppColors.cardBorder),
              ),
              child: HkIcon(
                icon,
                size: 21.w,
                color: AppIconColors.defaultColor,
              ),
            ),
            SizedBox(height: 4.h),
            Text(
              title,
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: selected ? FontWeight.w700 : FontWeight.w400,
                color: selected ? accentColor : AppColors.textPrimary,
                height: 1.5,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            ...lines.map(
              (l) => Text(
                l,
                style: TextStyle(
                  fontSize: 14.sp,
                  color: selected
                      ? AppColors.textPrimary
                      : AppColors.textSecondary,
                  height: 1.43,
                ),
                textAlign: TextAlign.center,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ── 2. 部分退款 Dialog (Figma 5050:45736) ──────────────────────

class _PartialRefundDialog extends StatefulWidget {
  const _PartialRefundDialog({required this.maxAmount, required this.onSubmit});
  final String maxAmount;
  final Future<void> Function(Map<String, dynamic>) onSubmit;

  @override
  State<_PartialRefundDialog> createState() => _PartialRefundDialogState();
}

class _PartialRefundDialogState extends State<_PartialRefundDialog> {
  final _amountCtrl = TextEditingController();
  String _reason = '';
  final _messageCtrl = TextEditingController();
  bool _submitting = false;

  static const _reasons = ['商品輕微瑕疵', '部分商品損壞', '數量短少', '品質不符預期', '其他'];

  @override
  void dispose() {
    _amountCtrl.dispose();
    _messageCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final max = int.tryParse(widget.maxAmount) ?? 1000;
    return _ResolveDialog(
      title: '部分退款 (協商退款)',
      subtitle: '買家將保留商品，您只需退還部分金額。適用於商品有小瑕疵但仍可使用的情況。',
      confirmLabel: '確認部分退款',
      confirmColor: AppColors.warning,
      submitting: _submitting,
      onConfirm: _submit,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('退款金額 (上限 \$$max)', style: _labelStyle),
          SizedBox(height: 6.h),
          _InputField(
            controller: _amountCtrl,
            hint: '輸入金額 (最大 $max)',
            keyboardType: TextInputType.number,
          ),
          SizedBox(height: 16.h),
          Text('退款原因', style: _labelStyle),
          SizedBox(height: 6.h),
          _DropdownField(
            value: _reason.isEmpty ? null : _reason,
            hint: '選擇原因',
            items: _reasons,
            onChanged: (v) => setState(() => _reason = v ?? ''),
          ),
          SizedBox(height: 16.h),
          Text('協商原因', style: _labelStyle),
          SizedBox(height: 6.h),
          _InputField(
            controller: _messageCtrl,
            hint: '例如：商品壞掉，與買家達成協議...',
            maxLines: 3,
          ),
        ],
      ),
    );
  }

  TextStyle get _labelStyle => TextStyle(
    fontSize: 14.sp,
    fontWeight: FontWeight.w700,
    color: AppTypography.colorEmphasis,
  );

  Future<void> _submit() async {
    if (_amountCtrl.text.trim().isEmpty) return;
    setState(() => _submitting = true);
    await widget.onSubmit({
      'resolutionType': AfterSaleResolutionType.partialRefund.value,
      'refundAmount': _amountCtrl.text.trim(),
      'rejectReason': _reason,
      'rejectMessage': _messageCtrl.text.trim(),
    });
    if (mounted) {
      setState(() => _submitting = false);
      Navigator.of(context).pop();
    }
  }
}

// ── 3. 全額退款(免退貨) Dialog (Figma 5050:46375) ──────────────

class _FullRefundDialog extends StatefulWidget {
  const _FullRefundDialog({required this.refundAmount, required this.onSubmit});
  final String refundAmount;
  final Future<void> Function(Map<String, dynamic>) onSubmit;

  @override
  State<_FullRefundDialog> createState() => _FullRefundDialogState();
}

class _FullRefundDialogState extends State<_FullRefundDialog> {
  final _messageCtrl = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _messageCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return _ResolveDialog(
      title: '全額退款 (免退貨)',
      subtitle: '買家將保留商品，您需退全額',
      confirmLabel: '確認全額退款',
      confirmColor: AppColors.error,
      submitting: _submitting,
      onConfirm: _submit,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 退款金額 (readonly)
          Container(
            padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(8.r),
              border: Border.all(color: AppColors.cardBorder),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  '退款金額',
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textPrimary,
                  ),
                ),
                Text(
                  'NT\$ ${widget.refundAmount}',
                  style: TextStyle(
                    fontSize: 18.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
              ],
            ),
          ),
          SizedBox(height: 16.h),
          Text(
            '協商原因',
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
          SizedBox(height: 6.h),
          _InputField(controller: _messageCtrl, hint: '蘋果全壞掉', maxLines: 3),
        ],
      ),
    );
  }

  Future<void> _submit() async {
    setState(() => _submitting = true);
    await widget.onSubmit({
      'resolutionType': AfterSaleResolutionType.fullRefundNoReturn.value,
      'refundAmount': widget.refundAmount,
      'rejectMessage': _messageCtrl.text.trim(),
    });
    if (mounted) {
      setState(() => _submitting = false);
      Navigator.of(context).pop();
    }
  }
}

// ── 4. 發放優惠券 Dialog (Figma 5050:47403) ────────────────────

class _CouponDialog extends StatefulWidget {
  const _CouponDialog({required this.onSubmit});
  final Future<void> Function(Map<String, dynamic>) onSubmit;

  @override
  State<_CouponDialog> createState() => _CouponDialogState();
}

class _CouponDialogState extends State<_CouponDialog> {
  final _amountCtrl = TextEditingController();
  final _minSpendCtrl = TextEditingController();
  final _messageCtrl = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _amountCtrl.dispose();
    _minSpendCtrl.dispose();
    _messageCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return _ResolveDialog(
      title: '發放補償優惠券',
      subtitle: '您可以發放優惠券作為退貨的替代方案或補償。\n此優惠卷為無期限，可以下次購物使用',
      confirmLabel: '確認發送',
      confirmColor: AppColors.warning,
      submitting: _submitting,
      onConfirm: _submit,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('優惠券面額', style: _labelStyle),
          SizedBox(height: 6.h),
          _InputField(
            controller: _amountCtrl,
            hint: '輸入金額',
            keyboardType: TextInputType.number,
          ),
          SizedBox(height: 16.h),
          Text('滿多少金額才能使用優惠卷 (上限 \$1000)', style: _labelStyle),
          SizedBox(height: 6.h),
          _InputField(
            controller: _minSpendCtrl,
            hint: '輸入金額 (最大 1000)',
            keyboardType: TextInputType.number,
          ),
          SizedBox(height: 16.h),
          Text('給買家的訊息', style: _labelStyle),
          SizedBox(height: 6.h),
          _InputField(
            controller: _messageCtrl,
            hint: '例如：很抱歉讓您有不好的體驗，這是一點心意...',
            maxLines: 3,
          ),
        ],
      ),
    );
  }

  TextStyle get _labelStyle => TextStyle(
    fontSize: 14.sp,
    fontWeight: FontWeight.w700,
    color: AppTypography.colorEmphasis,
  );

  Future<void> _submit() async {
    if (_amountCtrl.text.trim().isEmpty) return;
    setState(() => _submitting = true);
    await widget.onSubmit({
      'resolutionType': AfterSaleResolutionType.couponCompensation.value,
      'couponAmount': _amountCtrl.text.trim(),
      'couponMinSpend': _minSpendCtrl.text.trim(),
      'couponMessage': _messageCtrl.text.trim(),
    });
    if (mounted) {
      setState(() => _submitting = false);
      Navigator.of(context).pop();
    }
  }
}

// ── 5. 婉拒售後申請 Dialog (Figma 5053:24762) ──────────────────

class _RejectDialog extends StatefulWidget {
  const _RejectDialog({required this.afterSaleCase, required this.onSubmit});
  final AfterSaleCase afterSaleCase;
  final Future<void> Function(Map<String, dynamic>) onSubmit;

  @override
  State<_RejectDialog> createState() => _RejectDialogState();
}

class _RejectDialogState extends State<_RejectDialog> {
  String _reason = '';
  final _messageCtrl = TextEditingController();
  bool _submitting = false;

  static const _reasons = ['超過申訴時效', '買家保存不當', '非商品品質問題', '已使用/拆封', '其他'];

  @override
  void dispose() {
    _messageCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final createdAt = DateTime.tryParse(widget.afterSaleCase.createdAt);

    return _ResolveDialog(
      title: '婉拒售後申請',
      subtitle: '請選擇婉拒原因，系統將通知買家並保留爭議證據。',
      confirmLabel: '送出拒絕',
      confirmColor: AppColors.error,
      submitting: _submitting,
      onConfirm: _submit,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 時效與爭議比對（生鮮 60h 申訴期；deliveredAt 將於 [B-95] 後補上）
          if (createdAt != null) AppealDeadlineCard(createdAt: createdAt),
          SizedBox(height: 16.h),
          Text(
            '拒絕原因',
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
          SizedBox(height: 6.h),
          _DropdownField(
            value: _reason.isEmpty ? null : _reason,
            hint: '選擇主要原因',
            items: _reasons,
            onChanged: (v) => setState(() => _reason = v ?? ''),
          ),
          SizedBox(height: 16.h),
          Text(
            '回覆訊息 (通知買家)',
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
          SizedBox(height: 6.h),
          _InputField(
            controller: _messageCtrl,
            hint: '親愛的顧客您好，經查詢物流紀錄...',
            maxLines: 4,
          ),
        ],
      ),
    );
  }

  Future<void> _submit() async {
    if (_reason.isEmpty) return;
    setState(() => _submitting = true);
    await widget.onSubmit({
      'resolutionType': AfterSaleResolutionType.reject.value,
      'rejectReason': _reason,
      'rejectMessage': _messageCtrl.text.trim(),
    });
    if (mounted) {
      setState(() => _submitting = false);
      Navigator.of(context).pop();
    }
  }
}

// ── Form helpers ──────────────────────────────────────────────

class _InputField extends StatelessWidget {
  const _InputField({
    required this.controller,
    required this.hint,
    this.maxLines = 1,
    this.keyboardType,
  });
  final TextEditingController controller;
  final String hint;
  final int maxLines;
  final TextInputType? keyboardType;

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      maxLines: maxLines,
      keyboardType: keyboardType,
      decoration: InputDecoration(
        hintText: hint,
        hintStyle: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
        contentPadding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8.r),
          borderSide: const BorderSide(color: AppColors.cardBorder),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8.r),
          borderSide: const BorderSide(color: AppColors.cardBorder),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8.r),
          borderSide: const BorderSide(color: AppColors.primary, width: 1.5),
        ),
      ),
    );
  }
}

/// 售後處理彈窗的下拉欄位 — 走 design-system 的 [AppDropdown]，
/// 跟設定頁、認證頁、優惠券頁共用同一份視覺。
class _DropdownField extends StatelessWidget {
  const _DropdownField({
    required this.value,
    required this.hint,
    required this.items,
    required this.onChanged,
  });
  final String? value;
  final String hint;
  final List<String> items;
  final ValueChanged<String?> onChanged;

  @override
  Widget build(BuildContext context) {
    return AppDropdown(
      items: items,
      selectedItem: value,
      placeholder: hint,
      height: 44.h,
      onSelected: onChanged,
    );
  }
}
