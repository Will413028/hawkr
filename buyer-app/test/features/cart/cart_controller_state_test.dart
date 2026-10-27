import 'dart:collection';
import 'dart:io';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:hawkr_buyer/features/address/data/address_repository.dart';
import 'package:hawkr_buyer/features/cart/data/cart_data.dart';
import 'package:hawkr_buyer/features/cart/data/cart_model.dart';
import 'package:hawkr_buyer/features/cart/data/cart_repository.dart';
import 'package:hawkr_buyer/features/cart/presentation/cart_controller.dart';
import 'package:hawkr_buyer/features/cart/presentation/cart_state.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../support/default_error_text.dart';

void main() {
  group('CartController mutually exclusive content states', () {
    test('successful empty response is empty without an error', () async {
      final repository = _FakeCartRepository(loads: [CartData.empty()]);
      final container = _containerFor(repository);
      addTearDown(container.dispose);

      final state = await container.read(cartControllerProvider.future);

      expect(state.status, CartContentStatus.empty);
      expect(state.data, isNotNull);
      expect(state.problem, isNull);
    });

    test('initial network failure is failure, never empty', () async {
      final repository = _FakeCartRepository(
        loads: [const NetworkException('offline')],
      );
      final container = _containerFor(repository);
      addTearDown(container.dispose);

      final state = await container.read(cartControllerProvider.future);

      expect(state.status, CartContentStatus.failure);
      expect(state.data, isNull);
      expect(state.problem?.message, 'offline');
    });

    test(
      'refresh failure preserves the last successful non-empty data',
      () async {
        final repository = _FakeCartRepository(
          loads: [_cart(quantity: 2), const NetworkException('refresh failed')],
        );
        final container = _containerFor(repository);
        addTearDown(container.dispose);
        await container.read(cartControllerProvider.future);

        await container.read(cartControllerProvider.notifier).refresh();
        final state = container.read(cartControllerProvider).requireValue;

        expect(state.status, CartContentStatus.ready);
        expect(state.data?.items.single.count, 2);
        expect(state.problem?.kind, CartProblemKind.staleData);
        expect(state.problem?.message, 'refresh failed');
      },
    );

    test(
      'business validation error keeps cart data separate from empty',
      () async {
        final repository = _FakeCartRepository(
          loads: [_cart(quantity: 2)],
          validation: CartValidation(
            valid: false,
            result: const {'valid': false},
            shippingOptions: const [],
            errors: const [
              {'errorCode': 'BUSINESS_RULE', 'errorMessage': '此商品暫時無法結帳'},
            ],
          ),
        );
        final container = _containerFor(repository);
        addTearDown(container.dispose);
        await container.read(cartControllerProvider.future);

        await container.read(cartControllerProvider.notifier).validateNow();
        final state = container.read(cartControllerProvider).requireValue;

        expect(state.status, CartContentStatus.ready);
        expect(state.data?.items, isNotEmpty);
        expect(state.problem?.kind, CartProblemKind.business);
        expect(state.problem?.message, '此商品暫時無法結帳');
      },
    );
  });

  group('CartController optimistic quantity reconciliation', () {
    test('rapid changes send only the newest quantity', () async {
      final repository = _FakeCartRepository(loads: [_cart(quantity: 1)]);
      final container = _containerFor(repository);
      addTearDown(container.dispose);
      await container.read(cartControllerProvider.future);
      final controller = container.read(cartControllerProvider.notifier);

      controller
        ..updateQuantity(101, 2)
        ..updateQuantity(101, 3)
        ..updateQuantity(101, 4);
      expect(
        container
            .read(cartControllerProvider)
            .requireValue
            .data
            ?.items
            .single
            .count,
        4,
      );

      await controller.flushPendingQuantityUpdates();
      final state = container.read(cartControllerProvider).requireValue;

      expect(repository.quantityUpdates, [(skuId: 101, quantity: 4)]);
      expect(state.data?.items.single.count, 4);
      expect(state.updatingSkuIds, isEmpty);
      expect(await container.read(cartCountProvider.future), 4);
    });

    test(
      'failed optimistic mutation rolls back without emptying cart',
      () async {
        final repository = _FakeCartRepository(
          loads: [_cart(quantity: 2)],
          updateFailure: const NetworkException('quantity unavailable'),
        );
        final container = _containerFor(repository);
        addTearDown(container.dispose);
        await container.read(cartControllerProvider.future);
        final controller = container.read(cartControllerProvider.notifier);

        controller.updateQuantity(101, 5);
        expect(
          container
              .read(cartControllerProvider)
              .requireValue
              .data
              ?.items
              .single
              .count,
          5,
        );

        await controller.flushPendingQuantityUpdates();
        final state = container.read(cartControllerProvider).requireValue;

        expect(state.status, CartContentStatus.ready);
        expect(state.data?.items.single.count, 2);
        expect(state.problem?.kind, CartProblemKind.staleData);
        expect(state.problem?.message, 'quantity unavailable');
        expect(await container.read(cartCountProvider.future), 2);
      },
    );
  });

  test('successful merge survives snapshot refresh failure', () async {
    final dio = Dio();
    var reads = 0;
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          if (options.path == '/public/carts') {
            reads++;
            handler.reject(DioException(requestOptions: options));
          } else {
            handler.resolve(
              Response(requestOptions: options, data: {'code': 0}),
            );
          }
        },
      ),
    );
    final repository = CartRepository(dio);
    addTearDown(repository.dispose);
    addTearDown(dio.close);
    await expectLater(repository.mergeCarts('guest-cart-id'), completes);
    expect(reads, 1);
  });

  test(
    'guest/member merge updates screen state and badge from one snapshot',
    () async {
      var quantity = 1;
      final dio = Dio();
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (options, handler) {
            if (options.path == '/app/carts/merge') {
              expect(options.data, {'guestId': 'guest-cart-id'});
              quantity = 5;
            }
            handler.resolve(
              Response(
                requestOptions: options,
                data: {
                  'code': 0,
                  'data': {
                    'groups': [
                      {
                        'storeId': '7',
                        'items': [
                          {'skuId': 101, 'quantity': quantity, 'price': 120},
                        ],
                      },
                    ],
                  },
                },
              ),
            );
          },
        ),
      );
      final repository = CartRepository(dio);
      addTearDown(repository.dispose);
      addTearDown(dio.close);
      final container = _containerFor(repository);
      addTearDown(container.dispose);
      await container.read(cartControllerProvider.future);

      await repository.mergeCarts('guest-cart-id');
      expect(
        container
            .read(cartControllerProvider)
            .requireValue
            .data
            ?.items
            .single
            .count,
        5,
      );
      expect(await container.read(cartCountProvider.future), 5);
    },
  );

  test(
    'session switch cancels pending quantity work from prior cart',
    () async {
      final repository = _FakeCartRepository(
        loads: [_cart(quantity: 1), _cart(quantity: 7)],
      );
      final container = _containerFor(repository);
      addTearDown(container.dispose);
      await container.read(cartControllerProvider.future);
      final controller = container.read(cartControllerProvider.notifier);
      controller.updateQuantity(101, 4);

      (container.read(sessionProvider.notifier) as _ActiveSession).replaceUser(
        'buyer-2',
      );
      final switched = await container.read(cartControllerProvider.future);
      await controller.flushPendingQuantityUpdates();

      expect(switched.data?.items.single.count, 7);
      expect(repository.quantityUpdates, isEmpty);
    },
  );

  test('CartScreen has no transport or wire-schema data layer', () {
    final source = File(
      'lib/features/cart/presentation/cart_screen.dart',
    ).readAsStringSync();

    expect(source, isNot(contains('dioProvider')));
    expect(source, isNot(contains('Dio')));
    expect(source, isNot(contains('resp.data')));
    expect(source, isNot(contains("'/public/carts'")));
    expect(source, isNot(contains("'/app/checkout/validate'")));
    expect(source, isNot(contains("'/app/checkout/shipping_fee'")));
    expect(source, isNot(contains("'/app/buyers/me/addresses'")));
    expect(source, isNot(contains("'/app/store_pickups'")));
  });
}

ProviderContainer _containerFor(CartRepository repository) {
  return ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(_ActiveSession.new),
      cartRepositoryProvider.overrideWithValue(repository),
      addressRepositoryProvider.overrideWithValue(_EmptyAddressRepository()),
    ],
  );
}

class _EmptyAddressRepository extends AddressRepository {
  _EmptyAddressRepository()
    : super(
        BuyerAppServiceClient(FakeTransportBuilder().build()),
        defaults: zhDefaultErrorText,
      );

  @override
  Future<List<Address>> getAddresses() async => [];
}

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Buyer');

  void replaceUser(String userId) {
    state = AsyncData(SessionModel(userId: userId, nickname: 'Buyer'));
  }
}

class _FakeCartRepository extends CartRepository {
  _FakeCartRepository({
    required Iterable<Object> loads,
    this.validation,
    this.updateFailure,
  }) : loads = Queue<Object>.of(loads),
       super(Dio());

  final Queue<Object> loads;
  final CartValidation? validation;
  final Object? updateFailure;
  final List<({int skuId, int quantity})> quantityUpdates = [];

  @override
  Future<CartData> getCartData({String? guestId}) async {
    final result = loads.removeFirst();
    if (result is Exception) throw result;
    return result as CartData;
  }

  @override
  Future<List<Map<String, dynamic>>> getStorePickups() async => const [];

  @override
  Future<CartValidation> validateCheckout({
    required List<CartItem> items,
    required Map<int, String> productIdsBySku,
  }) async {
    return validation ??
        CartValidation(
          valid: true,
          result: const {'valid': true},
          shippingOptions: const [],
          errors: const [],
        );
  }

  @override
  Future<CartData> updateItem(int skuId, int count, {String? guestId}) async {
    quantityUpdates.add((skuId: skuId, quantity: count));
    final failure = updateFailure;
    if (failure is Exception) throw failure;
    return _cart(quantity: count);
  }
}

CartData _cart({required int quantity}) {
  final item = CartItem(
    id: 101,
    skuId: 101,
    count: quantity,
    price: 120,
    title: '測試蔬果箱',
    sellerId: 7,
    merchantName: '範例村',
    purchasable: true,
  );
  return CartData(
    groups: [
      CartMerchantGroup(storeId: '7', sellerName: '範例村', items: [item]),
    ],
    items: [item],
    stockBySku: const {101: 20},
    deliveryTempsBySku: const {
      101: ['3201'],
    },
    productIdsBySku: const {101: 'product-101'},
    lockedSkuIds: const {},
    estimatedTotalAmount: 120 * quantity,
  );
}
