import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/default_error_text.dart';

ProfileEdit _edit(Gender? gender) => (
  id: '2001',
  etag: 'v1',
  displayName: '王小明',
  phone: '0912345678',
  city: '臺北市',
  job: '設計師',
  gender: gender,
  birthDate: null,
);

void main() {
  buyer.UpdateProfileRequest? sent;
  buyer.Profile stored = buyer.Profile(id: '2001', etag: 'v1');

  final repository = AuthRepository(
    IdentityAppServiceClient(FakeTransportBuilder().build()),
    BuyerAppServiceClient(
      FakeTransportBuilder()
          .unary(
            BuyerAppService.getProfile,
            (_, _) => buyer.GetProfileResponse(profile: stored),
          )
          .unary(BuyerAppService.updateProfile, (request, _) {
            sent = request;
            return buyer.UpdateProfileResponse(profile: stored);
          })
          .build(),
    ),
    defaults: zhDefaultErrorText,
  );

  setUp(() {
    sent = null;
    stored = buyer.Profile(id: '2001', etag: 'v1');
  });

  test('a chosen gender is sent as that enum value', () async {
    await repository.updateProfile(_edit(Gender.GENDER_FEMALE));
    expect(sent?.profile.hasGender(), isTrue);
    expect(sent?.profile.gender, Gender.GENDER_FEMALE);
    expect(sent?.updateMask.paths, contains('gender'));
  });

  test('no gender clears: gender is in the mask but is not set, and '
      'GENDER_UNSPECIFIED is never sent', () async {
    await repository.updateProfile(_edit(null));
    expect(sent?.profile.hasGender(), isFalse);
    expect(sent?.updateMask.paths, contains('gender'));
  });

  test('the mask is exactly the six fields the form owns', () async {
    await repository.updateProfile(_edit(Gender.GENDER_MALE));
    expect(sent?.updateMask.paths, [
      'display_name',
      'phone',
      'city',
      'job',
      'gender',
      'birth_date',
    ]);
  });

  test('GetProfile reads an unset gender as null and OTHER as OTHER', () async {
    expect((await repository.getProfile()).gender, isNull);

    stored = buyer.Profile(
      id: '2001',
      etag: 'v1',
      gender: Gender.GENDER_UNSPECIFIED,
    );
    expect((await repository.getProfile()).gender, isNull);

    stored = buyer.Profile(id: '2001', etag: 'v1', gender: Gender.GENDER_OTHER);
    expect((await repository.getProfile()).gender, Gender.GENDER_OTHER);
  });
}
