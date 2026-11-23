import 'package:hawkr_seller/l10n/l10n.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/customer/data/customer_model.dart';
import 'package:hawkr_seller/features/customer/data/customer_repository.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_controller.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_management_screen.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations_zh.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockRepo extends Mock implements CustomerRepository {}

Future<void> _pump(WidgetTester tester, _MockRepo repo) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [customerRepositoryProvider.overrideWithValue(repo)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => const MaterialApp(
          locale: Locale('zh'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: CustomerManagementScreen(),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void _stubList(_MockRepo repo, Future<CustomerPage> Function(String token) f) {
  when(
    () => repo.listCustomers(
      type: any(named: 'type'),
      tagId: any(named: 'tagId'),
      sort: any(named: 'sort'),
      descending: any(named: 'descending'),
      pageToken: any(named: 'pageToken'),
      size: any(named: 'size'),
    ),
  ).thenAnswer((i) => f(i.namedArguments[#pageToken] as String));
}

void main() {
  setUpAll(() {
    registerFallbackValue(CustomerSort.CUSTOMER_SORT_UNSPECIFIED);
  });

  testWidgets('an empty list shows the empty state, not the error state', (
    tester,
  ) async {
    final repo = _MockRepo();
    _stubList(
      repo,
      (_) async => const CustomerPage(customers: [], nextPageToken: ''),
    );
    await _pump(tester, repo);
    expect(
      find.byKey(const ValueKey('seller-customers-empty-state')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('seller-customers-error-state')),
      findsNothing,
    );
  });

  testWidgets(
    'a read failure shows the server text and retry, not the empty state',
    (tester) async {
      final repo = _MockRepo();
      var fail = true;
      _stubList(repo, (_) async {
        if (fail) throw ConnectException(Code.permissionDenied, 'x');
        return const CustomerPage(
          customers: [Customer(id: '1', name: '甲')],
          nextPageToken: '',
        );
      });
      await _pump(tester, repo);
      expect(
        find.byKey(const ValueKey('seller-customers-error-state')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey('seller-customers-empty-state')),
        findsNothing,
      );
      fail = false;
      await tester.tap(find.text('重新載入'));
      await tester.pumpAndSettle();
      expect(find.text('甲'), findsOneWidget);
    },
  );

  testWidgets('scrolling to the end loads the next page with the token', (
    tester,
  ) async {
    final repo = _MockRepo();
    final tokens = <String>[];
    _stubList(repo, (t) async {
      tokens.add(t);
      return t.isEmpty
          ? CustomerPage(
              customers: [
                for (var i = 0; i < 12; i++) Customer(id: '$i', name: 'A$i'),
              ],
              nextPageToken: 'p2',
            )
          : const CustomerPage(
              customers: [Customer(id: '99', name: 'Z99')],
              nextPageToken: '',
            );
    });
    await _pump(tester, repo);
    await tester.drag(find.byType(ListView), const Offset(0, -3000));
    await tester.pumpAndSettle();
    expect(tokens, ['', 'p2']);
    expect(find.text('Z99'), findsOneWidget);
  });

  testWidgets('a load-more failure keeps loaded customers and offers retry', (
    tester,
  ) async {
    final repo = _MockRepo();
    _stubList(repo, (t) async {
      if (t.isEmpty) {
        return CustomerPage(
          customers: [
            for (var i = 0; i < 12; i++) Customer(id: '$i', name: 'A$i'),
          ],
          nextPageToken: 'p2',
        );
      }
      throw ConnectException(Code.unavailable, 'down');
    });
    await _pump(tester, repo);
    await tester.drag(find.byType(ListView), const Offset(0, -3000));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('seller-customers-load-more-error')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('seller-customers-error-state')),
      findsNothing,
    );
  });

  test(
    'sort labels come from the catalog; UNSPECIFIED reads as first order',
    () {
      expect(
        customerSortLabel(
          CustomerSort.CUSTOMER_SORT_ORDER_COUNT,
          UiLocale.zhTW,
        ),
        '訂單數',
      );
      expect(
        customerSortLabel(
          CustomerSort.CUSTOMER_SORT_TOTAL_SPENT,
          UiLocale.zhTW,
        ),
        '消費金額',
      );
      expect(
        customerSortLabel(
          CustomerSort.CUSTOMER_SORT_UNSPECIFIED,
          UiLocale.zhTW,
        ),
        '首購時間',
      );
    },
  );

  test('a deleted customer shows the catalog deleted name', () {
    const c = Customer(id: 'b1', deleted: true, name: '舊暱稱');
    expect(
      c.displayName(UiLocale.zhTW, AppLocalizationsZh()),
      buyerProfileDeletedName(UiLocale.zhTW),
    );
  });

  testWidgets('the default sort label is the order count', (tester) async {
    final repo = _MockRepo();
    _stubList(
      repo,
      (_) async => const CustomerPage(customers: [], nextPageToken: ''),
    );
    await _pump(tester, repo);
    expect(find.text('訂單數'), findsOneWidget);
    expect(find.text('消費次數'), findsNothing);
  });

  testWidgets(
    'tabs are all and VIP only; selecting VIP and a sort reaches the repository',
    (tester) async {
      final repo = _MockRepo();
      _stubList(
        repo,
        (_) async => const CustomerPage(customers: [], nextPageToken: ''),
      );
      await _pump(tester, repo);
      expect(find.text('黑名單'), findsNothing);
      expect(find.text('搜尋姓名或電話'), findsNothing);
      await tester.tap(find.text('VIP'));
      await tester.pumpAndSettle();
      verify(
        () => repo.listCustomers(
          type: CustomerType.CUSTOMER_TYPE_VIP,
          tagId: null,
          sort: CustomerSort.CUSTOMER_SORT_ORDER_COUNT,
          descending: true,
          pageToken: '',
          size: any(named: 'size'),
        ),
      ).called(1);
    },
  );

  test('default filter sorts by order count descending', () {
    const f = CustomerFilter();
    expect(f.sort, CustomerSort.CUSTOMER_SORT_ORDER_COUNT);
    expect(f.descending, isTrue);
    expect(f.type, isNull);
  });
}
