import 'dart:async';

import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/features/settings/data/cvs_shipping_mutation.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('CvsShippingMutationCoordinator', () {
    test('first failure keeps both goods types pending', () async {
      final coordinator = CvsShippingMutationCoordinator(
        (_, __) => throw const ApiException(code: 9240, message: '冷凍開通失敗'),
      );

      final outcome = await coordinator.apply(_bothEnableMutations());

      expect(outcome.isSuccess, isFalse);
      expect(outcome.completed, isEmpty);
      expect(outcome.pending.map((item) => item.type), [
        CvsGoodsType.frozen,
        CvsGoodsType.ambient,
      ]);
      expect(outcome.error, isA<ApiException>());
      expect(coordinator.isRunning, isFalse);
    });

    test(
      'second failure checkpoints first success and retry sends only pending step',
      () async {
        final sentGoodsTypes = <String>[];
        var ambientAttempts = 0;
        final coordinator = CvsShippingMutationCoordinator((_, body) async {
          final goodsType = body['goodsType'] as String;
          sentGoodsTypes.add(goodsType);
          if (goodsType == 'AMBIENT' && ambientAttempts++ == 0) {
            throw const ServerException(message: '服務暫時不可用', statusCode: 503);
          }
          return {
            'code': 0,
            'data': {
              'onboardingId': goodsType == 'FROZEN'
                  ? 'frozen-onboarding'
                  : 'ambient-onboarding',
            },
          };
        });

        final firstOutcome = await coordinator.apply(_bothEnableMutations());

        expect(firstOutcome.isSuccess, isFalse);
        expect(firstOutcome.completed, hasLength(1));
        expect(
          firstOutcome.completed.single.mutation.type,
          CvsGoodsType.frozen,
        );
        expect(firstOutcome.completed.single.onboardingId, 'frozen-onboarding');
        expect(firstOutcome.pending.map((item) => item.type), [
          CvsGoodsType.ambient,
        ]);

        final retryOutcome = await coordinator.apply([
          _mutation(
            type: CvsGoodsType.frozen,
            goodsType: 'FROZEN',
            initialEnabled: true,
          ),
          _mutation(
            type: CvsGoodsType.ambient,
            goodsType: 'AMBIENT',
            initialEnabled: false,
          ),
        ]);

        expect(retryOutcome.isSuccess, isTrue);
        expect(sentGoodsTypes, ['FROZEN', 'AMBIENT', 'AMBIENT']);
        expect(
          sentGoodsTypes.where((value) => value == 'FROZEN'),
          hasLength(1),
          reason: 'successful first step must not be resent on retry',
        );
        expect(coordinator.isRunning, isFalse);
      },
    );

    test('double submit performs only one HTTP mutation', () async {
      final response = Completer<Map<String, dynamic>>();
      var requestCount = 0;
      final coordinator = CvsShippingMutationCoordinator((_, __) {
        requestCount++;
        return response.future;
      });
      final mutations = [
        _mutation(
          type: CvsGoodsType.frozen,
          goodsType: 'FROZEN',
          initialEnabled: false,
        ),
      ];

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
          type: CvsGoodsType.ambient,
          goodsType: 'AMBIENT',
          initialEnabled: true,
          targetEnabled: false,
          onboardingId: 'ambient-id',
        ),
      ]);

      expect(outcome.isSuccess, isTrue);
      expect(path, '/api/v1/app/shipping_onboardings/ambient-id/toggle');
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
          _mutation(
            type: CvsGoodsType.frozen,
            goodsType: 'FROZEN',
            initialEnabled: true,
            targetEnabled: false,
          ),
        ]);

        expect(outcome.isSuccess, isFalse);
        expect(outcome.pending.single.type, CvsGoodsType.frozen);
        expect(outcome.error, isA<ApiException>());
        expect(outcome.error!.message, contains('請重新載入'));
        expect(requestCount, 0);
      },
    );
  });
}

List<CvsGoodsTypeMutation> _bothEnableMutations() => [
  _mutation(
    type: CvsGoodsType.frozen,
    goodsType: 'FROZEN',
    initialEnabled: false,
  ),
  _mutation(
    type: CvsGoodsType.ambient,
    goodsType: 'AMBIENT',
    initialEnabled: false,
  ),
];

CvsGoodsTypeMutation _mutation({
  required CvsGoodsType type,
  required String goodsType,
  required bool initialEnabled,
  bool targetEnabled = true,
  String? onboardingId,
}) {
  return CvsGoodsTypeMutation(
    type: type,
    goodsType: goodsType,
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
