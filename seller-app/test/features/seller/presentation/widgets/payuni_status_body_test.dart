import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/features/seller/presentation/widgets/payuni_status_body.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  // 個人賣家的身分欄位問題會一次列出多個欄位：同一個 Column 的子項不能有重複的 key（debug 的 widget 樹 assert）。
  testWidgets('多個要修正的欄位各畫一列，沒有重複 key 的錯誤', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(375, 812),
          builder: (_, __) => MaterialApp(
            home: Scaffold(
              body: PayuniStatusBody(
                payuni: SellerPayUni(
                  state: PayUniState.PAY_UNI_STATE_FIXABLE,
                  fixFields: [
                    KYCField.KYC_FIELD_REAL_NAME,
                    KYCField.KYC_FIELD_BIRTH_DATE,
                    KYCField.KYC_FIELD_BANK_NAME,
                    KYCField.KYC_FIELD_BANK_CODE,
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );

    expect(tester.takeException(), isNull);
    final rows = find.byWidgetPredicate(
      (w) =>
          w is Text &&
          w.key is ValueKey<String> &&
          (w.key! as ValueKey<String>).value.startsWith('payuni-fix-field-'),
    );
    expect(rows, findsNWidgets(4)); // 銀行名稱列出名稱與代碼兩個控制項
  });
}
