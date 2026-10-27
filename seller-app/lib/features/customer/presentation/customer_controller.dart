import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/customer_repository.dart';
import '../data/customer_model.dart';
import '../../coupon/data/coupon_model.dart';

part 'customer_controller.freezed.dart';
part 'customer_controller.g.dart';

@freezed
abstract class CustomerFilter with _$CustomerFilter {
  const factory CustomerFilter({
    /// 'normal' / 'vip' / 'blocklist' / null (全部)
    String? customerType,
    @Default('visitCount') String sortBy,
    String? keyword,
    int? tagId,
  }) = _CustomerFilter;
}

@riverpod
class CustomerFilterNotifier extends _$CustomerFilterNotifier {
  @override
  CustomerFilter build() => const CustomerFilter();

  void setCustomerType(String? type) =>
      state = state.copyWith(customerType: type);

  void setSortBy(String sortBy) => state = state.copyWith(sortBy: sortBy);

  void setKeyword(String? keyword) => state = state.copyWith(keyword: keyword);

  void setTagId(int? tagId) => state = state.copyWith(tagId: tagId);
}

@riverpod
Future<List<Customer>> customerListNotifier(Ref ref) async {
  final filter = ref.watch(customerFilterProvider);
  final repo = ref.read(customerRepositoryProvider);

  return repo.getCustomers(
    customerType: filter.customerType,
    sortBy: filter.sortBy,
    keyword: filter.keyword,
    tagId: filter.tagId,
  );
}

// ─── 顧客詳情相關 ────────────────────────────────────────────────────────────

@riverpod
Future<({List<OrderRecord> orders, List<SpendingRecord> spending})>
customerOrders(Ref ref, String customerId) {
  return ref.read(customerRepositoryProvider).getCustomerOrders(customerId);
}

@riverpod
Future<List<Coupon>> customerCoupons(Ref ref, String customerId) {
  return ref.read(customerRepositoryProvider).getCustomerCoupons(customerId);
}

@riverpod
Future<List<CustomerTag>> customerTags(Ref ref, String buyerId) {
  return ref.read(customerRepositoryProvider).getCustomerTags(buyerId);
}

// ─── 商家標籤 ────────────────────────────────────────────────────────────────

@riverpod
Future<List<CustomerTag>> sellerTags(Ref ref) {
  return ref.read(customerRepositoryProvider).getSellerTags();
}
