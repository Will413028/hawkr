package io.github.will413028.hawkr.shipping.provider.uber;

/**
 * 收件地址無法結構化（66 §6.5 的 {@code ADDRESS_UNPARSEABLE}）。
 *
 * <h2>🔴 為什麼是例外而不是回 {@code Optional.empty()}</h2>
 * 這條路徑的兩個呼叫端要做的事<b>不一樣</b>：報價端要把 Uber 選項標成不可用並附上可行動的理由，
 * 建單端則根本不該走到這裡（報價已經成功過一次）。回 {@code Optional} 會讓兩邊都能
 * 「忘記處理空值」而繼續往下跑，而往下跑的結果是<b>帶著空地址去打 Uber</b>——
 * 那會拿到一個指向 Uber 的 400，真正的原因在我方（66 §6.5 逐字要求在報價前置攔下）。
 *
 * <p>⚠️ 這是<b>類 A（可行動）</b>的失敗：買家改地址就能解決 ⇒ 訊息必須說得出是哪一格不對。
 */
public class UberAddressUnparseableException extends RuntimeException {

    public UberAddressUnparseableException(String message) {
        super(message);
    }
}
