import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/features/seller/data/seller_agreements_repository.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

SellerAgreement _agreement(AgreementKind kind, String version, String url) =>
    SellerAgreement(kind: kind, version: version, url: url);

final _privacy = _agreement(
  AgreementKind.AGREEMENT_KIND_KYC_PRIVACY_NOTICE,
  'p1',
  'https://www.example.test/privacy',
);
final _payuni = _agreement(
  AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS,
  'u1',
  'https://www.example.test/payuni',
);
final _seller = _agreement(
  AgreementKind.AGREEMENT_KIND_SELLER_TERMS,
  's1',
  'https://www.example.test/seller',
);

SellerAgreementSet _set(List<SellerAgreement> agreements) =>
    SellerAgreementSet(agreements);

void main() {
  test('KYC 的個資告知與 PayUni 條款：版本與網址取自伺服器，同意剛好這兩份', () {
    final agreements = _set([_seller, _privacy, _payuni]).kycAgreements();

    expect(agreements.privacyNotice.url, 'https://www.example.test/privacy');
    expect(agreements.payuniTerms.url, 'https://www.example.test/payuni');
    expect(agreements.acceptances.map((a) => (a.kind, a.version)), [
      (AgreementKind.AGREEMENT_KIND_KYC_PRIVACY_NOTICE, 'p1'),
      (AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS, 'u1'),
    ]);
  });

  test('缺任何一份、或版本與網址是空的，丟 StateError，不送空版本', () {
    expect(() => _set([_seller, _privacy]).kycAgreements(), throwsStateError);
    expect(
      () => _set([
        _privacy,
        _agreement(AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS, '', 'https://x'),
      ]).kycAgreements(),
      throwsStateError,
    );
    expect(
      () => _set([
        _privacy,
        _agreement(AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS, 'u1', ''),
      ]).kycAgreements(),
      throwsStateError,
    );
  });

  test('食品業者登錄的告知：版本與網址取自伺服器，缺少時丟 StateError', () {
    final notice = _agreement(
      AgreementKind.AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE,
      'n1',
      'https://www.example.test/privacy#seller-food-registration-notice',
    );
    expect(_set([_seller, notice]).foodRegistrationNotice().version, 'n1');
    expect(
      () => _set([_seller, _privacy]).foodRegistrationNotice(),
      throwsStateError,
    );
  });

  test('賣家契約仍照原樣取', () {
    expect(_set([_seller]).sellerTerms().version, 's1');
    expect(() => _set([_privacy]).sellerTerms(), throwsStateError);
  });

  test('賣家契約與 KYC 兩份文件共用同一次 GetSellerAgreements', () async {
    var calls = 0;
    final container = ProviderContainer(
      overrides: [
        sellerPublicClientProvider.overrideWithValue(
          SellerPublicServiceClient(
            FakeTransportBuilder().unary(
              SellerPublicService.getSellerAgreements,
              (_, _) {
                calls++;
                return GetSellerAgreementsResponse(
                  agreements: [_seller, _privacy, _payuni],
                );
              },
            ).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    expect((await container.read(sellerTermsProvider.future)).version, 's1');
    expect(
      (await container.read(
        kycAgreementsProvider.future,
      )).privacyNotice.version,
      'p1',
    );
    expect(calls, 1);

    container.invalidate(sellerAgreementsProvider);
    await container.read(kycAgreementsProvider.future);
    await container.read(sellerTermsProvider.future);
    expect(calls, 2, reason: '過期時 invalidate 一個 provider，兩種取用一起重讀');
  });
}
