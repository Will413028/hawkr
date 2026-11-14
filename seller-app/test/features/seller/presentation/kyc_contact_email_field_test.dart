import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/seller/data/contact_email.dart';
import 'package:hawkr_seller/features/seller/presentation/kyc_wizard_steps.dart';

Future<void> _pump(
  WidgetTester tester, {
  required AsyncValue<AccountContactEmail?>? contact,
  String value = '',
  String? error,
  ValueChanged<String>? onChanged,
  VoidCallback? onRetry,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      child: ScreenUtilInit(
        designSize: const Size(393, 852),
        builder: (_, _) => MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: KycContactEmailField(
                contact: contact,
                value: value,
                error: error,
                onChanged: onChanged ?? (_) {},
                onRetry: onRetry ?? () {},
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  group('contactEmailEditable', () {
    test('只有 provider 驗證過的信箱不能改；沒有信箱或自填的可以改', () {
      expect(contactEmailEditable(null), isTrue);
      expect(
        contactEmailEditable(
          const AccountContactEmail(email: 'a@b.co', verified: false),
        ),
        isTrue,
      );
      expect(
        contactEmailEditable(
          const AccountContactEmail(email: 'a@b.co', verified: true),
        ),
        isFalse,
      );
    });
  });

  group('validateContactEmail（與伺服器同一條規則）', () {
    test('去前後空白、轉小寫；local@domain，網域要有點，至多 254 字', () {
      expect(validateContactEmail(' Shop@Example.COM '), isNull);
      expect(validateContactEmail(''), '請輸入金流商聯絡信箱');
      expect(validateContactEmail('shop@example'), 'Email 格式不正確');
      expect(validateContactEmail('shop example@a.co'), 'Email 格式不正確');
      expect(validateContactEmail('${'a' * 250}@b.co'), '信箱最多 254 字');
    });
  });

  group('KycContactEmailField', () {
    testWidgets('帳號有驗證過的信箱：只顯示信箱與說明，沒有輸入框', (tester) async {
      await _pump(
        tester,
        contact: const AsyncData(
          AccountContactEmail(email: 'login@example.com', verified: true),
        ),
        value: 'login@example.com',
      );

      expect(find.byType(TextField), findsNothing);
      expect(find.byKey(const Key('kyc_contact_email')), findsNothing);
      expect(
        find.byKey(const Key('kyc_contact_email_verified')),
        findsOneWidget,
      );
      expect(find.text('login@example.com'), findsOneWidget);
      expect(find.text('金流商聯絡信箱'), findsOneWidget);
      expect(find.text('金流商會用這個信箱聯絡你收款事宜'), findsOneWidget);
    });

    testWidgets('帳號沒有任何信箱：顯示輸入框，輸入交給 onChanged', (tester) async {
      String? typed;
      await _pump(
        tester,
        contact: const AsyncData(null),
        onChanged: (v) => typed = v,
      );

      expect(find.byKey(const Key('kyc_contact_email')), findsOneWidget);
      await tester.enterText(find.byType(TextField), 'shop@example.com');
      expect(typed, 'shop@example.com');
    });

    testWidgets('只有自填的信箱：仍可修改，預設帶入目前的值', (tester) async {
      await _pump(
        tester,
        contact: const AsyncData(
          AccountContactEmail(email: 'mine@example.com', verified: false),
        ),
        value: 'mine@example.com',
        error: 'Email 格式不正確',
      );

      expect(
        tester.widget<TextField>(find.byType(TextField)).controller!.text,
        'mine@example.com',
      );
      expect(find.text('Email 格式不正確'), findsOneWidget);
    });

    testWidgets('還沒讀到：不顯示；讀取失敗：顯示重新載入', (tester) async {
      await _pump(tester, contact: const AsyncLoading());
      expect(find.byType(TextField), findsNothing);
      expect(find.text('金流商聯絡信箱'), findsNothing);

      var retried = false;
      await _pump(
        tester,
        contact: AsyncError<AccountContactEmail?>(
          StateError('down'),
          StackTrace.empty,
        ),
        onRetry: () => retried = true,
      );
      expect(find.byType(TextField), findsNothing);
      await tester.tap(find.text('重新載入'));
      expect(retried, isTrue);
    });
  });
}
