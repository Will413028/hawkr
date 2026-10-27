// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_feed_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 貼文 feed controller，支援 TikTok-style 無限往下滑。
///
/// 規則：
/// - 初始 build → 拿 page 1（20 筆），cache 在 state
/// - [loadMore] → 拿 page+1，dedup by id 後 append；< pageSize 視為已到底
/// - [likePost] → optimistic update 直接改 state，不 invalidateSelf 避免重置已載入頁

@ProviderFor(PostFeedController)
final postFeedControllerProvider = PostFeedControllerFamily._();

/// 貼文 feed controller，支援 TikTok-style 無限往下滑。
///
/// 規則：
/// - 初始 build → 拿 page 1（20 筆），cache 在 state
/// - [loadMore] → 拿 page+1，dedup by id 後 append；< pageSize 視為已到底
/// - [likePost] → optimistic update 直接改 state，不 invalidateSelf 避免重置已載入頁
final class PostFeedControllerProvider
    extends $AsyncNotifierProvider<PostFeedController, List<Post>> {
  /// 貼文 feed controller，支援 TikTok-style 無限往下滑。
  ///
  /// 規則：
  /// - 初始 build → 拿 page 1（20 筆），cache 在 state
  /// - [loadMore] → 拿 page+1，dedup by id 後 append；< pageSize 視為已到底
  /// - [likePost] → optimistic update 直接改 state，不 invalidateSelf 避免重置已載入頁
  PostFeedControllerProvider._({
    required PostFeedControllerFamily super.from,
    required int? super.argument,
  }) : super(
         retry: null,
         name: r'postFeedControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$postFeedControllerHash();

  @override
  String toString() {
    return r'postFeedControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  PostFeedController create() => PostFeedController();

  @override
  bool operator ==(Object other) {
    return other is PostFeedControllerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$postFeedControllerHash() =>
    r'7d9bb1dac538356c066eb6a8d876067941af6732';

/// 貼文 feed controller，支援 TikTok-style 無限往下滑。
///
/// 規則：
/// - 初始 build → 拿 page 1（20 筆），cache 在 state
/// - [loadMore] → 拿 page+1，dedup by id 後 append；< pageSize 視為已到底
/// - [likePost] → optimistic update 直接改 state，不 invalidateSelf 避免重置已載入頁

final class PostFeedControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          PostFeedController,
          AsyncValue<List<Post>>,
          List<Post>,
          FutureOr<List<Post>>,
          int?
        > {
  PostFeedControllerFamily._()
    : super(
        retry: null,
        name: r'postFeedControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 貼文 feed controller，支援 TikTok-style 無限往下滑。
  ///
  /// 規則：
  /// - 初始 build → 拿 page 1（20 筆），cache 在 state
  /// - [loadMore] → 拿 page+1，dedup by id 後 append；< pageSize 視為已到底
  /// - [likePost] → optimistic update 直接改 state，不 invalidateSelf 避免重置已載入頁

  PostFeedControllerProvider call({int? merchantId}) =>
      PostFeedControllerProvider._(argument: merchantId, from: this);

  @override
  String toString() => r'postFeedControllerProvider';
}

/// 貼文 feed controller，支援 TikTok-style 無限往下滑。
///
/// 規則：
/// - 初始 build → 拿 page 1（20 筆），cache 在 state
/// - [loadMore] → 拿 page+1，dedup by id 後 append；< pageSize 視為已到底
/// - [likePost] → optimistic update 直接改 state，不 invalidateSelf 避免重置已載入頁

abstract class _$PostFeedController extends $AsyncNotifier<List<Post>> {
  late final _$args = ref.$arg as int?;
  int? get merchantId => _$args;

  FutureOr<List<Post>> build({int? merchantId});
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<Post>>, List<Post>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<List<Post>>, List<Post>>,
              AsyncValue<List<Post>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(merchantId: _$args));
  }
}
