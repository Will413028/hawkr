import '../../product/data/hashtag_extractor.dart';

/// Typed product card data returned by `GET /public/products`.
///
/// The API has shipped both flat seller fields and a nested `seller` object.
/// Parsing those compatibility aliases here keeps the presentation layer free
/// from response-shape branching.
class HomeProduct {
  factory HomeProduct({
    required String id,
    required String name,
    required String imageUrl,
    required String priceMin,
    required String priceMax,
    required String listPriceMin,
    required String listPriceMax,
    required String salePriceMin,
    required String salePriceMax,
    required String sellerName,
    required String sellerAvatarUrl,
    required String sellerAccountId,
    required List<String> hashtags,
  }) {
    return HomeProduct._(
      id: id,
      name: name,
      imageUrl: imageUrl,
      priceMin: priceMin,
      priceMax: priceMax,
      listPriceMin: listPriceMin,
      listPriceMax: listPriceMax,
      salePriceMin: salePriceMin,
      salePriceMax: salePriceMax,
      sellerName: sellerName,
      sellerAvatarUrl: sellerAvatarUrl,
      sellerAccountId: sellerAccountId,
      hashtags: List.unmodifiable(hashtags),
    );
  }

  const HomeProduct._({
    required this.id,
    required this.name,
    required this.imageUrl,
    required this.priceMin,
    required this.priceMax,
    required this.listPriceMin,
    required this.listPriceMax,
    required this.salePriceMin,
    required this.salePriceMax,
    required this.sellerName,
    required this.sellerAvatarUrl,
    required this.sellerAccountId,
    required this.hashtags,
  });

  final String id;
  final String name;
  final String imageUrl;
  final String priceMin;
  final String priceMax;
  final String listPriceMin;
  final String listPriceMax;
  final String salePriceMin;
  final String salePriceMax;
  final String sellerName;
  final String sellerAvatarUrl;
  final String sellerAccountId;
  final List<String> hashtags;

  factory HomeProduct.fromJson(Map<String, dynamic> json) {
    final sellerRaw = json['seller'];
    final seller = sellerRaw is Map
        ? Map<String, dynamic>.from(sellerRaw)
        : const <String, dynamic>{};
    final id = (json['id'] ?? '').toString().trim();
    if (id.isEmpty) {
      throw const FormatException('Home product requires a non-empty id.');
    }

    return HomeProduct(
      id: id,
      name: (json['name'] ?? '').toString().trim(),
      // 49 §6.1 成對契約：imageKey（objectKey）優先，imageUrl 退路。
      imageUrl: _firstNonEmpty([json['imageKey'], json['imageUrl']]),
      priceMin: _firstNonEmpty([json['priceMin']]),
      priceMax: _firstNonEmpty([json['priceMax']]),
      listPriceMin: _firstNonEmpty([json['listPriceMin']]),
      listPriceMax: _firstNonEmpty([json['listPriceMax']]),
      salePriceMin: _firstNonEmpty([json['salePriceMin']]),
      salePriceMax: _firstNonEmpty([json['salePriceMax']]),
      // ProductItem 的 flat 層與巢狀 seller 物件都有店名；flat 層優先。
      sellerName: _firstNonEmpty([json['sellerName'], seller['name']]),
      sellerAvatarUrl: _firstNonEmpty([
        json['sellerLogoKey'],
        json['sellerLogoUrl'],
        seller['logoKey'],
        seller['logoUrl'],
      ]),
      sellerAccountId: _firstNonEmpty([json['storeId'], seller['storeId']]),
      // Keep the legacy description-block fallback in the data layer. The UI
      // receives only a normalized list of labels.
      hashtags: BuyerHashtagExtractor.fromProduct(json),
    );
  }

  static String _firstNonEmpty(Iterable<dynamic> values) {
    for (final value in values) {
      final text = value?.toString().trim() ?? '';
      if (text.isNotEmpty && text != 'null') return text;
    }
    return '';
  }
}

/// One immutable page from the home recommendation endpoint.
class HomeProductPage {
  HomeProductPage({
    required List<HomeProduct> products,
    required this.page,
    required this.pageSize,
    required this.hasMore,
    this.total,
  }) : products = List.unmodifiable(products);

  final List<HomeProduct> products;
  final int page;
  final int pageSize;
  final int? total;
  final bool hasMore;
}
