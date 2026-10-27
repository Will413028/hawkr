import 'package:hawkr_api/errors.dart' show DefaultErrorText, userMessage;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/session/my_store.dart';

part 'my_store_repository.g.dart';

/// 賣家自己的店家：`SellerAppService.GetMyStore`（Connect）。
///
/// not_found 是伺服器的答案（帳號還沒有店家），回 [MyStore.absent]；其他失敗拋 [MyStoreUnavailable]，
/// 呼叫端不能把它當成「沒有店家」。
class MyStoreRepository {
  MyStoreRepository(this._client, {required DefaultErrorText defaults})
    : _defaults = defaults;

  final SellerAppServiceClient _client;
  final DefaultErrorText _defaults;

  Future<MyStore> fetch() async {
    try {
      return MyStore(await _client.getMyStore(GetMyStoreRequest()));
    } catch (raw) {
      final error = ConnectException.from(raw);
      if (error.code == Code.notFound) return const MyStore.absent();
      throw MyStoreUnavailable(error, userMessage(error, defaults: _defaults));
    }
  }
}

/// 讀不到店家（暫時失敗或伺服器拒絕），不代表帳號沒有店家。
class MyStoreUnavailable implements Exception {
  const MyStoreUnavailable(this.cause, this.message);

  final ConnectException cause;

  /// 給使用者看的文字；呼叫被取消時是 null。
  final String? message;

  @override
  String toString() =>
      'MyStoreUnavailable(${cause.code.name}): ${cause.message}';
}

@Riverpod(keepAlive: true)
MyStoreRepository myStoreRepository(Ref ref) => MyStoreRepository(
  ref.watch(sellerClientProvider),
  defaults: ref.watch(defaultErrorTextProvider),
);
