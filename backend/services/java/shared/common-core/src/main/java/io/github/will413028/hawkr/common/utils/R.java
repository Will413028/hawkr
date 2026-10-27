package io.github.will413028.hawkr.common.utils;

import java.util.HashMap;
import java.util.Map;

/**
 * Unified API Response Wrapper
 * Compatible with existing R class but with improved design
 */
public class R extends HashMap<String, Object> {
    private static final long serialVersionUID = 1L;

    public static final int SUCCESS_CODE = 0;
    public static final int ERROR_CODE = 500;
    public static final String SUCCESS_MSG = "success";

    public R() {
        put("code", SUCCESS_CODE);
        put("msg", SUCCESS_MSG);
    }

    public static R error() {
        return error(ERROR_CODE, "未知异常，请联系管理员");
    }

    public static R error(String msg) {
        return error(ERROR_CODE, msg);
    }

    public static R error(int code, String msg) {
        R r = new R();
        r.put("code", code);
        r.put("msg", msg);
        return r;
    }

    public static R ok(String msg) {
        R r = new R();
        r.put("msg", msg);
        return r;
    }

    public static R ok(Map<String, Object> map) {
        R r = new R();
        r.putAll(map);
        return r;
    }

    public static R ok() {
        return new R();
    }

    public static R ok(Object data) {
        R r = new R();
        r.put("data", data);
        return r;
    }

    @Override
    public R put(String key, Object value) {
        super.put(key, value);
        return this;
    }

    public Integer getCode() {
        return (Integer) this.get("code");
    }

    public boolean isSuccess() {
        return SUCCESS_CODE == getCode();
    }
}
