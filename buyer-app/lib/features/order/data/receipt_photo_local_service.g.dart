// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'receipt_photo_local_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(receiptPhotoLocalService)
final receiptPhotoLocalServiceProvider = ReceiptPhotoLocalServiceProvider._();

final class ReceiptPhotoLocalServiceProvider
    extends
        $FunctionalProvider<
          ReceiptPhotoLocalService,
          ReceiptPhotoLocalService,
          ReceiptPhotoLocalService
        >
    with $Provider<ReceiptPhotoLocalService> {
  ReceiptPhotoLocalServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'receiptPhotoLocalServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$receiptPhotoLocalServiceHash();

  @$internal
  @override
  $ProviderElement<ReceiptPhotoLocalService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ReceiptPhotoLocalService create(Ref ref) {
    return receiptPhotoLocalService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ReceiptPhotoLocalService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ReceiptPhotoLocalService>(value),
    );
  }
}

String _$receiptPhotoLocalServiceHash() =>
    r'db05190b76ba927f57983749bfaa7e6eea818a97';
