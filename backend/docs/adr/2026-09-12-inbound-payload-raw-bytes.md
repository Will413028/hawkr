# `InboundCapability` 的簽章改為 `InboundPayload`：raw bytes 必須到得了 adapter

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-12 |
| 影響範圍 | shipping-service provider adapter 介面 |

- **脈絡**：實讀 `provider/capability/InboundCapability.java`，四個方法**全部**吃
  `Map<String, String> rawPayload`（`parseStatusEvent`／`isLabelEvent`／`parseLabelEvent`／`dedupKey`）。
  那是 **PayUni form-encoded 的形狀**——一個 provider 的 wire format 洩漏進了 provider-neutral 介面，
  而那正是本介面存在要消滅的東西。兩個後果：
  1. **表達不了 Uber**：巢狀 JSON（`courier_imminent` boolean、`related_deliveries` 陣列、
     `courier`／`manifest` 物件）攤平成 `Map<String,String>` 會產生「攤平規則」這個新的
     provider-specific 知識，而它**無處存放**
  2. 🔴 **raw bytes 在介面層就遺失**：66 §6.3.1 要求**驗簽 over raw body**，
     而 66 陷阱 A2 逐字警告「驗簽前做過 JSON parse／re-serialize ⇒ **偶發**失敗、測不出來、上線才炸」。
     `Map<String,String>` 已經是 parse 後的結果 ⇒ 驗簽只能留在 ingress，
     **而「怎麼驗簽」是 provider 專屬知識**——那正是本介面的職責
  ⚠️ 66 §4.5.0 寫的 `inbound().parse(raw)`／`inbound().toNormalized(...)` **這兩個方法名在
  Foundation 介面裡不存在** ⇒ 66 假設的介面形狀與 actual source 不符，本 ADR 一併收斂。
- **決定：引入 `InboundPayload`，兩種形態並存、不互相轉換。**

```java
/** provider 回調的原始承載。form params 與 raw body 並存——不從其中一個推導另一個。 */
public record InboundPayload(
        Map<String, String> formParams,  // form-encoded provider（PayUni）；JSON provider 為空 Map
        byte[] rawBody,                  // 🔴 原始位元組，驗簽用；form provider 可為 null
        Map<String, String> headers      // signature header（Uber `x-uber-signature`）等
) {}

InboundStatusEvent parseStatusEvent(InboundPayload raw);
boolean            isLabelEvent(InboundPayload raw);
Optional<InboundLabelEvent> parseLabelEvent(InboundPayload raw);
String             dedupKey(InboundPayload raw);
```

- **為什麼不在 Uber adapter 內攤平**：攤平會讓 ① 巢狀結構失真 ② raw bytes 仍然拿不到
  ③ 攤平規則變成第二份 provider 知識。**三者都是本介面的 javadoc 明文要消滅的形狀。**
- **影響範圍（可控）**：production 實作者唯二——`AbstractPayUniAdapter` 的匿名內部類
  與 `UberDirectAdapter:145`（現回 `Optional.empty()`）；呼叫端 `ShippingWebhookConsumer:96,110,111,130`
  ＋ `ShippingService:1645`；測試 3 支（`PayUniInboundWireContractTest`／
  `ShippingWebhookConsumerIngressTest`／`FulfillmentProviderRegistryTest`）。
- **後果**：
  1. PayUni 側改為 `raw.formParams()`，**行為不變**（契約測試是變異驗證的錨）
  2. 驗簽從 ingress 下放到 adapter ⇒ 硬規則「provider 專屬知識不留在 consumer」**多守住一格**
  3. 68 的檔案交換與 69 的商家 webhook 也受益（兩者都不是 form-encoded）
- **已落實於**：66 詳設 §4.5.0 的 `inbound()` 那一列（方法名同批更正）。
