import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/features/post/data/post_hashtag_repository.dart';

void main() {
  test('suggestions come from SocialPublicService.SuggestTags', () async {
    final requests = <pb.SuggestTagsRequest>[];
    final transport = FakeTransportBuilder().unary(
      SocialPublicService.suggestTags,
      (request, _) {
        requests.add(request);
        return pb.SuggestTagsResponse(
          tags: [
            pb.TagCount(name: '美食')..mergeFromProto3Json({'postCount': '7'}),
            pb.TagCount(name: '')..mergeFromProto3Json({'postCount': '1'}),
          ],
        );
      },
    );
    final repo = PostHashtagRepository(
      SocialPublicServiceClient(transport.build()),
    );

    final result = await repo.search(prefix: ' 美 ', limit: 10);

    expect(requests.single.query, '美');
    expect(requests.single.contains, isFalse);
    expect(requests.single.maxResults, 10);
    expect(result.single.name, '美食');
    expect(result.single.postCount, 7);
  });

  test('blank prefix makes no request', () async {
    final requests = <pb.SuggestTagsRequest>[];
    final transport = FakeTransportBuilder().unary(
      SocialPublicService.suggestTags,
      (request, _) {
        requests.add(request);
        return pb.SuggestTagsResponse();
      },
    );
    final repo = PostHashtagRepository(
      SocialPublicServiceClient(transport.build()),
    );
    expect(await repo.search(prefix: '  '), isEmpty);
    expect(requests, isEmpty);
  });
}
