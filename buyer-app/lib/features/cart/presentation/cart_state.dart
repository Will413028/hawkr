import 'dart:collection';

import 'package:flutter/foundation.dart';

import '../../address/data/address_model.dart';
import '../data/cart_data.dart';

enum CartContentStatus { initial, empty, ready, failure }

enum CartProblemKind { staleData, business }

@immutable
class CartProblem {
  const CartProblem({
    required this.kind,
    required this.message,
    required this.revision,
  });

  final CartProblemKind kind;
  final String message;
  final int revision;
}

/// Immutable state rendered by [CartScreen].
///
/// A problem never replaces [data]. This is what makes a stale-data failure
/// different from a successful empty cart.
@immutable
class CartState {
  CartState({
    required this.status,
    required this.data,
    Set<int> checkedSkuIds = const {},
    this.isRefreshing = false,
    this.problem,
    List<Address> userAddresses = const [],
    this.selectedAddressId,
    List<Map<String, dynamic>> storePickups = const [],
    this.selectedStorePickupId,
    Map<String, dynamic>? validateResult,
    List<Map<String, dynamic>> shippingOptions = const [],
    this.selectedShippingIndex = 0,
    this.validateLoading = false,
    this.shippingFeeLoading = false,
    this.preciseShippingFee,
    this.lastValidatedAt,
    Set<int> updatingSkuIds = const {},
  }) : checkedSkuIds = Set<int>.unmodifiable(checkedSkuIds),
       userAddresses = List<Address>.unmodifiable(userAddresses),
       storePickups = List<Map<String, dynamic>>.unmodifiable(
         storePickups.map(
           (store) => UnmodifiableMapView<String, dynamic>(Map.of(store)),
         ),
       ),
       validateResult = validateResult == null
           ? null
           : UnmodifiableMapView<String, dynamic>(Map.of(validateResult)),
       shippingOptions = List<Map<String, dynamic>>.unmodifiable(
         shippingOptions.map(
           (option) => UnmodifiableMapView<String, dynamic>(Map.of(option)),
         ),
       ),
       updatingSkuIds = Set<int>.unmodifiable(updatingSkuIds);

  factory CartState.initial() =>
      CartState(status: CartContentStatus.initial, data: null);

  factory CartState.fromData(
    CartData data, {
    Set<int> checkedSkuIds = const {},
  }) => CartState(
    status: data.isEmpty ? CartContentStatus.empty : CartContentStatus.ready,
    data: data,
    checkedSkuIds: checkedSkuIds,
  );

  factory CartState.failure(CartProblem problem) => CartState(
    status: CartContentStatus.failure,
    data: null,
    problem: problem,
  );

  final CartContentStatus status;
  final CartData? data;
  final Set<int> checkedSkuIds;
  final bool isRefreshing;
  final CartProblem? problem;
  final List<Address> userAddresses;
  final int? selectedAddressId;
  final List<Map<String, dynamic>> storePickups;
  final String? selectedStorePickupId;
  final Map<String, dynamic>? validateResult;
  final List<Map<String, dynamic>> shippingOptions;

  /// 買家選定的配送選項在 [shippingOptions] 內的 index。
  ///
  /// 畫面不直接讀它，一律走 [shippingSelection]——那是唯一的判別點（含越界檢查）。
  final int selectedShippingIndex;

  final bool validateLoading;
  final bool shippingFeeLoading;

  /// The checkout snapshot must finish syncing before review can open.
  bool get checkoutPending =>
      isRefreshing ||
      updatingSkuIds.isNotEmpty ||
      validateLoading ||
      shippingFeeLoading;
  final double? preciseShippingFee;
  final DateTime? lastValidatedAt;
  final Set<int> updatingSkuIds;

  int get itemCount => data?.itemCount ?? 0;

  Address? get selectedAddress {
    final id = selectedAddressId;
    if (id == null) return null;
    for (final address in userAddresses) {
      if (address.id == id) return address;
    }
    return null;
  }

  /// 目前選定的配送方式 —— **唯一**判別點。
  CartShippingSelection get shippingSelection {
    if (shippingOptions.isEmpty ||
        selectedShippingIndex < 0 ||
        selectedShippingIndex >= shippingOptions.length) {
      return const ShippingUnselected();
    }
    return TieredShippingSelected(
      selectedShippingIndex,
      shippingOptions[selectedShippingIndex],
    );
  }

  Map<String, dynamic>? get selectedStorePickup {
    final id = selectedStorePickupId;
    if (id == null) return null;
    for (final store in storePickups) {
      if (store['id']?.toString() == id) return store;
    }
    return null;
  }

  CartState copyWith({
    CartContentStatus? status,
    Object? data = _notProvided,
    Set<int>? checkedSkuIds,
    bool? isRefreshing,
    Object? problem = _notProvided,
    List<Address>? userAddresses,
    Object? selectedAddressId = _notProvided,
    List<Map<String, dynamic>>? storePickups,
    Object? selectedStorePickupId = _notProvided,
    Object? validateResult = _notProvided,
    List<Map<String, dynamic>>? shippingOptions,
    int? selectedShippingIndex,
    bool? validateLoading,
    bool? shippingFeeLoading,
    Object? preciseShippingFee = _notProvided,
    Object? lastValidatedAt = _notProvided,
    Set<int>? updatingSkuIds,
  }) => CartState(
    status: status ?? this.status,
    data: identical(data, _notProvided) ? this.data : data as CartData?,
    checkedSkuIds: checkedSkuIds ?? this.checkedSkuIds,
    isRefreshing: isRefreshing ?? this.isRefreshing,
    problem: identical(problem, _notProvided)
        ? this.problem
        : problem as CartProblem?,
    userAddresses: userAddresses ?? this.userAddresses,
    selectedAddressId: identical(selectedAddressId, _notProvided)
        ? this.selectedAddressId
        : selectedAddressId as int?,
    storePickups: storePickups ?? this.storePickups,
    selectedStorePickupId: identical(selectedStorePickupId, _notProvided)
        ? this.selectedStorePickupId
        : selectedStorePickupId as String?,
    validateResult: identical(validateResult, _notProvided)
        ? this.validateResult
        : validateResult as Map<String, dynamic>?,
    shippingOptions: shippingOptions ?? this.shippingOptions,
    selectedShippingIndex: selectedShippingIndex ?? this.selectedShippingIndex,
    validateLoading: validateLoading ?? this.validateLoading,
    shippingFeeLoading: shippingFeeLoading ?? this.shippingFeeLoading,
    preciseShippingFee: identical(preciseShippingFee, _notProvided)
        ? this.preciseShippingFee
        : preciseShippingFee as double?,
    lastValidatedAt: identical(lastValidatedAt, _notProvided)
        ? this.lastValidatedAt
        : lastValidatedAt as DateTime?,
    updatingSkuIds: updatingSkuIds ?? this.updatingSkuIds,
  );
}

const Object _notProvided = Object();

/// 「買家現在選了哪一種配送」的判別聯合。
@immutable
sealed class CartShippingSelection {
  const CartShippingSelection();
}

/// 現在沒有可用的配送選擇（尚未驗證、或全部超限）。
@immutable
final class ShippingUnselected extends CartShippingSelection {
  const ShippingUnselected();
}

/// 選定了一筆來自 `availableShippingOptions` 的配送選項。
@immutable
final class TieredShippingSelected extends CartShippingSelection {
  const TieredShippingSelected(this.index, this.option);
  final int index;
  final Map<String, dynamic> option;

  String get shippingMethod => option['shippingMethod']?.toString() ?? '';
}
