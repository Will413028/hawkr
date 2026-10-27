import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'welcome route keeps the brand loader and removes the cow welcome UI',
    () {
      final source = File(
        'lib/features/auth/presentation/welcome_screen.dart',
      ).readAsStringSync();

      expect(source, contains('return const BrandStartupSplash();'));
      expect(source, isNot(contains('assets/farm/w')));
      expect(
        source,
        isNot(contains('Future.delayed(const Duration(seconds: 1))')),
      );
    },
  );
}
