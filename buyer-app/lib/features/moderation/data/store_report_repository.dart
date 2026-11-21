import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
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

/// 一則檢舉的補充說明上限（`report.proto`；去重補充與主動補充共用）。
const storeReportMaxSupplements = 10;

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

/// 讀取或補充我的檢舉時，畫面要分流處理的失敗；[cause] 是原始例外，用來產生錯誤文字。其餘錯誤原樣拋出。
enum StoreReportFailureKind { notFound, preconditionFailed }

class StoreReportFailure implements Exception {
  const StoreReportFailure(this.kind, this.cause);

  final StoreReportFailureKind kind;
  final Object cause;
}

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

  Future<pb.GetMyReportResponse> get(String reportId) async {
    try {
      return await _client.getMyReport(
        pb.GetMyReportRequest(reportId: reportId),
      );
    } on ConnectException catch (e) {
      if (e.code == Code.notFound) {
        throw StoreReportFailure(StoreReportFailureKind.notFound, e);
      }
      rethrow;
    }
  }

  Future<pb.AddReportSupplementResponse> addSupplement({
    required String reportId,
    required String description,
    List<String> evidenceStagedTokens = const [],
  }) async {
    try {
      return await _client.addReportSupplement(
        pb.AddReportSupplementRequest(
          reportId: reportId,
          description: description,
          evidenceStagedTokens: evidenceStagedTokens,
        ),
      );
    } on ConnectException catch (e) {
      if (e.code == Code.failedPrecondition) {
        throw StoreReportFailure(StoreReportFailureKind.preconditionFailed, e);
      }
      rethrow;
    }
  }
}
