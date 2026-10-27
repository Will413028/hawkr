import 'analytics_constants.dart';
import 'analytics_service.dart';

/// Typed merchant operational events. One method per taxonomy entry.
class AnalyticsEvents {
  AnalyticsEvents(this._svc);
  final AnalyticsService _svc;

  Future<void> login(String method) async {
    await _svc.logLogin(method);
    await _svc.setUserProperty(kAnalyticsLoginMethodProperty, method);
  }

  Future<void> shipOrder(String orderNo, {required String shippingMethod}) =>
      _svc.logEvent('ship_order', {
        'order_no': orderNo,
        'shipping_method': shippingMethod,
      });

  Future<void> requestLogistics(String orderNo) =>
      _svc.logEvent('request_logistics', {'order_no': orderNo});

  Future<void> cancelOrder(String orderNo, {String? cancelReason}) =>
      _svc.logEvent('cancel_order', {
        'order_no': orderNo,
        'cancel_reason': cancelReason,
      });

  Future<void> printCvsLabel(String orderNo) =>
      _svc.logEvent('print_cvs_label', {'order_no': orderNo});

  Future<void> createProduct(String productId, {String? categoryId}) =>
      _svc.logEvent('create_product', {
        'product_id': productId,
        'category_id': categoryId,
      });

  Future<void> updateProduct(String productId) =>
      _svc.logEvent('update_product', {'product_id': productId});

  Future<void> setProductStatus(String productId, String status) =>
      _svc.logEvent('set_product_status', {
        'product_id': productId,
        'status': status,
      });

  Future<void> sendMessage() => _svc.logEvent('send_message', const {});

  Future<void> onboardingComplete() =>
      _svc.logEvent('onboarding_complete', const {});
}
