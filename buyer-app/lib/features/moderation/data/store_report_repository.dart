import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as pb;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'store_report_repository.g.dart';

@riverpod
StoreReportRepository storeReportRepository(Ref ref) =>
    StoreReportRepository(ref.watch(sellerReportAppClientProvider));

/// 店家檢舉表單的長度與張數上限（`report.proto` CreateReportRequest）。
const storeReportDescriptionMaxRunes = 2000;
const storeReportOrderNoMaxRunes = 64;
const storeReportMaxEvidence = 5;

/// 檢舉人可選的類別：政策分類除了 UNSPECIFIED 與 PLATFORM_ABUSE（只給平台判定），順序依列舉值。
final storeReportCategories = List<PolicyCategory>.unmodifiable(
  PolicyCategory.values.where(
    (c) =>
        c != PolicyCategory.POLICY_CATEGORY_UNSPECIFIED &&
        c != PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE,
  ),
);

/// 畫面要顯示的類別與標籤：沒有目錄文字的類別不顯示。
List<(PolicyCategory, String)> storeReportCategoryOptions(UiLocale locale) => [
  for (final c in storeReportCategories)
    if (c.uiText(locale) case final label?) (c, label),
];

/// 檢舉店家（`SellerReportAppService.CreateReport`）。
class StoreReportRepository {
  StoreReportRepository(this._client);

  final SellerReportAppServiceClient _client;

  Future<pb.CreateReportResponse> create({
    required String storeId,
    required PolicyCategory category,
    required String description,
    String productId = '',
    String orderNo = '',
    List<String> evidenceStagedTokens = const [],
  }) {
    assert(storeReportCategories.contains(category));
    return _client.createReport(
      pb.CreateReportRequest(
        storeId: storeId,
        category: category,
        description: description,
        productId: productId,
        orderNo: orderNo,
        evidenceStagedTokens: evidenceStagedTokens,
      ),
    );
  }
}
