import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  const auditedPolicies = <String, List<bool>>{
    'lib/shared/widgets/share_bottom_sheet.dart': [true],
    'lib/shared/widgets/text_size_settings_sheet.dart': [false],
    'lib/shared/widgets/chat_product_picker_sheet.dart': [false],
    'lib/shared/widgets/chat_order_picker_sheet.dart': [false],
    'lib/shared/widgets/fcm_debug_overlay.dart': [false],
    'lib/features/home/presentation/home_screen.dart': [false],
    'lib/features/home/presentation/home_widget/product_add_to_cart_sheet.dart':
        [true],
    'lib/features/product/presentation/search_screen.dart': [false],
    'lib/features/order/presentation/order_card_widgets/retry_payment_sheet.dart':
        [false],
    'lib/features/order/presentation/order_card_widgets/atm_info_sheet.dart': [
      false,
    ],
    'lib/features/cart/presentation/store_pickup_form_screen.dart': [false],
    'lib/features/cart/presentation/cart_screen.dart': [true],
    'lib/features/cart/presentation/widgets/checkout_payment_sheet.dart': [
      false,
    ],
    'lib/features/cart/presentation/widgets/address_picker_sheet.dart': [true],
    'lib/features/social/presentation/post_detail_screen.dart': [false],
    'lib/features/social/presentation/merchant_posts_feed_screen.dart': [false],
    'lib/features/chat/presentation/chat_thread_screen.dart': [false],
    'lib/features/seller/presentation/widgets/merchant_home_tab.dart': [false],
    'lib/features/address/presentation/address_management_screen.dart': [false],
    'lib/features/category/presentation/category_screen.dart': [
      false,
      false,
      false,
      false,
    ],
  };

  test('每個 modal bottom sheet 都明確指定是否使用 Material 拖曳把手', () {
    final dartFiles = Directory('lib')
        .listSync(recursive: true)
        .whereType<File>()
        .where((file) => file.path.endsWith('.dart'));
    final missingPolicy = <String>[];
    final callPattern = RegExp(r'\bshowModalBottomSheet(?:<[^\n]+>)?\s*\(');

    for (final file in dartFiles) {
      final source = file.readAsStringSync();
      for (final match in callPattern.allMatches(source)) {
        final builderIndex = source.indexOf('builder:', match.start);
        if (builderIndex < 0) {
          missingPolicy.add('${file.path}:${_lineOf(source, match.start)}');
          continue;
        }
        final callConfiguration = source.substring(match.start, builderIndex);
        if (!callConfiguration.contains('showDragHandle:')) {
          missingPolicy.add('${file.path}:${_lineOf(source, match.start)}');
        }
      }
    }

    expect(
      missingPolicy,
      isEmpty,
      reason:
          'Bottom sheet 必須明確設定 showDragHandle；有自製把手用 false，'
          '沒有自製把手才用 true。缺少設定：${missingPolicy.join(', ')}',
    );
  });

  test('23 個 bottom sheet 入口只顯示一個已盤點的拖曳把手', () {
    var auditedCallCount = 0;
    for (final entry in auditedPolicies.entries) {
      final source = File(entry.key).readAsStringSync();
      final actual = _dragHandlePolicies(source);
      auditedCallCount += actual.length;
      expect(actual, entry.value, reason: '拖曳把手分類錯誤：${entry.key}');
    }
    expect(auditedCallCount, 23);
  });
}

int _lineOf(String source, int offset) =>
    '\n'.allMatches(source.substring(0, offset)).length + 1;

List<bool> _dragHandlePolicies(String source) {
  final callPattern = RegExp(r'\bshowModalBottomSheet(?:<[^\n]+>)?\s*\(');
  return callPattern.allMatches(source).map((match) {
    final builderIndex = source.indexOf('builder:', match.start);
    final configuration = source.substring(match.start, builderIndex);
    final policy = RegExp(
      r'showDragHandle:\s*(true|false)',
    ).firstMatch(configuration);
    return policy!.group(1) == 'true';
  }).toList();
}
