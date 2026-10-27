// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'product_listing_chat_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(productListingChatRepository)
final productListingChatRepositoryProvider =
    ProductListingChatRepositoryProvider._();

final class ProductListingChatRepositoryProvider
    extends
        $FunctionalProvider<
          ProductListingChatRepository,
          ProductListingChatRepository,
          ProductListingChatRepository
        >
    with $Provider<ProductListingChatRepository> {
  ProductListingChatRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'productListingChatRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$productListingChatRepositoryHash();

  @$internal
  @override
  $ProviderElement<ProductListingChatRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ProductListingChatRepository create(Ref ref) {
    return productListingChatRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ProductListingChatRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ProductListingChatRepository>(value),
    );
  }
}

String _$productListingChatRepositoryHash() =>
    r'89b49df22aba8926e66d8fe0a800e34630fab2f0';
