// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_card_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(storeCardLoader)
final storeCardLoaderProvider = StoreCardLoaderProvider._();

final class StoreCardLoaderProvider
    extends
        $FunctionalProvider<StoreCardLoader, StoreCardLoader, StoreCardLoader>
    with $Provider<StoreCardLoader> {
  StoreCardLoaderProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storeCardLoaderProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storeCardLoaderHash();

  @$internal
  @override
  $ProviderElement<StoreCardLoader> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  StoreCardLoader create(Ref ref) {
    return storeCardLoader(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StoreCardLoader value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StoreCardLoader>(value),
    );
  }
}

String _$storeCardLoaderHash() => r'60119d8751143dccc9dd6c7ad9cd50df885185ee';

/// 一家店的卡片，同一個畫面同時要的店家共用一次批次讀取。
///
/// 沒有卡片是資料 null；讀取失敗是 AsyncError（不快取成 null），`ref.invalidate` 或下一次重新建立時重讀。

@ProviderFor(storeCard)
final storeCardProvider = StoreCardFamily._();

/// 一家店的卡片，同一個畫面同時要的店家共用一次批次讀取。
///
/// 沒有卡片是資料 null；讀取失敗是 AsyncError（不快取成 null），`ref.invalidate` 或下一次重新建立時重讀。

final class StoreCardProvider
    extends
        $FunctionalProvider<
          AsyncValue<StoreCard?>,
          StoreCard?,
          FutureOr<StoreCard?>
        >
    with $FutureModifier<StoreCard?>, $FutureProvider<StoreCard?> {
  /// 一家店的卡片，同一個畫面同時要的店家共用一次批次讀取。
  ///
  /// 沒有卡片是資料 null；讀取失敗是 AsyncError（不快取成 null），`ref.invalidate` 或下一次重新建立時重讀。
  StoreCardProvider._({
    required StoreCardFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'storeCardProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$storeCardHash();

  @override
  String toString() {
    return r'storeCardProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<StoreCard?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<StoreCard?> create(Ref ref) {
    final argument = this.argument as String;
    return storeCard(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is StoreCardProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storeCardHash() => r'644f8c60411e9c3e01dcba80440db93e08bda31b';

/// 一家店的卡片，同一個畫面同時要的店家共用一次批次讀取。
///
/// 沒有卡片是資料 null；讀取失敗是 AsyncError（不快取成 null），`ref.invalidate` 或下一次重新建立時重讀。

final class StoreCardFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<StoreCard?>, String> {
  StoreCardFamily._()
    : super(
        retry: null,
        name: r'storeCardProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 一家店的卡片，同一個畫面同時要的店家共用一次批次讀取。
  ///
  /// 沒有卡片是資料 null；讀取失敗是 AsyncError（不快取成 null），`ref.invalidate` 或下一次重新建立時重讀。

  StoreCardProvider call(String storeId) =>
      StoreCardProvider._(argument: storeId, from: this);

  @override
  String toString() => r'storeCardProvider';
}
