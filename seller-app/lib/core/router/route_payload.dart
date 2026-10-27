import '../../features/order/data/order_model.dart';

/// Converts an untrusted route payload into a string-keyed map.
///
/// GoRouter's `extra` is process-local and may be absent after a deep link or
/// process restoration. Callers must therefore treat both the object and every
/// nested value as untrusted.
Map<String, dynamic> stringKeyedRouteMap(Object? value) {
  if (value is! Map) return const {};
  return <String, dynamic>{
    for (final entry in value.entries)
      if (entry.key is String) entry.key as String: entry.value,
  };
}

T? routeValue<T>(Map<String, dynamic> payload, String key) {
  final value = payload[key];
  return value is T ? value : null;
}

Map<String, dynamic>? routeNestedMap(Map<String, dynamic> payload, String key) {
  final value = payload[key];
  if (value is! Map) return null;
  return stringKeyedRouteMap(value);
}

List<T>? routeTypedList<T>(Map<String, dynamic> payload, String key) {
  final value = payload[key];
  if (value is! List || value.any((item) => item is! T)) return null;
  return value.cast<T>();
}

List<Map<String, dynamic>>? routeMapList(
  Map<String, dynamic> payload,
  String key,
) {
  final value = payload[key];
  if (value is! List) return null;

  final result = <Map<String, dynamic>>[];
  for (final item in value) {
    if (item is! Map) return null;
    result.add(stringKeyedRouteMap(item));
  }
  return result;
}

Map<int, String>? routeIntStringMap(Map<String, dynamic> payload, String key) {
  final value = payload[key];
  if (value is! Map) return null;

  final result = <int, String>{};
  for (final entry in value.entries) {
    if (entry.key is! int || entry.value is! String) return null;
    result[entry.key as int] = entry.value as String;
  }
  return result;
}

/// Adapts both the typed in-app payload and legacy map payload to the current
/// order details model. A query ID is enough to restore the route safely.
OrderSummary? orderSummaryFromRoutePayload(
  Object? extra, {
  String? fallbackOrderSn,
}) {
  if (extra is OrderSummary) return extra;
  if (extra is! Map) {
    final orderSn = fallbackOrderSn?.trim() ?? '';
    if (orderSn.isEmpty) return null;
    return OrderSummary(
      id: orderSn,
      orderSn: orderSn,
      title: '',
      subtitle: '',
      dateTime: '',
      productName: '',
      price: '',
    );
  }

  final map = stringKeyedRouteMap(extra);
  final orderSn =
      (map['orderSn'] ??
              map['orderNo'] ??
              map['order_no'] ??
              fallbackOrderSn ??
              '')
          .toString()
          .trim();
  if (orderSn.isEmpty) return null;

  return OrderSummary(
    id: orderSn,
    orderSn: orderSn,
    statusCode: int.tryParse((map['status'] ?? '').toString()),
    title: (map['statusSummary'] ?? map['status'] ?? '').toString(),
    subtitle: (map['status'] ?? '').toString(),
    dateTime: (map['createTime'] ?? '').toString(),
    productName: (map['productName'] ?? '').toString(),
    price: (map['totalActualAmount'] ?? map['totalAmount'] ?? '').toString(),
  );
}
