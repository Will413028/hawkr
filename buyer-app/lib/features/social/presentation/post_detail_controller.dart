import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/exceptions/app_exception.dart';
import '../data/social_model.dart';
import '../data/social_repository.dart';

part 'post_detail_controller.g.dart';

class PostDetailState {
  final Post post;
  final List<PostComment> comments;
  const PostDetailState({required this.post, this.comments = const []});
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
    final repo = ref.watch(socialRepositoryProvider);
    final results = await Future.wait([
      repo.getPostDetail(postId),
      repo.getComments(postId),
    ]);
    return PostDetailState(
      post: results[0] as Post,
      comments: results[1] as List<PostComment>,
    );
  }

  Future<void> addComment(String content) async {
    await ref.read(socialRepositoryProvider).addComment(postId, content);
    ref.invalidateSelf();
  }

  Future<void> likePost() async {
    final prev = state.value;
    if (prev == null) return;
    // Optimistic update
    final wasLiked = prev.post.liked;
    final newCount = wasLiked
        ? (prev.post.likeCount > 0 ? prev.post.likeCount - 1 : 0)
        : prev.post.likeCount + 1;
    state = AsyncData(
      PostDetailState(
        post: prev.post.copyWith(liked: !wasLiked, likeCount: newCount),
        comments: prev.comments,
      ),
    );
    try {
      final resp = await ref.read(socialRepositoryProvider).likePost(postId);
      // If server returns liked state, correct if needed
      if (resp != null) {
        final data = resp['data'];
        final serverLiked = data is Map ? data['liked'] : resp['liked'];
        if (serverLiked is bool && serverLiked != !wasLiked) {
          final current = state.value;
          if (current != null) {
            state = AsyncData(
              PostDetailState(
                post: current.post.copyWith(liked: serverLiked),
                comments: current.comments,
              ),
            );
          }
        }
      }
    } catch (_) {
      // Revert on error
      state = AsyncData(prev);
    }
  }

  Future<void> likeComment(int commentId) async {
    await ref.read(socialRepositoryProvider).likeComment(commentId);
    ref.invalidateSelf();
  }
}
