// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_detail_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(PostDetailController)
final postDetailControllerProvider = PostDetailControllerFamily._();

final class PostDetailControllerProvider
    extends $AsyncNotifierProvider<PostDetailController, PostDetailState> {
  PostDetailControllerProvider._({
    required PostDetailControllerFamily super.from,
    required int super.argument,
  }) : super(
         retry: _retryUnlessNotFound,
         name: r'postDetailControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$postDetailControllerHash();

  @override
  String toString() {
    return r'postDetailControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  PostDetailController create() => PostDetailController();

  @override
  bool operator ==(Object other) {
    return other is PostDetailControllerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$postDetailControllerHash() =>
    r'e3416d1c46ff154ebdf6d77c526ab4dbe5a783cf';

final class PostDetailControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          PostDetailController,
          AsyncValue<PostDetailState>,
          PostDetailState,
          FutureOr<PostDetailState>,
          int
        > {
  PostDetailControllerFamily._()
    : super(
        retry: _retryUnlessNotFound,
        name: r'postDetailControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  PostDetailControllerProvider call(int postId) =>
      PostDetailControllerProvider._(argument: postId, from: this);

  @override
  String toString() => r'postDetailControllerProvider';
}

abstract class _$PostDetailController extends $AsyncNotifier<PostDetailState> {
  late final _$args = ref.$arg as int;
  int get postId => _$args;

  FutureOr<PostDetailState> build(int postId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<PostDetailState>, PostDetailState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<PostDetailState>, PostDetailState>,
              AsyncValue<PostDetailState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
