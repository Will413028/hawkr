import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_seller/features/report/presentation/store_block_capability.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_seller/features/post/data/post_comment.dart';
import 'package:hawkr_seller/features/report/data/store_block_rules.dart';

void main() {
  test('canBlockBuyers is true while the store snapshot is not loaded', () {
    final c = ProviderContainer(
      overrides: [myStoreSnapshotProvider.overrideWithValue(null)],
    );
    addTearDown(c.dispose);
    expect(c.read(canBlockBuyersProvider), true);
  });

  group('canBlockAuthor', () {
    bool rule({
      bool canBlockBuyers = true,
      bool isMine = false,
      bool viewerCanDelete = true,
      bool authorIsBuyer = true,
    }) => canBlockAuthor(
      canBlockBuyers: canBlockBuyers,
      isMine: isMine,
      viewerCanDelete: viewerCanDelete,
      authorIsBuyer: authorIsBuyer,
    );

    test('buyer author, others, deletable, capable', () {
      expect(rule(), true);
    });
    test('store author cannot', () {
      expect(rule(authorIsBuyer: false), false);
    });
    test('own cannot', () {
      expect(rule(isMine: true), false);
    });
    test('not deletable cannot', () {
      expect(rule(viewerCanDelete: false), false);
    });
    test('missing capability cannot', () {
      expect(rule(canBlockBuyers: false), false);
    });
  });

  group('blockTargetName', () {
    test('uses the one fallback for empty and blank names', () {
      expect(blockTargetName(''), '此人');
      expect(blockTargetName('  '), '此人');
      expect(blockTargetName('小明'), '小明');
    });
  });

  group('PostComment.fromProto author kind', () {
    for (final c in [
      (ActorKind.ACTOR_KIND_BUYER, true),
      (ActorKind.ACTOR_KIND_STORE, false),
      (ActorKind.ACTOR_KIND_UNSPECIFIED, false),
    ]) {
      test('${c.$1.name} -> authorIsBuyer ${c.$2}', () {
        final comment = PostComment.fromProto(
          pb.Comment(
            id: '1',
            author: pb.Actor(kind: c.$1),
          ),
        );
        expect(comment.authorIsBuyer, c.$2);
      });
    }
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
