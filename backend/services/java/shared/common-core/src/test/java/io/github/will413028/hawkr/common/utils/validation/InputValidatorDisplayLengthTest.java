package io.github.will413028.hawkr.common.utils.validation;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * InputValidator 顯示長度（PayUni 計長）相關方法測試。
 * 對齊 bug #1187：超商收件人姓名「Ca」（2 英文字）過短被 PayUni 拒（SHIP02114）。
 */
class InputValidatorDisplayLengthTest {

    // ==================== displayLength（英文/半形 1、中文/全形 2） ====================

    @Test
    void displayLength_ascii_countsOne() {
        assertEquals(2, InputValidator.displayLength("Ca"));   // bug #1187 case
        assertEquals(4, InputValidator.displayLength("John"));
        assertEquals(0, InputValidator.displayLength(""));
        assertEquals(0, InputValidator.displayLength(null));
    }

    @Test
    void displayLength_cjk_countsTwo() {
        assertEquals(2, InputValidator.displayLength("王"));        // 單名 1 中文 = 2
        assertEquals(4, InputValidator.displayLength("王偉"));      // 2 中文 = 4
        assertEquals(6, InputValidator.displayLength("週乾均"));    // 3 中文 = 6（#1187 成功 case）
        assertEquals(10, InputValidator.displayLength("陳一二三四")); // 5 中文 = 10
    }

    @Test
    void displayLength_mixed() {
        assertEquals(3, InputValidator.displayLength("陳A"));      // 1 中文(2) + 1 英文(1)
        assertEquals(5, InputValidator.displayLength("陳A2B"));    // 2 + 1 + 1 + 1
    }

    // ==================== requireDisplayLengthBetween（超商姓名 [4,10]） ====================

    @Test
    void cvsRecipientName_tooShort_rejected() {
        // "Ca" = 2 < 4 → 拒絕（正是 #1187 根因）
        IllegalArgumentException ex = assertThrows(IllegalArgumentException.class, () ->
                InputValidator.requireDisplayLengthBetween("Ca",
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MIN,
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MAX, "收件人姓名"));
        assertTrue(ex.getMessage().contains("長度過短"));
    }

    @Test
    void cvsRecipientName_singleChineseName_rejected() {
        // "王" = 2 < 4 → 拒絕（PayUni 官方要求最短 2 中文字）
        assertThrows(IllegalArgumentException.class, () ->
                InputValidator.requireDisplayLengthBetween("王",
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MIN,
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MAX, "收件人姓名"));
    }

    @Test
    void cvsRecipientName_validBoundaries_pass() {
        // 2 中文 = 4（下限）/ 4 英文 = 4（下限）/ 3 中文 = 6 / 5 中文 = 10（上限）
        assertEquals("王偉", InputValidator.requireDisplayLengthBetween("王偉", 4, 10, "收件人姓名"));
        assertEquals("John", InputValidator.requireDisplayLengthBetween("John", 4, 10, "收件人姓名"));
        assertEquals("週乾均", InputValidator.requireDisplayLengthBetween("週乾均", 4, 10, "收件人姓名"));
        assertEquals("陳一二三四", InputValidator.requireDisplayLengthBetween("陳一二三四", 4, 10, "收件人姓名"));
    }

    @Test
    void cvsRecipientName_tooLong_rejected() {
        // 6 中文 = 12 > 10 → 拒絕
        IllegalArgumentException ex = assertThrows(IllegalArgumentException.class, () ->
                InputValidator.requireDisplayLengthBetween("陳一二三四五", 4, 10, "收件人姓名"));
        assertTrue(ex.getMessage().contains("長度過長"));
    }

    @Test
    void recipientName_empty_rejected() {
        assertThrows(IllegalArgumentException.class, () ->
                InputValidator.requireDisplayLengthBetween("", 4, 10, "收件人姓名"));
        assertThrows(IllegalArgumentException.class, () ->
                InputValidator.requireDisplayLengthBetween(null, 4, 10, "收件人姓名"));
    }

    // ==================== truncateByDisplayLength（商品名稱截斷，不切半字） ====================

    @Test
    void truncate_underLimit_unchanged() {
        assertEquals("有機高麗菜", InputValidator.truncateByDisplayLength("有機高麗菜", 20));
        assertEquals("", InputValidator.truncateByDisplayLength("", 20));
        assertNull(InputValidator.truncateByDisplayLength(null, 20));
    }

    @Test
    void truncate_overLimit_cutsAtDisplayLength() {
        // 11 中文 = 22 > 20，截斷至顯示長度 20 = 前 10 中文字
        String name = "一二三四五六七八九十十一";
        String result = InputValidator.truncateByDisplayLength(name, 20);
        assertEquals(20, InputValidator.displayLength(result));
        assertEquals("一二三四五六七八九十", result);
    }

    @Test
    void truncate_doesNotCutHalfFullWidthChar() {
        // 上限 5：A(1)中(2)文(2)=5 剛好；字(2)會超 → "A中文"
        assertEquals("A中文", InputValidator.truncateByDisplayLength("A中文字", 5));
        // 上限 4：A(1)中(2)=3, 文(2)會超過 4 → "A中"
        assertEquals("A中", InputValidator.truncateByDisplayLength("A中文字", 4));
    }
}
