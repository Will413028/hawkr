import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/session_provider.dart';
import '../data/visit_repository.dart';

part 'favorite_products.g.dart';

/// 單一商品的收藏狀態，全 App 共用；`null` 表示還沒讀到。
///
/// 由 [FavoriteProducts.check] 的批次讀取填入，使用者的收藏／取消收藏直接寫入。
/// 讀取進行中使用者改了這個商品，較舊的讀取結果不覆蓋本地狀態。
/// 換帳號時重置。
@Riverpod(keepAlive: true)
class FavoriteProduct extends _$FavoriteProduct {
  /// 這個商品目前有效的那次讀取；使用者改動後作廢。
  Object? _pendingRead;

  @override
  bool? build(int productId) {
    ref.watch(sessionProvider.select((session) => session.value?.userId));
    _pendingRead = null;
    return null;
  }

  Object _beginRead() => _pendingRead = Object();

  void _completeRead(Object read, bool favorited) {
    if (!identical(_pendingRead, read)) return;
    _pendingRead = null;
    state = favorited;
  }

  /// 本地的改動（樂觀更新、失敗還原、其他畫面回傳的結果）。
  void set(bool favorited) {
    _pendingRead = null;
    state = favorited;
  }
}

/// 收藏的讀寫入口：批次讀取畫面上的商品、切換收藏，結果寫進各商品的 [favoriteProductProvider]。
@Riverpod(keepAlive: true)
FavoriteProducts favoriteProducts(Ref ref) => FavoriteProducts(ref);

class FavoriteProducts {
  FavoriteProducts(this._ref);
  final Ref _ref;

  FavoriteProduct _product(int id) =>
      _ref.read(favoriteProductProvider(id).notifier);

  /// 以一次批次檢查讀取 [productIds] 的收藏狀態；未登入時不讀。
  Future<void> check(Iterable<int> productIds) async {
    if (!hasUsableSession(_ref.read(sessionProvider).value)) return;
    final ids = productIds.toSet();
    if (ids.isEmpty) return;
    final reads = {for (final id in ids) id: _product(id)._beginRead()};
    final favorited = await _ref
        .read(visitRepositoryProvider)
        .batchCheckFavorites(ids);
    for (final MapEntry(key: id, value: read) in reads.entries) {
      _product(id)._completeRead(read, favorited.contains(id));
    }
  }

  /// 以 [current] 為準切換收藏：先樂觀更新，寫入失敗時還原並丟出錯誤。
  Future<void> toggle(int productId, {required bool current}) async {
    final product = _product(productId);
    product.set(!current);
    try {
      final repository = _ref.read(visitRepositoryProvider);
      if (current) {
        await repository.removeFavorite(productId);
      } else {
        await repository.addFavorite(productId);
      }
    } catch (_) {
      product.set(current);
      rethrow;
    }
  }
}
