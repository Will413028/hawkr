// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'order_status_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Riverpod bridge — `ref.watch(orderStatusStreamProvider)` 取得事件 Stream。

@ProviderFor(orderStatusStream)
final orderStatusStreamProvider = OrderStatusStreamProvider._();

/// Riverpod bridge — `ref.watch(orderStatusStreamProvider)` 取得事件 Stream。

final class OrderStatusStreamProvider
    extends
        $FunctionalProvider<
          AsyncValue<OrderStatusEvent>,
          OrderStatusEvent,
          Stream<OrderStatusEvent>
        >
    with $FutureModifier<OrderStatusEvent>, $StreamProvider<OrderStatusEvent> {
  /// Riverpod bridge — `ref.watch(orderStatusStreamProvider)` 取得事件 Stream。
  OrderStatusStreamProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'orderStatusStreamProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$orderStatusStreamHash();

  @$internal
  @override
  $StreamProviderElement<OrderStatusEvent> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<OrderStatusEvent> create(Ref ref) {
    return orderStatusStream(ref);
  }
}

String _$orderStatusStreamHash() => r'92d21a87c325325ac83c52ab708cb1e8453dfffa';
