// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_entity_store.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postSnapshotProvider`（列表、預覽卡）與 `postEntityProvider`（詳情）讀這裡的同一份。
///
/// 換觀看者時整份重建清空。伺服器給的值存成基準；讚是集中的 mutation，進行中的意圖以每個 id 一份的
/// 樂觀疊加層蓋在基準上（[PostEntities.posts] 是疊加後的值）：列表或詳情讀到新資料只更新基準、不抹掉疊加層；
/// 請求成功以基準加意圖為準，失敗只丟掉疊加層，不把舊快照寫回較新的基準。
/// 實體只在被引用時留著：列表以 [setListed] 登記持有的 id，畫面以 [retain] 監看；
/// 兩者都沒有之後，等 [postEntityGcTimeProvider] 的時間才移除，期間重新被引用就保留。

@ProviderFor(PostEntityStore)
final postEntityStoreProvider = PostEntityStoreProvider._();

/// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postSnapshotProvider`（列表、預覽卡）與 `postEntityProvider`（詳情）讀這裡的同一份。
///
/// 換觀看者時整份重建清空。伺服器給的值存成基準；讚是集中的 mutation，進行中的意圖以每個 id 一份的
/// 樂觀疊加層蓋在基準上（[PostEntities.posts] 是疊加後的值）：列表或詳情讀到新資料只更新基準、不抹掉疊加層；
/// 請求成功以基準加意圖為準，失敗只丟掉疊加層，不把舊快照寫回較新的基準。
/// 實體只在被引用時留著：列表以 [setListed] 登記持有的 id，畫面以 [retain] 監看；
/// 兩者都沒有之後，等 [postEntityGcTimeProvider] 的時間才移除，期間重新被引用就保留。
final class PostEntityStoreProvider
    extends $NotifierProvider<PostEntityStore, PostEntities> {
  /// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postSnapshotProvider`（列表、預覽卡）與 `postEntityProvider`（詳情）讀這裡的同一份。
  ///
  /// 換觀看者時整份重建清空。伺服器給的值存成基準；讚是集中的 mutation，進行中的意圖以每個 id 一份的
  /// 樂觀疊加層蓋在基準上（[PostEntities.posts] 是疊加後的值）：列表或詳情讀到新資料只更新基準、不抹掉疊加層；
  /// 請求成功以基準加意圖為準，失敗只丟掉疊加層，不把舊快照寫回較新的基準。
  /// 實體只在被引用時留著：列表以 [setListed] 登記持有的 id，畫面以 [retain] 監看；
  /// 兩者都沒有之後，等 [postEntityGcTimeProvider] 的時間才移除，期間重新被引用就保留。
  PostEntityStoreProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'postEntityStoreProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$postEntityStoreHash();

  @$internal
  @override
  PostEntityStore create() => PostEntityStore();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PostEntities value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PostEntities>(value),
    );
  }
}

String _$postEntityStoreHash() => r'cc2fb9d159d67341d78b001e464fb7d254eb3a78';

/// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postSnapshotProvider`（列表、預覽卡）與 `postEntityProvider`（詳情）讀這裡的同一份。
///
/// 換觀看者時整份重建清空。伺服器給的值存成基準；讚是集中的 mutation，進行中的意圖以每個 id 一份的
/// 樂觀疊加層蓋在基準上（[PostEntities.posts] 是疊加後的值）：列表或詳情讀到新資料只更新基準、不抹掉疊加層；
/// 請求成功以基準加意圖為準，失敗只丟掉疊加層，不把舊快照寫回較新的基準。
/// 實體只在被引用時留著：列表以 [setListed] 登記持有的 id，畫面以 [retain] 監看；
/// 兩者都沒有之後，等 [postEntityGcTimeProvider] 的時間才移除，期間重新被引用就保留。

abstract class _$PostEntityStore extends $Notifier<PostEntities> {
  PostEntities build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<PostEntities, PostEntities>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<PostEntities, PostEntities>,
              PostEntities,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
