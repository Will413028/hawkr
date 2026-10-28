import 'dart:async';

import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/core/networking/ambient_shipping_temperature.dart';
import 'package:hawkr_seller/features/settings/data/cvs_shipping_mutation.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('CvsShippingMutationCoordinator', () {
    test('activation request carries the ambient goodsType', () async {
      String? path;
      Map<String, dynamic>? body;
      final coordinator = CvsShippingMutationCoordinator((
        requestPath,
        requestBody,
      ) async {
        path = requestPath;
        body = requestBody;
        return {
          'code': 0,
          'data': {'onboardingId': 'cvs-onboarding'},
        };
      });

      final outcome = await coordinator.apply([
        _mutation(initialEnabled: false),
      ]);

      expect(outcome.isSuccess, isTrue);
      expect(path, '/api/v1/app/shipping_onboardings/cvs');
      expect(body?['goodsType'], '3201');
      expect(body?['goodsType'], kAmbientShippingTemperature);
      expect(outcome.completed.single.onboardingId, 'cvs-onboarding');
    });

    test('failure keeps the mutation pending', () async {
      final coordinator = CvsShippingMutationCoordinator(
        (_, __) => throw const ApiException(code: 9240, message: '開通失敗'),
      );

      final outcome = await coordinator.apply([
        _mutation(initialEnabled: false),
      ]);

      expect(outcome.isSuccess, isFalse);
      expect(outcome.completed, isEmpty);
      expect(outcome.pending, hasLength(1));
      expect(outcome.error, isA<ApiException>());
      expect(coordinator.isRunning, isFalse);
    });

    test('double submit performs only one HTTP mutation', () async {
      final response = Completer<Map<String, dynamic>>();
      var requestCount = 0;
      final coordinator = CvsShippingMutationCoordinator((_, __) {
        requestCount++;
        return response.future;
      });
      final mutations = [_mutation(initialEnabled: false)];

      final first = coordinator.apply(mutations);
      final duplicate = await coordinator.apply(mutations);

      expect(duplicate.alreadyRunning, isTrue);
      expect(duplicate.isSuccess, isFalse);
      expect(requestCount, 1);

      response.complete({'code': 0});
      expect((await first).isSuccess, isTrue);
      expect(coordinator.isRunning, isFalse);
    });

    test('existing onboarding uses typed toggle request', () async {
      String? path;
      Map<String, dynamic>? body;
      final coordinator = CvsShippingMutationCoordinator((
        requestPath,
        requestBody,
      ) async {
        path = requestPath;
        body = requestBody;
        return {'code': 0};
      });

      final outcome = await coordinator.apply([
        _mutation(
          initialEnabled: true,
          targetEnabled: false,
          onboardingId: 'cvs-id',
        ),
      ]);

      expect(outcome.isSuccess, isTrue);
      expect(path, '/api/v1/app/shipping_onboardings/cvs-id/toggle');
      expect(body, {'enabled': false});
    });

    test(
      'disable without onboarding id fails instead of reporting success',
      () async {
        var requestCount = 0;
        final coordinator = CvsShippingMutationCoordinator((_, __) async {
          requestCount++;
          return {'code': 0};
        });

        final outcome = await coordinator.apply([
          _mutation(initialEnabled: true, targetEnabled: false),
        ]);

        expect(outcome.isSuccess, isFalse);
        expect(outcome.pending, hasLength(1));
        expect(outcome.error, isA<ApiException>());
        expect(outcome.error!.message, contains('請重新載入'));
        expect(requestCount, 0);
      },
    );
  });
}

CvsGoodsTypeMutation _mutation({
  required bool initialEnabled,
  bool targetEnabled = true,
  String? onboardingId,
}) {
  return CvsGoodsTypeMutation(
    initialEnabled: initialEnabled,
    targetEnabled: targetEnabled,
    onboardingId: onboardingId,
    createBody: const {
      'senderName': '王小明',
      'senderPhone': '0912345678',
      'returnStoreCode': '123456',
    },
  );
}
