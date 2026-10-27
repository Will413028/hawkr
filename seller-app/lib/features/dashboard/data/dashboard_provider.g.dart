// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'dashboard_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// [storeId] 這間店家的儀表板（`GetMyDashboard`）。`GetMyDashboard` 讀的是目前的店家，所以以店家 ID 為鍵：同一個帳號
/// 關店再開店（店家 ID 改變）也不會看到上一間店的數字，也不會被 30 秒的保鮮期略過重讀。
/// [currentDashboard]（keepAlive）在目前有店家期間一直監看這個 provider，所以它在這期間都保留（沒有畫面時
/// 也不釋放，讀到的值留給下次開啟）；換店家、換帳號、登出時才釋放（新的店家從讀取中開始，不先顯示上一間店的數字）。
///
/// 讀取失敗而且已經有值時，狀態是帶著上一次的值的 error（`AsyncValue.value` 仍是上一次的值）：畫面繼續顯示它。
/// 沒有輪詢，也不以 silent push 為依據：畫面在可見、回前景、收到訂單推播、下拉時用 [refresh] 重讀
/// （A3b 計畫第 4 刀的新鮮度決定）。四個區塊讀不到時伺服器不設，不是 0，所以這裡不做任何補值。
///
/// 每一次讀取都經過 [CoalescingRefresher]：第一次與依賴改變（例如 client 換了）的重建把自己那一次讀取登記成進行中的
/// 一輪，登記永遠指向最新的那次讀取；被取代的讀取不會改變狀態，也不會記成功（不影響保鮮期）。

@ProviderFor(StoreDashboard)
final storeDashboardProvider = StoreDashboardFamily._();

/// [storeId] 這間店家的儀表板（`GetMyDashboard`）。`GetMyDashboard` 讀的是目前的店家，所以以店家 ID 為鍵：同一個帳號
/// 關店再開店（店家 ID 改變）也不會看到上一間店的數字，也不會被 30 秒的保鮮期略過重讀。
/// [currentDashboard]（keepAlive）在目前有店家期間一直監看這個 provider，所以它在這期間都保留（沒有畫面時
/// 也不釋放，讀到的值留給下次開啟）；換店家、換帳號、登出時才釋放（新的店家從讀取中開始，不先顯示上一間店的數字）。
///
/// 讀取失敗而且已經有值時，狀態是帶著上一次的值的 error（`AsyncValue.value` 仍是上一次的值）：畫面繼續顯示它。
/// 沒有輪詢，也不以 silent push 為依據：畫面在可見、回前景、收到訂單推播、下拉時用 [refresh] 重讀
/// （A3b 計畫第 4 刀的新鮮度決定）。四個區塊讀不到時伺服器不設，不是 0，所以這裡不做任何補值。
///
/// 每一次讀取都經過 [CoalescingRefresher]：第一次與依賴改變（例如 client 換了）的重建把自己那一次讀取登記成進行中的
/// 一輪，登記永遠指向最新的那次讀取；被取代的讀取不會改變狀態，也不會記成功（不影響保鮮期）。
final class StoreDashboardProvider
    extends $AsyncNotifierProvider<StoreDashboard, Dashboard> {
  /// [storeId] 這間店家的儀表板（`GetMyDashboard`）。`GetMyDashboard` 讀的是目前的店家，所以以店家 ID 為鍵：同一個帳號
  /// 關店再開店（店家 ID 改變）也不會看到上一間店的數字，也不會被 30 秒的保鮮期略過重讀。
  /// [currentDashboard]（keepAlive）在目前有店家期間一直監看這個 provider，所以它在這期間都保留（沒有畫面時
  /// 也不釋放，讀到的值留給下次開啟）；換店家、換帳號、登出時才釋放（新的店家從讀取中開始，不先顯示上一間店的數字）。
  ///
  /// 讀取失敗而且已經有值時，狀態是帶著上一次的值的 error（`AsyncValue.value` 仍是上一次的值）：畫面繼續顯示它。
  /// 沒有輪詢，也不以 silent push 為依據：畫面在可見、回前景、收到訂單推播、下拉時用 [refresh] 重讀
  /// （A3b 計畫第 4 刀的新鮮度決定）。四個區塊讀不到時伺服器不設，不是 0，所以這裡不做任何補值。
  ///
  /// 每一次讀取都經過 [CoalescingRefresher]：第一次與依賴改變（例如 client 換了）的重建把自己那一次讀取登記成進行中的
  /// 一輪，登記永遠指向最新的那次讀取；被取代的讀取不會改變狀態，也不會記成功（不影響保鮮期）。
  StoreDashboardProvider._({
    required StoreDashboardFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'storeDashboardProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$storeDashboardHash();

  @override
  String toString() {
    return r'storeDashboardProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  StoreDashboard create() => StoreDashboard();

  @override
  bool operator ==(Object other) {
    return other is StoreDashboardProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storeDashboardHash() => r'afe057a69300dfe92ed882c538ef87059de096a9';

/// [storeId] 這間店家的儀表板（`GetMyDashboard`）。`GetMyDashboard` 讀的是目前的店家，所以以店家 ID 為鍵：同一個帳號
/// 關店再開店（店家 ID 改變）也不會看到上一間店的數字，也不會被 30 秒的保鮮期略過重讀。
/// [currentDashboard]（keepAlive）在目前有店家期間一直監看這個 provider，所以它在這期間都保留（沒有畫面時
/// 也不釋放，讀到的值留給下次開啟）；換店家、換帳號、登出時才釋放（新的店家從讀取中開始，不先顯示上一間店的數字）。
///
/// 讀取失敗而且已經有值時，狀態是帶著上一次的值的 error（`AsyncValue.value` 仍是上一次的值）：畫面繼續顯示它。
/// 沒有輪詢，也不以 silent push 為依據：畫面在可見、回前景、收到訂單推播、下拉時用 [refresh] 重讀
/// （A3b 計畫第 4 刀的新鮮度決定）。四個區塊讀不到時伺服器不設，不是 0，所以這裡不做任何補值。
///
/// 每一次讀取都經過 [CoalescingRefresher]：第一次與依賴改變（例如 client 換了）的重建把自己那一次讀取登記成進行中的
/// 一輪，登記永遠指向最新的那次讀取；被取代的讀取不會改變狀態，也不會記成功（不影響保鮮期）。

final class StoreDashboardFamily extends $Family
    with
        $ClassFamilyOverride<
          StoreDashboard,
          AsyncValue<Dashboard>,
          Dashboard,
          FutureOr<Dashboard>,
          String
        > {
  StoreDashboardFamily._()
    : super(
        retry: noRetry,
        name: r'storeDashboardProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [storeId] 這間店家的儀表板（`GetMyDashboard`）。`GetMyDashboard` 讀的是目前的店家，所以以店家 ID 為鍵：同一個帳號
  /// 關店再開店（店家 ID 改變）也不會看到上一間店的數字，也不會被 30 秒的保鮮期略過重讀。
  /// [currentDashboard]（keepAlive）在目前有店家期間一直監看這個 provider，所以它在這期間都保留（沒有畫面時
  /// 也不釋放，讀到的值留給下次開啟）；換店家、換帳號、登出時才釋放（新的店家從讀取中開始，不先顯示上一間店的數字）。
  ///
  /// 讀取失敗而且已經有值時，狀態是帶著上一次的值的 error（`AsyncValue.value` 仍是上一次的值）：畫面繼續顯示它。
  /// 沒有輪詢，也不以 silent push 為依據：畫面在可見、回前景、收到訂單推播、下拉時用 [refresh] 重讀
  /// （A3b 計畫第 4 刀的新鮮度決定）。四個區塊讀不到時伺服器不設，不是 0，所以這裡不做任何補值。
  ///
  /// 每一次讀取都經過 [CoalescingRefresher]：第一次與依賴改變（例如 client 換了）的重建把自己那一次讀取登記成進行中的
  /// 一輪，登記永遠指向最新的那次讀取；被取代的讀取不會改變狀態，也不會記成功（不影響保鮮期）。

  StoreDashboardProvider call(String storeId) =>
      StoreDashboardProvider._(argument: storeId, from: this);

  @override
  String toString() => r'storeDashboardProvider';
}

/// [storeId] 這間店家的儀表板（`GetMyDashboard`）。`GetMyDashboard` 讀的是目前的店家，所以以店家 ID 為鍵：同一個帳號
/// 關店再開店（店家 ID 改變）也不會看到上一間店的數字，也不會被 30 秒的保鮮期略過重讀。
/// [currentDashboard]（keepAlive）在目前有店家期間一直監看這個 provider，所以它在這期間都保留（沒有畫面時
/// 也不釋放，讀到的值留給下次開啟）；換店家、換帳號、登出時才釋放（新的店家從讀取中開始，不先顯示上一間店的數字）。
///
/// 讀取失敗而且已經有值時，狀態是帶著上一次的值的 error（`AsyncValue.value` 仍是上一次的值）：畫面繼續顯示它。
/// 沒有輪詢，也不以 silent push 為依據：畫面在可見、回前景、收到訂單推播、下拉時用 [refresh] 重讀
/// （A3b 計畫第 4 刀的新鮮度決定）。四個區塊讀不到時伺服器不設，不是 0，所以這裡不做任何補值。
///
/// 每一次讀取都經過 [CoalescingRefresher]：第一次與依賴改變（例如 client 換了）的重建把自己那一次讀取登記成進行中的
/// 一輪，登記永遠指向最新的那次讀取；被取代的讀取不會改變狀態，也不會記成功（不影響保鮮期）。

abstract class _$StoreDashboard extends $AsyncNotifier<Dashboard> {
  late final _$args = ref.$arg as String;
  String get storeId => _$args;

  FutureOr<Dashboard> build(String storeId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<Dashboard>, Dashboard>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<Dashboard>, Dashboard>,
              AsyncValue<Dashboard>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}

/// 目前店家的儀表板；沒有登入、還沒讀到店家或沒有店家時是 null（和值是 null 的 `AsyncData` 不同）。

@ProviderFor(currentDashboard)
final currentDashboardProvider = CurrentDashboardProvider._();

/// 目前店家的儀表板；沒有登入、還沒讀到店家或沒有店家時是 null（和值是 null 的 `AsyncData` 不同）。

final class CurrentDashboardProvider
    extends
        $FunctionalProvider<
          AsyncValue<Dashboard>?,
          AsyncValue<Dashboard>?,
          AsyncValue<Dashboard>?
        >
    with $Provider<AsyncValue<Dashboard>?> {
  /// 目前店家的儀表板；沒有登入、還沒讀到店家或沒有店家時是 null（和值是 null 的 `AsyncData` 不同）。
  CurrentDashboardProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentDashboardProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentDashboardHash();

  @$internal
  @override
  $ProviderElement<AsyncValue<Dashboard>?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AsyncValue<Dashboard>? create(Ref ref) {
    return currentDashboard(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<Dashboard>? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<Dashboard>?>(value),
    );
  }
}

String _$currentDashboardHash() => r'b8ed468d37ceb76a3ebdf6857f48e28ae02eda29';
