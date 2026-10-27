import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';

/// 儀表板的顯示規則（和 website `lib/seller/dashboard.ts`、`DashboardPage.tsx` 的 `buildStats` 同一組）。
///
/// 四個區塊讀不到時伺服器不設，不是 0：沒設的區塊一律顯示 [dashboardUnset]，不補 0。

const dashboardUnset = '—';

/// 較前 7 日的成長百分比（整數，四捨五入，.5 往正無限大，同 JavaScript 的 `Math.round`）。
/// 區塊沒設或前期為 0 時是 null：沒有可比的基準，不是 0%。
int? periodGrowthPercent({required int current, required int previous}) {
  if (previous == 0) return null;
  return ((current - previous) / previous * 100 + 0.5).floor();
}

/// 「較前 7 日 +12%」；null 時是 [dashboardUnset]（不掛期間詞，「較前 7 日 —」讀不出意思）。
String formatGrowth(int? percent) {
  if (percent == null) return dashboardUnset;
  final sign = percent > 0 ? '+' : '';
  return '較前 7 日 $sign$percent%';
}

String _monthDay(Date? date) {
  if (date == null || date.month == 0 || date.day == 0) return '';
  return '${date.month}/${date.day}';
}

/// 期間的顯示文字（臺北日曆日，含頭含尾），例如「9/30–10/6」；沒設或日期不完整時是 null。
String? periodLabel(DashboardPeriod? period) {
  if (period == null) return null;
  final first = _monthDay(period.hasFirstDate() ? period.firstDate : null);
  final last = _monthDay(period.hasLastDate() ? period.lastDate : null);
  if (first.isEmpty || last.isEmpty) return null;
  return '$first–$last';
}

/// 數字卡共用的期間說明（目前期間，近 7 日的指標都在同一段期間）；沒設期間時是 null。website 的 `buildStats` 在營收、商品瀏覽、
/// 新追蹤者都帶，不論那一塊有沒有值，App 的每張 7 日數字卡一致。
String? currentPeriodCaption(Dashboard? dashboard) =>
    dashboard != null && dashboard.hasCurrentPeriod()
    ? periodLabel(dashboard.currentPeriod)
    : null;

/// 千分位（`1234567` → `1,234,567`）。
String formatThousands(int value) {
  final digits = value.abs().toString();
  final out = StringBuffer(value < 0 ? '-' : '');
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) out.write(',');
    out.write(digits[i]);
  }
  return out.toString();
}

/// 「NT$ 1,234」。
String formatRevenue(int amount) => 'NT\$ ${formatThousands(amount)}';

/// 新追蹤者：有新增顯示 `+N`，沒有顯示 `0`（同 website）。
String formatNewFollowers(int count) => count > 0 ? '+$count' : '0';

/// 一張數字卡的內容：[value] 與 [change] 沒設時都是 [dashboardUnset]；[growth] 是用來決定顏色的百分比。
class DashboardStat {
  const DashboardStat({
    required this.value,
    required this.change,
    this.growth,
    this.caption,
  });

  /// 區塊沒設（或還沒讀到儀表板）：值與變化都是 [dashboardUnset]，不補 0；期間說明照帶。
  const DashboardStat.unset({this.caption})
    : value = dashboardUnset,
      change = dashboardUnset,
      growth = null;

  final String value;
  final String change;
  final int? growth;

  /// 數字所屬的期間；沒設期間時是 null。
  final String? caption;
}

DashboardStat _periodStat(
  Dashboard? dashboard,
  ({int current, int previous})? pair,
  String Function(int current) format,
) {
  final caption = currentPeriodCaption(dashboard);
  if (pair == null) return DashboardStat.unset(caption: caption);
  final growth = periodGrowthPercent(
    current: pair.current,
    previous: pair.previous,
  );
  return DashboardStat(
    value: format(pair.current),
    change: formatGrowth(growth),
    growth: growth,
    caption: caption,
  );
}

/// 近 7 日營收。
DashboardStat revenueStat(Dashboard? dashboard) {
  final revenue =
      dashboard != null && dashboard.hasSales() && dashboard.sales.hasRevenue()
      ? dashboard.sales.revenue
      : null;
  return _periodStat(
    dashboard,
    revenue == null
        ? null
        : (
            current: revenue.current.toInt(),
            previous: revenue.previous.toInt(),
          ),
    formatRevenue,
  );
}

/// 近 7 日訂單。
DashboardStat ordersStat(Dashboard? dashboard) {
  final orders =
      dashboard != null && dashboard.hasSales() && dashboard.sales.hasOrders()
      ? dashboard.sales.orders
      : null;
  return _periodStat(
    dashboard,
    orders == null
        ? null
        : (current: orders.current, previous: orders.previous),
    formatThousands,
  );
}

/// 近 7 日顧客。
DashboardStat customersStat(Dashboard? dashboard) {
  final customers =
      dashboard != null &&
          dashboard.hasSales() &&
          dashboard.sales.hasCustomers()
      ? dashboard.sales.customers
      : null;
  return _periodStat(
    dashboard,
    customers == null
        ? null
        : (current: customers.current, previous: customers.previous),
    formatThousands,
  );
}

/// 商品瀏覽。
DashboardStat productViewsStat(Dashboard? dashboard) {
  final views = dashboard != null && dashboard.hasProductViews()
      ? dashboard.productViews
      : null;
  return _periodStat(
    dashboard,
    views == null ? null : (current: views.current, previous: views.previous),
    formatThousands,
  );
}

/// 新追蹤者：`+N`，變化欄固定寫「近 7 日」（沒有比較值）；沒設時都是 [dashboardUnset]。
DashboardStat newFollowersStat(Dashboard? dashboard) {
  final followers =
      dashboard != null &&
          dashboard.hasFollowers() &&
          dashboard.followers.hasNewFollowers()
      ? dashboard.followers.newFollowers
      : null;
  final caption = currentPeriodCaption(dashboard);
  if (followers == null) return DashboardStat.unset(caption: caption);
  return DashboardStat(
    value: formatNewFollowers(followers.current),
    change: '近 7 日',
    caption: caption,
  );
}

/// 訂單狀態的目前快照；沒設（讀不到）時是 null。
OrderStatusCounts? orderStatusCountsOf(Dashboard? dashboard) =>
    dashboard != null && dashboard.hasOrderStatusCounts()
    ? dashboard.orderStatusCounts
    : null;

/// 訂單狀態的一個件數；訂單狀態讀不到時是 [dashboardUnset]，不是 0。
String orderCountText(
  OrderStatusCounts? counts,
  int Function(OrderStatusCounts counts) pick,
) => counts == null ? dashboardUnset : '${pick(counts)}';
