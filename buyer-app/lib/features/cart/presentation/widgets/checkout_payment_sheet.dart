import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/core/codemeta/shipping_presentation.dart';

import '../../../../core/exceptions/seller_suspension_codes.dart';
import '../../../../core/l10n/ui_locale.dart';
import '../../../../l10n/l10n.dart';
import '../../../order/data/payment_flow.dart';
import '../../../order/presentation/payment_outcome_text.dart';
import '../../../../core/networking/dio_provider.dart';
import '../../../../shared/widgets/common_dialog.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/swipe_to_pay.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../../address/data/address_model.dart';
import '../../data/cart_model.dart';


/// 結帳付款彈窗的配置參數。
class CheckoutParams {
  const CheckoutParams({
    required this.selectedItems,
    required this.shippingOption,
    required this.selectedAddress,
    required this.selectedStorePickup,
    required this.skuProductIds,
    required this.deliveryNotesText,
    required this.appliedCouponId,
    required this.couponDiscount,
    required this.canSelectCoupon,
  });

  final List<CartItem> selectedItems;
  final Map<String, dynamic> shippingOption;
  final Address? selectedAddress;
  final Map<String, dynamic>? selectedStorePickup;
  final Map<int, String> skuProductIds;
  final String deliveryNotesText;
  final String appliedCouponId;
  final double couponDiscount;
  final bool canSelectCoupon;
}

/// 結帳付款底部彈窗的回調。
class CheckoutCallbacks {
  const CheckoutCallbacks({
    required this.onShowCouponSheet,
    required this.onReloadCart,
    required this.onTriggerShippingFee,
    required this.onClearCoupons,
    required this.onNavigateToOrders,
    required this.onNavigateHome,
    required this.onInvalidateCartCount,
    this.onPaymentSucceeded,
  });

  final Future<Map<String, dynamic>?> Function() onShowCouponSheet;
  final VoidCallback onReloadCart;
  final VoidCallback onTriggerShippingFee;
  final VoidCallback onClearCoupons;

  /// 導向訂單頁（傳 tab index）。
  final VoidCallback onNavigateToOrders;

  /// 導向首頁。
  final VoidCallback onNavigateHome;

  /// 讓主頁面 invalidate cartCount。
  final VoidCallback onInvalidateCartCount;

  /// 付款經 GetPayment 確認成功後回調（傳入已確認付款的 orderNo），用於觸發 GA4 purchase 事件。
  /// 多賣家拆單時每筆 orderNo 各呼叫一次。
  final void Function(String orderNo)? onPaymentSucceeded;
}

/// 結帳付款底部彈窗內容。
///
/// 透過 [showCheckoutPaymentSheet] 呼叫。
class CheckoutPaymentSheet extends StatefulWidget {
  const CheckoutPaymentSheet({
    super.key,
    required this.params,
    required this.callbacks,
    required this.dio,
    required this.parentContext,
  });

  final CheckoutParams params;
  final CheckoutCallbacks callbacks;
  final Dio dio;

  /// 主畫面的 BuildContext，用於顯示 SnackBar（底部彈窗 context 不同）。
  final BuildContext parentContext;

  @override
  State<CheckoutPaymentSheet> createState() => _CheckoutPaymentSheetState();
}

class _CheckoutPaymentSheetState extends State<CheckoutPaymentSheet> {
  Map<String, dynamic>? _previewData;
  bool _previewLoading = true;
  String? _previewError;
  bool _isConfirming = false;
  bool _previewRequested = false;
  late String _appliedCouponId;

  @override
  void initState() {
    super.initState();
    _appliedCouponId = widget.params.appliedCouponId;
    _loadPreview();
  }


  Future<void> _loadPreview() async {
    if (_previewRequested) return;
    _previewRequested = true;

    final p = widget.params;
    final shipping = shippingPresentationOf(
      p.shippingOption['shippingMethod']?.toString(),
    );
    if (shipping == ShippingPresentation.unsupported) {
      await Future<void>.delayed(Duration.zero);
      if (!mounted) return;
      setState(() {
        _previewLoading = false;
        _previewError = '此配送方式暫不支援，請返回購物車改選其他配送方式';
      });
      return;
    }
    final addr = shipping.usesRecipientAddress ? p.selectedAddress : null;
    final zipCode = postalCodeByTownCode(addr?.townCode ?? '');
    final storePickup = shipping == ShippingPresentation.cvsStorePickup
        ? p.selectedStorePickup
        : null;

    try {
      final resp = await widget.dio.post<Map<String, dynamic>>(
        '/app/checkout/preview',
        data: {
          'items': p.selectedItems
              .map(
                (i) => {
                  'skuId': i.skuId.toString(),
                  'quantity': i.count.toString(),
                  'productId': p.skuProductIds[i.skuId] ?? '0',
                },
              )
              .toList(),
          'shippingMethod': p.shippingOption['shippingMethod'] ?? '',
          'deliveryTemp': p.shippingOption['deliveryTemp'] ?? '',
          if (shipping.usesRecipientAddress) 'receiverZipCode': zipCode,
          if (shipping == ShippingPresentation.cvsStorePickup &&
              storePickup != null) ...{
            'storeId': storePickup['storeCode']?.toString() ?? '',
            'storeName': storePickup['storeName']?.toString() ?? '',
          },
          if (_appliedCouponId.isNotEmpty) 'couponTemplateId': _appliedCouponId,
        },
      );
      if (!mounted) return;
      final body = resp.data;
      if (body != null && body['code'] == 0) {
        setState(() {
          _previewData = body['data'] as Map<String, dynamic>?;
          _previewLoading = false;
        });
      } else {
        final code = body?['code']?.toString();
        final msg = body?['message']?.toString() ?? '';

        if ((code == '4001' || code == '4002') && mounted) {
          Navigator.of(context).pop();
          if (!widget.parentContext.mounted) return;
          showCommonSnackBar(
            widget.parentContext,
            msg.isNotEmpty ? msg : '部分商品已被其他買家搶先購買，請重新確認數量',
          );
          widget.callbacks.onReloadCart();
          return;
        }

        if (code == '4003' && mounted) {
          Navigator.of(context).pop();
          if (!widget.parentContext.mounted) return;
          showCommonSnackBar(
            widget.parentContext,
            msg.isNotEmpty ? msg : '部分商品已下架，請重新確認購物車',
          );
          widget.callbacks.onReloadCart();
          return;
        }

        if (code == '4015' && mounted) {
          Navigator.of(context).pop();
          if (!widget.parentContext.mounted) return;
          showCommonSnackBar(
            widget.parentContext,
            msg.isNotEmpty ? msg : '不可購買自己上架的商品',
          );
          widget.callbacks.onReloadCart();
          return;
        }

        setState(() {
          _previewError = msg.isNotEmpty ? msg : '預覽失敗，請稍後再試';
          _previewLoading = false;
        });
      }
    } catch (e) {
      if (!mounted) return;
      final isTimeout =
          e is DioException &&
          (e.type == DioExceptionType.connectionTimeout ||
              e.type == DioExceptionType.receiveTimeout);
      setState(() {
        _previewError = isTimeout ? '連線逾時，請檢查網路後重試' : '無法取得結帳資訊，請稍後再試';
        _previewLoading = false;
      });
    }
  }

  Future<void> _refreshPreview() async {
    if (!mounted) return;
    setState(() {
      _previewRequested = false;
      _previewData = null;
      _previewError = null;
      _previewLoading = true;
    });
    await _loadPreview();
  }

  String _couponTemplateIdFrom(Map<String, dynamic> coupon) {
    return coupon['couponId']?.toString() ??
        coupon['coupon_id']?.toString() ??
        coupon['templateId']?.toString() ??
        '';
  }


  Future<void> _doConfirm() async {
    if (_isConfirming) return;
    final checkoutId = _previewData?['checkoutId']?.toString() ?? '';
    if (checkoutId.isEmpty) {
      await showCommonDialog(
        context: context,
        title: '請稍候',
        content: '結帳資訊尚未載入，請稍後再試',
      );
      return;
    }

    final p = widget.params;
    final cb = widget.callbacks;
    final confirmShipping = shippingPresentationOf(
      p.shippingOption['shippingMethod']?.toString(),
    );
    final addr = confirmShipping.usesRecipientAddress
        ? p.selectedAddress
        : null;
    final confirmStore = confirmShipping == ShippingPresentation.cvsStorePickup
        ? p.selectedStorePickup
        : null;

    switch (confirmShipping) {
      case ShippingPresentation.homeDelivery:
        if (addr == null) {
          await showCommonDialog(
            context: context,
            title: '提示',
            content: '請先選擇收件地址',
          );
          return;
        }
      case ShippingPresentation.cvsStorePickup:
        if (confirmStore == null) {
          await showCommonDialog(
            context: context,
            title: '提示',
            content: '請先選擇取貨門市',
          );
          return;
        }
      case ShippingPresentation.unsupported:
        await showCommonDialog(
          context: context,
          title: '提示',
          content: '此配送方式暫不支援，請返回購物車改選其他配送方式',
        );
        return;
    }
    if (!mounted) return;

    _isConfirming = true;
    final zipCode = postalCodeByTownCode(addr?.townCode ?? '');

    TruckLoadingOverlay.show(context, message: context.l10n.paymentProcessing);
    try {
      final confirmResp = await widget.dio.post<Map<String, dynamic>>(
        '/app/checkout/confirm',
        data: {
          'checkoutId': checkoutId,
          'items': p.selectedItems
              .map(
                (i) => {
                  'skuId': i.skuId.toString(),
                  'quantity': i.count.toString(),
                  'productId': p.skuProductIds[i.skuId] ?? '0',
                },
              )
              .toList(),
          'shippingMethod': p.shippingOption['shippingMethod'] ?? '',
          'deliveryTemp': p.shippingOption['deliveryTemp'] ?? '',
          'shippingNote': p.deliveryNotesText,
          'orderRemark': p.deliveryNotesText,
          if (confirmShipping.usesRecipientAddress) ...{
            'receiverZipCode': zipCode,
            'recipientName': addr?.receiverName ?? '',
            'recipientPhone': addr?.receiverPhone ?? '',
            'shippingAddress': addr?.fullAddress ?? '',
          },
          if (confirmShipping == ShippingPresentation.cvsStorePickup &&
              confirmStore != null) ...{
            'storeId': confirmStore['storeCode']?.toString() ?? '',
            'storeName': confirmStore['storeName']?.toString() ?? '',
            'recipientName': confirmStore['receiverName']?.toString() ?? '',
            'recipientPhone': confirmStore['receiverPhone']?.toString() ?? '',
          },
          if (_appliedCouponId.isNotEmpty) 'couponTemplateId': _appliedCouponId,
        },
      );
      if (!mounted) return;
      final confirmBody = confirmResp.data;

      if (confirmBody == null || confirmBody['code'] != 0) {
        TruckLoadingOverlay.hide(context);
        _isConfirming = false;
        await _handleConfirmError(confirmBody);
        return;
      }

      final confirmData = confirmBody['data'] as Map<String, dynamic>? ?? {};
      final orderNos = (confirmData['orderNos'] as List? ?? [])
          .map((e) => e.toString())
          .toList();
      final actualPayAmount = confirmData['totalPayAmount']?.toString() ?? '0';
      final orderNo = orderNos.isNotEmpty ? orderNos.first : '';
      final orderText = orderNos.isNotEmpty
          ? '訂單編號：${orderNos.join(', ')}'
          : '訂單已建立';

      if (orderNo.isEmpty) {
        if (!mounted) return;
        TruckLoadingOverlay.hide(context);
        await showCommonDialog(
          context: context,
          title: '下單成功',
          content: orderText,
        );
        cb.onNavigateHome();
        return;
      }

      await _processPayment(
        orderNo: orderNo,
        orderText: orderText,
        totalPayAmount: actualPayAmount,
        orderNos: orderNos,
      );
    } catch (e) {
      _isConfirming = false;
      if (!mounted) return;
      TruckLoadingOverlay.hide(context);
      if (mounted) {
        await showCommonDialog(
          context: context,
          title: '下單失敗',
          content: '訂單建立失敗，請稍後再試',
        );
      }
    }
  }

  Future<void> _handleConfirmError(Map<String, dynamic>? confirmBody) async {
    final cb = widget.callbacks;
    final confirmCode = confirmBody?['code']?.toString();
    final confirmMsg = confirmBody?['message']?.toString() ?? '';

    if (confirmCode == '4001' || confirmCode == '4002') {
      if (mounted) Navigator.of(context).pop();
      showCommonSnackBar(
        widget.parentContext,
        confirmMsg.isNotEmpty ? confirmMsg : '部分商品已被其他買家搶先購買，請重新確認數量',
      );
      cb.onReloadCart();
      return;
    }

    if (confirmCode == SellerSuspensionCode.sellerSuspendedProductUnavailable) {
      if (mounted) Navigator.of(context).pop();
      showCommonSnackBar(widget.parentContext, '部分商品暫停販售，已自結帳移除');
      cb.onReloadCart();
      return;
    }

    if (confirmCode == '4015') {
      if (mounted) Navigator.of(context).pop();
      showCommonSnackBar(
        widget.parentContext,
        confirmMsg.isNotEmpty ? confirmMsg : '不可購買自己上架的商品',
      );
      cb.onReloadCart();
      return;
    }

    if (confirmCode == '4091' && mounted) {
      Navigator.of(context).pop();
      await showCommonDialog(
        context: widget.parentContext,
        title: '價格已變動',
        content: confirmMsg.isNotEmpty ? confirmMsg : '商品價格已變動，請重新確認訂單',
      );
      cb.onClearCoupons();
      cb.onReloadCart();
      cb.onTriggerShippingFee();
      return;
    }

    if (confirmCode == '4006' && mounted) {
      Navigator.of(context).pop();
      showCommonSnackBar(
        widget.parentContext,
        confirmMsg.isNotEmpty ? confirmMsg : '結帳已過期，請重新操作',
      );
      return;
    }

    debugPrint('[Checkout] confirm error: ${confirmBody?['message']}');
    if (mounted) {
      await showCommonDialog(
        context: context,
        title: '下單失敗',
        content: confirmMsg.isNotEmpty ? confirmMsg : '訂單建立失敗，請稍後再試',
      );
    }
  }

  Future<void> _processPayment({
    required String orderNo,
    required String orderText,
    required String totalPayAmount,
    List<String> orderNos = const [],
  }) async {
    final cb = widget.callbacks;
    final container = ProviderScope.containerOf(context);
    final l10n = context.l10n;

    if (_isZeroAmount(totalPayAmount)) {
      if (!mounted) return;
      TruckLoadingOverlay.hide(context);
      await showCommonDialog(
        context: context,
        title: '付款完成',
        content: '$orderText\n折抵後無需付款，訂單已完成付款。',
      );
      cb.onReloadCart();
      cb.onInvalidateCartCount();
      cb.onNavigateToOrders();
      return;
    }

    PaymentOutcome? outcome;
    String? errorMessage;
    try {
      outcome = await container.read(paymentFlowProvider).pay(orderNo);
    } catch (e) {
      errorMessage = errorText(
        e,
        defaults: container.read(defaultErrorTextProvider),
      );
    }
    if (!mounted) return;
    TruckLoadingOverlay.hide(context);

    if (outcome == PaymentOutcome.succeeded) {
      await showCommonDialog(
        context: context,
        title: l10n.paymentSucceededTitle,
        content: '$orderText\n付款金額：NT\$ $totalPayAmount',
      );
      cb.onReloadCart();
      cb.onInvalidateCartCount();
      for (final no in orderNos.isNotEmpty ? orderNos : [orderNo]) {
        cb.onPaymentSucceeded?.call(no);
      }
      cb.onNavigateHome();
      return;
    }

    final message = errorMessage ?? paymentOutcomeMessage(l10n, outcome!);
    await showCommonDialog(
      context: context,
      title: l10n.paymentNotCompletedTitle,
      content: '$orderText\n$message',
    );
    cb.onReloadCart();
    cb.onInvalidateCartCount();
    cb.onNavigateToOrders();
  }


  /// 付款明細行：icon + label + amount。
  Widget _paymentRow(HkIconName icon, String label, String amount) {
    final labelStyle = CartTypography.body;
    final amountStyle = CartTypography.amount.copyWith(
      color: AppColors.textEmphasis,
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        final requiredWidth =
            20 +
            HkSpace.sm +
            _singleLineWidth(context, label, labelStyle) +
            HkSpace.sm +
            _singleLineWidth(context, amount, amountStyle);
        final stack =
            constraints.maxWidth <
                280 * MediaQuery.textScalerOf(context).scale(16) / 16 ||
            requiredWidth > constraints.maxWidth;
        final labelGroup = Row(
          children: [
            HkIcon(icon, size: 20.sp, color: AppIconColors.defaultColor),
            const SizedBox(width: HkSpace.sm),
            Expanded(
              child: Text(
                label,
                maxLines: 1,
                softWrap: false,
                overflow: TextOverflow.ellipsis,
                style: labelStyle,
              ),
            ),
          ],
        );
        final amountText = Text(
          amount,
          maxLines: 1,
          softWrap: false,
          style: amountStyle,
          textAlign: TextAlign.right,
        );

        if (stack) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              labelGroup,
              const SizedBox(height: HkSpace.xs),
              Align(alignment: Alignment.centerRight, child: amountText),
            ],
          );
        }
        return Row(
          children: [
            Expanded(child: labelGroup),
            const SizedBox(width: HkSpace.sm),
            amountText,
          ],
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final totalOriginal =
        _previewData?['totalOriginalAmount']?.toString() ?? '—';
    final totalShipping = _previewData?['totalShippingFee']?.toString() ?? '—';
    final discountNum =
        double.tryParse(_previewData?['totalDiscount']?.toString() ?? '0') ?? 0;
    final totalActual = _previewData?['totalActualAmount']?.toString() ?? '—';

    final bottomPad = MediaQuery.of(context).padding.bottom;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(height: 20.h),
        Container(
          width: 50.w,
          height: 4.h,
          decoration: BoxDecoration(
            color: AppColors.gray300,
            borderRadius: BorderRadius.circular(2.r),
          ),
        ),
        SizedBox(height: 22.h),

        Flexible(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(24.w, 0, 24.w, 16.h),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (_previewLoading)
                  Padding(
                    padding: EdgeInsets.symmetric(vertical: 32.h),
                    child: const TruckLoadingOverlay(message: '載入中...'),
                  )
                else if (_previewError != null)
                  Padding(
                    padding: EdgeInsets.symmetric(vertical: 24.h),
                    child: Column(
                      children: [
                        Icon(
                          AppIcons.errorOutline,
                          color: AppIconColors.danger,
                          size: 32.sp,
                        ),
                        SizedBox(height: 8.h),
                        Text(
                          _previewError!,
                          style: TextStyle(
                            color: AppColors.danger,
                            fontSize: 14,
                          ),
                          textAlign: TextAlign.center,
                        ),
                      ],
                    ),
                  )
                else ...[
                  _buildPaymentDetailsCard(
                    totalOriginal,
                    totalShipping,
                    discountNum,
                    totalActual,
                  ),
                  SizedBox(height: 22.h),

                  if (_isZeroAmount(totalActual)) ...[
                    _buildZeroAmountCard(),
                    SizedBox(height: 16.h),
                  ],
                ],
              ],
            ),
          ),
        ),

        Padding(
          padding: EdgeInsets.fromLTRB(24.w, 0, 24.w, bottomPad + 12),
          child: SwipeToPay(
            onComplete: () async {
              if (_previewLoading || _previewData == null) {
                await showCommonDialog(
                  context: context,
                  title: '請稍候',
                  content: '結帳資訊尚未載入',
                );
                return;
              }
              await _doConfirm();
            },
          ),
        ),
      ],
    );
  }

  Widget _buildPaymentDetailsCard(
    String totalOriginal,
    String totalShipping,
    double discountNum,
    String totalActual,
  ) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 20.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.textPrimary.withValues(alpha: 0.2)),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Row(
            children: [
              HkIcon(
                HkIconName.payment,
                size: 24.sp,
                color: AppIconColors.strong,
              ),
              SizedBox(width: 6.w),
              Text('付款詳情', style: CartTypography.section),
            ],
          ),
          SizedBox(height: 10.h),
          _paymentRow(HkIconName.goods, '商品總金額', 'NT\$ $totalOriginal'),
          SizedBox(height: 10.h),
          _paymentRow(HkIconName.truck, '運費總金額', 'NT\$ $totalShipping'),
          SizedBox(height: 10.h),
          _buildCouponRow(discountNum),
          SizedBox(height: 10.h),
          const Divider(height: 1, color: AppColors.dividerLight),
          SizedBox(height: 10.h),
          _buildTotalRow(totalActual),
        ],
      ),
    );
  }

  Widget _buildZeroAmountCard() {
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.textPrimary.withValues(alpha: 0.2)),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Row(
        children: [
          Icon(
            AppIcons.checkCircleRounded,
            size: 20.sp,
            color: AppIconColors.active,
          ),
          SizedBox(width: 8.w),
          Expanded(
            child: Text('折抵後無需付款，送出後系統會自動完成付款。', style: CartTypography.body),
          ),
        ],
      ),
    );
  }

  Widget _buildCouponRow(double discountNum) {
    final labelStyle = CartTypography.body;
    final discountStyle = CartTypography.amount;
    const buttonLabel = '選擇優惠券';
    final buttonStyle = CartTypography.button.copyWith(
      color: AppColors.textInverse,
    );
    final discount = 'NT\$ -${discountNum.toStringAsFixed(0)}';

    final labelGroup = Row(
      children: [
        HkIcon(HkIconName.tag, size: 24.sp, color: AppIconColors.defaultColor),
        const SizedBox(width: HkSpace.md),
        Expanded(
          child: Text(
            '折扣券',
            maxLines: 1,
            softWrap: false,
            overflow: TextOverflow.ellipsis,
            style: labelStyle,
          ),
        ),
      ],
    );

    Widget couponButton() => FilledButton(
      onPressed: widget.params.canSelectCoupon
          ? () async {
              final selected = await widget.callbacks.onShowCouponSheet();
              if (!mounted || selected == null) return;
              final couponId = _couponTemplateIdFrom(selected);
              if (couponId.isEmpty) return;
              _appliedCouponId = couponId;
              await _refreshPreview();
            }
          : null,
      style: FilledButton.styleFrom(
        minimumSize: const Size(0, HkControl.minTap),
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
        backgroundColor: AppColors.primary,
        disabledBackgroundColor: AppColors.primary.withValues(alpha: 0.4),
        disabledForegroundColor: AppColors.textInverse,
        shape: const StadiumBorder(),
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
      ),
      child: HkButtonLabel(buttonLabel, style: buttonStyle),
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        final requiredWidth =
            24 +
            HkSpace.md +
            _singleLineWidth(context, '折扣券', labelStyle) +
            HkSpace.sm +
            _singleLineWidth(context, buttonLabel, buttonStyle) +
            HkSpace.md * 2 +
            (discountNum > 0
                ? HkSpace.sm +
                      _singleLineWidth(context, discount, discountStyle)
                : 0);
        final stack =
            constraints.maxWidth <
                280 * MediaQuery.textScalerOf(context).scale(16) / 16 ||
            requiredWidth > constraints.maxWidth;
        final actions = Wrap(
          alignment: WrapAlignment.end,
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: HkSpace.sm,
          runSpacing: HkSpace.xs,
          children: [
            if (discountNum > 0)
              Text(
                discount,
                maxLines: 1,
                softWrap: false,
                style: discountStyle,
              ),
            couponButton(),
          ],
        );

        if (stack) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              labelGroup,
              const SizedBox(height: HkSpace.xs),
              actions,
            ],
          );
        }
        return Row(
          children: [
            Expanded(child: labelGroup),
            const SizedBox(width: HkSpace.sm),
            actions,
          ],
        );
      },
    );
  }

  Widget _buildTotalRow(String totalActual) {
    final labelStyle = CartTypography.body;
    final amountStyle = CartTypography.total.copyWith(
      color: AppColors.textEmphasis,
    );
    final amount = 'NT\$ $totalActual';

    return LayoutBuilder(
      builder: (context, constraints) {
        final stack =
            constraints.maxWidth <
                280 * MediaQuery.textScalerOf(context).scale(16) / 16 ||
            _singleLineWidth(context, '總付款金額', labelStyle) +
                    HkSpace.sm +
                    _singleLineWidth(context, amount, amountStyle) >
                constraints.maxWidth;
        final label = Text(
          '總付款金額',
          maxLines: 1,
          softWrap: false,
          style: labelStyle,
        );
        final value = Text(
          amount,
          maxLines: 1,
          softWrap: false,
          style: amountStyle,
          textAlign: TextAlign.right,
        );

        if (stack) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              label,
              const SizedBox(height: HkSpace.xs),
              Align(alignment: Alignment.centerRight, child: value),
            ],
          );
        }
        return Row(
          children: [
            Expanded(child: label),
            const SizedBox(width: HkSpace.sm),
            value,
          ],
        );
      },
    );
  }

  double _singleLineWidth(BuildContext context, String value, TextStyle style) {
    final painter = TextPainter(
      text: TextSpan(text: value, style: style),
      maxLines: 1,
      textDirection: Directionality.of(context),
      textScaler: MediaQuery.textScalerOf(context),
    )..layout();
    final width = painter.width;
    painter.dispose();
    return width;
  }
}

bool _isZeroAmount(String value) {
  final cleaned = value.replaceAll(RegExp(r'[^0-9.-]'), '');
  if (cleaned.isEmpty) return false;
  return (num.tryParse(cleaned) ?? -1) == 0;
}

/// 開啟結帳付款底部彈窗。
///
/// 封裝 [showModalBottomSheet] 的呼叫，回傳 Future。
Future<void> showCheckoutPaymentSheet({
  required BuildContext context,
  required CheckoutParams params,
  required CheckoutCallbacks callbacks,
}) {
  final dio = ProviderScope.containerOf(context).read(dioProvider);
  return showModalBottomSheet(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: false,
    backgroundColor: AppColors.bgPrimary,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
    ),
    builder: (ctx) => CheckoutPaymentSheet(
      params: params,
      callbacks: callbacks,
      dio: dio,
      parentContext: context,
    ),
  );
}
