import 'dart:typed_data';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/moderation/data/block_repository.dart';
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
  group('BlockRepository.blockUser', () {
    test('送出 POST /app/blocks/{userId}，body 為 null 時靜默視為成功', () async {
      final adapter = _ScriptedAdapter('');
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await BlockRepository(dio).blockUser('42');

      expect(adapter.lastRequest?.method, 'POST');
      expect(adapter.lastRequest?.path, '/app/blocks/42');
    });

    test('code != 0 時拋出 ServerException，訊息優先讀 message 而非 msg', () async {
      final adapter = _ScriptedAdapter(
        '{"code":4001,"message":"已達封鎖上限","msg":"legacy"}',
      );
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        BlockRepository(dio).blockUser('42'),
        throwsA(
          isA<ServerException>().having((e) => e.message, 'message', '已達封鎖上限'),
        ),
      );
    });

    test('401 拋出 UnauthorizedException', () async {
      final adapter = _ScriptedAdapter(
        '{"code":401,"message":"unauthorized"}',
        401,
      );
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        BlockRepository(dio).blockUser('42'),
        throwsA(isA<UnauthorizedException>()),
      );
    });

    test('非 401 的其他錯誤且回應體有 message 時拋出 ServerException(該訊息)', () async {
      final adapter = _ScriptedAdapter('{"code":500,"message":"boom"}', 500);
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        BlockRepository(dio).blockUser('42'),
        throwsA(
          isA<ServerException>().having((e) => e.message, 'message', 'boom'),
        ),
      );
    });

    test('非 401 的其他錯誤且回應體無 message 時 fallback 為 NetworkException', () async {
      final adapter = _ScriptedAdapter('{"code":500}', 500);
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        BlockRepository(dio).blockUser('42'),
        throwsA(isA<NetworkException>()),
      );
    });
  });

  group('BlockRepository.unblockUser', () {
    test('送出 DELETE /app/blocks/{userId}，body 為 null 時靜默視為成功', () async {
      final adapter = _ScriptedAdapter('');
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await BlockRepository(dio).unblockUser('42');

      expect(adapter.lastRequest?.method, 'DELETE');
      expect(adapter.lastRequest?.path, '/app/blocks/42');
    });

    test('非 401 錯誤且回應體有 message 時拋出 ServerException(該訊息)', () async {
      final adapter = _ScriptedAdapter(
        '{"code":400,"message":"目前並未封鎖此使用者"}',
        400,
      );
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        BlockRepository(dio).unblockUser('42'),
        throwsA(
          isA<ServerException>().having(
            (e) => e.message,
            'message',
            '目前並未封鎖此使用者',
          ),
        ),
      );
    });
  });

  group('BlockRepository.getBlockedUsers', () {
    test('送出 GET /app/blocks 並帶 page/page_size query，解析 data.records', () async {
      final adapter = _ScriptedAdapter('''
      {
        "code": 0,
        "message": "success",
        "data": {
          "records": [
            {"id": "857045288972403572", "blocked_id": "1001", "create_time_ms": 1721800000000},
            {"id": "857045288972403573", "blocked_id": "1002", "create_time_ms": 1721800001000}
          ]
        }
      }
      ''');
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      final result = await BlockRepository(
        dio,
      ).getBlockedUsers(page: 2, pageSize: 10);

      expect(adapter.lastRequest?.method, 'GET');
      expect(adapter.lastRequest?.path, '/app/blocks');
      expect(adapter.lastRequest?.queryParameters, {
        'page': 2,
        'page_size': 10,
      });
      expect(
        result.map((u) => u.userId),
        ['1001', '1002'],
        reason:
            'userId 必須來自 blocked_id（後端 UserBlock proto 唯一的使用者 id 欄位），不是 id（封鎖紀錄自己的 TSID）',
      );
    });

    test('body 為 null 時回傳空陣列', () async {
      final adapter = _ScriptedAdapter('');
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      final result = await BlockRepository(dio).getBlockedUsers();

      expect(result, isEmpty);
    });

    test('429 rate limit 且回應體有 message 時拋出 ServerException(該訊息)', () async {
      final adapter = _ScriptedAdapter(
        '{"code":429,"message":"請求過於頻繁，請稍後再試"}',
        429,
      );
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await expectLater(
        BlockRepository(dio).getBlockedUsers(),
        throwsA(
          isA<ServerException>().having(
            (e) => e.message,
            'message',
            '請求過於頻繁，請稍後再試',
          ),
        ),
      );
    });
  });
}
