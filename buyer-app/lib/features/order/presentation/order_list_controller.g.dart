// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'order_list_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(OrderListController)
final orderListControllerProvider = OrderListControllerProvider._();

final class OrderListControllerProvider
    extends $AsyncNotifierProvider<OrderListController, List<Order>> {
  OrderListControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'orderListControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$orderListControllerHash();

  @$internal
  @override
  OrderListController create() => OrderListController();
}

String _$orderListControllerHash() =>
    r'f631a7005273ad904a1b0a6efb13c9718362714d';

abstract class _$OrderListController extends $AsyncNotifier<List<Order>> {
  FutureOr<List<Order>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<Order>>, List<Order>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<List<Order>>, List<Order>>,
              AsyncValue<List<Order>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
