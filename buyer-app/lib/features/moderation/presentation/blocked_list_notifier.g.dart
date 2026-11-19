// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'blocked_list_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 封鎖名單：店家與買家兩段各自分頁。換觀看者整份重建；每個跨 await 的路徑回來先比對世代。

@ProviderFor(BlockedList)
final blockedListProvider = BlockedListProvider._();

/// 封鎖名單：店家與買家兩段各自分頁。換觀看者整份重建；每個跨 await 的路徑回來先比對世代。
final class BlockedListProvider
    extends $NotifierProvider<BlockedList, BlockedListState> {
  /// 封鎖名單：店家與買家兩段各自分頁。換觀看者整份重建；每個跨 await 的路徑回來先比對世代。
  BlockedListProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'blockedListProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$blockedListHash();

  @$internal
  @override
  BlockedList create() => BlockedList();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(BlockedListState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<BlockedListState>(value),
    );
  }
}

String _$blockedListHash() => r'ead927884d4a146f48463fab061eff0b6ec29ad4';

/// 封鎖名單：店家與買家兩段各自分頁。換觀看者整份重建；每個跨 await 的路徑回來先比對世代。

abstract class _$BlockedList extends $Notifier<BlockedListState> {
  BlockedListState build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<BlockedListState, BlockedListState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<BlockedListState, BlockedListState>,
              BlockedListState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
