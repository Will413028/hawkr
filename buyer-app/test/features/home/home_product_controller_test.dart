import 'dart:async';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/home/data/home_product_model.dart';
import 'package:hawkr_buyer/features/home/data/home_product_repository.dart';
import 'package:hawkr_buyer/features/home/presentation/home_product_controller.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _FakeHomeProductRepository implements HomeProductRepository {
  _FakeHomeProductRepository(this.handler);

  final Future<HomeProductPage> Function(int page, int pageSize) handler;
  final List<({int page, int pageSize})> requests = [];

  @override
  Future<HomeProductPage> fetchPage({
    required int page,
    required int pageSize,
  }) {
    requests.add((page: page, pageSize: pageSize));
    return handler(page, pageSize);
  }
}

void main() {
  test(
    'initial failure is explicit and retry requests the preview again',
    () async {
      var attempt = 0;
      final repository = _FakeHomeProductRepository((page, pageSize) async {
        attempt++;
        if (attempt == 1) throw const NetworkException('離線');
        return _page(count: 12);
      });
      final container = ProviderContainer(
        overrides: [
          homeProductRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      final controller = container.read(homeProductControllerProvider.notifier);

      await controller.loadInitial();

      var state = container.read(homeProductControllerProvider);
      expect(state.products, isEmpty);
      expect(state.initialError, isA<NetworkException>());
      expect(state.isInitialLoading, isFalse);

      await controller.retryInitial();

      state = container.read(homeProductControllerProvider);
      expect(repository.requests, [
        (page: 1, pageSize: HomeProductController.previewLimit),
        (page: 1, pageSize: HomeProductController.previewLimit),
      ]);
      expect(state.products, hasLength(HomeProductController.previewLimit));
      expect(state.products.last.id, '5');
      expect(state.initialError, isNull);
    },
  );

  test('the session preview does not refetch after it has content', () async {
    final repository = _FakeHomeProductRepository(
      (_, __) async => _page(count: HomeProductController.previewLimit),
    );
    final container = ProviderContainer(
      overrides: [homeProductRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);
    final controller = container.read(homeProductControllerProvider.notifier);

    await controller.loadInitial();
    await controller.loadInitial();

    expect(repository.requests, hasLength(1));
  });

  test('concurrent initial requests are coalesced into one request', () async {
    final response = Completer<HomeProductPage>();
    final repository = _FakeHomeProductRepository((_, __) => response.future);
    final container = ProviderContainer(
      overrides: [homeProductRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);
    final controller = container.read(homeProductControllerProvider.notifier);

    final first = controller.loadInitial();
    final duplicate = controller.loadInitial();
    expect(repository.requests, hasLength(1));

    response.complete(_page(count: HomeProductController.previewLimit));
    await Future.wait([first, duplicate]);

    expect(
      container.read(homeProductControllerProvider).products,
      hasLength(HomeProductController.previewLimit),
    );
  });
}

HomeProductPage _page({required int count}) {
  return HomeProductPage(
    products: List.generate(count, _product),
    page: 1,
    pageSize: HomeProductController.previewLimit,
    hasMore: count >= HomeProductController.previewLimit,
  );
}

HomeProduct _product(int index) => HomeProduct(
  id: '$index',
  name: '商品 $index',
  imageUrl: '',
  priceMin: '${100 + index}',
  priceMax: '',
  listPriceMin: '',
  listPriceMax: '',
  salePriceMin: '',
  salePriceMax: '',
  sellerName: '商家',
  sellerAvatarUrl: '',
  sellerAccountId: 'seller',
  hashtags: const [],
);
