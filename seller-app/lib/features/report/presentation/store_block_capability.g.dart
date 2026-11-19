// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_block_capability.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 店家有封鎖買家的能力（`SELLER_CAPABILITY_CONTENT_CONTRACT`）。店家還沒讀到時不下結論（true），伺服器是最後一道。

@ProviderFor(canBlockBuyers)
final canBlockBuyersProvider = CanBlockBuyersProvider._();

/// 店家有封鎖買家的能力（`SELLER_CAPABILITY_CONTENT_CONTRACT`）。店家還沒讀到時不下結論（true），伺服器是最後一道。

final class CanBlockBuyersProvider extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// 店家有封鎖買家的能力（`SELLER_CAPABILITY_CONTENT_CONTRACT`）。店家還沒讀到時不下結論（true），伺服器是最後一道。
  CanBlockBuyersProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'canBlockBuyersProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$canBlockBuyersHash();

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    return canBlockBuyers(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$canBlockBuyersHash() => r'639b42ac6aa8044a21f2f4e7ee3a0d2972d363a4';
