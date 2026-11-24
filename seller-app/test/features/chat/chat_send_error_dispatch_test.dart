import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/chat/presentation/chat_send_error_dispatch.dart';

import 'package:hawkr_seller/core/exceptions/app_exception.dart';

import '../../support/connect_errors.dart';

String _defaults(Code c) => 'default:${c.name}';

void main() {
  for (final reason in ['CHAT_BLOCKED', 'STORE_CLOSED', 'RECEIVER_DELETED']) {
    test('$reason makes the thread read-only and shows the server text', () {
      final f = classifyChatSendError(
        reasonError(Code.failedPrecondition, reason, message: '伺服器句子'),
        defaults: _defaults,
      );
      expect(f.readOnly, isTrue);
      expect(f.message, '伺服器句子');
    });
  }

  test('other errors stay sendable and fall back to the code default', () {
    final f = classifyChatSendError(
      ConnectException(Code.unavailable, 'down'),
      defaults: _defaults,
    );
    expect(f.readOnly, isFalse);
    expect(f.message, 'default:unavailable');
  });

  test('moderation rejection shows the server sentence without read-only', () {
    final f = classifyChatSendError(
      reasonError(Code.invalidArgument, 'TEXT_REJECTED', message: '內容未通過'),
      defaults: _defaults,
    );
    expect(f.readOnly, isFalse);
    expect(f.message, '內容未通過');
  });

  test('an upload rejection keeps its own text without read-only', () {
    final f = classifyChatSendError(
      const UploadRejectedException(message: '圖片不符規定'),
      defaults: _defaults,
    );
    expect(f.readOnly, isFalse);
    expect(f.message, '圖片不符規定');
  });
}
