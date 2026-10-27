import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/core/networking/api_request_helper.dart';
import 'package:hawkr_seller/core/networking/api_result.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('ApiRequestHelper typed mutation contract', () {
    test('returns typed success for HTTP 2xx and business code 0', () async {
      final helper = _helperReturning(200, {
        'code': 0,
        'data': {'id': 7},
      });

      final result = await helper.apiPostResult('/mutation');

      expect(result, isA<ApiSuccess<Map<String, dynamic>>>());
      expect(result.valueOrThrow['data'], {'id': 7});
    });

    test('maps HTTP 2xx business failure to ApiException', () async {
      final helper = _helperReturning(200, {
        'code': 9240,
        'message': '物流狀態不允許此操作',
      });

      final result = await helper.apiPostResult('/mutation');

      expect(result, isA<ApiFailure<Map<String, dynamic>>>());
      expect(
        () => result.valueOrThrow,
        throwsA(
          isA<ApiException>()
              .having((error) => error.code, 'code', 9240)
              .having((error) => error.message, 'message', '物流狀態不允許此操作'),
        ),
      );
    });

    test('maps HTTP 4xx to ClientException', () async {
      final helper = _helperReturning(422, {'code': 422, 'message': '欄位格式錯誤'});

      await expectLater(
        helper.apiDeleteOrThrow('/mutation'),
        throwsA(
          isA<ClientException>()
              .having((error) => error.statusCode, 'statusCode', 422)
              .having((error) => error.message, 'message', '欄位格式錯誤'),
        ),
      );
    });

    test('maps HTTP 5xx to ServerException', () async {
      final helper = _helperReturning(503, {'code': 503, 'message': '服務暫時不可用'});

      await expectLater(
        helper.apiPostOrThrow('/mutation'),
        throwsA(
          isA<ServerException>()
              .having((error) => error.statusCode, 'statusCode', 503)
              .having((error) => error.message, 'message', '服務暫時不可用'),
        ),
      );
    });

    test('maps Dio timeout to TimeoutException', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = _TimeoutAdapter();
      final helper = _ApiHelper(dio);

      await expectLater(
        helper.apiPostOrThrow('/mutation'),
        throwsA(isA<TimeoutException>()),
      );
    });

    test('legacy map caller remains non-throwing', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = _TimeoutAdapter();
      final helper = _ApiHelper(dio);

      final result = await helper.apiPost('/mutation');

      expect(result['code'], -1);
    });
  });
}

_ApiHelper _helperReturning(int statusCode, Map<String, dynamic> body) {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
    ..httpClientAdapter = _ResponseAdapter(statusCode, body);
  return _ApiHelper(dio);
}

class _ApiHelper with ApiRequestHelper {
  _ApiHelper(this.dio);

  @override
  final Dio dio;
}

class _ResponseAdapter implements HttpClientAdapter {
  _ResponseAdapter(this.statusCode, this.body);

  final int statusCode;
  final Map<String, dynamic> body;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      jsonEncode(body),
      statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

class _TimeoutAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    throw DioException(
      requestOptions: options,
      type: DioExceptionType.connectionTimeout,
      message: 'timed out',
    );
  }

  @override
  void close({bool force = false}) {}
}
