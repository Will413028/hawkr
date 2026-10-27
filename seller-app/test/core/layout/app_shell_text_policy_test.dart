import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('the global seller chrome shares the selected text-size policy', () {
    final appSource = File('lib/app.dart').readAsStringSync();

    expect(
      RegExp(
        r'final appSurface =[\s\S]{0,900}return MediaQuery\('
        r'[\s\S]{0,160}data: mediaQuery,[\s\S]{0,160}child: appSurface',
      ).hasMatch(appSource),
      isTrue,
      reason:
          'The MediaQuery must wrap routes, the environment banner, and the '
          'debug overlay so every global surface uses one text-size policy.',
    );
  });
}
