import 'package:hawkr_seller/core/theme/app_colors.dart';

import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_icon.dart';

/// 託運單 / 寄件單 PDF 預覽頁。
///
/// iOS 的 WKWebView 原生支援直接渲染 PDF，直接載 [pdfUrl]。
///
/// 右上角附「外部開啟」按鈕作為 fallback，
/// 讓用戶可直接用系統瀏覽器/檔案 App 開啟。
class PdfPreviewScreen extends StatefulWidget {
  const PdfPreviewScreen({
    super.key,
    required this.pdfUrl,
    this.title = '託運單預覽',
  });

  final String pdfUrl;
  final String title;

  @override
  State<PdfPreviewScreen> createState() => _PdfPreviewScreenState();
}

class _PdfPreviewScreenState extends State<PdfPreviewScreen> {
  late final WebViewController _controller;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setBackgroundColor(AppColors.bgCard)
      ..setNavigationDelegate(
        NavigationDelegate(
          onNavigationRequest: (request) {
            // 阻擋非 http(s) scheme（如自訂 deeplink / intent://），避免被導向外部 App。
            final uri = Uri.tryParse(request.url);
            if (uri != null && uri.scheme != 'http' && uri.scheme != 'https') {
              return NavigationDecision.prevent;
            }
            return NavigationDecision.navigate;
          },
          onPageFinished: (_) {
            if (mounted) setState(() => _loading = false);
          },
          onWebResourceError: (error) {
            debugPrint('[PdfPreview] WebView error: ${error.description}');
          },
        ),
      )
      ..loadRequest(Uri.parse(widget.pdfUrl));
  }

  Future<void> _openInBrowser() async {
    final uri = Uri.parse(widget.pdfUrl);
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!ok && mounted) {
      FloatingToast.error(context, '無法開啟外部瀏覽器');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: HkGlassAppBar(
        title: Text(widget.title),
        actions: [
          IconButton(
            tooltip: '在外部瀏覽器開啟',
            icon: const HkIcon(HkIconName.externalLink),
            onPressed: _openInBrowser,
          ),
        ],
      ),
      body: Stack(
        children: [
          WebViewWidget(controller: _controller),
          if (_loading)
            const Center(
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: AppColors.primary,
              ),
            ),
        ],
      ),
    );
  }
}
