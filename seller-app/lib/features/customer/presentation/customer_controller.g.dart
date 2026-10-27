// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'customer_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(CustomerFilterNotifier)
final customerFilterProvider = CustomerFilterNotifierProvider._();

final class CustomerFilterNotifierProvider
    extends $NotifierProvider<CustomerFilterNotifier, CustomerFilter> {
  CustomerFilterNotifierProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'customerFilterProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$customerFilterNotifierHash();

  @$internal
  @override
  CustomerFilterNotifier create() => CustomerFilterNotifier();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CustomerFilter value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CustomerFilter>(value),
    );
  }
}

String _$customerFilterNotifierHash() =>
    r'94becf719d4470d9dc89211441d30afc17242ae7';

abstract class _$CustomerFilterNotifier extends $Notifier<CustomerFilter> {
  CustomerFilter build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<CustomerFilter, CustomerFilter>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<CustomerFilter, CustomerFilter>,
              CustomerFilter,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(customerListNotifier)
final customerListProvider = CustomerListNotifierProvider._();

final class CustomerListNotifierProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Customer>>,
          List<Customer>,
          FutureOr<List<Customer>>
        >
    with $FutureModifier<List<Customer>>, $FutureProvider<List<Customer>> {
  CustomerListNotifierProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'customerListProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$customerListNotifierHash();

  @$internal
  @override
  $FutureProviderElement<List<Customer>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<Customer>> create(Ref ref) {
    return customerListNotifier(ref);
  }
}

String _$customerListNotifierHash() =>
    r'47eb4487042189df9415b3f51e679f17bfead52b';

@ProviderFor(customerOrders)
final customerOrdersProvider = CustomerOrdersFamily._();

final class CustomerOrdersProvider
    extends
        $FunctionalProvider<
          AsyncValue<
            ({List<OrderRecord> orders, List<SpendingRecord> spending})
          >,
          ({List<OrderRecord> orders, List<SpendingRecord> spending}),
          FutureOr<({List<OrderRecord> orders, List<SpendingRecord> spending})>
        >
    with
        $FutureModifier<
          ({List<OrderRecord> orders, List<SpendingRecord> spending})
        >,
        $FutureProvider<
          ({List<OrderRecord> orders, List<SpendingRecord> spending})
        > {
  CustomerOrdersProvider._({
    required CustomerOrdersFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'customerOrdersProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$customerOrdersHash();

  @override
  String toString() {
    return r'customerOrdersProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<
    ({List<OrderRecord> orders, List<SpendingRecord> spending})
  >
  $createElement($ProviderPointer pointer) => $FutureProviderElement(pointer);

  @override
  FutureOr<({List<OrderRecord> orders, List<SpendingRecord> spending})> create(
    Ref ref,
  ) {
    final argument = this.argument as String;
    return customerOrders(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is CustomerOrdersProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$customerOrdersHash() => r'4e949b1c3d9563359bff323cd9483558545a422b';

final class CustomerOrdersFamily extends $Family
    with
        $FunctionalFamilyOverride<
          FutureOr<({List<OrderRecord> orders, List<SpendingRecord> spending})>,
          String
        > {
  CustomerOrdersFamily._()
    : super(
        retry: null,
        name: r'customerOrdersProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  CustomerOrdersProvider call(String customerId) =>
      CustomerOrdersProvider._(argument: customerId, from: this);

  @override
  String toString() => r'customerOrdersProvider';
}

@ProviderFor(customerCoupons)
final customerCouponsProvider = CustomerCouponsFamily._();

final class CustomerCouponsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Coupon>>,
          List<Coupon>,
          FutureOr<List<Coupon>>
        >
    with $FutureModifier<List<Coupon>>, $FutureProvider<List<Coupon>> {
  CustomerCouponsProvider._({
    required CustomerCouponsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'customerCouponsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$customerCouponsHash();

  @override
  String toString() {
    return r'customerCouponsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<List<Coupon>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<Coupon>> create(Ref ref) {
    final argument = this.argument as String;
    return customerCoupons(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is CustomerCouponsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$customerCouponsHash() => r'62b4175f5820c20dc3902beae68b5eac294f56c7';

final class CustomerCouponsFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<Coupon>>, String> {
  CustomerCouponsFamily._()
    : super(
        retry: null,
        name: r'customerCouponsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  CustomerCouponsProvider call(String customerId) =>
      CustomerCouponsProvider._(argument: customerId, from: this);

  @override
  String toString() => r'customerCouponsProvider';
}

@ProviderFor(customerTags)
final customerTagsProvider = CustomerTagsFamily._();

final class CustomerTagsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<CustomerTag>>,
          List<CustomerTag>,
          FutureOr<List<CustomerTag>>
        >
    with
        $FutureModifier<List<CustomerTag>>,
        $FutureProvider<List<CustomerTag>> {
  CustomerTagsProvider._({
    required CustomerTagsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'customerTagsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$customerTagsHash();

  @override
  String toString() {
    return r'customerTagsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<List<CustomerTag>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<CustomerTag>> create(Ref ref) {
    final argument = this.argument as String;
    return customerTags(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is CustomerTagsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$customerTagsHash() => r'b86cc038bba580031be69368103aba96517e7185';

final class CustomerTagsFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<CustomerTag>>, String> {
  CustomerTagsFamily._()
    : super(
        retry: null,
        name: r'customerTagsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  CustomerTagsProvider call(String buyerId) =>
      CustomerTagsProvider._(argument: buyerId, from: this);

  @override
  String toString() => r'customerTagsProvider';
}

@ProviderFor(sellerTags)
final sellerTagsProvider = SellerTagsProvider._();

final class SellerTagsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<CustomerTag>>,
          List<CustomerTag>,
          FutureOr<List<CustomerTag>>
        >
    with
        $FutureModifier<List<CustomerTag>>,
        $FutureProvider<List<CustomerTag>> {
  SellerTagsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerTagsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerTagsHash();

  @$internal
  @override
  $FutureProviderElement<List<CustomerTag>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<CustomerTag>> create(Ref ref) {
    return sellerTags(ref);
  }
}

String _$sellerTagsHash() => r'6983b72b11b70c077ca71c5aed00e539cbd739b1';
