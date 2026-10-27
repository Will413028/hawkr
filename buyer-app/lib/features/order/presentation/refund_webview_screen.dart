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

/// ATM 退款資訊填寫 WebView
///
/// 流程（對齊 B-104 / `09-AfterSales Service:1624-1695, 2416-2445`）：
/// 1. 後端 `POST /app/orders/{orderNo}/refund_page` 取 PayUni 退款頁 URL
/// 2. 載入該 URL，買家填寫戶名/銀行代碼/帳號/UID
/// 3. PayUni Form POST 回 Gateway `/atm-refund/return`
/// 4. Gateway 解密驗章後 302 redirect 到
///    `hawkrbuyer://refund/result?orderNo=xxx&status=success|failure`
/// 5. WebView 攔截 deep link → Toast → 導回訂單詳情
class RefundWebViewScreen extends StatefulWidget {
  const RefundWebViewScreen({
    super.key,
    required this.redirectUrl,
    required this.orderNo,
  });

  final String redirectUrl;
  final String orderNo;

  @override
  State<RefundWebViewScreen> createState() => _RefundWebViewScreenState();
}

class _RefundWebViewScreenState extends State<RefundWebViewScreen> {
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

    // B-104：hawkrbuyer://refund/result
    //   ?orderNo=xxx
    //   &status=success|failed
    //   &refundAmount={金額，元，字串}
    //   &estimatedRefundDate={ISO 8601 UTC}
    //   &error={url_encoded 失敗訊息}
    if (uri.scheme == AppConstants.paymentDeepLinkScheme &&
        uri.host == 'refund') {
      final status = uri.queryParameters['status']?.toLowerCase() ?? '';
      final refundAmount = uri.queryParameters['refundAmount'] ?? '';
      final estimatedDate = uri.queryParameters['estimatedRefundDate'] ?? '';
      final error = uri.queryParameters['error'] ?? '';
      _handleResult(status, refundAmount, estimatedDate, error);
      return NavigationDecision.prevent;
    }

    // 安全：付款 WebView 僅允許 http/https 導頁（PayUni / 3DS 銀行頁），
    // 阻擋 javascript:/file:/data:/about: 等可被注入濫用的 scheme。
    if (uri.scheme != 'http' && uri.scheme != 'https') {
      return NavigationDecision.prevent;
    }
    return NavigationDecision.navigate;
  }

  void _handleResult(
    String status,
    String refundAmount,
    String estimatedDate,
    String error,
  ) {
    if (!mounted) return;

    void goToAfterSale() {
      // 回訂單詳情（買家按「查看售後」會進售後詳情）
      context.go('/orders/detail', extra: {'orderSn': widget.orderNo});
    }

    switch (status) {
      case 'success':
        // B-114：success 時顯示金額 + 預計入帳日期
        final parts = <String>[];
        if (refundAmount.isNotEmpty) parts.add('退款金額 NT\$ $refundAmount');
        if (estimatedDate.isNotEmpty) {
          final date = _formatIsoDate(estimatedDate);
          parts.add('預計 $date 入帳');
        }
        final message = parts.isNotEmpty
            ? parts.join('，')
            : '約 2-3 個工作天內將匯入您提供的帳戶';
        FloatingToast.show(
          title: '退款資訊已送出',
          message: message,
          icon: AppIcons.checkCircle,
          iconColor: AppColors.primary,
          duration: const Duration(seconds: 4),
        );
        goToAfterSale();
      default:
        // failed or unknown — 後端會在 error 參數帶失敗訊息
        FloatingToast.show(
          title: '退款資訊送出失敗',
          message: error.isNotEmpty ? error : '請稍後至「我的訂單」重新填寫，或聯繫客服',
          icon: AppIcons.errorOutline,
          iconColor: AppColors.danger,
          duration: const Duration(seconds: 5),
        );
        goToAfterSale();
    }
  }

  /// ISO 8601 UTC → "yyyy-MM-dd"（顯示用）
  String _formatIsoDate(String iso) {
    final dt = DateTime.tryParse(iso);
    if (dt == null) return iso;
    final local = dt.toLocal();
    return '${local.year}-${local.month.toString().padLeft(2, '0')}-${local.day.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: buildCommonTitleAppBar(
        context,
        title: '填寫退款資訊',
        backIcon: AppIcons.close,
        onBack: _showCancelDialog,
      ),
      body: Stack(
        children: [
          WebViewWidget(controller: _controller),
          if (_isLoading) const TruckLoadingOverlay(message: '載入退款頁面...'),
        ],
      ),
    );
  }

  Future<void> _showCancelDialog() async {
    final shouldLeave = await showCommonDialog(
      context: context,
      title: '確定離開？',
      content: '尚未完成退款資訊填寫，離開後可到訂單頁重新填寫。',
      confirmText: '離開',
      cancelText: '繼續填寫',
      confirmColor: AppColors.danger,
      barrierDismissible: true,
    );
    if (shouldLeave == true && mounted) {
      context.go('/orders/detail', extra: {'orderSn': widget.orderNo});
    }
  }
}
