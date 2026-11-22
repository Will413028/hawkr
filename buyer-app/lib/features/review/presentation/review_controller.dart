import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/review_repository.dart';

part 'review_controller.g.dart';

@riverpod
class AddReviewController extends _$AddReviewController {
  @override
  AsyncValue<void> build() => const AsyncData(null);

  Future<pb.Review?> submit({
    required String orderNo,
    required String orderItemId,
    required int rating,
    required String content,
    List<String> stagedTokens = const [],
  }) async {
    state = const AsyncLoading();
    final result = await AsyncValue.guard(
      () => ref
          .read(reviewRepositoryProvider)
          .createReview(
            orderNo: orderNo,
            orderItemId: orderItemId,
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
    return result.value;
  }
}
