/// Product recommendation returned by `GET /public/marketplace`.
///
/// The aggregate endpoint uses `handler.ProductItem` (camelCase). Snake-case
/// aliases are accepted because older deployed responses and the search
/// landing already support them.
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

  factory CategoryMarketplaceProduct.fromJson(Map<String, dynamic> json) {
    final seller = json['seller'];
    final sellerMap = seller is Map
        ? Map<String, dynamic>.from(seller)
        : const <String, dynamic>{};
    final id = (json['id'] ?? '').toString().trim();
    final name = (json['name'] ?? '').toString().trim();
    if (id.isEmpty || name.isEmpty) {
      throw const FormatException(
        'Marketplace product requires a non-empty id and name.',
      );
    }

    return CategoryMarketplaceProduct(
      id: id,
      name: name,
      // 49 §6.1 成對契約：imageKey（objectKey）優先，imageUrl 退路，
      // 最後才退到 imageKeys 陣列第一張。
      imageUrl: _firstNonEmpty([
        json['imageKey'],
        json['imageUrl'],
        _firstImage(json['imageKeys']),
      ]),
      merchantId: _firstNonEmpty([json['storeId'], sellerMap['storeId']]),
      merchantName: _firstNonEmpty([json['sellerName'], sellerMap['name']]),
      priceMin: _firstNonEmpty([json['priceMin']]),
      priceMax: _firstNonEmpty([json['priceMax']]),
    );
  }

  static String _firstImage(dynamic value) {
    if (value is! List) return '';
    for (final item in value) {
      final image = item?.toString().trim() ?? '';
      if (image.isNotEmpty && image != 'null') return image;
    }
    return '';
  }

  static String _firstNonEmpty(Iterable<dynamic> values) {
    for (final value in values) {
      final text = value?.toString().trim() ?? '';
      if (text.isNotEmpty && text != 'null') return text;
    }
    return '';
  }
}

/// Typed marketplace aggregate response used by the category recommendation.
class CategoryMarketplaceData {
  const CategoryMarketplaceData({required this.products});

  final List<CategoryMarketplaceProduct> products;

  factory CategoryMarketplaceData.fromResponse(dynamic body) {
    if (body is! Map) {
      throw const FormatException('Marketplace response must be an object.');
    }

    final code = body['code'];
    if (code != null && int.tryParse(code.toString()) != 0) {
      throw FormatException('Marketplace returned business code $code.');
    }

    final rawData = body['data'];
    if (rawData is! Map) {
      throw const FormatException('Marketplace response is missing data.');
    }

    final rawProducts = rawData['hotProducts'];
    if (rawProducts == null) {
      return const CategoryMarketplaceData(products: []);
    }
    if (rawProducts is! List) {
      throw const FormatException('Marketplace hotProducts must be a list.');
    }

    final products = <CategoryMarketplaceProduct>[];
    final seenIds = <String>{};
    for (final raw in rawProducts) {
      if (raw is! Map) continue;
      try {
        final product = CategoryMarketplaceProduct.fromJson(
          Map<String, dynamic>.from(raw),
        );
        if (seenIds.add(product.id)) products.add(product);
      } on FormatException {
        // A partial aggregate response should keep the valid cards usable.
      }
    }
    return CategoryMarketplaceData(products: List.unmodifiable(products));
  }
}
