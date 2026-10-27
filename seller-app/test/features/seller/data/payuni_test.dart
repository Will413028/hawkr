import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/payuni.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/kyc_fixtures.dart';
import '../../../support/my_store_fixtures.dart';

SellerPayUni _p(
  PayUniState state, {
  List<KYCField> fields = const [],
  List<StoreField> storeFields = const [],
}) =>
    SellerPayUni(state: state, fixFields: fields, fixStoreFields: storeFields);

void main() {
  group('payuniHasState', () {
    test('沒有 payuni 或 APPROVED 沒有面板，其他狀態（含未知）都有', () {
      expect(payuniHasState(null), isFalse);
      expect(payuniHasState(_p(PayUniState.PAY_UNI_STATE_APPROVED)), isFalse);
      for (final state in PayUniState.values) {
        if (state == PayUniState.PAY_UNI_STATE_APPROVED) continue;
        expect(payuniHasState(_p(state)), isTrue, reason: '$state');
      }
    });
  });

  group('payuniStateTitle', () {
    test('每個有效狀態都有目錄名詞，UNSPECIFIED 是「未知」', () {
      const l = UiLocale.zhTW;
      for (final state in PayUniState.values) {
        final title = payuniStateTitle(state, l);
        if (state == PayUniState.PAY_UNI_STATE_UNSPECIFIED) {
          expect(title, commonUnknown(l));
        } else {
          expect(title, isNot(commonUnknown(l)), reason: '$state');
          expect(title, isNotEmpty);
        }
      }
      expect(
        payuniStateTitle(PayUniState.PAY_UNI_STATE_REVIEWING, l),
        'PayUni 審核中',
      );
    });

    test('只有審核中不是警示', () {
      for (final state in PayUniState.values) {
        expect(
          payuniStateAlert(state),
          state != PayUniState.PAY_UNI_STATE_REVIEWING,
          reason: '$state',
        );
      }
    });
  });

  group('payuniFixFieldLabels', () {
    test('銀行名稱與代碼是兩個控制項都列；店家欄位另列在後；沒有時是空的', () {
      final labels = payuniFixFieldLabels(
        _p(
          PayUniState.PAY_UNI_STATE_FIXABLE,
          fields: [KYCField.KYC_FIELD_BANK_NAME, KYCField.KYC_FIELD_BANK_CODE],
          storeFields: [StoreField.STORE_FIELD_CONTACT_MOBILE],
        ),
        UiLocale.zhTW,
      );
      expect(labels.length, 3);
      expect(labels.last, '聯絡人手機');
      expect(payuniFixFieldLabels(null, UiLocale.zhTW), isEmpty);
      expect(
        payuniFixFieldLabels(
          _p(PayUniState.PAY_UNI_STATE_REVIEWING),
          UiLocale.zhTW,
        ),
        isEmpty,
      );
    });
  });

  group('payuniFixPointsAtStore', () {
    test('只看 fixStoreFields 有沒有認得的值', () {
      expect(payuniFixPointsAtStore(null), isFalse);
      expect(
        payuniFixPointsAtStore(_p(PayUniState.PAY_UNI_STATE_FIXABLE)),
        isFalse,
      );
      expect(
        payuniFixPointsAtStore(
          _p(
            PayUniState.PAY_UNI_STATE_FIX_VIA_SUPPORT,
            storeFields: [StoreField.STORE_FIELD_ADDRESS],
          ),
        ),
        isTrue,
      );
      expect(
        payuniFixPointsAtStore(
          _p(
            PayUniState.PAY_UNI_STATE_FIXABLE,
            storeFields: [StoreField.STORE_FIELD_UNSPECIFIED],
          ),
        ),
        isFalse,
      );
    });
  });

  group('walletCanWithdraw', () {
    final certified = myStore(publicListing: publicListingMissing());
    final uncertified = myStore(
      publicListing: publicListingMissing([
        SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
      ]),
    );
    MyKyc kycWith(PayUniState? state) => MyKyc.fromApplication(
      individualApplication(
        status: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
        payuni: state == null ? null : SellerPayUni(state: state),
      ),
    );

    test('店家已認證而且 flow.payuni 是 APPROVED 才顯示；其他任何狀態都不', () {
      final approved = kycWith(PayUniState.PAY_UNI_STATE_APPROVED);
      expect(walletCanWithdraw(certified, approved), isTrue);
      expect(walletCanWithdraw(uncertified, approved), isFalse);
      expect(walletCanWithdraw(null, approved), isFalse);
      expect(walletCanWithdraw(certified, null), isFalse);
      expect(walletCanWithdraw(certified, kycWith(null)), isFalse);
      for (final state in PayUniState.values) {
        if (state == PayUniState.PAY_UNI_STATE_APPROVED) continue;
        expect(
          walletCanWithdraw(certified, kycWith(state)),
          isFalse,
          reason: '$state',
        );
      }
    });
  });
}
