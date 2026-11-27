package io.github.will413028.hawkr.common.utils.validation;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * InputValidator 顯示長度（PayUni 計長）相關方法測試。
 * 對齊 bug #1187：超商收件人姓名「Ca」（2 英文字）過短被 PayUni 拒（SHIP02114）。
 */
class InputValidatorDisplayLengthTest {


    @Test
    void displayLength_ascii_countsOne() {
        assertEquals(2, InputValidator.displayLength("Ca"));
        assertEquals(4, InputValidator.displayLength("John"));
        assertEquals(0, InputValidator.displayLength(""));
        assertEquals(0, InputValidator.displayLength(null));
    }

    @Test
    void displayLength_cjk_countsTwo() {
        assertEquals(2, InputValidator.displayLength("王"));
        assertEquals(4, InputValidator.displayLength("王偉"));
        assertEquals(6, InputValidator.displayLength("週乾均"));
        assertEquals(10, InputValidator.displayLength("陳一二三四"));
    }

    @Test
    void displayLength_mixed() {
        assertEquals(3, InputValidator.displayLength("陳A"));
        assertEquals(5, InputValidator.displayLength("陳A2B"));
    }


    @Test
    void cvsRecipientName_tooShort_rejected() {
        IllegalArgumentException ex = assertThrows(IllegalArgumentException.class, () ->
                InputValidator.requireDisplayLengthBetween("Ca",
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MIN,
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MAX, "收件人姓名"));
        assertTrue(ex.getMessage().contains("長度過短"));
    }

    @Test
    void cvsRecipientName_singleChineseName_rejected() {
        assertThrows(IllegalArgumentException.class, () ->
                InputValidator.requireDisplayLengthBetween("王",
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MIN,
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MAX, "收件人姓名"));
    }

    @Test
    void cvsRecipientName_validBoundaries_pass() {
        assertEquals("王偉", InputValidator.requireDisplayLengthBetween("王偉", 4, 10, "收件人姓名"));
        assertEquals("John", InputValidator.requireDisplayLengthBetween("John", 4, 10, "收件人姓名"));
        assertEquals("週乾均", InputValidator.requireDisplayLengthBetween("週乾均", 4, 10, "收件人姓名"));
        assertEquals("陳一二三四", InputValidator.requireDisplayLengthBetween("陳一二三四", 4, 10, "收件人姓名"));
    }

    @Test
    void cvsRecipientName_tooLong_rejected() {
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


    @Test
    void truncate_underLimit_unchanged() {
        assertEquals("有機高麗菜", InputValidator.truncateByDisplayLength("有機高麗菜", 20));
        assertEquals("", InputValidator.truncateByDisplayLength("", 20));
        assertNull(InputValidator.truncateByDisplayLength(null, 20));
    }

    @Test
    void truncate_overLimit_cutsAtDisplayLength() {
        String name = "一二三四五六七八九十十一";
        String result = InputValidator.truncateByDisplayLength(name, 20);
        assertEquals(20, InputValidator.displayLength(result));
        assertEquals("一二三四五六七八九十", result);
    }

    @Test
    void truncate_doesNotCutHalfFullWidthChar() {
        assertEquals("A中文", InputValidator.truncateByDisplayLength("A中文字", 5));
        assertEquals("A中", InputValidator.truncateByDisplayLength("A中文字", 4));
    }
}
