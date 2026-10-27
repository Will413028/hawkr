import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';

import 'my_store_fixtures.dart';
import 'default_error_text.dart';

/// 假的 `SellerAppService.GetMyStore`：[response] 是伺服器的回應；[failure] 不是 null 時回該錯誤（not_found 表示沒有店家）；
/// [gate] 不是 null 時等它完成才回答（回答的內容是呼叫當下的設定）。[calls] 記錄被呼叫的次數。
class FakeMyStoreServer {
  FakeMyStoreServer({GetMyStoreResponse? response, this.failure, this.gate})
    : response = response ?? myStoreResponse();

  GetMyStoreResponse response;
  ConnectException? failure;
  Future<void>? gate;
  int calls = 0;

  MyStoreRepository get repository => MyStoreRepository(
    SellerAppServiceClient(
      FakeTransportBuilder().unary(SellerAppService.getMyStore, (_, _) async {
        calls++;
        final answer = response;
        final error = failure;
        await gate;
        if (error != null) throw error;
        return answer;
      }).build(),
    ),
    defaults: zhDefaultErrorText,
  );

  /// 沒有店家：伺服器回 not_found。
  static FakeMyStoreServer absent() =>
      FakeMyStoreServer(failure: ConnectException(Code.notFound, 'no store'));

  /// 暫時失敗：服務不可用。
  static FakeMyStoreServer unavailable() => FakeMyStoreServer(
    failure: ConnectException(Code.unavailable, 'try later'),
  );
}
