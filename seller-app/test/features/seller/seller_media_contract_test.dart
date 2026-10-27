import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/features/seller/data/store_editor.dart';
import 'package:hawkr_seller/features/seller/data/store_form.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/my_store_fixtures.dart';
import '../../support/default_error_text.dart';

/// logo / 封面的換圖契約：上傳取得 staged token 之後，以 `UpdateMyStore` 帶 etag，update_mask 只列那個 token 欄位。
void main() {
  late List<UpdateMyStoreRequest> requests;
  late int reloads;

  StoreEditor editor({String? etag = 'v1'}) {
    requests = [];
    reloads = 0;
    final client = SellerAppServiceClient(
      FakeTransportBuilder().unary(SellerAppService.updateMyStore, (req, _) {
        requests.add(req);
        return UpdateMyStoreResponse(store: Store(etag: 'v2'));
      }).build(),
    );
    return StoreEditor(
      client: client,
      current: () => etag == null ? null : myStore(etag: etag),
      reload: () async {
        reloads++;
        return true;
      },
      adopt: (_) {},
      defaults: zhDefaultErrorText,
    );
  }

  test('換 logo：logo_staged_token 與 etag，mask 只有 logo_staged_token', () async {
    final outcome = await editor().save(
      const StoreChanges(logoStagedToken: 'logo-token'),
    );

    expect(outcome, isA<StoreSaved>());
    expect(requests.single.store.logoStagedToken, 'logo-token');
    expect(requests.single.store.coverStagedToken, isEmpty);
    expect(requests.single.store.etag, 'v1');
    expect(requests.single.updateMask.paths, ['logo_staged_token']);
    expect(reloads, 1);
  });

  test('換封面：cover_staged_token 與 etag，mask 只有 cover_staged_token', () async {
    final outcome = await editor().save(
      const StoreChanges(coverStagedToken: 'cover-token'),
    );

    expect(outcome, isA<StoreSaved>());
    expect(requests.single.store.coverStagedToken, 'cover-token');
    expect(requests.single.store.logoStagedToken, isEmpty);
    expect(requests.single.updateMask.paths, ['cover_staged_token']);
  });

  test('沒有讀到店家（沒有 etag）不送出', () async {
    final outcome = await editor(
      etag: null,
    ).save(const StoreChanges(logoStagedToken: 'logo-token'));

    expect(outcome, isA<StoreSaveFailed>());
    expect(requests, isEmpty);
  });
}
