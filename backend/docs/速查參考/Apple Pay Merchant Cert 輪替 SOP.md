# Apple Pay Merchant Cert 輪替 SOP

> Apple Pay Merchant Identity Certificate 由 Apple Developer 後台簽發、有效期 **25 個月**。
> 過期前必須輪替，否則 mTLS handshake 失敗、`/api/v1/app/payments/apple_pay/validate_merchant` 回 `8030 APPLE_PAY_MERCHANT_VALIDATION_FAILED`。
>
> 對應設計：[05-支付服務設計書 §5.11](../詳細設計書/05-支付服務（Payment%20Service）詳細設計書.md#511-apple-pay-web-merchant-validation-流程)

## 適用情境

- **定期輪替**：cert 即將到期前（建議 2 週前處理）
- **應急輪替**：cert 已過期、private key 遺失、cert 被洩漏
- **首次建立**：新環境（如 prod）首次部署 Apple Pay

## dev / sandbox vs prod 差異

| 項目 | dev / sandbox | prod |
|---|---|---|
| Apple Merchant ID | `merchant.io.github.will413028.hawkr.sandbox` | `merchant.io.github.will413028.hawkr` |
| Apple Domain（須 verify） | `web-dev.example.com` | `example.com` |
| K8s Secret 名稱 | `hawkr-apple-pay`（namespace=hawkr） | `hawkr-apple-pay`（namespace=hawkr） |
| 後端 SSH 目標 | Dev VM `203.0.113.20` | prod VM（依當時部署環境） |

兩環境**步驟相同**，只是 merchant ID / domain / VM target 不同。

## SOP

### Phase 1：生成新 CSR + Key（本機）

```bash
# 1. 切到本機憑證目錄（避免污染他處）
cd ~/Documents/hawkr-certs/<env>/apple-pay   # env=dev 或 prod

# 2. 備份舊版（萬一回滾需要）
mkdir -p archive/$(date +%Y%m%d)
mv merchant_id.cer "merchant_id.cer(apply-pay-web).pem" merchant_id.key merchant_identity.csr \
   archive/$(date +%Y%m%d)/  2>/dev/null || true

# 3. 產 RSA 2048 PKCS#8 private key（Apple 規定 RSA 2048）
openssl genrsa -out merchant_id.key 2048

# 4. 轉 PKCS#8（JDK 不支援 PKCS#1，避免 §5.11.5 PKCS#1 friendly fail）
openssl pkcs8 -topk8 -nocrypt -in merchant_id.key -out merchant_id.pkcs8.key
mv merchant_id.pkcs8.key merchant_id.key

# 5. 產 CSR
openssl req -new -key merchant_id.key -out merchant_identity.csr \
  -subj "/UID=<MERCHANT_ID>/CN=Apple Pay Merchant Identity:<MERCHANT_ID>/OU=<TEAM_ID>/O=<COMPANY>"
# 範例：UID=merchant.io.github.will413028.hawkr.sandbox CN=Apple Pay Merchant Identity:merchant.io.github.will413028.hawkr.sandbox OU=V2H73DN9FP O="Hawkr Co. Ltd"
```

### Phase 2：Apple Developer 後台簽發

1. 登入 [Apple Developer Account](https://developer.apple.com/account/)
2. **Identifiers → Merchant IDs** → 點對應 Merchant
3. **Apple Pay Payment Processing Certificate** 區段（**不是** Payment Authentication）：
   - 若有舊 cert：點 **Revoke**（dev / sandbox 沒在用的話可直接 revoke；prod 要先 stage 新舊重疊期）
   - 點 **Create Certificate** → 上傳 `merchant_identity.csr` → **Download** 拿到 `.cer` (DER 格式)
4. 在本機把 `.cer` 轉 PEM：
   ```bash
   openssl x509 -inform DER -in merchant_id.cer -out merchant_id.pem
   ```
5. 立即驗證 cert + key 配對：
   ```bash
   diff <(openssl x509 -in merchant_id.pem -noout -modulus) \
        <(openssl rsa  -in merchant_id.key -noout -modulus)
   # 必須沒有 diff 輸出（兩個 modulus 相同）
   ```

### Phase 3：建 / 更新 K8s Secret（ssh stdin pipe，VM 磁碟無殘留）

> dev 用 SSH `root@203.0.113.20`（走已授權的金鑰；憑證向維運者索取）
> prod 路徑同此，target VM 不同。

```bash
CERT=~/Documents/hawkr-certs/dev/apple-pay/merchant_id.pem
KEY=~/Documents/hawkr-certs/dev/apple-pay/merchant_id.key

# 本機建 manifest（base64 編進 stdin）→ ssh pipe → kubectl apply --force（覆蓋舊 secret）
kubectl create secret generic hawkr-apple-pay \
  --namespace hawkr \
  --from-file=merchant_id.pem="$CERT" \
  --from-file=merchant_id.key="$KEY" \
  --dry-run=client -o yaml \
| ssh root@203.0.113.20 \
  'kubectl apply -f -'

# 驗證 secret 內容（不會印出 cert/key 內容、只看 keys + size）
ssh root@203.0.113.20 \
  'kubectl get secret hawkr-apple-pay -n hawkr -o jsonpath="{.data}" | jq "to_entries | map({key, len: (.value|length)})"'
# 應看到 merchant_id.pem ~2928 bytes、merchant_id.key ~2272 bytes (base64 encoded)
```

### Phase 4：Domain Association 重驗（如有換 domain 或首次設定）

> Apple Developer 後台會要求對應 domain 提供 `apple-developer-merchantid-domain-association` 檔，
> 此檔由 **hawkr-website 前端** 負責 host（不在本 repo）。

1. Apple Developer 後台 → Merchant IDs → 該 merchant → **Merchant Domains**
2. 找對應 domain → **Download Domain Association File**
3. 把檔案內容（一段 5KB base64 純文字）給前端團隊，請他們更新 `hawkr-website` 的 `APPLE_PAY_DOMAIN_ASSOCIATION` 並**重跑一次部署**
4. ⚠️ **web 部署位置＝Cloudflare Workers，不是 VM**（2026-08-04 覆核；網頁已於 2026-07-01/02 cutover，Infra VM `203.0.113.10` 上**已無 website 容器、無 `/root/hawkr-website*` 目錄**）。~~舊 SOP 寫「SSH 上 Infra VM `docker compose --env-file .env.dev` 重啟」~~ **已不可行**——生效路徑是改該值後由 `hawkr-website/.github/workflows/deploy.yml` 的 `deploy-cloudflare` job 重新部署（值本身走 wrangler secret / GitHub secret）。
5. 確認：
   ```bash
   curl https://web-dev.example.com/.well-known/apple-developer-merchantid-domain-association
   # 應回 HTTP 200 + 約 5KB base64
   ```
6. Apple 後台對該 domain 點 **Verify** → 變綠 ✓

### Phase 5：rollout payment-service

```bash
# 觸發 Pod 重建，掛載新 secret
ssh root@203.0.113.20 \
  'kubectl rollout restart deployment/payment-service -n hawkr && kubectl rollout status deployment/payment-service -n hawkr --timeout=180s'
```

### Phase 6：驗證

```bash
# 1. 為買家建一個 dev session 並取 token（seed 的帳號 ID 見 backend/infra/seed-data/01-account.sql）
TOKEN=$(make -s dev-token args='-account <帳號 ID>')

# 2. 真打 dev URL（cert/key 配對正確時應拿到 merchantSession 含 PKCS#7 簽章）
curl -X POST https://test-api.example.com/api/v1/app/payments/apple_pay/validate_merchant \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"validationURL":"https://apple-pay-gateway-cert.apple.com/paymentservices/startSession","domainName":"web-dev.example.com","displayName":"Hawkr"}'
# 預期：{"code":0, "data":{"merchantSessionJson":"{...merchantSessionIdentifier, signature, ...}"}}

# 3. 跑 dev 結帳 smoke 確認既有付款流程未破壞
./infra/smoke-checkout.sh --env=dev
```

## 常見問題

### Q1：cert/key modulus 不配對怎麼辦

**Root cause**：cert 與 private key 是同一輪 CSR 申請的；如果你拿到的 cert 是 PayUni 或別人申請的，private key 不在你手上。
**修法**：路徑 B Phase 1-2 重新申請（會 revoke 舊 cert）。

### Q2：PKCS#1 RSA key not supported

**Root cause**：openssl 預設輸出 PKCS#1（檔頭 `BEGIN RSA PRIVATE KEY`），但 JDK 內建 KeyFactory 只認 PKCS#8（`BEGIN PRIVATE KEY`）。
**修法**：`openssl pkcs8 -topk8 -nocrypt -in old.key -out new.key`，覆蓋 secret 後 rollout。

### Q3：domain 沒 verify（後台顯示 Pending）

**Root cause**：web 服務沒 serve 正確的 association 檔。
**修法**：Phase 4 重做；確認 `curl <web-domain>/.well-known/apple-developer-merchantid-domain-association` HTTP 200。

### Q4：SSLHandshakeException - decrypt_error

**Root cause**：90% 是 cert/key 不配對；10% 是 cert 過期或 Apple 撤銷。
**修法**：先做 Q1 modulus 比對。

### Q5：本機 cert 遺失（電腦壞 / 重灌）

**Root cause**：沒備份。
**避免**：cert/key/csr 三件同時備份至 1Password / Bitwarden（附檔功能）+ 雲端加密副本（age + iCloud Drive）。
