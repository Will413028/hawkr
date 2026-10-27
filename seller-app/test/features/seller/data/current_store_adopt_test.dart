import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/fake_my_store.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

void main() {
  Future<ProviderContainer> loaded(FakeMyStoreServer server) async {
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(server.repository),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(currentStoreProvider.future);
    return container;
  }

  test('採用同一個店家的寫入結果：etag 立刻更新', () async {
    final container = await loaded(FakeMyStoreServer());

    container
        .read(currentStoreProvider.notifier)
        .adoptWritten(Store(id: 'store-1', storeName: '新店名', etag: 'v2'));

    final store = container.read(myStoreSnapshotProvider)!;
    expect(store.etag, 'v2');
    expect(store.storeName, '新店名');
  });

  test('店家 id 不同：不採用', () async {
    final container = await loaded(FakeMyStoreServer());

    container
        .read(currentStoreProvider.notifier)
        .adoptWritten(Store(id: 'other-store', storeName: '別家', etag: 'v9'));

    final store = container.read(myStoreSnapshotProvider)!;
    expect(store.etag, 'etag-1');
    expect(store.storeName, '阿明果園');
  });

  test('沒有店家：不採用', () async {
    final container = await loaded(FakeMyStoreServer.absent());

    container
        .read(currentStoreProvider.notifier)
        .adoptWritten(Store(id: 'store-1', etag: 'v2'));

    expect(container.read(myStoreSnapshotProvider)!.hasStore, isFalse);
  });

  test('關店：直接變成沒有店家，重讀失敗也不會回到開店的快照', () async {
    final server = FakeMyStoreServer();
    final container = await loaded(server);

    container.read(currentStoreProvider.notifier).adoptClosed();

    expect(container.read(myStoreSnapshotProvider)!.hasStore, isFalse);
    expect(container.read(needsOnboardingProvider), isTrue);

    server.failure = ConnectException(Code.unavailable, 'try later');
    await container.read(currentStoreProvider.notifier).refresh();

    expect(container.read(myStoreSnapshotProvider)!.hasStore, isFalse);
    expect(container.read(storeUnavailableProvider), isFalse);
  });
}
