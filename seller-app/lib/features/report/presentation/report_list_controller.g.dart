// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'report_list_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 我的檢舉列表 AsyncNotifier。

@ProviderFor(MyReportsController)
final myReportsControllerProvider = MyReportsControllerProvider._();

/// 我的檢舉列表 AsyncNotifier。
final class MyReportsControllerProvider
    extends $AsyncNotifierProvider<MyReportsController, MyReportsState> {
  /// 我的檢舉列表 AsyncNotifier。
  MyReportsControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'myReportsControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$myReportsControllerHash();

  @$internal
  @override
  MyReportsController create() => MyReportsController();
}

String _$myReportsControllerHash() =>
    r'09a8671fe1f48db6e8a34951213a7d111402709e';

/// 我的檢舉列表 AsyncNotifier。

abstract class _$MyReportsController extends $AsyncNotifier<MyReportsState> {
  FutureOr<MyReportsState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<MyReportsState>, MyReportsState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<MyReportsState>, MyReportsState>,
              AsyncValue<MyReportsState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

/// 店家封鎖的買家名單（`ListBlockedBuyers`，keyset 分頁）。
///
/// 每次從第一頁重載（build、refresh、silentRefresh）都換一個世代；跨過重載才回來的 loadMore 結果屬於舊世代，直接丟掉。
/// 進行中的解除封鎖以 [_pendingUnblock] 記錄，重載與分頁的結果都濾掉這些買家，直到那次 RPC 結束；
/// 成功的解除再記在 [_unblockedThisGeneration]，擋住重載前發出、之後才回來的分頁帶回舊資料。

@ProviderFor(BlockListController)
final blockListControllerProvider = BlockListControllerProvider._();

/// 店家封鎖的買家名單（`ListBlockedBuyers`，keyset 分頁）。
///
/// 每次從第一頁重載（build、refresh、silentRefresh）都換一個世代；跨過重載才回來的 loadMore 結果屬於舊世代，直接丟掉。
/// 進行中的解除封鎖以 [_pendingUnblock] 記錄，重載與分頁的結果都濾掉這些買家，直到那次 RPC 結束；
/// 成功的解除再記在 [_unblockedThisGeneration]，擋住重載前發出、之後才回來的分頁帶回舊資料。
final class BlockListControllerProvider
    extends $AsyncNotifierProvider<BlockListController, BlockListState> {
  /// 店家封鎖的買家名單（`ListBlockedBuyers`，keyset 分頁）。
  ///
  /// 每次從第一頁重載（build、refresh、silentRefresh）都換一個世代；跨過重載才回來的 loadMore 結果屬於舊世代，直接丟掉。
  /// 進行中的解除封鎖以 [_pendingUnblock] 記錄，重載與分頁的結果都濾掉這些買家，直到那次 RPC 結束；
  /// 成功的解除再記在 [_unblockedThisGeneration]，擋住重載前發出、之後才回來的分頁帶回舊資料。
  BlockListControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'blockListControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$blockListControllerHash();

  @$internal
  @override
  BlockListController create() => BlockListController();
}

String _$blockListControllerHash() =>
    r'a7f0029e1061414f2a305e9c6e7b3e9a5d12f3b8';

/// 店家封鎖的買家名單（`ListBlockedBuyers`，keyset 分頁）。
///
/// 每次從第一頁重載（build、refresh、silentRefresh）都換一個世代；跨過重載才回來的 loadMore 結果屬於舊世代，直接丟掉。
/// 進行中的解除封鎖以 [_pendingUnblock] 記錄，重載與分頁的結果都濾掉這些買家，直到那次 RPC 結束；
/// 成功的解除再記在 [_unblockedThisGeneration]，擋住重載前發出、之後才回來的分頁帶回舊資料。

abstract class _$BlockListController extends $AsyncNotifier<BlockListState> {
  FutureOr<BlockListState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<BlockListState>, BlockListState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<BlockListState>, BlockListState>,
              AsyncValue<BlockListState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
