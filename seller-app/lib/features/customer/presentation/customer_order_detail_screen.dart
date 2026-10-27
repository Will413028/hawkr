import 'package:flutter/material.dart';

import '../data/customer_model.dart';
import '../../order/presentation/shared_order_detail_screen.dart';

/// 訂單細節頁面 — Figma: 6444:62660
class CustomerOrderDetailScreen extends StatelessWidget {
  const CustomerOrderDetailScreen({
    super.key,
    required this.order,
    required this.customerName,
  });

  final OrderRecord order;

  // Kept for route compatibility with the customer flow.
  final String customerName;

  @override
  Widget build(BuildContext context) {
    return SharedOrderDetailScreen(
      orderId: order.orderId,
      fallbackDateText: order.date,
      fallbackItemsText: order.items,
      fallbackAmount: order.amount,
    );
  }
}
