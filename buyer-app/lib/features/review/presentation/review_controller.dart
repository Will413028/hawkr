import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../data/review_model.dart';
import '../data/review_repository.dart';

part 'review_controller.g.dart';

@Riverpod(keepAlive: true)
class ProductReviewsController extends _$ProductReviewsController {
  @override
  Future<List<Review>> build(int skuId) async {
    ref.keepAlive();
    return ref.watch(reviewRepositoryProvider).getProductReviews(skuId);
  }

  Future<void> refresh() async {
    ref.invalidateSelf();
    await future;
  }
}

@riverpod
class AddReviewController extends _$AddReviewController {
  @override
  AsyncValue<void> build() => const AsyncData(null);

  Future<Review?> submit({
    required String orderNo,
    required int orderItemId,
    required int skuId,
    required int rating,
    required String content,
    List<String>? stagedTokens,
  }) async {
    state = const AsyncLoading();
    final result = await AsyncValue.guard(
      () => ref
          .read(reviewRepositoryProvider)
          .addReview(
            orderNo: orderNo,
            orderItemId: orderItemId,
            skuId: skuId,
            rating: rating,
            content: content,
            stagedTokens: stagedTokens,
          ),
    );
    state = result.when(
      data: (_) => const AsyncData(null),
      loading: () => const AsyncLoading(),
      error: AsyncError.new,
    );
    if (result is AsyncData) {
      ref.invalidate(productReviewsControllerProvider(skuId));
      return result.value;
    }
    return null;
  }
}
