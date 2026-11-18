import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/exceptions/app_exception.dart';
import '../data/social_content_repository.dart';
import '../data/social_model.dart';

part 'post_detail_controller.g.dart';

class PostDetailState {
  final Post post;
  const PostDetailState({required this.post});
}

/// 找不到不是暫時的失敗，自動重試只會讓畫面在轉圈中多等幾秒才顯示結果。
Duration? _retryUnlessNotFound(int retryCount, Object error) =>
    error is NotFoundException
    ? null
    : ProviderContainer.defaultRetry(retryCount, error);

@Riverpod(retry: _retryUnlessNotFound)
class PostDetailController extends _$PostDetailController {
  @override
  Future<PostDetailState> build(int postId) async {
    final post = await ref
        .watch(socialContentRepositoryProvider)
        .getPost(postId);
    return PostDetailState(post: post);
  }

  Future<void> likePost() async {
    final prev = state.value;
    if (prev == null) return;
    final wasLiked = prev.post.liked;
    final newCount = wasLiked
        ? (prev.post.likeCount > 0 ? prev.post.likeCount - 1 : 0)
        : prev.post.likeCount + 1;
    state = AsyncData(
      PostDetailState(
        post: prev.post.copyWith(liked: !wasLiked, likeCount: newCount),
      ),
    );
    try {
      await ref
          .read(socialContentRepositoryProvider)
          .setPostLiked(postId, liked: !wasLiked);
    } catch (_) {
      state = AsyncData(prev);
    }
  }
}
