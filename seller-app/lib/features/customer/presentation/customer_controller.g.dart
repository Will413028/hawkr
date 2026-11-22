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
    r'e89be7e72daf293ee652a8a30135bba017ffceaf';

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

@ProviderFor(CustomerList)
final customerListProvider = CustomerListProvider._();

final class CustomerListProvider
    extends $AsyncNotifierProvider<CustomerList, CustomerListState> {
  CustomerListProvider._()
    : super(
        from: null,
        argument: null,
        retry: _noRetry,
        name: r'customerListProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$customerListHash();

  @$internal
  @override
  CustomerList create() => CustomerList();
}

String _$customerListHash() => r'e1d20cfd4a23d1dae8107bc09b31facdf109ef33';

abstract class _$CustomerList extends $AsyncNotifier<CustomerListState> {
  FutureOr<CustomerListState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<AsyncValue<CustomerListState>, CustomerListState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<CustomerListState>, CustomerListState>,
              AsyncValue<CustomerListState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

/// 客戶的訂單：店家訂單列表以 `buyer_id` 篩選（最近 50 筆）。

@ProviderFor(customerOrders)
final customerOrdersProvider = CustomerOrdersFamily._();

/// 客戶的訂單：店家訂單列表以 `buyer_id` 篩選（最近 50 筆）。

final class CustomerOrdersProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<OrderRecord>>,
          List<OrderRecord>,
          FutureOr<List<OrderRecord>>
        >
    with
        $FutureModifier<List<OrderRecord>>,
        $FutureProvider<List<OrderRecord>> {
  /// 客戶的訂單：店家訂單列表以 `buyer_id` 篩選（最近 50 筆）。
  CustomerOrdersProvider._({
    required CustomerOrdersFamily super.from,
    required String super.argument,
  }) : super(
         retry: _noRetry,
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
  $FutureProviderElement<List<OrderRecord>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<OrderRecord>> create(Ref ref) {
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

String _$customerOrdersHash() => r'491cd219b9d2b950adae74ef4c5664d9a2a5b646';

/// 客戶的訂單：店家訂單列表以 `buyer_id` 篩選（最近 50 筆）。

final class CustomerOrdersFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<OrderRecord>>, String> {
  CustomerOrdersFamily._()
    : super(
        retry: _noRetry,
        name: r'customerOrdersProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 客戶的訂單：店家訂單列表以 `buyer_id` 篩選（最近 50 筆）。

  CustomerOrdersProvider call(String buyerId) =>
      CustomerOrdersProvider._(argument: buyerId, from: this);

  @override
  String toString() => r'customerOrdersProvider';
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
        retry: _noRetry,
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

String _$sellerTagsHash() => r'78a0c708375b19d7b0f29f0179cbc3c401156640';
