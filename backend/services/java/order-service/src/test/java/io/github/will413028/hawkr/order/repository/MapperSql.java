package io.github.will413028.hawkr.order.repository;

import org.apache.ibatis.annotations.Select;

import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * 讀 mapper 上 {@code @Select} 字面值的共用工具（測試用）。
 *
 * <p>存在的理由是「測試不要自己抄一份 SQL」：抄過來的那份會與正式碼各自演化，最後測的是
 * 測試自己寫的 SQL，正式碼改壞了也照樣綠。排序決定性的三支測試共用本類，解析行為只有一份。
 */
final class MapperSql {

    /** ORDER BY 起至 LIMIT／結尾。SQL 關鍵字在本 repo 一律大寫，忽略大小寫只是防手滑。 */
    private static final Pattern ORDER_BY =
            Pattern.compile("ORDER\\s+BY\\s+(.+?)(?:\\s+LIMIT\\b|$)", Pattern.CASE_INSENSITIVE);

    private MapperSql() {
    }

    /** 取指定 mapper 方法的 {@code @Select} SQL（已壓平）。方法簽章對不上直接讓測試失敗。 */
    static String select(Class<?> mapper, String method, Class<?>... paramTypes) {
        try {
            return flatten(mapper.getMethod(method, paramTypes).getAnnotation(Select.class).value());
        } catch (NoSuchMethodException e) {
            throw new AssertionError("mapper 方法簽章已變更：" + mapper.getSimpleName() + "#" + method, e);
        }
    }

    /** 壓平空白並去掉 {@code <script>} 外殼，讓 ORDER BY 之後只剩排序鍵。 */
    static String flatten(String[] value) {
        return String.join(" ", value)
                .replaceAll("</?script>", " ")
                .replaceAll("\\s+", " ")
                .trim();
    }

    /** 回傳 ORDER BY 之後的排序鍵（壓平空白），沒有 ORDER BY 則回 null。 */
    static String orderByOf(String sql) {
        Matcher matcher = ORDER_BY.matcher(sql);
        return matcher.find() ? matcher.group(1).trim() : null;
    }

    /** 收尾鍵前允許表別名（{@code o.id}），但不得是別的欄名的尾巴（{@code store_id} 不算 {@code id}）。 */
    static Pattern endsWith(String column) {
        return Pattern.compile("(?<![_a-zA-Z])" + Pattern.quote(column) + "(\\s+(ASC|DESC))?$",
                Pattern.CASE_INSENSITIVE);
    }
}
