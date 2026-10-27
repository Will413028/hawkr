import 'dart:convert';

import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('detects 9240 from decoded, string, and byte response bodies', () {
    const body = '{"code":9240,"message":"blocked"}';

    expect(isSellerSuspensionPayload(const {'code': 9240}), isTrue);
    expect(isSellerSuspensionPayload(body), isTrue);
    expect(isSellerSuspensionPayload(utf8.encode(body)), isTrue);
  });

  test('ignores ordinary binary and non-suspension payloads', () {
    expect(isSellerSuspensionPayload(const <int>[0, 1, 2, 3]), isFalse);
    expect(isSellerSuspensionPayload(const {'code': 0}), isFalse);
    expect(isSellerSuspensionPayload('not json'), isFalse);
  });
}
