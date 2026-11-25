import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../../seller/data/account_scope.dart';

part 'earnings.g.dart';

/// 錢包入帳紀錄一頁的筆數（契約預設 20、上限 100）。
const earningsPageSize = 20;

/// 入帳列狀態的色調（版面，留在 client）；文字取自共用文案目錄（`payment.EarningsEntryStatus`）。
/// `UNSPECIFIED` 與不認得的值沒有色調（null）。
HkStatusTone? earningsStatusTone(EarningsEntryStatus status) =>
    switch (status) {
      EarningsEntryStatus.EARNINGS_ENTRY_STATUS_PENDING => HkStatusTone.info,
      EarningsEntryStatus.EARNINGS_ENTRY_STATUS_COMPLETED =>
        HkStatusTone.success,
      EarningsEntryStatus.EARNINGS_ENTRY_STATUS_FAILED => HkStatusTone.danger,
      _ => null,
    };

/// 一張訂單的入帳列與淨額（`GetMyOrderEarnings`）。訂單不是自己店家的、或沒有付款，伺服器回 not_found：
/// 這裡回 null（訂單詳情不顯示入帳區），不是錯誤；其他失敗丟出該錯誤。
Future<GetMyOrderEarningsResponse?> fetchMyOrderEarnings(
  SellerAppServiceClient client,
  String orderNo,
) async {
  try {
    return await client.getMyOrderEarnings(
      GetMyOrderEarningsRequest(orderNo: orderNo),
    );
  } catch (error) {
    if (ConnectException.from(error).code == Code.notFound) return null;
    rethrow;
  }
}

/// 訂單詳情的入帳區：[orderNo] 這張訂單的入帳列；null 是沒有入帳資料（見 [fetchMyOrderEarnings]）。
@Riverpod(retry: noRetry)
Future<GetMyOrderEarningsResponse?> myOrderEarnings(Ref ref, String orderNo) =>
    fetchMyOrderEarnings(ref.watch(sellerClientProvider), orderNo);

/// 錢包的入帳紀錄：已讀到的列與下一頁的 token（空字串是沒有下一頁）。
class EarningsLedger {
  const EarningsLedger({
    required this.entries,
    required this.nextPageToken,
    this.loadingMore = false,
    this.loadMoreError,
  });

  final List<EarningsEntry> entries;
  final String nextPageToken;
  final bool loadingMore;

  /// 繼續載入失敗的錯誤；已讀到的列保留，不當成整個列表讀取失敗。
  final Object? loadMoreError;

  bool get hasMore => nextPageToken.isNotEmpty;
}

/// [accountId] 這個帳號的入帳紀錄（`ListMyEarnings`，新到舊）：先讀第一頁，[loadMore] 讀下一頁並接在後面。
/// 以帳號為鍵，換帳號不會看到上一個帳號的紀錄；店家由伺服器依登入身分決定，請求不帶店家。
@Riverpod(retry: noRetry)
class MyEarnings extends _$MyEarnings {
  @override
  Future<EarningsLedger> build(String accountId) async {
    final page = await _page('');
    return EarningsLedger(
      entries: page.entries,
      nextPageToken: page.nextPageToken,
    );
  }

  Future<ListMyEarningsResponse> _page(String pageToken) => ref
      .read(sellerClientProvider)
      .listMyEarnings(
        ListMyEarningsRequest(pageSize: earningsPageSize, pageToken: pageToken),
      );

  /// 讀下一頁；沒有下一頁或正在讀時不做事。失敗記在 [EarningsLedger.loadMoreError]，已讀到的列不變。
  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || !current.hasMore || current.loadingMore) return;
    state = AsyncData(
      EarningsLedger(
        entries: current.entries,
        nextPageToken: current.nextPageToken,
        loadingMore: true,
      ),
    );
    try {
      final page = await _page(current.nextPageToken);
      if (!ref.mounted) return;
      state = AsyncData(
        EarningsLedger(
          entries: [...current.entries, ...page.entries],
          nextPageToken: page.nextPageToken,
        ),
      );
    } catch (error) {
      if (!ref.mounted) return;
      state = AsyncData(
        EarningsLedger(
          entries: current.entries,
          nextPageToken: current.nextPageToken,
          loadMoreError: error,
        ),
      );
    }
  }
}
