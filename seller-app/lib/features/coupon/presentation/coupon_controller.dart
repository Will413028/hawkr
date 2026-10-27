import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/coupon_model.dart';
import '../data/coupon_repository.dart';

part 'coupon_controller.g.dart';

/// Controller for the merchant coupon list.
///
/// Provides CRUD operations and keeps the local list in sync.
@riverpod
class CouponListController extends _$CouponListController {
  CouponRepository get _repo => ref.read(couponRepositoryProvider);

  @override
  FutureOr<CouponListState> build() async {
    final coupons = await _repo.getSellerCoupons();
    return CouponListState(coupons: coupons);
  }

  /// Refresh the list from the server.
  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final coupons = await _repo.getSellerCoupons();
      return CouponListState(coupons: coupons);
    });
  }

  /// Create a coupon template and append it to the local list.
  Future<Coupon> create(Map<String, dynamic> body) async {
    final coupon = await _repo.createCoupon(body);
    // Refresh full list from server to get complete data
    unawaited(refresh());
    return coupon;
  }

  /// Delete a coupon by templateId.
  Future<bool> delete(String templateId) async {
    final ok = await _repo.deleteCoupon(templateId);
    if (ok) {
      final current = state.value ?? const CouponListState();
      state = AsyncValue.data(
        current.copyWith(
          coupons: current.coupons
              .where((c) => c.templateId != templateId)
              .toList(),
        ),
      );
    }
    return ok;
  }
}
