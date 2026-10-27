import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';

/// 7-ELEVEN 門市地圖 WebView
///
/// 流程：
/// 1. 載入 redirectUrl（PayUni 7-ELEVEN 電子地圖）
/// 2. 使用者選擇門市後，頁面觸發 deep link：
///    hawkrbuyer://store_map/result?storeId=916712&storeName=敦安門市&address=台北市...&insularArea=I
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
    // 攔截 deep link scheme（必須在 Uri.tryParse 之前，custom scheme 可能解析失敗）
    if (request.url.startsWith('hawkrbuyer://') ||
        request.url.startsWith('hawkrseller://')) {
      final uri = Uri.parse(request.url);
      if (mounted) {
        Navigator.of(context).pop(uri.queryParameters);
      }
      return NavigationDecision.prevent;
    }

    return NavigationDecision.navigate;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: buildCommonTitleAppBar(context, title: '新增 7-11 門市'),
      body: Stack(
        children: [
          WebViewWidget(controller: _controller),
          if (_isLoading) const TruckLoadingOverlay(message: '載入中...'),
        ],
      ),
    );
  }
}
