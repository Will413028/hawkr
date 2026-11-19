import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart' as common;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';

PostRepository _repository(FakeTransportBuilder transport) {
  final container = ProviderContainer();
  addTearDown(container.dispose);
  return PostRepository(
    Dio(),
    container.read(Provider<Ref>((ref) => ref)),
    SocialAppServiceClient(transport.build()),
    SocialPublicServiceClient(transport.build()),
  );
}

pb.Post _proto({
  String id = '42',
  PostStatus status = PostStatus.POST_STATUS_PUBLISHED,
  String? takedownMeasureId,
}) =>
    pb.Post(
      id: id,
      etag: 'etag-$id',
      body: '貼文內容',
      status: status,
      url: 'https://hawkr.example/p/$id',
      image: common.Image(uriTemplate: 'https://m.test/i/{preset}/posts/a.jpg'),
      takedownMeasureId: takedownMeasureId,
      product: pb.PostProduct(
        id: '900',
        name: '蘋果',
        image: common.Image(uriTemplate: 'https://m.test/i/{preset}/p/9.jpg'),
      )..mergeFromProto3Json({'minPrice': '120'}),
    )..mergeFromProto3Json({
      'likeCount': '7',
      'commentCount': '3',
      'shareCount': '2',
    });

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  test('maps the proto Post fields the screens show', () async {
    final repository = _repository(
      FakeTransportBuilder()
          .unary(
            SocialAppService.listMyPosts,
            (_, _) => pb.ListMyPostsResponse(
              posts: [
                _proto(takedownMeasureId: 'measure-1'),
                _proto(id: '43', status: PostStatus.POST_STATUS_DRAFT),
              ],
            ),
          )
          .unary(
            SocialAppService.listPendingSchedules,
            (_, _) => pb.ListPendingSchedulesResponse(
              schedules: [
                pb.PostSchedule(
                  postId: '43',
                  publishTime: Timestamp.fromDateTime(
                    DateTime.utc(2026, 12, 1, 3),
                  ),
                ),
              ],
            ),
          ),
    );

    final posts = (await repository.listMyPostsPage()).posts;

    final hidden = posts.first;
    expect(hidden.id, 42);
    expect(hidden.etag, 'etag-42');
    expect(hidden.status, PostStatus.POST_STATUS_PUBLISHED);
    expect(hidden.isHidden, isTrue);
    expect(hidden.likeCount, 7);
    expect(hidden.commentCount, 3);
    expect(hidden.shareCount, 2);
    expect(hidden.url, 'https://hawkr.example/p/42');
    expect(hidden.imageUrl, 'https://m.test/i/original/posts/a.jpg');
    expect(
      hidden.linkedProduct,
      const PostLinkedProduct(
        id: '900',
        name: '蘋果',
        price: 120,
        imageUrl: 'http://localhost:8080/i/original/p/9.jpg',
      ),
    );
    expect(hidden.scheduledPublishAt, isNull);
    final draft = posts.last;
    expect(draft.isHidden, isFalse);
    expect(draft.status, PostStatus.POST_STATUS_DRAFT);
    expect(draft.scheduledPublishAt, DateTime.utc(2026, 12, 1, 3));
  });

  test('listMyPosts follows page tokens to the end', () async {
    final tokens = <String>[];
    var scheduleReads = 0;
    final repository = _repository(
      FakeTransportBuilder()
          .unary(SocialAppService.listMyPosts, (request, _) {
            tokens.add(request.pageToken);
            return request.pageToken.isEmpty
                ? pb.ListMyPostsResponse(
                    posts: [_proto()],
                    nextPageToken: 'next',
                  )
                : pb.ListMyPostsResponse(posts: [_proto(id: '43')]);
          })
          .unary(SocialAppService.listPendingSchedules, (_, _) {
            scheduleReads++;
            return pb.ListPendingSchedulesResponse();
          }),
    );

    final posts = await repository.listMyPosts();

    expect(posts.map((p) => p.id), [42, 43]);
    expect(tokens, ['', 'next']);
    expect(scheduleReads, 1, reason: 'schedules are read once per list load');
  });

  test('R2-1 list failure keeps the original ConnectException', () async {
    final repository = _repository(
      FakeTransportBuilder()
          .unary(
            SocialAppService.listMyPosts,
            (_, _) => throw ConnectException(Code.unauthenticated, 'login'),
          )
          .unary(
            SocialAppService.listPendingSchedules,
            (_, _) => pb.ListPendingSchedulesResponse(),
          ),
    );

    await expectLater(
      repository.listMyPostsPage(),
      throwsA(
        isA<ConnectException>().having(
          (e) => e.code,
          'code',
          Code.unauthenticated,
        ),
      ),
    );
  });

  test('publish and unpublish send the post etag', () async {
    final published = <pb.PublishPostRequest>[];
    final unpublished = <pb.UnpublishPostRequest>[];
    final repository = _repository(
      FakeTransportBuilder()
          .unary(SocialAppService.publishPost, (request, _) {
            published.add(request);
            return pb.PublishPostResponse();
          })
          .unary(SocialAppService.unpublishPost, (request, _) {
            unpublished.add(request);
            return pb.UnpublishPostResponse();
          }),
    );

    await repository.publishPostOrThrow(42, etag: 'e1');
    await repository.unpublishPostOrThrow(42, etag: 'e2');

    expect(published.single.postId, '42');
    expect(published.single.etag, 'e1');
    expect(unpublished.single.postId, '42');
    expect(unpublished.single.etag, 'e2');
  });

  test('a stale etag surfaces as a ConnectException', () async {
    final repository = _repository(
      FakeTransportBuilder().unary(
        SocialAppService.publishPost,
        (_, _) => throw ConnectException(Code.aborted, 'stale'),
      ),
    );

    expect(
      repository.publishPostOrThrow(42, etag: 'old'),
      throwsA(isA<ConnectException>()),
    );
  });

  test('update lists only the changed fields in the mask', () async {
    final requests = <pb.UpdatePostRequest>[];
    final repository = _repository(
      FakeTransportBuilder().unary(SocialAppService.updatePost, (request, _) {
        requests.add(request);
        return pb.UpdatePostResponse(post: _proto());
      }),
    );

    await repository.updatePost(42, etag: 'e1', content: '新內文');
    await repository.updatePost(
      42,
      etag: 'e1',
      content: '新內文',
      imageStagedToken: 'tok',
      productId: '900',
      updateProduct: true,
      hashtags: const ['#a'],
    );

    expect(requests.first.updateMask.paths, ['body', 'hashtag_inputs']);
    expect(requests.first.post.hasProductId(), isFalse);
    expect(requests.last.updateMask.paths, [
      'body',
      'hashtag_inputs',
      'product_id',
      'image_staged_token',
    ]);
    expect(requests.last.post.imageStagedToken, 'tok');
    expect(requests.last.post.productId, '900');
    expect(requests.last.post.etag, 'e1');
  });

  test(
    'update with updateProduct and no productId clears the product',
    () async {
      final requests = <pb.UpdatePostRequest>[];
      final repository = _repository(
        FakeTransportBuilder().unary(SocialAppService.updatePost, (request, _) {
          requests.add(request);
          return pb.UpdatePostResponse(post: _proto());
        }),
      );

      await repository.updatePost(
        42,
        etag: 'e1',
        content: 'x',
        updateProduct: true,
      );

      expect(requests.single.updateMask.paths, contains('product_id'));
      expect(requests.single.post.hasProductId(), isFalse);
    },
  );

  test('create publishes the new post and delete targets the id', () async {
    final created = <pb.CreatePostRequest>[];
    final deleted = <pb.DeletePostRequest>[];
    final repository = _repository(
      FakeTransportBuilder()
          .unary(SocialAppService.createPost, (request, _) {
            created.add(request);
            return pb.CreatePostResponse(post: _proto());
          })
          .unary(SocialAppService.deletePost, (request, _) {
            deleted.add(request);
            return pb.DeletePostResponse();
          }),
    );

    final post = await repository.createPost(
      content: '內文',
      imageStagedToken: 'tok',
    );
    await repository.deletePost(42);

    expect(post.id, 42);
    expect(created.single.publish, isTrue);
    expect(created.single.imageStagedToken, 'tok');
    expect(deleted.single.postId, '42');
  });

  group('comments and forum over Connect', () {
    pb.Comment commentProto({
      String id = '5',
      String? parentId,
      CommentState state = CommentState.COMMENT_STATE_VISIBLE,
      bool canAct = true,
    }) => pb.Comment(
      id: id,
      postId: '42',
      parentId: parentId,
      state: state,
      author: pb.Actor(
        kind: ActorKind.ACTOR_KIND_BUYER,
        id: '9',
        name: '王小明',
        avatar: common.Image(uriTemplate: 'https://m.test/i/{preset}/a.jpg'),
      ),
      body: '好吃',
      etag: 'ce-$id',
      isMine: false,
      viewerLiked: true,
      viewerCanDelete: canAct,
      viewerCanReply: canAct,
      viewerCanEdit: false,
    )..mergeFromProto3Json({'likeCount': '3', 'replyCount': '2'});

    test('maps the proto Comment, including viewer_can_* and etag', () async {
      final requests = <pb.ListCommentsRequest>[];
      final repository = _repository(
        FakeTransportBuilder().unary(SocialPublicService.listComments, (
          request,
          _,
        ) {
          requests.add(request);
          return pb.ListCommentsResponse(
            comments: [
              commentProto(),
              commentProto(
                id: '6',
                state: CommentState.COMMENT_STATE_DELETED,
                canAct: false,
              ),
            ],
          );
        }),
      );

      final list = await repository.listComments(42);

      expect(requests.single.postId, '42');
      expect(requests.single.hasParentId(), isFalse);
      final c = list.first;
      expect(c.id, '5');
      expect(c.authorName, '王小明');
      expect(c.authorAvatar, startsWith('https://m.test/i/'));
      expect(c.body, '好吃');
      expect(c.likeCount, 3);
      expect(c.replyCount, 2);
      expect(c.viewerLiked, isTrue);
      expect(c.viewerCanDelete, isTrue);
      expect(c.viewerCanReply, isTrue);
      expect(c.viewerCanEdit, isFalse);
      expect(c.etag, 'ce-5');
      expect(c.placeholderTextFor(UiLocale.zhTW), isNull);
      expect(list.last.placeholderTextFor(UiLocale.zhTW), '此留言已刪除');
      // viewer_can_* 取伺服器的值：沒設的就是 false，不由 client 補。
      expect(list.last.viewerCanDelete, isFalse);
      expect(list.last.viewerCanReply, isFalse);
      expect(list.last.viewerCanEdit, isFalse);
    });

    test('lists replies by parent_id and follows next_page_token', () async {
      final tokens = <String>[];
      final repository = _repository(
        FakeTransportBuilder().unary(SocialPublicService.listComments, (
          request,
          _,
        ) {
          tokens.add('${request.parentId}|${request.pageToken}');
          return request.pageToken.isEmpty
              ? pb.ListCommentsResponse(
                  comments: [commentProto(id: '7', parentId: '5')],
                  nextPageToken: 'p2',
                )
              : pb.ListCommentsResponse(
                  comments: [commentProto(id: '8', parentId: '5')],
                );
        }),
      );

      final list = await repository.listComments(42, parentId: '5');

      expect(tokens, ['5|', '5|p2']);
      expect(list.map((c) => c.id), ['7', '8']);
      expect(list.first.parentId, '5');
    });

    test(
      'create sends parent and reply target; delete sends the etag',
      () async {
        final created = <pb.CreateCommentRequest>[];
        final deleted = <pb.DeleteCommentRequest>[];
        final repository = _repository(
          FakeTransportBuilder()
              .unary(SocialAppService.createComment, (request, _) {
                created.add(request);
                return pb.CreateCommentResponse(comment: commentProto(id: '9'));
              })
              .unary(SocialAppService.deleteComment, (request, _) {
                deleted.add(request);
                return pb.DeleteCommentResponse();
              }),
        );

        final c = await repository.createComment(
          42,
          body: '謝謝',
          parentId: '5',
          replyToId: '7',
        );
        await repository.deleteComment('9', etag: 'ce-9');

        expect(c.id, '9');
        expect(created.single.postId, '42');
        expect(created.single.parentId, '5');
        expect(created.single.replyToId, '7');
        expect(deleted.single.commentId, '9');
        expect(deleted.single.etag, 'ce-9');
      },
    );

    test('like and unlike target the comment id', () async {
      final liked = <String>[];
      final repository = _repository(
        FakeTransportBuilder()
            .unary(SocialAppService.likeComment, (request, _) {
              liked.add('like ${request.commentId}');
              return pb.LikeCommentResponse();
            })
            .unary(SocialAppService.unlikeComment, (request, _) {
              liked.add('unlike ${request.commentId}');
              return pb.UnlikeCommentResponse();
            }),
      );

      await repository.likeComment('5');
      await repository.unlikeComment('5');

      expect(liked, ['like 5', 'unlike 5']);
    });

    test('maps the forum entry and sends etag on update and delete', () async {
      final updated = <pb.UpdateForumEntryRequest>[];
      final deleted = <pb.DeleteForumEntryRequest>[];
      final replied = <pb.CreateForumEntryRequest>[];
      final entry = pb.ForumEntry(
        id: '3',
        storeId: '7',
        state: CommentState.COMMENT_STATE_VISIBLE,
        body: '請問',
        etag: 'fe-3',
        viewerCanReply: true,
        viewerCanDelete: false,
        viewerCanEdit: false,
      );
      final repository = _repository(
        FakeTransportBuilder()
            .unary(
              SocialPublicService.listForumEntries,
              (_, _) => pb.ListForumEntriesResponse(
                entries: [
                  entry,
                  pb.ForumEntry(id: '4', storeId: '7', etag: 'fe-4'),
                ],
              ),
            )
            .unary(SocialAppService.createForumEntry, (request, _) {
              replied.add(request);
              return pb.CreateForumEntryResponse(entry: entry);
            })
            .unary(SocialAppService.updateForumEntry, (request, _) {
              updated.add(request);
              return pb.UpdateForumEntryResponse(entry: entry);
            })
            .unary(SocialAppService.deleteForumEntry, (request, _) {
              deleted.add(request);
              return pb.DeleteForumEntryResponse();
            }),
      );

      final list = await repository.listForumEntries('7');
      await repository.replyForumEntry('7', parentId: '3', body: '有貨');
      await repository.updateForumEntry('4', etag: 'fe-4', body: '改');
      await repository.deleteForumEntry('4', etag: 'fe-4');

      expect(list.first.viewerCanReply, isTrue);
      expect(list.first.viewerCanDelete, isFalse);
      expect(list.first.etag, 'fe-3');
      expect(list.last.viewerCanReply, isFalse);
      expect(list.last.viewerCanDelete, isFalse);
      expect(replied.single.storeId, '7');
      expect(replied.single.parentId, '3');
      expect(updated.single.entry.id, '4');
      expect(updated.single.entry.etag, 'fe-4');
      expect(updated.single.updateMask.paths, ['body']);
      expect(deleted.single.entryId, '4');
      expect(deleted.single.etag, 'fe-4');
    });
  });
}
