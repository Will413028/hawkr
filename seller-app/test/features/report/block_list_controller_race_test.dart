import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_seller/features/report/data/store_block_repository.dart';
import 'package:hawkr_seller/features/report/presentation/report_list_controller.dart';

BlockedBuyerItem _b(String id) =>
    BlockedBuyerItem(buyerId: id, deleted: false, name: id);

class _Repo implements StoreBlockRepository {
  _Repo(this.firstPage);

  final List<BlockedBuyerItem> firstPage;
  Completer<BlockedBuyersPage>? nextPage;
  Completer<void>? unblockResult;

  @override
  Future<BlockedBuyersPage> list({String pageToken = '', int pageSize = 20}) {
    if (pageToken.isEmpty) {
      return Future.value(
        BlockedBuyersPage(items: firstPage, nextPageToken: 'next'),
      );
    }
    return (nextPage = Completer<BlockedBuyersPage>()).future;
  }

  @override
  Future<void> unblock(String buyerId) =>
      (unblockResult = Completer<void>()).future;

  @override
  Future<void> blockByComment(String commentId) => throw UnimplementedError();

  @override
  Future<void> blockByForumEntry(String forumEntryId) =>
      throw UnimplementedError();
}

Future<(ProviderContainer, _Repo)> _setup() async {
  final repo = _Repo([_b('a'), _b('b')]);
  final container = ProviderContainer(
    overrides: [storeBlockRepositoryProvider.overrideWithValue(repo)],
  );
  addTearDown(container.dispose);
  container.listen(blockListControllerProvider, (_, _) {});
  await container.read(blockListControllerProvider.future);
  return (container, repo);
}

List<String> _ids(ProviderContainer c) => c
    .read(blockListControllerProvider)
    .requireValue
    .blocks
    .map((b) => b.buyerId)
    .toList();

void main() {
  test(
    'loadMore keeps an unblock that happened while it was in flight',
    () async {
      final (c, repo) = await _setup();
      final notifier = c.read(blockListControllerProvider.notifier);
      final more = notifier.loadMore();
      final unblock = notifier.unblock('a');
      expect(_ids(c), ['b']);
      repo.unblockResult!.complete();
      await unblock;
      repo.nextPage!.complete(
        BlockedBuyersPage(items: [_b('a'), _b('c')], nextPageToken: ''),
      );
      await more;
      expect(_ids(c), ['b', 'c']);
    },
  );

  test(
    'failed unblock reinserts only that item into the latest state',
    () async {
      final (c, repo) = await _setup();
      final notifier = c.read(blockListControllerProvider.notifier);
      final unblock = notifier.unblock('a');
      final more = notifier.loadMore();
      repo.nextPage!.complete(
        BlockedBuyersPage(items: [_b('c')], nextPageToken: ''),
      );
      await more;
      repo.unblockResult!.completeError(StateError('x'));
      await expectLater(unblock, throwsStateError);
      expect(_ids(c), ['a', 'b', 'c']);
    },
  );

  test('loadMore dedupes by buyer id', () async {
    final (c, repo) = await _setup();
    final notifier = c.read(blockListControllerProvider.notifier);
    final more = notifier.loadMore();
    repo.nextPage!.complete(
      BlockedBuyersPage(items: [_b('b'), _b('c'), _b('c')], nextPageToken: ''),
    );
    await more;
    expect(_ids(c), ['a', 'b', 'c']);
  });
}
