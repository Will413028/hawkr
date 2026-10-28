import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('full-screen GoRouter destinations use themed builder routes', () {
    final source = File('lib/core/router/app_router.dart').readAsStringSync();
    const themedRoutes = <String>[
      'SettingsRoutes.chat',
      'OrderRoutes.details',
      'OrderRoutes.receipts',
      'OrderRoutes.receiptsDetail',
      'OrderRoutes.logistics',
      'ProductRoutes.picker',
      'ProductRoutes.info',
      'ProductRoutes.images',
      'ProductRoutes.specs',
    ];

    for (final route in themedRoutes) {
      final marker = 'path: $route,';
      final start = source.indexOf(marker);
      expect(start, isNonNegative, reason: '$route is missing from GoRouter');

      final nextRoute = source.indexOf(
        '\n      GoRoute(',
        start + marker.length,
      );
      final routeSource = source.substring(
        start,
        nextRoute == -1 ? source.length : nextRoute,
      );
      expect(
        RegExp(r'\bbuilder\s*:').hasMatch(routeSource),
        isTrue,
        reason: '$route must inherit the app PageTransitionsTheme',
      );
      expect(
        routeSource,
        isNot(contains('pageBuilder:')),
        reason: '$route must not bypass the app PageTransitionsTheme',
      );
    }

    expect(source, isNot(contains('_noAnimationPage')));
    expect(source, isNot(contains('CustomTransitionPage')));
  });

  test('legacy route factories use MaterialPageRoute', () {
    const routeFiles = <String>[
      'lib/features/chat/presentation/chat_list_screen.dart',
      'lib/features/order/presentation/logistics_tracking_screen.dart',
      'lib/features/order/presentation/order_receipts_screen.dart',
      'lib/features/order/presentation/order_details_receipts_screen.dart',
    ];

    for (final path in routeFiles) {
      final source = File(path).readAsStringSync();
      expect(
        source,
        contains('return MaterialPageRoute<T>('),
        reason: '$path must inherit Material page transition policy',
      );
      expect(source, isNot(contains('NoAnimationPageRoute')));
      expect(source, isNot(contains('no_animation_page_route.dart')));
    }

    expect(
      File('lib/shared/widgets/no_animation_page_route.dart').existsSync(),
      isFalse,
    );
  });
}
