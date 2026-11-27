import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('buyer-app ignores the generated iOS GoogleService-Info.plist', () {
    final lines = File(
      '.gitignore',
    ).readAsLinesSync().map((l) => l.trim()).toList();
    expect(lines, contains('ios/Runner/GoogleService-Info.plist'));
  });
}
