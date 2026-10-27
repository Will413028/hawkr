import 'dart:io';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:flutter_test/flutter_test.dart';
import '../../support/default_error_text.dart';

ProfileEdit _edit({String nickname = '新暱稱', Date? birthDate}) => (
  id: '2001',
  etag: 'v1',
  displayName: nickname,
  phone: '0912345678',
  city: '臺北市',
  job: '設計師',
  gender: null,
  birthDate: birthDate,
);

void main() {
  final profile = buyer.Profile(
    id: '2001',
    displayName: '王小明',
    etag: 'v1',
    avatar: Image(uriTemplate: 'https://media.test/i/{preset}/key'),
  );
  final birth = profile.ensureBirthDate();
  birth.year = 1990;
  birth.month = 5;
  birth.day = 20;

  buyer.UpdateProfileRequest? updated;
  final transport = FakeTransportBuilder()
      .unary(
        BuyerAppService.getProfile,
        (_, _) => buyer.GetProfileResponse(profile: profile),
      )
      .unary(BuyerAppService.updateProfile, (request, _) {
        updated = request;
        return buyer.UpdateProfileResponse(profile: profile);
      })
      .build();
  final repository = AuthRepository(
    IdentityAppServiceClient(FakeTransportBuilder().build()),
    BuyerAppServiceClient(transport),
    defaults: zhDefaultErrorText,
  );

  test(
    'GetProfile maps the generated profile without losing buyer ID or birth date',
    () async {
      final data = await repository.getProfile();
      expect(data.id, '2001');
      expect(data.etag, 'v1');
      expect(data.displayName, '王小明');
      expect(data.birthDate?.year, 1990);
      expect(data.birthDate?.month, 5);
      expect(data.birthDate?.day, 20);
      expect(data.avatarUrl, 'https://media.test/i/avatar/key');
    },
  );

  test(
    'UpdateProfile sends etag and field mask, without an avatar field',
    () async {
      await repository.updateProfile(
        _edit(birthDate: Date(year: 1990, month: 5, day: 21)),
      );
      expect(updated?.profile.etag, 'v1');
      expect(updated?.profile.displayName, '新暱稱');
      expect(updated?.profile.birthDate.day, 21);
      expect(updated?.updateMask.paths, [
        'display_name',
        'phone',
        'city',
        'job',
        'gender',
        'birth_date',
      ]);
      expect(updated?.profile.hasAvatar(), isFalse);
    },
  );

  test('the mask is fixed: a form without a birthday still lists birth_date, '
      'and sends no birth date value', () async {
    updated = null;
    await repository.updateProfile(_edit());
    expect(updated?.updateMask.paths, contains('birth_date'));
    expect(updated?.profile.hasBirthDate(), isFalse);
  });

  test(
    'a new avatar goes through UpdateProfile with the staged token in the mask '
    'and resolves the URI template',
    () async {
      updated = null;
      final avatar = await repository.updateAvatar(
        id: '2001',
        stagedToken: 'staged-abc',
        etag: 'v1',
      );
      expect(updated?.profile.id, '2001');
      expect(updated?.profile.etag, 'v1');
      expect(updated?.profile.avatarStagedToken, 'staged-abc');
      expect(updated?.updateMask.paths, ['avatar_staged_token']);
      expect(avatar, 'https://media.test/i/avatar/key');
    },
  );

  test('an empty staged token is not sent', () async {
    updated = null;
    await expectLater(
      repository.updateAvatar(id: '2001', stagedToken: '', etag: 'v1'),
      throwsA(isA<ServerException>()),
    );
    expect(updated, isNull);
  });

  test('stale profile etag reaches the write and returns a conflict', () async {
    buyer.UpdateProfileRequest? sent;
    var reads = 0;
    final failing = AuthRepository(
      IdentityAppServiceClient(FakeTransportBuilder().build()),
      BuyerAppServiceClient(
        FakeTransportBuilder()
            .unary(BuyerAppService.getProfile, (_, _) {
              reads++;
              return buyer.GetProfileResponse(
                profile: buyer.Profile(id: '2001', etag: 'v2'),
              );
            })
            .unary(BuyerAppService.updateProfile, (request, _) {
              sent = request;
              throw ConnectException(Code.aborted, '版本衝突');
            })
            .build(),
      ),
      defaults: zhDefaultErrorText,
    );
    await expectLater(
      failing.updateProfile(_edit()),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          '資源衝突',
        ),
      ),
    );
    expect(reads, 0);
    expect(sent?.profile.etag, 'v1');
  });

  group('GetProfile errors', () {
    AuthRepository failingWith(Object error) => AuthRepository(
      IdentityAppServiceClient(FakeTransportBuilder().build()),
      BuyerAppServiceClient(
        FakeTransportBuilder()
            .unary(BuyerAppService.getProfile, (_, _) => throw error)
            .build(),
      ),
      defaults: zhDefaultErrorText,
    );

    test(
      'permission_denied is a server answer, not a network failure',
      () async {
        await expectLater(
          failingWith(
            ConnectException(Code.permissionDenied, 'not a buyer'),
          ).getProfile(),
          throwsA(
            isA<ServerException>().having(
              (e) => e.toString(),
              'message',
              '無權操作',
            ),
          ),
        );
      },
    );

    test('failed_precondition is not a network failure either', () async {
      await expectLater(
        failingWith(
          ConnectException(Code.failedPrecondition, 'buyer closed'),
        ).getProfile(),
        throwsA(isA<ServerException>()),
      );
    });

    test('only a missing response is a network failure', () async {
      await expectLater(
        failingWith(
          ConnectException.from(const SocketException('offline')),
        ).getProfile(),
        throwsA(isA<NetworkException>()),
      );
    });
  });
}
