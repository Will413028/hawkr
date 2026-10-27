import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('all buyer chat entry points use the semantic chat icon', () {
    final bubble = File(
      'lib/shared/widgets/chat_bubble_icon.dart',
    ).readAsStringSync();
    final badge = File(
      'lib/shared/widgets/chat_icon_with_badge.dart',
    ).readAsStringSync();
    final cart = File(
      'lib/features/cart/presentation/cart_screen.dart',
    ).readAsStringSync();

    expect(bubble, contains('HkIconName.chat'));
    expect(badge, contains('HkIconName.chat'));
    expect(badge, isNot(contains('iconWidget')));
    expect(cart, isNot(contains('assets/cart/')));
  });

  test('branded top actions and address rows draw static HkIcon glyphs', () {
    final cart = File(
      'lib/shared/widgets/cart_icon_with_badge.dart',
    ).readAsStringSync();
    final notification = File(
      'lib/shared/widgets/top_navigation.dart',
    ).readAsStringSync();
    final homeAddress = File(
      'lib/features/home/presentation/home_widget/home_address_section.dart',
    ).readAsStringSync();
    final categoryAddress = File(
      'lib/features/category/presentation/category_landing_screen.dart',
    ).readAsStringSync();
    final merchantHeader = File(
      'lib/features/seller/presentation/widgets/merchant_header.dart',
    ).readAsStringSync();

    expect(cart, contains('HkIconName.cart'));
    expect(notification, contains('HkIconName.notification'));
    expect(homeAddress, contains('HkIconName.truck'));
    expect(categoryAddress, contains('HkIconName.truck'));
    expect(merchantHeader, contains('icon: HkIconName.cart'));
    expect(merchantHeader, contains('icon: HkIconName.chat'));
    for (final source in [
      cart,
      notification,
      homeAddress,
      categoryAddress,
      merchantHeader,
    ]) {
      expect(source, isNot(contains('replayId')));
      expect(source, isNot(contains('_replayId')));
    }
  });
}
