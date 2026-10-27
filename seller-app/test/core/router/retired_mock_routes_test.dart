import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('legacy dispute mock redirects to server-backed after-sales', () {
    final router = File('lib/core/router/app_router.dart').readAsStringSync();

    expect(router, isNot(contains('OrderDisputeScreen(order:')));
    expect(
      RegExp(
        r'path: OrderRoutes\.dispute,[\s\S]{0,300}'
        r'redirect: \(context, state\) => ToolRoutes\.afterSales',
      ).hasMatch(router),
      isTrue,
    );
  });
}
