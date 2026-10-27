import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/features/chat/data/chat_picker_model.dart';
import 'package:hawkr_seller/features/chat/data/chat_picker_remote_service.dart';
import 'package:hawkr_seller/features/chat/data/chat_picker_repository.dart';
import 'package:hawkr_seller/features/chat/presentation/chat_picker_controller.dart';
import 'package:hawkr_seller/features/product/data/product_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test(
    'remote service uses the injected central client and typed repository',
    () async {
      final adapter = _CatalogAdapter();
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = adapter;
      final container = ProviderContainer(
        overrides: [dioProvider.overrideWithValue(dio)],
      );
      addTearDown(container.dispose);

      final repository = RemoteChatPickerRepository(
        container.read(chatPickerRemoteServiceProvider),
        container.read(productRepositoryProvider),
      );
      final orders = await repository.loadOrders('42');
      final products = await repository.loadProducts();

      expect(orders.single.orderNo, 'HK-1');
      expect(products.single.pickerData['productName'], '青江菜');
      expect(adapter.requests[0].path, '/api/v1/app/orders');
      expect(adapter.requests[0].queryParameters['buyer_id'], '42');
      expect(adapter.requests[1].path, '/api/v1/app/products');
      expect(adapter.requests[1].queryParameters['status'], '4004');
    },
  );

  test(
    'controller exposes immutable loading and ready command states',
    () async {
      final repository = _CompletingChatPickerRepository();
      final container = ProviderContainer(
        overrides: [chatPickerRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      final controller = container.read(chatPickerControllerProvider.notifier);

      final operation = controller.loadOrders('42');
      expect(
        container.read(chatPickerControllerProvider),
        isA<ChatPickerLoading>().having(
          (state) => state.kind,
          'kind',
          ChatPickerKind.order,
        ),
      );

      repository.orders.complete([
        ChatOrderPickerOption({'orderNo': 'HK-42'}),
      ]);
      final options = await operation;

      expect(options.single.orderNo, 'HK-42');
      expect(
        container.read(chatPickerControllerProvider),
        isA<ChatPickerReady>()
            .having((state) => state.kind, 'kind', ChatPickerKind.order)
            .having((state) => state.itemCount, 'itemCount', 1),
      );
    },
  );
}

class _CompletingChatPickerRepository implements ChatPickerRepository {
  final orders = Completer<List<ChatOrderPickerOption>>();

  @override
  Future<List<ChatOrderPickerOption>> loadOrders(String buyerAccountId) {
    return orders.future;
  }

  @override
  Future<List<ChatProductPickerOption>> loadProducts() {
    return Future.value(const []);
  }
}

class _CatalogAdapter implements HttpClientAdapter {
  final requests = <RequestOptions>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    final data = options.path.endsWith('/orders')
        ? {
            'code': 0,
            'data': {
              'records': [
                {'orderNo': 'HK-1'},
              ],
            },
          }
        : {
            'code': 0,
            'data': [
              {'id': 9, 'productName': '青江菜'},
            ],
          };
    return ResponseBody.fromString(
      jsonEncode(data),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
