import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';

/// 以 proto3 JSON 組出 `Dashboard`（int64 是字串；沒有寫的區塊就是沒設）。
Dashboard dashboardFromJson(Map<String, Object?> json) =>
    Dashboard()..mergeFromProto3Json(json);

Map<String, Object?> _period(
  int fromMonth,
  int fromDay,
  int toMonth,
  int toDay,
) => {
  'firstDate': {'year': 2026, 'month': fromMonth, 'day': fromDay},
  'lastDate': {'year': 2026, 'month': toMonth, 'day': toDay},
};

/// 四個區塊都有的儀表板（期間 9/30–10/6，前期 9/23–9/29）。
Map<String, Object?> fullDashboardJson({
  String revenue = '128400',
  String previousRevenue = '114643',
  int orders = 36,
  int previousOrders = 35,
  int customers = 72,
  int previousCustomers = 68,
  int views = 1840,
  int previousViews = 1897,
  int newFollowers = 5,
  int pendingShipment = 5,
  int inProgress = 8,
  int closed = 23,
  int aftersales = 2,
}) => {
  'currentPeriod': _period(9, 30, 10, 6),
  'previousPeriod': _period(9, 23, 9, 29),
  'sales': {
    'revenue': {'current': revenue, 'previous': previousRevenue},
    'orders': {'current': orders, 'previous': previousOrders},
    'customers': {'current': customers, 'previous': previousCustomers},
  },
  'followers': {
    'total': 108,
    'newFollowers': {'current': newFollowers, 'previous': 0},
  },
  'productViews': {'current': views, 'previous': previousViews},
  'orderStatusCounts': {
    'pendingShipmentCount': pendingShipment,
    'inProgressCount': inProgress,
    'closedCount': closed,
    'aftersalesCount': aftersales,
  },
};

Dashboard fullDashboard() => dashboardFromJson(fullDashboardJson());
