// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'address_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(AddressController)
final addressControllerProvider = AddressControllerProvider._();

final class AddressControllerProvider
    extends $AsyncNotifierProvider<AddressController, List<Address>> {
  AddressControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'addressControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$addressControllerHash();

  @$internal
  @override
  AddressController create() => AddressController();
}

String _$addressControllerHash() => r'e42ce845169340c7672df686f62359f26be777ff';

abstract class _$AddressController extends $AsyncNotifier<List<Address>> {
  FutureOr<List<Address>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<Address>>, List<Address>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<List<Address>>, List<Address>>,
              AsyncValue<List<Address>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
