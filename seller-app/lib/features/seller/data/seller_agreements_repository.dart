import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'seller_agreements_repository.g.dart';

/// 送出 KYC（`SubmitKYC`）要確認的兩份文件的現行版本與網址。
final class KycAgreements {
  const KycAgreements({required this.privacyNotice, required this.payuniTerms});

  /// 個資蒐集告知：確認已閱讀。
  final SellerAgreement privacyNotice;

  /// PayUni 服務條款：同意。
  final SellerAgreement payuniTerms;

  /// 要帶回 `SubmitKYC.accepted_agreements` 的同意，剛好這兩份。
  List<AgreementAcceptance> get acceptances => [
    AgreementAcceptance(
      kind: AgreementKind.AGREEMENT_KIND_KYC_PRIVACY_NOTICE,
      version: privacyNotice.version,
    ),
    AgreementAcceptance(
      kind: AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS,
      version: payuniTerms.version,
    ),
  ];
}

/// 賣家入駐文件（賣家契約、KYC 個資告知、PayUni 條款）的一次讀取：`SellerPublicService.GetSellerAgreements`。
/// 取用某一種時缺少、版本或網址是空的，丟 [StateError]。
final class SellerAgreementSet {
  const SellerAgreementSet(this.agreements);

  final List<SellerAgreement> agreements;

  SellerAgreement _of(AgreementKind kind) {
    for (final agreement in agreements) {
      if (agreement.kind == kind &&
          agreement.version.isNotEmpty &&
          agreement.url.isNotEmpty) {
        return agreement;
      }
    }
    throw StateError('GetSellerAgreements: missing ${kind.name}');
  }

  SellerAgreement sellerTerms() =>
      _of(AgreementKind.AGREEMENT_KIND_SELLER_TERMS);

  KycAgreements kycAgreements() => KycAgreements(
    privacyNotice: _of(AgreementKind.AGREEMENT_KIND_KYC_PRIVACY_NOTICE),
    payuniTerms: _of(AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS),
  );
}

class SellerAgreementsRepository {
  SellerAgreementsRepository(this._client);

  final SellerPublicServiceClient _client;

  Future<SellerAgreementSet> load() async {
    final response = await _client.getSellerAgreements(
      GetSellerAgreementsRequest(),
    );
    return SellerAgreementSet(List.unmodifiable(response.agreements));
  }
}

@Riverpod(keepAlive: true)
SellerAgreementsRepository sellerAgreementsRepository(Ref ref) =>
    SellerAgreementsRepository(ref.watch(sellerPublicClientProvider));

/// 入駐文件的現行版本（一次 `GetSellerAgreements`，開店與送出 KYC 共用）；條款過期後 invalidate 這個 provider 重讀。
@riverpod
Future<SellerAgreementSet> sellerAgreements(Ref ref) =>
    ref.watch(sellerAgreementsRepositoryProvider).load();

/// 賣家契約（開店時同意）；取自 [sellerAgreementsProvider]。
@riverpod
Future<SellerAgreement> sellerTerms(Ref ref) async =>
    (await ref.watch(sellerAgreementsProvider.future)).sellerTerms();

/// 送出 KYC 要確認的兩份文件；取自 [sellerAgreementsProvider]。
@riverpod
Future<KycAgreements> kycAgreements(Ref ref) async =>
    (await ref.watch(sellerAgreementsProvider.future)).kycAgreements();
