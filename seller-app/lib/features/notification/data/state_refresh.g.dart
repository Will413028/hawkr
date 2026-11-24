// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'state_refresh.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(stateRefreshActions)
final stateRefreshActionsProvider = StateRefreshActionsProvider._();

final class StateRefreshActionsProvider
    extends
        $FunctionalProvider<
          StateRefreshActions,
          StateRefreshActions,
          StateRefreshActions
        >
    with $Provider<StateRefreshActions> {
  StateRefreshActionsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'stateRefreshActionsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$stateRefreshActionsHash();

  @$internal
  @override
  $ProviderElement<StateRefreshActions> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  StateRefreshActions create(Ref ref) {
    return stateRefreshActions(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StateRefreshActions value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StateRefreshActions>(value),
    );
  }
}

String _$stateRefreshActionsHash() =>
    r'af4af550b7f1fdaac7cf42e7a467b9e0a7c4791a';
