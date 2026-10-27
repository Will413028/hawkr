"""Regenerates spatial-fixture.json from Hawkr_Transformer_Pricebook's
actual build_spatial_surface — same "don't hand-derive golden values"
principle as testdata/parity/regenerate/gen_parity_fixture.py, applied to
the spatial engine instead of the ridge model.

Usage:
    PRICEBOOK_REPO=~/Hawkr_Transformer_Pricebook \
        python3 gen_spatial_fixture.py
"""
import json
import os
import sys
from pathlib import Path

PRICEBOOK_REPO = Path(os.environ.get(
    "PRICEBOOK_REPO",
    Path.home() / "Hawkr_Transformer_Pricebook",
))
sys.path.insert(0, str(PRICEBOOK_REPO / "src"))

from hawkr_price_ai.spatial.interpolation import (  # noqa: E402
    MarketObservation, SpatialConfig, build_spatial_surface, wgs84_to_twd97_tm2, twd97_tm2_to_wgs84,
)
from hawkr_price_ai.spatial.reference import BoundaryRegion, BoundarySnapshot  # noqa: E402


def corner(x, y):
    return list(twd97_tm2_to_wgs84(x, y, epsg=3826))


def main():
    cx, cy = wgs84_to_twd97_tm2(120.9, 24.15, epsg=3826)
    half = 10_000.0
    ring = [
        corner(cx - half, cy - half), corner(cx + half, cy - half),
        corner(cx + half, cy + half), corner(cx - half, cy + half),
        corner(cx - half, cy - half),
    ]
    region = BoundaryRegion(
        region_id="test_region",
        polygons=((tuple(tuple(p) for p in ring),),),
        interpolation_epsg=3826,
    )
    boundary = BoundarySnapshot(
        version="test-v1", payload_sha256="test", regions=(region,),
        bbox_wgs84=(120.7, 24.0, 121.1, 24.3), source_path=Path("test"),
    )
    obs = [
        MarketObservation(series_id="s1", market_id="moa:m1", market_name="M1", crop_id="moa:A1",
                           crop_name="A1", category_code="N04", average_price_twd_per_kg=20.0,
                           volume_kg=100.0, source_snapshot_id="snap", record_hash="h1", revision_no=1,
                           longitude=120.85, latitude=24.10, surface_region="test_region", coordinate_source="test"),
        MarketObservation(series_id="s2", market_id="moa:m2", market_name="M2", crop_id="moa:A1",
                           crop_name="A1", category_code="N04", average_price_twd_per_kg=40.0,
                           volume_kg=100.0, source_snapshot_id="snap", record_hash="h2", revision_no=1,
                           longitude=120.95, latitude=24.10, surface_region="test_region", coordinate_source="test"),
        MarketObservation(series_id="s3", market_id="moa:m3", market_name="M3", crop_id="moa:A1",
                           crop_name="A1", category_code="N04", average_price_twd_per_kg=30.0,
                           volume_kg=100.0, source_snapshot_id="snap", record_hash="h3", revision_no=1,
                           longitude=120.90, latitude=24.20, surface_region="test_region", coordinate_source="test"),
        MarketObservation(series_id="s4", market_id="moa:m4", market_name="M4", crop_id="moa:A1",
                           crop_name="A1", category_code="N04", average_price_twd_per_kg=10.0,
                           volume_kg=100.0, source_snapshot_id="snap", record_hash="h4", revision_no=1,
                           longitude=120.90, latitude=24.08, surface_region="test_region", coordinate_source="test"),
    ]
    config = SpatialConfig(min_points=4, grid_resolution_m=2500, idw_power=2, max_support_distance_km=50,
                            min_supporting_neighbors=3, min_supported_land_ratio=0.1, max_grid_cells_per_region=2000)
    surface = build_spatial_surface(obs, boundary=boundary, contour_breaks=(15.0, 25.0, 35.0), config=config)
    fixture = {
        "region_ring_wgs84": ring,
        "status": surface.status,
        "surface_geojson": surface.surface_geojson,
        "contours_geojson": surface.contours_geojson,
        "points_geojson": surface.points_geojson,
        "config": {"grid_resolution_m": 2500, "idw_power": 2, "max_support_distance_km": 50,
                   "min_supporting_neighbors": 3, "contour_breaks": [15.0, 25.0, 35.0]},
    }
    out_path = Path(__file__).resolve().parent.parent / "spatial-fixture.json"
    out_path.write_text(json.dumps(fixture, indent=2, sort_keys=True))
    print(f"wrote {out_path} ({len(surface.surface_geojson['features'])} surface + "
          f"{len(surface.contours_geojson['features'])} contour features)")


if __name__ == "__main__":
    main()
