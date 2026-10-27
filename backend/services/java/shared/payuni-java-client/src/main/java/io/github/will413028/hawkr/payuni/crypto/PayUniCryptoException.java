package io.github.will413028.hawkr.payuni.crypto;

/**
 * PayUni 加解密例外
 */
public class PayUniCryptoException extends RuntimeException {

    public PayUniCryptoException(String message) {
        super(message);
    }

    public PayUniCryptoException(String message, Throwable cause) {
        super(message, cause);
    }
}
