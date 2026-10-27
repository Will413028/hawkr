import 'dart:convert';
import 'dart:io';

import 'package:hawkr_seller/features/chat/data/chat_picker_model.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('order picker preserves the order_prev message contract', () {
    final option = ChatOrderPickerOption({
      'orderNo': 'HK-100',
      'status': 'paid',
      'totalActualAmount': 320,
    });

    final payload =
        jsonDecode(option.toMessageContent()) as Map<String, dynamic>;

    expect(payload['messageType'], 'order_prev');
    expect(payload['orderNo'], 'HK-100');
    expect(payload['status'], 'paid');
    expect(payload['totalActualAmount'], 320);
    expect(
      () => option.pickerData['status'] = 'changed',
      throwsUnsupportedError,
    );
  });

  test('product picker preserves the product message contract', () {
    final option = ChatProductPickerOption({
      'id': 77,
      'skuName': '青江菜',
      'unitPrice': 45,
      'imageUrl': 'https://example.test/product.jpg',
    });

    final payload =
        jsonDecode(option.toMessageContent()) as Map<String, dynamic>;

    expect(payload, {
      'messageType': 'product',
      'productName': '青江菜',
      'price': 45,
      'imageUrl': 'https://example.test/product.jpg',
      'skuId': 77,
      'productId': 77,
    });
  });

  test('chat thread delegates attachment I/O to the extracted section', () {
    final thread = File(
      'lib/features/chat/presentation/chat_thread_screen.dart',
    ).readAsStringSync();
    final section = File(
      'lib/features/chat/presentation/chat_attachment_picker.dart',
    ).readAsStringSync();

    expect(thread, contains("import 'chat_attachment_picker.dart';"));
    expect(thread, isNot(contains('.apiGet(')));
    expect(thread, isNot(contains('showOrderPickerSheet(')));
    expect(section, contains('openChatOrderPicker'));
    expect(section, contains('openChatProductPicker'));
  });
}
