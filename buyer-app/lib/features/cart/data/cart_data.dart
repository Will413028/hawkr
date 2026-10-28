import 'dart:collection';

import 'package:flutter/foundation.dart';

import 'cart_model.dart';

/// Repository-owned, immutable representation of one seller group in the cart.
@immutable
class CartMerchantGroup {
  CartMerchantGroup({
    required this.storeId,
    required this.sellerName,
    required List<CartItem> items,
    this.logoUrl,
    this.sellerCity = '',
    this.sellerDistrict = '',
  }) : items = List<CartItem>.unmodifiable(items);

  final String storeId;
  final String sellerName;
  final List<CartItem> items;
  final String? logoUrl;
  final String sellerCity;
  final String sellerDistrict;

  String get locationText => '$sellerCity$sellerDistrict'.trim();

  CartMerchantGroup copyWith({List<CartItem>? items}) => CartMerchantGroup(
    storeId: storeId,
    sellerName: sellerName,
    items: items ?? this.items,
    logoUrl: logoUrl,
    sellerCity: sellerCity,
    sellerDistrict: sellerDistrict,
  );
}

/// The successfully decoded cart snapshot.
///
/// Both the cart screen and the global cart badge derive from this object.
@immutable
class CartData {
  CartData({
    required List<CartMerchantGroup> groups,
    required List<CartItem> items,
    required Map<int, int> stockBySku,
    required Map<int, String> productIdsBySku,
    required Set<int> lockedSkuIds,
    required this.estimatedTotalAmount,
  }) : groups = List<CartMerchantGroup>.unmodifiable(groups),
       items = List<CartItem>.unmodifiable(items),
       stockBySku = UnmodifiableMapView<int, int>(Map.of(stockBySku)),
       productIdsBySku = UnmodifiableMapView<int, String>(
         Map.of(productIdsBySku),
       ),
       lockedSkuIds = Set<int>.unmodifiable(lockedSkuIds);

  factory CartData.empty() => CartData(
    groups: const [],
    items: const [],
    stockBySku: const {},
    productIdsBySku: const {},
    lockedSkuIds: const {},
    estimatedTotalAmount: 0,
  );

  final List<CartMerchantGroup> groups;
  final List<CartItem> items;
  final Map<int, int> stockBySku;
  final Map<int, String> productIdsBySku;
  final Set<int> lockedSkuIds;
  final int estimatedTotalAmount;

  bool get isEmpty => items.isEmpty;

  int get itemCount => items.fold(0, (sum, item) => sum + item.count);

  CartSummary get summary =>
      CartSummary(items: items, totalAmount: estimatedTotalAmount);

  CartData replaceQuantity(int skuId, int quantity) {
    CartItem update(CartItem item) =>
        item.skuId == skuId ? item.copyWith(count: quantity) : item;

    return CartData(
      groups: groups
          .map(
            (group) => group.copyWith(items: group.items.map(update).toList()),
          )
          .toList(),
      items: items.map(update).toList(),
      stockBySku: stockBySku,
      productIdsBySku: productIdsBySku,
      lockedSkuIds: lockedSkuIds,
      estimatedTotalAmount: estimatedTotalAmount,
    );
  }
}

@immutable
class CartValidation {
  CartValidation({
    required this.valid,
    required Map<String, dynamic> result,
    required List<Map<String, dynamic>> shippingOptions,
    required List<Map<String, dynamic>> errors,
  }) : result = UnmodifiableMapView<String, dynamic>(Map.of(result)),
       shippingOptions = List<Map<String, dynamic>>.unmodifiable(
         shippingOptions.map(
           (option) => UnmodifiableMapView<String, dynamic>(Map.of(option)),
         ),
       ),
       errors = List<Map<String, dynamic>>.unmodifiable(
         errors.map(
           (error) => UnmodifiableMapView<String, dynamic>(Map.of(error)),
         ),
       );

  final bool valid;
  final Map<String, dynamic> result;
  final List<Map<String, dynamic>> shippingOptions;
  final List<Map<String, dynamic>> errors;

  String? get firstErrorMessage {
    for (final error in errors) {
      final message = error['errorMessage']?.toString().trim() ?? '';
      if (message.isNotEmpty) return message;
    }
    return null;
  }

  bool hasErrorCode(String code) =>
      errors.any((error) => error['errorCode']?.toString() == code);
}

class CartBusinessException implements Exception {
  const CartBusinessException(this.code, this.message);

  final String code;
  final String message;

  @override
  String toString() => message;
}
