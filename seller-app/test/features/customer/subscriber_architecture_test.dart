import 'dart:async';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_api/images.dart';
import 'package:hawkr_seller/features/customer/data/customer_model.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/features/customer/data/customer_repository.dart';
import 'package:hawkr_seller/features/customer/data/subscriber_model.dart';
import 'package:hawkr_seller/features/customer/data/subscriber_remote_service.dart';
import 'package:hawkr_seller/features/customer/data/subscriber_repository.dart';
import 'package:hawkr_seller/features/customer/presentation/subscriber_list_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test(
    'repository maps BuyerCard fields with one request per page and no REST',
    () async {
      final requests = <seller.ListMyFollowersRequest>[];
      final service = SubscriberRemoteService(
        SellerAppServiceClient(
          FakeTransportBuilder().unary(SellerAppService.listMyFollowers, (
            request,
            _,
          ) {
            requests.add(request);
            return seller.ListMyFollowersResponse(
              nextPageToken: 'cursor-2',
              followers: [
                seller.Follower(
                  buyer: seller.BuyerCard(
                    id: '1',
                    displayName: '陳小農',
                    avatar: Image(
                      uriTemplate: 'https://media.example.com/i/{preset}/a.png',
                    ),
                  ),
                ),
                seller.Follower(
                  buyer: seller.BuyerCard(id: '2', deleted: true),
                ),
              ],
            );
          }).build(),
        ),
      );
      final restPaths = <String>[];
      final repository = RemoteSubscriberRepository(
        remoteService: service,
        customerRepository: _FakeCustomerRepository(restPaths),
      );

      final page = await repository.loadSubscribers(size: 20);

      expect(requests, hasLength(1));
      expect(restPaths, isEmpty, reason: 'no per-row metadata request');
      expect(page.nextPageToken, 'cursor-2');
      final [named, deleted] = page.subscribers;
      expect(named.buyerId, '1');
      expect(named.displayName, '陳小農');
      expect(named.avatarUrl, 'https://media.example.com/i/avatar/a.png');
      expect(named.deleted, isFalse);
      expect(deleted.buyerId, '2');
      expect(deleted.deleted, isTrue);
      expect(deleted.nickname, isEmpty);
      expect(deleted.avatarUrl, isEmpty);
      expect(
        () => page.subscribers.add(
          const Subscriber(buyerId: '11', subscribedAt: ''),
        ),
        throwsUnsupportedError,
      );
    },
  );

  test(
    'controller exposes immutable typed data and repository command',
    () async {
      final repository = _FakeSubscriberRepository();
      final container = ProviderContainer(
        overrides: [subscriberRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);

      final value = await container.read(
        subscriberListControllerProvider.future,
      );
      final customer = await container
          .read(subscriberListControllerProvider.notifier)
          .resolveCustomer('42');

      expect(value.subscribers.single.displayName, '測試買家');
      expect(customer?.id, 'customer-42');
      expect(repository.resolvedAccountIds, ['42']);
      expect(
        () => value.subscribers.add(
          const Subscriber(buyerId: '43', subscribedAt: ''),
        ),
        throwsUnsupportedError,
      );
    },
  );

  group('Q15 — loadMore result is dropped once the list was replaced', () {
    test('refresh during loadMore is not overwritten', () async {
      final repository = _ControlledSubscriberRepository();
      final container = ProviderContainer(
        overrides: [subscriberRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        subscriberListControllerProvider,
        (_, _) {},
      );
      addTearDown(subscription.close);
      final notifier = container.read(
        subscriberListControllerProvider.notifier,
      );

      repository.next.complete(_page(['1'], 'cursor-2'));
      await container.read(subscriberListControllerProvider.future);

      repository.next = Completer();
      final more = repository.next;
      final loadMore = notifier.loadMore();

      repository.next = Completer()..complete(_page(['9'], ''));
      await notifier.refresh();

      more.complete(_page(['2'], 'cursor-3'));
      await loadMore;

      final value = container.read(subscriberListControllerProvider).value!;
      expect(value.subscribers.map((s) => s.buyerId), ['9']);
      expect(value.nextPageToken, isEmpty);
      expect(value.loadingMore, isFalse);
      expect(repository.pageTokens, ['', 'cursor-2', '']);
    });

    test('loadMore finishing after dispose does not throw', () async {
      final repository = _ControlledSubscriberRepository();
      final container = ProviderContainer(
        overrides: [subscriberRepositoryProvider.overrideWithValue(repository)],
      );
      final subscription = container.listen(
        subscriberListControllerProvider,
        (_, _) {},
      );
      final notifier = container.read(
        subscriberListControllerProvider.notifier,
      );
      repository.next.complete(_page(['1'], 'cursor-2'));
      await container.read(subscriberListControllerProvider.future);

      repository.next = Completer();
      final more = repository.next;
      final loadMore = notifier.loadMore();
      subscription.close();
      container.dispose();

      more.complete(_page(['2'], ''));
      await expectLater(loadMore, completes);
    });
  });
}

class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter(this.paths);

  final List<String> paths;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.path);
    return ResponseBody.fromString('{"code":0,"data":{}}', 200);
  }

  @override
  void close({bool force = false}) {}
}

class _FakeCustomerRepository extends CustomerRepository {
  _FakeCustomerRepository(List<String> paths)
    : super(Dio()..httpClientAdapter = _RecordingAdapter(paths));
}

class _FakeSubscriberRepository implements SubscriberRepository {
  final resolvedAccountIds = <String>[];

  @override
  Future<({List<Subscriber> subscribers, String nextPageToken})>
  loadSubscribers({String pageToken = '', int size = 20}) async {
    return (
      subscribers: const [
        Subscriber(buyerId: '42', subscribedAt: '2026-07-27', nickname: '測試買家'),
      ],
      nextPageToken: '',
    );
  }

  @override
  Future<Customer?> resolveCustomer(String accountId) async {
    resolvedAccountIds.add(accountId);
    return Customer(id: 'customer-$accountId', name: '測試買家', phone: '');
  }
}

({List<Subscriber> subscribers, String nextPageToken}) _page(
  List<String> ids,
  String nextPageToken,
) => (
  subscribers: [
    for (final id in ids) Subscriber(buyerId: id, subscribedAt: ''),
  ],
  nextPageToken: nextPageToken,
);

class _ControlledSubscriberRepository implements SubscriberRepository {
  var next =
      Completer<({List<Subscriber> subscribers, String nextPageToken})>();
  final pageTokens = <String>[];

  @override
  Future<({List<Subscriber> subscribers, String nextPageToken})>
  loadSubscribers({String pageToken = '', int size = 20}) {
    pageTokens.add(pageToken);
    return next.future;
  }

  @override
  Future<Customer?> resolveCustomer(String accountId) async => null;
}
