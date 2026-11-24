import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/features/notification/presentation/notification_controller.dart';
import 'package:hawkr_buyer/features/notification/presentation/notifications_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/l10n/gen/app_localizations.dart';

import '../../support/notification_fakes.dart';

class _PaginationErrorController extends NotificationListController {
  int retryCalls = 0;

  @override
  Future<NotificationListState> build() async {
    return NotificationListState(
      notifications: [inboxItem('1', title: '既有通知', body: '這筆資料必須保留')],
      unreadCount: 0,
      nextPageToken: 'tok',
      paginationError: ConnectException(Code.unavailable, 'page'),
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
            builder: (_, __) => MaterialApp(
              locale: const Locale('zh'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: NotificationsScreen(),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump();

      expect(find.text('既有通知'), findsOneWidget);
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
