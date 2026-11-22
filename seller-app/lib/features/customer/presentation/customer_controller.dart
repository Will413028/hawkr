import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../order/data/order_repository.dart';
import '../data/customer_model.dart';
import '../data/customer_orders.dart';
import '../data/customer_repository.dart';

part 'customer_controller.g.dart';

/// 讀取失敗直接呈現錯誤與重試，不走 provider 的自動重試（重試期間畫面會一直停在讀取中，失敗看不見）。
Duration? _noRetry(int retryCount, Object error) => null;

/// 客戶列表的篩選與排序：[type] 不給是全部客戶，[tagId] 是單一標籤。預設依訂單數由多到少。
class CustomerFilter {
  const CustomerFilter({
    this.type,
    this.sort = CustomerSort.CUSTOMER_SORT_ORDER_COUNT,
    this.descending = true,
    this.tagId,
  });

  final CustomerType? type;
  final CustomerSort sort;
  final bool descending;
  final String? tagId;

  CustomerFilter copyWith({
    CustomerType? type,
    bool clearType = false,
    CustomerSort? sort,
    bool? descending,
    String? tagId,
    bool clearTagId = false,
  }) => CustomerFilter(
    type: clearType ? null : (type ?? this.type),
    sort: sort ?? this.sort,
    descending: descending ?? this.descending,
    tagId: clearTagId ? null : (tagId ?? this.tagId),
  );
}

@riverpod
class CustomerFilterNotifier extends _$CustomerFilterNotifier {
  @override
  CustomerFilter build() => const CustomerFilter();

  void setType(CustomerType? type) =>
      state = state.copyWith(type: type, clearType: type == null);

  void setSort(CustomerSort sort, {bool descending = true}) =>
      state = state.copyWith(sort: sort, descending: descending);

  void setTagId(String? tagId) =>
      state = state.copyWith(tagId: tagId, clearTagId: tagId == null);
}

/// 客戶列表已載入的頁數加上繼續載入的狀態；[nextPageToken] 為空表示沒有更多。
class CustomerListState {
  const CustomerListState({
    required this.customers,
    required this.nextPageToken,
    this.loadingMore = false,
    this.loadMoreError,
  });

  final List<Customer> customers;
  final String nextPageToken;
  final bool loadingMore;

  /// 繼續載入失敗的錯誤；已載入的客戶保留，不當成清單讀取失敗。
  final Object? loadMoreError;

  bool get hasMore => nextPageToken.isNotEmpty;

  CustomerListState copyWith({
    List<Customer>? customers,
    String? nextPageToken,
    bool? loadingMore,
    Object? loadMoreError,
    bool clearError = false,
  }) => CustomerListState(
    customers: customers ?? this.customers,
    nextPageToken: nextPageToken ?? this.nextPageToken,
    loadingMore: loadingMore ?? this.loadingMore,
    loadMoreError: clearError ? null : (loadMoreError ?? this.loadMoreError),
  );
}

@Riverpod(retry: _noRetry)
class CustomerList extends _$CustomerList {
  @override
  Future<CustomerListState> build() async {
    final filter = ref.watch(customerFilterProvider);
    final page = await ref
        .read(customerRepositoryProvider)
        .listCustomers(
          type: filter.type,
          tagId: filter.tagId,
          sort: filter.sort,
          descending: filter.descending,
        );
    return CustomerListState(
      customers: page.customers,
      nextPageToken: page.nextPageToken,
    );
  }

  /// 載入下一頁；篩選換了（provider 重建）之後舊頁的結果丟棄。
  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || !current.hasMore || current.loadingMore) return;
    final filter = ref.read(customerFilterProvider);
    state = AsyncData(current.copyWith(loadingMore: true, clearError: true));
    try {
      final page = await ref
          .read(customerRepositoryProvider)
          .listCustomers(
            type: filter.type,
            tagId: filter.tagId,
            sort: filter.sort,
            descending: filter.descending,
            pageToken: current.nextPageToken,
          );
      if (!ref.mounted || ref.read(customerFilterProvider) != filter) return;
      state = AsyncData(
        current.copyWith(
          customers: [...current.customers, ...page.customers],
          nextPageToken: page.nextPageToken,
          loadingMore: false,
        ),
      );
    } catch (e) {
      if (!ref.mounted) return;
      state = AsyncData(current.copyWith(loadingMore: false, loadMoreError: e));
    }
  }
}

// ─── 顧客詳情相關 ────────────────────────────────────────────────────────────

/// 客戶的訂單：店家訂單列表以 `buyer_id` 篩選（最近 50 筆）。
@Riverpod(retry: _noRetry)
Future<List<OrderRecord>> customerOrders(Ref ref, String buyerId) async {
  final orders = await ref
      .read(orderRepositoryProvider)
      .fetchMatchedOrders(pageSize: 50, buyerId: buyerId);
  return orders.map(orderRecordFromSummary).toList();
}

// ─── 商家標籤 ────────────────────────────────────────────────────────────────

@Riverpod(retry: _noRetry)
Future<List<CustomerTag>> sellerTags(Ref ref) {
  return ref.read(customerRepositoryProvider).listTags();
}
