import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/home_product_model.dart';
import '../data/home_product_repository.dart';

part 'home_product_controller.g.dart';

@immutable
class HomeProductFeedState {
  factory HomeProductFeedState({
    List<HomeProduct> products = const [],
    bool isInitialLoading = false,
    Object? initialError,
  }) {
    return HomeProductFeedState._(
      products: List.unmodifiable(products),
      isInitialLoading: isInitialLoading,
      initialError: initialError,
    );
  }

  const HomeProductFeedState._({
    required this.products,
    required this.isInitialLoading,
    required this.initialError,
  });

  final List<HomeProduct> products;
  final bool isInitialLoading;
  final Object? initialError;

  HomeProductFeedState copyWith({
    List<HomeProduct>? products,
    bool? isInitialLoading,
    Object? initialError,
    bool clearInitialError = false,
  }) {
    return HomeProductFeedState._(
      products: products == null ? this.products : List.unmodifiable(products),
      isInitialLoading: isInitialLoading ?? this.isInitialLoading,
      initialError: clearInitialError
          ? null
          : (initialError ?? this.initialError),
    );
  }
}

@Riverpod(keepAlive: true)
class HomeProductController extends _$HomeProductController {
  /// 首頁只負責提供精選入口；完整商品瀏覽由市集承接。
  static const int previewLimit = 6;
  static const int pageSize = previewLimit;

  HomeProductRepository get _repository =>
      ref.read(homeProductRepositoryProvider);

  @override
  HomeProductFeedState build() => HomeProductFeedState();

  /// Loads page one once for the session-scoped home tab.
  ///
  /// Existing content is kept during an explicit refresh so a transient
  /// network failure cannot blank an already usable home feed.
  Future<void> loadInitial({bool force = false}) async {
    if (state.isInitialLoading) return;
    if (!force && (state.products.isNotEmpty || state.initialError != null)) {
      return;
    }

    state = state.copyWith(isInitialLoading: true, clearInitialError: true);
    try {
      final result = await _repository.fetchPage(page: 1, pageSize: pageSize);
      state = HomeProductFeedState(
        products: result.products.take(previewLimit).toList(growable: false),
      );
    } catch (error) {
      state = state.copyWith(isInitialLoading: false, initialError: error);
    }
  }

  Future<void> retryInitial() => loadInitial(force: true);
}
