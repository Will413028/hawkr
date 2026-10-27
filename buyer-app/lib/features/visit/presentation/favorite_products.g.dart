// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'favorite_products.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 單一商品的收藏狀態，全 App 共用；`null` 表示還沒讀到。
///
/// 由 [FavoriteProducts.check] 的批次讀取填入，使用者的收藏／取消收藏直接寫入。
/// 讀取進行中使用者改了這個商品，較舊的讀取結果不覆蓋本地狀態。
/// 換帳號時重置。

@ProviderFor(FavoriteProduct)
final favoriteProductProvider = FavoriteProductFamily._();

/// 單一商品的收藏狀態，全 App 共用；`null` 表示還沒讀到。
///
/// 由 [FavoriteProducts.check] 的批次讀取填入，使用者的收藏／取消收藏直接寫入。
/// 讀取進行中使用者改了這個商品，較舊的讀取結果不覆蓋本地狀態。
/// 換帳號時重置。
final class FavoriteProductProvider
    extends $NotifierProvider<FavoriteProduct, bool?> {
  /// 單一商品的收藏狀態，全 App 共用；`null` 表示還沒讀到。
  ///
  /// 由 [FavoriteProducts.check] 的批次讀取填入，使用者的收藏／取消收藏直接寫入。
  /// 讀取進行中使用者改了這個商品，較舊的讀取結果不覆蓋本地狀態。
  /// 換帳號時重置。
  FavoriteProductProvider._({
    required FavoriteProductFamily super.from,
    required int super.argument,
  }) : super(
         retry: null,
         name: r'favoriteProductProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$favoriteProductHash();

  @override
  String toString() {
    return r'favoriteProductProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  FavoriteProduct create() => FavoriteProduct();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is FavoriteProductProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$favoriteProductHash() => r'7eaa81bbee968aed446250372a9f88dd5517932f';

/// 單一商品的收藏狀態，全 App 共用；`null` 表示還沒讀到。
///
/// 由 [FavoriteProducts.check] 的批次讀取填入，使用者的收藏／取消收藏直接寫入。
/// 讀取進行中使用者改了這個商品，較舊的讀取結果不覆蓋本地狀態。
/// 換帳號時重置。

final class FavoriteProductFamily extends $Family
    with $ClassFamilyOverride<FavoriteProduct, bool?, bool?, bool?, int> {
  FavoriteProductFamily._()
    : super(
        retry: null,
        name: r'favoriteProductProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// 單一商品的收藏狀態，全 App 共用；`null` 表示還沒讀到。
  ///
  /// 由 [FavoriteProducts.check] 的批次讀取填入，使用者的收藏／取消收藏直接寫入。
  /// 讀取進行中使用者改了這個商品，較舊的讀取結果不覆蓋本地狀態。
  /// 換帳號時重置。

  FavoriteProductProvider call(int productId) =>
      FavoriteProductProvider._(argument: productId, from: this);

  @override
  String toString() => r'favoriteProductProvider';
}

/// 單一商品的收藏狀態，全 App 共用；`null` 表示還沒讀到。
///
/// 由 [FavoriteProducts.check] 的批次讀取填入，使用者的收藏／取消收藏直接寫入。
/// 讀取進行中使用者改了這個商品，較舊的讀取結果不覆蓋本地狀態。
/// 換帳號時重置。

abstract class _$FavoriteProduct extends $Notifier<bool?> {
  late final _$args = ref.$arg as int;
  int get productId => _$args;

  bool? build(int productId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<bool?, bool?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<bool?, bool?>,
              bool?,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}

/// 收藏的讀寫入口：批次讀取畫面上的商品、切換收藏，結果寫進各商品的 [favoriteProductProvider]。

@ProviderFor(favoriteProducts)
final favoriteProductsProvider = FavoriteProductsProvider._();

/// 收藏的讀寫入口：批次讀取畫面上的商品、切換收藏，結果寫進各商品的 [favoriteProductProvider]。

final class FavoriteProductsProvider
    extends
        $FunctionalProvider<
          FavoriteProducts,
          FavoriteProducts,
          FavoriteProducts
        >
    with $Provider<FavoriteProducts> {
  /// 收藏的讀寫入口：批次讀取畫面上的商品、切換收藏，結果寫進各商品的 [favoriteProductProvider]。
  FavoriteProductsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'favoriteProductsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$favoriteProductsHash();

  @$internal
  @override
  $ProviderElement<FavoriteProducts> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  FavoriteProducts create(Ref ref) {
    return favoriteProducts(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(FavoriteProducts value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<FavoriteProducts>(value),
    );
  }
}

String _$favoriteProductsHash() => r'21a722a85baa7dec1c84ad383cfa398a93b9c727';
