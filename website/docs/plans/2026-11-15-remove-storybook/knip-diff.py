"""Compare `pnpm knip --reporter json` against knip-baseline.json.

Findings are keyed by (category, file, name); line/column are ignored.
  python3 knip-diff.py --normalize < raw.json > knip-baseline.json
  pnpm -s knip --reporter json | python3 knip-diff.py   # exit 1 on new findings
"""
import json, pathlib, sys

def keys(raw):
    out = set()
    for entry in raw["issues"]:
        for cat, val in entry.items():
            if cat == "file":
                continue
            items = val.values() if isinstance(val, dict) else val if isinstance(val, list) else []
            for it in items:
                if isinstance(it, list):
                    name = "|".join(sorted(x["name"] for x in it))
                else:
                    name = it["name"] if isinstance(it, dict) else str(it)
                out.add((cat, entry["file"], name))
    return out

raw = json.load(sys.stdin)
if "--normalize" in sys.argv:
    json.dump(sorted(keys(raw)), sys.stdout, indent=0, ensure_ascii=False)
    sys.stdout.write("\n")
    sys.exit(0)
base = {tuple(k) for k in json.loads((pathlib.Path(__file__).parent / "knip-baseline.json").read_text())}
cur = keys(raw)
added, removed = sorted(cur - base), sorted(base - cur)
for k in added:
    print("+", *k)
print(f"new {len(added)}, resolved {len(removed)}")
sys.exit(1 if added else 0)
