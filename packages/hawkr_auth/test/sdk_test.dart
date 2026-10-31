import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_auth/sdk.dart';

void main() {
  group('isLineLoginCanceled', () {
    test('authorizeFailed（3003）且原因是使用者取消', () {
      expect(
        isLineLoginCanceled(
          PlatformException(
            code: '3003',
            message: 'The user cancelled the authorization.',
          ),
        ),
        isTrue,
      );
    });

    test('其他原因的 authorizeFailed 與其他錯誤碼不是取消', () {
      expect(
        isLineLoginCanceled(
          PlatformException(code: '3003', message: 'invalid redirect'),
        ),
        isFalse,
      );
      expect(
        isLineLoginCanceled(
          PlatformException(code: '1002', message: 'cancelled'),
        ),
        isFalse,
      );
    });
  });
}
