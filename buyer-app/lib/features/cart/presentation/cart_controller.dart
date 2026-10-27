import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/codemeta/shipping_presentation.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/exceptions/seller_suspension_codes.dart';
import '../../../core/session/session_model.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/storage/auth_storage.dart';
import '../../address/data/address_repository.dart';
import '../data/cart_data.dart';
import '../data/cart_repository.dart';
import 'cart_state.dart';

part 'cart_controller.g.dart';

Future<String?> _resolveGuestId(Ref ref) async {
  final session = await ref.read(sessionProvider.future);
  if (hasUsableSession(session)) return null;
  return AuthStorage.guestCartId();
}

Future<String?> _guestIdForSession(SessionModel? session) async {
  if (hasUsableSession(session)) return null;
  return AuthStorage.guestCartId();
}

/// Badge and cart screen read the exact same controller snapshot.
@riverpod
Future<int> cartCount(Ref ref) async {
  final cart = await ref.watch(cartControllerProvider.future);
  return cart.itemCount;
}

@Riverpod(keepAlive: true)
class CartController extends _$CartController {
  static const quantityDebounce = Duration(milliseconds: 500);
  static const validationDebounce = Duration(milliseconds: 300);

  StreamSubscription<CartData>? _snapshotSubscription;
  Timer? _validationTimer;
  final Map<int, Timer> _quantityTimers = {};
  final Map<int, int> _pendingQuantities = {};
  final Map<int, int> _confirmedQuantities = {};
  final Map<int, int> _quantityGenerations = {};
  int _problemRevision = 0;
  int _validationGeneration = 0;
  int _shippingFeeGeneration = 0;

  @override
  Future<CartState> build() async {
    ref.keepAlive();
    // A session change rebuilds this keepAlive controller. Guest debounces
    // must never be allowed to flush into the newly authenticated member cart.
    _cancelTransientWork();
    _snapshotSubscription?.cancel();
    final repository = ref.watch(cartRepositoryProvider);
    _snapshotSubscription = repository.snapshots.listen(_onRepositorySnapshot);
    ref.onDispose(_dispose);

    final session = await ref.watch(sessionProvider.future);
    final guestId = await _guestIdForSession(session);
    try {
      final data = await repository.getCartData(guestId: guestId);
      var next = _stateForData(data, previous: null);
      next = await _loadSupportingData(next);
      return next;
    } on Object catch (error) {
      return CartState.failure(_problemFor(error, stale: true));
    }
  }

  void _dispose() {
    _snapshotSubscription?.cancel();
    _cancelTransientWork();
  }

  void _cancelTransientWork() {
    _validationGeneration++;
    _shippingFeeGeneration++;
    _validationTimer?.cancel();
    _validationTimer = null;
    for (final timer in _quantityTimers.values) {
      timer.cancel();
    }
    _quantityTimers.clear();
    _pendingQuantities.clear();
    _confirmedQuantities.clear();
    _quantityGenerations.clear();
  }

  Future<CartState> _loadSupportingData(CartState initial) async {
    var next = initial;
    try {
      final addresses = await ref
          .read(addressRepositoryProvider)
          .getAddresses();
      int? selectedAddressId = next.selectedAddressId;
      if (selectedAddressId == null && addresses.isNotEmpty) {
        final defaultAddress = addresses.firstWhere(
          (address) => address.isDefault,
          orElse: () => addresses.first,
        );
        selectedAddressId = defaultAddress.id;
      }
      next = next.copyWith(
        userAddresses: addresses,
        selectedAddressId: selectedAddressId,
      );
    } on Object {
      // Guest carts and temporarily unavailable address services must not
      // replace a successfully loaded cart.
    }

    try {
      final stores = await ref.read(cartRepositoryProvider).getStorePickups();
      String? selectedStoreId = next.selectedStorePickupId;
      if (selectedStoreId == null && stores.isNotEmpty) {
        final defaultStore = stores.firstWhere(
          (store) => store['isDefault']?.toString() == 'true',
          orElse: () => stores.first,
        );
        selectedStoreId = defaultStore['id']?.toString();
      }
      next = next.copyWith(
        storePickups: stores,
        selectedStorePickupId: selectedStoreId,
      );
    } on Object {
      // Store pickup is optional until checkout; preserve the cart snapshot.
    }
    return next;
  }

  Future<void> initialize({int? preSelectedSkuId}) async {
    final current = state.value;
    if (current == null) return;
    final data = current.data;
    if (data == null) return;

    final checked = _normalizedSelection(
      data,
      current.checkedSkuIds,
      preSelectedSkuId: preSelectedSkuId,
    );
    state = AsyncData(current.copyWith(checkedSkuIds: checked));
    scheduleValidation();
  }

  Future<void> refresh() async {
    final current = state.value;
    if (current == null) return;
    state = AsyncData(current.copyWith(isRefreshing: true, problem: null));
    try {
      final guestId = await _resolveGuestId(ref);
      final data = await ref
          .read(cartRepositoryProvider)
          .getCartData(guestId: guestId);
      _setCartData(data, clearProblem: true, refreshing: false);
    } on Object catch (error) {
      final latest = state.value ?? current;
      state = AsyncData(
        latest.copyWith(
          isRefreshing: false,
          problem: _problemFor(error, stale: true),
          status: latest.data == null
              ? CartContentStatus.failure
              : latest.status,
        ),
      );
    }
  }

  Future<void> refreshAddresses() async {
    final current = state.value;
    if (current == null) return;
    try {
      final addresses = await ref
          .read(addressRepositoryProvider)
          .getAddresses();
      final latest = state.value;
      if (latest == null) return;
      var selectedId = latest.selectedAddressId;
      if (!addresses.any((address) => address.id == selectedId)) {
        selectedId = addresses.isEmpty
            ? null
            : addresses
                  .firstWhere(
                    (address) => address.isDefault,
                    orElse: () => addresses.first,
                  )
                  .id;
      }
      state = AsyncData(
        latest.copyWith(
          userAddresses: addresses,
          selectedAddressId: selectedId,
          preciseShippingFee: null,
          shippingFeeLoading: false,
        ),
      );
      unawaited(calculateShippingFee());
    } on Object catch (error) {
      _setProblem(error, stale: true);
    }
  }

  Future<void> refreshStorePickups() async {
    final current = state.value;
    if (current == null) return;
    try {
      final stores = await ref.read(cartRepositoryProvider).getStorePickups();
      final latest = state.value;
      if (latest == null) return;
      var selectedId = latest.selectedStorePickupId;
      if (!stores.any((store) => store['id']?.toString() == selectedId)) {
        selectedId = stores.isEmpty
            ? null
            : stores
                  .firstWhere(
                    (store) => store['isDefault']?.toString() == 'true',
                    orElse: () => stores.first,
                  )['id']
                  ?.toString();
      }
      state = AsyncData(
        latest.copyWith(
          storePickups: stores,
          selectedStorePickupId: selectedId,
        ),
      );
    } on Object catch (error) {
      _setProblem(error, stale: true);
    }
  }

  void selectGroup(CartMerchantGroup group, bool selected) {
    final current = state.value;
    if (current == null) return;
    final checked = Set<int>.of(current.checkedSkuIds);
    if (selected) {
      checked
        ..clear()
        ..addAll(
          group.items
              .where((item) => item.purchasable)
              .map((item) => item.skuId),
        );
    } else {
      checked.removeAll(group.items.map((item) => item.skuId));
    }
    state = AsyncData(current.copyWith(checkedSkuIds: checked));
    scheduleValidation();
  }

  void selectItem(int skuId, bool selected) {
    final current = state.value;
    final data = current?.data;
    if (current == null || data == null) return;
    final owner = data.groups
        .where((group) => group.items.any((item) => item.skuId == skuId))
        .firstOrNull;
    if (owner == null) return;
    if (selected &&
        !owner.items.any((item) => item.skuId == skuId && item.purchasable)) {
      return;
    }

    final checked = Set<int>.of(current.checkedSkuIds);
    if (selected) {
      final activeStore = _activeStoreId(data, checked);
      if (activeStore != null && activeStore != owner.storeId) checked.clear();
      checked.add(skuId);
    } else {
      checked.remove(skuId);
    }
    state = AsyncData(current.copyWith(checkedSkuIds: checked));
    scheduleValidation();
  }

  void selectAddress(int addressId) {
    final current = state.value;
    if (current == null) return;
    state = AsyncData(
      current.copyWith(
        selectedAddressId: addressId,
        preciseShippingFee: null,
        shippingFeeLoading: false,
      ),
    );
    unawaited(calculateShippingFee());
  }

  void selectStorePickup(String? storeId) {
    final current = state.value;
    if (current == null) return;
    state = AsyncData(current.copyWith(selectedStorePickupId: storeId));
  }

  void selectShippingOption(int index) {
    final current = state.value;
    if (current == null ||
        index < 0 ||
        index >= current.shippingOptions.length) {
      return;
    }
    state = AsyncData(
      current.copyWith(
        selectedShippingIndex: index,
        preciseShippingFee: null,
        shippingFeeLoading: false,
      ),
    );
    unawaited(calculateShippingFee());
  }

  void scheduleValidation() {
    _validationTimer?.cancel();
    // A superseded response must never restore another store's checkout data.
    _validationGeneration++;
    _shippingFeeGeneration++;
    final current = state.value;
    if (current == null) return;
    if (current.checkedSkuIds.isEmpty) {
      state = AsyncData(
        current.copyWith(
          validateResult: null,
          shippingOptions: const [],
          selectedShippingIndex: 0,
          validateLoading: false,
          shippingFeeLoading: false,
          preciseShippingFee: null,
          lastValidatedAt: null,
        ),
      );
      return;
    }

    state = AsyncData(
      current.copyWith(
        validateLoading: true,
        shippingFeeLoading: false,
        validateResult: null,
        shippingOptions: const [],
        preciseShippingFee: null,
        lastValidatedAt: null,
      ),
    );
    _validationTimer = Timer(
      validationDebounce,
      () => unawaited(validateNow()),
    );
  }

  Future<void> validateNow() async {
    _validationTimer?.cancel();
    final generation = ++_validationGeneration;
    _shippingFeeGeneration++;
    final current = state.value;
    final data = current?.data;
    if (current == null || data == null) return;
    final selectedItems = data.items
        .where((item) => current.checkedSkuIds.contains(item.skuId))
        .toList();
    if (selectedItems.isEmpty) {
      state = AsyncData(
        current.copyWith(
          validateLoading: false,
          shippingFeeLoading: false,
          validateResult: null,
          shippingOptions: const [],
          preciseShippingFee: null,
          lastValidatedAt: null,
        ),
      );
      return;
    }

    state = AsyncData(
      current.copyWith(
        validateLoading: true,
        shippingFeeLoading: false,
        validateResult: null,
        shippingOptions: const [],
        preciseShippingFee: null,
        lastValidatedAt: null,
      ),
    );
    try {
      final validation = await ref
          .read(cartRepositoryProvider)
          .validateCheckout(
            items: selectedItems,
            productIdsBySku: data.productIdsBySku,
          );
      if (generation != _validationGeneration) return;
      final latest = state.value;
      if (latest == null) return;
      if (!validation.valid) {
        state = AsyncData(
          latest.copyWith(
            validateResult: validation.result,
            shippingOptions: const [],
            selectedShippingIndex: 0,
            validateLoading: false,
            lastValidatedAt: DateTime.now(),
            problem: CartProblem(
              kind: CartProblemKind.business,
              message: _validationMessage(validation),
              revision: ++_problemRevision,
            ),
          ),
        );
        if (_validationRequiresRefresh(validation)) unawaited(refresh());
        return;
      }

      final selectedIndex =
          latest.selectedShippingIndex < validation.shippingOptions.length
          ? latest.selectedShippingIndex
          : 0;
      state = AsyncData(
        latest.copyWith(
          validateResult: validation.result,
          shippingOptions: validation.shippingOptions,
          selectedShippingIndex: selectedIndex,
          validateLoading: false,
          lastValidatedAt: DateTime.now(),
          problem: null,
        ),
      );
    } on CartBusinessException catch (error) {
      if (generation != _validationGeneration) return;
      final latest = state.value;
      if (latest == null) return;
      state = AsyncData(
        latest.copyWith(
          validateResult: const <String, dynamic>{},
          shippingOptions: const [],
          selectedShippingIndex: 0,
          validateLoading: false,
          problem: CartProblem(
            kind: CartProblemKind.business,
            message: _businessMessage(error.code, error.message),
            revision: ++_problemRevision,
          ),
        ),
      );
      if (_businessCodeRequiresRefresh(error.code)) unawaited(refresh());
    } on Object catch (error) {
      if (generation != _validationGeneration) return;
      final latest = state.value;
      if (latest == null) return;
      state = AsyncData(
        latest.copyWith(
          validateResult: const <String, dynamic>{},
          shippingOptions: const [],
          selectedShippingIndex: 0,
          validateLoading: false,
          problem: _problemFor(error, stale: true),
        ),
      );
    }
  }

  Future<void> calculateShippingFee() async {
    final generation = ++_shippingFeeGeneration;
    final current = state.value;
    final data = current?.data;
    if (current == null || data == null) return;
    if (current.shippingOptions.isEmpty ||
        current.selectedShippingIndex < 0 ||
        current.selectedShippingIndex >= current.shippingOptions.length) {
      return;
    }
    final shippingOption =
        current.shippingOptions[current.selectedShippingIndex];
    // 運費精算只對「宅配到府」有意義（要用收件地址算）。
    // 明確分派而非 `!isStorePickup`：未知 / 本版未支援的配送方式不得被當成宅配
    // 拿去打精算 API。
    if (shippingPresentationOf(shippingOption['shippingMethod']?.toString()) !=
        ShippingPresentation.homeDelivery) {
      return;
    }
    final address = current.selectedAddress;
    if (address == null) return;
    final items = data.items
        .where((item) => current.checkedSkuIds.contains(item.skuId))
        .toList();
    if (items.isEmpty) return;

    state = AsyncData(
      current.copyWith(shippingFeeLoading: true, preciseShippingFee: null),
    );
    try {
      final fee = await ref
          .read(cartRepositoryProvider)
          .getShippingFee(
            items: items,
            productIdsBySku: data.productIdsBySku,
            shippingOption: shippingOption,
            address: address,
          );
      if (generation != _shippingFeeGeneration) return;
      final latest = state.value;
      if (latest != null) {
        state = AsyncData(
          latest.copyWith(preciseShippingFee: fee, shippingFeeLoading: false),
        );
      }
    } on Object catch (error) {
      if (generation != _shippingFeeGeneration) return;
      final latest = state.value;
      if (latest == null) return;
      state = AsyncData(
        latest.copyWith(
          shippingFeeLoading: false,
          problem: _problemFor(error, stale: true),
        ),
      );
    }
  }

  void updateQuantity(int skuId, int quantity) {
    final current = state.value;
    final data = current?.data;
    if (current == null || data == null || quantity < 1) return;
    final item = data.items
        .where((candidate) => candidate.skuId == skuId)
        .firstOrNull;
    if (item == null) return;

    _confirmedQuantities.putIfAbsent(skuId, () => item.count);
    _pendingQuantities[skuId] = quantity;
    _quantityGenerations[skuId] = (_quantityGenerations[skuId] ?? 0) + 1;
    _quantityTimers[skuId]?.cancel();
    final updating = Set<int>.of(current.updatingSkuIds)..add(skuId);
    state = AsyncData(
      current.copyWith(
        data: data.replaceQuantity(skuId, quantity),
        updatingSkuIds: updating,
        problem: null,
      ),
    );
    _quantityTimers[skuId] = Timer(
      quantityDebounce,
      () => unawaited(_flushQuantity(skuId)),
    );
    if (current.checkedSkuIds.contains(skuId)) scheduleValidation();
  }

  Future<void> flushPendingQuantityUpdates() async {
    final skuIds = _pendingQuantities.keys.toList(growable: false);
    for (final skuId in skuIds) {
      _quantityTimers.remove(skuId)?.cancel();
      await _flushQuantity(skuId);
    }
  }

  Future<void> _flushQuantity(int skuId) async {
    final desired = _pendingQuantities[skuId];
    final generation = _quantityGenerations[skuId];
    if (desired == null || generation == null) return;
    try {
      final guestId = await _resolveGuestId(ref);
      final refreshed = await ref
          .read(cartRepositoryProvider)
          .updateItem(skuId, desired, guestId: guestId);
      if (_quantityGenerations[skuId] != generation) return;

      _pendingQuantities.remove(skuId);
      _confirmedQuantities.remove(skuId);
      _quantityTimers.remove(skuId)?.cancel();
      final latest = state.value;
      final updating = Set<int>.of(latest?.updatingSkuIds ?? const {})
        ..remove(skuId);
      _setCartData(refreshed, clearProblem: true, updatingSkuIds: updating);
    } on Object catch (error) {
      if (_quantityGenerations[skuId] != generation) return;
      final rollback = _confirmedQuantities.remove(skuId);
      _pendingQuantities.remove(skuId);
      _quantityTimers.remove(skuId)?.cancel();
      final latest = state.value;
      if (latest == null) return;
      final updating = Set<int>.of(latest.updatingSkuIds)..remove(skuId);
      state = AsyncData(
        latest.copyWith(
          data: rollback == null
              ? latest.data
              : latest.data?.replaceQuantity(skuId, rollback),
          updatingSkuIds: updating,
          problem: _problemFor(error, stale: true),
        ),
      );
      scheduleValidation();
    }
  }

  Future<bool> removeItem(int skuId) async {
    _cancelPendingQuantity(skuId);
    try {
      final guestId = await _resolveGuestId(ref);
      final data = await ref.read(cartRepositoryProvider).removeItems([
        skuId,
      ], guestId: guestId);
      _setCartData(data, clearProblem: true);
      scheduleValidation();
      return true;
    } on Object catch (error) {
      _setProblem(error, stale: true);
      return false;
    }
  }

  Future<bool> clearCart() async {
    final current = state.value;
    final ids =
        current?.data?.items.map((item) => item.id).where((id) => id > 0) ??
        const Iterable<int>.empty();
    if (ids.isEmpty) return true;
    try {
      final guestId = await _resolveGuestId(ref);
      final data = await ref
          .read(cartRepositoryProvider)
          .removeItems(ids.toList(), guestId: guestId);
      _setCartData(data, clearProblem: true);
      return data.isEmpty;
    } on Object catch (error) {
      _setProblem(error, stale: true);
      return false;
    }
  }

  Future<bool> mergeGuestCart(String guestId) async {
    final current = state.value;
    try {
      final repository = ref.read(cartRepositoryProvider);
      await repository.mergeCarts(guestId);
      final data = await repository.getCartData();
      _setCartData(data, clearProblem: true);
      return true;
    } on Object catch (error) {
      if (current != null) {
        state = AsyncData(
          current.copyWith(problem: _problemFor(error, stale: true)),
        );
      }
      return false;
    }
  }

  Future<List<Map<String, dynamic>>> loadAvailableCoupons({
    required String storeId,
    required int orderAmount,
  }) => ref
      .read(cartRepositoryProvider)
      .getAvailableCoupons(storeId: storeId, orderAmount: orderAmount);

  void _cancelPendingQuantity(int skuId) {
    _quantityTimers.remove(skuId)?.cancel();
    _pendingQuantities.remove(skuId);
    _confirmedQuantities.remove(skuId);
    _quantityGenerations.remove(skuId);
  }

  void _onRepositorySnapshot(CartData data) {
    final current = state.value;
    if (current == null) return;
    var reconciled = data;
    for (final entry in _pendingQuantities.entries) {
      reconciled = reconciled.replaceQuantity(entry.key, entry.value);
    }
    _setCartData(reconciled, clearProblem: false);
  }

  void _setCartData(
    CartData data, {
    required bool clearProblem,
    bool? refreshing,
    Set<int>? updatingSkuIds,
  }) {
    final current = state.value;
    state = AsyncData(
      _stateForData(data, previous: current).copyWith(
        isRefreshing: refreshing ?? current?.isRefreshing ?? false,
        problem: clearProblem ? null : current?.problem,
        updatingSkuIds: updatingSkuIds ?? current?.updatingSkuIds ?? const {},
      ),
    );
    scheduleValidation();
  }

  CartState _stateForData(CartData data, {required CartState? previous}) {
    final checked = _normalizedSelection(
      data,
      previous?.checkedSkuIds ?? const {},
      selectFirstStore: previous == null,
    );
    return CartState(
      status: data.isEmpty ? CartContentStatus.empty : CartContentStatus.ready,
      data: data,
      checkedSkuIds: checked,
      isRefreshing: previous?.isRefreshing ?? false,
      problem: previous?.problem,
      userAddresses: previous?.userAddresses ?? const [],
      selectedAddressId: previous?.selectedAddressId,
      storePickups: previous?.storePickups ?? const [],
      selectedStorePickupId: previous?.selectedStorePickupId,
      validateResult: previous?.validateResult,
      shippingOptions: previous?.shippingOptions ?? const [],
      selectedShippingIndex: previous?.selectedShippingIndex ?? 0,
      validateLoading: previous?.validateLoading ?? false,
      shippingFeeLoading: previous?.shippingFeeLoading ?? false,
      preciseShippingFee: previous?.preciseShippingFee,
      lastValidatedAt: previous?.lastValidatedAt,
      updatingSkuIds: previous?.updatingSkuIds ?? const {},
    );
  }

  Set<int> _normalizedSelection(
    CartData data,
    Set<int> existing, {
    int? preSelectedSkuId,
    bool selectFirstStore = false,
  }) {
    if (data.items.isEmpty) return const {};
    final purchasableIds = data.items
        .where((item) => item.purchasable)
        .map((item) => item.skuId)
        .toSet();
    var checked = existing.intersection(purchasableIds);

    if (preSelectedSkuId != null && purchasableIds.contains(preSelectedSkuId)) {
      checked = {preSelectedSkuId};
    }

    final activeStore = _activeStoreId(data, checked);
    if (activeStore != null) {
      final activeIds = data.groups
          .where((group) => group.storeId == activeStore)
          .expand((group) => group.items)
          .map((item) => item.skuId)
          .toSet();
      checked = checked.intersection(activeIds);
    }

    if (checked.isEmpty && selectFirstStore) {
      for (final group in data.groups) {
        final available = group.items.where((item) => item.purchasable);
        if (available.isNotEmpty) {
          return available.map((item) => item.skuId).toSet();
        }
      }
    }
    return checked;
  }

  String? _activeStoreId(CartData data, Set<int> checked) {
    for (final group in data.groups) {
      if (group.items.any((item) => checked.contains(item.skuId))) {
        return group.storeId;
      }
    }
    return null;
  }

  void _setProblem(Object error, {required bool stale}) {
    final current = state.value;
    if (current == null) return;
    state = AsyncData(
      current.copyWith(problem: _problemFor(error, stale: stale)),
    );
  }

  CartProblem _problemFor(Object error, {required bool stale}) {
    final message = switch (error) {
      AppException() => error.message,
      CartBusinessException() => error.message,
      _ => '購物車更新失敗，請稍後再試',
    };
    return CartProblem(
      kind: stale ? CartProblemKind.staleData : CartProblemKind.business,
      message: message,
      revision: ++_problemRevision,
    );
  }

  String _validationMessage(CartValidation validation) {
    if (validation.hasErrorCode(
      SellerSuspensionCode.sellerSuspendedProductUnavailable,
    )) {
      return '部分商品暫停販售，已自結帳移除';
    }
    if (validation.hasErrorCode('4003')) {
      return validation.firstErrorMessage ?? '部分商品已下架，請重新確認購物車';
    }
    if (validation.hasErrorCode('4015')) {
      return validation.firstErrorMessage ?? '不可購買自己上架的商品';
    }
    if (validation.hasErrorCode('4001') || validation.hasErrorCode('4002')) {
      return validation.firstErrorMessage ?? '部分商品庫存不足，請調整數量';
    }
    return validation.firstErrorMessage ?? '商品驗證失敗，請重新確認';
  }

  bool _validationRequiresRefresh(CartValidation validation) =>
      validation.hasErrorCode(
        SellerSuspensionCode.sellerSuspendedProductUnavailable,
      ) ||
      validation.hasErrorCode('4003') ||
      validation.hasErrorCode('4015') ||
      validation.hasErrorCode('4001') ||
      validation.hasErrorCode('4002');

  String _businessMessage(String code, String fallback) {
    if (code == SellerSuspensionCode.sellerSuspendedProductUnavailable) {
      return '部分商品暫停販售，已自結帳移除';
    }
    if (code == '4015') return fallback.isEmpty ? '不可購買自己上架的商品' : fallback;
    return fallback;
  }

  bool _businessCodeRequiresRefresh(String code) =>
      code == SellerSuspensionCode.sellerSuspendedProductUnavailable ||
      code == '4015';
}
