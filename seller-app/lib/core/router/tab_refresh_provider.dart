import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'tab_refresh_provider.g.dart';

/// Dashboard / 通知若要直接打開「配送中」並套用售後處理篩選，使用這個特殊 request。
const int ordersAfterSaleInTransitTabRequest = -1001;

/// Counter provider — Shell scaffold 在切換到 Category tab 時遞增，
/// PageScreen 透過 ref.listen 偵測變化後觸發 banner 刷新。
@riverpod
class PageTabRefresh extends _$PageTabRefresh {
  @override
  int build() => 0;

  void increment() => state++;
}

/// Counter provider — Shell scaffold 在切換到 Settings tab 時遞增，
/// ProfileScreen 透過 ref.listen 偵測變化後觸發認證狀態刷新。
@riverpod
class SettingsTabRefresh extends _$SettingsTabRefresh {
  @override
  int build() => 0;

  void increment() => state++;
}

/// Counter provider — Shell scaffold 在切換到 Orders tab 時遞增，
/// OrderListScreen 透過 ref.listen 偵測變化後重新拉取最新訂單。
@riverpod
class OrdersTabRefresh extends _$OrdersTabRefresh {
  @override
  int build() => 0;

  void increment() => state++;
}

/// 用於從 Dashboard / 通知導航到訂單 tab 時指定初始 tab index。
/// 設定後由 OrderListScreen 消費並重置為 null。
@riverpod
class OrdersInitialTab extends _$OrdersInitialTab {
  @override
  int? build() => null;

  void request(int tabIndex) => state = tabIndex;
  void requestAfterSaleInTransit() =>
      state = ordersAfterSaleInTransitTabRequest;
  void consume() => state = null;
}
