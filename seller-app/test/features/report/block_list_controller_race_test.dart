import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_seller/features/report/data/store_block_repository.dart';
import 'package:hawkr_seller/features/report/presentation/report_list_controller.dart';

BlockedBuyerItem _b(String id, [int rank = 0]) => BlockedBuyerItem(
  buyerId: id,
  deleted: false,
  name: id,
  createTime: DateTime.utc(2026, 1, 1).subtract(Duration(days: rank)),
);

class _Repo implements StoreBlockRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();

  _Repo(this.firstPage);

  List<BlockedBuyerItem> firstPage;
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
  final repo = _Repo([_b('a', 0), _b('b', 1)]);
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
        BlockedBuyersPage(items: [_b('a', 0), _b('c', 2)], nextPageToken: ''),
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
        BlockedBuyersPage(items: [_b('c', 2)], nextPageToken: ''),
      );
      await more;
      repo.unblockResult!.completeError(Exception('x'));
      await expectLater(unblock, throwsException);
      expect(_ids(c), ['a', 'b', 'c']);
    },
  );

  test('loadMore dedupes by buyer id', () async {
    final (c, repo) = await _setup();
    final notifier = c.read(blockListControllerProvider.notifier);
    final more = notifier.loadMore();
    repo.nextPage!.complete(
      BlockedBuyersPage(
        items: [_b('b', 1), _b('c', 2), _b('c', 2)],
        nextPageToken: '',
      ),
    );
    await more;
    expect(_ids(c), ['a', 'b', 'c']);
  });

  test('refresh during an in-flight loadMore drops the stale page', () async {
    final (c, repo) = await _setup();
    final notifier = c.read(blockListControllerProvider.notifier);
    final more = notifier.loadMore();
    repo.firstPage = [_b('x', 0)];
    await notifier.silentRefresh();
    repo.nextPage!.complete(
      BlockedBuyersPage(items: [_b('c', 2)], nextPageToken: ''),
    );
    await more;
    expect(_ids(c), ['x']);
    expect(c.read(blockListControllerProvider).requireValue.hasMore, true);
    expect(
      c.read(blockListControllerProvider).requireValue.isLoadingMore,
      false,
    );
  });

  test(
    'refresh during an in-flight unblock does not resurrect the buyer',
    () async {
      final (c, repo) = await _setup();
      final notifier = c.read(blockListControllerProvider.notifier);
      final unblock = notifier.unblock('a');
      await notifier.silentRefresh();
      expect(_ids(c), ['b']);
      await notifier.refresh();
      expect(_ids(c), ['b']);
      repo.unblockResult!.complete();
      await unblock;
      expect(_ids(c), ['b']);
    },
  );

  test(
    'failed unblock after a refresh reloads instead of reinserting',
    () async {
      final (c, repo) = await _setup();
      final notifier = c.read(blockListControllerProvider.notifier);
      final unblock = notifier.unblock('a');
      await notifier.silentRefresh();
      repo.unblockResult!.completeError(Exception('x'));
      await expectLater(unblock, throwsException);
      expect(_ids(c), ['a', 'b']);
    },
  );

  test('failed unblock reinserts at its keyset position', () async {
    final (c, repo) = await _setup();
    final notifier = c.read(blockListControllerProvider.notifier);
    final more = notifier.loadMore();
    repo.nextPage!.complete(
      BlockedBuyersPage(items: [_b('c', 2)], nextPageToken: ''),
    );
    await more;
    final unblock = notifier.unblock('b');
    expect(_ids(c), ['a', 'c']);
    repo.unblockResult!.completeError(Exception('x'));
    await expectLater(unblock, throwsException);
    expect(_ids(c), ['a', 'b', 'c']);
  });

  test(
    'an Error from unblock propagates and clears the pending guard',
    () async {
      final (c, repo) = await _setup();
      final notifier = c.read(blockListControllerProvider.notifier);
      final unblock = notifier.unblock('a');
      repo.unblockResult!.completeError(StateError('x'));
      await expectLater(unblock, throwsStateError);
      await notifier.silentRefresh();
      expect(_ids(c), ['a', 'b']);
    },
  );
}
