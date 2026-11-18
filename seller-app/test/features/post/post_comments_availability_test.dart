import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_seller/features/post/data/post_comments_availability.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:flutter_test/flutter_test.dart';

PostCommentsNotice? _of({
  StoreShare share = const StoreShareEnabled('s'),
  PostStatus publishStatus = PostStatus.POST_STATUS_PUBLISHED,
  bool hidden = false,
}) => postCommentsNotice(share: share, status: publishStatus, hidden: hidden);

void main() {
  test('a published, visible post of a public store has no notice', () {
    expect(_of(), isNull);
  });

  test('a store that cannot go public wins over the post state', () {
    expect(
      _of(
        share: const StoreShareDisabled(),
        publishStatus: PostStatus.POST_STATUS_DRAFT,
      ),
      PostCommentsNotice.storeNotPublic,
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
          PostCommentsNotice.platformHidden,
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
      postCommentsNoticeOfPost(share, hidden),
      PostCommentsNotice.platformHidden,
    );
    expect(
      postCommentsNoticeOfPost(share, hidden.copyWith(takedownMeasureId: null)),
      PostCommentsNotice.notPublished,
    );
  });

  test('draft and self-unpublished are not published', () {
    expect(
      _of(publishStatus: PostStatus.POST_STATUS_DRAFT),
      PostCommentsNotice.notPublished,
    );
    expect(
      _of(publishStatus: PostStatus.POST_STATUS_UNPUBLISHED),
      PostCommentsNotice.notPublished,
    );
  });

  test('a published post hidden by the platform is platform-hidden', () {
    expect(_of(hidden: true), PostCommentsNotice.platformHidden);
  });

  test('an unknown store or missing publish status does not conclude', () {
    expect(_of(share: const StoreShareUnknown()), isNull);
    expect(_of(publishStatus: PostStatus.POST_STATUS_UNSPECIFIED), isNull);
  });

  test('messages are the approved copy', () {
    expect(PostCommentsNotice.storeNotPublic.message, '店家目前不公開，買家看不到這些留言。');
    expect(PostCommentsNotice.notPublished.message, '這則貼文已下架，買家看不到這些留言。');
    expect(PostCommentsNotice.platformHidden.message, '這則貼文已被平台下架，買家看不到留言。');
  });
}
