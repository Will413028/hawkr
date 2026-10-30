# social/internal/reads

帶可見性規則的讀取與 keyset 分頁。

## 不變量

- keyset 游標是單一列比較，沒有布林開關。守門：`TestKeysetQueriesCompareOneRowAndHaveNoBooleanSwitch`、`TestKeysetSQLCQueriesHaveNoBooleanCursorSwitch`。規格：§4。
