package io.github.will413028.hawkr.shipping.provider.action;

/**
 * 單一領域動作在「當下這一筆配送單」上的可用性。
 *
 * <h2>🔴 為什麼這裡沒有 {@code href} 與 {@code method}</h2>
 * 對外契約的 {@code allowedActions[]} 確實要帶 href ＋ method（缺一則約束只擋住一半：
 * 前端「不推導可做什麼、卻要推導打哪裡」，而後者同樣需要 provider 知識）。
 * 但那兩格是**路由知識**不是業務判斷：
 * <ul>
 *   <li>本服務（shipping）算的是「這顆按鈕現在能不能按、不能按的原因是什麼」——業務判斷</li>
 *   <li>Gateway 把 action 映射成 href ＋ method——它是唯一對外入口，路由表在它身上</li>
 * </ul>
 * 讓 shipping-service 直接吐 HTTP 路徑，等於把對外路由散到微服務裡，
 * 而微服務本來就不暴露 HTTP。⇒ 分層維持：<b>業務層算可用性，Gateway 補 href。</b>
 *
 * @param actorAccountTypes  哪些身分可以執行這個動作：{@code AccountType} 的 value 集合
 *                           （{@code "1"} 買家／{@code "2"} 賣家／{@code "4"} 管理員），已排序。
 *                           🔴 <b>與 href 不同，actor eligibility 是業務判斷不是路由知識</b>——
 *                           「誰有資格做這件事」由領域決定，Gateway 只做兩件事：
 *                           viewer 在不在這個集合內（純執行）、以及據 viewer 選 href（純路由）。
 *                           🔴 <b>是集合不是單值</b>：同一動作買賣家都能做、只是端點不同
 *                           （{@code track}／{@code view_only}，未來的 {@code cancel} 亦然）是常態。
 *                           先前是單值，於是 gateway 用它自己的路由表 key 當資格判定，
 *                           形成第二份「哪個 action 屬於誰」的業務矩陣。
 *                           沿用既有的 {@code AccountType} 值域而不另立 enum：
 *                           gateway 的 {@code x-account-type} 用的就是這一組值，
 *                           再開一組高度重疊的值域就是第二個真相。
 * @param action             {@code FulfillmentAction} 的 value（snake_case，例 {@code "cancel"}）
 * @param enabled            這顆按鈕現在能不能按
 * @param disabledReasonCode 不可用時的業務碼（{@code GrpcResponseCode} 的 code）。
 *                           🔴 {@code enabled=false} 時**必填**——沒有它，前端只能讓按鈕憑空消失，
 *                           而「不可用要說明原因、不得憑空消失」是前端規格的硬要求
 */
public record ActionAvailability(String action, boolean enabled, Integer disabledReasonCode,
                                 java.util.List<String> actorAccountTypes) {

    public ActionAvailability {
        if (action == null || action.isBlank()) {
            throw new IllegalArgumentException("action 不得為空");
        }
        if (actorAccountTypes == null || actorAccountTypes.isEmpty()) {
            // 🔴 與 disabledReasonCode 同一個理由：允許不填 actor，
            // 等於允許「買家收到賣家才能執行的動作」——那是 QA 實測到的失效
            // （買家端 /app/orders/{no}/logistics 回三條掛 seller capability 的 href，
            //   買家跟隨即 403，而賣家在那支端點上根本拿不到自己的動作）。
            throw new IllegalArgumentException(
                    "action=" + action + " 沒有 actor——無法判斷誰可以執行，"
                            + "對外契約會把賣家動作發給買家");
        }
        if (!enabled && disabledReasonCode == null) {
            // 🔴 這不是防禦性程式碼，是契約：允許 enabled=false 而不給原因，
            // 就等於允許「按鈕靜默消失」——那正是本模型要消滅的失效模式。
            throw new IllegalArgumentException(
                    "action=" + action + " 標為不可用卻沒有 disabledReasonCode——"
                            + "前端無法向使用者說明原因，按鈕會憑空消失");
        }
    }

    public static ActionAvailability enabled(String action, java.util.List<String> actorAccountTypes) {
        return new ActionAvailability(action, true, null, actorAccountTypes);
    }

    public static ActionAvailability disabled(String action, int reasonCode,
                                              java.util.List<String> actorAccountTypes) {
        return new ActionAvailability(action, false, reasonCode, actorAccountTypes);
    }
}
