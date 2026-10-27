// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_publication_list_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(sellerPublicationList)
final sellerPublicationListProvider = SellerPublicationListProvider._();

final class SellerPublicationListProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<SellerPublicationListItem>>,
          List<SellerPublicationListItem>,
          FutureOr<List<SellerPublicationListItem>>
        >
    with
        $FutureModifier<List<SellerPublicationListItem>>,
        $FutureProvider<List<SellerPublicationListItem>> {
  SellerPublicationListProvider._()
    : super(
        from: null,
        argument: null,
        retry: _noAutoRetry,
        name: r'sellerPublicationListProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerPublicationListHash();

  @$internal
  @override
  $FutureProviderElement<List<SellerPublicationListItem>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<SellerPublicationListItem>> create(Ref ref) {
    return sellerPublicationList(ref);
  }
}

String _$sellerPublicationListHash() =>
    r'2c791b7d8986150a06a565b113685b2df3808c2d';
