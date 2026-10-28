import 'dart:async';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:hawkr_buyer/features/address/data/address_repository.dart';
import 'package:hawkr_buyer/features/cart/data/cart_data.dart';
import 'package:hawkr_buyer/features/cart/data/cart_model.dart';
import 'package:hawkr_buyer/features/cart/data/cart_repository.dart';
import 'package:hawkr_buyer/features/cart/presentation/cart_controller.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../support/default_error_text.dart';

void main() {
  late _Repository repository;
  late _AddressRepository addresses;
  late ProviderContainer container;
  late CartController controller;

  setUp(() async {
    repository = _Repository();
    addresses = _AddressRepository();
    container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_Session.new),
        cartRepositoryProvider.overrideWithValue(repository),
        addressRepositoryProvider.overrideWithValue(addresses),
      ],
    );
    await container.read(cartControllerProvider.future);
    controller = container.read(cartControllerProvider.notifier);
  });
  tearDown(() => container.dispose());

  test(
    'store selection replaces selection but preserves both stores and unavailable rows',
    () {
      controller.selectGroup(repository.data.groups.last, true);
      final state = container.read(cartControllerProvider).requireValue;
      expect(state.checkedSkuIds, {202});
      expect(state.data!.items.map((item) => item.skuId), [101, 202, 203]);
      expect(state.data!.items.last.purchasable, false);
      controller.selectItem(101, true);
      expect(
        container.read(cartControllerProvider).requireValue.checkedSkuIds,
        {101},
      );
      controller.selectItem(203, true);
      expect(
        container.read(cartControllerProvider).requireValue.checkedSkuIds,
        {101},
      );
    },
  );

  test(
    'cleared selection stays empty after refresh and selection preparation',
    () async {
      controller.selectGroup(repository.data.groups.first, false);
      await controller.refresh();
      await controller.initialize();
      final state = container.read(cartControllerProvider).requireValue;
      expect(state.checkedSkuIds, isEmpty);
      expect(state.shippingOptions, isEmpty);
      expect(state.lastValidatedAt, isNull);
      expect(state.data!.groups, hasLength(2));
    },
  );

  test('explicit product preselection still switches to its store', () async {
    await controller.initialize(preSelectedSkuId: 202);
    expect(container.read(cartControllerProvider).requireValue.checkedSkuIds, {
      202,
    });
  });

  test(
    'late validation from prior store cannot replace new shipping or unlock checkout',
    () async {
      final oldRequest = controller.validateNow();
      controller.selectGroup(repository.data.groups.last, true);
      var state = container.read(cartControllerProvider).requireValue;
      expect(state.validateLoading, true);
      expect(state.shippingOptions, isEmpty);
      expect(state.preciseShippingFee, isNull);
      expect(state.lastValidatedAt, isNull);
      final newRequest = controller.validateNow();
      repository.validations[0].complete(_validation('old-store', 120));
      await oldRequest;
      state = container.read(cartControllerProvider).requireValue;
      expect(state.checkedSkuIds, {202});
      expect(state.validateLoading, true);
      expect(state.shippingOptions, isEmpty);
      repository.validations[1].complete(_validation('new-store', 250));
      await newRequest;
      state = container.read(cartControllerProvider).requireValue;
      expect(state.shippingOptions.single['store'], 'new-store');
      expect(state.validateLoading, false);
      expect(state.lastValidatedAt, isNotNull);
      expect(repository.validatedSkus, [
        [101],
        [202],
      ]);
    },
  );

  test(
    'late validation failure from prior store does not replace current state',
    () async {
      final oldRequest = controller.validateNow();
      controller.selectGroup(repository.data.groups.last, true);
      final newRequest = controller.validateNow();
      repository.validations[1].complete(_validation('new-store', 250));
      await newRequest;
      repository.validations[0].completeError(
        const NetworkException('old-store offline'),
      );
      await oldRequest;
      final state = container.read(cartControllerProvider).requireValue;
      expect(state.problem, isNull);
      expect(state.shippingOptions.single['store'], 'new-store');
    },
  );

  test('cancelled selection ignores validation already in flight', () async {
    final request = controller.validateNow();
    controller.selectGroup(repository.data.groups.first, false);
    repository.validations.single.complete(_validation('old-store', 120));
    await request;
    final state = container.read(cartControllerProvider).requireValue;
    expect(state.checkedSkuIds, isEmpty);
    expect(state.shippingOptions, isEmpty);
    expect(state.validateLoading, false);
    expect(state.lastValidatedAt, isNull);
  });

  for (final isAddressRefresh in [true, false]) {
    test(
      'late ${isAddressRefresh ? 'address' : 'pickup'} refresh preserves the newly selected store',
      () async {
        if (isAddressRefresh) {
          addresses.pending = Completer<List<Address>>();
        } else {
          repository.supportingRefresh =
              Completer<List<Map<String, dynamic>>>();
        }
        final request = isAddressRefresh
            ? controller.refreshAddresses()
            : controller.refreshStorePickups();
        controller.selectGroup(repository.data.groups.last, true);
        if (isAddressRefresh) {
          addresses.pending!.complete([]);
        } else {
          repository.supportingRefresh!.complete([]);
        }
        await request;
        final state = container.read(cartControllerProvider).requireValue;
        expect(state.checkedSkuIds, {202});
        expect(state.validateLoading, true);
        expect(state.shippingOptions, isEmpty);
      },
    );
  }

  test(
    'refresh keeps checkout pending even while its previous snapshot is still visible',
    () async {
      final validation = controller.validateNow();
      repository.validations.single.complete(_validation('first-store', 120));
      await validation;
      expect(
        container.read(cartControllerProvider).requireValue.checkoutPending,
        false,
      );
      repository.refreshResponse = Completer<CartData>();
      final refresh = controller.refresh();
      expect(
        container.read(cartControllerProvider).requireValue.checkoutPending,
        true,
      );
      repository.refreshResponse!.complete(repository.data);
      await refresh;
      expect(
        container.read(cartControllerProvider).requireValue.validateLoading,
        true,
      );
      expect(
        container.read(cartControllerProvider).requireValue.lastValidatedAt,
        isNull,
      );
    },
  );

  test(
    'a fast validation cannot unlock checkout while quantity mutation is still pending',
    () async {
      controller.updateQuantity(101, 2);
      final flush = controller.flushPendingQuantityUpdates();
      final validation = controller.validateNow();
      repository.validations.single.complete(_validation('first-store', 120));
      await validation;
      final state = container.read(cartControllerProvider).requireValue;
      expect(state.validateLoading, false);
      expect(state.updatingSkuIds, {101});
      expect(state.checkoutPending, true);
      repository.quantityResponse.complete(
        repository.data.replaceQuantity(101, 2),
      );
      await flush;
      expect(
        container.read(cartControllerProvider).requireValue.validateLoading,
        true,
      );
    },
  );

  test(
    'current validation network failure remains a visible problem without crashing',
    () async {
      final request = controller.validateNow();
      repository.validations.single.completeError(
        const NetworkException('暫時無法確認配送'),
      );
      await request;
      final state = container.read(cartControllerProvider).requireValue;
      expect(state.problem?.message, '暫時無法確認配送');
      expect(state.validateLoading, false);
      expect(state.shippingOptions, isEmpty);
      expect(state.lastValidatedAt, isNull);
    },
  );

  test(
    'late precise fee from prior store cannot replace current store fee',
    () async {
      final oldValidation = controller.validateNow();
      repository.validations[0].complete(_validation('old-store', 120));
      await oldValidation;
      final oldFee = controller.calculateShippingFee();
      controller.selectGroup(repository.data.groups.last, true);
      final newValidation = controller.validateNow();
      repository.validations[1].complete(_validation('new-store', 250));
      await newValidation;
      final newFee = controller.calculateShippingFee();
      expect(
        container.read(cartControllerProvider).requireValue.checkoutPending,
        true,
      );
      repository.fees[1].complete(250);
      await newFee;
      repository.fees[0].complete(120);
      await oldFee;
      final state = container.read(cartControllerProvider).requireValue;
      expect(state.preciseShippingFee, 250);
      expect(state.checkoutPending, false);
      expect(state.shippingOptions.single['store'], 'new-store');
    },
  );
}

class _Session extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Buyer');
}

class _Repository extends CartRepository {
  _Repository() : super(Dio());
  final CartData data = _cart();
  final validations = <Completer<CartValidation>>[];
  final validatedSkus = <List<int>>[];
  final fees = <Completer<double>>[];
  Completer<CartData>? refreshResponse;
  final quantityResponse = Completer<CartData>();
  Completer<List<Map<String, dynamic>>>? supportingRefresh;
  @override
  Future<CartData> getCartData({String? guestId}) async =>
      refreshResponse != null ? await refreshResponse!.future : data;
  @override
  Future<CartData> updateItem(int skuId, int count, {String? guestId}) =>
      quantityResponse.future;
  @override
  Future<List<Map<String, dynamic>>> getStorePickups() async =>
      supportingRefresh != null ? await supportingRefresh!.future : const [];
  @override
  Future<CartValidation> validateCheckout({
    required List<CartItem> items,
    required Map<int, String> productIdsBySku,
  }) {
    validatedSkus.add(items.map((item) => item.skuId).toList());
    final completer = Completer<CartValidation>();
    validations.add(completer);
    return completer.future;
  }

  @override
  Future<double> getShippingFee({
    required List<CartItem> items,
    required Map<int, String> productIdsBySku,
    required Map<String, dynamic> shippingOption,
    required Address address,
  }) {
    final completer = Completer<double>();
    fees.add(completer);
    return completer.future;
  }
}

class _AddressRepository extends AddressRepository {
  _AddressRepository()
    : super(
        BuyerAppServiceClient(FakeTransportBuilder().build()),
        defaults: zhDefaultErrorText,
      );

  Completer<List<Address>>? pending;

  @override
  Future<List<Address>> getAddresses() async => pending == null
      ? const [
          Address(
            id: 1,
            isDefault: true,
            etag: 'v1',
            receiverName: '測試收件人',
            receiverPhone: '0912345678',
          ),
        ]
      : pending!.future;
}

CartValidation _validation(String store, int fee) => CartValidation(
  valid: true,
  result: const {'valid': true},
  shippingOptions: [
    {'store': store, 'shippingMethod': '3001', 'fee': '$fee'},
  ],
  errors: const [],
);

CartData _cart() {
  final first = CartItem(
    id: 101,
    skuId: 101,
    count: 1,
    price: 120,
    title: '蔬果箱',
    sellerId: 7,
    purchasable: true,
  );
  final second = CartItem(
    id: 202,
    skuId: 202,
    count: 1,
    price: 420,
    title: '海鮮箱',
    sellerId: 8,
    purchasable: true,
  );
  final unavailable = CartItem(
    id: 203,
    skuId: 203,
    count: 1,
    price: 500,
    title: '已售完商品',
    sellerId: 8,
    purchasable: false,
  );
  return CartData(
    groups: [
      CartMerchantGroup(storeId: '7', sellerName: '範例蔬果店', items: [first]),
      CartMerchantGroup(
        storeId: '8',
        sellerName: '範例海鮮店',
        items: [second, unavailable],
      ),
    ],
    items: [first, second, unavailable],
    stockBySku: const {101: 20, 202: 5, 203: 0},
    productIdsBySku: const {
      101: 'product-101',
      202: 'product-202',
      203: 'product-203',
    },
    lockedSkuIds: const {203},
    estimatedTotalAmount: 1040,
  );
}
