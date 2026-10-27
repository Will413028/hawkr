// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(PostList)
final postListProvider = PostListProvider._();

final class PostListProvider
    extends $AsyncNotifierProvider<PostList, List<Post>> {
  PostListProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'postListProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$postListHash();

  @$internal
  @override
  PostList create() => PostList();
}

String _$postListHash() => r'01e6e8b14c819020e276646b6de8601f5bcb0e1a';

abstract class _$PostList extends $AsyncNotifier<List<Post>> {
  FutureOr<List<Post>> build();
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
    element.handleCreate(ref, build);
  }
}
