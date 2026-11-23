// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'review_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(AddReviewController)
final addReviewControllerProvider = AddReviewControllerProvider._();

final class AddReviewControllerProvider
    extends $NotifierProvider<AddReviewController, AsyncValue<void>> {
  AddReviewControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'addReviewControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$addReviewControllerHash();

  @$internal
  @override
  AddReviewController create() => AddReviewController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<void> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<void>>(value),
    );
  }
}

String _$addReviewControllerHash() =>
    r'7c494dbea42d0e78505d5da15d0cd33033e2a5e9';

abstract class _$AddReviewController extends $Notifier<AsyncValue<void>> {
  AsyncValue<void> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<void>, AsyncValue<void>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<void>, AsyncValue<void>>,
              AsyncValue<void>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
