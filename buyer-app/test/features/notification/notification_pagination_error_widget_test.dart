import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/notification/data/notification_model.dart';
import 'package:hawkr_buyer/features/notification/presentation/notification_controller.dart';
import 'package:hawkr_buyer/features/notification/presentation/notifications_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _PaginationErrorController extends NotificationListController {
  int retryCalls = 0;

  @override
  Future<NotificationListState> build() async {
    return NotificationListState(
      notifications: [
        NotificationItem(
          id: 1,
          type: 'SYSTEM',
          title: '既有通知',
          content: '這筆資料必須保留',
          createdAt: DateTime(2026, 7, 27),
        ),
      ],
      unreadCount: 0,
      hasMore: true,
      paginationError: const NetworkException('下一頁暫時無法載入'),
    );
  }

  @override
  Future<void> silentRefresh() async {}

  @override
  Future<void> retryLoadMore() async {
    retryCalls++;
  }
}

void main() {
  testWidgets(
    'pagination failure stays below existing records and exposes retry',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationInboxStreamProvider.overrideWithValue(
              const Stream<Object?>.empty(),
            ),
            notificationListControllerProvider.overrideWith(
              _PaginationErrorController.new,
            ),
          ],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, __) => const MaterialApp(home: NotificationsScreen()),
          ),
        ),
      );
      await tester.pump();
      await tester.pump();

      expect(find.text('既有通知'), findsOneWidget);
      expect(find.text('下一頁暫時無法載入'), findsOneWidget);
      expect(
        find.byKey(const ValueKey('notifications-pagination-error')),
        findsOneWidget,
      );

      await tester.tap(
        find.byKey(const ValueKey('notifications-pagination-retry')),
      );
      await tester.pump();

      final context = tester.element(find.byType(NotificationsScreen));
      final container = ProviderScope.containerOf(context);
      final controller = container.read(
        notificationListControllerProvider.notifier,
      );
      expect(controller, isA<_PaginationErrorController>());
      expect((controller as _PaginationErrorController).retryCalls, 1);
      expect(find.text('既有通知'), findsOneWidget);
    },
  );
}
