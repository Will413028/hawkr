import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../category/data/category_repository.dart';
import '../../category/data/product_item_model.dart';

part 'search_controller.g.dart';

@riverpod
class SearchController extends _$SearchController {
  static const pageSize = 20;

  @override
  FutureOr<void> build() {}

  Future<ProductListResult> fetchSearchPage(String keyword, int page) {
    return ref
        .read(categoryRepositoryProvider)
        .searchProducts(keyword: keyword, page: page, pageSize: pageSize);
  }

  Future<List<String>> getSuggestions(String q) {
    return ref.read(categoryRepositoryProvider).fetchSuggestions(q);
  }
}
