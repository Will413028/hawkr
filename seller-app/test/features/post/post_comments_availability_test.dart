import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_seller/features/post/data/post_comments_availability.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:flutter_test/flutter_test.dart';

PostCommentsAvailability _of({
  StoreShare share = const StoreShareEnabled('s'),
  PostStatus publishStatus = PostStatus.POST_STATUS_PUBLISHED,
  bool hidden = false,
}) => postCommentsAvailability(
  share: share,
  status: publishStatus,
  hidden: hidden,
);

void main() {
  test('a published, visible post of a public store is available', () {
    expect(_of(), PostCommentsAvailability.available);
    expect(PostCommentsAvailability.available.message, isNull);
  });

  test('a store that cannot go public wins over the post state', () {
    expect(
      _of(
        share: const StoreShareDisabled(),
        publishStatus: PostStatus.POST_STATUS_DRAFT,
      ),
      PostCommentsAvailability.storeNotPublic,
    );
  });

  test('platform-hidden wins over a draft, an unpublished post and a '
      'non-public store', () {
    for (final status in [
      PostStatus.POST_STATUS_DRAFT,
      PostStatus.POST_STATUS_UNPUBLISHED,
      PostStatus.POST_STATUS_PUBLISHED,
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

  test('a post with a takedown measure is platform-hidden', () {
    const share = StoreShareEnabled('s');
    final hidden = Post(
      id: 1,
      content: 'c',
      status: PostStatus.POST_STATUS_DRAFT,
      takedownMeasureId: 'measure-1',
    );
    expect(
      postCommentsAvailabilityOfPost(share, hidden),
      PostCommentsAvailability.platformHidden,
    );
    expect(
      postCommentsAvailabilityOfPost(
        share,
        hidden.copyWith(takedownMeasureId: null),
      ),
      PostCommentsAvailability.notPublished,
    );
  });

  test('draft and self-unpublished are not published', () {
    expect(
      _of(publishStatus: PostStatus.POST_STATUS_DRAFT),
      PostCommentsAvailability.notPublished,
    );
    expect(
      _of(publishStatus: PostStatus.POST_STATUS_UNPUBLISHED),
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
    expect(
      _of(publishStatus: PostStatus.POST_STATUS_UNSPECIFIED),
      PostCommentsAvailability.available,
    );
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
