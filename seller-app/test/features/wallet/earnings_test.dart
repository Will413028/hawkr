import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/features/wallet/data/earnings.dart';

EarningsEntry _entry(String id) => EarningsEntry(id: id, orderNo: 'O-$id');

class _Server {
  final pages = <String, ListMyEarningsResponse>{};
  final listRequests = <ListMyEarningsRequest>[];
  final orderRequests = <String>[];
  Object? listFailure;
  Object? orderFailure;

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.listMyEarnings, (request, _) {
          listRequests.add(request);
          final error = listFailure;
          if (error != null) throw error;
          return pages[request.pageToken] ?? ListMyEarningsResponse();
        })
        .unary(SellerAppService.getMyOrderEarnings, (request, _) {
          orderRequests.add(request.orderNo);
          final error = orderFailure;
          if (error != null) throw error;
          return GetMyOrderEarningsResponse(entries: [_entry('t1')])
            ..mergeFromProto3Json({'netAmount': '980'});
        })
        .build(),
  );
}

ProviderContainer _container(_Server server) {
  final container = ProviderContainer(
    overrides: [sellerClientProvider.overrideWithValue(server.client)],
  );
  addTearDown(container.dispose);
  return container;
}

void main() {
  group('fetchMyOrderEarnings', () {
    test('帶訂單編號讀入帳列與淨額', () async {
      final server = _Server();
      final result = await fetchMyOrderEarnings(server.client, 'O-1');
      expect(server.orderRequests, ['O-1']);
      expect(result!.entries.single.id, 't1');
      expect(result.netAmount.toInt(), 980);
    });

    test('not_found（沒有付款或別家店的訂單）是 null，不是錯誤', () async {
      final server = _Server()
        ..orderFailure = ConnectException(Code.notFound, 'not found');
      expect(await fetchMyOrderEarnings(server.client, 'O-1'), isNull);
    });

    test('其他失敗丟出', () async {
      final server = _Server()
        ..orderFailure = ConnectException(Code.unavailable, 'down');
      await expectLater(
        fetchMyOrderEarnings(server.client, 'O-1'),
        throwsA(isA<ConnectException>()),
      );
    });
  });

  group('MyEarnings', () {
    test('第一頁之後 loadMore 帶 next_page_token 並接在後面', () async {
      final server = _Server()
        ..pages[''] = ListMyEarningsResponse(
          entries: [_entry('a')],
          nextPageToken: 'p2',
        )
        ..pages['p2'] = ListMyEarningsResponse(entries: [_entry('b')]);
      final container = _container(server);

      final first = await container.read(myEarningsProvider('acc-1').future);
      expect(first.entries.map((e) => e.id), ['a']);
      expect(first.hasMore, isTrue);
      expect(server.listRequests.single.pageSize, earningsPageSize);

      await container.read(myEarningsProvider('acc-1').notifier).loadMore();
      final after = container.read(myEarningsProvider('acc-1')).value!;
      expect(after.entries.map((e) => e.id), ['a', 'b']);
      expect(after.hasMore, isFalse);
      expect(server.listRequests.last.pageToken, 'p2');
    });

    test('loadMore 失敗保留已讀到的列並記下錯誤', () async {
      final server = _Server()
        ..pages[''] = ListMyEarningsResponse(
          entries: [_entry('a')],
          nextPageToken: 'p2',
        );
      final container = _container(server);
      await container.read(myEarningsProvider('acc-1').future);

      server.listFailure = ConnectException(Code.unavailable, 'down');
      await container.read(myEarningsProvider('acc-1').notifier).loadMore();
      final after = container.read(myEarningsProvider('acc-1')).value!;
      expect(after.entries.map((e) => e.id), ['a']);
      expect(after.loadMoreError, isA<ConnectException>());
      expect(after.hasMore, isTrue);
    });

    test('沒有下一頁時 loadMore 不呼叫伺服器', () async {
      final server = _Server()
        ..pages[''] = ListMyEarningsResponse(entries: [_entry('a')]);
      final container = _container(server);
      await container.read(myEarningsProvider('acc-1').future);
      await container.read(myEarningsProvider('acc-1').notifier).loadMore();
      expect(server.listRequests, hasLength(1));
    });
  });
}
