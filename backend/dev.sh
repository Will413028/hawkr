#!/usr/bin/env bash
# =============================================================================
# Hawkr 本地開發工具
# =============================================================================
set -euo pipefail

# ─── 常數 ───────────────────────────────────────────────────────────────────
COMPOSE_FILE="docker-compose-local.yml"
COMPOSE="docker compose -f $COMPOSE_FILE"

# 基礎設施
INFRA_SERVICES=(postgres redis rabbitmq otel-collector)

# Go 服務（Air hot-reload，改完自動生效）
GO_SERVICES=(gateway social-service ai-service im-service review-service crm-service notification-service invoice-service)

# Java 服務（需 rebuild image 才能更新）
JAVA_SERVICES=(payment-service media-service product-service order-service cart-service coupon-service shipping-service aftersales-service payuni-webhook-service product-import-service)

# 全部應用服務
ALL_APP_SERVICES=("${GO_SERVICES[@]}" "${JAVA_SERVICES[@]}")

# 顏色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ─── 工具函數 ────────────────────────────────────────────────────────────────
info()  { echo -e "${BLUE}ℹ ${NC}$*"; }
ok()    { echo -e "${GREEN}✓ ${NC}$*"; }
warn()  { echo -e "${YELLOW}⚠ ${NC}$*"; }
err()   { echo -e "${RED}✗ ${NC}$*"; }
title() { echo -e "\n${BOLD}${CYAN}══ $* ══${NC}\n"; }

# 服務歸類判斷（rebuild / status 診斷共用）
is_go_service()   { printf '%s\n' "${GO_SERVICES[@]}"   | grep -qx "$1"; }
is_java_service() { printf '%s\n' "${JAVA_SERVICES[@]}" | grep -qx "$1"; }

# Go dev container 的 Air build 是否失敗（#2025）
# 為何需要：Go 服務的 PID 1 是 air，不是服務本身。build 失敗時 air 依然活著，容器維持
# `Up`（只是 unhealthy），docker 不會重啟也不會反映成容器層級失敗——實測 6 個服務就這樣
# 「Up 但沒在服務」8~9 天無人察覺。這裡直接讀 log 特徵，把根因與解法講出來。
# 涵蓋兩類：編譯失敗（GOPROXY／build error／exit 127），以及編譯成功但程序起不來
# （port 被殘留 process 佔住 → bind 失敗 → exit 1）。後者特別陰險：舊 process 繼續
# 服務，容器一路 healthy，於是每次 restart 都「成功」而跑的仍是舊 binary。
air_build_failed() {
  docker logs --tail 60 "$1" 2>&1 \
    | grep -qE 'module lookup disabled by GOPROXY|failed to build|Process Exit with Code: 127|Process Exit with Code: 1$|address already in use'
}

# 容器 healthy、但跑的其實是舊 binary（issue #2034 順帶；上面那個函式的註解描述
# 的「特別陰險」情境，原本沒有任何地方會去查）
#
# 為何 air_build_failed 抓不到：cmd_status 只在「非 healthy」那條分支才呼叫它。
# 而這個情境的定義就是**容器一路 healthy**——舊 process 還佔著 port 繼續服務、
# healthcheck 照樣過，新建出來的 binary 每次都 bind 失敗當場結束。實測本機
# gateway 就是這樣：health=healthy，air 最後一筆是 `Process Exit with Code: 1`
# ＋`listen tcp :9900: bind: address already in use`，於是所有人以為在測新碼，
# 其實測的是 10 小時前的 binary。
#
# 判準用「air 生命週期的最後一筆標記」而不是「最近 N 行有沒有出現失敗字樣」：
# 後者會把「失敗過但之後已重建成功」誤報成壞的。air 的詞彙是穩定的
# building... → running...（成功）／Process Exit with Code: N（失敗），只看最後
# 一筆是哪一種即可。building... 不算終局（正在建置中），故不納入比較。
air_running_stale_binary() {
  local last
  last=$(docker logs --tail 400 "$1" 2>&1 \
    | grep -oE 'running\.\.\.|Process Exit with Code: [0-9]+' | tail -1)
  [[ "$last" == Process* ]]
}

# 補建缺少的資料庫（issue #2034）
#
# 為何需要：scripts/init-databases.sh 是資料庫清單的唯一真實來源，但它掛在
# postgres 的 /docker-entrypoint-initdb.d/ 下，**只有在 data volume 初次建立那一次**
# 會被執行。之後新增的服務在既存開發機上永遠拿不到自己的 DB（所有既存開發機的 volume
# 都早於它建立），服務啟動時直接 `database "..." does not exist` 死在 Fx。
# 這是「volume 已存在 + 之後才加服務」這整類的結構性缺口，所以補在 up 的共用路徑上。
#
# 直接在容器內重跑同一份腳本而不是在 dev.sh 複製一份清單：清單留在單一位置，
# 不會有 dev.sh 與 init-databases.sh 各自漂移的第二份真實來源。該腳本本身以
# `SELECT 1 FROM pg_database` 判存在、CREATE EXTENSION 用 IF NOT EXISTS，天生冪等，
# 對已存在的資料庫完全不寫入（不會動到既有本地資料）。
#
# 用 `bash -s` 從 stdin 餵而不是執行容器內的 /docker-entrypoint-initdb.d/ 掛載點：
# 那個 bind mount 綁的是 host 檔案的 inode，之後任何一次改寫（git checkout／編輯器
# 覆寫）都會讓長壽的 postgres 容器裡那條路徑指向已刪除的 inode——實測本機該路徑
# 就是 `No such file or directory`（容器 12 天前建立，腳本 07-28 才被 commit 改過）。
# 從 stdin 餵永遠讀到當下 host 上的版本，不受容器年齡影響。
ensure_databases() {
  info "檢查資料庫是否齊全..."
  local out
  if ! out=$(docker exec -i hawkr-postgres bash -s -- local < scripts/init-databases.sh 2>&1); then
    warn "資料庫補建未全數成功，缺 DB 的服務會起不來（詳見下方輸出）"
    echo "$out" | grep -E '✗|失敗' || echo "$out" | tail -5
    return 0
  fi
  local created
  created=$(echo "$out" | grep -c '建立成功' || true)
  if [[ "${created:-0}" -gt 0 ]]; then
    ok "補建了 ${created} 個缺少的資料庫"
    echo "$out" | grep '建立成功' | sed 's/^/    /'
    warn "新建的 Go 服務資料庫是空的（無 schema），需要 ./dev.sh seed 套用 Atlas migrations"
  else
    ok "資料庫齊全"
  fi
}

# ─── 指令：up ────────────────────────────────────────────────────────────────
cmd_up() {
  title "啟動本地開發環境"

  # 1. 檢查是否有本機 process 佔用 9900 port
  local pid
  pid=$(lsof -ti :9900 -sTCP:LISTEN 2>/dev/null | head -1 || true)
  if [[ -n "$pid" ]]; then
    local cmd_name
    cmd_name=$(ps -p "$pid" -o comm= 2>/dev/null || echo "unknown")
    local cmd_lower="$(printf '%s' "$cmd_name" | tr '[:upper:]' '[:lower:]')"
    if [[ "$cmd_lower" != *"docker"* && "$cmd_lower" != *"orbstack"* ]]; then
      warn "Port 9900 被本機 process 佔用 (PID=$pid, $cmd_name)"
      read -rp "   是否終止它？[Y/n] " ans
      if [[ "${ans:-Y}" =~ ^[Yy]$ ]]; then
        kill "$pid" 2>/dev/null && ok "已終止 PID $pid"
        sleep 1
      fi
    fi
  fi

  # 2. 先啟動基礎設施
  info "啟動基礎設施..."
  $COMPOSE up -d "${INFRA_SERVICES[@]}"

  # 3. 等待基礎設施 healthy
  info "等待基礎設施就緒..."
  local max_wait=60
  local waited=0
  while [[ $waited -lt $max_wait ]]; do
    local all_ready=true
    for svc in postgres redis rabbitmq; do
      local status
      status=$(docker inspect --format='{{.State.Health.Status}}' "hawkr-${svc}" 2>/dev/null || echo "missing")
      if [[ "$status" != "healthy" ]]; then
        all_ready=false
        break
      fi
    done
    if $all_ready; then break; fi
    sleep 2
    waited=$((waited + 2))
  done

  if [[ $waited -ge $max_wait ]]; then
    warn "基礎設施尚未完全就緒，部分服務可能需要稍後重啟"
  else
    ok "基礎設施就緒 (${waited}s)"
  fi

  # 3.5 補建缺少的資料庫（issue #2034）
  ensure_databases

  # 4. 基礎設施就緒後，啟動全部應用服務
  info "啟動全部微服務..."
  $COMPOSE up -d

  # 5. 等待應用服務就緒（Gateway health check 全綠）
  info "等待微服務啟動..."
  local app_wait=0
  local app_max=90
  while [[ $app_wait -lt $app_max ]]; do
    # 以 compose healthcheck 為準：具 healthcheck 的容器全達 (healthy) 即就緒
    # （otel-collector 無 healthcheck，排除不計；啟動中的容器不含 (healthy) 會續等）
    local not_ready
    not_ready=$($COMPOSE ps --format '{{.Name}}\t{{.Status}}' 2>/dev/null \
      | grep -v 'otel-collector' | awk '$0 !~ /\(healthy\)/ {c++} END {print c+0}')
    if [[ "${not_ready:-1}" -eq 0 ]]; then
      ok "全部微服務就緒 (${app_wait}s)"
      break
    fi
    sleep 3
    app_wait=$((app_wait + 3))
  done
  if [[ $app_wait -ge $app_max ]]; then
    warn "部分微服務尚未就緒，請稍後執行 ./dev.sh status 確認"
  fi

  # 6. 啟動 ngrok（如果尚未啟動）
  start_ngrok

  # 7. 顯示狀態（cmd_status 異常時回 1，但 up 本身不因此失敗——上面已 warn 過）
  echo ""
  cmd_status || true
}

# ─── ngrok 自動啟動 ──────────────────────────────────────────────────────────
start_ngrok() {
  # 檢查 ngrok 是否已在運行
  local existing_url
  existing_url=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null \
    | python3 -c "import sys,json; ts=json.load(sys.stdin).get('tunnels',[]); print(next((t['public_url'] for t in ts if 'https' in t['public_url']),''))" 2>/dev/null || echo "")

  if [[ -n "$existing_url" ]]; then
    ok "ngrok 已在運行"
  else
    # 檢查 ngrok 是否已安裝
    if ! command -v ngrok &>/dev/null; then
      warn "ngrok 未安裝，跳過（安裝方式：brew install ngrok）"
      return
    fi

    info "啟動 ngrok..."
    ngrok http 9900 --log=stdout --log-level=warn > /dev/null 2>&1 &
    disown

    # 等待 ngrok 啟動
    local waited=0
    while [[ $waited -lt 10 ]]; do
      existing_url=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null \
        | python3 -c "import sys,json; ts=json.load(sys.stdin).get('tunnels',[]); print(next((t['public_url'] for t in ts if 'https' in t['public_url']),''))" 2>/dev/null || echo "")
      if [[ -n "$existing_url" ]]; then break; fi
      sleep 1
      waited=$((waited + 1))
    done

    if [[ -z "$existing_url" ]]; then
      warn "ngrok 啟動逾時，請手動執行：ngrok http 9900"
      return
    fi
    ok "ngrok 已啟動"
  fi

  # 複製到剪貼簿並顯示
  echo "$existing_url" | pbcopy
  echo ""
  echo -e "  ${BOLD}📋 ngrok URL（已複製到剪貼簿）：${NC}"
  echo -e "  ${GREEN}${BOLD}$existing_url${NC}"
  echo ""
  info "請將此 URL 貼到 Flutter App 的 API base URL"

  # 注入 GATEWAY_URL 到需要回調 URL 的 Java 服務
  inject_gateway_url "$existing_url"
}

# ─── 注入 GATEWAY_URL 到 Java 服務 ────────────────────────────────────────────
# payment-service、shipping-service 需要 GATEWAY_URL 來組裝外部回調 URL
# 同步更新 .env 中所有 ngrok URL（GATEWAY_URL、PAYUNI_*_URL）
inject_gateway_url() {
  local ngrok_url="$1"
  local env_file=".env"

  # 更新 .env 中的 ngrok URL（取代舊的 ngrok URL）
  if [[ -f "$env_file" ]]; then
    # 替換所有 https://*.ngrok-free.app 或 https://*.ngrok.io 為新 URL
    sed -i '' -E "s|https://[a-z0-9-]+\.ngrok-free\.app|${ngrok_url}|g" "$env_file"
    sed -i '' -E "s|https://[a-z0-9-]+\.ngrok\.io|${ngrok_url}|g" "$env_file"
    ok ".env 已更新 ngrok URL"
  fi

  # 檢查服務是否正在運行
  local running
  running=$(docker ps --format '{{.Names}}' | grep -E '^(payment-service|shipping-service|aftersales-service)$' || true)
  if [[ -z "$running" ]]; then
    return
  fi

  info "重啟 payment-service、shipping-service、aftersales-service 套用新 URL..."
  $COMPOSE up -d --no-deps payment-service shipping-service aftersales-service 2>/dev/null
  ok "GATEWAY_URL 已更新：$ngrok_url"
}

# ─── 指令：down ──────────────────────────────────────────────────────────────
cmd_down() {
  title "停止所有服務"

  # 停止 ngrok
  local ngrok_pid
  ngrok_pid=$(pgrep -f "ngrok http" || true)
  if [[ -n "$ngrok_pid" ]]; then
    kill "$ngrok_pid" 2>/dev/null && info "ngrok 已停止"
  fi

  $COMPOSE down
  ok "已停止"
}

# ─── 指令：rebuild ───────────────────────────────────────────────────────────
cmd_rebuild() {
  title "重建微服務"

  local services_to_rebuild=()
  local go_to_rebuild=()

  if [[ $# -gt 0 ]]; then
    # 手動指定服務
    for svc in "$@"; do
      # 支援簡寫（如 payment → payment-service）；gateway 本身沒有 -service 後綴
      if [[ "$svc" != *-service ]] && ! is_go_service "$svc"; then
        svc="${svc}-service"
      fi
      if is_java_service "$svc"; then
        services_to_rebuild+=("$svc")
      elif is_go_service "$svc"; then
        # Go 服務平時靠 Air 熱重載，不需 rebuild；但 dev image 烘進 layer 的
        # /go/pkg/mod 快照過期時（#2025），容器內 go build 會解不了 module graph，
        # 此時重建 image 是唯一復原路徑。過去這裡直接拒絕並回「無需 rebuild」，
        # 把開發者導向死路，是 #2025 拖了 8 天沒解的原因之一。
        go_to_rebuild+=("$svc")
      else
        warn "$svc 不是本專案的服務（用 ./dev.sh help 看服務清單）"
      fi
    done
  else
    # 自動偵測：比較 git 變更
    info "偵測 Java 服務程式碼變更..."
    local changed_files
    changed_files=$(git diff --name-only HEAD 2>/dev/null || true)
    changed_files+=$'\n'
    changed_files+=$(git diff --name-only --cached 2>/dev/null || true)
    changed_files+=$'\n'
    changed_files+=$(git ls-files --others --exclude-standard 2>/dev/null || true)

    # 檢查 services/java/shared/ 是否有變更（影響所有 Java 服務）
    local shared_changed=false
    if echo "$changed_files" | grep -q "^services/java/shared/"; then
      shared_changed=true
      warn "services/java/shared/ 有變更，所有 Java 服務都需要 rebuild"
    fi

    for svc in "${JAVA_SERVICES[@]}"; do
      local svc_dir="services/java/${svc}"
      if $shared_changed; then
        services_to_rebuild+=("$svc")
      elif echo "$changed_files" | grep -q "^${svc_dir}/"; then
        services_to_rebuild+=("$svc")
      fi
    done

    if [[ ${#services_to_rebuild[@]} -eq 0 ]]; then
      ok "沒有偵測到 Java 服務程式碼變更"
      info "提示：可手動指定 → ./dev.sh rebuild payment shipping"
      return 0
    fi
  fi

  if [[ ${#services_to_rebuild[@]} -eq 0 && ${#go_to_rebuild[@]} -eq 0 ]]; then
    warn "沒有需要 rebuild 的服務"
    return 0
  fi

  # ── Go 服務：不經 mvn，重建 dev image 以刷新 module cache 快照（#2025）─────────
  if [[ ${#go_to_rebuild[@]} -gt 0 ]]; then
    echo -e "${BOLD}即將 rebuild（Go dev image）：${NC}${go_to_rebuild[*]}"
    echo ""
    for svc in "${go_to_rebuild[@]}"; do
      info "Docker building $svc..."
      if $COMPOSE build "$svc" 2>&1 | tail -3; then
        ok "$svc image build 完成"
      else
        err "$svc image build 失敗"
        continue
      fi
      info "Restarting $svc..."
      $COMPOSE up -d --force-recreate "$svc"
      ok "$svc 已重啟"
      echo ""
    done
  fi

  if [[ ${#services_to_rebuild[@]} -eq 0 ]]; then
    info "等待服務就緒..."
    sleep 8
    cmd_status || true
    return 0
  fi

  echo -e "${BOLD}即將 rebuild：${NC}${services_to_rebuild[*]}"
  echo ""

  # 用 mvnw reactor 一次編完所有變更服務的 jar（shared 自動帶上 -am）。
  # 詳細設計：docs/基礎建設/CI_CD 自動化部署（K8s + Helm）.md §4.3 / §9.2。
  # thin Dockerfile 純 COPY 預編 jar，必須先在 host 跑 mvnw 產出 services/java/*/target/*.jar。
  # -Dmaven.test.skip=true：跳過 test 編譯與執行（對齊舊 Dockerfile 行為，.dockerignore
  # 排除了 **/*Test.java；測試由獨立 CI step 負責，不阻擋 build artifact）。
  local mvn_pl
  mvn_pl=$(printf ":%s," "${services_to_rebuild[@]}" | sed 's/,$//')
  info "(cd services/java && ./mvnw install -pl $mvn_pl -am -T 1C -Dmaven.test.skip=true)"
  if ! (cd services/java && ./mvnw install -pl "$mvn_pl" -am -T 1C -B -Dmaven.test.skip=true); then
    err "mvnw reactor build 失敗，rebuild 中止"
    return 1
  fi
  ok "mvnw reactor build 完成"

  for svc in "${services_to_rebuild[@]}"; do
    # thin Dockerfile 純 COPY jar，不需要 --no-cache
    info "Docker building $svc..."
    if $COMPOSE build "$svc" 2>&1 | tail -3; then
      ok "$svc image build 完成"
    else
      err "$svc image build 失敗"
      continue
    fi

    info "Restarting $svc..."
    $COMPOSE up -d --force-recreate "$svc"
    ok "$svc 已重啟"
    echo ""
  done

  # 自動清理 rebuild 殘留的 dangling 映像與過期 build cache（避免 OrbStack 膨脹）
  info "清理 rebuild 殘留..."
  local pruned_images pruned_cache
  pruned_images=$(docker image prune -f 2>/dev/null | awk '/Total reclaimed space/{print $NF$(NF-1)}' || echo "0")
  pruned_cache=$(docker builder prune -f --filter "until=24h" 2>/dev/null | awk '/Total:/{print $NF$(NF-1)}' || echo "0")
  ok "已回收 dangling 映像 ${pruned_images:-0}、build cache ${pruned_cache:-0}"

  # 等待服務就緒
  info "等待服務就緒..."
  sleep 8
  cmd_status || true
}

# ─── 指令：clean ─────────────────────────────────────────────────────────────
# 顯式深度清理 Docker 佔用空間（不影響跑中的容器、不動有 tag 的映像）
cmd_clean() {
  title "Docker 深度清理"

  # 顯示清理前狀態
  info "目前 Docker 用量:"
  docker system df
  echo ""

  warn "將執行:"
  echo "  1. docker image prune -f       (清 dangling <none> 映像)"
  echo "  2. docker builder prune -af    (清所有 build cache)"
  echo "  3. docker container prune -f   (清已停止容器)"
  echo ""
  warn "不會動到:跑中的容器、有 tag 的映像、volumes"
  read -rp "確認執行?[y/N] " ans
  if [[ ! "${ans:-N}" =~ ^[Yy]$ ]]; then
    info "已取消"
    return 0
  fi

  info "1/3 清理 dangling 映像..."
  docker image prune -f

  info "2/3 清理 build cache..."
  docker builder prune -af

  info "3/3 清理已停止容器..."
  docker container prune -f

  echo ""
  info "清理後 Docker 用量:"
  docker system df
  echo ""
  warn "⚠ OrbStack 的 data.img.raw 是 sparse file,實體磁碟空間不會立刻縮"
  warn "  到 OrbStack → Settings → Disk → Reclaim space 才會真正歸還給 macOS"
}

# ─── 指令：logs ──────────────────────────────────────────────────────────────
cmd_logs() {
  local services=()
  local follow=true
  local level=""
  local tail_lines=80
  local grep_pattern=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --no-follow|-n) follow=false; shift ;;
      --tail|-t)      tail_lines="$2"; shift 2 ;;
      --error|-e)     level="error"; shift ;;
      --warn|-w)      level="warn"; shift ;;
      --grep|-g)      grep_pattern="$2"; shift 2 ;;
      --all|-a)       shift ;;  # 預設就是全部
      -*)             warn "未知選項: $1"; shift ;;
      *)
        local svc="$1"
        [[ "$svc" != *-service && "$svc" != "gateway" ]] && svc="${svc}-service"
        services+=("$svc")
        shift
        ;;
    esac
  done

  # 構建 compose logs 指令
  local cmd="$COMPOSE logs --tail=$tail_lines"
  if $follow; then
    cmd+=" -f"
  fi

  if [[ ${#services[@]} -gt 0 ]]; then
    cmd+=" ${services[*]}"
  else
    # 預設只看我們負責的服務 + gateway
    cmd+=" gateway payment-service media-service product-service order-service cart-service coupon-service shipping-service"
  fi

  title "查看日誌"
  if [[ ${#services[@]} -gt 0 ]]; then
    info "服務：${services[*]}"
  else
    info "服務：我們負責的微服務（加 --all 看全部）"
  fi
  if [[ -n "$level" ]]; then
    info "過濾：$level 等級以上"
  fi
  if [[ -n "$grep_pattern" ]]; then
    info "搜尋：$grep_pattern"
  fi
  echo ""

  # 執行
  if [[ -n "$level" || -n "$grep_pattern" ]]; then
    local filter=""
    case "$level" in
      error) filter="ERROR\|FATAL\|PANIC\|error\|fatal\|panic" ;;
      warn)  filter="WARN\|ERROR\|FATAL\|warn\|error\|fatal" ;;
    esac
    if [[ -n "$grep_pattern" && -n "$filter" ]]; then
      filter="$filter\|$grep_pattern"
    elif [[ -n "$grep_pattern" ]]; then
      filter="$grep_pattern"
    fi
    eval "$cmd" 2>&1 | grep --line-buffered -i "$filter"
  else
    eval "$cmd" 2>&1
  fi
}

# ─── 指令：status ────────────────────────────────────────────────────────────
# 退出碼：0 = 全數健康；1 = 有異常（可當機器可判讀的閘門用，例：`./dev.sh status || exit 1`）
cmd_status() {
  title "服務狀態"

  # 狀態以 docker compose 的 healthcheck 為準（Gateway 無 /health/all 聚合端點）。
  # otel-collector 無 healthcheck，只要 Up 即視為正常；啟動中的容器顯示「啟動中」。
  # --all（#2025）：預設輸出只含跑中的容器，整個沒起來的服務會直接從清單消失、
  #                 反而讓總結顯示 HEALTHY。列出 Exited/Created 才不會漏。
  local ps_out
  ps_out=$($COMPOSE ps --all --format '{{.Name}}\t{{.Service}}\t{{.Status}}' 2>/dev/null || true)
  if [[ -z "$ps_out" ]]; then
    warn "無法取得容器狀態（docker compose 未執行？先跑 ./dev.sh up）"
    return 1
  fi

  local total=0 healthy=0 problem=0 name svc status
  local seen=" "
  local broken_go=()
  local stale_go=()
  while IFS=$'\t' read -r name svc status; do
    [[ -z "$name" ]] && continue
    seen+="${svc} "
    total=$((total + 1))
    if [[ "$status" == *"(healthy)"* ]]; then
      # healthy 不等於「跑的是最新的 code」：舊 process 佔著 port 時，
      # healthcheck 會一直過，而新 binary 每次都 bind 失敗（見 air_running_stale_binary）。
      if is_go_service "$svc" && air_running_stale_binary "$name"; then
        problem=$((problem + 1))
        stale_go+=("$svc")
        echo -e "  ${RED}✗${NC} ${name}  容器 healthy，但 Air 最新一次啟動失敗＝服務跑的是舊 binary"
      else
        healthy=$((healthy + 1))
        echo -e "  ${GREEN}✓${NC} ${name}"
      fi
    elif [[ "$status" == *"starting"* ]]; then
      echo -e "  ${YELLOW}…${NC} ${name}  啟動中"
    elif [[ "$name" == *"otel-collector"* && "$status" == Up* ]]; then
      healthy=$((healthy + 1))
      echo -e "  ${GREEN}✓${NC} ${name}  (無 healthcheck)"
    else
      problem=$((problem + 1))
      echo -e "  ${RED}✗${NC} ${name}  ${status}"
      if is_go_service "$svc" && air_build_failed "$name"; then
        broken_go+=("$svc")
      fi
    fi
  done <<< "$ps_out"

  # compose 有定義、但連容器都沒建出來的服務（--all 也看不到）
  local s
  for s in $($COMPOSE config --services 2>/dev/null || true); do
    [[ "$seen" == *" ${s} "* ]] && continue
    total=$((total + 1))
    problem=$((problem + 1))
    echo -e "  ${RED}✗${NC} ${s}  容器未建立（compose 有定義但從未啟動）"
  done

  echo ""
  if [[ $problem -eq 0 ]]; then
    echo -e "  整體狀態：${GREEN}${BOLD}HEALTHY${NC}  (${healthy}/${total})"
    return 0
  fi

  echo -e "  整體狀態：${RED}${BOLD}DEGRADED${NC}  (${healthy}/${total}，${problem} 異常)"
  if [[ ${#broken_go[@]} -gt 0 ]]; then
    echo ""
    err "下列 Go 服務容器是 Up，但 Air build 失敗＝服務並沒有在跑："
    printf '       - %s\n' "${broken_go[@]}"
    echo -e "     ${BOLD}解法${NC}：./dev.sh rebuild ${broken_go[*]}"
  fi
  if [[ ${#stale_go[@]} -gt 0 ]]; then
    echo ""
    err "下列 Go 服務 healthcheck 是綠的，但服務跑的是舊 binary（新 binary 每次啟動都失敗）："
    printf '       - %s\n' "${stale_go[@]}"
    echo -e "     多半是舊 process 沒被收掉還佔著 port，於是你以為在測新 code，其實不是。"
    echo -e "     ${BOLD}解法${NC}：./dev.sh restart ${stale_go[*]}（仍不行再 ./dev.sh rebuild ${stale_go[*]}）"
  fi
  return 1
}

# ─── 指令：seed ──────────────────────────────────────────────────────────────
# 職責分離（issue #1497）：seed 只負責灌測試資料；schema 一律走各工具的正常 migrate 命令。
#   - Go 服務（buyer/seller 等）：Atlas versioned migrate apply（有 atlas_schema_revisions
#     版本表、天生冪等），drop+recreate 該服務專屬 DB 後套用完整 migrations/ 歷史。
#     為何 drop+recreate 而非就地 apply：本地 DB 過去由已移除的「逐檔 psql 重放 + `|| true` 吞錯」
#     機制建立、從未被 atlas 追蹤版本。多次呼叫 seed 會讓同一個 ADD COLUMN / RENAME COLUMN
#     語句反覆重跑且錯誤被吞掉，實測已造成 sellers 表同時存在殭屍重建的 seller_account_id
#     與正確 rename 結果 store_id、且 chk_sellers_status / chk_sellers_active_is_current 兩個
#     CHECK 約束遺失。本地帳號/賣家/買家本來就是可重建的測試資料，drop+recreate 用真正的
#     Atlas apply 從乾淨狀態重建，是唯一能保證與 migrations/ 目錄真實歷史一致（且天生冪等）
#     的方式；就地 apply 需要幫已污染的歷史狀態猜 baseline，等於把污染攤平成「已驗證」。
#   - Java 服務：Flyway 於服務啟動時自動套用，seed 不代勞、不重放。
# 全程 fail-loud（無 `|| true` / 錯誤吞噬）：任何一步失敗立即中止並印出真實錯誤。
GO_SEED_SERVICES=(notification-service ai-service crm-service im-service review-service social-service invoice-service)

go_seed_db_for() {
  case "$1" in
    notification-service) echo "hawkr_notification" ;;
    ai-service)           echo "hawkr_ai" ;;
    crm-service)          echo "hawkr_crm" ;;
    im-service)           echo "hawkr_im" ;;
    review-service)       echo "hawkr_review" ;;
    social-service)       echo "hawkr_social" ;;
    invoice-service)      echo "hawkr_invoice" ;;
    *) err "go_seed_db_for: 未知服務 $1"; exit 1 ;;
  esac
}

cmd_seed() {
  title "匯入測試資料"

  # 0. 前置檢查：atlas CLI 是保證 schema 正確的唯一手段，缺少就直接中止（不硬塞 raw SQL 重放）
  if ! command -v atlas &>/dev/null; then
    err "找不到 atlas CLI，無法保證 Go 服務 schema 為最新，中止 seed"
    err "請先安裝：brew install ariga/tap/atlas"
    return 1
  fi
  if ! docker inspect hawkr-postgres &>/dev/null; then
    err "找不到 hawkr-postgres 容器，請先 ./dev.sh up"
    return 1
  fi

  # 帳號、買家與店家在單體的 identity、buyer、seller schema（seed 不代跑單體的 migration，同 Java 服務的 Flyway）
  if [[ "$(docker exec hawkr-postgres psql -U dbadmin -d hawkr -tAc "SELECT to_regclass('identity.accounts') IS NOT NULL AND to_regclass('buyer.buyers') IS NOT NULL AND to_regclass('seller.stores') IS NOT NULL" 2>/dev/null)" != "t" ]]; then
    err "單體的 identity、buyer 或 seller schema 不存在，請先在 backend 跑 make migrate"
    return 1
  fi

  local db_password
  db_password=$(docker exec hawkr-postgres printenv POSTGRES_PASSWORD)

  # 1. Go 服務 schema：drop + recreate 專屬 DB，再用 atlas migrate apply 套用完整版本歷史
  info "同步 Go 服務 schema（Atlas migrate apply）..."
  for service in "${GO_SEED_SERVICES[@]}"; do
    local db
    db=$(go_seed_db_for "$service")
    info "  [$service] 重建 $db..."
    docker exec -i hawkr-postgres psql -U dbadmin -d postgres \
      -c "DROP DATABASE IF EXISTS \"$db\" WITH (FORCE);"
    docker exec -i hawkr-postgres psql -U dbadmin -d postgres \
      -c "CREATE DATABASE \"$db\";"
    (cd "services/${service}/db" && atlas migrate apply \
      --dir "file://migrations" \
      --url "postgres://dbadmin:${db_password}@localhost:5432/${db}?sslmode=disable")
    ok "  [$service] schema 已套用至最新"
  done

  # 2. 匯入 seed data（infra/seed-test-data.sh 本身已 fail-loud，見該檔）
  echo ""
  info "匯入測試資料..."
  ./infra/seed-test-data.sh

  echo ""
  ok "完成！重啟受影響的服務以載入新資料..."
  $COMPOSE restart "${GO_SEED_SERVICES[@]}"
}

# ─── 指令：restart ───────────────────────────────────────────────────────────
cmd_restart() {
  if [[ $# -eq 0 ]]; then
    warn "請指定服務名稱，例如：./dev.sh restart shipping payment"
    return 1
  fi

  for svc in "$@"; do
    [[ "$svc" != *-service && "$svc" != "gateway" ]] && svc="${svc}-service"
    info "重啟 $svc..."
    $COMPOSE restart "$svc"
    ok "$svc 已重啟"
  done
}

# ─── 指令：ngrok ─────────────────────────────────────────────────────────────
cmd_ngrok() {
  title "ngrok 通道"
  start_ngrok
}

# ─── 指令：help ──────────────────────────────────────────────────────────────
cmd_help() {
  cat <<'HELP'

  ╔══════════════════════════════════════════════════════════╗
  ║        Hawkr 本地開發工具  (dev.sh)                ║
  ╚══════════════════════════════════════════════════════════╝

  使用方式：./dev.sh <指令> [選項]

  ─── 啟動 / 停止 ────────────────────────────────────────────

    up              啟動全部服務（基礎設施 + 微服務 + ngrok）
                    • 自動偵測 port 衝突並提示處理
                    • 等待基礎設施 healthy 後顯示狀態
                    • 自動啟動 ngrok 並複製 URL 到剪貼簿

    down            停止全部服務

    restart <服務>  重啟指定服務（支援簡寫：payment = payment-service）
                    例：./dev.sh restart shipping gateway

  ─── Java 服務更新 ──────────────────────────────────────────

    rebuild         自動偵測有程式碼變更的 Java 服務並重建
                    • 比對 git diff 判斷哪些服務需要 rebuild
                    • services/java/shared/ 變更時全部 Java 服務 rebuild
                    • 不自動偵測 Go 服務（改 code 由 Air 自動生效，不需 rebuild）

    rebuild <服務>  手動指定要 rebuild 的服務（Java 或 Go 皆可）
                    例：./dev.sh rebuild payment shipping
                    例：./dev.sh rebuild order  (自動補 -service)
                    例：./dev.sh rebuild social-service   (Go：重建 dev image)

                    Go 服務何時需要手動 rebuild：./dev.sh status 報
                    「Up 但 Air build 失敗」時——dev image 烘進 layer 的
                    /go/pkg/mod 快照太舊，重建 image 才能刷新（#2025）。

  ─── 日誌查看 ────────────────────────────────────────────────

    logs            即時追蹤我們負責的微服務日誌
    logs <服務>     即時追蹤指定服務日誌
                    例：./dev.sh logs shipping gateway

    logs 選項：
      -e, --error     只顯示 ERROR/FATAL 等級
      -w, --warn      只顯示 WARN 以上等級
      -g, --grep <關鍵字>  搜尋包含關鍵字的日誌
      -t, --tail <行數>    起始顯示行數（預設 80）
      -n, --no-follow      不即時追蹤，只顯示歷史日誌

    常用範例：
      ./dev.sh logs -e                    # 只看錯誤
      ./dev.sh logs shipping -g "PayUni"  # 看 shipping 中含 PayUni 的日誌
      ./dev.sh logs gateway -w            # 看 gateway 的警告和錯誤
      ./dev.sh logs -n -t 200             # 顯示最近 200 行（不追蹤）

  ─── 其他工具 ────────────────────────────────────────────────

    status          顯示所有服務健康狀態（依 docker compose healthcheck）
                    • 有異常時 exit 1，可當閘門用：./dev.sh status || exit 1
                    • Go 服務「Up 但 Air build 失敗」會標出來並給復原指令
    seed            匯入測試資料（Go migration + seed SQL）
    ngrok           啟動 ngrok（如未啟動）並複製 URL 到剪貼簿
    clean           深度清理 Docker 佔用空間
                    • 清 dangling 映像 + 全部 build cache + 已停止容器
                    • 不動跑中的容器、有 tag 的映像、volumes
                    • rebuild 結束會自動跑輕量版（dangling + 24h+ cache）
    help            顯示此說明

  ─── 架構說明 ────────────────────────────────────────────────

    Go 服務（改 code 自動生效，無需任何操作）：
      gateway, seller, social, crm,
      ai, im, review, notification

    單體（content 等已搬進的模組，不在 compose 裡）：
      在本機另外跑 make infra-up、make migrate、make serve（:8080），
      gateway 以 Connect 代理給它（MONOLITH_HOST 預設 host.docker.internal）

    Java 服務（改完需 ./dev.sh rebuild）：
      payment, media, product, order, cart, coupon,
      shipping, aftersales, payuni-webhook, product-import

    基礎設施：
      postgres (5432), redis (6379), rabbitmq (5672/15672),
      otel-collector (4317/4318)

HELP
}

# ─── 主程式 ──────────────────────────────────────────────────────────────────
main() {
  # 切到專案根目錄
  cd "$(dirname "$0")"

  # Go 服務 dev image 的版本只以 .go-version 為準，compose 的 build args 讀這個變數。
  export GO_VERSION
  GO_VERSION="$(cat .go-version)"

  # 個人本地差異覆蓋檔（docker-compose.override.yml，見 .example 範本）存在時自動疊加，
  # base 之後套用故同一欄位以 override 為準；不存在則行為完全不變。
  if [[ -f "docker-compose.override.yml" ]]; then
    COMPOSE="$COMPOSE -f docker-compose.override.yml"
  fi

  local cmd="${1:-help}"
  shift 2>/dev/null || true

  case "$cmd" in
    up)       cmd_up "$@" ;;
    down)     cmd_down "$@" ;;
    rebuild)  cmd_rebuild "$@" ;;
    logs)     cmd_logs "$@" ;;
    status)   cmd_status "$@" ;;
    seed)     cmd_seed "$@" ;;
    restart)  cmd_restart "$@" ;;
    ngrok)    cmd_ngrok "$@" ;;
    clean)    cmd_clean "$@" ;;
    help|-h|--help) cmd_help ;;
    *)
      err "未知指令：$cmd"
      cmd_help
      exit 1
      ;;
  esac
}

main "$@"
