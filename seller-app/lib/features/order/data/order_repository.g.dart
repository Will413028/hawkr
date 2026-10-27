// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'order_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(orderRepository)
final orderRepositoryProvider = OrderRepositoryProvider._();

final class OrderRepositoryProvider
    extends
        $FunctionalProvider<OrderRepository, OrderRepository, OrderRepository>
    with $Provider<OrderRepository> {
  OrderRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'orderRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$orderRepositoryHash();

  @$internal
  @override
  $ProviderElement<OrderRepository> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  OrderRepository create(Ref ref) {
    return orderRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(OrderRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<OrderRepository>(value),
    );
  }
}

String _$orderRepositoryHash() => r'3b241f983b53f8c0947b0fd8b88c2edbf34f61c4';

/// 依 orderNo 抓 OrderSummary（keepAlive 快取,用於售後卡片內嵌訂單資訊）

@ProviderFor(orderByNo)
final orderByNoProvider = OrderByNoFamily._();

/// 依 orderNo 抓 OrderSummary（keepAlive 快取,用於售後卡片內嵌訂單資訊）

final class OrderByNoProvider
    extends
        $FunctionalProvider<
          AsyncValue<OrderSummary?>,
          OrderSummary?,
          FutureOr<OrderSummary?>
        >
    with $FutureModifier<OrderSummary?>, $FutureProvider<OrderSummary?> {
  /// 依 orderNo 抓 OrderSummary（keepAlive 快取,用於售後卡片內嵌訂單資訊）
  OrderByNoProvider._({
    required OrderByNoFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'orderByNoProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$orderByNoHash();

  @override
  String toString() {
    return r'orderByNoProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<OrderSummary?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<OrderSummary?> create(Ref ref) {
    final argument = this.argument as String;
    return orderByNo(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is OrderByNoProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$orderByNoHash() => r'6e0aa43f4c2bfcb4f220b8098875287cc3679348';

/// 依 orderNo 抓 OrderSummary（keepAlive 快取,用於售後卡片內嵌訂單資訊）

final class OrderByNoFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<OrderSummary?>, String> {
  OrderByNoFamily._()
    : super(
        retry: null,
        name: r'orderByNoProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// 依 orderNo 抓 OrderSummary（keepAlive 快取,用於售後卡片內嵌訂單資訊）

  OrderByNoProvider call(String orderNo) =>
      OrderByNoProvider._(argument: orderNo, from: this);

  @override
  String toString() => r'orderByNoProvider';
}
