import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart' as common;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';

PostRepository _repository(FakeTransportBuilder transport) {
  final container = ProviderContainer();
  addTearDown(container.dispose);
  return PostRepository(
    Dio(),
    container.read(Provider<Ref>((ref) => ref)),
    SocialAppServiceClient(transport.build()),
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
        imageKey: 'https://m.test/i/original/p/9.jpg',
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
}
