import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';
import 'category_marketplace_model.dart';
import 'category_model.dart';
import 'product_item_model.dart';

part 'category_repository.g.dart';

class MarketSellerPreview {
  const MarketSellerPreview({
    required this.id,
    required this.name,
    required this.avatarUrl,
    required this.city,
    required this.district,
  });

  final String id;
  final String name;
  final String avatarUrl;
  final String city;
  final String district;

  String get location => '$city$district'.trim();
}

@riverpod
CategoryRepository categoryRepository(Ref ref) =>
    CategoryRepository(ref.watch(dioProvider));

class CategoryRepository {
  final Dio _dio;
  CategoryRepository(this._dio);

  List<CategoryMarketplaceProduct>? _marketplaceRecommendationCache;
  Future<List<CategoryMarketplaceProduct>>? _marketplaceRecommendationRequest;

  /// GET /public/categories/tree → nested category tree
  Future<List<Category>> fetchCategoryTree() async {
    final resp = await _dio.get('/public/categories/tree');
    final data = resp.data;
    List<dynamic> list = [];
    if (data is List) {
      list = data;
    } else if (data is Map) {
      list = (data['data'] ?? data['result'] ?? []) as List? ?? [];
    }
    return list
        .whereType<Map>()
        .map((e) => Category.fromJson(Map<String, dynamic>.from(e)))
        .toList();
  }

  /// GET /public/products → paginated product list by category
  Future<ProductListResult> fetchProducts({
    String? categoryId,
    int page = 1,
    int pageSize = 20,
    String? keyword,
    String? sort,
    String? priceMin,
    String? priceMax,
    List<String> hashtags = const [],
    bool followedSellerOnly = false,
  }) async {
    final params = <String, dynamic>{'page': page, 'page_size': pageSize};
    if (categoryId != null && categoryId != '0') {
      params['category_id'] = categoryId;
    }
    if (keyword != null && keyword.isNotEmpty) params['keyword'] = keyword;
    if (sort != null && sort.isNotEmpty) params['sort'] = sort;
    if (priceMin != null && priceMin.isNotEmpty) {
      params['price_min'] = priceMin;
    }
    if (priceMax != null && priceMax.isNotEmpty) {
      params['price_max'] = priceMax;
    }
    if (hashtags.isNotEmpty) params['hashtags'] = hashtags.join(',');
    if (followedSellerOnly) params['followed_seller_only'] = true;

    final resp = await _dio.get('/public/products', queryParameters: params);
    final body = resp.data;

    if (body is Map) {
      final inner = body['data'];
      if (inner is Map) {
        final rawRecords = (inner['records'] as List? ?? [])
            .whereType<Map>()
            .toList();
        final records = rawRecords
            .map((e) => ProductItem.fromJson(Map<String, dynamic>.from(e)))
            .toList();
        return ProductListResult(
          records: records,
          total: int.tryParse(inner['total']?.toString() ?? '0') ?? 0,
          page: int.tryParse(inner['page']?.toString() ?? '1') ?? page,
          pageSize:
              int.tryParse(inner['pageSize']?.toString() ?? '20') ?? pageSize,
        );
      }
    }
    return ProductListResult(
      records: [],
      total: 0,
      page: page,
      pageSize: pageSize,
    );
  }

  /// GET /public/hashtags/popular → Web 市集側欄的熱門 hashtag。
  Future<List<String>> fetchPopularHashtags({int limit = 8}) async {
    final resp = await _dio.get(
      '/public/hashtags/popular',
      queryParameters: {'limit': limit},
    );
    final body = resp.data;
    final data = body is Map ? body['data'] : null;
    final records = data is Map ? data['records'] : null;
    if (records is! List) return const [];
    final result = <String>[];
    final seen = <String>{};
    for (final raw in records.whereType<Map>()) {
      final value = (raw['hashtag'] ?? raw['name'] ?? '')
          .toString()
          .trim()
          .replaceFirst(RegExp(r'^#+'), '');
      if (value.isEmpty || !seen.add(value.toLowerCase())) continue;
      result.add(value);
    }
    return result;
  }

  /// GET /public/marketplace → Web 市集頂部的推薦商家 story rail。
  Future<List<MarketSellerPreview>> fetchRecommendedSellers({
    int limit = 10,
  }) async {
    final resp = await _dio.get(
      '/public/marketplace',
      queryParameters: {
        'seller_limit': limit,
        'post_limit': 0,
        'product_limit': 0,
      },
    );
    final body = resp.data;
    final data = body is Map ? body['data'] : null;
    final records = data is Map ? data['recommendedSellers'] : null;
    if (records is! List) return const [];
    return records
        .whereType<Map>()
        .map(
          (raw) => MarketSellerPreview(
            id: (raw['storeId'] ?? '').toString(),
            name: (raw['storeName'] ?? '').toString(),
            // 頭像取 logoKey/logoUrl（49 §6.1 成對契約）；交
            // AppCachedImage(merchantAvatar) 套 avatar preset，與市集頂部橫列一致。
            avatarUrl: (raw['logoKey'] ?? raw['logoUrl'] ?? '').toString(),
            city: (raw['city'] ?? '').toString(),
            district: (raw['district'] ?? '').toString(),
          ),
        )
        .where((seller) => seller.id.isNotEmpty && seller.name.isNotEmpty)
        .toList(growable: false);
  }

  /// Category landing's generic product recommendations.
  ///
  /// The aggregate response already contains product image and seller fields,
  /// so this is deliberately the only network request for the section.
  /// Successful data is retained for this provider container (the current app
  /// session). Concurrent callers share one in-flight request.
  Future<List<CategoryMarketplaceProduct>> fetchMarketplaceRecommendations({
    int limit = 12,
    bool forceRefresh = false,
  }) {
    final inFlight = _marketplaceRecommendationRequest;
    if (inFlight != null) return inFlight;

    final cached = _marketplaceRecommendationCache;
    if (!forceRefresh && cached != null) return Future.value(cached);

    late final Future<List<CategoryMarketplaceProduct>> request;
    request = _fetchMarketplaceRecommendations(limit: limit).whenComplete(() {
      if (identical(_marketplaceRecommendationRequest, request)) {
        _marketplaceRecommendationRequest = null;
      }
    });
    _marketplaceRecommendationRequest = request;
    return request;
  }

  Future<List<CategoryMarketplaceProduct>> _fetchMarketplaceRecommendations({
    required int limit,
  }) async {
    final resp = await _dio.get<Map<String, dynamic>>(
      '/public/marketplace',
      queryParameters: {
        'seller_limit': 0,
        'post_limit': 0,
        'product_limit': limit,
      },
    );
    final parsed = CategoryMarketplaceData.fromResponse(resp.data);
    final products = List<CategoryMarketplaceProduct>.unmodifiable(
      parsed.products.take(limit),
    );
    _marketplaceRecommendationCache = products;
    return products;
  }

  /// Clears the category recommendation cache without starting a request.
  void invalidateMarketplaceRecommendations() {
    _marketplaceRecommendationCache = null;
  }

  /// 分頁計數的解析。後端一律以 string 過線（§4），因此這裡**只收 string**——
  /// 順手收 num 就是把雙讀相容層寫回來，而相容層正是讓「後端改了、前端沒改」
  /// 這件事永遠不會被任何測試發現的原因。
  static int? _parsePageInt(dynamic v) => v is String ? int.tryParse(v) : null;

  /// GET /public/search/products → 全文搜尋
  Future<ProductListResult> searchProducts({
    required String keyword,
    String? categoryId,
    int page = 1,
    int pageSize = 20,
    String? sortBy,
    String? sortOrder,
  }) async {
    final params = <String, dynamic>{
      'q': keyword,
      'page': page,
      'page_size': pageSize,
    };
    if (categoryId != null && categoryId != '0') {
      params['category_id'] = categoryId;
    }
    if (sortBy != null && sortBy.isNotEmpty) params['sort_by'] = sortBy;
    if (sortOrder != null && sortOrder.isNotEmpty) {
      params['sort_order'] = sortOrder;
    }

    final resp = await _dio.get(
      '/public/search/products',
      queryParameters: params,
    );
    final body = resp.data;

    if (body is Map) {
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '搜尋失敗').toString());
      }
      final inner = body['data'];
      if (inner is Map) {
        final products = (inner['records'] as List? ?? [])
            .whereType<Map>()
            .map((e) => ProductItem.fromSearchHit(Map<String, dynamic>.from(e)))
            .where(
              (product) =>
                  product.id.trim().isNotEmpty &&
                  product.name.trim().isNotEmpty,
            )
            .toList();
        // 分頁四欄以 string 過線（後端 §4）。用 `as int?` 直接接會靜默拿到
        // null → 全部退回預設值，畫面看起來正常但總筆數永遠是 0。
        return ProductListResult(
          records: products,
          total: _parsePageInt(inner['total']) ?? 0,
          page: _parsePageInt(inner['page']) ?? page,
          pageSize: _parsePageInt(inner['pageSize']) ?? pageSize,
        );
      }
    }
    return ProductListResult(
      records: [],
      total: 0,
      page: page,
      pageSize: pageSize,
    );
  }

  /// GET /public/sellers/{id} → seller brief info (name + logo)
  Future<({String name, String logo})> fetchSellerInfo(String accountId) async {
    final resp = await _dio.get('/public/sellers/$accountId');
    final body = resp.data;
    if (body is Map) {
      final data = body['data'];
      if (data is Map) {
        final storeName = data['storeName']?.toString() ?? '';
        final sellerName = data['sellerName']?.toString() ?? '';
        final name = storeName.isNotEmpty ? storeName : sellerName;
        // 49 §6.1 成對契約：logoKey（裸 key）優先，logoUrl 退路。
        // 舊碼讀的是 `logo` / `avatar` —— /public/sellers/{id} 從來沒有這兩個
        // 欄位，所以商家 logo 在這條路徑上一直是空的。
        final logoVal = data['logoKey']?.toString() ?? '';
        final urlVal = data['logoUrl']?.toString() ?? '';
        final logo = logoVal.isNotEmpty ? logoVal : urlVal;
        return (name: name, logo: logo);
      }
    }
    return (name: '', logo: '');
  }

  /// GET /public/search/suggest → autocomplete suggestions
  Future<List<String>> fetchSuggestions(String prefix) async {
    if (prefix.trim().isEmpty) return [];
    final resp = await _dio.get(
      '/public/search/suggest',
      queryParameters: {'q': prefix.trim(), 'limit': 8},
    );
    final body = resp.data;
    if (body is Map) {
      final inner = body['data'];
      if (inner is Map) {
        final list = inner['suggestions'];
        if (list is List) return list.map((e) => e.toString()).toList();
      }
    }
    return [];
  }
}
