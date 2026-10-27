# Prod RabbitMQ 3-Node Cluster

> 現況（作品集轉 public 步驟 2，D7）：公司時期的部署環境設定已刪除——`infra/mq/envs/{dev,prod}`、`mq-topology.yml` 等部署 workflow、`monitoring/` 的 prod exporter 與 `prod-infra.json`、image-gateway 的 dev／production 環境與 custom domain、website／admin 的 `wrangler.jsonc` env 區塊。下文提到這些的部署、plan／apply 與環境操作段落是公司時期的歷史紀錄，不是現行流程；部署設定等選定 demo 部署目標時重寫。

## 背景

Prod RabbitMQ 原本單機跑在 Prod-infra VM（192.0.2.115），已標記 🔴 SPOF。
該台已整台停機除役（Zeabur `vmStatus: STOPPED`）。
2026-07-21 起改為 Prod MQ1/2/3 三節點 cluster。設計理由見
`docs/superpowers/specs/2026-07-20-prod-rabbitmq-cluster-design.md`
（gitignored，僅本機）。

## 節點

| 節點 | 私網 IP | 角色 |
|---|---|---|
| Prod MQ1 | 198.51.100.98 | cluster node |
| Prod MQ2 | 198.51.100.99 | cluster node |
| Prod MQ3 | 198.51.100.100 | cluster node |

同 VPC `172.17.0.0/18`，跟 Prod-backend（172.17.0.97）/Prod-infra（192.0.2.115）
私網互通。SSH 連線資訊不放本文件，向維運者索取。

## Bootstrap（首次建 cluster）

在**每一台** Prod MQ1/2/3 上執行：

1. 安裝 Docker：
   ```bash
   curl -fsSL https://get.docker.com | sh
   ```
2. `scp` `infra/rabbitmq-cluster/{docker-compose.yml,rabbitmq.conf}` 到 VM 的 `/root/rabbitmq-cluster/`
3. 在 VM 上建立 `.env`（三台 `RABBITMQ_ERLANG_COOKIE` 必須完全相同，`NODE_IP` 各自不同）：
   ```bash
   cat > /root/rabbitmq-cluster/.env <<'EOF'
   NODE_IP=<這台自己的私網 IP>
   RABBITMQ_DEFAULT_USER=guest
   RABBITMQ_DEFAULT_PASS=guest
   RABBITMQ_ERLANG_COOKIE=<三台共用，openssl rand -hex 20 產生一次>
   EOF
   ```
4. 起容器：
   ```bash
   cd /root/rabbitmq-cluster && docker compose up -d
   ```
5. iptables 私網鎖定（比照 Prod-infra 既有模式，DOCKER-USER + INPUT chain，僅放行
   `172.17.0.0/18` + loopback，涵蓋 4369/5672/15672/25672/15692）。⚠️ 這些規則是
   runtime 狀態、未持久化，VM 重開機會消失（跟 Prod-infra 既有的已知 gap 一樣，見
   `docs/基礎建設/Tailscale VPN 連線 Prod 機器指南.md` 風險表）：
   ```bash
   for PORT in 4369 5672 15672 25672 15692; do
     iptables -I DOCKER-USER -p tcp --dport $PORT -j DROP
     iptables -I DOCKER-USER -p tcp --dport $PORT -s 172.17.0.0/18 -j ACCEPT
     iptables -I DOCKER-USER -p tcp --dport $PORT -s 127.0.0.0/8 -j ACCEPT
     iptables -I INPUT -p tcp --dport $PORT -j DROP
     iptables -I INPUT -p tcp --dport $PORT -s 172.17.0.0/18 -j ACCEPT
     iptables -I INPUT -p tcp --dport $PORT -s 127.0.0.0/8 -j ACCEPT
   done
   ```
   兩個地雷，2026-07-21 bootstrap 時實測都踩到過，指令順序已經是修正後版本：
   - **`-I` 插入順序**：`-I` 預設插到 chain 最前面，同一個 port 內先插 DROP、
     後插 ACCEPT，ACCEPT 才會排在 DROP 前面生效。順序反過來（先插 ACCEPT
     後插 DROP）會讓 DROP 排到最前面，連合法私網流量都被擋。
   - **loopback 一定要另外放行**：RabbitMQ 開機時的 `duplicate_node_check`
     prelaunch 檢查會經由 `127.0.0.1` 自連 epmd，這個來源不在
     `172.17.0.0/18` 網段內。沒放行的話，節點會卡在 `rabbit_prelaunch_dist`
     的 `nodistribution`/`register/listen error: etimedout`，且完全不噴
     log、`epmd -names` 永遠是空的、`rabbitmqctl` 系列指令全部 hang 到逾時
     ——很容易誤判成「還在開機中」，實際上是永久卡死，重開容器只會重複同樣
     的錯誤。
6. 驗證單一節點健康：`docker exec hawkr-rabbitmq-cluster rabbitmq-diagnostics check_running`

**全部 3 台跑完上面 6 步後**，`classic_config` peer discovery 應該已經自動把 3 個節點
join 成一個 cluster（新節點的 mnesia 是空的，符合 discovery 自動 join 的條件，
理論上不用手動 `rabbitmqctl join_cluster`）。在任一台驗證：

```bash
docker exec hawkr-rabbitmq-cluster rabbitmqctl cluster_status
```

Expected：`Running Nodes` 底下看到 3 個 `rabbit@172.17.1.9x` 都在，無
`Partitions` 段（沒有 partition）。

若沒有自動 join（mnesia 非空、或 peer discovery 沒觸發），手動在 MQ2/MQ3 上跑：

```bash
docker exec hawkr-rabbitmq-cluster rabbitmqctl stop_app
docker exec hawkr-rabbitmq-cluster rabbitmqctl join_cluster rabbit@198.51.100.98
docker exec hawkr-rabbitmq-cluster rabbitmqctl start_app
```

## Monitoring

SSH 到 Monitoring VM（203.0.113.10），編輯 `/etc/prometheus/targets/prod-infra.json`
（file_sd 熱載入，不用重啟 Prometheus），比照
`monitoring/prometheus/targets/prod-infra.json.example` 的新格式加入 3 個
`rabbitmq` target（`node: mq1/mq2/mq3` label）。在 Grafana 確認 3 個節點都有
metrics 進來、`rabbitmq_build_info` 3 個 series 都在。

⚠️ **以下保留期注意事項已於 2026-08-01 失效**（舊 broker 已刪除）：live 的
`/etc/prometheus/targets/prod-infra.json` 現在應與 `.example` 一致——rabbitmq 只有新
cluster 3 個 target，Prod-infra 那台只剩 `:9187`／`:9399`／`:9100` 三個 exporter。
以下段落保留當歷史脈絡：

⚠️ 加新的 3 個 target 時**不要移除**舊 Prod-infra 的
`<PROD_INFRA_IP>:15692` rabbitmq target——`.example` 範本已改成只列新
cluster 的 3 個（範本只示範最終穩態格式），但 Cutover 後 7 天觀察期舊
broker 原樣保留當 rollback 備援（見下方 Cutover 步驟 6），這段期間舊
broker 若不再被 scrape 就完全沒有健康度可見度，一旦真的要 rollback 會
不知道退回去的目標是否健康。所以 live 的 `/etc/prometheus/targets/prod-infra.json`
在這 7 天內應該是「舊 1 個 + 新 3 個」共 4 個 rabbitmq target，等舊
Prod-infra RabbitMQ 真正除役後才移除舊的那個。

## Cutover（短維護窗）

> ✅ **已於 2026-07-26 執行完成**。21/21 服務遷移至 3-node cluster：新 cluster **100 consumers**
> （= 基線 98 + 2 個 per-pod 的 `gateway.sse` 臨時 queue）、舊 broker **0 consumers 0 訊息堆積**。
> 舊 Prod-infra 單機 broker 原訂保留當 rollback 備援至 2026-08-02，**實際已於 2026-08-01 提前刪除**
> （見下方「Rollback（⚠️ 已失效）」）；Prod-infra 現在只剩監控 exporter，不再有 broker。
> 執行中踩到一個坑並已修：`media-service` / `product-import-service` 是全 21 服務中唯二有
> **egress NetworkPolicy** 的，5672 規則的 ipBlock 只列了舊 broker IP，切換後新 pod 一律
> `Connection refused`（PR #1824 補上三個節點 IP）——所以下方前置條件多了那一項檢查。
>
> 以下步驟保留供 **rollback 或未來同類遷移**使用。

前置：
- Task 1-4 的 PR 皆已 merge 並部署到 prod（`Publisher` 重連邏輯已上線）
- **passive declare 已在 prod 生效**（2026-07-25 完成，PR #1814 → promote #1815，
  21 服務全數部署並驗證：consumer 98→98 零減少、log 零 passive 錯誤）。
  `common.yaml` 的 `MQ_PASSIVE_DECLARE=true` + `java-common.yaml` 的
  `SPRING_RABBITMQ_DYNAMIC=false` 讓服務**完全不宣告 topology**，只做
  passive 驗存在。
- 新 cluster 的 topology 由 `infra/mq/` Terraform 佈建完成，且 nightly drift
  連續乾淨
- **egress NetworkPolicy 的 ipBlock 已含新 cluster 三個 IP**：
  ```bash
  kubectl -n hawkr get networkpolicy | grep egress
  # 逐一確認 5672 那條 to[].ipBlock 含 198.51.100.98/99/100
  ```
  2026-07-25 cutover 就是漏了這項——只有 `media-service` 與 `product-import-service`
  有 egress policy，白名單僅 `192.0.2.115/32`（舊 broker），切換後這兩個服務的新 pod
  一律 `Connection refused`、卡在 CrashLoopBackOff（舊 pod 因 rolling update 保護仍在
  服務，所以不會立刻發現）。**NetworkPolicy 比對的是封包實際目的地（Endpoints 後端 IP），
  不是 Service VIP**，所以換 broker IP 一定要同步這裡。定義在
  `k8s/external-services/networkpolicy.yaml`。

⚠️ **不要加 `MQ_QUEUE_TYPE=quorum`**。這個 env-gate 是 passive 之前的設計：當時
服務會 active declare，所以必須讓 app 宣告的 args 與 Terraform 建的逐 byte 對齊，
才不會對既有 queue 觸發 406 PRECONDITION_FAILED。passive 生效後服務根本不宣告
queue，這個變數對 `pkg/mq.ResilientConsumer` 已完全沒有作用，加了只會製造「還有
一條 active declare 路徑」的錯覺。維護窗因此只剩「切 Endpoints + 重啟」一個變因。

1. 挑低峰時段公告維護窗（預估 < 15 分鐘）
2. 暫停 producer（或直接執行下一步，讓現有連線斷線後排空——outbox 事件本來就
   容忍 MQ 短暫不可用）
3. 執行 Endpoints 切換：
   ```bash
   RABBITMQ_IPS="198.51.100.98,198.51.100.99,198.51.100.100" \
     bash k8s/scripts/init-environment.sh prod <既有 prod infra-ip>
   ```
4. **緊接著 rolling restart**。腳本是 `kubectl patch endpoints`，既有 TCP 連線
   不會被切斷——不重啟 pod 的話服務會繼續用舊連線打舊 broker。不需要改任何
   config（passive 已生效），所以走 `kubectl rollout restart` 即可，不必經
   deploy workflow 重 build image：
   ```bash
   # 第一批：Go 服務（啟動快）
   kubectl -n hawkr rollout restart deployment/gateway deployment/account-service \
     deployment/ai-service deployment/buyer-service deployment/content-service \
     deployment/crm-service deployment/im-service deployment/notification-service \
     deployment/review-service deployment/seller-service deployment/social-service
   # 第二批：Java 服務（啟動需 115-133 秒，等 Go 就緒再推，避免搶 CPU 拖長啟動）
   kubectl -n hawkr rollout restart deployment/aftersales-service \
     deployment/cart-service deployment/coupon-service deployment/media-service \
     deployment/order-service deployment/payment-service deployment/payuni-webhook-service \
     deployment/product-import-service deployment/product-service deployment/shipping-service
   ```
5. 驗證：
   - `kubectl get endpoints rabbitmq -n hawkr` 顯示 3 個 IP
   - **consumer 數比對**（passive 下最直接的健康訊號：任何一條訂閱 passive
     check 404 就起不來，consumer 會少）。切換前先對舊 broker 抓基線，重啟後
     對新 cluster 比對：
     ```bash
     curl -s -u <user>:<pass> "http://<broker>:15672/api/queues/%2F?columns=name,consumers"
     ```
     ⚠️ 比 **consumer 總數**（2026-07-25 基線為 98），不要比 queue 總數——舊
     broker 上有 128 條 notification 粒度重構前的孤兒 queue（0 訊息 0 consumer，
     不在 `topology.json` 內），新 cluster 沒有它們，queue 數從 287 掉到 159
     是預期的，不是缺漏。
   - Grafana 看到 3 個 RabbitMQ 節點都有真實流量（不是只有 metrics 心跳）
   - 抽查一個關鍵事件（如 `order.paid`）走完整發布/消費流程（用既有
     `live-wire-verify` skill 手法）
   - 舊 broker 上 consumer 歸零、且無訊息堆積（確認流量真的切走了；還有殘留
     表示某些 pod 沒重啟成功）
   - `rabbitmqctl list_queues name type` 確認 queue 是 `quorum` 類型（這些是
     Terraform 預先建好的，不是服務啟動時宣告出來的）
   - ~~確認 `MaxRetries==0` 的 legacy queue 在 quorum queue 上
     `Nack(requeue=true)` 的行為跟 classic queue 一致，沒有意外的
     delivery-limit 導致訊息被丟棄~~ **已於 2026-07-22 在 Prod MQ1 手動預先驗證**：
     建一個跟 app 宣告方式相同的 quorum test queue（只帶 `x-queue-type`，
     不帶 `x-delivery-limit`），模擬 `Nack(requeue=true)` 連續 25 輪，
     `x-delivery-count` 一路累加到 49 訊息仍未被丟棄/DLX，確認 quorum
     queue 未設 `x-delivery-limit` 時預設是 unlimited，行為跟 classic
     queue 一致。此項免在維護窗內驗證。
6. 觀察 7 天，舊 Prod-infra RabbitMQ 原樣保留當 rollback 備援（**已於 2026-08-01
   提前結束**，見下方「Rollback（已失效）」）

## Rollback（⚠️ 已失效，2026-08-01）

**舊 Prod-infra VM（192.0.2.115）上的 RabbitMQ container 已於 2026-08-01 刪除**
（同時間 Redis 也遷至獨立主機，見 `系統架構與資料流向.md`）。**該台機器本身已停機除役**，
NetworkPolicy 對 `192.0.2.115/32` 的 egress 放行也已移除 —— 即使照下面的步驟切回
Endpoints，封包也會被 NetworkPolicy 擋掉。原本觀察期到
2026-08-02，Will 確認風險後提前終止，理由是需要一併整理該台機器。

下面是**已失效**的原始 rollback 步驟，保留當歷史記錄——**不要照著執行**：
若現在對 192.0.2.115 跑第 1 步的 `init-environment.sh`，會把 `rabbitmq`
Endpoints 指向一台完全沒有 broker 在監聽 5672 的機器，等同讓 RabbitMQ 全滅，
比不 rollback 更糟。

若 3-node cluster 未來真的需要 rollback 到單機，必須先在某台 VM **重新從零
bootstrap** 一個單節點 broker（image + config 見本文件「背景」），確認涵蓋
passive 檢查清單零缺口後才能切
Endpoints——不再是「切回去」這麼單純。

<details>
<summary>原始步驟（僅供歷史參考，已失效）</summary>

若新 cluster 出問題，退回舊單機。passive 生效後不再有「先拔 `MQ_QUEUE_TYPE`
再切 Endpoints」的順序約束（那個變數已不使用），rollback 就是切回去 + 重啟：

1. 切回 Endpoints：
   ```bash
   RABBITMQ_IPS="" bash k8s/scripts/init-environment.sh prod 192.0.2.115
   ```
   （`RABBITMQ_IPS` 留空會 fallback 回單一 `infra-ip` 邏輯，等同切換前的狀態）
2. 同 Cutover 步驟 4 的 rolling restart（分兩批）。

⚠️ **passive 給 rollback 加了一個新前提**：服務不宣告 topology，所以退回去的
舊 broker 必須**仍然涵蓋 passive 檢查清單**（每條訂閱的 queue + `MaxRetries>0`
的 `*.retry` exchange），缺一條該訂閱就 404 起不來。2026-07-25 切 passive 前
已核對過舊 broker 零缺口，但那是當時的快照——cutover 之後 Terraform 若對新
cluster 新增 queue，舊 broker 不會跟著長出來。因此：

- 觀察期內盡量凍結 topology 變更；真的要加，記得舊 broker 也補一份
- rollback 前先跑一次核對：先展開成攤平清單
  `python3 infra/mq/tools/expand.py --topology infra/mq/topology/topology.yaml --emit-expanded /tmp/topology-expanded.json`，
  再比對該清單的 queue + `*.retry` exchange 是否都存在於舊 broker 的
  `/api/queues/%2F`、`/api/exchanges/%2F`，確認零缺口再切

</details>

## 已知缺口 / 後續

- ~~Java 服務（~10 個）的 queue 仍是 classic type，跨節點無 HA，待後續 plan~~
  **已由 Terraform 化解決**：新 cluster 的 topology 全部由 `infra/mq/` 佈建，
  `envs/prod` 的 `queue_type = "quorum"` 套用到 `topology.json` 的每一條 queue，
  其中含 Java 服務的 24 條（`dlq.*`、`product.import.*`、`media.import.*` 等）。
  cutover 之後 Java 的 queue 一樣是 quorum、跨節點有 HA，不需要另外的 plan。
- iptables 規則未持久化，VM 重開機需重新套用（跟 Prod-infra 既有 gap 一致，
  待評估搬到 Zeabur 雲端防火牆 API 或裝 iptables-persistent）
- ~~舊 Prod-infra RabbitMQ 除役時機留待觀察期後另行決策~~ **已於 2026-08-01 除役**
  （container 已刪除，見上方「Rollback（已失效）」）
