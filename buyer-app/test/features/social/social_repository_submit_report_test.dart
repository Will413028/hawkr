import 'dart:typed_data';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/social/data/social_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// 依既有 repo 慣例（見 `test/features/follow/follow_repository_contract_test.dart`）：用自訂
/// `HttpClientAdapter` 攔截 Dio 呼叫直接驗證 URL/body/回應解析，不需 mockito。
class _ScriptedAdapter implements HttpClientAdapter {
  _ScriptedAdapter(this.body, [this.statusCode = 200]);

  final String body;
  final int statusCode;
  RequestOptions? lastRequest;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastRequest = options;
    return ResponseBody.fromString(
      body,
      statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Dio _dioWith(_ScriptedAdapter adapter) =>
    Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;

void main() {
  group('SocialRepository.submitReport', () {
    test('送出 POST /app/reports 完整 body 並從 data.report_id 回傳', () async {
      final adapter = _ScriptedAdapter(
        '{"code":0,"message":"success","data":{"reportId":"rpt_123"}}',
      );
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      final reportId = await SocialRepository(dio).submitReport(
        targetType: 'POST',
        targetId: '456',
        reasonCode: 'SPAM',
        description: '測試說明',
        // 裸 objectKey，不是完整 URL：Gateway 對外來域名與非 public/ 前綴回 400
        // （C3，10 檔 §25）。這裡放 URL 等於替「欄位名在說謊」的舊契約背書。
        evidenceKeys: ['public/report/2623900520/1.png'],
      );

      expect(adapter.lastRequest?.method, 'POST');
      expect(adapter.lastRequest?.path, '/app/reports');
      expect(adapter.lastRequest?.data, {
        'targetType': 'POST',
        'targetId': '456',
        'reasonCode': 'SPAM',
        'description': '測試說明',
        'evidenceKeys': ['public/report/2623900520/1.png'],
      });
      expect(reportId, 'rpt_123');
    });

    test('省略 description/evidenceKeys 時不送出該欄位', () async {
      final adapter = _ScriptedAdapter(
        '{"code":0,"message":"success","data":{"reportId":"rpt_456"}}',
      );
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await SocialRepository(dio).submitReport(
        targetType: 'REVIEW',
        targetId: '789',
        reasonCode: 'FRAUD',
      );

      final sentBody = adapter.lastRequest?.data as Map;
      expect(sentBody.containsKey('description'), isFalse);
      expect(sentBody.containsKey('evidenceKeys'), isFalse);
      // 舊欄位名已物理移除，不得再出現在 wire 上（C3 一次切，無相容層）。
      expect(sentBody.containsKey('evidence_urls'), isFalse);
    });

    test('code != 0 時拋出 ServerException，訊息優先讀 message 而非 msg', () async {
      final adapter = _ScriptedAdapter(
        '{"code":4001,"message":"M1","msg":"M2"}',
      );
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        SocialRepository(
          dio,
        ).submitReport(targetType: 'POST', targetId: '1', reasonCode: 'OTHER'),
        throwsA(
          isA<ServerException>().having((e) => e.message, 'message', 'M1'),
        ),
      );
    });

    test('body 為 null 時拋出 ServerException（與 block/unblock 靜默成功不同）', () async {
      final adapter = _ScriptedAdapter('');
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        SocialRepository(
          dio,
        ).submitReport(targetType: 'POST', targetId: '1', reasonCode: 'OTHER'),
        throwsA(
          isA<ServerException>().having((e) => e.message, 'message', '舉報失敗'),
        ),
      );
    });

    test('401 拋出 UnauthorizedException', () async {
      final adapter = _ScriptedAdapter('{"code":401}', 401);
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        SocialRepository(
          dio,
        ).submitReport(targetType: 'POST', targetId: '1', reasonCode: 'OTHER'),
        throwsA(isA<UnauthorizedException>()),
      );
    });

    test('409 拋出重複檢舉訊息', () async {
      final adapter = _ScriptedAdapter('{"code":409}', 409);
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        SocialRepository(
          dio,
        ).submitReport(targetType: 'POST', targetId: '1', reasonCode: 'OTHER'),
        throwsA(
          isA<ServerException>().having(
            (e) => e.message,
            'message',
            '您已檢舉過此內容，請勿重複送出',
          ),
        ),
      );
    });

    test('400 invalid target 且回應體有 message 時拋出 ServerException(該訊息)', () async {
      final adapter = _ScriptedAdapter('{"code":400,"message":"檢舉目標不存在"}', 400);
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        SocialRepository(
          dio,
        ).submitReport(targetType: 'POST', targetId: '1', reasonCode: 'OTHER'),
        throwsA(
          isA<ServerException>().having((e) => e.message, 'message', '檢舉目標不存在'),
        ),
      );
    });

    test('非 401/409 錯誤且回應體無 message 時 fallback 為 NetworkException', () async {
      final adapter = _ScriptedAdapter('{"code":500}', 500);
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        SocialRepository(
          dio,
        ).submitReport(targetType: 'POST', targetId: '1', reasonCode: 'OTHER'),
        throwsA(isA<NetworkException>()),
      );
    });
  });

  group('SocialRepository.getMyReports', () {
    test('429 rate limit 且回應體有 message 時拋出 ServerException(該訊息)', () async {
      final adapter = _ScriptedAdapter(
        '{"code":429,"message":"請求過於頻繁，請稍後再試"}',
        429,
      );
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        SocialRepository(dio).getMyReports(),
        throwsA(
          isA<ServerException>().having(
            (e) => e.message,
            'message',
            '請求過於頻繁，請稍後再試',
          ),
        ),
      );
    });

    test('401 拋出 UnauthorizedException', () async {
      final adapter = _ScriptedAdapter('{"code":401}', 401);
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        SocialRepository(dio).getMyReports(),
        throwsA(isA<UnauthorizedException>()),
      );
    });
  });
}
