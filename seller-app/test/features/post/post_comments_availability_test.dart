import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/features/post/data/post_comments_availability.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:flutter_test/flutter_test.dart';

PostCommentsAvailability _of({
  StoreShare share = const StoreShareEnabled('s'),
  String? publishStatus = '4402',
  bool hidden = false,
}) => postCommentsAvailability(
  share: share,
  publishStatus: publishStatus,
  hidden: hidden,
);

void main() {
  test('a published, visible post of a public store is available', () {
    expect(_of(), PostCommentsAvailability.available);
    expect(PostCommentsAvailability.available.message, isNull);
  });

  test('a store that cannot go public wins over the post state', () {
    expect(
      _of(share: const StoreShareDisabled(), publishStatus: '4401'),
      PostCommentsAvailability.storeNotPublic,
    );
  });

  test('platform-hidden wins over a draft, an unpublished post and a '
      'non-public store', () {
    for (final status in [
      PostPublishStatus.draft.value,
      PostPublishStatus.unpublished.value,
      PostPublishStatus.published.value,
    ]) {
      for (final share in const <StoreShare>[
        StoreShareEnabled('s'),
        StoreShareDisabled(),
        StoreShareUnknown(),
      ]) {
        expect(
          _of(share: share, publishStatus: status, hidden: true),
          PostCommentsAvailability.platformHidden,
        );
      }
    }
  });

  test('the Post and map variants use the same hidden rule', () {
    const share = StoreShareEnabled('s');
    final hidden = Post(
      id: 1,
      sellerId: 7,
      contentType: 'IMAGE',
      content: 'c',
      publishStatus: '4401',
      hiddenAt: 1760000000,
    );
    expect(
      postCommentsAvailabilityOfPost(share, hidden),
      PostCommentsAvailability.platformHidden,
    );
    expect(
      postCommentsAvailabilityOfMap(share, {
        'publishStatus': '4401',
        'hiddenAt': 1760000000,
      }),
      PostCommentsAvailability.platformHidden,
    );
    expect(
      postCommentsAvailabilityOfPost(share, hidden.copyWith(hiddenAt: null)),
      PostCommentsAvailability.notPublished,
    );
  });

  test('draft and self-unpublished are not published', () {
    expect(
      _of(publishStatus: PostPublishStatus.draft.value),
      PostCommentsAvailability.notPublished,
    );
    expect(
      _of(publishStatus: PostPublishStatus.unpublished.value),
      PostCommentsAvailability.notPublished,
    );
  });

  test('a published post hidden by the platform is platform-hidden', () {
    expect(_of(hidden: true), PostCommentsAvailability.platformHidden);
  });

  test('an unknown store or missing publish status does not conclude', () {
    expect(
      _of(share: const StoreShareUnknown()),
      PostCommentsAvailability.available,
    );
    expect(_of(publishStatus: null), PostCommentsAvailability.available);
  });

  test('messages are the approved copy', () {
    expect(
      PostCommentsAvailability.storeNotPublic.message,
      '店家公開後，買家的留言會顯示在這裡。',
    );
    expect(PostCommentsAvailability.notPublished.message, '貼文發布後，買家的留言會顯示在這裡。');
    expect(
      PostCommentsAvailability.platformHidden.message,
      '這則貼文已被平台下架，買家看不到留言。',
    );
  });
}
