import 'product_item_model.dart';

/// 分類首頁的商品推薦：依銷量排序的商品列表（`GET /public/products?sort=SALES`）的一筆。
class CategoryMarketplaceProduct {
  const CategoryMarketplaceProduct({
    required this.id,
    required this.name,
    required this.imageUrl,
    required this.merchantId,
    required this.merchantName,
    required this.priceMin,
    required this.priceMax,
  });

  final String id;
  final String name;
  final String imageUrl;
  final String merchantId;
  final String merchantName;
  final String priceMin;
  final String priceMax;

  double? get navigationPrice => double.tryParse(priceMin);

  /// 商品列表（`GET /public/products`）的一筆；沒有名稱的不能顯示，回 null。
  static CategoryMarketplaceProduct? fromProductItem(ProductItem item) {
    final id = item.id.trim();
    final name = item.name.trim();
    if (id.isEmpty || name.isEmpty) return null;
    return CategoryMarketplaceProduct(
      id: id,
      name: name,
      imageUrl: item.imageUrl?.trim() ?? '',
      merchantId: item.storeId?.trim() ?? '',
      merchantName: item.sellerName?.trim() ?? '',
      priceMin: item.priceMin?.trim() ?? '',
      priceMax: item.priceMax?.trim() ?? '',
    );
  }
}
