import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/notification/presentation/notification_controller.dart';
import 'package:hawkr_buyer/features/notification/presentation/notifications_screen.dart';
import 'package:hawkr_buyer/features/notification/presentation/widgets/notification_type_icon.dart';
import 'package:hawkr_buyer/l10n/gen/app_localizations.dart';
import 'package:hawkr_buyer/shared/time/taipei_display.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/notification_fakes.dart';

class _ServerTextController extends NotificationListController {
  @override
  Future<NotificationListState> build() async => NotificationListState(
    unreadCount: 2,
    notifications: [
      inboxItem(
        'a',
        title: 'Server composed title 賣家',
        body: 'Server composed body 店家 已出貨',
        presentation: 'INBOX_PRESENTATION_REFUND',
      ),
      inboxItem(
        'b',
        title: '公告 title',
        body: '公告 body',
        presentation: 'INBOX_PRESENTATION_PROMOTION',
      ),
    ],
  );

  @override
  Future<void> silentRefresh() async {}
}

void main() {
  testWidgets('inbox shows the server text verbatim and icon by presentation', (
    tester,
  ) async {
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
            _ServerTextController.new,
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
            home: const TaipeiDisplayScope(
              deviceOffset: Duration(hours: 8),
              child: NotificationsScreen(),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();

    expect(find.text('Server composed title 賣家'), findsOneWidget);
    expect(find.text('Server composed body 店家 已出貨'), findsOneWidget);
    expect(find.text('公告 title'), findsOneWidget);

    final icons = tester
        .widgetList<NotificationTypeIcon>(find.byType(NotificationTypeIcon))
        .map((w) => w.kind)
        .toList();
    expect(icons, [NotificationIconKind.refund, NotificationIconKind.coupon]);
  });
}
