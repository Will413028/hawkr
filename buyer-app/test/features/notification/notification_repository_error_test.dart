import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/notification/data/notification_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('notification transport timeout remains a TimeoutException', () async {
    final dio = _dioRejecting(DioExceptionType.connectionTimeout);
    final repository = NotificationRepository(dio);

    await expectLater(
      repository.getNotifications(),
      throwsA(isA<TimeoutException>()),
    );
  });

  test('notification connection failure remains a NetworkException', () async {
    final dio = _dioRejecting(DioExceptionType.connectionError);
    final repository = NotificationRepository(dio);

    await expectLater(
      repository.getNotifications(),
      throwsA(isA<NetworkException>()),
    );
  });

  test('notification business failure remains a ServerException', () async {
    // envelope 的訊息欄名是 `message`（backend#2198 起 `msg` 已從 wire 移除）。
    final repository = NotificationRepository(
      _dioResolving({'code': 7102, 'message': 'business rejected'}),
    );

    await expectLater(
      repository.getNotifications(),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          'business rejected',
        ),
      ),
    );
  });

  test('舊 msg 欄名不再被讀取（沒有相容 fallback）', () {
    // 鑑別力：上一條若靠 `data['msg'] ?? data['message']` 通過，這條會失敗。
    final repository = NotificationRepository(
      _dioResolving({'code': 7102, 'msg': 'business rejected'}),
    );

    return expectLater(
      repository.getNotifications(),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          '獲取通知列表失敗',
        ),
      ),
    );
  });
}

Dio _dioResolving(Map<String, dynamic> body) {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.invalid'));
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        handler.resolve(
          Response<Map<String, dynamic>>(
            requestOptions: options,
            statusCode: 200,
            data: body,
          ),
        );
      },
    ),
  );
  return dio;
}

Dio _dioRejecting(DioExceptionType type) {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.invalid'));
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        handler.reject(DioException(requestOptions: options, type: type));
      },
    ),
  );
  return dio;
}
