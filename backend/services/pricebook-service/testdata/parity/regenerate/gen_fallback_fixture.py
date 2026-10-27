"""產生涵蓋 seasonal_naive fallback 分支的 parity fixture。

demo 產出的 model.json 全部是 direct_ridge（training-report 的
seasonal_naive_model_count = 0），但 model_family 叫
`per-series-direct-ridge-with-seasonal-naive-fallback`——Go port 仍得實作那條
分支，卻沒有任何 golden 覆蓋得到它。

決定 kind 的是 training.py:744：
    len(targets) >= config.min_samples_per_model and proper_training_count >= 2
demo 的 sample_count 是 118(D+1) 遞減到 112(D+7)，門檻設 115 就會讓
D+1~D+4 走 ridge、D+5~D+7 落 seasonal_naive，一份 fixture 覆蓋兩條分支。

用自訂 TrainingConfig 直接呼叫 train_model()，完全繞過 demo pipeline
（demo 指令不吃 config，且會覆寫 demo-output）。輸出寫到 scratchpad，
不碰 資料團隊的 repo。
"""

import json
import os
import sys
from datetime import date, datetime, timedelta
from pathlib import Path
from zoneinfo import ZoneInfo

# 來源：資料團隊的 Hawkr_Transformer_Pricebook（本機 clone，不在本 repo 內）。
# 用環境變數覆寫路徑；預設值是 2026-07-29 產生這批 fixture 時的位置。
#
# ⚠️ RevisionStore 會對 sqlite 開 WAL，建議先把 demo-output 複製一份再指過去，
#    不要直接對來源 repo 跑（產生這批 fixture 時就是跑在副本上，
#    事後 `git status --porcelain` 與 `diff -rq` 雙重確認來源 repo 零改動）。
PRICEBOOK_REPO = Path(
    os.environ.get(
        "PRICEBOOK_REPO",
        str(Path.home() / "Hawkr_Transformer_Pricebook"),
    )
)
ROOT = Path(os.environ.get("DEMO_OUTPUT", str(PRICEBOOK_REPO / "demo-output")))
# 預設寫回本 fixture 目錄（本檔的上一層）
OUT = Path(os.environ.get("FIXTURE_OUT", str(Path(__file__).resolve().parent.parent)))

MIN_SAMPLES_THRESHOLD = 115  # 介於 112(D+7) 與 118(D+1) 之間，刻意製造混合

import hawkr_price_ai.forecasting as F  # noqa: E402
from hawkr_price_ai.baseline import RidgeParameters  # noqa: E402
from hawkr_price_ai.features import PricePoint  # noqa: E402
from hawkr_price_ai.market_categories import MARKET_TYPE_TO_CATEGORY_CODE  # noqa: E402
from hawkr_price_ai.storage.revisions import RevisionStore  # noqa: E402
from hawkr_price_ai.training import TrainingConfig, train_model  # noqa: E402

assert "torch" not in sys.modules, "訓練/推論路徑不該載入 torch"

tz = ZoneInfo("Asia/Taipei")
origin = datetime(2026, 7, 18, 23, 45, tzinfo=tz)

store = RevisionStore(ROOT / "data/state/price_ai.sqlite3")
rows = store.list_price_points(as_of=origin, latest_only=True)
points = [PricePoint.from_mapping(r) for r in rows if str(r["crop_code"]).casefold() != "rest"]
print(f"訓練點數: {len(points)}")

OUT.mkdir(parents=True, exist_ok=True)
history_file = OUT / "history.json"
expected_points = [p.canonical_dict() for p in points]
if history_file.exists():
    existing = json.loads(history_file.read_text())
    if existing["points"] != expected_points:
        raise SystemExit(
            "history.json 與本次重算的歷史點不一致——demo 與 fallback fixture "
            "理論上該共用同一份歷史（同 store、同 origin），出現差異代表兩支 "
            "腳本的資料來源已經分岔，先查清楚原因再繼續。"
        )
    print("history.json 已存在且與本次重算一致，不覆寫")
else:
    history_file.write_text(
        json.dumps(
            {
                "schema": "hawkr.parity-history/v1",
                "note": "見 gen_parity_fixture.py 的說明——兩支腳本共用同一份歷史。",
                "origin": origin.isoformat(),
                "points": expected_points,
            },
            indent=2,
            ensure_ascii=False,
        )
        + "\n"
    )
    print(f"history 點數: {len(points)} -> {history_file}")

# --- 用調高門檻的 config 重訓（參數其餘與 demo 一致，見 model.json 的 config 段）---
config = TrainingConfig(
    horizon_days=7,
    ridge_alpha=10.0,
    min_samples_per_model=MIN_SAMPLES_THRESHOLD,
    calibration_fraction=0.2,
    min_calibration_samples=7,
    seasonal_lag_days=7,
    availability_policy="latest_corrected_backfill",
)
artifact, report = train_model(points, config, trained_at=origin, calendar_timezone="Asia/Taipei")

kind_counts: dict[str, int] = {}
for s in artifact.series.values():
    for hm in s.horizons.values():
        kind_counts[hm.kind] = kind_counts.get(hm.kind, 0) + 1
print(f"重訓後 kind 分佈: {kind_counts}")

# --- 休市日曆（同 gen_parity_fixture.py）------------------------------------
market_closed: dict[str, set] = {}
category_closed: dict[tuple, set] = {}
for item in store.list_market_rest_days(as_of=origin):
    cat = MARKET_TYPE_TO_CATEGORY_CODE.get(str(item["market_type"]).upper())
    if cat is not None:
        category_closed.setdefault((str(item["market_id"]), cat), set()).add(item["rest_date"])
for item in rows:
    if item["is_market_closed"]:
        cat = item["category_code"]
        if cat is None:
            market_closed.setdefault(str(item["market_id"]), set()).add(item["trade_date"])
        else:
            category_closed.setdefault((str(item["market_id"]), str(cat)), set()).add(
                item["trade_date"]
            )

target_years = {origin.year, (origin.date() + timedelta(days=7)).year}
verified_calendar_keys = set()
for year in target_years:
    for market_id, market_type in store.list_market_rest_coverage(year, as_of=origin):
        cat = MARKET_TYPE_TO_CATEGORY_CODE.get(market_type.upper())
        if cat is not None:
            verified_calendar_keys.add((market_id, cat, year))

# --- 掛鉤 ------------------------------------------------------------------
features_by_key: dict[tuple, dict] = {}
centers_by_key: dict[tuple, float] = {}
_orig_build = F.build_feature_vector_from_daily_points
_orig_predict = RidgeParameters.predict


def _ctx(depth: int = 2) -> dict:
    loc = sys._getframe(depth).f_locals
    return {"series_id": loc.get("series_id"), "horizon_days": loc.get("horizon")}


def patched_build(*args, **kwargs):
    fv = _orig_build(*args, **kwargs)
    c = _ctx()
    features_by_key[(c["series_id"], c["horizon_days"])] = {
        "values": [float(v) for v in fv.values],
        "max_feature_available_at": (
            fv.max_feature_available_at.isoformat() if fv.max_feature_available_at else None
        ),
    }
    return fv


def patched_predict(self, features):
    c = _ctx()
    center = _orig_predict(self, features)
    centers_by_key[(c["series_id"], c["horizon_days"])] = float(center)
    return center


F.build_feature_vector_from_daily_points = patched_build
RidgeParameters.predict = patched_predict

calendar_file = OUT / "calendar.json"
expected_calendar = {
    "closed_by_market": {
        market_id: sorted(str(d) for d in dates)
        for market_id, dates in sorted(market_closed.items())
    },
    "closed_by_market_category": [
        {
            "market_id": market_id,
            "category_code": category_code,
            "dates": sorted(str(d) for d in dates),
        }
        for (market_id, category_code), dates in sorted(category_closed.items())
    ],
    "verified_calendar": [
        {"market_id": market_id, "category_code": category_code, "year": year}
        for market_id, category_code, year in sorted(verified_calendar_keys)
    ],
}
if calendar_file.exists():
    existing = json.loads(calendar_file.read_text())
    diffs = [k for k, v in expected_calendar.items() if existing.get(k) != v]
    if diffs:
        raise SystemExit(
            "calendar.json 與本次重算的休市輸入不一致，欄位："
            + ", ".join(diffs)
            + "。demo 與 fallback fixture 用同一個 store、同一個 origin，"
            "日曆理應完全相同——出現差異代表兩支腳本的資料來源已經分岔，"
            "先查清楚原因再繼續。"
        )
    print("calendar.json 已存在且與本次重算一致，不覆寫")
else:
    raise SystemExit(
        "calendar.json 不存在。請先跑 gen_parity_fixture.py（它是這個檔案的產生者），"
        "本腳本只做一致性驗證。"
    )

forecasts = F.forecast_seven_days(
    artifact,
    points,
    origin,
    closed_dates_by_market=market_closed,
    closed_dates_by_market_category=category_closed,
    verified_calendar_market_categories=verified_calendar_keys,
)

F.build_feature_vector_from_daily_points = _orig_build
RidgeParameters.predict = _orig_predict

# --- 組 fixture ------------------------------------------------------------
cases = []
for fc in forecasts:
    d = fc.to_dict()
    key = (d["series_id"], d["horizon_days"])
    feat = features_by_key.get(key)
    hm = artifact.series[d["series_id"]].horizons[d["horizon_days"]]
    cases.append(
        {
            "series_id": d["series_id"],
            "market_id": d["market_id"],
            "crop_id": d["crop_id"],
            "horizon_days": d["horizon_days"],
            "target_date": d["target_date"],
            "model_kind": d["model_kind"],
            "trained_kind": hm.kind,
            "sample_count": hm.sample_count,
            "fallback_price": hm.fallback_price,
            "seasonal_lag_days": hm.seasonal_lag_days,
            "prediction_status": d["prediction_status"],
            "missing_reason": d["missing_reason"],
            "calendar_status": d["calendar_status"],
            "market_status": d["market_status"],
            "features": feat["values"] if feat else None,
            "max_feature_available_at": d["max_feature_available_at"],
            "ridge_center": centers_by_key.get(key),
            "price_p10": d["price_p10"],
            "price_p50": d["price_p50"],
            "price_p90": d["price_p90"],
        }
    )

fixture = {
    "schema": "hawkr.parity-fixture/v1",
    "variant": "fallback-coverage",
    "note": (
        f"刻意把 min_samples_per_model 調到 {MIN_SAMPLES_THRESHOLD}（demo 原值 30）重訓，"
        "讓 sample_count 較少的長 horizon（D+5~D+7，112~114 筆）落入 seasonal_naive，"
        "補上 demo artifact 完全沒有覆蓋到的 fallback 分支。"
        "已實測：這份的 8 個 direct_ridge case 與 parity-fixture.json 的對應 case "
        "在 features / ridge_center / p10 / p50 / p90 上完全一致——該門檻只決定走哪條分支，"
        "不影響 ridge 本身的訓練結果（calibration 切分由 calibration_fraction 與 "
        "min_calibration_samples 決定，兩者未動）。所以兩份 fixture 的 ridge 部分互為驗證。"
    ),
    "source": {
        "model_version": artifact.model_version,
        "dataset_digest": artifact.dataset_digest,
        "trained_at": str(artifact.trained_at),
        "min_samples_per_model": MIN_SAMPLES_THRESHOLD,
        "ridge_alpha": 10.0,
        "availability_policy": "latest_corrected_backfill",
        "calendar_timezone": "Asia/Taipei",
        "forecast_origin_local": origin.isoformat(),
        "input_point_count": len(points),
    },
    "feature_schema": list(artifact.feature_schema),
    "cases": cases,
}

out_file = OUT / "parity-fixture-fallback.json"
out_file.write_text(json.dumps(fixture, indent=2, ensure_ascii=False) + "\n")

# 也存下重訓的 model.json，Go 端載入時需要對應的係數
model_file = OUT / "model-fallback.json"
model_file.write_text(json.dumps(artifact.to_dict(), indent=2, ensure_ascii=False) + "\n")

print(f"\ncases            : {len(cases)}")
kinds: dict[str, int] = {}
for c in cases:
    kinds[c["model_kind"]] = kinds.get(c["model_kind"], 0) + 1
print(f"model_kind 分佈  : {kinds}")
statuses: dict[str, int] = {}
for c in cases:
    statuses[c["prediction_status"]] = statuses.get(c["prediction_status"], 0) + 1
print(f"prediction_status: {statuses}")
print(f"有 ridge_center  : {sum(1 for c in cases if c['ridge_center'] is not None)}")
print("\n逐 case（series / horizon / trained_kind / model_kind / p50）:")
for c in cases:
    print(
        f"  {c['series_id']:<14} h{c['horizon_days']} "
        f"{c['trained_kind']:<15} {c['model_kind']:<15} "
        f"p50={c['price_p50']}"
    )
print(f"\n寫出: {out_file.name} ({out_file.stat().st_size} B) / {model_file.name} ({model_file.stat().st_size} B)")
