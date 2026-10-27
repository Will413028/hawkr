import 'dart:collection';

import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';
import 'package:hawkr_buyer/features/order/data/order_repository.dart';
import 'package:hawkr_buyer/features/order/presentation/order_list_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test(
    'OrderRepository sends pagination contract and normalizes seller logo',
    () async {
      RequestOptions? request;
      final dio = Dio(BaseOptions(baseUrl: 'https://orders.test'));
      addTearDown(dio.close);
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (options, handler) {
            request = options;
            handler.resolve(
              Response<Map<String, dynamic>>(
                requestOptions: options,
                statusCode: 200,
                data: {
                  'code': 0,
                  'data': {
                    'records': [
                      {
                        'orderNo': 'HK-1',
                        'status': '0',
                        // gateway `OrderSummaryResponse` 只回這一組成對欄位。
                        'logoKey': 'seller/logo.webp',
                        'logoUrl':
                            'https://media.test/i/original/seller/logo.webp',
                      },
                    ],
                    'total': '1',
                  },
                },
              ),
            );
          },
        ),
      );

      final result = await OrderRepository(
        dio,
      ).getOrders(status: '0', page: 2, pageSize: 10);

      expect(request?.method, 'GET');
      expect(request?.path, '/app/orders');
      expect(request?.queryParameters, {
        'page': '2',
        'page_size': '10',
        'status': '0',
      });
      expect(result.total, 1);
      expect(result.records.single.orderSn, 'HK-1');
      // 49 §6.1：裸 key 優先（顯示端帶 scene 走 buildImageUrl dual-mode）。
      expect(result.records.single.sellerAvatarUrl, 'seller/logo.webp');
    },
  );

  test('訂單列表的商家 logo 只認 logoKey/logoUrl，舊候選 key 一個都讀不到', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://orders.test'));
    addTearDown(dio.close);
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          handler.resolve(
            Response<Map<String, dynamic>>(
              requestOptions: options,
              statusCode: 200,
              data: {
                'code': 0,
                'data': {
                  'records': [
                    {
                      'orderNo': 'HK-2',
                      'status': '0',
                      // 以下八個是 08-05 契約收斂前被當成候選 key 讀的名字，
                      // wire 上一個都不存在；讀得到任何一個代表死候選又長回來。
                      'sellerAvatarKey': 'dead/1.webp',
                      'seller_avatar_key': 'dead/2.webp',
                      'seller_logo_key': 'dead/3.webp',
                      'seller_logo': 'dead/4.webp',
                      'sellerLogo': 'dead/5.webp',
                      'merchantAvatar': 'dead/6.webp',
                      'avatar': 'dead/7.webp',
                      'logo': 'dead/8.webp',
                    },
                  ],
                  'total': '1',
                },
              },
            ),
          );
        },
      ),
    );

    final result = await OrderRepository(dio).getOrders(page: 1, pageSize: 10);

    expect(result.records.single.sellerAvatarUrl, '');
  });

  test(
    'OrderListController paginates without replacing the first page',
    () async {
      final repository = _FakeOrderRepository([
        (records: [_order('HK-1')], total: 2),
        (records: [_order('HK-2')], total: 2),
      ]);
      final container = ProviderContainer(
        overrides: [
          sessionProvider.overrideWith(_ActiveSession.new),
          orderRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);

      await container.read(sessionProvider.future);
      final firstPage = await container.read(
        orderListControllerProvider.future,
      );
      expect(firstPage.map((order) => order.orderSn), ['HK-1']);
      expect(
        container.read(orderListControllerProvider.notifier).hasMore,
        isTrue,
      );

      await container.read(orderListControllerProvider.notifier).loadMore();

      final allOrders = container
          .read(orderListControllerProvider)
          .requireValue;
      expect(allOrders.map((order) => order.orderSn), ['HK-1', 'HK-2']);
      expect(repository.pages, [1, 2]);
      expect(repository.statuses, ['0', '0']);
    },
  );

  test('OrderListController guest build never calls member endpoint', () async {
    final repository = _FakeOrderRepository(const []);
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_GuestSession.new),
        orderRepositoryProvider.overrideWithValue(repository),
      ],
    );
    addTearDown(container.dispose);

    await container.read(sessionProvider.future);
    expect(await container.read(orderListControllerProvider.future), isEmpty);
    expect(repository.pages, isEmpty);
  });
}

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Buyer');
}

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

class _FakeOrderRepository extends OrderRepository {
  _FakeOrderRepository(Iterable<({List<Order> records, int total})> responses)
    : _responses = Queue.of(responses),
      super(Dio());

  final Queue<({List<Order> records, int total})> _responses;
  final List<int> pages = [];
  final List<String?> statuses = [];

  @override
  Future<({List<Order> records, int total})> getOrders({
    String? status,
    int page = 1,
    int pageSize = 10,
  }) async {
    pages.add(page);
    statuses.add(status);
    return _responses.removeFirst();
  }
}

Order _order(String orderSn) => Order(orderSn: orderSn, status: '0');
