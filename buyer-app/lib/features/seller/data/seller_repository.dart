import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_api/errors.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';

part 'seller_repository.g.dart';

@riverpod
SellerRepository sellerRepository(Ref ref) => SellerRepository(
  ref.watch(sellerPublicClientProvider),
  ref.watch(sellerPublicOptionalAuthClientProvider),
  defaults: ref.watch(defaultErrorTextProvider),
);

class SellerRepository {
  /// BatchGetStores 一次最多的店家數。
  static const batchGetStoresLimit = 100;

  final SellerPublicServiceClient _public;
  final SellerPublicServiceClient _publicWithSession;
  final DefaultErrorText _defaults;
  SellerRepository(
    this._public,
    this._publicWithSession, {
    required DefaultErrorText defaults,
  }) : _defaults = defaults;

  /// 店面頁的店家與統計。店家不存在或不公開（not_found）丟 [NotFoundException]；
  /// 沒收到回應丟 [NetworkException]；其他失敗丟 [ServerException]，文字取自伺服器訊息或該 code 的預設文字。
  Future<seller.GetStoreResponse> getStore(String storeCode) async {
    try {
      return await _public.getStore(
        seller.GetStoreRequest(storeCode: storeCode),
      );
    } catch (raw) {
      if (raw is AppException) rethrow;
      final error = ConnectException.from(raw);
      final message = errorText(error, defaults: _defaults);
      if (error.code == Code.notFound) throw NotFoundException(message);
      if (ErrorFacts.fromException(error).noResponse) {
        throw const NetworkException();
      }
      throw ServerException(message);
    }
  }

  /// 依店家 id 批次讀卡片，回傳以 id 為鍵的表；沒有卡片的店家（從沒公開過、不存在）不在表裡。
  Future<Map<String, StoreCard>> batchGetStores(Iterable<String> ids) async {
    final unique = <String>{
      for (final id in ids)
        if (id.trim().isNotEmpty) id.trim(),
    }.toList();
    final chunks = <List<String>>[
      for (var i = 0; i < unique.length; i += batchGetStoresLimit)
        unique.sublist(
          i,
          i + batchGetStoresLimit > unique.length
              ? unique.length
              : i + batchGetStoresLimit,
        ),
    ];
    final responses = await Future.wait([
      for (final chunk in chunks)
        _public.batchGetStores(seller.BatchGetStoresRequest(ids: chunk)),
    ]);
    return {
      for (final response in responses)
        for (final card in response.stores) card.id: card,
    };
  }

  /// 商品頁的瀏覽 beacon。登入與否由伺服器判斷，失敗不影響頁面。
  Future<void> recordProductView(String productId) async {
    if (productId.trim().isEmpty) return;
    try {
      await _publicWithSession.recordProductView(
        seller.RecordProductViewRequest(productId: productId),
      );
    } catch (_) {}
  }
}
