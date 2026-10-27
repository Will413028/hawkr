import 'dart:collection';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/shop_tab_repository.dart';

part 'shop_tab_controller.g.dart';

final class ShopTabState {
  ShopTabState({
    this.isLoadingProducts = false,
    Iterable<Map<String, dynamic>> products = const [],
  }) : products = UnmodifiableListView<Map<String, dynamic>>(
         products
             .map(
               (product) => Map<String, dynamic>.unmodifiable(
                 Map<String, dynamic>.from(product),
               ),
             )
             .toList(growable: false),
       );

  final bool isLoadingProducts;
  final UnmodifiableListView<Map<String, dynamic>> products;

  ShopTabState copyWith({
    bool? isLoadingProducts,
    Iterable<Map<String, dynamic>>? products,
  }) {
    return ShopTabState(
      isLoadingProducts: isLoadingProducts ?? this.isLoadingProducts,
      products: products ?? this.products,
    );
  }
}

@riverpod
class ShopTabController extends _$ShopTabController {
  @override
  ShopTabState build(String sellerId) => ShopTabState();

  Future<void> refreshProducts() async {
    if (sellerId.isEmpty) return;
    final repository = ref.read(shopTabRepositoryProvider);
    state = state.copyWith(isLoadingProducts: true);

    final cached = await repository.readCachedProducts(sellerId);
    if (cached.isNotEmpty) {
      state = state.copyWith(products: cached);
    }

    try {
      final products = await repository.refreshProducts(sellerId);
      state = state.copyWith(products: products);
    } finally {
      state = state.copyWith(isLoadingProducts: false);
    }
  }

  void removeProduct(String skuKey) {
    state = state.copyWith(
      products: state.products.where(
        (product) =>
            (product['skuId'] ?? product['sku_id'] ?? product['id'])
                .toString() !=
            skuKey,
      ),
    );
  }
}
