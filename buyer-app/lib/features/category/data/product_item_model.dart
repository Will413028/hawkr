/// Product list item from GET /public/products API.
///
/// PayUni issue #758:
/// - priceMin/Max = 實際售價區間（salePrice ?? price）
/// - listPriceMin/Max = 原價區間（刪除線）
/// - salePriceMin/Max = 特價區間，全部無特價時為 "0"
library;

class ProductItem {
  final String id;
  final String name;
  final String? categoryId;
  final String? categoryName;
  final String? imageUrl;
  final List<String> images;
  final String? origin;
  final String? priceMin;
  final String? priceMax;
  final String? listPriceMin;
  final String? listPriceMax;
  final String? salePriceMin;
  final String? salePriceMax;
  final String? rating;
  final String? reviewCount;
  final String? storeId;
  final String? sellerName;
  final String? sellerAvatar;
  final List<String> hashtags;
  final String? status;
  final int? stockQty;
  final bool isFavorited;

  const ProductItem({
    required this.id,
    required this.name,
    this.categoryId,
    this.categoryName,
    this.imageUrl,
    this.images = const [],
    this.origin,
    this.priceMin,
    this.priceMax,
    this.listPriceMin,
    this.listPriceMax,
    this.salePriceMin,
    this.salePriceMax,
    this.rating,
    this.reviewCount,
    this.storeId,
    this.sellerName,
    this.sellerAvatar,
    this.hashtags = const [],
    this.status,
    this.stockQty,
    this.isFavorited = false,
  });

  /// `GET /public/products` / `GET /public/sellers/{id}/products` 的一筆 record
  /// （gateway `ProductItem`）。JSON body 欄名一律 camelCase（10 檔 §2）。
  factory ProductItem.fromJson(Map<String, dynamic> json) {
    final seller = json['seller'];
    final sellerMap = seller is Map
        ? Map<String, dynamic>.from(seller)
        : const <String, dynamic>{};
    return ProductItem(
      id: (json['id'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      categoryId: json['categoryId']?.toString(),
      categoryName: json['categoryName']?.toString(),
      imageUrl: (json['imageKey'] ?? json['imageUrl'])?.toString(),
      images: _parseImages(json),
      origin: json['origin']?.toString(),
      priceMin: json['priceMin']?.toString(),
      priceMax: json['priceMax']?.toString(),
      listPriceMin: json['listPriceMin']?.toString(),
      listPriceMax: json['listPriceMax']?.toString(),
      salePriceMin: json['salePriceMin']?.toString(),
      salePriceMax: json['salePriceMax']?.toString(),
      rating: _formatRating(json['rating']),
      reviewCount: json['reviewCount']?.toString(),
      storeId: (json['storeId'] ?? sellerMap['storeId'])?.toString(),
      sellerName: _firstNonEmptyString([json['sellerName'], sellerMap['name']]),
      sellerAvatar: _firstNonEmptyString([
        json['sellerLogoKey'],
        json['sellerLogoUrl'],
        sellerMap['logoKey'],
        sellerMap['logoUrl'],
      ]),
      hashtags: _parseHashtags(json['hashtags']),
      status: json['status']?.toString(),
      stockQty: _parseInt(json['stock']),
      isFavorited: _parseBool(json['isFavorited']),
    );
  }

  /// `GET /public/search/products` 的 `ProductHitData`，與 `/public/products`
  /// 同形，但不含 `listPrice*` / `salePrice*` / `isFavorited`（搜尋索引不帶）。
  factory ProductItem.fromSearchHit(Map<String, dynamic> json) {
    final seller = json['seller'];
    final sellerMap = seller is Map
        ? Map<String, dynamic>.from(seller)
        : const <String, dynamic>{};
    return ProductItem(
      id: (json['id'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      categoryId: json['categoryId']?.toString(),
      categoryName: json['categoryName']?.toString(),
      imageUrl: (json['imageKey'] ?? json['imageUrl'])?.toString(),
      images: _parseImages(json),
      origin: json['origin']?.toString(),
      priceMin: json['priceMin']?.toString(),
      priceMax: json['priceMax']?.toString(),
      rating: _formatRating(json['rating']),
      reviewCount: json['reviewCount']?.toString(),
      storeId: (json['storeId'] ?? sellerMap['storeId'])?.toString(),
      sellerName: _firstNonEmptyString([json['sellerName'], sellerMap['name']]),
      sellerAvatar: _firstNonEmptyString([
        json['sellerLogoKey'],
        json['sellerLogoUrl'],
        sellerMap['logoKey'],
        sellerMap['logoUrl'],
      ]),
      hashtags: _parseHashtags(json['hashtags']),
      status: json['status']?.toString(),
      stockQty: _parseInt(json['stock']),
    );
  }

  static bool _parseBool(dynamic value) {
    if (value is bool) return value;
    final normalized = value?.toString().toLowerCase().trim() ?? '';
    return normalized == 'true' || normalized == '1';
  }

  static List<String> _parseHashtags(dynamic value) {
    if (value is! List) return const [];
    final result = <String>[];
    final seen = <String>{};
    for (final raw in value) {
      final text = raw is Map ? raw['name']?.toString() : raw?.toString();
      final tag = (text ?? '').trim().replaceFirst(RegExp(r'^#+'), '');
      if (tag.isEmpty || !seen.add(tag.toLowerCase())) continue;
      result.add(tag);
      if (result.length == 3) break;
    }
    return result;
  }

  static int? _parseInt(dynamic value) {
    if (value is int) return value;
    if (value is num) return value.toInt();
    return int.tryParse(value?.toString() ?? '');
  }

  static String? _firstNonEmptyString(Iterable<dynamic> values) {
    for (final value in values) {
      final text = value?.toString().trim() ?? '';
      if (text.isNotEmpty && text != 'null') return text;
    }
    return null;
  }

  static String? _formatRating(dynamic value) {
    if (value == null) return null;
    final number = value is num
        ? value.toDouble()
        : double.tryParse(value.toString());
    if (number == null) {
      final text = value.toString().trim();
      return text.isEmpty || text == 'null' ? null : text;
    }
    final normalized = number > 5 && number <= 50 ? number / 10 : number;
    if (normalized == normalized.roundToDouble()) {
      return normalized.toStringAsFixed(0);
    }
    return normalized.toStringAsFixed(1);
  }

  static List<String> _parseImages(Map<String, dynamic> json) {
    for (final field in const ['imageKeys', 'imageUrls']) {
      final value = json[field];
      if (value is List) {
        final list = value
            .map((e) => e.toString())
            .where((s) => s.isNotEmpty && s != 'null')
            .toList();
        if (list.isNotEmpty) return list;
      }
    }
    final cover = (json['imageKey'] ?? json['imageUrl'] ?? '').toString();
    if (cover.isNotEmpty && cover != 'null') return [cover];
    return [];
  }


  String get displayImage {
    if (imageUrl != null && imageUrl!.isNotEmpty && imageUrl != 'null') {
      return imageUrl!;
    }
    if (images.isNotEmpty) return images.first;
    return '';
  }

  bool get isSoldOut {
    final normalized = status?.toLowerCase().trim() ?? '';
    if (const {
      'sold_out',
      'soldout',
      'out_of_stock',
      'outofstock',
      'sold-out',
      '已售完',
      '售完',
    }.contains(normalized)) {
      return true;
    }
    if (stockQty != null && stockQty! <= 0) return true;
    return false;
  }
}

/// Paginated product list response.
class ProductListResult {
  final List<ProductItem> records;
  final int total;
  final int page;
  final int pageSize;

  const ProductListResult({
    required this.records,
    required this.total,
    required this.page,
    required this.pageSize,
  });

  /// Uses the server's unfiltered pagination metadata. The client may omit an
  /// invalid record from [records], so list length is not a safe last-page
  /// signal.
  bool get isLastPage => pageSize <= 0 || page * pageSize >= total;
}
