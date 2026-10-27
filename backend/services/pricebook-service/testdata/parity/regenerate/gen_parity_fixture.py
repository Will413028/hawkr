"""產生 Go↔Python parity fixture：36 維特徵向量 + ridge center + 最終 p10/p50/p90。

為什麼需要中間層：forecast.csv 只有最終價格，parity 一旦不過就分不出是
「特徵建構錯了」還是「線性代數錯了」。特徵建構才是 Go port 的真正難點
（bitemporal revision tie-break、rolling window 的 offset 語意、lag 從 1 起算
但 last_price 從 0 起算）。

做法：monkey-patch 兩個掛點，不修改 資料團隊的 repo 任何檔案。
  1. forecasting.build_feature_vector_from_daily_points → 所有分支的 36 維輸入
     （含 seasonal_naive 與 not_predicted，那兩條不會呼叫 predict）
  2. baseline.RidgeParameters.predict → ridge 分支的中心值

context（series_id / horizon / target_date）不在 FeatureVector 上，靠
sys._getframe(1) 從 forecast_seven_days 的 local 變數取。
"""

import json
import os
import sys
from datetime import datetime
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

import hawkr_price_ai.forecasting as F  # noqa: E402
from hawkr_price_ai.baseline import RidgeParameters  # noqa: E402
from hawkr_price_ai.features import PricePoint  # noqa: E402
from hawkr_price_ai.market_categories import MARKET_TYPE_TO_CATEGORY_CODE  # noqa: E402
from hawkr_price_ai.storage.revisions import RevisionStore  # noqa: E402
from hawkr_price_ai.training import load_artifact  # noqa: E402

assert "torch" not in sys.modules, "推論路徑不該載入 torch"

# --- 載入 artifact ---------------------------------------------------------
pointer = json.loads((ROOT / "artifacts/models/latest.json").read_text())
# DEPLOY.md 記載 latest.json 的 artifact_path 在 Windows 訓練機上可能是反斜線
artifact_path = ROOT / "artifacts/models" / str(pointer["artifact_path"]).replace("\\", "/")
artifact = load_artifact(artifact_path)

tz = ZoneInfo(str(artifact.metadata.get("calendar_timezone", "UTC")))
origin = datetime(2026, 7, 18, 23, 45, tzinfo=tz)

store = RevisionStore(ROOT / "data/state/price_ai.sqlite3")
rows = store.list_price_points(as_of=origin, latest_only=True)
points = [PricePoint.from_mapping(r) for r in rows if str(r["crop_code"]).casefold() != "rest"]

OUT.mkdir(parents=True, exist_ok=True)
history_file = OUT / "history.json"
history_file.write_text(
    json.dumps(
        {
            "schema": "hawkr.parity-history/v1",
            "note": (
                "餵進 build_feature_vector_from_daily_points / "
                "seasonal_naive_from_daily_points 的原始歷史觀測點（已由 "
                "RevisionStore 依 as_of+latest_only 解過 revision，一個 "
                "(series_id, trade_date) 對應一筆）。parity-fixture.json 與 "
                "parity-fixture-fallback.json 共用同一份——同一個 store、"
                "同一個 origin，只是訓練門檻不同。"
            ),
            "origin": origin.isoformat(),
            "points": [p.canonical_dict() for p in points],
        },
        indent=2,
        ensure_ascii=False,
    )
    + "\n"
)
print(f"history 點數: {len(points)} -> {history_file} ({history_file.stat().st_size} bytes)")

# --- 掛鉤 ------------------------------------------------------------------
features_by_key: dict[tuple, dict] = {}
centers_by_key: dict[tuple, float] = {}

_orig_build = F.build_feature_vector_from_daily_points
_orig_predict = RidgeParameters.predict


def _caller_context(depth: int = 2) -> dict:
    """從 forecast_seven_days 的 frame 取 series/horizon context。"""
    frame = sys._getframe(depth)
    loc = frame.f_locals
    series = loc.get("series")
    return {
        "caller": frame.f_code.co_name,
        "series_id": loc.get("series_id"),
        "horizon_days": loc.get("horizon"),
        "target_date": str(loc.get("target_date")),
        "market_id": getattr(series, "market_id", None),
        "crop_id": getattr(series, "crop_id", None),
        "category_code": getattr(series, "category_code", None),
    }


def patched_build(*args, **kwargs):
    fv = _orig_build(*args, **kwargs)
    ctx = _caller_context()
    key = (ctx["series_id"], ctx["horizon_days"])
    features_by_key[key] = {
        **ctx,
        "names": list(fv.names),
        "values": [float(v) for v in fv.values],
        "max_feature_available_at": (
            fv.max_feature_available_at.isoformat() if fv.max_feature_available_at else None
        ),
    }
    return fv


def patched_predict(self, features):
    ctx = _caller_context()
    center = _orig_predict(self, features)
    centers_by_key[(ctx["series_id"], ctx["horizon_days"])] = float(center)
    return center


F.build_feature_vector_from_daily_points = patched_build
RidgeParameters.predict = patched_predict

# --- 休市日曆 ---------------------------------------------------------------
# 複製 pipeline._closed_dates / _verified_calendar_keys 的邏輯（pipeline.py:1100-1191）。
# 不傳這三個參數的話，休市日會被當成正常交易日而產出預測，`not_applicable`
# 這條 gate 就完全不會出現在 fixture 裡——第一版就是這樣錯的，靠比對既有
# forecast.csv 才發現。
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

target_years = {origin.year, (origin.date() + __import__("datetime").timedelta(days=7)).year}
verified_calendar_keys = set()
for year in target_years:
    for market_id, market_type in store.list_market_rest_coverage(year, as_of=origin):
        cat = MARKET_TYPE_TO_CATEGORY_CODE.get(market_type.upper())
        if cat is not None:
            verified_calendar_keys.add((market_id, cat, year))

calendar_file = OUT / "calendar.json"
calendar_file.write_text(
    json.dumps(
        {
            "schema": "hawkr.parity-calendar/v1",
            "note": (
                "傳給 forecast_seven_days 的三個休市／日曆輸入，逐字對應 "
                "closed_dates_by_market / closed_dates_by_market_category / "
                "verified_calendar_market_categories 三個參數。複合鍵用 "
                "list-of-object 表示（JSON 的 key 不能是 tuple）。"
                "沒有這份資料，Go 端的編排層 parity test 就只能對 "
                "not_applicable（休市）case 驗形狀，驗不到休市判定本身——"
                "而這個 case 恰好是 (market, category) 兩層鍵的關鍵證據："
                "moa:109:A1 是 N05、moa:109:LA1 是 N04，同市場同一天 "
                "2026-07-21 只有 N04 休市。"
            ),
            "origin": origin.isoformat(),
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
        },
        indent=2,
        ensure_ascii=False,
    )
    + "\n"
)
print(
    f"日曆: by_market={len(market_closed)} "
    f"by_market_category={len(category_closed)} "
    f"verified={len(verified_calendar_keys)} -> {calendar_file}"
)

# --- 跑預測 ----------------------------------------------------------------
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
    cases.append(
        {
            "series_id": d["series_id"],
            "market_id": d["market_id"],
            "crop_id": d["crop_id"],
            "category_code": d["category_code"],
            "horizon_days": d["horizon_days"],
            "target_date": d["target_date"],
            "model_kind": d["model_kind"],
            "prediction_status": d["prediction_status"],
            "missing_reason": d["missing_reason"],
            "calendar_status": d["calendar_status"],
            "market_status": d["market_status"],
            # 中間層：Go port 的第一個比對點
            "features": feat["values"] if feat else None,
            "max_feature_available_at": d["max_feature_available_at"],
            # ridge 原始輸出（seasonal_naive / not_predicted 為 null）
            "ridge_center": centers_by_key.get(key),
            # 最終輸出：第二個比對點
            "price_p10": d["price_p10"],
            "price_p50": d["price_p50"],
            "price_p90": d["price_p90"],
        }
    )

fixture = {
    "schema": "hawkr.parity-fixture/v1",
    "note": (
        "由 Hawkr_Transformer_Pricebook 的 demo artifact 產生，"
        "用於 backend-payuni pricebook-service 的 Go 推論實作 parity test。"
        "features 是餵給 ridge.predict 之前的 36 維向量（比對點 1），"
        "ridge_center 是 predict 的原始輸出（比對點 2），"
        "price_p10/p50/p90 是加上 residual_quantiles、max(0)、單調修正後的最終值（比對點 3）。"
    ),
    "source": {
        "model_version": artifact.model_version,
        "dataset_digest": artifact.dataset_digest,
        "trained_at": str(artifact.trained_at),
        "calendar_timezone": str(artifact.metadata.get("calendar_timezone")),
        "model_family": artifact.metadata.get("model_family"),
        "series_count": artifact.metadata.get("series_count"),
        "training_point_count": artifact.metadata.get("training_point_count"),
        "forecast_origin_local": origin.isoformat(),
        "input_point_count": len(points),
    },
    "feature_schema": list(artifact.feature_schema),
    "cases": cases,
}

out_file = OUT / "parity-fixture.json"
out_file.write_text(json.dumps(fixture, indent=2, ensure_ascii=False) + "\n")

# --- 自我驗證：重算結果必須跟 demo 既有的 forecast.csv 一致 ------------------
import csv  # noqa: E402

pred_dir = ROOT / "data/predictions"
csvs = sorted(pred_dir.rglob("forecast.csv"))
mismatches = []
if csvs:
    with csvs[-1].open() as fh:
        existing = {(r["series_id"], int(r["horizon_days"])): r for r in csv.DictReader(fh)}
    for c in cases:
        k = (c["series_id"], c["horizon_days"])
        old = existing.get(k)
        if not old:
            mismatches.append(f"{k} 不在既有 forecast.csv")
            continue
        for fld in ("price_p10", "price_p50", "price_p90"):
            a, b = old[fld], c[fld]
            a_norm = None if a == "" else float(a)
            if a_norm != b:
                mismatches.append(f"{k} {fld}: csv={a!r} recomputed={b!r}")

print(f"cases            : {len(cases)}")
print(f"有 feature dump  : {sum(1 for c in cases if c['features'])}")
print(f"有 ridge_center  : {sum(1 for c in cases if c['ridge_center'] is not None)}")
kinds: dict[str, int] = {}
for c in cases:
    kinds[c["model_kind"]] = kinds.get(c["model_kind"], 0) + 1
print(f"model_kind 分佈  : {kinds}")
statuses: dict[str, int] = {}
for c in cases:
    statuses[c["prediction_status"]] = statuses.get(c["prediction_status"], 0) + 1
print(f"prediction_status: {statuses}")
print(f"對照既有 csv     : {'一致 ✓' if not mismatches else f'{len(mismatches)} 處不一致'}")
for m in mismatches[:5]:
    print("  -", m)
print(f"寫出             : {out_file} ({out_file.stat().st_size} bytes)")
