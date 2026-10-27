# `NotoSansTC-subset.ttf`

收據渲染測試用的字型子集（#3026）。

## 為什麼是子集而不是完整字型

完整的 Noto Sans TC 是 **11.9 MB**，進版控的成本每個 clone 都要付一次。而測試要驗的是
**「渲染器產出的是一份真的 PDF、而且字型子集有被內嵌」**，不是「這套字型涵蓋多少個漢字」
—— 後者由 Dockerfile 的 SHA256 釘選 ＋ DI 的 fail-closed 保證（字型讀不起來 pod 起不來）。

⚠️ 因此**不要**拿這個檔去產真實收據：它只含測試字串用得到的字，其餘字會渲染成空白。

## 產生方式（要重做時照跑）

```bash
python3 -m venv /tmp/fv && /tmp/fv/bin/pip install fonttools brotli
curl -fsSLo /tmp/NotoSansTC.ttf \
  'https://raw.githubusercontent.com/google/fonts/main/ofl/notosanstc/NotoSansTC%5Bwght%5D.ttf'
# sha256 必須是 864727d210d54f2537bbe23b3a839436c3992af72de9322af5270897246bd44f
#（與 services/invoice-service/Dockerfile 的 NOTO_TC_SHA256 同一個值）
/tmp/fv/bin/pyftsubset /tmp/NotoSansTC.ttf \
  --output-file=NotoSansTC-subset.ttf \
  --text='出據人統一編號營業地址買受消費者收單訂日期品項數量價金合計本非發票陳記蔬果行台北市內湖區瑞光路號有機高麗菜溫室小番茄平臺代製' \
  --unicodes='U+0020-007E,U+00A0,U+3001,U+3002,U+FF08,U+FF09,U+FF1A,U+FF0C' \
  --layout-features='' --no-hinting --desubroutinize --drop-tables+=DSIG
```

## 授權

Noto Sans TC — SIL Open Font License 1.1（<https://openfontlicense.org>）。
OFL 允許自由使用、修改（含子集化）與再散布，條件是保留版權與授權聲明：

> Copyright 2022 The Noto Project Authors (https://github.com/notofonts/chinese)
> This Font Software is licensed under the SIL Open Font License, Version 1.1.
