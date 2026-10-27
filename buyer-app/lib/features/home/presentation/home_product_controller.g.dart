// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'home_product_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(HomeProductController)
final homeProductControllerProvider = HomeProductControllerProvider._();

final class HomeProductControllerProvider
    extends $NotifierProvider<HomeProductController, HomeProductFeedState> {
  HomeProductControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'homeProductControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$homeProductControllerHash();

  @$internal
  @override
  HomeProductController create() => HomeProductController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(HomeProductFeedState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<HomeProductFeedState>(value),
    );
  }
}

String _$homeProductControllerHash() =>
    r'ce9e8f0ff3ecf60e73cc2dfd5da153566b157e7a';

abstract class _$HomeProductController extends $Notifier<HomeProductFeedState> {
  HomeProductFeedState build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<HomeProductFeedState, HomeProductFeedState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<HomeProductFeedState, HomeProductFeedState>,
              HomeProductFeedState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
