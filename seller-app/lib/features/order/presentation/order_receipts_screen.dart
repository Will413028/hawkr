import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_adaptive_page.dart';
import 'package:hawkr_seller/shared/widgets/hk_async_state_panel.dart';
import 'package:hawkr_seller/shared/widgets/hk_content_card.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

import '../data/order_model.dart';

/// 訂單文件中心。
///
/// 後端尚未回傳正式文件時顯示明確狀態，不渲染範例發票號碼或 QR Code。
class OrderReceiptsScreen extends StatelessWidget {
  const OrderReceiptsScreen({super.key, this.order});

  final OrderSummary? order;

  static Route<T> route<T extends Object?>({
    RouteSettings? settings,
    OrderSummary? order,
  }) {
    return MaterialPageRoute<T>(
      builder: (_) => OrderReceiptsScreen(order: order),
      settings: settings,
    );
  }

  @override
  Widget build(BuildContext context) {
    final currentOrder = order;
    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: const HkGlassAppBar(title: Text('訂單文件'), subtitle: '收據與可下載文件'),
      body: HkAdaptivePage(
        maxWidth: 720,
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (currentOrder != null) ...[
                _OrderDocumentSummary(order: currentOrder),
                const SizedBox(height: 16),
              ],
              HkAsyncStatePanel(
                kind: currentOrder == null
                    ? HkAsyncStateKind.error
                    : HkAsyncStateKind.empty,
                title: currentOrder == null ? '無法取得訂單資料' : '目前沒有可下載文件',
                description: currentOrder == null
                    ? '請回到訂單列表重新開啟這筆訂單。'
                    : '平台收據或物流文件產生後，會集中顯示在這裡。',
                actionLabel: currentOrder == null ? '返回' : null,
                onAction: currentOrder == null
                    ? () => Navigator.of(context).maybePop()
                    : null,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _OrderDocumentSummary extends StatelessWidget {
  const _OrderDocumentSummary({required this.order});

  final OrderSummary order;

  @override
  Widget build(BuildContext context) {
    final orderNo = order.orderSn.isNotEmpty
        ? order.orderSn
        : order.id.toString();
    final amount = order.price.trim().isEmpty
        ? '—'
        : order.price.startsWith('NT')
        ? order.price
        : 'NT\$${order.price}';

    return HkContentCard(
      elevated: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Row(
            children: [
              HkIcon(
                HkIconName.document,
                color: AppIconColors.active,
                size: 22,
              ),
              SizedBox(width: 8),
              Text(
                '文件所屬訂單',
                style: TextStyle(
                  color: AppColors.textEmphasis,
                  fontSize: 18,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          _SummaryLine(label: '訂單編號', value: orderNo),
          const SizedBox(height: 10),
          _SummaryLine(
            label: '成立時間',
            value: order.dateTime.trim().isEmpty ? '—' : order.dateTime,
          ),
          const SizedBox(height: 10),
          _SummaryLine(label: '訂單金額', value: amount),
        ],
      ),
    );
  }
}

class _SummaryLine extends StatelessWidget {
  const _SummaryLine({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 76,
          child: Text(
            label,
            style: const TextStyle(color: AppColors.textMuted, fontSize: 14),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            value,
            textAlign: TextAlign.right,
            style: const TextStyle(
              color: AppColors.textEmphasis,
              fontSize: 16,
              fontWeight: FontWeight.w500,
            ),
          ),
        ),
      ],
    );
  }
}
