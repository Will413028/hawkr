import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:hawkr_buyer/features/cart/data/cart_model.dart';
import 'package:hawkr_buyer/features/cart/data/cart_repository.dart';

class _RecordingAdapter implements HttpClientAdapter {
  final List<Map<String, dynamic>> bodies = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    bodies.add(Map<String, dynamic>.from(options.data as Map));
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {'shippingFee': '120'},
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test('shipping_fee sends the selected option deliveryTemp as is', () async {
    final adapter = _RecordingAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://cart.test'))
      ..httpClientAdapter = adapter;
    addTearDown(dio.close);

    final fee = await CartRepository(dio).getShippingFee(
      items: [CartItem(id: 1, skuId: 101, count: 1, price: 100)],
      productIdsBySku: const {101: '9001'},
      shippingOption: {
        'shippingMethod': ShippingMethod.taqbin.value,
        'deliveryTemp': ShippingTemperature.frozen.value,
      },
      address: const Address(
        id: 1,
        townCode: '63000050',
        etag: 'v1',
        receiverName: '測試收件人',
        receiverPhone: '0912345678',
      ),
    );

    expect(fee, 120);
    expect(
      adapter.bodies.single['deliveryTemp'],
      ShippingTemperature.frozen.value,
    );
  });
}
