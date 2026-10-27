import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// build phase 會把 config/<flavor>/GoogleService-Info.plist 複製成
// ios/Runner/GoogleService-Info.plist；這個產物不得進版控。
// 各 App 只檢查自己的 .gitignore（買家端有自己的同名測試）。
void main() {
  test('seller-app ignores the generated iOS GoogleService-Info.plist', () {
    final lines = File(
      '.gitignore',
    ).readAsLinesSync().map((l) => l.trim()).toList();
    expect(lines, contains('ios/Runner/GoogleService-Info.plist'));
  });
}
