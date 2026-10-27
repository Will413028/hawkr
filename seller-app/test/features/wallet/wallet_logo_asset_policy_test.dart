import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('order and wallet headers use the transparent brand mark SVG', () {
    const assetPath = 'assets/brand/logo-four-color.svg';
    final asset = File(assetPath);
    final walletSource = File(
      'lib/features/wallet/presentation/wallet_screen.dart',
    ).readAsStringSync();
    final orderSource = File(
      'lib/features/order/presentation/order_list_screen.dart',
    ).readAsStringSync();
    final svg = asset.readAsStringSync();

    expect(asset.existsSync(), isTrue);
    expect(asset.lengthSync(), greaterThan(0));
    expect(
      RegExp(r'leading: LogoHomeButton\(\)').allMatches(walletSource).length,
      1,
    );
    expect(orderSource, contains(': const LogoHomeButton()'));
    expect(walletSource, isNot(contains('logo-four-color.png')));
    expect(orderSource, isNot(contains('logo-four-color.png')));
    expect(svg, contains('viewBox="0 0 120 120"'));
    expect(svg, isNot(contains('<rect width="120" height="120"')));
    expect(svg.toLowerCase(), isNot(contains('#ffffff')));
  });
}
