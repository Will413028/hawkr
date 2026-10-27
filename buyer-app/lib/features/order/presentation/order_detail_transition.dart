/// Stable shared-element tag used by an order card and its detail screen.
///
/// Notification/deep-link entries intentionally omit the tag and open the same
/// detail route without a Hero flight.
Object? orderDetailHeroTag(String orderSn) {
  final normalizedOrderSn = orderSn.trim();
  if (normalizedOrderSn.isEmpty) return null;
  return 'order-detail-card:$normalizedOrderSn';
}
