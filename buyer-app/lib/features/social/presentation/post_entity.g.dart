// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_entity.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 一則貼文的實體（讚、讚數、留言數、`viewer_*`）。實體存放處已有就直接給，沒有才讀 `GetPost`；
/// 之後跟著存放處更新，所以列表、詳情、預覽卡讀到的是同一份。

@ProviderFor(PostEntity)
final postEntityProvider = PostEntityFamily._();

/// 一則貼文的實體（讚、讚數、留言數、`viewer_*`）。實體存放處已有就直接給，沒有才讀 `GetPost`；
/// 之後跟著存放處更新，所以列表、詳情、預覽卡讀到的是同一份。
final class PostEntityProvider
    extends $AsyncNotifierProvider<PostEntity, Post> {
  /// 一則貼文的實體（讚、讚數、留言數、`viewer_*`）。實體存放處已有就直接給，沒有才讀 `GetPost`；
  /// 之後跟著存放處更新，所以列表、詳情、預覽卡讀到的是同一份。
  PostEntityProvider._({
    required PostEntityFamily super.from,
    required int super.argument,
  }) : super(
         retry: _retryUnlessNotFound,
         name: r'postEntityProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$postEntityHash();

  @override
  String toString() {
    return r'postEntityProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  PostEntity create() => PostEntity();

  @override
  bool operator ==(Object other) {
    return other is PostEntityProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$postEntityHash() => r'c3fb8de2e0b9b9602a8f3bc035946236a914b64a';

/// 一則貼文的實體（讚、讚數、留言數、`viewer_*`）。實體存放處已有就直接給，沒有才讀 `GetPost`；
/// 之後跟著存放處更新，所以列表、詳情、預覽卡讀到的是同一份。

final class PostEntityFamily extends $Family
    with
        $ClassFamilyOverride<
          PostEntity,
          AsyncValue<Post>,
          Post,
          FutureOr<Post>,
          int
        > {
  PostEntityFamily._()
    : super(
        retry: _retryUnlessNotFound,
        name: r'postEntityProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 一則貼文的實體（讚、讚數、留言數、`viewer_*`）。實體存放處已有就直接給，沒有才讀 `GetPost`；
  /// 之後跟著存放處更新，所以列表、詳情、預覽卡讀到的是同一份。

  PostEntityProvider call(int postId) =>
      PostEntityProvider._(argument: postId, from: this);

  @override
  String toString() => r'postEntityProvider';
}

/// 一則貼文的實體（讚、讚數、留言數、`viewer_*`）。實體存放處已有就直接給，沒有才讀 `GetPost`；
/// 之後跟著存放處更新，所以列表、詳情、預覽卡讀到的是同一份。

abstract class _$PostEntity extends $AsyncNotifier<Post> {
  late final _$args = ref.$arg as int;
  int get postId => _$args;

  FutureOr<Post> build(int postId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<Post>, Post>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<Post>, Post>,
              AsyncValue<Post>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}

/// 列表項目與預覽卡讀的快照：只看實體存放處已有的貼文，不發請求。
/// 觀看者換了、存放處清空時是 null，畫面據此收掉；需要補讀的詳情用 [postEntityProvider]。

@ProviderFor(postSnapshot)
final postSnapshotProvider = PostSnapshotFamily._();

/// 列表項目與預覽卡讀的快照：只看實體存放處已有的貼文，不發請求。
/// 觀看者換了、存放處清空時是 null，畫面據此收掉；需要補讀的詳情用 [postEntityProvider]。

final class PostSnapshotProvider
    extends $FunctionalProvider<Post?, Post?, Post?>
    with $Provider<Post?> {
  /// 列表項目與預覽卡讀的快照：只看實體存放處已有的貼文，不發請求。
  /// 觀看者換了、存放處清空時是 null，畫面據此收掉；需要補讀的詳情用 [postEntityProvider]。
  PostSnapshotProvider._({
    required PostSnapshotFamily super.from,
    required int super.argument,
  }) : super(
         retry: null,
         name: r'postSnapshotProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$postSnapshotHash();

  @override
  String toString() {
    return r'postSnapshotProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<Post?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  Post? create(Ref ref) {
    final argument = this.argument as int;
    return postSnapshot(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Post? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Post?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is PostSnapshotProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$postSnapshotHash() => r'7cd6e57a7c1924b75c5df35ba1b58133abceda86';

/// 列表項目與預覽卡讀的快照：只看實體存放處已有的貼文，不發請求。
/// 觀看者換了、存放處清空時是 null，畫面據此收掉；需要補讀的詳情用 [postEntityProvider]。

final class PostSnapshotFamily extends $Family
    with $FunctionalFamilyOverride<Post?, int> {
  PostSnapshotFamily._()
    : super(
        retry: null,
        name: r'postSnapshotProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 列表項目與預覽卡讀的快照：只看實體存放處已有的貼文，不發請求。
  /// 觀看者換了、存放處清空時是 null，畫面據此收掉；需要補讀的詳情用 [postEntityProvider]。

  PostSnapshotProvider call(int postId) =>
      PostSnapshotProvider._(argument: postId, from: this);

  @override
  String toString() => r'postSnapshotProvider';
}
