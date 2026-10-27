import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../../shared/widgets/truck_loading_overlay.dart';

/// 7-ELEVEN 門市地圖 WebView
///
/// 流程：
/// 1. 載入 redirectUrl（PayUni 7-ELEVEN 電子地圖）
/// 2. 使用者選擇門市後，頁面觸發 deep link：
///    hawkrseller://store_map/result?storeId=916712&storeName=敦安門市&address=...
/// 3. 攔截 deep link → 解析參數 → pop 回傳 Map 結果
class StoreMapWebViewScreen extends StatefulWidget {
  const StoreMapWebViewScreen({super.key, required this.redirectUrl});

  final String redirectUrl;

  @override
  State<StoreMapWebViewScreen> createState() => _StoreMapWebViewScreenState();
}

class _StoreMapWebViewScreenState extends State<StoreMapWebViewScreen> {
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
          onWebResourceError: (_) {
            if (mounted) setState(() => _isLoading = false);
          },
        ),
      )
      ..loadRequest(Uri.parse(widget.redirectUrl));
  }

  NavigationDecision _onNavigationRequest(NavigationRequest request) {
    if (request.url.startsWith('hawkrseller://') ||
        request.url.startsWith('hawkrbuyer://')) {
      final uri = Uri.parse(request.url);
      if (mounted) {
        Navigator.of(context).pop(uri.queryParameters);
      }
      return NavigationDecision.prevent;
    }
    // 阻擋非 http(s) scheme（如其他自訂 deeplink / intent://），避免被導向外部 App。
    final uri = Uri.tryParse(request.url);
    if (uri != null && uri.scheme != 'http' && uri.scheme != 'https') {
      return NavigationDecision.prevent;
    }
    return NavigationDecision.navigate;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: HkGlassAppBar(
        title: const Text('選擇 7-11 門市'),
        onBackPressed: () => Navigator.of(context).pop(),
      ),
      body: Stack(
        children: [
          WebViewWidget(controller: _controller),
          if (_isLoading) const TruckLoadingOverlay(message: '載入中...'),
        ],
      ),
    );
  }
}
