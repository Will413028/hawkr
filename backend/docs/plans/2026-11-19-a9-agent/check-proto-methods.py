#!/usr/bin/env python3
"""A9-1b 驗收：四個新 proto 的 rpc 清單與預期逐支對上（缺的與多的都失敗）。

預期清單依計畫 A9-D2、A9-D12 從 interface-inventory.md §1 收斂：括號裡是盤點的 #。
用法：python3 check-proto-methods.py [api 目錄，預設 repo 根目錄的 api]
"""
import re
import sys
from pathlib import Path

# (檔案相對 api/, service) -> [(rpc, 盤點 #)]
EXPECTED = {
    ("hawkr/imageai/v1alpha1/imageai.proto", "ImageAiAppService"): [
        ("TransformImage", "1"),
        ("ListAiStyles", "2"),
    ],
    ("hawkr/admin/imageai/v1alpha1/imageai.proto", "ImageAiAdminService"): [
        ("ListAiStylesForAdmin", "3"),
        ("CreateAiStyle", "4"),
        ("UpdateAiStyle", "5"),
        ("DeleteAiStyle", "6"),
        ("GetAiQuota", "7"),
        ("UpdateAiQuota", "8"),
        ("DeleteAiQuota", "9"),
    ],
    # A9-D12：取代 #18、#19、#20、#22（四條助理串流）與 #23 以外的串流形狀；#23 是 GetLatestAssistantSession。
    ("hawkr/assistant/v1alpha1/assistant.proto", "AssistantAppService"): [
        ("SendAssistantMessage", "18,19,20,22"),
        ("WatchAssistantTurn", "18,19,20,22"),
        ("GetLatestAssistantSession", "23"),
    ],
    ("hawkr/procurement/v1alpha1/procurement.proto", "BuyerProcurementAppService"): [
        ("SubmitTurn", "24"),
        ("GetSnapshot", "26"),
        ("StartCandidateSearch", "27"),
        ("GetCandidateSearch", "28"),
        ("ToggleSelection", "29"),
        ("SubmitAnswer", "30"),
        ("Rewind", "31"),
        ("PreviewRfqPrice", "32"),
        ("ConfirmRfqSnapshot", "33"),
        ("PreviewRfqPublication", "34"),
        ("ExecuteRfqPublication", "35"),
        ("ListConversations", "37"),
        ("GetConversationMessages", "38"),
        ("GetRuntimeSnapshot", "39"),
        ("ListSellerResponses", "51"),
        ("PutShortlistEntry", "52"),
        ("DeleteShortlistEntry", "53"),
        ("AcceptSellerQuote", "54"),
        ("RenameConversation", "55"),
        ("DeleteConversation", "56"),
        ("WatchRun", "57"),
        ("CancelRun", "58"),
        ("ResumeRun", "59"),
    ],
    ("hawkr/procurement/v1alpha1/procurement.proto", "SellerProcurementAppService"): [
        ("ListSellerPublications", "40"),
        ("GetSellerPublication", "41"),
        ("UpsertSellerResponseDraft", "42"),
        ("SubmitSellerResponse", "48"),
        ("WithdrawSellerResponse", "49"),
        ("DeclineRfqPublication", "50"),
    ],
}

SERVICE_RE = re.compile(r"^\s*service\s+(\w+)\s*\{")
RPC_RE = re.compile(r"^\s*rpc\s+(\w+)\s*\(")


def actual(api: Path) -> dict:
    """{(檔, service): [rpc 名稱]}；檔案缺少時 service 對應空清單。"""
    out = {}
    files = sorted({f for f, _ in EXPECTED})
    for rel in files:
        path = api / rel
        if not path.is_file():
            print(f"缺檔：{rel}")
            out[(rel, None)] = []
            continue
        current = None
        for line in path.read_text(encoding="utf-8").splitlines():
            if line.lstrip().startswith("//"):
                continue
            m = SERVICE_RE.match(line)
            if m:
                current = m.group(1)
                out.setdefault((rel, current), [])
                continue
            m = RPC_RE.match(line)
            if m and current:
                out[(rel, current)].append(m.group(1))
    return out


def main() -> int:
    api = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).resolve().parents[4] / "api"
    if not api.is_dir():
        print(f"找不到 api 目錄：{api}")
        return 1
    found = actual(api)
    failures = 0
    for key in sorted(set(EXPECTED) | set(found), key=str):
        want = [name for name, _ in EXPECTED.get(key, [])]
        have = found.get(key, [])
        missing = [n for n in want if n not in have]
        extra = [n for n in have if n not in want]
        dup = sorted({n for n in have if have.count(n) > 1})
        if key not in EXPECTED:
            print(f"多出的 service：{key[0]} {key[1]}")
            failures += 1
        for n in missing:
            num = dict(EXPECTED[key])[n]
            print(f"缺少：{key[1]}.{n}（盤點 #{num}）")
            failures += 1
        for n in extra:
            print(f"多出：{key[0]} {key[1]}.{n}")
            failures += 1
        for n in dup:
            print(f"重複：{key[1]}.{n}")
            failures += 1
    total = sum(len(v) for v in EXPECTED.values())
    if failures:
        print(f"失敗：{failures} 項不符（預期 {total} 支 rpc）")
        return 1
    print(f"通過：{total} 支 rpc 與預期逐支相符")
    return 0


if __name__ == "__main__":
    sys.exit(main())
