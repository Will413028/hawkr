import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('the global buyer chrome shares the selected text-size policy', () {
    final mainSource = File('lib/main.dart').readAsStringSync();

    expect(
      RegExp(
        r'final appSurface =[\s\S]{0,900}return MediaQuery\('
        r'[\s\S]{0,160}data: mq,[\s\S]{0,160}child: appSurface',
      ).hasMatch(mainSource),
      isTrue,
      reason:
          'The MediaQuery must wrap the entire app surface, including global '
          'navigation and overlays, rather than only the router child.',
    );
  });
}
