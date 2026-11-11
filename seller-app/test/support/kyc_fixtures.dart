import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';

/// 伺服器回的一份個人草稿申請。
KYCApplication individualApplication({
  String etag = 'v1',
  KYCStatus kycStatus = KYCStatus.KYC_STATUS_DRAFT,
  String realName = '',
  String idMasked = '',
  List<KYCField> missing = const [],
  List<StoreField> missingStore = const [],
  KYCRejection? rejection,
  List<KYCIssue> outstanding = const [],
  String? frontAssetId,
}) {
  final app = KYCApplication(
    etag: etag,
    status: kycStatus,
    realName: realName,
    individual: IndividualKYC(idCardNoMasked: idMasked),
    missingFields: missing,
    missingStoreFields: missingStore,
    lastRejection: rejection,
    outstandingIssues: outstanding,
  );
  if (frontAssetId != null) {
    app.idCardFront = KYCDocument(
      assetId: frontAssetId,
      downloadUri: '/api/v1/app/media/secure_download/$frontAssetId',
    );
  }
  return app;
}

/// 伺服器回的一份企業草稿申請。
KYCApplication businessApplication({
  String etag = 'v1',
  KYCStatus kycStatus = KYCStatus.KYC_STATUS_DRAFT,
  String companyName = '',
  String taxId = '',
  List<KYCField> missing = const [],
  List<StoreField> missingStore = const [],
  KYCRejection? rejection,
  List<KYCIssue> outstanding = const [],
}) => KYCApplication(
  etag: etag,
  status: kycStatus,
  business: BusinessKYC(
    companyName: companyName,
    taxId: taxId,
    companyRegisterDate: Date(year: 2020, month: 1, day: 2),
    companyAddress: PostalAddress(townCode: '63000010', addressDetail: '路一號'),
  ),
  missingFields: missing,
  missingStoreFields: missingStore,
  lastRejection: rejection,
  outstandingIssues: outstanding,
);

MyKyc myKycOf(KYCApplication? app) =>
    app == null ? const MyKyc.notStarted() : MyKyc.fromApplication(app);
