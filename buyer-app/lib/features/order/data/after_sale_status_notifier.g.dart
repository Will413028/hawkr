// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'after_sale_status_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Riverpod bridge — `ref.watch(afterSaleStatusStreamProvider)` 取得事件 Stream。

@ProviderFor(afterSaleStatusStream)
final afterSaleStatusStreamProvider = AfterSaleStatusStreamProvider._();

/// Riverpod bridge — `ref.watch(afterSaleStatusStreamProvider)` 取得事件 Stream。

final class AfterSaleStatusStreamProvider
    extends
        $FunctionalProvider<
          AsyncValue<AfterSaleStatusEvent>,
          AfterSaleStatusEvent,
          Stream<AfterSaleStatusEvent>
        >
    with
        $FutureModifier<AfterSaleStatusEvent>,
        $StreamProvider<AfterSaleStatusEvent> {
  /// Riverpod bridge — `ref.watch(afterSaleStatusStreamProvider)` 取得事件 Stream。
  AfterSaleStatusStreamProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'afterSaleStatusStreamProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$afterSaleStatusStreamHash();

  @$internal
  @override
  $StreamProviderElement<AfterSaleStatusEvent> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<AfterSaleStatusEvent> create(Ref ref) {
    return afterSaleStatusStream(ref);
  }
}

String _$afterSaleStatusStreamHash() =>
    r'5441ffaebcd263818d9d40afba6648bfb9179c3e';
