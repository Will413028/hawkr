// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'order_status_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Riverpod bridge — 讓 UI 用 `ref.watch(orderStatusStreamProvider)` 訂閱訂單事件，
/// 享受 AsyncValue / `.when()` 的便利。

@ProviderFor(orderStatusStream)
final orderStatusStreamProvider = OrderStatusStreamProvider._();

/// Riverpod bridge — 讓 UI 用 `ref.watch(orderStatusStreamProvider)` 訂閱訂單事件，
/// 享受 AsyncValue / `.when()` 的便利。

final class OrderStatusStreamProvider
    extends
        $FunctionalProvider<
          AsyncValue<OrderStatusEvent>,
          OrderStatusEvent,
          Stream<OrderStatusEvent>
        >
    with $FutureModifier<OrderStatusEvent>, $StreamProvider<OrderStatusEvent> {
  /// Riverpod bridge — 讓 UI 用 `ref.watch(orderStatusStreamProvider)` 訂閱訂單事件，
  /// 享受 AsyncValue / `.when()` 的便利。
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

String _$orderStatusStreamHash() => r'fc5223633162b228d58a9dd6a99925b9b0314269';
