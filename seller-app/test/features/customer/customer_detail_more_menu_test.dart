import 'dart:async';

import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/customer/data/customer_model.dart';
import 'package:hawkr_seller/features/customer/data/customer_repository.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_detail_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockCustomerRepository extends Mock implements CustomerRepository {}

Future<void> _pumpCustomer(
  WidgetTester tester,
  _MockCustomerRepository repository,
) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [customerRepositoryProvider.overrideWithValue(repository)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: CustomerDetailScreen(
            customer: Customer(id: '42', name: '測試顧客', phone: '0900000000'),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump();
}

void main() {
  testWidgets(
    'cancel menu does not mutate; pending update blocks repeat and failure permits retry',
    (tester) async {
      final repository = _MockCustomerRepository();
      var customer = Customer(id: '42', name: '測試顧客', phone: '0900000000');
      when(
        () => repository.getCustomerByAccountId('42'),
      ).thenAnswer((_) async => customer);
      when(() => repository.getCustomerTags('42')).thenAnswer((_) async => []);
      var response = Completer<void>();
      when(
        () => repository.updateCustomerType('42', 'vip'),
      ).thenAnswer((_) => response.future);
      await _pumpCustomer(tester, repository);
      await tester.tap(find.byTooltip('顧客更多操作'));
      await tester.pumpAndSettle();
      await tester.tapAt(const Offset(10, 400));
      await tester.pumpAndSettle();
      verifyNever(() => repository.updateCustomerType(any(), any()));
      await tester.tap(find.byTooltip('顧客更多操作'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(PopupMenuItem<String>, '升級 VIP'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 350));
      await tester.tap(find.byTooltip('顧客更多操作'));
      await tester.pump();
      expect(find.byType(PopupMenuItem<String>), findsNothing);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      verify(() => repository.updateCustomerType('42', 'vip')).called(1);
      response.completeError(StateError('offline'));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      response = Completer<void>();
      await tester.tap(find.byTooltip('顧客更多操作'));
      await tester.pumpAndSettle();
      expect(
        find.widgetWithText(PopupMenuItem<String>, '升級 VIP'),
        findsOneWidget,
      );
      await tester.tap(find.widgetWithText(PopupMenuItem<String>, '升級 VIP'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 350));
      customer = customer.copyWith(customerType: 'vip');
      response.complete();
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('顧客更多操作'));
      await tester.pumpAndSettle();
      expect(
        find.widgetWithText(PopupMenuItem<String>, '取消 VIP'),
        findsOneWidget,
      );
      await tester.tapAt(const Offset(10, 400));
      await tester.pumpAndSettle();
      await tester.pump(const Duration(seconds: 3));
      expect(tester.takeException(), isNull);
    },
  );

  for (final stage in ['initial load', 'mutation', 'refresh']) {
    testWidgets('leaving customer during $stage ignores late response', (
      tester,
    ) async {
      final repository = _MockCustomerRepository();
      final customer = Customer(id: '42', name: '測試顧客', phone: '0900000000');
      final detail = Completer<Customer?>();
      final response = Completer<void>();
      var loads = 0;
      when(() => repository.getCustomerByAccountId('42')).thenAnswer((_) {
        loads++;
        return stage == 'initial load' || loads > 1
            ? detail.future
            : Future.value(customer);
      });
      when(() => repository.getCustomerTags('42')).thenAnswer((_) async => []);
      when(
        () => repository.updateCustomerType('42', 'vip'),
      ).thenAnswer((_) => response.future);
      await _pumpCustomer(tester, repository);
      if (stage != 'initial load') {
        await tester.tap(find.byTooltip('顧客更多操作'));
        await tester.pumpAndSettle();
        await tester.tap(find.widgetWithText(PopupMenuItem<String>, '升級 VIP'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        if (stage == 'refresh') {
          response.complete();
          await tester.pump();
        }
      }
      await tester.pumpWidget(const SizedBox.shrink());
      if (!response.isCompleted) response.complete();
      detail.complete(customer.copyWith(customerType: 'vip'));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });
  }

  for (final action in [
    (initial: 'normal', selected: '升級 VIP', next: 'vip', after: '取消 VIP'),
    (initial: 'vip', selected: '取消 VIP', next: 'normal', after: '升級 VIP'),
    (initial: 'normal', selected: '設黑名單', next: 'blocklist', after: '解除黑名單'),
    (initial: 'blocklist', selected: '解除黑名單', next: 'normal', after: '設黑名單'),
  ]) {
    testWidgets(
      'circular customer menu preserves ${action.selected} selection',
      (tester) async {
        tester.view.physicalSize = const Size(390, 844);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final semantics = tester.ensureSemantics();
        try {
          final repository = _MockCustomerRepository();
          var customer = Customer(
            id: '42',
            name: '測試顧客',
            phone: '0900000000',
            customerType: action.initial,
          );
          when(
            () => repository.getCustomerByAccountId('42'),
          ).thenAnswer((_) async => customer);
          when(
            () => repository.getCustomerTags('42'),
          ).thenAnswer((_) async => []);
          when(
            () => repository.updateCustomerType('42', action.next),
          ).thenAnswer((_) async {
            customer = customer.copyWith(customerType: action.next);
          });
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                customerRepositoryProvider.overrideWithValue(repository),
              ],
              child: ScreenUtilInit(
                designSize: const Size(390, 844),
                builder: (_, _) => MaterialApp(
                  theme: AppTheme.light,
                  home: CustomerDetailScreen(customer: customer),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();
          final menu = find.bySemanticsLabel('顧客更多操作');
          expect(menu, findsOneWidget);
          expect(tester.getSize(menu), const Size(48, 48));
          await tester.tap(menu);
          await tester.pumpAndSettle();
          final selected = find.widgetWithText(
            PopupMenuItem<String>,
            action.selected,
          );
          expect(selected, findsOneWidget);
          await tester.tap(selected);
          await tester.pumpAndSettle();
          verify(
            () => repository.updateCustomerType('42', action.next),
          ).called(1);
          await tester.tap(menu);
          await tester.pumpAndSettle();
          expect(
            find.widgetWithText(PopupMenuItem<String>, action.after),
            findsOneWidget,
          );
          await tester.tapAt(const Offset(10, 400));
          await tester.pumpAndSettle();
          await tester.pump(const Duration(seconds: 3));
          expect(tester.takeException(), isNull);
        } finally {
          semantics.dispose();
        }
      },
    );
  }
}
