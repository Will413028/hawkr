// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'my_store_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(myStoreRepository)
final myStoreRepositoryProvider = MyStoreRepositoryProvider._();

final class MyStoreRepositoryProvider
    extends
        $FunctionalProvider<
          MyStoreRepository,
          MyStoreRepository,
          MyStoreRepository
        >
    with $Provider<MyStoreRepository> {
  MyStoreRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'myStoreRepositoryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$myStoreRepositoryHash();

  @$internal
  @override
  $ProviderElement<MyStoreRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  MyStoreRepository create(Ref ref) {
    return myStoreRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(MyStoreRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<MyStoreRepository>(value),
    );
  }
}

String _$myStoreRepositoryHash() => r'29c6a2ae4562423ddc8923cd0be3f0d0be79f02e';
