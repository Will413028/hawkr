// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_entity_store.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postEntityProvider` 讀這裡的同一份。
///
/// 換觀看者時整份重建清空；讚是集中的 mutation（樂觀、以 id 回滾、世代檢查）。

@ProviderFor(PostEntityStore)
final postEntityStoreProvider = PostEntityStoreProvider._();

/// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postEntityProvider` 讀這裡的同一份。
///
/// 換觀看者時整份重建清空；讚是集中的 mutation（樂觀、以 id 回滾、世代檢查）。
final class PostEntityStoreProvider
    extends $NotifierProvider<PostEntityStore, PostEntities> {
  /// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postEntityProvider` 讀這裡的同一份。
  ///
  /// 換觀看者時整份重建清空；讚是集中的 mutation（樂觀、以 id 回滾、世代檢查）。
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

String _$postEntityStoreHash() => r'd6e6c9c1cefe34bf07eb646eea895d2fa50be142';

/// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postEntityProvider` 讀這裡的同一份。
///
/// 換觀看者時整份重建清空；讚是集中的 mutation（樂觀、以 id 回滾、世代檢查）。

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
