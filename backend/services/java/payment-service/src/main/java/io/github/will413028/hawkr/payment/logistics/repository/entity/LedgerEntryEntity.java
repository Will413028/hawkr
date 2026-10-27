package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code ledger_entries}（V9）—— 雙分錄帳本的分錄列。
 *
 * <p>🔴 <b>本表的主鍵是 {@code bigserial}，不是 TSID</b> ⇒ {@link IdType#AUTO}
 * （內部表，不對外暴露；10 檔 §7 的分類）。這一格是逐表實查 DDL 決定的，不是照抄兄弟表。
 *
 * <p>🔴 {@code amount} <b>恆為正</b>（{@code chk_ledgentry_amount}），方向由 {@code direction}
 * 表達。與 {@code logistics_adjustments.amount}（允許負數）<b>刻意不同</b>。
 *
 * <p>🔴 借貸平衡由 {@code trg_ledger_balanced} 守，它是 {@code DEFERRABLE INITIALLY DEFERRED}
 * 的 CONSTRAINT TRIGGER ⇒ 不平衡會在 <b>COMMIT 當下</b>被擋，而不是 INSERT 那一句。
 * 症狀會出現在呼叫端的 commit，不在這裡。
 */
@Data
@TableName(value = "ledger_entries", autoResultMap = true)
public class LedgerEntryEntity {

    /** 🔴 bigserial ⇒ DB 產生。 */
    @TableId(type = IdType.AUTO)
    private Long id;

    private Long txnId;

    /** 表頭內序號，≥1（{@code smallint}）。 */
    private Short lineSeq;

    /** CodeMeta {@code LedgerAccountCode}。 */
    private String accountCode;

    /** CodeMeta {@code LedgerOwnerType}：PLATFORM / SELLER / PROVIDER。 */
    private String ownerType;

    /** PLATFORM ⇒ 0；SELLER ⇒ store_id；PROVIDER ⇒ ShippingMethod 碼值。 */
    private Long ownerRefId;

    /** {@code DEBIT} / {@code CREDIT}。 */
    private String direction;

    /** 🔴 元（整數，INV-L2），恆為正。 */
    private Long amount;

    private String memo;
    private LocalDateTime createdAt;
}
