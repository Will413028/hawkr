package io.github.will413028.hawkr.shipping.service.uber;

import com.baomidou.mybatisplus.core.toolkit.Wrappers;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.shipping.entity.PackagePlanEntity;
import io.github.will413028.hawkr.shipping.entity.UberQuoteEntity;
import io.github.will413028.hawkr.shipping.mapper.PackagePlanMapper;
import io.github.will413028.hawkr.shipping.mapper.UberQuoteMapper;
import io.github.will413028.hawkr.shipping.provider.uber.UberQuoteStatus;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

/**
 * {@code uber_quotes} ＋ {@code package_plan} 的交易寫入。
 *
 * <h2>🔴 為什麼是獨立的 bean 而不是 {@code UberQuoteService} 的一個方法</h2>
 * 10 檔 §11.1：同 class 內以 {@code this.} 呼叫自家 {@code @Transactional} 方法，
 * Spring proxy <b>不會攔截</b> ⇒ 每一句各自 auto-commit。
 * 本 repo 已經因為這個形狀踩過一次（#1370：四個名字帶「InTx」的方法從來沒有事務），
 * 而症狀是<b>部分寫入</b>——報價列寫進去了、分箱結果沒有，
 * 然後建單時拿到一筆沒有箱子的 quote。
 *
 * <p>⚠️ 命名不帶「InTx」：那個後綴正是 #1370 讓人誤信的東西。
 * 事務的證據是 {@code @Transactional} 加上<b>跨 bean 呼叫</b>，不是名字。
 */
@Component
public class UberQuoteWriter {

    private final UberQuoteMapper quoteMapper;
    private final PackagePlanMapper packagePlanMapper;

    public UberQuoteWriter(UberQuoteMapper quoteMapper, PackagePlanMapper packagePlanMapper) {
        this.quoteMapper = quoteMapper;
        this.packagePlanMapper = packagePlanMapper;
    }

    /**
     * 一次寫入報價與它的分箱結果。
     *
     * <p>🔴 兩者必須同一個事務：{@code package_plan} 的生命週期掛在 quote 上
     * （66 §5.1.8），一筆沒有箱子的 quote 在建單時才會被發現，而那時買家已經付過錢。
     *
     * @param packages {@code seq} 已由分箱器填好；{@code id} 與 {@code quoteId} 在這裡補
     */
    @Transactional
    public void insertQuoteWithPackages(UberQuoteEntity quote, List<PackagePlanEntity> packages) {
        quoteMapper.insert(quote);
        for (PackagePlanEntity pkg : packages) {
            pkg.setId(TsidUtil.generate());
            pkg.setQuoteId(quote.getQuoteId());
            packagePlanMapper.insert(pkg);
        }
    }

    /**
     * 舊報價換手：同一個 {@code payload_hash} 軸上的 {@code CREATED} 全部標 {@code SUPERSEDED}。
     *
     * <h2>🔴 述詞必須帶 {@code status = CREATED}</h2>
     * 它同時是 allowed-from：{@code CHECKOUT_ACCEPTED} 的列<b>不得</b>被換手——
     * 那是「買家已經確認過的價」，把它改掉等於讓一筆已被接受的報價消失，
     * 而 INV-U1 要的正是「訂單的 Uber 運費恆等於被 {@code CHECKOUT_ACCEPTED} 的那筆」。
     *
     * @return 實際換手的列數（0 是正常值：第一次報價時本來就沒有舊的）
     */
    @Transactional
    public int supersedeCreated(Long buyerId, Long sellerStoreId) {
        UberQuoteEntity patch = new UberQuoteEntity();
        patch.setStatus(UberQuoteStatus.SUPERSEDED);
        return quoteMapper.update(patch, Wrappers.<UberQuoteEntity>lambdaUpdate()
                .eq(UberQuoteEntity::getBuyerId, buyerId)
                .eq(UberQuoteEntity::getSellerStoreId, sellerStoreId)
                .eq(UberQuoteEntity::getStatus, UberQuoteStatus.CREATED));
    }

    /**
     * {@code CREATED → CHECKOUT_ACCEPTED}（結帳確認，66 §6.1.4）。
     *
     * <h2>🔴 CAS：述詞帶 {@code status = CREATED}</h2>
     * 回 {@code 0} 代表這筆已經不是 {@code CREATED} 了（被換手、已過期、或已經被接受過）
     * ⇒ 呼叫端<b>不得 capture</b>。把它寫成無條件 UPDATE 會讓一筆已 {@code SUPERSEDED}
     * 的報價復活，而那筆的金額正是購物車改動前的舊價。
     *
     * @return 1 ＝ 成功；0 ＝ 前置狀態不成立
     */
    @Transactional
    public int markCheckoutAccepted(String quoteId, String orderNo) {
        UberQuoteEntity patch = new UberQuoteEntity();
        patch.setStatus(UberQuoteStatus.CHECKOUT_ACCEPTED);
        patch.setOrderNo(orderNo);
        return quoteMapper.update(patch, Wrappers.<UberQuoteEntity>lambdaUpdate()
                .eq(UberQuoteEntity::getQuoteId, quoteId)
                .eq(UberQuoteEntity::getStatus, UberQuoteStatus.CREATED));
    }
}
