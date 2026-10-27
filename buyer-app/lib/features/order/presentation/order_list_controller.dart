import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/session/session_provider.dart';
import '../data/order_model.dart';
import '../data/order_repository.dart';

part 'order_list_controller.g.dart';

@riverpod
class OrderListController extends _$OrderListController {
  String _status = '0'; // 預設待付款
  int _page = 1;
  static const int _pageSize = 10;
  int _total = 0;
  List<Order> _allOrders = [];
  bool _isLoadingMore = false;

  @override
  Future<List<Order>> build() async {
    ref.keepAlive();
    _status = '0'; // 預設待付款 tab
    _page = 1;
    _allOrders = [];
    _isLoadingMore = false;
    // Apple App Review 合規：訪客（無 token）打 /app/orders 必 401，回空不打。
    // tab 在 IndexedStack 內會被 eager build，這裡守住避免訪客觸發 401。
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return const [];
    }
    return _fetchPage();
  }

  /// 判斷待付款訂單是否已過期（backend 應自動取消但有時延遲）
  /// 優先使用後端 paymentDeadline；舊資料 fallback statusTime。
  bool _isExpiredPendingPayment(Order o) {
    if (o.status != '0') return false;
    final deadlineStr = o.paymentDeadline.isNotEmpty
        ? o.paymentDeadline
        : o.statusTime;
    if (deadlineStr.isEmpty) return false;
    final deadline = DateTime.tryParse(deadlineStr);
    if (deadline == null) return false;
    final nowUtc = DateTime.now().toUtc();
    return deadline.toUtc().isBefore(nowUtc);
  }

  Future<List<Order>> _fetchPage() async {
    // Apple App Review 合規：訪客（無 token）不打 /app/orders。_fetchPage 是
    // 唯一打 API 的 choke point（build / switchTab / refresh / loadMore 都經這），
    // 守在這裡涵蓋 OrderListScreen.initState 直接呼叫 switchTab 的情況。
    final session = ref.read(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return _allOrders;
    }
    final result = await ref
        .read(orderRepositoryProvider)
        .getOrders(
          status: _status.isEmpty ? null : _status,
          page: _page,
          pageSize: _pageSize,
        );
    _total = result.total;
    // 過濾已過期的待付款訂單（backend 尚未自動取消時的安全網）
    final filteredRecords = result.records
        .where((o) => !_isExpiredPendingPayment(o))
        .toList();
    if (_page == 1) {
      _allOrders = filteredRecords;
    } else {
      _allOrders = [..._allOrders, ...filteredRecords];
    }
    // 若本頁回傳空，強制結束分頁
    if (result.records.isEmpty) {
      _total = _allOrders.length;
    }
    return _allOrders;
  }

  bool get hasMore => _allOrders.length < _total;

  Future<void> loadMore() async {
    if (!hasMore || _isLoadingMore) return;
    _isLoadingMore = true;
    try {
      _page++;
      state = AsyncData(await _fetchPage());
    } finally {
      _isLoadingMore = false;
    }
  }

  Future<void> switchTab(String status) async {
    _status = status;
    _page = 1;
    _allOrders = [];
    _isLoadingMore = false;
    state = const AsyncLoading();
    state = await AsyncValue.guard(() => _fetchPage());
  }

  Future<void> refresh() async {
    _page = 1;
    _allOrders = [];
    _isLoadingMore = false;
    // 下拉刷新時不設 AsyncLoading，避免觸發 TruckLoadingOverlay
    state = await AsyncValue.guard(() => _fetchPage());
  }
}
