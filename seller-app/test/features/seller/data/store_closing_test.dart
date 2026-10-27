import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/features/seller/data/store_closing.dart';
import 'package:flutter_test/flutter_test.dart';
import '../../../support/default_error_text.dart';

void main() {
  late int closes;
  late int reloads;
  late int adopted;
  Object? failure;

  StoreClosing build() => StoreClosing(
    client: SellerAppServiceClient(
      FakeTransportBuilder().unary(SellerAppService.closeMyStore, (_, _) {
        closes++;
        final error = failure;
        if (error != null) throw error;
        return CloseMyStoreResponse(
          store: Store(status: StoreStatus.STORE_STATUS_CLOSED),
        );
      }).build(),
    ),
    adoptClosed: () => adopted++,
    reload: () async {
      reloads++;
      return true;
    },
    defaults: zhDefaultErrorText,
  );

  setUp(() {
    closes = 0;
    reloads = 0;
    adopted = 0;
    failure = null;
  });

  test('成功：重讀店家狀態', () async {
    expect(await build().close(), isA<StoreClosed>());
    expect(closes, 1);
    expect(adopted, 1);
    expect(reloads, 1);
  });

  test('失敗：不重讀，沒有伺服器訊息時用預設文字', () async {
    failure = ConnectException(Code.failedPrecondition, 'open orders');

    final outcome = await build().close();

    expect(outcome, isA<StoreClosingFailed>());
    expect(adopted, 0);
    expect(reloads, 0);
  });
}
