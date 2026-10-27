// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'after_sale_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(afterSaleRepository)
final afterSaleRepositoryProvider = AfterSaleRepositoryProvider._();

final class AfterSaleRepositoryProvider
    extends
        $FunctionalProvider<
          AfterSaleRepository,
          AfterSaleRepository,
          AfterSaleRepository
        >
    with $Provider<AfterSaleRepository> {
  AfterSaleRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'afterSaleRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$afterSaleRepositoryHash();

  @$internal
  @override
  $ProviderElement<AfterSaleRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AfterSaleRepository create(Ref ref) {
    return afterSaleRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AfterSaleRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AfterSaleRepository>(value),
    );
  }
}

String _$afterSaleRepositoryHash() =>
    r'58dbd22650f001789c68270caf92f2b3541fa0ed';
