import 'dart:async';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pb.dart'
    as mod;
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;

buyer.BlockedStore blockedStore(
  String id, {
  String? name,
  StoreCardStatus status = StoreCardStatus.STORE_CARD_STATUS_OPEN,
}) => buyer.BlockedStore(
  storeId: id,
  store: name == null
      ? null
      : StoreCard(id: id, name: name, status: status, storeCode: 'code-$id'),
);

buyer.BlockedBuyer blockedBuyer(
  String id, {
  String name = '',
  bool deleted = false,
}) => buyer.BlockedBuyer(buyerId: id, displayName: name, deleted: deleted);

/// `BuyerAppService` 封鎖相關的 fake。名單以頁給：第 n 頁的 token 是 `page-n`，最後一頁沒有 next token。
class FakeBuyerApp {
  FakeBuyerApp({
    this.storePages = const [[]],
    this.buyerPages = const [[]],
    this.blockError,
    this.unblockError,
    this.listStoresError,
    this.listBuyersError,
  });

  List<List<buyer.BlockedStore>> storePages;
  List<List<buyer.BlockedBuyer>> buyerPages;
  ConnectException? blockError;
  ConnectException? unblockError;
  ConnectException? listStoresError;
  ConnectException? listBuyersError;

  final List<buyer.BlockStoreRequest> storeBlocks = [];
  final List<buyer.BlockAuthorRequest> authorBlocks = [];
  final List<buyer.UnblockStoreRequest> storeUnblocks = [];
  final List<buyer.UnblockBuyerRequest> buyerUnblocks = [];
  final List<buyer.ListBlockedStoresRequest> storeListRequests = [];
  final List<buyer.ListBlockedBuyersRequest> buyerListRequests = [];

  int _pageIndex(String token) =>
      token.isEmpty ? 0 : int.parse(token.substring('page-'.length));

  String _next(int index, int length) =>
      index + 1 < length ? 'page-${index + 1}' : '';

  BuyerAppServiceClient client() => BuyerAppServiceClient(
    FakeTransportBuilder()
        .unary(BuyerAppService.blockStore, (req, _) async {
          storeBlocks.add(req);
          if (blockError != null) throw blockError!;
          return buyer.BlockStoreResponse(changed: true);
        })
        .unary(BuyerAppService.blockAuthor, (req, _) async {
          authorBlocks.add(req);
          if (blockError != null) throw blockError!;
          return buyer.BlockAuthorResponse(changed: true);
        })
        .unary(BuyerAppService.unblockStore, (req, _) async {
          storeUnblocks.add(req);
          if (unblockError != null) throw unblockError!;
          return buyer.UnblockStoreResponse(changed: true);
        })
        .unary(BuyerAppService.unblockBuyer, (req, _) async {
          buyerUnblocks.add(req);
          if (unblockError != null) throw unblockError!;
          return buyer.UnblockBuyerResponse(changed: true);
        })
        .unary(BuyerAppService.listBlockedStores, (req, _) async {
          storeListRequests.add(req);
          if (listStoresError != null) throw listStoresError!;
          final i = _pageIndex(req.pageToken);
          return buyer.ListBlockedStoresResponse(
            blockedStores: storePages[i],
            nextPageToken: _next(i, storePages.length),
          );
        })
        .unary(BuyerAppService.listBlockedBuyers, (req, _) async {
          buyerListRequests.add(req);
          if (listBuyersError != null) throw listBuyersError!;
          final i = _pageIndex(req.pageToken);
          return buyer.ListBlockedBuyersResponse(
            blockedBuyers: buyerPages[i],
            nextPageToken: _next(i, buyerPages.length),
          );
        })
        .build(),
  );
}

mod.ReportRecord reportRecord(
  String id, {
  ReportTargetKind kind = ReportTargetKind.REPORT_TARGET_KIND_POST,
  PolicyCategory category = PolicyCategory.POLICY_CATEGORY_SPAM,
  ReportOutcome outcome = ReportOutcome.REPORT_OUTCOME_PENDING,
  String description = '',
}) => mod.ReportRecord(
  id: id,
  target: mod.ReportTarget(kind: kind, id: 'target-$id'),
  category: category,
  description: description,
  outcome: outcome,
);

/// `ModerationAppService` 的 fake。[failFirst] 次 `SubmitReport` 先失敗（模擬網路中斷後重試）。
class FakeModeration {
  FakeModeration({
    this.reportPages = const [[]],
    this.failFirst = 0,
    this.listError,
  });

  List<List<mod.ReportRecord>> reportPages;
  int failFirst;
  ConnectException? listError;

  /// 設了就讓 `ListMyReports` 卡住，直到測試完成它。
  Completer<void>? listGate;

  final List<mod.SubmitReportRequest> submitted = [];
  final List<mod.ListMyReportsRequest> listRequests = [];

  ModerationAppServiceClient client() => ModerationAppServiceClient(
    FakeTransportBuilder()
        .unary(ModerationAppService.submitReport, (req, _) async {
          submitted.add(req);
          if (failFirst > 0) {
            failFirst--;
            throw ConnectException(Code.unavailable, 'offline');
          }
          return mod.SubmitReportResponse(
            reportId: 'report-${submitted.length}',
          );
        })
        .unary(ModerationAppService.listMyReports, (req, _) async {
          listRequests.add(req);
          await listGate?.future;
          if (listError != null) throw listError!;
          final i = req.pageToken.isEmpty
              ? 0
              : int.parse(req.pageToken.substring('page-'.length));
          return mod.ListMyReportsResponse(
            reports: reportPages[i],
            nextPageToken: i + 1 < reportPages.length ? 'page-${i + 1}' : '',
          );
        })
        .build(),
  );
}
