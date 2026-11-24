import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/features/notification/data/push/manager.dart';
import 'package:hawkr_seller/features/notification/data/state_refresh.dart';

/// 推播 data 的 `state_refresh_hints`（前景）與 `inbox_item_id`（點擊）。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
  });

  late final PushNotificationManager manager;
  setUpAll(() => manager = PushNotificationManager.instance);

  late List<String> refreshCalls;
  late List<String> markedRead;
  late ProviderContainer container;

  setUp(() {
    refreshCalls = [];
    markedRead = [];
    final transport = FakeTransportBuilder()
        .unary(
          NotificationAppService.listInboxItems,
          (_, _) => ListInboxItemsResponse(),
        )
        .unary(
          NotificationAppService.getUnreadCount,
          (_, _) => GetUnreadCountResponse(),
        )
        .unary(NotificationAppService.markInboxItemRead, (r, _) {
          markedRead.add(r.id);
          return MarkInboxItemReadResponse();
        })
        .build();
    container = ProviderContainer(
      overrides: [
        notificationClientProvider.overrideWithValue(
          NotificationAppServiceClient(transport),
        ),
        stateRefreshActionsProvider.overrideWithValue(
          StateRefreshActions(
            refreshInbox: () => refreshCalls.add('inbox'),
            refreshOrders: () => refreshCalls.add('orders'),
            refreshAfterSales: () => refreshCalls.add('afterSales'),
            refreshSellerStatus: () => refreshCalls.add('sellerStatus'),
          ),
        ),
      ],
    );
    manager.seedContainerForTest(container);
  });

  tearDown(() {
    manager.seedContainerForTest(null);
    container.dispose();
  });

  test('foreground push applies state_refresh_hints like the stream', () {
    manager.handleForegroundMessageForTest(
      const RemoteMessage(
        data: {
          'state_refresh_hints':
              'STATE_REFRESH_HINT_ORDER,STATE_REFRESH_HINT_SELLER_STATUS',
          'deep_link': '',
        },
      ),
    );
    expect(refreshCalls, ['inbox', 'orders', 'sellerStatus']);
  });

  test('foreground push without hints only refreshes the inbox', () {
    manager.handleForegroundMessageForTest(
      const RemoteMessage(data: {'deep_link': ''}),
    );
    expect(refreshCalls, ['inbox']);
  });

  test('tapping a push marks its inbox item read', () async {
    manager.handleMessageOpenedAppForTest(
      const RemoteMessage(
        data: {'inbox_item_id': 'n42', 'deep_link': '/seller-center/wallet'},
      ),
    );
    await pumpEventQueue();
    expect(markedRead, ['n42']);
  });

  test('tapping a push without inbox_item_id marks nothing', () async {
    manager.handleMessageOpenedAppForTest(
      const RemoteMessage(data: {'deep_link': ''}),
    );
    await pumpEventQueue();
    expect(markedRead, isEmpty);
  });
}
