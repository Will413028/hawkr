// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'order_detail_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(OrderDetailController)
final orderDetailControllerProvider = OrderDetailControllerFamily._();

final class OrderDetailControllerProvider
    extends $AsyncNotifierProvider<OrderDetailController, Order> {
  OrderDetailControllerProvider._({
    required OrderDetailControllerFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'orderDetailControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$orderDetailControllerHash();

  @override
  String toString() {
    return r'orderDetailControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  OrderDetailController create() => OrderDetailController();

  @override
  bool operator ==(Object other) {
    return other is OrderDetailControllerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$orderDetailControllerHash() =>
    r'392eb323f78ed97fb1b7dcb88a85a0f21ce02d3e';

final class OrderDetailControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          OrderDetailController,
          AsyncValue<Order>,
          Order,
          FutureOr<Order>,
          String
        > {
  OrderDetailControllerFamily._()
    : super(
        retry: null,
        name: r'orderDetailControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  OrderDetailControllerProvider call(String orderSn) =>
      OrderDetailControllerProvider._(argument: orderSn, from: this);

  @override
  String toString() => r'orderDetailControllerProvider';
}

abstract class _$OrderDetailController extends $AsyncNotifier<Order> {
  late final _$args = ref.$arg as String;
  String get orderSn => _$args;

  FutureOr<Order> build(String orderSn);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<Order>, Order>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<Order>, Order>,
              AsyncValue<Order>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
