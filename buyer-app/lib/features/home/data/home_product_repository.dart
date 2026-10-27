import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';
import 'home_product_model.dart';

part 'home_product_repository.g.dart';

@riverpod
HomeProductRepository homeProductRepository(Ref ref) =>
    DioHomeProductRepository(ref.watch(dioProvider));

abstract interface class HomeProductRepository {
  Future<HomeProductPage> fetchPage({required int page, required int pageSize});
}

class DioHomeProductRepository implements HomeProductRepository {
  DioHomeProductRepository(this._dio);

  final Dio _dio;

  @override
  Future<HomeProductPage> fetchPage({
    required int page,
    required int pageSize,
  }) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/public/products',
        queryParameters: {'page': page, 'page_size': pageSize},
      );
      return _parsePage(response.data, requestedPage: page, pageSize: pageSize);
    } on AppException {
      rethrow;
    } on DioException catch (error) {
      final type = error.type;
      if (type == DioExceptionType.connectionTimeout ||
          type == DioExceptionType.sendTimeout ||
          type == DioExceptionType.receiveTimeout) {
        throw const TimeoutException();
      }
      final message = _responseMessage(error.response?.data);
      if (message != null) throw ServerException(message);
      throw const NetworkException();
    } on FormatException {
      throw const ServerException('商品資料格式錯誤，請稍後再試');
    } on TypeError {
      throw const ServerException('商品資料格式錯誤，請稍後再試');
    }
  }

  HomeProductPage _parsePage(
    Map<String, dynamic>? body, {
    required int requestedPage,
    required int pageSize,
  }) {
    if (body == null) {
      throw const FormatException('Products response body is missing.');
    }
    final code = int.tryParse(body['code']?.toString() ?? '0') ?? -1;
    if (code != 0) {
      throw ServerException(_responseMessage(body) ?? '商品載入失敗，請稍後再試');
    }
    final rawData = body['data'];
    if (rawData is! Map) {
      throw const FormatException('Products response data is missing.');
    }
    final data = Map<String, dynamic>.from(rawData);
    final rawRecords = data['records'] ?? data['items'];
    if (rawRecords is! List) {
      throw const FormatException('Products records must be a list.');
    }

    final products = <HomeProduct>[];
    final seenIds = <String>{};
    for (final raw in rawRecords) {
      if (raw is! Map) continue;
      try {
        final product = HomeProduct.fromJson(Map<String, dynamic>.from(raw));
        if (seenIds.add(product.id)) products.add(product);
      } on FormatException {
        // A malformed record must not make valid cards on the same page
        // unusable. Page-level schema failures still surface explicitly.
      }
    }

    final responsePage =
        int.tryParse(data['page']?.toString() ?? '') ?? requestedPage;
    // 分頁四欄一律 camelCase（`records`／`total`／`page`／`pageSize`，10 檔 §2）。
    final responsePageSize =
        int.tryParse(data['pageSize']?.toString() ?? '') ?? pageSize;
    final total = int.tryParse(data['total']?.toString() ?? '');
    final hasMore = total == null
        ? rawRecords.length >= pageSize
        : responsePage * responsePageSize < total;

    return HomeProductPage(
      products: products,
      page: responsePage,
      pageSize: responsePageSize,
      total: total,
      hasMore: hasMore,
    );
  }

  String? _responseMessage(dynamic body) {
    if (body is! Map) return null;
    for (final value in [body['message'], body['error'], body['detail']]) {
      final message = value?.toString().trim() ?? '';
      if (message.isNotEmpty && message != 'null') return message;
    }
    return null;
  }
}
