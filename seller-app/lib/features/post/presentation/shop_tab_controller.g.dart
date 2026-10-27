// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shop_tab_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(ShopTabController)
final shopTabControllerProvider = ShopTabControllerFamily._();

final class ShopTabControllerProvider
    extends $NotifierProvider<ShopTabController, ShopTabState> {
  ShopTabControllerProvider._({
    required ShopTabControllerFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'shopTabControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$shopTabControllerHash();

  @override
  String toString() {
    return r'shopTabControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  ShopTabController create() => ShopTabController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ShopTabState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ShopTabState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ShopTabControllerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$shopTabControllerHash() => r'0ab4687f7469bea7adb8de66906e76acc3c11b7c';

final class ShopTabControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          ShopTabController,
          ShopTabState,
          ShopTabState,
          ShopTabState,
          String
        > {
  ShopTabControllerFamily._()
    : super(
        retry: null,
        name: r'shopTabControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  ShopTabControllerProvider call(String sellerId) =>
      ShopTabControllerProvider._(argument: sellerId, from: this);

  @override
  String toString() => r'shopTabControllerProvider';
}

abstract class _$ShopTabController extends $Notifier<ShopTabState> {
  late final _$args = ref.$arg as String;
  String get sellerId => _$args;

  ShopTabState build(String sellerId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<ShopTabState, ShopTabState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ShopTabState, ShopTabState>,
              ShopTabState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
