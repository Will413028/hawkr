import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/refresh/refresh_clock.dart';
import 'package:hawkr_seller/features/dashboard/data/dashboard_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/seller_status_refresh.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/dashboard_fixtures.dart';

/// 目前店家的 ID（測試用，取代 `GetMyStore` 的結果）；null 是沒有店家（登出、關店後）。
class _Store extends Notifier<String?> {
  @override
  String? build() => 'store-1';

  void switchTo(String? storeId) => state = storeId;
}

final _storeProvider = NotifierProvider<_Store, String?>(_Store.new);

/// 一個可控的 `GetMyDashboard`：計次、可失敗、可卡住，時鐘由測試推進。
class _Server {
  int calls = 0;
  int active = 0;
  int maxConcurrent = 0;
  Object? failure;
  final gates = <Completer<void>>[];
  var now = DateTime.utc(2026, 10, 12, 8);
  void advance(Duration d) => now = now.add(d);

  /// 第 N 次呼叫（從 1 起算）回的待出貨數，用來辨認讀到的是哪一輪。
  int pendingOf(int call) => call;

  /// [holdFirstStore] 為真時測試自己持有 store-1 的訂閱；釋放行為的測試不持有，只靠 [currentDashboardProvider]。
  ProviderContainer container({bool holdFirstStore = true}) {
    final container = ProviderContainer(
      overrides: [
        sellerStoreIdProvider.overrideWith((ref) => ref.watch(_storeProvider)),
        refreshClockProvider.overrideWithValue(() => now),
        // 每次依賴重建都是一個新的 client（和換掉 client 的情況一樣）。
        sellerClientProvider.overrideWith(
          (ref) => SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyDashboard, (
              _,
              _,
            ) async {
              final mine = ++calls;
              active++;
              if (active > maxConcurrent) maxConcurrent = active;
              if (gates.length >= mine) await gates[mine - 1].future;
              active--;
              final error = failure;
              if (error != null) throw error;
              return GetMyDashboardResponse(
                dashboard: dashboardFromJson(
                  fullDashboardJson(pendingShipment: pendingOf(mine)),
                ),
              );
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    if (holdFirstStore) {
      final sub = container.listen(
        storeDashboardProvider('store-1'),
        (_, _) {},
      );
      addTearDown(sub.close);
    }
    return container;
  }
}

int? _pending(ProviderContainer container) => container
    .read(storeDashboardProvider('store-1'))
    .value
    ?.orderStatusCounts
    .pendingShipmentCount;

void main() {
  test('第一次讀取成功；沒有輪詢，時間過去也不會再讀', () async {
    final server = _Server();
    final container = server.container();

    await container.read(storeDashboardProvider('store-1').future);
    expect(server.calls, 1);
    expect(_pending(container), 1);

    server.advance(const Duration(minutes: 30));
    await Future<void>.delayed(const Duration(milliseconds: 50));
    expect(server.calls, 1);
  });

  test('回應沒有儀表板：是失敗，不是空資料', () async {
    final container = ProviderContainer(
      overrides: [
        sellerStoreIdProvider.overrideWith((ref) => ref.watch(_storeProvider)),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder()
                .unary(
                  SellerAppService.getMyDashboard,
                  (_, _) => GetMyDashboardResponse(),
                )
                .build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(storeDashboardProvider('store-1'), (_, _) {});
    addTearDown(sub.close);

    await expectLater(
      container.read(storeDashboardProvider('store-1').future),
      throwsA(isA<StateError>()),
    );
    expect(container.read(storeDashboardProvider('store-1')).hasValue, isFalse);
  });

  group('因為成為可見而重讀（maxAge）', () {
    test('30 秒內讀過就不讀，超過才讀', () async {
      final server = _Server();
      final container = server.container();
      await container.read(storeDashboardProvider('store-1').future);
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );

      server.advance(const Duration(seconds: 29));
      expect(await notifier.refresh(maxAge: statusStaleTime), isTrue);
      expect(server.calls, 1);

      server.advance(const Duration(seconds: 1));
      expect(await notifier.refresh(maxAge: statusStaleTime), isTrue);
      expect(server.calls, 2);
      expect(_pending(container), 2);
    });

    test('回前景、下拉與重試不帶 maxAge：剛讀過也照讀', () async {
      final server = _Server();
      final container = server.container();
      await container.read(storeDashboardProvider('store-1').future);
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );

      expect(await notifier.refresh(), isTrue);
      expect(server.calls, 2);
    });

    test('第一次讀取進行中，畫面剛開啟的重讀共用它（一輪只讀一次）', () async {
      final server = _Server()..gates.add(Completer<void>());
      final container = server.container();
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );

      final visible = notifier.refresh(maxAge: statusStaleTime);
      final resumed = notifier.refresh();
      server.gates.first.complete();

      expect(await visible, isTrue);
      expect(await resumed, isTrue);
      expect(server.calls, 1);
      expect(server.maxConcurrent, 1);
    });

    test('第一次讀取失敗後，可見重讀不被 maxAge 擋下', () async {
      final server = _Server()
        ..failure = ConnectException(Code.unavailable, 'x');
      final container = server.container();
      await expectLater(
        container.read(storeDashboardProvider('store-1').future),
        throwsA(anything),
      );
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );

      server.failure = null;
      expect(await notifier.refresh(maxAge: statusStaleTime), isTrue);
      expect(server.calls, 2);
      expect(_pending(container), 2);
    });
  });

  group('同一輪只讀一次', () {
    test('進行中時的一般呼叫（回前景、下拉）共用那一輪', () async {
      final server = _Server();
      final container = server.container();
      await container.read(storeDashboardProvider('store-1').future);
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );
      server.gates
        ..add(Completer<void>()..complete())
        ..add(Completer<void>());

      final first = notifier.refresh();
      final second = notifier.refresh();
      final third = notifier.refresh(maxAge: statusStaleTime);
      await Future<void>.delayed(Duration.zero);
      server.gates.last.complete();

      expect(await Future.wait([first, second, third]), [true, true, true]);
      expect(server.calls, 2);
    });

    test('訂單推播（fresh）在進行中的那輪之後再讀一次，多個推播共用這一輪', () async {
      final server = _Server();
      final container = server.container();
      await container.read(storeDashboardProvider('store-1').future);
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );
      final gate = Completer<void>();
      server.gates
        ..add(Completer<void>()..complete())
        ..add(gate);

      final running = notifier.refresh();
      await Future<void>.delayed(Duration.zero);
      final pushA = notifier.refresh(fresh: true);
      final pushB = notifier.refresh(fresh: true);
      gate.complete();

      expect(await running, isTrue);
      expect(await pushA, isTrue);
      expect(await pushB, isTrue);
      expect(server.calls, 3);
      expect(server.maxConcurrent, 1);
      expect(_pending(container), 3);
    });
  });

  group('失敗', () {
    test('重讀失敗：回 false，畫面繼續顯示上一次成功的值', () async {
      final server = _Server();
      final container = server.container();
      await container.read(storeDashboardProvider('store-1').future);
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );

      server.failure = ConnectException(Code.unavailable, 'down');
      expect(await notifier.refresh(), isFalse);

      final state = container.read(storeDashboardProvider('store-1'));
      expect(state.hasError, isTrue);
      expect(state.value, isNotNull);
      expect(_pending(container), 1);
    });

    test('失敗之後的成功重讀恢復正常，下一輪不被失敗那輪卡住', () async {
      final server = _Server();
      final container = server.container();
      await container.read(storeDashboardProvider('store-1').future);
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );
      server.failure = ConnectException(Code.unavailable, 'down');
      await notifier.refresh();

      server.failure = null;
      expect(await notifier.refresh(), isTrue);
      expect(
        container.read(storeDashboardProvider('store-1')).hasError,
        isFalse,
      );
      expect(_pending(container), 3);
    });

    test('失敗那輪不算「讀過」：下一次可見重讀不會因為 30 秒內而跳過', () async {
      final server = _Server();
      final container = server.container();
      await container.read(storeDashboardProvider('store-1').future);
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );
      server.advance(const Duration(seconds: 40));
      server.failure = ConnectException(Code.unavailable, 'down');
      await notifier.refresh();

      server.failure = null;
      expect(await notifier.refresh(maxAge: statusStaleTime), isTrue);
      expect(server.calls, 3);
    });
  });

  group('依賴改變的重建', () {
    test('讀取進行中 client 換了：新的讀取取代舊的，舊的結果不改變狀態、不算讀過；一輪仍只有一個進行中', () async {
      final server = _Server()..gates.add(Completer<void>());
      final container = server.container();
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );
      await Future<void>.delayed(Duration.zero);
      expect(server.calls, 1);

      // 第 1 次讀取還卡著時依賴改變：重建出第 2 次讀取（沒有 gate，立刻完成）。
      container.invalidate(sellerClientProvider);
      await container.read(storeDashboardProvider('store-1').future);
      expect(server.calls, 2);
      expect(_pending(container), 2);
      // 第 2 次讀取剛完成：保鮮期內，可見重讀不讀。
      expect(
        await notifier.refresh(maxAge: const Duration(seconds: 1)),
        isTrue,
      );
      expect(server.calls, 2);
      server.advance(const Duration(minutes: 5));

      // 被取代的第 1 次讀取現在才回來：狀態不動、不算成功（保鮮期不重設）。
      server.gates.first.complete();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(_pending(container), 2);
      expect(server.calls, 2);
      // 保鮮期沒被重設，所以可見重讀照讀。
      expect(await notifier.refresh(maxAge: statusStaleTime), isTrue);
      expect(server.calls, 3);
    });

    test('進行中的讀取被取代後，呼叫共用最新的那一次，不另開讀取', () async {
      final server = _Server()
        ..gates.addAll([Completer<void>(), Completer<void>()]);
      final container = server.container();
      final notifier = container.read(
        storeDashboardProvider('store-1').notifier,
      );
      await Future<void>.delayed(Duration.zero);
      container.invalidate(sellerClientProvider);
      await Future<void>.delayed(Duration.zero);
      expect(server.calls, 2);

      final joined = notifier.refresh();
      // 舊的先回來也不會讓「進行中」結束。
      server.gates[0].complete();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      final again = notifier.refresh();
      expect(server.calls, 2);

      server.gates[1].complete();
      expect(await joined, isTrue);
      expect(await again, isTrue);
      expect(server.calls, 2);
      expect(server.maxConcurrent, 2);
      expect(_pending(container), 2);
    });
  });

  group('換店家與登出釋放儀表板', () {
    test('離開的店家釋放；回來的店家從讀取中開始，不先顯示上一間店的數字', () async {
      final server = _Server();
      final container = server.container(holdFirstStore: false);
      final current = container.listen(currentDashboardProvider, (_, _) {});
      addTearDown(current.close);
      await container.read(storeDashboardProvider('store-1').future);
      expect(_pending(container), 1);

      container.read(_storeProvider.notifier).switchTo('store-2');
      await container.read(storeDashboardProvider('store-2').future);
      await Future<void>.delayed(Duration.zero);
      expect(container.exists(storeDashboardProvider('store-1')), isFalse);

      final gate = Completer<void>();
      server.gates.add(gate);
      container.read(_storeProvider.notifier).switchTo('store-1');
      final back = container.read(currentDashboardProvider)!;
      expect(back.isLoading, isTrue);
      expect(back.value, isNull);
      expect(server.calls, 3);
      gate.complete();
    });

    test('同一個帳號關店再開店（店家 ID 改變）：新店家重新讀取，不顯示舊店家的數字，也不被保鮮期略過', () async {
      final server = _Server();
      final container = server.container(holdFirstStore: false);
      final current = container.listen(currentDashboardProvider, (_, _) {});
      addTearDown(current.close);
      await container.read(storeDashboardProvider('store-1').future);
      expect(_pending(container), 1);

      final notifier = container.read(_storeProvider.notifier);
      notifier.switchTo(null); // 關店
      await Future<void>.delayed(Duration.zero);
      expect(container.read(currentDashboardProvider), isNull);

      final gate = Completer<void>();
      server.gates.add(gate);
      notifier.switchTo('store-9'); // 開新店
      final fresh = container.read(currentDashboardProvider)!;
      expect(fresh.isLoading, isTrue);
      expect(fresh.value, isNull);
      expect(container.exists(storeDashboardProvider('store-1')), isFalse);
      gate.complete();
      await container.read(storeDashboardProvider('store-9').future);
      expect(server.calls, 2);
      expect(
        container
            .read(storeDashboardProvider('store-9'))
            .value
            ?.orderStatusCounts
            .pendingShipmentCount,
        2,
      );
    });

    test('登出（沒有店家）：目前店家的儀表板釋放', () async {
      final server = _Server();
      final container = server.container(holdFirstStore: false);
      final current = container.listen(currentDashboardProvider, (_, _) {});
      addTearDown(current.close);
      await container.read(storeDashboardProvider('store-1').future);

      container.read(_storeProvider.notifier).switchTo(null);
      await Future<void>.delayed(Duration.zero);
      expect(container.read(currentDashboardProvider), isNull);
      expect(container.exists(storeDashboardProvider('store-1')), isFalse);
    });
  });

  test('currentDashboard：沒有店家是 null', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(container.read(currentDashboardProvider), isNull);
  });
}
