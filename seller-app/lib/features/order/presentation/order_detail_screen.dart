import 'package:flutter/material.dart';

import '../data/order_model.dart';
import 'shared_order_detail_screen.dart';

class OrderDetailScreen extends StatelessWidget {
  const OrderDetailScreen({super.key, required this.order});

  final OrderSummary order;

  @override
  Widget build(BuildContext context) {
    final orderId = order.orderSn.isNotEmpty ? order.orderSn : order.id;

    return SharedOrderDetailScreen(
      orderId: orderId,
      initialSummary: order,
      fallbackDateText: order.dateTime,
      fallbackItemsText: order.productName,
      fallbackAmount: _fallbackAmount(order),
    );
  }
}

int _fallbackAmount(OrderSummary order) {
  final total = order.totalAmount.round();
  if (total > 0) return total;
  return _parseAmount(order.price);
}

int _parseAmount(String value) {
  final normalized = value.replaceAll(RegExp(r'[^0-9.]'), '');
  if (normalized.isEmpty) return 0;
  return (double.tryParse(normalized) ?? 0).round();
}
