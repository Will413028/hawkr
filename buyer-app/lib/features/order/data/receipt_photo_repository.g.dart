// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'receipt_photo_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(receiptPhotoRepository)
final receiptPhotoRepositoryProvider = ReceiptPhotoRepositoryProvider._();

final class ReceiptPhotoRepositoryProvider
    extends
        $FunctionalProvider<
          ReceiptPhotoRepository,
          ReceiptPhotoRepository,
          ReceiptPhotoRepository
        >
    with $Provider<ReceiptPhotoRepository> {
  ReceiptPhotoRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'receiptPhotoRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$receiptPhotoRepositoryHash();

  @$internal
  @override
  $ProviderElement<ReceiptPhotoRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ReceiptPhotoRepository create(Ref ref) {
    return receiptPhotoRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ReceiptPhotoRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ReceiptPhotoRepository>(value),
    );
  }
}

String _$receiptPhotoRepositoryHash() =>
    r'ef03f9f210aed718efe1cbbc88b4b362abdf4edb';
