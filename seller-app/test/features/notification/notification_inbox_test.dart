import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/features/notification/presentation/notifications_screen.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations.dart';

InboxItem _item(String id, String title, String body, {bool read = false}) =>
    InboxItem(
      id: id,
      category: NotificationCategory.NOTIFICATION_CATEGORY_ORDER,
      presentation: InboxPresentation.INBOX_PRESENTATION_ORDER,
      title: title,
      body: body,
      deepLink: '/seller-center/orders/O1',
      read: read,
      createTime: Timestamp.fromDateTime(DateTime.now().toUtc()),
    );

void main() {
  testWidgets('inbox rows show the server-composed title and body as is', (
    tester,
  ) async {
    final listRequests = <ListInboxItemsRequest>[];
    final transport = FakeTransportBuilder()
        .unary(NotificationAppService.listInboxItems, (r, _) {
          listRequests.add(r);
          return ListInboxItemsResponse(
            items: [_item('n1', 'SERVER-TITLE', 'SERVER-BODY: 買家 A')],
            totalSize: 1,
          );
        })
        .unary(
          NotificationAppService.getUnreadCount,
          (_, _) => GetUnreadCountResponse(unreadCount: 1),
        )
        .build();
    final overrides = <Override>[
      notificationClientProvider.overrideWithValue(
        NotificationAppServiceClient(transport),
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: overrides,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp(
            locale: const Locale('zh'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const NotificationsScreen(),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('SERVER-TITLE'), findsOneWidget);
    expect(find.text('SERVER-BODY: 買家 A'), findsOneWidget);
    expect(
      listRequests.first.audience,
      NotificationAudience.NOTIFICATION_AUDIENCE_STORE,
    );
  });
}
