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
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/core/codemeta/shipping_presentation.dart';

import '../../../../core/exceptions/seller_suspension_codes.dart';
import '../../../../core/networking/dio_provider.dart';
import '../../../../core/payment/pay_token_service.dart';
import '../../../../shared/widgets/common_dialog.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/floating_toast.dart';
import '../../../../shared/widgets/swipe_to_pay.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../../address/data/address_model.dart';
import '../../data/cart_model.dart';

// ---------------------------------------------------------------------------
// 結帳付款底部彈窗。
//
// 呈現結帳預覽資訊（商品總額、運費、折扣、總付款金額）、
// 付款方式選擇（Apple Pay / ATM / 信用卡），
// 以及滑動確認付款。
//
// 內部管理 preview API 呼叫、confirm + payment 流程。
// ---------------------------------------------------------------------------

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
    required this.onNavigateToPaymentWebview,
    required this.onInvalidateCartCount,
    this.onPayMethodChanged,
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

  /// 導向信用卡 WebView 付款。
  final void Function({
    required String redirectUrl,
    required String tradeNo,
    required String orderNo,
    required String amount,
  })
  onNavigateToPaymentWebview;

  /// 讓主頁面 invalidate cartCount。
  final VoidCallback onInvalidateCartCount;

  /// 付款方式切換時回調（傳入 payType 字串）。
  final void Function(String payType)? onPayMethodChanged;

  /// Apple Pay 付款成功後回調（傳入已確認付款的 orderNo）。
  /// 用於觸發 GA4 purchase 事件；credit card 走 WebView 回調，ATM 未付款，
  /// 故此 callback 只在即時付款成功路徑呼叫。
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
  // 付款方式列表（App 只發 iOS，錢包是 Apple Pay）
  List<({String label, String payType})> get _payMethods => [
    (label: 'Apple Pay', payType: PaymentMethod.applePay.value),
    (label: 'ATM 虛擬轉帳', payType: PaymentMethod.atm.value),
    (label: '信用卡', payType: PaymentMethod.creditCard.value),
  ];

  // 後端 §3.4.1 付款方式可用性：依金額判定哪些方式可用 + 不可用理由。
  // 空清單（未載入 / 舊後端 / 零元）graceful 視為不限制；零元另由 _isZeroAmount 走無需付款分支。
  ({bool eligible, String reason}) _availabilityFor(String payType) {
    final list = _previewData?['availablePaymentMethods'];
    if (list is! List || list.isEmpty) return (eligible: true, reason: '');
    for (final m in list) {
      if (m is Map && m['paymentType']?.toString() == payType) {
        return (
          eligible: m['eligible'] == true,
          reason: m['ineligibleReason']?.toString() ?? '',
        );
      }
    }
    return (eligible: true, reason: '');
  }

  int _payMethodIdx = 0;
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

  // ── Preview API ──

  Future<void> _loadPreview() async {
    if (_previewRequested) return;
    _previewRequested = true;

    final p = widget.params;
    // preview payload 依配送方式**明確分派**。
    // 🔴 不得用 `!isStorePickup` 當宅配：未知 / 本版未支援的配送方式會因此帶著
    // receiverZipCode、不帶 storeId 送進 /app/checkout/preview（請求面 P0）。
    final shipping = shippingPresentationOf(
      p.shippingOption['shippingMethod']?.toString(),
    );
    if (shipping == ShippingPresentation.unsupported) {
      // 首次呼叫來自 initState（第一次 build 之前），該時點 setState 會 assert；
      // 先讓出一個 event loop turn 再更新狀態，retry 路徑（_refreshPreview）亦適用。
      await Future<void>.delayed(Duration.zero);
      if (!mounted) return;
      setState(() {
        _previewLoading = false;
        _previewError = '此配送方式暫不支援，請返回購物車改選其他配送方式';
      });
      return;
    }
    // 判準是 [ShippingPresentationX.usesRecipientAddress]，與 confirm 同一份。
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

        // 庫存不足
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

        // 商品下架
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

        // 自買防護
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

  // ── Confirm + Pay ──

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

    // 後端 §3.4.1 金額 gating 守門：非零元時選中方式必須 eligible（前端已禁點，此為保險）
    final guardZero = _isZeroAmount(
      _previewData?['totalActualAmount']?.toString() ?? '',
    );
    if (!guardZero &&
        !_availabilityFor(_payMethods[_payMethodIdx].payType).eligible) {
      await showCommonDialog(
        context: context,
        title: '提示',
        content: '所選付款方式不適用本訂單金額，請改選其他付款方式',
      );
      return;
    }

    final p = widget.params;
    final cb = widget.callbacks;
    // confirm payload 依配送方式**明確分派**（與 _loadPreview 同一套判定）。
    // 🔴 不得用 `!confirmIsStorePickup` 當宅配：那會讓未知 / 本版未支援的配送方式
    // 帶著 receiverZipCode + shippingAddress 建出一張語意錯誤的訂單。
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
    // switch 的每個分支若彈過 dialog 都已 return，但 analyzer 對 switch 的
    // async gap 分析較保守；這道 guard 同時是正確行為（跨 await 後 widget 可能已卸載）。
    if (!mounted) return;

    _isConfirming = true;
    final zipCode = postalCodeByTownCode(addr?.townCode ?? '');

    TruckLoadingOverlay.show(context, message: '處理中...');
    try {
      // Step 1: 確認訂單
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

      // Step 2: 付款
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

      final payType = _payMethods[_payMethodIdx].payType;
      await _processPayment(
        payType: payType,
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

    // 庫存問題
    if (confirmCode == '4001' || confirmCode == '4002') {
      if (mounted) Navigator.of(context).pop();
      showCommonSnackBar(
        widget.parentContext,
        confirmMsg.isNotEmpty ? confirmMsg : '部分商品已被其他買家搶先購買，請重新確認數量',
      );
      cb.onReloadCart();
      return;
    }

    // 賣家停權 — 商品暫停販售（46 號設計書「賣家能力權限層」§7.2；文案為
    // 使用者 2026-07-13 圈選定案，不採後端 message，固定顯示此文案）
    if (confirmCode == SellerSuspensionCode.sellerSuspendedProductUnavailable) {
      if (mounted) Navigator.of(context).pop();
      showCommonSnackBar(widget.parentContext, '部分商品暫停販售，已自結帳移除');
      cb.onReloadCart();
      return;
    }

    // 自買防護
    if (confirmCode == '4015') {
      if (mounted) Navigator.of(context).pop();
      showCommonSnackBar(
        widget.parentContext,
        confirmMsg.isNotEmpty ? confirmMsg : '不可購買自己上架的商品',
      );
      cb.onReloadCart();
      return;
    }

    // 價格變動
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

    // 結帳過期
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
    required String payType,
    required String orderNo,
    required String orderText,
    required String totalPayAmount,
    List<String> orderNos = const [],
  }) async {
    final cb = widget.callbacks;
    final isZeroAmount = _isZeroAmount(totalPayAmount);

    try {
      // Apple Pay：先取得 token
      String? nativePayToken;
      if (!isZeroAmount && payType == PaymentMethod.applePay.value) {
        if (mounted) TruckLoadingOverlay.hide(context);

        try {
          final payTokenService = PayTokenService();
          await payTokenService.init();
          if (!payTokenService.isAvailable) {
            if (mounted) {
              await showCommonDialog(
                context: context,
                title: '不支援',
                content:
                    '此裝置不支援 Apple Pay，'
                    '請選擇其他付款方式或至「我的訂單」重新付款',
              );
              cb.onNavigateToOrders();
            }
            return;
          }
          nativePayToken = await payTokenService.requestToken(
            amount: totalPayAmount,
            label: 'Hawkr 訂單',
          );
        } on PaymentCancelledException {
          if (mounted) {
            await showCommonDialog(
              context: context,
              title: '付款取消',
              content: '訂單已建立（$orderText），您可以到「我的訂單」重新付款',
            );
            cb.onNavigateToOrders();
          }
          return;
        } on PaymentTokenUnavailableException catch (e) {
          if (mounted) {
            await showCommonDialog(
              context: context,
              title: '付款憑證取得失敗',
              content: '訂單已建立（$orderText），$e',
            );
            cb.onNavigateToOrders();
          }
          return;
        } catch (e, stackTrace) {
          debugPrint('[CheckoutPaymentSheet] native pay token failed: $e');
          debugPrintStack(stackTrace: stackTrace);
          if (mounted) {
            await showCommonDialog(
              context: context,
              title: '付款授權失敗',
              content: '訂單已建立（$orderText），請到「我的訂單」重新付款或改用其他付款方式',
            );
            cb.onNavigateToOrders();
          }
          return;
        }

        if (!mounted) return;
        TruckLoadingOverlay.show(context, message: '處理中...');
      }

      final payResp = await widget.dio.post<Map<String, dynamic>>(
        '/app/payments',
        data: {
          'orderNo': orderNo,
          'paymentType': payType,
          if (payType == PaymentMethod.applePay.value &&
              (nativePayToken?.trim().isNotEmpty ?? false))
            'applePayToken': nativePayToken!.trim(),
        },
      );
      if (!mounted) return;
      TruckLoadingOverlay.hide(context);

      final payCode = payResp.data?['code'] as int? ?? -1;
      if (payCode != 0) {
        final message = payCode == 8021
            // 8021 付款方式金額不符（前端 gating 已擋，此為 fallback）
            ? '所選付款方式不適用本訂單金額，請改選其他付款方式'
            : payResp.data?['message']?.toString() ?? '付款請求失敗';
        debugPrint('[Payment] API error: $message');
        await showCommonDialog(
          context: context,
          title: '付款處理失敗',
          content: '訂單已建立（$orderText），$message\n請到「我的訂單」重新付款',
        );
        cb.onNavigateToOrders();
        return;
      }

      final payData = payResp.data?['data'] as Map<String, dynamic>? ?? {};
      final tradeNo = payData['tradeNo']?.toString() ?? '';
      final responsePaymentType =
          payData['paymentType']?.toString() ??
          payData['paymentMethod']?.toString() ??
          '';
      final paidWithoutRedirect =
          _isPaidPaymentStatus(payData['status']) &&
          (isZeroAmount ||
              responsePaymentType == PaymentMethod.zeroAmount.value ||
              _isZeroAmount(payData['amount']?.toString() ?? ''));

      if (paidWithoutRedirect) {
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

      if (payType == PaymentMethod.creditCard.value) {
        // 信用卡 UPP
        final redirectUrl = payData['redirectUrl']?.toString() ?? '';
        if (redirectUrl.isNotEmpty) {
          cb.onReloadCart();
          cb.onInvalidateCartCount();
          cb.onNavigateToPaymentWebview(
            redirectUrl: redirectUrl,
            tradeNo: tradeNo,
            orderNo: orderNo,
            amount: totalPayAmount,
          );
        } else {
          await showCommonDialog(
            context: context,
            title: '付款處理失敗',
            content: '訂單已建立（$orderText），付款頁面無法開啟，請到「我的訂單」重新付款',
          );
          cb.onNavigateToOrders();
        }
      } else if (payType == PaymentMethod.atm.value) {
        // ATM 虛擬轉帳
        final bankCode = payData['atmBankCode']?.toString() ?? '';
        final virtualAccount = payData['atmVirtualAccount']?.toString() ?? '';
        FloatingToast.show(
          title: 'ATM 轉帳資訊',
          message: '銀行代碼：$bankCode\n虛擬帳號：$virtualAccount',
          icon: AppIcons.accountBalance,
          duration: const Duration(seconds: 8),
        );
        cb.onNavigateToOrders();
      } else if (payType == PaymentMethod.applePay.value) {
        await showCommonDialog(
          context: context,
          title: '付款成功',
          content: '$orderText\n付款金額：NT\$ $totalPayAmount',
        );
        cb.onReloadCart();
        cb.onInvalidateCartCount();
        // GA4 purchase：對每筆已付款 orderNo 觸發一次（多賣家拆單各呼叫一次）。
        final trackedNos = orderNos.isNotEmpty
            ? orderNos
            : (orderNo.isNotEmpty ? [orderNo] : <String>[]);
        for (final no in trackedNos) {
          cb.onPaymentSucceeded?.call(no);
        }
        cb.onNavigateHome();
      } else {
        await showCommonDialog(
          context: context,
          title: '付款處理失敗',
          content: '訂單已建立（$orderText），不支援的付款方式，請到「我的訂單」重新付款',
        );
        cb.onNavigateToOrders();
      }
    } on DioException catch (e) {
      if (!mounted) return;
      TruckLoadingOverlay.hide(context);
      final data = e.response?.data;
      final message = data is Map ? data['message']?.toString() : null;
      await showCommonDialog(
        context: context,
        title: '付款處理失敗',
        content:
            '訂單已建立（$orderText），${message?.trim().isNotEmpty == true ? message : '請到「我的訂單」重新付款'}',
      );
      cb.onNavigateToOrders();
    } catch (_) {
      if (!mounted) return;
      TruckLoadingOverlay.hide(context);
      await showCommonDialog(
        context: context,
        title: '付款處理失敗',
        content: '訂單已建立（$orderText），請到「我的訂單」重新付款',
      );
      cb.onNavigateToOrders();
    }
  }

  // ── UI ──

  Widget _payMethodIcon(String payType) {
    if (payType == PaymentMethod.atm.value) {
      return Text(
        'ATM',
        style: TextStyle(
          color: AppColors.primary,
          fontWeight: FontWeight.bold,
          fontSize: 20,
        ),
      );
    }
    return const SizedBox.shrink();
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
    // 從 preview 計算顯示值
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
        // 拖拽手柄
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

        // 可滾動內容
        Flexible(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(24.w, 0, 24.w, 16.h),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // Loading / Error
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
                  // 付款詳情卡片
                  _buildPaymentDetailsCard(
                    totalOriginal,
                    totalShipping,
                    discountNum,
                    totalActual,
                  ),
                  SizedBox(height: 22.h),

                  // 付款方式卡片
                  _buildPaymentMethodCard(),
                  SizedBox(height: 16.h),
                ],
              ],
            ),
          ),
        ),

        // 底部滑動付款
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
          // 標題
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
          // 折扣券
          _buildCouponRow(discountNum),
          SizedBox(height: 10.h),
          const Divider(height: 1, color: AppColors.dividerLight),
          SizedBox(height: 10.h),
          // 總付款金額
          _buildTotalRow(totalActual),
        ],
      ),
    );
  }

  Widget _buildPaymentMethodCard() {
    final zeroAmount = _isZeroAmount(
      _previewData?['totalActualAmount']?.toString() ?? '',
    );
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
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              HkIcon(
                HkIconName.payment,
                size: 24.sp,
                color: AppIconColors.strong,
              ),
              SizedBox(width: 6.w),
              Text('付款方式', style: CartTypography.section),
            ],
          ),
          SizedBox(height: 12.h),
          if (zeroAmount)
            Row(
              children: [
                Icon(
                  AppIcons.checkCircleRounded,
                  size: 20.sp,
                  color: AppIconColors.active,
                ),
                SizedBox(width: 8.w),
                Expanded(
                  child: Text(
                    '折抵後無需付款，送出後系統會自動完成付款。',
                    style: CartTypography.body,
                  ),
                ),
              ],
            )
          else
            // Block UI 對齊再次付款彈窗：label Expanded 而非固定寬，ATM 下方加虛擬帳號標語。
            ..._payMethods.asMap().entries.expand((e) {
              final idx = e.key;
              final method = e.value;
              final isSelected = idx == _payMethodIdx;
              // 後端 §3.4.1 金額 gating：不符金額 → 灰化 + 禁點 + 顯示理由
              final avail = _availabilityFor(method.payType);
              final eligible = avail.eligible;
              final isActiveSelection = isSelected && eligible;
              final onSelect = eligible
                  ? () {
                      setState(() => _payMethodIdx = idx);
                      widget.callbacks.onPayMethodChanged?.call(method.payType);
                    }
                  : null;
              return [
                Semantics(
                  container: true,
                  button: true,
                  enabled: eligible,
                  selected: isActiveSelection,
                  inMutuallyExclusiveGroup: true,
                  label: method.label,
                  hint: eligible
                      ? '點兩下選擇付款方式'
                      : (_kIneligibleReasonLabel[avail.reason] ??
                            '此付款方式不適用本訂單金額'),
                  onTap: onSelect,
                  child: ExcludeSemantics(
                    child: GestureDetector(
                      behavior: HitTestBehavior.opaque,
                      onTap: onSelect,
                      child: Opacity(
                        opacity: eligible ? 1.0 : 0.45,
                        child: ConstrainedBox(
                          constraints: const BoxConstraints(
                            minHeight: HkControl.minTap,
                          ),
                          child: Padding(
                            padding: EdgeInsets.symmetric(vertical: 8.h),
                            child: Row(
                              children: [
                                Container(
                                  width: 16.w,
                                  height: 16.w,
                                  decoration: BoxDecoration(
                                    shape: BoxShape.circle,
                                    color: AppColors.bgCard,
                                    border: Border.all(
                                      color: AppColors.textEmphasis.withValues(
                                        alpha: 0.2,
                                      ),
                                    ),
                                    boxShadow: [
                                      BoxShadow(
                                        color: AppColors.cardShadow,
                                        blurRadius: 2,
                                        offset: const Offset(0, 1),
                                      ),
                                    ],
                                  ),
                                  child: (isSelected && eligible)
                                      ? Center(
                                          child: Container(
                                            width: 8.w,
                                            height: 8.w,
                                            decoration: const BoxDecoration(
                                              shape: BoxShape.circle,
                                              color: AppColors.primary,
                                            ),
                                          ),
                                        )
                                      : null,
                                ),
                                SizedBox(width: 11.w),
                                Expanded(
                                  child: Text(
                                    method.label,
                                    maxLines: 1,
                                    softWrap: false,
                                    style: CartTypography.body.copyWith(
                                      color: AppColors.textEmphasis,
                                    ),
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ),
                                _payMethodIcon(method.payType),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
                if (!eligible && avail.reason.isNotEmpty)
                  Padding(
                    padding: EdgeInsets.only(left: 24.w, bottom: 4.h),
                    child: Text(
                      _kIneligibleReasonLabel[avail.reason] ?? '此付款方式不適用本訂單金額',
                      style: CartTypography.support.copyWith(
                        color: AppColors.warningText,
                      ),
                    ),
                  )
                else if (method.payType == PaymentMethod.atm.value)
                  Padding(
                    padding: EdgeInsets.only(left: 24.w, bottom: 4.h),
                    child: Text(
                      '虛擬帳號只能用網路銀行跟ATM轉帳，不能臨櫃匯款',
                      style: CartTypography.supportMuted,
                    ),
                  ),
                if (idx < _payMethods.length - 1)
                  Divider(
                    height: 1,
                    color: AppColors.textPrimary.withValues(alpha: 0.1),
                  ),
              ];
            }),
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

// 付款方式不可用理由碼 → 通用文案（對齊後端 PaymentMethodIneligibleReason；§3.4.1）
const Map<String, String> _kIneligibleReasonLabel = {
  '2101': '金額未達此付款方式下限',
  '2102': '金額超過此付款方式上限',
};

bool _isZeroAmount(String value) {
  final cleaned = value.replaceAll(RegExp(r'[^0-9.-]'), '');
  if (cleaned.isEmpty) return false;
  return (num.tryParse(cleaned) ?? -1) == 0;
}

bool _isPaidPaymentStatus(dynamic value) {
  final status = value?.toString().trim().toUpperCase() ?? '';
  return status == '1' ||
      status == 'PAID' ||
      status == 'SUCCESS' ||
      status == '1102';
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
    // 內容已提供專用拖曳把手；關閉 Theme 的預設把手，避免畫出兩條。
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
