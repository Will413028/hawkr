// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'private_document_uploader.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 私密文件（認證證件、申訴佐證）走 legacy media 的 `secure_upload`，身分由傳輸層的 Bearer 決定；
/// 回傳的 staged token 由送出該文件所屬的請求帶上。沒有 staged token 的回應是錯誤。

@ProviderFor(privateDocumentUploader)
final privateDocumentUploaderProvider = PrivateDocumentUploaderProvider._();

/// 私密文件（認證證件、申訴佐證）走 legacy media 的 `secure_upload`，身分由傳輸層的 Bearer 決定；
/// 回傳的 staged token 由送出該文件所屬的請求帶上。沒有 staged token 的回應是錯誤。

final class PrivateDocumentUploaderProvider
    extends
        $FunctionalProvider<
          PrivateDocumentUploader,
          PrivateDocumentUploader,
          PrivateDocumentUploader
        >
    with $Provider<PrivateDocumentUploader> {
  /// 私密文件（認證證件、申訴佐證）走 legacy media 的 `secure_upload`，身分由傳輸層的 Bearer 決定；
  /// 回傳的 staged token 由送出該文件所屬的請求帶上。沒有 staged token 的回應是錯誤。
  PrivateDocumentUploaderProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'privateDocumentUploaderProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$privateDocumentUploaderHash();

  @$internal
  @override
  $ProviderElement<PrivateDocumentUploader> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  PrivateDocumentUploader create(Ref ref) {
    return privateDocumentUploader(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PrivateDocumentUploader value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PrivateDocumentUploader>(value),
    );
  }
}

String _$privateDocumentUploaderHash() =>
    r'5dee47c2a78bdcde8766352c22d49bbe5c848abd';
