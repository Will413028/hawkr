package io.github.will413028.hawkr.common.utils.proto;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class ProtoConversionUtilsTest {

    // ==================== parseDecimalToInt ====================

    @Test
    void parseDecimalToInt_integerString() {
        assertEquals(18, ProtoConversionUtils.parseDecimalToInt("18", "lengthCm"));
        assertEquals(0, ProtoConversionUtils.parseDecimalToInt("0", "lengthCm"));
    }

    @Test
    void parseDecimalToInt_decimalStringTruncatesTowardsZero() {
        assertEquals(18, ProtoConversionUtils.parseDecimalToInt("18.00", "lengthCm"));
        assertEquals(18, ProtoConversionUtils.parseDecimalToInt("18.5", "lengthCm"));
        assertEquals(18, ProtoConversionUtils.parseDecimalToInt("18.9", "lengthCm"));
        assertEquals(0, ProtoConversionUtils.parseDecimalToInt("0.60", "weight"));
    }

    @Test
    void parseDecimalToInt_scientificNotation() {
        assertEquals(500, ProtoConversionUtils.parseDecimalToInt("5e2", "v"));
        assertEquals(1800, ProtoConversionUtils.parseDecimalToInt("1.8E3", "v"));
    }

    @Test
    void parseDecimalToInt_negativeTruncatesTowardsZero() {
        // BigInteger.toBigInteger() 向 0 truncate：-18.9 → -18
        assertEquals(-18, ProtoConversionUtils.parseDecimalToInt("-18.9", "v"));
        assertEquals(-18, ProtoConversionUtils.parseDecimalToInt("-18.00", "v"));
    }

    @Test
    void parseDecimalToInt_nullOrBlankThrows() {
        IllegalArgumentException e1 = assertThrows(IllegalArgumentException.class,
                () -> ProtoConversionUtils.parseDecimalToInt(null, "lengthCm"));
        assertTrue(e1.getMessage().contains("lengthCm 不可為空"));

        assertThrows(IllegalArgumentException.class,
                () -> ProtoConversionUtils.parseDecimalToInt("", "lengthCm"));
        assertThrows(IllegalArgumentException.class,
                () -> ProtoConversionUtils.parseDecimalToInt("   ", "lengthCm"));
    }

    @Test
    void parseDecimalToInt_nonNumericThrows() {
        IllegalArgumentException e = assertThrows(IllegalArgumentException.class,
                () -> ProtoConversionUtils.parseDecimalToInt("abc", "lengthCm"));
        assertTrue(e.getMessage().contains("lengthCm 格式不合法: abc"));
    }

    @Test
    void parseDecimalToInt_overflowThrows() {
        IllegalArgumentException e = assertThrows(IllegalArgumentException.class,
                () -> ProtoConversionUtils.parseDecimalToInt("9999999999", "lengthCm"));
        assertTrue(e.getMessage().contains("lengthCm 超出 int 範圍"));
    }

    @Test
    void parseDecimalToInt_maxInt() {
        assertEquals(Integer.MAX_VALUE,
                ProtoConversionUtils.parseDecimalToInt(String.valueOf(Integer.MAX_VALUE), "v"));
        assertEquals(Integer.MIN_VALUE,
                ProtoConversionUtils.parseDecimalToInt(String.valueOf(Integer.MIN_VALUE), "v"));
    }

    // ==================== parseDecimalToLong ====================

    @Test
    void parseDecimalToLong_integerString() {
        assertEquals(600L, ProtoConversionUtils.parseDecimalToLong("600", "weightGrams"));
    }

    @Test
    void parseDecimalToLong_decimalStringTruncates() {
        assertEquals(600L, ProtoConversionUtils.parseDecimalToLong("600.00", "weightGrams"));
        assertEquals(600L, ProtoConversionUtils.parseDecimalToLong("600.9", "weightGrams"));
    }

    @Test
    void parseDecimalToLong_blankThrows() {
        assertThrows(IllegalArgumentException.class,
                () -> ProtoConversionUtils.parseDecimalToLong("", "weightGrams"));
    }

    @Test
    void parseDecimalToLong_overflowThrows() {
        IllegalArgumentException e = assertThrows(IllegalArgumentException.class,
                () -> ProtoConversionUtils.parseDecimalToLong("99999999999999999999", "weightGrams"));
        assertTrue(e.getMessage().contains("超出 long 範圍"));
    }

    @Test
    void parseDecimalToLong_maxLong() {
        assertEquals(Long.MAX_VALUE,
                ProtoConversionUtils.parseDecimalToLong(String.valueOf(Long.MAX_VALUE), "v"));
    }
}
