import 'package:hawkr_seller/features/seller/presentation/profile_form_widgets.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget wrap(Widget child) => MaterialApp(
    home: Scaffold(body: SingleChildScrollView(child: child)),
  );

  testWidgets('鎖定時顯示提示，且點不進編輯', (tester) async {
    var tapped = false;
    await tester.pumpWidget(
      wrap(
        ProfileEditableInfoRow(
          label: '店舖名稱',
          value: '梨山盛花園',
          lockedNotice: kPayuniLockedNotice,
          onTap: () => tapped = true,
        ),
      ),
    );

    expect(find.text(kPayuniLockedNotice), findsOneWidget);
    await tester.tap(find.text('梨山盛花園'));
    await tester.pump();
    expect(tapped, isFalse, reason: '鎖定的欄位不該開啟編輯對話框');
  });

  testWidgets('未鎖定時沒有提示，且點得進編輯', (tester) async {
    var tapped = false;
    await tester.pumpWidget(
      wrap(
        ProfileEditableInfoRow(
          label: '店舖名稱',
          value: '梨山盛花園',
          onTap: () => tapped = true,
        ),
      ),
    );

    expect(find.text(kPayuniLockedNotice), findsNothing);
    await tester.tap(find.text('梨山盛花園'));
    await tester.pump();
    expect(tapped, isTrue);
  });

  // 兩個欄位各出現一次——這是賣家實際會看到的畫面形狀。
  testWidgets('店舖名稱與商家介紹同時鎖定時，提示各出現一次', (tester) async {
    await tester.pumpWidget(
      wrap(
        ProfileInfoSection(
          title: '商家資訊',
          rows: [
            ProfileEditableInfoRow(
              label: '店舖名稱',
              value: '梨山盛花園',
              lockedNotice: kPayuniLockedNotice,
              onTap: () {},
            ),
            ProfileEditableInfoRow(
              label: '商家介紹',
              value: '海拔兩千公尺的高冷蔬菜',
              maxLines: 4,
              lockedNotice: kPayuniLockedNotice,
              onTap: () {},
            ),
          ],
        ),
      ),
    );

    expect(find.text(kPayuniLockedNotice), findsNWidgets(2));
  });
}
