// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_feed_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 貼文列表 controller，支援無限往下滑；[merchantId] 為 null 是全平台。
///
/// - 初始 build → 拿第一頁，貼文寫入實體存放處，列表只留 id
/// - [loadMore] → 用目前的 page token 拿下一頁，dedup 後 append；沒有 token 視為已到底
/// - 重新整理或觀看者換了：build 重跑，之前發出的讀取與分頁回來時被丟棄

@ProviderFor(PostFeedController)
final postFeedControllerProvider = PostFeedControllerFamily._();

/// 貼文列表 controller，支援無限往下滑；[merchantId] 為 null 是全平台。
///
/// - 初始 build → 拿第一頁，貼文寫入實體存放處，列表只留 id
/// - [loadMore] → 用目前的 page token 拿下一頁，dedup 後 append；沒有 token 視為已到底
/// - 重新整理或觀看者換了：build 重跑，之前發出的讀取與分頁回來時被丟棄
final class PostFeedControllerProvider
    extends $AsyncNotifierProvider<PostFeedController, PostListState> {
  /// 貼文列表 controller，支援無限往下滑；[merchantId] 為 null 是全平台。
  ///
  /// - 初始 build → 拿第一頁，貼文寫入實體存放處，列表只留 id
  /// - [loadMore] → 用目前的 page token 拿下一頁，dedup 後 append；沒有 token 視為已到底
  /// - 重新整理或觀看者換了：build 重跑，之前發出的讀取與分頁回來時被丟棄
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
    r'462226cc50f3c881f5a7c097afcc5a5e6ce8e3f7';

/// 貼文列表 controller，支援無限往下滑；[merchantId] 為 null 是全平台。
///
/// - 初始 build → 拿第一頁，貼文寫入實體存放處，列表只留 id
/// - [loadMore] → 用目前的 page token 拿下一頁，dedup 後 append；沒有 token 視為已到底
/// - 重新整理或觀看者換了：build 重跑，之前發出的讀取與分頁回來時被丟棄

final class PostFeedControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          PostFeedController,
          AsyncValue<PostListState>,
          PostListState,
          FutureOr<PostListState>,
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

  /// 貼文列表 controller，支援無限往下滑；[merchantId] 為 null 是全平台。
  ///
  /// - 初始 build → 拿第一頁，貼文寫入實體存放處，列表只留 id
  /// - [loadMore] → 用目前的 page token 拿下一頁，dedup 後 append；沒有 token 視為已到底
  /// - 重新整理或觀看者換了：build 重跑，之前發出的讀取與分頁回來時被丟棄

  PostFeedControllerProvider call({int? merchantId}) =>
      PostFeedControllerProvider._(argument: merchantId, from: this);

  @override
  String toString() => r'postFeedControllerProvider';
}

/// 貼文列表 controller，支援無限往下滑；[merchantId] 為 null 是全平台。
///
/// - 初始 build → 拿第一頁，貼文寫入實體存放處，列表只留 id
/// - [loadMore] → 用目前的 page token 拿下一頁，dedup 後 append；沒有 token 視為已到底
/// - 重新整理或觀看者換了：build 重跑，之前發出的讀取與分頁回來時被丟棄

abstract class _$PostFeedController extends $AsyncNotifier<PostListState> {
  late final _$args = ref.$arg as int?;
  int? get merchantId => _$args;

  FutureOr<PostListState> build({int? merchantId});
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<PostListState>, PostListState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<PostListState>, PostListState>,
              AsyncValue<PostListState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(merchantId: _$args));
  }
}
