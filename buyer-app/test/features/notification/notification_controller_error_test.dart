import 'dart:collection';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/notification/data/notification_model.dart';
import 'package:hawkr_buyer/features/notification/data/notification_repository.dart';
import 'package:hawkr_buyer/features/notification/presentation/notification_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  group('NotificationListController initial load', () {
    final scenarios = <({String name, AppException error})>[
      (name: 'network', error: const NetworkException()),
      (name: 'business', error: const ServerException('business failure')),
      (name: 'timeout', error: const TimeoutException()),
    ];

    for (final scenario in scenarios) {
      test(
        '${scenario.name} failure surfaces AsyncError, not empty data',
        () async {
          final repository = _FakeNotificationRepository([scenario.error]);
          final container = _containerFor(repository);
          addTearDown(container.dispose);
          await container.read(sessionProvider.future);

          await expectLater(
            container.read(notificationListControllerProvider.future),
            throwsA(same(scenario.error)),
          );

          final state = container.read(notificationListControllerProvider);
          expect(state.hasError, isTrue);
          expect(state.value, isNull);
          expect(repository.notificationCalls, 1);
        },
      );
    }
  });

  group('NotificationListController pagination', () {
    test(
      'load-more error preserves records and retry appends next page',
      () async {
        const paginationError = NetworkException('page unavailable');
        final repository = _FakeNotificationRepository([
          _page([_item(1)], hasMore: true),
          paginationError,
          _page([_item(2)], hasMore: false),
        ]);
        final container = _containerFor(repository);
        addTearDown(container.dispose);
        await container.read(sessionProvider.future);

        final initial = await container.read(
          notificationListControllerProvider.future,
        );
        expect(initial.notifications.map((item) => item.id), [1]);

        final controller = container.read(
          notificationListControllerProvider.notifier,
        );
        await controller.loadMore();

        final failedPage = container
            .read(notificationListControllerProvider)
            .requireValue;
        expect(
          container.read(notificationListControllerProvider).hasError,
          isFalse,
        );
        expect(failedPage.notifications.map((item) => item.id), [1]);
        expect(failedPage.paginationError, same(paginationError));
        expect(failedPage.isLoadingMore, isFalse);
        expect(failedPage.hasMore, isTrue);

        // Scroll-driven loadMore is paused after an error. Retry is explicit.
        await controller.loadMore();
        expect(repository.notificationCalls, 2);

        await controller.retryLoadMore();
        final retried = container
            .read(notificationListControllerProvider)
            .requireValue;
        expect(retried.notifications.map((item) => item.id), [1, 2]);
        expect(retried.paginationError, isNull);
        expect(retried.hasMore, isFalse);
        expect(repository.cursors, [0, 1, 1]);
      },
    );

    test('empty successful page never reads last notification id', () async {
      final repository = _FakeNotificationRepository([
        _page(const [], hasMore: true),
      ]);
      final container = _containerFor(repository);
      addTearDown(container.dispose);
      await container.read(sessionProvider.future);
      await container.read(notificationListControllerProvider.future);

      await container
          .read(notificationListControllerProvider.notifier)
          .loadMore();

      final state = container
          .read(notificationListControllerProvider)
          .requireValue;
      expect(state.notifications, isEmpty);
      expect(state.hasMore, isFalse);
      expect(state.paginationError, isNull);
      expect(repository.notificationCalls, 1);
    });
  });
}

ProviderContainer _containerFor(NotificationRepository repository) {
  return ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(_ActiveSession.new),
      notificationRepositoryProvider.overrideWithValue(repository),
    ],
  );
}

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Buyer');
}

class _FakeNotificationRepository extends NotificationRepository {
  _FakeNotificationRepository(Iterable<Object> responses)
    : _responses = Queue<Object>.of(responses),
      super(Dio());

  final Queue<Object> _responses;
  final List<int> cursors = [];
  int notificationCalls = 0;

  @override
  Future<NotificationListResult> getNotifications({
    int cursorId = 0,
    int size = 20,
  }) async {
    notificationCalls++;
    cursors.add(cursorId);
    final response = _responses.removeFirst();
    if (response is AppException) throw response;
    return response as NotificationListResult;
  }

  @override
  Future<UnreadCount> getUnreadCount() async => const UnreadCount(count: 1);
}

NotificationListResult _page(
  List<NotificationItem> records, {
  required bool hasMore,
}) {
  return NotificationListResult(
    records: records,
    total: records.length,
    hasMore: hasMore,
  );
}

NotificationItem _item(int id) {
  return NotificationItem(
    id: id,
    type: 'ORDER',
    title: 'Order $id',
    content: 'Updated',
    createdAt: DateTime(2026, 7, 27),
  );
}
