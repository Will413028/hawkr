import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/wallet/data/wallet_remote_service.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// 領取優惠券的端點契約。
///
/// 背景：舊碼打 `POST /app/coupons/{id}/receive`，Gateway 註冊的是
/// `CouponsClaim = "/:template_id/claim"`（`gateway/internal/router/routes_coupon.go`）。
/// 2026-08-05 對 dev 實打：`/receive` 回 gin 純文字 404、`/claim` 回 JSON 業務碼，
/// 代表舊路徑根本不存在 → 領券永遠失敗。
class _RecordingAdapter implements HttpClientAdapter {
  RequestOptions? lastRequest;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastRequest = options;
    return ResponseBody.fromString(
      jsonEncode(const <String, dynamic>{
        'code': 0,
        'message': 'success',
        'data': <String, dynamic>{},
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test('receiveCoupon 打 Gateway 真的有的 /claim，不是 /receive', () async {
    final adapter = _RecordingAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;
    addTearDown(dio.close);

    await DioWalletRemoteService(dio).receiveCoupon('7490317712170336958');

    expect(adapter.lastRequest?.method, 'POST');
    expect(adapter.lastRequest?.path, '/app/coupons/7490317712170336958/claim');
    expect(adapter.lastRequest?.path, isNot(endsWith('/receive')));
  });
}
