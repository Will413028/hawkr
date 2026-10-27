import '../../../core/codemeta/code_meta.dart';

/// 回價表單上的選項文案。
///
/// 值域本身是 CodeMeta 登記的 [ProcurementSellerResponseType] 與
/// [ProcurementTaxInclusionBasis]（backend #2538），本檔**不重新定義值域**，也不
/// 出現任何 wire 字面值——只提供這張表單要顯示的字串。
///
/// 為什麼不直接用生成物的 `label`：CodeMeta 的 label 是跨端共用的短名
/// （「自行報價」「含稅」「未稅」），而本表單沿用 #200 落地時的說明式文案
/// （「提出我的條件與報價」「報價含稅」「報價未稅」）。文案是 UI，不是契約；
/// 換掉它是 UX 改動，要另外提案，不該當作值域同步的副作用。
///
/// switch 刻意寫成窮盡（不留 default）：backend 之後若往這兩組值域加值，這裡會
/// **編譯失敗**，逼人決定新選項在表單上叫什麼，而不是靜默落到某個 fallback 字串。
extension SellerQuoteResponseTypeLabel on ProcurementSellerResponseType {
  String get formLabel => switch (this) {
    ProcurementSellerResponseType.manualQuote => '提出我的條件與報價',
    ProcurementSellerResponseType.acceptNonPriceTermsWithQuote => '接受非價格條款並報價',
  };
}

extension SellerQuoteTaxInclusionBasisLabel on ProcurementTaxInclusionBasis {
  String get formLabel => switch (this) {
    ProcurementTaxInclusionBasis.taxIncluded => '報價含稅',
    ProcurementTaxInclusionBasis.taxExcluded => '報價未稅',
  };
}
