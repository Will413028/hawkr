#!/usr/bin/env bash
# 印出 proto 生成用 image 的完整參照（含 tag）。
#
#   hawkr-proto-gen:<sha256(scripts/proto/Dockerfile) 前 12 碼>
#
# ## 為什麼 tag 要內容定址
#
# 此前 tag 寫死成 PROTOC_VERSION（`hawkr-proto-gen:33.4`），而 cache 判斷是
# `docker image inspect <tag>` ——命中就不重建。protoc 與兩個 plugin 的版本都由
# Dockerfile 的 ARG 釘死了，但**跑 `go install`、決定生成檔註解排版的 Go toolchain 沒有**：
# base 是 `golang:1.25-alpine` 這種可變 tag，不同時間點建出來的 image 裡是不同的 Go patch
# 版本，而 Go 的 doc comment 格式化行為在 patch 之間會變。
#
# 後果是「跑在哪台 runner 上決定生成結果」：各台 runner 在不同時間建過同一個 tag，
# 內容不同卻都命中 cache。2026-08-20 實際發生——proto 對拍在 backend-payuni 的 runner
# 上一路綠，落到 hawkr-website 的 runner 就紅，diff 全是註解裡全形引號前的空白與
# 縮排。那不是有人改壞了什麼，是 CI 從來沒有真的釘住這個 image。
#
# tag 改成 Dockerfile 內容的雜湊之後：改 base digest／protoc 版本／任何一行 → tag 變 →
# 所有 runner 都必須重建 → 大家收斂到同一個 image。跟 go-base 與 22 個服務用的是同一招
# （見 k8s/scripts/go-base-tag.sh、k8s/scripts/content-addressed-tag.sh）。
#
# 這也讓「IMAGE tag 必須等於 PROTOC_VERSION」那條人工對帳規則消失——tag 由檔案內容算出，
# 沒有第二個地方可以寫錯。
set -euo pipefail

# 以腳本位置定位 backend 根目錄：monorepo 裡 git 頂層是上一層，不是 backend/。
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
F="scripts/proto/Dockerfile"

# macOS 內建沒有 sha256sum（是 shasum -a 256），本腳本開發機與 CI 都要能跑。
if command -v sha256sum >/dev/null 2>&1; then
  H=$(sha256sum "$F" | cut -c1-12)
else
  H=$(shasum -a 256 "$F" | cut -c1-12)
fi

printf 'hawkr-proto-gen:%s\n' "$H"
