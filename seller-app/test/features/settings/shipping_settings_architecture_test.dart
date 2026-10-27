import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/features/settings/data/shipping_settings_remote_service.dart';
import 'package:hawkr_seller/features/settings/data/shipping_settings_repository.dart';
import 'package:hawkr_seller/features/settings/presentation/shipping_settings_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test(
    'shipping service preserves bootstrap contracts and immutability',
    () async {
      final adapter = _RoutingJsonAdapter({
        '/api/v1/app/shipping_onboardings': _JsonResponse(200, {
          'code': 0,
          'data': [
            {'shippingType': '3652', 'enabled': true},
          ],
        }),
      });
      final service = ShippingSettingsRemoteService(
        Dio(BaseOptions(baseUrl: 'https://example.test'))
          ..httpClientAdapter = adapter,
      );

      final onboardings = await service.fetchOnboardings();

      expect(onboardings.single['shippingType'], '3652');
      expect(
        () => onboardings.single['enabled'] = false,
        throwsUnsupportedError,
      );
    },
  );

  test('shipping service converts transport and business failures', () {
    final transportService = ShippingSettingsRemoteService(
      Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = _RoutingJsonAdapter({
          '/api/v1/app/shipping_onboardings': _JsonResponse(503, {
            'message': '物流服務忙碌中',
          }),
        }),
    );
    final businessService = ShippingSettingsRemoteService(
      Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = _RoutingJsonAdapter({
          '/api/v1/app/shipping_onboardings': _JsonResponse(200, {
            'code': 9240,
            'message': '帳號停權',
          }),
        }),
    );

    expect(transportService.fetchOnboardings, throwsA(isA<ServerException>()));
    expect(businessService.fetchOnboardings, throwsA(isA<ApiException>()));
  });

  test(
    'shipping controller exposes typed commands and immutable bootstrap',
    () async {
      final repository = _FakeShippingSettingsRepository();
      final container = ProviderContainer(
        overrides: [
          shippingSettingsRepositoryProvider.overrideWithValue(repository),
        ],
      );
      addTearDown(container.dispose);
      final controller = container.read(
        shippingSettingsControllerProvider.notifier,
      );

      final bootstrap = await controller.loadBootstrap();
      final result = await controller.activateHomeDelivery({
        'senderName': '測試商家',
      });
      final storeMap = await controller.loadStoreMapSession();

      expect(
        () => bootstrap.onboardings.single['enabled'] = false,
        throwsUnsupportedError,
      );
      expect(result.isSuccess, isTrue);
      expect(storeMap?['redirectUrl'], 'https://example.test/store-map');
      expect(repository.homeDeliveryBody?['senderName'], '測試商家');
    },
  );

  test('shipping presentation has no low-level I/O and TGOS lives in data', () {
    final source = File(
      'lib/features/settings/presentation/shipping_settings/'
      'shipping_settings_screen.dart',
    ).readAsStringSync();

    expect(source, isNot(contains('.apiGet(')));
    expect(source, isNot(contains('.apiPost(')));
    expect(source, isNot(contains('/api/v1/app/sellers/me')));
    expect(source, isNot(contains('sellerRepositoryProvider')));
    expect(source, isNot(contains('productRepositoryProvider')));
    expect(source, contains('shippingSettingsControllerProvider'));
    expect(
      File('lib/features/settings/data/tgos_address_service.dart').existsSync(),
      isTrue,
    );
    expect(
      File(
        'lib/features/settings/presentation/shipping_settings/data/'
        'tgos_address_service.dart',
      ).existsSync(),
      isFalse,
    );
  });
}

class _FakeShippingSettingsRepository implements ShippingSettingsRepository {
  Map<String, dynamic>? homeDeliveryBody;

  @override
  Future<ShippingCommandResult> activateHomeDelivery(
    Map<String, dynamic> body,
  ) async {
    homeDeliveryBody = Map<String, dynamic>.of(body);
    return const ShippingCommandResult.success();
  }

  @override
  Future<ShippingSettingsBootstrap> loadBootstrap() async {
    return ShippingSettingsBootstrap(
      onboardings: const [
        {'shippingType': '3652', 'enabled': true},
      ],
    );
  }

  @override
  Future<Map<String, dynamic>?> loadStoreMapSession() async {
    return const {'redirectUrl': 'https://example.test/store-map'};
  }

  @override
  Future<Map<String, dynamic>> sendMutation(
    String path,
    Map<String, dynamic> body,
  ) async {
    return const {'code': 0};
  }
}

class _JsonResponse {
  const _JsonResponse(this.statusCode, this.payload);

  final int statusCode;
  final Map<String, dynamic> payload;
}

class _RoutingJsonAdapter implements HttpClientAdapter {
  _RoutingJsonAdapter(this.responses);

  final Map<String, _JsonResponse> responses;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final response = responses[options.path];
    if (response == null) {
      return ResponseBody.fromString(
        jsonEncode({'message': 'missing fixture'}),
        500,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    return ResponseBody.fromString(
      jsonEncode(response.payload),
      response.statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
