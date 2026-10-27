import 'package:firebase_analytics/firebase_analytics.dart';

import '../../features/order/data/order_model.dart';
import 'analytics_constants.dart';
import 'analytics_service.dart';
import 'ga4_item.dart';

/// Typed GA4 ecommerce events. One method per web `events.ts` track function.
/// Each builds the items array + scalar params, then dispatches through the
/// gated [AnalyticsService] using FirebaseAnalytics' typed ecommerce methods.
class AnalyticsEvents {
  AnalyticsEvents(this._svc);
  final AnalyticsService _svc;

  List<AnalyticsEventItem> _items(List<GA4Item> items) =>
      items.map((e) => e.toAnalyticsEventItem()).toList();

  Future<void> trackViewItemList(
    List<GA4Item> items, {
    required String listId,
    required String listName,
  }) => _svc.logEcommerce(
    (fa) => fa.logViewItemList(
      itemListId: listId,
      itemListName: listName,
      items: _items(items),
    ),
  );

  Future<void> trackSelectItem(
    GA4Item item, {
    required String listId,
    required String listName,
  }) => _svc.logEcommerce(
    (fa) => fa.logSelectItem(
      itemListId: listId,
      itemListName: listName,
      items: _items([item]),
    ),
  );

  Future<void> trackViewItem(GA4Item item) => _svc.logEcommerce(
    (fa) => fa.logViewItem(
      currency: kAnalyticsCurrency,
      value: (item.price ?? 0).toDouble(),
      items: _items([item]),
    ),
  );

  Future<void> trackAddToCart(GA4Item item) => _svc.logEcommerce(
    (fa) => fa.logAddToCart(
      currency: kAnalyticsCurrency,
      value: ((item.price ?? 0) * (item.quantity ?? 1)).toDouble(),
      items: _items([item]),
    ),
  );

  Future<void> trackRemoveFromCart(GA4Item item) => _svc.logEcommerce(
    (fa) => fa.logRemoveFromCart(
      currency: kAnalyticsCurrency,
      value: ((item.price ?? 0) * (item.quantity ?? 1)).toDouble(),
      items: _items([item]),
    ),
  );

  Future<void> trackViewCart(List<GA4Item> items) {
    if (items.isEmpty) return Future.value();
    return _svc.logEcommerce(
      (fa) => fa.logViewCart(
        currency: kAnalyticsCurrency,
        value: sumValue(items).toDouble(),
        items: _items(items),
      ),
    );
  }

  Future<void> trackAddToWishlist(GA4Item item) => _svc.logEcommerce(
    (fa) => fa.logAddToWishlist(
      currency: kAnalyticsCurrency,
      value: (item.price ?? 0).toDouble(),
      items: _items([item]),
    ),
  );

  Future<void> trackBeginCheckout(List<GA4Item> items) => _svc.logEcommerce(
    (fa) => fa.logBeginCheckout(
      currency: kAnalyticsCurrency,
      value: sumValue(items).toDouble(),
      items: _items(items),
    ),
  );

  Future<void> trackAddShippingInfo(
    List<GA4Item> items, {
    required String shippingTier,
  }) => _svc.logEcommerce(
    (fa) => fa.logAddShippingInfo(
      currency: kAnalyticsCurrency,
      value: sumValue(items).toDouble(),
      shippingTier: shippingTier,
      items: _items(items),
    ),
  );

  Future<void> trackAddPaymentInfo(
    List<GA4Item> items, {
    required String paymentType,
  }) => _svc.logEcommerce(
    (fa) => fa.logAddPaymentInfo(
      currency: kAnalyticsCurrency,
      value: sumValue(items).toDouble(),
      paymentType: paymentType,
      items: _items(items),
    ),
  );

  Future<void> trackPurchase(Order order) {
    final items = order.items.map(orderItemToGA4Item).toList();
    return _svc.logEcommerce(
      (fa) => fa.logPurchase(
        currency: kAnalyticsCurrency,
        transactionId: order.orderSn,
        value: _orderMoney(order.totalActualAmount).toDouble(),
        shipping: _orderMoney(order.shippingFee).toDouble(),
        items: items.map((e) => e.toAnalyticsEventItem()).toList(),
      ),
    );
  }

  Future<void> trackLogin(String method) async {
    await _svc.logEcommerce((fa) => fa.logLogin(loginMethod: method));
    await _svc.setUserProperty(kAnalyticsLoginMethodProperty, method);
  }
}

/// Parses an Order money string ("350", "60.0") to a num, defaulting to 0.
/// (Order amounts arrive as strings; mirrors web `parsePriceValue`.)
num _orderMoney(String raw) => num.tryParse(raw.trim()) ?? 0;
