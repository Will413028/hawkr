"""數 openapi.yaml 裡的社群 operation：brief 口徑、social 實際服務、A7-4 可刪三個數字。

在 backend/ 執行：python3 -I docs/plans/2026-10-18-a7-social/count-rest-ops.py
"""

import collections

import yaml

BRIEF_TAGS = {"Post", "Comment", "Forum", "Moderation", "AdminModeration", "AdminPost", "PostHashtag", "Hashtag", "CommentRealtime"}
PRODUCT_TAGS = {"Hashtag"}  # /public/hashtags、/public/hashtags/popular：handler 呼叫 product-service
KEEP_UNTIL_A6 = set()  # S3（2026-10-18）之後 /app/blocks* 也在 A7-4e 刪除；舊口徑是這兩條路徑（3 個 operation）

spec = yaml.safe_load(open("api/openapi/openapi.yaml", encoding="utf-8"))
rows = []
for path, item in spec["paths"].items():
    for method, op in item.items():
        if method not in ("get", "post", "put", "patch", "delete"):
            continue
        tags = set(op.get("tags", []))
        if tags & BRIEF_TAGS:
            rows.append((sorted(tags)[0], method.upper(), path))

brief = len(rows)
social = [r for r in rows if r[0] not in PRODUCT_TAGS]
deletable = [r for r in social if r[2] not in KEEP_UNTIL_A6]
print(f"brief 口徑 {brief}；social 實際服務 {len(social)}；A7-4 可刪 {len(deletable)}")
print(collections.Counter(r[0] for r in rows))
for r in sorted(rows):
    mark = "product" if r[0] in PRODUCT_TAGS else ("keep-A6" if r[2] in KEEP_UNTIL_A6 else "delete")
    print("|".join(r), mark, sep="|")
