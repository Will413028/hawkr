import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/category_marketplace_model.dart';
import '../data/category_repository.dart';

part 'category_marketplace_controller.g.dart';

Duration? _disableAutomaticRetry(int _, Object __) => null;

/// Session-scoped UI state for the category landing recommendation rail.
///
/// Retry is user-driven so a failed section never starts hidden repeat
/// requests and the request-count contract stays deterministic.
@Riverpod(keepAlive: true, retry: _disableAutomaticRetry)
class CategoryMarketplaceController extends _$CategoryMarketplaceController {
  CategoryRepository get _repository => ref.read(categoryRepositoryProvider);

  @override
  Future<List<CategoryMarketplaceProduct>> build() {
    return ref
        .watch(categoryRepositoryProvider)
        .fetchMarketplaceRecommendations();
  }

  /// Explicit network refresh. The repository cache is bypassed and replaced
  /// only when the aggregate response parses successfully.
  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => _repository.fetchMarketplaceRecommendations(forceRefresh: true),
    );
  }

  /// Explicitly invalidate the session cache without eagerly fetching.
  void invalidateCache() {
    _repository.invalidateMarketplaceRecommendations();
  }
}
