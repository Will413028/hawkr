import 'package:hawkr_buyer/features/cart/presentation/widgets/address_picker_sheet.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  testWidgets(
    'eight long addresses scroll at 320dp and 1.4x while add CTA stays reachable',
    (tester) async {
      var addCount = 0;
      final addresses = List.generate(8, (index) {
        final number = index + 1;
        return Address(
          etag: 'v1',
          id: number,
          townCode: '65000010',
          addressDetail: '文化路二段一百二十三號二十樓之$number',
          receiverName: '長姓名收件人$number',
          receiverPhone: '09123456${number.toString().padLeft(2, '0')}',
        );
      });
      const lastAddress = '新北市板橋區文化路二段一百二十三號二十樓之8';

      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(320, 568),
        systemTextScale: 2,
        home: Scaffold(
          body: Builder(
            builder: (context) => FilledButton(
              onPressed: () async {
                await showAddressPickerSheet(
                  context: context,
                  userAddresses: addresses,
                  selectedAddressId: 1,
                  onAddAddress: () => addCount += 1,
                );
              },
              child: const Text('開啟地址選擇'),
            ),
          ),
        ),
      );

      await tester.tap(find.text('開啟地址選擇'));
      await tester.pumpAndSettle();

      final list = find.byKey(const ValueKey('checkout-address-picker-scroll'));
      final scrollable = find.descendant(
        of: list,
        matching: find.byType(Scrollable),
      );
      final addButton = find.byKey(
        const ValueKey('checkout-address-picker-add'),
      );
      expect(list, findsOneWidget);
      expect(scrollable, findsOneWidget);
      expect(addButton, findsOneWidget);
      expect(tester.getSize(addButton).height, greaterThanOrEqualTo(48));
      expect(tester.getRect(addButton).bottom, lessThanOrEqualTo(568));
      expect(tester.takeException(), isNull);

      await tester.scrollUntilVisible(
        find.text(lastAddress),
        160,
        scrollable: scrollable,
      );
      await tester.pump();

      expect(find.text(lastAddress), findsOneWidget);
      expect(find.text(lastAddress).hitTestable(), findsOneWidget);
      expect(addButton.hitTestable(), findsOneWidget);
      expect(tester.takeException(), isNull);

      await tester.tap(addButton);
      await tester.pumpAndSettle();
      expect(addCount, 1);
      expect(find.text('選擇收件地址'), findsNothing);
    },
  );
}
