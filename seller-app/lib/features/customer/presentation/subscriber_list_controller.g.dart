// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'subscriber_list_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(SubscriberListController)
final subscriberListControllerProvider = SubscriberListControllerProvider._();

final class SubscriberListControllerProvider
    extends
        $AsyncNotifierProvider<
          SubscriberListController,
          SubscriberListViewData
        > {
  SubscriberListControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'subscriberListControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$subscriberListControllerHash();

  @$internal
  @override
  SubscriberListController create() => SubscriberListController();
}

String _$subscriberListControllerHash() =>
    r'ede669bd792fc4680f5c2a74027dec1e61107d32';

abstract class _$SubscriberListController
    extends $AsyncNotifier<SubscriberListViewData> {
  FutureOr<SubscriberListViewData> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<AsyncValue<SubscriberListViewData>, SubscriberListViewData>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<SubscriberListViewData>,
                SubscriberListViewData
              >,
              AsyncValue<SubscriberListViewData>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
