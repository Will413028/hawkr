import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../product/data/product_repository.dart';
import 'shop_tab_local_service.dart';

part 'shop_tab_repository.g.dart';

abstract interface class ShopTabRepository {
  Future<List<Map<String, dynamic>>> readCachedProducts(String sellerId);

  Future<List<Map<String, dynamic>>> refreshProducts(String sellerId);
}

class RemoteShopTabRepository implements ShopTabRepository {
  const RemoteShopTabRepository({
    required ProductRepository productRepository,
    required ShopTabLocalService localService,
  }) : _productRepository = productRepository,
       _localService = localService;

  final ProductRepository _productRepository;
  final ShopTabLocalService _localService;

  @override
  Future<List<Map<String, dynamic>>> readCachedProducts(String sellerId) async {
    try {
      return await _localService.readProducts(sellerId);
    } catch (_) {
      return const [];
    }
  }

  @override
  Future<List<Map<String, dynamic>>> refreshProducts(String sellerId) async {
    final products = await _productRepository.getSellerProducts(sellerId);
    try {
      await _localService.writeProducts(sellerId, products);
    } catch (_) {
      // Cache writes are best effort and never hide fresh remote data.
    }
    return _freezeProducts(products);
  }

  List<Map<String, dynamic>> _freezeProducts(
    Iterable<Map<String, dynamic>> products,
  ) {
    return List<Map<String, dynamic>>.unmodifiable(
      products.map(
        (product) => Map<String, dynamic>.unmodifiable(
          Map<String, dynamic>.from(product),
        ),
      ),
    );
  }
}

@riverpod
ShopTabRepository shopTabRepository(Ref ref) {
  return RemoteShopTabRepository(
    productRepository: ref.watch(productRepositoryProvider),
    localService: ref.watch(shopTabLocalServiceProvider),
  );
}
