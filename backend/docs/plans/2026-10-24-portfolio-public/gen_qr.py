#!/usr/bin/env python3
"""重新產生 App 的示範 QR 圖（`rece_qr.png`、`rece_qr2.png`），內容是 https://example.com。

用法（repo 根目錄；segno 不在 repo 的相依裡，放進暫存 venv）：
  python3 -m venv /tmp/qr-venv && /tmp/qr-venv/bin/pip install segno
  /tmp/qr-venv/bin/python backend/docs/plans/2026-10-24-portfolio-public/gen_qr.py

尺寸維持原圖的 225x225：QR 版本 2（25 格）加四周各 10 格留白，每格 5 像素。
"""
import segno

TARGETS = [
    "buyer-app/assets/rece_qr.png",
    "buyer-app/assets/rece_qr2.png",
    "seller-app/assets/rece_qr.png",
    "seller-app/assets/rece_qr2.png",
]

qr = segno.make("https://example.com", version=2, error="m", micro=False)
for path in TARGETS:
    qr.save(path, scale=5, border=10)
