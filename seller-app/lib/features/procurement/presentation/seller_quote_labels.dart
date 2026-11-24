import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.pbenum.dart';

/// 回價表單上的選項文案。
///
/// 值域是契約的 [SellerResponseType]、[TaxInclusionBasis] 與 [SellerResponseStatus]；本檔只提供這張表單要顯示的
/// 字串，三個 enum 都是單端（只有賣家 App 顯示）的文案。UNSPECIFIED 與不認得的值回空字串，呼叫端只對可選的值
/// （`kSelectableResponseTypes`、`kSelectableTaxBases`）取文字。
extension SellerQuoteResponseTypeLabel on SellerResponseType {
  String get formLabel {
    if (this == SellerResponseType.SELLER_RESPONSE_TYPE_MANUAL_QUOTE) {
      return '提出我的條件與報價';
    }
    if (this ==
        SellerResponseType
            .SELLER_RESPONSE_TYPE_ACCEPT_NON_PRICE_TERMS_WITH_QUOTE) {
      return '接受非價格條款並報價';
    }
    return '';
  }
}

extension SellerQuoteTaxInclusionBasisLabel on TaxInclusionBasis {
  String get formLabel {
    if (this == TaxInclusionBasis.TAX_INCLUSION_BASIS_TAX_INCLUDED) {
      return '報價含稅';
    }
    if (this == TaxInclusionBasis.TAX_INCLUSION_BASIS_TAX_EXCLUDED) {
      return '報價未稅';
    }
    return '';
  }
}

extension SellerResponseStatusLabel on SellerResponseStatus {
  String get label {
    if (this == SellerResponseStatus.SELLER_RESPONSE_STATUS_ACTIVE) return '有效';
    if (this == SellerResponseStatus.SELLER_RESPONSE_STATUS_SUPERSEDED) {
      return '已被改版取代';
    }
    if (this == SellerResponseStatus.SELLER_RESPONSE_STATUS_WITHDRAWN) {
      return '已撤回';
    }
    if (this == SellerResponseStatus.SELLER_RESPONSE_STATUS_EXPIRED) {
      return '已逾有效期';
    }
    return '';
  }
}
