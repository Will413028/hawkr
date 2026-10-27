// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_quote_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(sellerQuoteRepository)
final sellerQuoteRepositoryProvider = SellerQuoteRepositoryProvider._();

final class SellerQuoteRepositoryProvider
    extends
        $FunctionalProvider<
          SellerQuoteRepository,
          SellerQuoteRepository,
          SellerQuoteRepository
        >
    with $Provider<SellerQuoteRepository> {
  SellerQuoteRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerQuoteRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerQuoteRepositoryHash();

  @$internal
  @override
  $ProviderElement<SellerQuoteRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SellerQuoteRepository create(Ref ref) {
    return sellerQuoteRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SellerQuoteRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SellerQuoteRepository>(value),
    );
  }
}

String _$sellerQuoteRepositoryHash() =>
    r'cf825ed5e12a81bed7119a1511e058453d2945e6';
