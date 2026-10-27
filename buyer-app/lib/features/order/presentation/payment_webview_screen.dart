import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../../config/app_constants.dart';
import '../../../shared/widgets/common_dialog.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/floating_toast.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';

/// 信用卡 UPP 付款 WebView 頁面
///
/// 流程：
/// 1. 載入 PayUni UPP 付款頁面 (redirectUrl)
/// 2. 付款完成 → PayUni 導向後端 ReturnURL
/// 3. 後端回傳結果頁並觸發 deep link，帶 status 參數：
///    hawkrbuyer://payment/result?tradeNo=xxx&status=SUCCESS
///    hawkrbuyer://payment/result?tradeNo=xxx&status=FAILED
/// 4. WebView 攔截 deep link → 依 status 顯示 Toast → 導回訂單流程
class PaymentWebViewScreen extends StatefulWidget {
  const PaymentWebViewScreen({
    super.key,
    required this.redirectUrl,
    required this.tradeNo,
    required this.orderNo,
    required this.amount,
  });

  final String redirectUrl;
  final String tradeNo;
  final String orderNo;
  final String amount;

  @override
  State<PaymentWebViewScreen> createState() => _PaymentWebViewScreenState();
}

class _PaymentWebViewScreenState extends State<PaymentWebViewScreen> {
  late final WebViewController _controller;
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setNavigationDelegate(
        NavigationDelegate(
          onNavigationRequest: _onNavigationRequest,
          onPageStarted: (_) {
            if (mounted) setState(() => _isLoading = true);
          },
          onPageFinished: (_) {
            if (mounted) setState(() => _isLoading = false);
          },
        ),
      )
      ..loadRequest(Uri.parse(widget.redirectUrl));
  }

  NavigationDecision _onNavigationRequest(NavigationRequest request) {
    final uri = Uri.tryParse(request.url);
    if (uri == null) return NavigationDecision.navigate;

    // 攔截 deep link: hawkrbuyer://payment/result?tradeNo=xxx&status=SUCCESS
    if (uri.scheme == AppConstants.paymentDeepLinkScheme) {
      final status = uri.queryParameters['status']?.toUpperCase() ?? '';
      _handleResult(status);
      return NavigationDecision.prevent;
    }

    // 安全：付款 WebView 僅允許 http/https 導頁（PayUni / 3DS 銀行頁），
    // 阻擋 javascript:/file:/data:/about: 等可被注入濫用的 scheme。
    if (uri.scheme != 'http' && uri.scheme != 'https') {
      return NavigationDecision.prevent;
    }
    return NavigationDecision.navigate;
  }

  void _handleResult(String status) {
    if (!mounted) return;

    void goToOrderFlow() {
      if (widget.orderNo.isNotEmpty) {
        context.go(
          '/orders/detail',
          extra: {'orderSn': widget.orderNo, 'fromCheckout': true},
        );
      } else {
        context.go('/?tab=3');
      }
    }

    switch (status) {
      case 'SUCCESS' || 'PAID':
        FloatingToast.show(
          title: '付款成功',
          message: '訂單編號：${widget.orderNo}\n付款金額：NT\$ ${widget.amount}',
          icon: AppIcons.checkCircle,
          iconColor: AppColors.primary,
          duration: const Duration(seconds: 4),
        );
        goToOrderFlow();
      case 'PENDING':
        FloatingToast.show(
          title: '付款處理中',
          message: '訂單已建立（${widget.orderNo}），請稍後查看訂單狀態',
          icon: AppIcons.hourglassEmpty,
          iconColor: AppColors.warningText,
          duration: const Duration(seconds: 5),
        );
        goToOrderFlow();
      default:
        // FAILED or unknown
        FloatingToast.show(
          title: '付款失敗',
          message: '訂單已建立（${widget.orderNo}），請到「我的訂單」重新付款',
          icon: AppIcons.errorOutline,
          iconColor: AppColors.danger,
          duration: const Duration(seconds: 5),
        );
        goToOrderFlow();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: buildCommonTitleAppBar(
        context,
        title: '信用卡付款',
        backIcon: AppIcons.close,
        onBack: _showCancelDialog,
      ),
      body: Stack(
        children: [
          WebViewWidget(controller: _controller),
          if (_isLoading) const TruckLoadingOverlay(message: '付款處理中...'),
        ],
      ),
    );
  }

  Future<void> _showCancelDialog() async {
    final shouldLeave = await showCommonDialog(
      context: context,
      title: '確定離開？',
      content: '付款尚未完成，離開後可到「我的訂單」重新付款。',
      confirmText: '離開',
      cancelText: '繼續付款',
      confirmColor: AppColors.danger,
      barrierDismissible: true,
    );
    if (shouldLeave == true && mounted) {
      if (widget.orderNo.isNotEmpty) {
        context.go('/orders/detail', extra: {'orderSn': widget.orderNo});
      } else {
        context.go('/?tab=3');
      }
    }
  }
}
