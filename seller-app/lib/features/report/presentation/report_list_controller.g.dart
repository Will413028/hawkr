// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'report_list_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 我的檢舉列表 AsyncNotifier（backend-payuni#645）。

@ProviderFor(MyReportsController)
final myReportsControllerProvider = MyReportsControllerProvider._();

/// 我的檢舉列表 AsyncNotifier（backend-payuni#645）。
final class MyReportsControllerProvider
    extends $AsyncNotifierProvider<MyReportsController, MyReportsState> {
  /// 我的檢舉列表 AsyncNotifier（backend-payuni#645）。
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

/// 我的檢舉列表 AsyncNotifier（backend-payuni#645）。

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

/// 封鎖名單 AsyncNotifier（backend-payuni#645）。

@ProviderFor(BlockListController)
final blockListControllerProvider = BlockListControllerProvider._();

/// 封鎖名單 AsyncNotifier（backend-payuni#645）。
final class BlockListControllerProvider
    extends $AsyncNotifierProvider<BlockListController, BlockListState> {
  /// 封鎖名單 AsyncNotifier（backend-payuni#645）。
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
    r'dd4f5738ad3b36ed420e72e0e311291da1bbda97';

/// 封鎖名單 AsyncNotifier（backend-payuni#645）。

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
