# seller/internal/stint

stint（一個帳號的一段店家身分）的狀態推導。

## 不變量

- 停權不存，推導點只有 `stint.Derive`；措施「生效中」的各處 SQL 定義與 `StateOf` 在邊界一致。守門：`TestEffectiveMeasuresAndStateAgreeAtTheBoundaries`。規格：§5、§17。
