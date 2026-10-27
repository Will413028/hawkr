/// `POST /app/reviews`（提交訂單商品評論）的 wire 契約。
///
/// backend#2198 改了兩件事：
///   1. HTTP status **201 → 200**
///   2. data 的 id 欄名 `review_id` → `reviewId`
///
/// 第 1 點是這支測試存在的理由。`addReview` 原本寫成
/// `if (resp.statusCode == 201 || code == 0)` —— 兩個條件是 or，所以 201 消失
/// 之後它**仍然會過**（靠 `code == 0` 接住），改動不會被任何既有測試抓到，
/// 而那個恆為 false 的 201 會一直留著暗示「HTTP status 也是成功判準」。
/// 業務結果一律看 envelope 的 `code`，不看 HTTP status
/// （見 reference_gateway_bizcode_collapses_to_409：業務碼還會塌縮成 409）。
///
/// ⚠️ 下面的 fixture 是 2026-08-05 對本機 gateway（含 backend#2198）的**實打回應**：
///   POST /api/v1/app/reviews
///   → HTTP/1.1 200 OK
///   → {"data":{"reviewId":"7490697432373003765"},"message":"success","code":0}
/// （該筆測試資料已於當下還原）
library;

import 'dart:typed_data';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/review/data/review_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

const _successBody =
    '{"data":{"reviewId":"7490697432373003765"},"message":"success","code":0}';

/// 業務拒絕：gateway 把業務碼塌縮成 HTTP 409（428 = 評論期限已過）。
const _windowExpiredBody =
    '{"message":"review window expired","messageZh":"review window expired",'
    '"traceId":"4191023bbd3a6b8ddf648b9813954a16","code":428}';

class _ScriptedAdapter implements HttpClientAdapter {
  _ScriptedAdapter(this.body, this.status);

  final String body;
  final int status;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      body,
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Dio _dioWith(_ScriptedAdapter adapter) => Dio(
  BaseOptions(
    baseUrl: 'https://test.local',
    // 讓 409 進到 addReview 的正常回應路徑而不是 DioException，
    // 才驗得到「業務碼分流」這一層。
    validateStatus: (s) => s != null && s < 500,
  ),
)..httpClientAdapter = adapter;

void main() {
  group('ReviewRepository.addReview 的 envelope 契約', () {
    test('HTTP 200 + code 0 → 成功', () async {
      final dio = _dioWith(_ScriptedAdapter(_successBody, 200));
      addTearDown(dio.close);

      final review = await ReviewRepository(dio).addReview(
        orderNo: 'O041282666',
        orderItemId: 840784674062254122,
        skuId: 3,
        rating: 5,
        content: 'T4 契約驗證用評論',
      );
      expect(review.rating, 5);
      expect(review.content, 'T4 契約驗證用評論');
    });

    test('HTTP 201 已不再出現；若只靠 status 判定，成功路徑會漏接', () async {
      // 鑑別力：這條釘的是「不准把 HTTP status 當成功判準」。
      // 201 + code 0 仍要成功（因為判準是 code），
      // 而下一條的 200 + code≠0 必須失敗 —— 兩條合起來才唯一決定判準是 code。
      final dio = _dioWith(_ScriptedAdapter(_successBody, 201));
      addTearDown(dio.close);

      final review = await ReviewRepository(dio).addReview(
        orderNo: 'O041282666',
        orderItemId: 840784674062254122,
        skuId: 3,
        rating: 5,
        content: 'x',
      );
      expect(review.rating, 5);
    });

    test('HTTP 200 但 code≠0 → 必須失敗（不得因為 status 是 2xx 就當成功）', () async {
      final dio = _dioWith(_ScriptedAdapter(_windowExpiredBody, 200));
      addTearDown(dio.close);

      await expectLater(
        ReviewRepository(dio).addReview(
          orderNo: 'O041282666',
          orderItemId: 840784674062254122,
          skuId: 3,
          rating: 5,
          content: 'x',
        ),
        throwsA(isA<ServerException>()),
      );
    });

    test('業務拒絕塌縮成 HTTP 409 → 失敗', () async {
      final dio = _dioWith(_ScriptedAdapter(_windowExpiredBody, 409));
      addTearDown(dio.close);

      await expectLater(
        ReviewRepository(dio).addReview(
          orderNo: 'O041282666',
          orderItemId: 840784674062254122,
          skuId: 3,
          rating: 5,
          content: 'x',
        ),
        throwsA(isA<ServerException>()),
      );
    });
  });
}
