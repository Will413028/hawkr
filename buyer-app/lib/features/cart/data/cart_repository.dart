import 'dart:async';

import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart';
import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../address/data/address_model.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'cart_data.dart';
import 'cart_error_message.dart';
import 'cart_model.dart';

part 'cart_repository.g.dart';

@Riverpod(keepAlive: true)
CartRepository cartRepository(Ref ref) {
  final repository = CartRepository(ref.watch(dioProvider));
  ref.onDispose(repository.dispose);
  return repository;
}

/// The cart data layer and single source of successfully decoded cart data.
///
/// Every cart mutation refreshes and publishes the same [CartData] snapshot.
/// Screens and badge providers therefore never need their own JSON mapping or
/// independent cart copies.
class CartRepository {
  CartRepository(this._dio);

  final Dio _dio;
  final StreamController<CartData> _snapshots =
      StreamController<CartData>.broadcast(sync: true);

  CartData? _latest;

  Stream<CartData> get snapshots => _snapshots.stream;
  CartData? get latest => _latest;

  void dispose() {
    _snapshots.close();
  }

  void _publish(CartData value) {
    _latest = value;
    if (!_snapshots.isClosed) _snapshots.add(value);
  }

  /// GET /public/carts and decode the entire cart contract once.
  Future<CartData> getCartData({String? guestId}) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/public/carts',
        queryParameters: guestId != null ? {'guest_id': guestId} : null,
      );
      final body = response.data;
      if (body == null) throw const ServerException('購物車資料格式錯誤');
      _requireSuccess(body, '讀取購物車失敗');

      final data = _asStringMap(body['data']) ?? const <String, dynamic>{};
      final groupsJson = data['groups'] as List? ?? const [];
      final groups = <CartMerchantGroup>[];
      final allItems = <CartItem>[];
      final stockBySku = <int, int>{};
      final deliveryTempsBySku = <int, List<String>>{};
      final productIdsBySku = <int, String>{};
      final lockedSkuIds = <int>{};

      for (final rawGroup in groupsJson) {
        final group = _asStringMap(rawGroup);
        if (group == null) continue;
        final storeId = group['storeId']?.toString() ?? '';
        final sellerName = group['sellerName']?.toString() ?? '';
        final groupItems = <CartItem>[];

        for (final rawItem in group['items'] as List? ?? const []) {
          final item = _asStringMap(rawItem);
          if (item == null) continue;
          final skuId = _asInt(item['skuId']);
          if (skuId == null || skuId <= 0) continue;

          final status =
              item['status']?.toString() ?? CartItemStatus.normal.value;
          if (status == CartItemStatus.locked.value) lockedSkuIds.add(skuId);

          final explicitPurchasable = item['purchasable'];
          final purchasable = explicitPurchasable is bool
              ? explicitPurchasable
              : status != 'SOLD_OUT' && status != 'OFF_SHELF';

          final stock = _asInt(item['stockQty'] ?? item['stock']);
          if (stock != null) stockBySku[skuId] = stock;

          final deliveryTemps = _stringList(item['deliveryTemps']);
          if (deliveryTemps.isNotEmpty) {
            deliveryTempsBySku[skuId] = deliveryTemps;
          }

          final productId = item['productId']?.toString().trim() ?? '';
          if (productId.isNotEmpty) productIdsBySku[skuId] = productId;

          final cartItem = CartItem(
            id: skuId,
            skuId: skuId,
            count: _asInt(item['quantity']) ?? 0,
            price: _moneyAsInt(item['price']),
            listPrice: item['listPrice'] == null
                ? null
                : _moneyAsInt(item['listPrice']),
            title: item['name']?.toString(),
            // gateway `CartItemResponse`：imageKey ＋ imageUrl 成對（49 §6.1）。
            image: item['imageUrl']?.toString() ?? item['imageKey']?.toString(),
            sellerId: int.tryParse(storeId),
            merchantName: sellerName,
            specName: item['specName']?.toString(),
            stockQty: item['stockQty']?.toString(),
            purchasable: purchasable,
            unavailableReason: item['unavailableReason']?.toString(),
          );
          groupItems.add(cartItem);
          allItems.add(cartItem);
        }

        if (groupItems.isNotEmpty) {
          groups.add(
            CartMerchantGroup(
              storeId: storeId,
              sellerName: sellerName,
              items: groupItems,
              // gateway `CartGroupResponse`：logoKey ＋ logoUrl 成對（49 §6.1）。
              logoUrl: (group['logoKey'] ?? group['logoUrl'])?.toString(),
              sellerCity: group['sellerCity']?.toString() ?? '',
              sellerDistrict: group['sellerDistrict']?.toString() ?? '',
            ),
          );
        }
      }

      final snapshot = CartData(
        groups: groups,
        items: allItems,
        stockBySku: stockBySku,
        deliveryTempsBySku: deliveryTempsBySku,
        productIdsBySku: productIdsBySku,
        lockedSkuIds: lockedSkuIds,
        estimatedTotalAmount: _moneyAsInt(data['estimatedTotalAmount']),
      );
      _publish(snapshot);
      return snapshot;
    } on DioException catch (error) {
      throw _transportException(error);
    }
  }

  /// Compatibility API for older call sites.
  Future<({CartSummary summary, Set<int> lockedSkuIds})> getCartList({
    String? guestId,
  }) async {
    final data = await getCartData(guestId: guestId);
    return (summary: data.summary, lockedSkuIds: data.lockedSkuIds);
  }

  /// POST /public/carts/items.
  Future<CartData> addToCart(int skuId, int count, {String? guestId}) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/public/carts/items',
        data: {
          'skuId': skuId.toString(),
          'quantity': count.toString(),
          if (guestId != null) 'guestId': guestId,
        },
      );
      final body = response.data;
      if (body == null) throw const ServerException('加入購物車失敗');
      _requireSuccess(body, '加入購物車失敗');
      return getCartData(guestId: guestId);
    } on DioException catch (error) {
      final selfPurchaseMessage = _selfPurchaseMessageFromDio(error);
      if (selfPurchaseMessage != null) {
        throw ServerException(selfPurchaseMessage);
      }
      final data = error.response?.data;
      if (data is Map) {
        throw ServerException(_messageFromBody(data, '加入購物車失敗'));
      }
      throw _transportException(error);
    }
  }

  /// PUT /public/carts/items/{sku_id}/quantity.
  Future<CartData> updateItem(int skuId, int count, {String? guestId}) async {
    try {
      final response = await _dio.put<Map<String, dynamic>>(
        '/public/carts/items/$skuId/quantity',
        data: {
          'quantity': count.toString(),
          if (guestId != null) 'guestId': guestId,
        },
      );
      final body = response.data;
      if (body == null) throw const ServerException('更新數量失敗');
      _requireSuccess(body, '更新數量失敗');
      return getCartData(guestId: guestId);
    } on DioException catch (error) {
      throw _transportException(error);
    }
  }

  /// DELETE /public/carts/items/{sku_id}.
  ///
  /// A partial delete still refreshes and publishes the server truth before
  /// surfacing the failure.
  Future<CartData> removeItems(List<int> skuIds, {String? guestId}) async {
    Object? failure;
    StackTrace? failureStack;
    for (final skuId in skuIds) {
      try {
        final response = await _dio.delete<Map<String, dynamic>>(
          '/public/carts/items/$skuId',
          queryParameters: guestId != null ? {'guest_id': guestId} : null,
        );
        final body = response.data;
        if (body == null) throw const ServerException('刪除商品失敗');
        _requireSuccess(body, '刪除商品失敗');
      } on Object catch (error, stack) {
        failure = error is DioException ? _transportException(error) : error;
        failureStack = stack;
        break;
      }
    }

    final refreshed = await getCartData(guestId: guestId);
    if (failure != null) {
      Error.throwWithStackTrace(failure, failureStack ?? StackTrace.current);
    }
    return refreshed;
  }

  /// The badge uses the same decoded snapshot as the cart screen.
  Future<int> countItems({String? guestId}) async {
    final data = await getCartData(guestId: guestId);
    return data.itemCount;
  }

  /// POST /app/carts/merge.
  Future<void> mergeCarts(String guestId) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/app/carts/merge',
        data: {'guestId': guestId},
      );
      final body = response.data;
      if (body == null) throw const ServerException('合併購物車失敗');
      _requireSuccess(body, '合併購物車失敗');
    } on DioException catch (error) {
      throw _transportException(error);
    }
    try {
      await getCartData();
    } catch (error, stack) {
      HKLog.logW('[CartRepository] merged cart refresh failed', error, stack);
    }
  }

  Future<List<Map<String, dynamic>>> getStorePickups() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/app/store_pickups',
      );
      final body = response.data;
      if (body == null) throw const ServerException('讀取取貨門市失敗');
      _requireSuccess(body, '讀取取貨門市失敗');
      return _mapList(body['data']);
    } on DioException catch (error) {
      throw _transportException(error);
    }
  }

  Future<CartValidation> validateCheckout({
    required List<CartItem> items,
    required Map<int, String> productIdsBySku,
  }) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/app/checkout/validate',
        data: {
          'items': items
              .map(
                (item) => {
                  'productId': productIdsBySku[item.skuId] ?? '0',
                  'quantity': item.count.toString(),
                  'skuId': item.skuId.toString(),
                },
              )
              .toList(),
        },
      );
      final body = response.data;
      if (body == null) throw const ServerException('驗證商品失敗');
      if (body['code'] != 0) {
        throw CartBusinessException(
          body['code']?.toString() ?? '',
          _messageFromBody(body, '商品驗證失敗，請重新確認'),
        );
      }
      final data = _asStringMap(body['data']) ?? const <String, dynamic>{};
      return CartValidation(
        valid: data['valid'] is bool ? data['valid'] as bool : true,
        result: data,
        shippingOptions: _mapList(data['availableShippingOptions']),
        errors: _mapList(data['errors']),
      );
    } on DioException catch (error) {
      throw _transportException(error);
    }
  }

  Future<double> getShippingFee({
    required List<CartItem> items,
    required Map<int, String> productIdsBySku,
    required Map<String, dynamic> shippingOption,
    required Address address,
  }) async {
    final zipCode = postalCodeByTownCode(address.townCode);
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/app/checkout/shipping_fee',
        data: {
          'items': items
              .map(
                (item) => {
                  'skuId': item.skuId.toString(),
                  'quantity': item.count.toString(),
                  'productId': productIdsBySku[item.skuId] ?? '0',
                },
              )
              .toList(),
          'shippingMethod': shippingOption['shippingMethod'] ?? '',
          'deliveryTemp': shippingOption['deliveryTemp'] ?? '',
          'receiverZipCode': zipCode,
        },
      );
      final body = response.data;
      if (body == null) throw const ServerException('計算運費失敗');
      _requireSuccess(body, '計算運費失敗');
      final data = _asStringMap(body['data']) ?? const <String, dynamic>{};
      return double.tryParse(data['shippingFee']?.toString() ?? '') ?? 0;
    } on DioException catch (error) {
      throw _transportException(error);
    }
  }

  Future<List<Map<String, dynamic>>> getAvailableCoupons({
    required String storeId,
    required int orderAmount,
  }) async {
    // store_id 是後端必填參數，沒有選定商家時直接回空，不送註定 400 的請求。
    if (storeId.isEmpty) return const [];
    try {
      final params = <String, dynamic>{
        'page': '1',
        'page_size': '20',
        'order_amount': orderAmount.toString(),
        'store_id': storeId,
      };
      final response = await _dio.get<dynamic>(
        '/app/coupons/available',
        queryParameters: params,
      );
      final body = response.data;
      if (body is! Map) throw const ServerException('讀取優惠券失敗');
      _requireSuccess(body, '讀取優惠券失敗');
      final data = body['data'];
      final records = data is Map
          ? data['records'] ?? data['rows'] ?? data['list']
          : data;
      if (records is! List) return const [];

      return List<Map<String, dynamic>>.unmodifiable(
        records.map((record) {
          final map = _asStringMap(record) ?? const <String, dynamic>{};
          final template = _asStringMap(map['template']);
          if (template == null) return Map<String, dynamic>.of(map);
          return <String, dynamic>{
            ...template,
            if (map['estimatedDiscount'] != null)
              'estimatedDiscount': map['estimatedDiscount'].toString(),
          };
        }),
      );
    } on DioException catch (error) {
      throw _transportException(error);
    }
  }

  static Map<String, dynamic>? _asStringMap(Object? value) {
    if (value is! Map) return null;
    final result = <String, dynamic>{};
    for (final entry in value.entries) {
      if (entry.key is String) result[entry.key as String] = entry.value;
    }
    return result;
  }

  static List<Map<String, dynamic>> _mapList(Object? value) {
    if (value is! List) return const [];
    return List<Map<String, dynamic>>.unmodifiable(
      value.map(_asStringMap).whereType<Map<String, dynamic>>(),
    );
  }

  static List<String> _stringList(Object? value) {
    if (value == null) return const [];
    if (value is List) {
      return value.map((item) => item.toString()).toList(growable: false);
    }
    return [value.toString()];
  }

  static int? _asInt(Object? value) => int.tryParse(value?.toString() ?? '');

  static int _moneyAsInt(Object? value) =>
      int.tryParse(
        (value?.toString() ?? '0').replaceAll(RegExp(r'[^0-9-]'), ''),
      ) ??
      0;

  void _requireSuccess(Map<dynamic, dynamic> body, String fallback) {
    if (body['code'] == 0) return;
    throw ServerException(_messageFromBody(body, fallback));
  }

  AppException _transportException(DioException error) {
    if (error.response?.statusCode == 401) {
      return const UnauthorizedException();
    }
    final data = error.response?.data;
    if (data is Map) {
      return ServerException(_messageFromBody(data, '請求失敗，請稍後再試'));
    }
    return const NetworkException();
  }

  String? _selfPurchaseMessageFromDio(DioException error) {
    final data = error.response?.data;
    if (data is! Map) return null;
    final code = data['code']?.toString();
    if (!_isSelfPurchaseCode(code)) return null;
    return _messageFromBody(data, cannotAddOwnProductToCartMessage);
  }

  bool _isSelfPurchaseCode(String? code) => code == '6006' || code == '4015';

  String _messageFromBody(dynamic body, String fallback) {
    if (body is! Map) return fallback;
    final code = body['code']?.toString();
    if (_isSelfPurchaseCode(code)) return cannotAddOwnProductToCartMessage;
    return (body['messageZh'] ?? body['message'] ?? fallback).toString();
  }
}
