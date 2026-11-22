import 'dart:async';

import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/customer/data/customer_model.dart';
import 'package:hawkr_seller/features/customer/data/customer_repository.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_detail_screen.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockCustomerRepository extends Mock implements CustomerRepository {}

const _normal = Customer(id: '42', name: '測試顧客');

Future<void> _pump(WidgetTester tester, _MockCustomerRepository repo) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [customerRepositoryProvider.overrideWithValue(repo)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: const CustomerDetailScreen(customer: _normal),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump();
}

void main() {
  setUpAll(() => registerFallbackValue(CustomerType.CUSTOMER_TYPE_VIP));

  testWidgets('menu has only the VIP action: no blacklist entry', (
    tester,
  ) async {
    final repo = _MockCustomerRepository();
    when(() => repo.getCustomer('42')).thenAnswer((_) async => _normal);
    await _pump(tester, repo);
    await tester.tap(find.byTooltip('顧客更多操作'));
    await tester.pumpAndSettle();
    expect(find.text('升級 VIP'), findsOneWidget);
    expect(find.textContaining('黑名單'), findsNothing);
  });

  testWidgets('VIP update sends the contract type and flips the menu label', (
    tester,
  ) async {
    final repo = _MockCustomerRepository();
    when(() => repo.getCustomer('42')).thenAnswer((_) async => _normal);
    when(
      () => repo.updateCustomerType('42', CustomerType.CUSTOMER_TYPE_VIP),
    ).thenAnswer(
      (_) async => _normal.copyWith(type: CustomerType.CUSTOMER_TYPE_VIP),
    );
    await _pump(tester, repo);
    await tester.tap(find.byTooltip('顧客更多操作'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('升級 VIP'));
    await tester.pumpAndSettle();
    verify(
      () => repo.updateCustomerType('42', CustomerType.CUSTOMER_TYPE_VIP),
    ).called(1);
    await tester.tap(find.byTooltip('顧客更多操作'));
    await tester.pumpAndSettle();
    expect(find.text('取消 VIP'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('a failed update shows the server text and keeps the type', (
    tester,
  ) async {
    final repo = _MockCustomerRepository();
    when(() => repo.getCustomer('42')).thenAnswer((_) async => _normal);
    when(() => repo.updateCustomerType(any(), any())).thenAnswer(
      (_) =>
          Future.error(ConnectException(Code.permissionDenied, '店家已停權，無法操作客戶')),
    );
    await _pump(tester, repo);
    await tester.tap(find.byTooltip('顧客更多操作'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('升級 VIP'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('顧客更多操作'));
    await tester.pumpAndSettle();
    expect(find.text('升級 VIP'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('a failed detail read shows an error row with retry', (
    tester,
  ) async {
    final repo = _MockCustomerRepository();
    var fail = true;
    when(() => repo.getCustomer('42')).thenAnswer((_) async {
      if (fail) throw ConnectException(Code.unavailable, 'down');
      return _normal;
    });
    await _pump(tester, repo);
    expect(
      find.byKey(const ValueKey('customer-detail-load-error')),
      findsOneWidget,
    );
    fail = false;
    await tester.tap(find.text('重試'));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('customer-detail-load-error')),
      findsNothing,
    );
  });

  testWidgets(
    'leaving the page before the read finishes ignores the late response',
    (tester) async {
      final repo = _MockCustomerRepository();
      final late = Completer<Customer?>();
      when(() => repo.getCustomer('42')).thenAnswer((_) => late.future);
      await _pump(tester, repo);
      await tester.pumpWidget(const SizedBox.shrink());
      late.complete(_normal);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    },
  );
}
