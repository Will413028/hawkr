import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_seller/features/post/data/post_comment.dart';
import 'package:hawkr_seller/features/report/data/store_block_rules.dart';

void main() {
  group('canBlockCommentAuthor', () {
    test('others deletable comment can be blocked', () {
      expect(canBlockCommentAuthor(isMine: false, viewerCanDelete: true), true);
    });
    test('own comment cannot', () {
      expect(canBlockCommentAuthor(isMine: true, viewerCanDelete: true), false);
    });
    test('not deletable cannot', () {
      expect(
        canBlockCommentAuthor(isMine: false, viewerCanDelete: false),
        false,
      );
    });
  });

  group('canBlockForumAuthor', () {
    test('buyer author, others, deletable', () {
      expect(
        canBlockForumAuthor(
          isMine: false,
          viewerCanDelete: true,
          authorIsBuyer: true,
        ),
        true,
      );
    });
    test('store author cannot', () {
      expect(
        canBlockForumAuthor(
          isMine: false,
          viewerCanDelete: true,
          authorIsBuyer: false,
        ),
        false,
      );
    });
    test('own entry cannot', () {
      expect(
        canBlockForumAuthor(
          isMine: true,
          viewerCanDelete: true,
          authorIsBuyer: true,
        ),
        false,
      );
    });
    test('not deletable cannot', () {
      expect(
        canBlockForumAuthor(
          isMine: false,
          viewerCanDelete: false,
          authorIsBuyer: true,
        ),
        false,
      );
    });
  });

  group('StoreForumEntry.fromProto author kind', () {
    for (final c in [
      (ActorKind.ACTOR_KIND_BUYER, true),
      (ActorKind.ACTOR_KIND_STORE, false),
      (ActorKind.ACTOR_KIND_UNSPECIFIED, false),
    ]) {
      test('${c.$1.name} -> authorIsBuyer ${c.$2}', () {
        final entry = StoreForumEntry.fromProto(
          pb.ForumEntry(
            id: '1',
            author: pb.Actor(kind: c.$1),
          ),
        );
        expect(entry.authorIsBuyer, c.$2);
      });
    }
  });
}
