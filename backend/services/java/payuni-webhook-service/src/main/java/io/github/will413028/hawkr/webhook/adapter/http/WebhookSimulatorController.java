package io.github.will413028.hawkr.webhook.adapter.http;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.shipping.grpc.LookupShipTradeNoRequest;
import io.github.will413028.hawkr.shipping.grpc.LookupShipTradeNoResponse;
import io.github.will413028.hawkr.shipping.grpc.ShippingServiceGrpc;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.HttpEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.client.RestTemplate;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;

/**
 * PayUni Webhook 模擬器
 *
 * 提供 Web UI 讓測試人員手動推進訂單狀態（付款/退款/物流）。
 * 使用既有 PayUniCryptoService 加密後 POST 到目標 webhook endpoint，
 * 走完整 pipeline（解密 → inbox → outbox → MQ → consumer）。
 *
 * 安全防護：僅 APP_ENV=dev/local 環境允許存取，否則回傳 403。
 */
@RestController
@RequestMapping("/simulator")
public class WebhookSimulatorController {

    private static final ObsLogger log = ObsLogger.forClass(WebhookSimulatorController.class);
    private static final Set<String> ALLOWED_ENVS = Set.of("dev", "local");

    private final PayUniCryptoService cryptoService;
    private final GrpcStubFactory grpcStubFactory;
    private final RestTemplate restTemplate;
    private final boolean enabled;

    public WebhookSimulatorController(
            PayUniCryptoService cryptoService,
            GrpcStubFactory grpcStubFactory,
            @Value("${app.env:production}") String appEnv) {
        this.cryptoService = cryptoService;
        this.grpcStubFactory = grpcStubFactory;
        this.restTemplate = new RestTemplate();
        this.enabled = ALLOWED_ENVS.contains(appEnv.toLowerCase());
    }

    @PostConstruct
    public void init() {
        if (enabled) {
            log.info("WebhookSimulatorController.init", "Webhook 模擬器已啟用");
        } else {
            log.info("WebhookSimulatorController.init", "Webhook 模擬器已停用（非允許環境）");
        }
    }

    /**
     * 渲染模擬器 HTML 頁面
     */
    @GetMapping(produces = MediaType.TEXT_HTML_VALUE)
    public ResponseEntity<String> page() {
        if (!enabled) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN)
                    .body("此工具僅限 dev/local 環境使用");
        }
        return ResponseEntity.ok(SIMULATOR_HTML);
    }

    /**
     * 依訂單編號查詢 ShipTradeNo（透過 shipping-service gRPC）
     */
    @GetMapping(value = "/lookup", produces = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<Map<String, Object>> lookup(
            @RequestParam("orderNo") String orderNo,
            @RequestParam(value = "isReturn", defaultValue = "false") boolean isReturn) {
        if (!enabled) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN)
                    .body(Map.of("success", false, "message", "此工具僅限 dev/local 環境使用"));
        }

        try {
            ShippingServiceGrpc.ShippingServiceBlockingStub stub =
                    grpcStubFactory.blocking("shipping", ShippingServiceGrpc::newBlockingStub);

            LookupShipTradeNoResponse resp = stub.lookupShipTradeNo(
                    LookupShipTradeNoRequest.newBuilder()
                            .setOrderNo(orderNo)
                            .setIsReturn(isReturn)
                            .build());

            if (resp.getCode() == 0) {
                return ResponseEntity.ok(Map.of(
                        "success", true,
                        "shipTradeNo", resp.getShipTradeNo(),
                        "lgsType", resp.getLgsType(),
                        "shipType", resp.getShipType(),
                        "merId", resp.getMerId()
                ));
            } else {
                return ResponseEntity.ok(Map.of(
                        "success", false,
                        "message", resp.getMessage()
                ));
            }
        } catch (Exception e) {
            log.error("WebhookSimulatorController.lookup.failed", "查詢 ShipTradeNo 失敗", e);
            return ResponseEntity.ok(Map.of("success", false, "message", "查詢失敗: " + e.getMessage()));
        }
    }

    /**
     * 接收模擬參數 → 加密 → POST 到目標 Webhook URL → 回傳結果
     */
    @PostMapping(value = "/send", consumes = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<Map<String, Object>> send(@RequestBody SimulatorRequest request) {
        if (!enabled) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN)
                    .body(Map.of("success", false, "message", "此工具僅限 dev/local 環境使用"));
        }

        Map<String, Object> result = new LinkedHashMap<>();

        try {
            // 1. 加密 payload
            Map<String, String> params = request.params();
            String encryptInfo = cryptoService.encrypt(params);
            String hashInfo = cryptoService.calculateHash(encryptInfo);

            // 組裝加密前原始 payload（debug 用）
            TreeMap<String, String> sorted = new TreeMap<>(params);
            StringBuilder raw = new StringBuilder();
            sorted.forEach((k, v) -> {
                if (!raw.isEmpty()) raw.append("\n");
                raw.append(k).append("=").append(v);
            });
            result.put("rawPayload", raw.toString());

            // 2. POST 到目標 URL（模擬 PayUni form-urlencoded）
            HttpHeaders headers = new HttpHeaders();
            headers.setContentType(MediaType.APPLICATION_FORM_URLENCODED);

            MultiValueMap<String, String> formData = new LinkedMultiValueMap<>();
            formData.add("MerID", request.merId());
            formData.add("Version", "1.0");
            formData.add("EncryptInfo", encryptInfo);
            formData.add("HashInfo", hashInfo);

            HttpEntity<MultiValueMap<String, String>> httpEntity = new HttpEntity<>(formData, headers);

            ResponseEntity<String> response = restTemplate.postForEntity(
                    request.targetUrl(), httpEntity, String.class);

            result.put("success", true);
            result.put("webhookStatusCode", response.getStatusCode().value());
            result.put("webhookResponse", response.getBody());

            log.info("WebhookSimulatorController.send.success",
                    "模擬 Webhook 發送成功: " + request.eventType());

        } catch (Exception e) {
            result.put("success", false);
            result.put("webhookStatusCode", 0);
            result.put("webhookResponse", "發送失敗: " + e.getMessage());

            log.error("WebhookSimulatorController.send.failed", "模擬 Webhook 發送失敗", e);
        }

        return ResponseEntity.ok(result);
    }

    /**
     * 模擬器請求 DTO
     */
    record SimulatorRequest(
            String targetUrl,
            String merId,
            String eventType,
            Map<String, String> params
    ) {}

    // ==================== HTML 頁面 ====================

    private static final String SIMULATOR_HTML = """
<!DOCTYPE html>
<html lang="zh-TW">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PayUni Webhook 模擬器</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f7fa; color: #333; padding: 20px; }
        .container { max-width: 720px; margin: 0 auto; }
        h1 { text-align: center; margin-bottom: 8px; color: #1a1a2e; font-size: 24px; }
        .subtitle { text-align: center; color: #888; margin-bottom: 24px; font-size: 14px; }
        .config-section { background: #fff; border-radius: 8px; padding: 20px; margin-bottom: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .config-section h3 { margin-bottom: 12px; font-size: 15px; color: #555; }
        .form-row { display: flex; gap: 12px; margin-bottom: 12px; align-items: center; }
        .form-row label { min-width: 100px; font-size: 14px; font-weight: 500; color: #555; }
        .form-row input, .form-row select { flex: 1; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px; }
        .form-row input:focus, .form-row select:focus { outline: none; border-color: #4a90d9; box-shadow: 0 0 0 2px rgba(74,144,217,0.2); }
        .form-row input:read-only { background: #f0f0f0; color: #666; }
        #customUrlRow { display: none; }
        .tabs { display: flex; gap: 0; margin-bottom: 0; }
        .tab-btn { flex: 1; padding: 12px; text-align: center; cursor: pointer; background: #e8ecef; border: none; font-size: 14px; font-weight: 500; color: #666; transition: all 0.2s; }
        .tab-btn:first-child { border-radius: 8px 0 0 0; }
        .tab-btn:last-child { border-radius: 0 8px 0 0; }
        .tab-btn.active { background: #fff; color: #1a1a2e; font-weight: 600; }
        .tab-btn:hover:not(.active) { background: #dce1e6; }
        .tab-content { display: none; background: #fff; padding: 20px; border-radius: 0 0 8px 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .tab-content.active { display: block; }
        .radio-group { display: flex; gap: 16px; margin-bottom: 12px; }
        .radio-group label { min-width: auto; display: flex; align-items: center; gap: 4px; cursor: pointer; font-size: 14px; }
        .submit-row { margin-top: 16px; text-align: center; }
        .submit-btn { padding: 12px 48px; background: #4a90d9; color: #fff; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; transition: background 0.2s; }
        .submit-btn:hover { background: #3a7bc8; }
        .submit-btn:disabled { background: #aaa; cursor: not-allowed; }
        .lookup-btn { padding: 8px 16px; background: #6c757d; color: #fff; border: none; border-radius: 6px; font-size: 13px; cursor: pointer; white-space: nowrap; }
        .lookup-btn:hover { background: #5a6268; }
        .lookup-msg { font-size: 12px; margin-top: -8px; margin-bottom: 8px; padding-left: 112px; }
        .lookup-msg.ok { color: #2e7d32; }
        .lookup-msg.err { color: #c62828; }
        .result-section { margin-top: 16px; background: #fff; border-radius: 8px; padding: 20px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); display: none; }
        .result-section h3 { margin-bottom: 12px; font-size: 15px; }
        .result-status { padding: 8px 16px; border-radius: 6px; font-weight: 600; display: inline-block; margin-bottom: 12px; }
        .result-status.success { background: #d4edda; color: #155724; }
        .result-status.error { background: #f8d7da; color: #721c24; }
        .result-detail { margin-top: 8px; }
        .result-detail summary { cursor: pointer; font-size: 13px; color: #666; margin-bottom: 4px; }
        .result-detail pre { background: #f8f9fa; padding: 12px; border-radius: 6px; font-size: 12px; overflow-x: auto; white-space: pre-wrap; word-break: break-all; max-height: 300px; overflow-y: auto; }
        .history-section { margin-top: 16px; }
        .history-section h3 { font-size: 15px; color: #555; margin-bottom: 8px; }
        .history-item { background: #fff; border-radius: 6px; padding: 10px 14px; margin-bottom: 6px; box-shadow: 0 1px 2px rgba(0,0,0,0.06); display: flex; justify-content: space-between; align-items: center; font-size: 13px; }
        .history-item .tag { padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; }
        .history-item .tag.payment { background: #e3f2fd; color: #1565c0; }
        .history-item .tag.refund { background: #fce4ec; color: #c62828; }
        .history-item .tag.aftersales { background: #fff3e0; color: #ef6c00; }
        .history-item .tag.shipping { background: #e8f5e9; color: #2e7d32; }
        .history-item .time { color: #999; font-size: 11px; }
    </style>
</head>
<body>
<div class="container">
    <h1>PayUni Webhook 模擬器</h1>
    <p class="subtitle">模擬 PayUni 回調通知，推進訂單付款 / 退款 / 物流狀態</p>

    <div class="config-section">
        <h3>基本設定</h3>
        <div class="form-row">
            <label>目標環境</label>
            <select id="envSelect" onchange="onEnvChange()">
                <option value="self">本機 (webhook-service)</option>
                <option value="dev">Dev (test-api.example.com)</option>
                <option value="custom">自訂 URL</option>
            </select>
        </div>
        <div class="form-row" id="customUrlRow">
            <label>Webhook URL</label>
            <input type="text" id="customUrl" placeholder="http://payuni-webhook-service:9946/webhooks/payuni/notify">
        </div>
        <div class="form-row">
            <label>MerID</label>
            <input type="text" id="merIdInput" value="HAWK9" placeholder="商店代號（如 HAWK9）">
        </div>
    </div>

    <div class="tabs">
        <button class="tab-btn active" onclick="switchTab('payment')">付款通知</button>
        <button class="tab-btn" onclick="switchTab('refund')">退款轉匯結果</button>
        <button class="tab-btn" onclick="switchTab('shipping')">物流 / 退貨貨態</button>
    </div>

    <div class="tab-content active" id="tab-payment">
        <div class="form-row"><label>MerTradeNo *</label><input type="text" id="p_merTradeNo" placeholder="系統交易編號（必填）"></div>
        <div class="form-row"><label>TradeNo</label><input type="text" id="p_tradeNo" placeholder="PayUni 交易編號（留空自動生成）"></div>
        <div class="form-row"><label>交易狀態</label>
            <select id="p_tradeStatus" onchange="onTradeStatusChange()">
                <option value="1">1 — 付款成功</option>
                <option value="2">2 — 付款失敗</option>
                <option value="0">0 — 取號成功（處理中）</option>
                <option value="3">3 — 付款取消</option>
            </select>
        </div>
        <div class="form-row"><label>交易金額</label><input type="text" id="p_tradeAmt" value="100" placeholder="新台幣元"></div>
        <div class="form-row"><label>支付方式</label>
            <select id="p_payType">
                <option value="Credit">Credit — 信用卡</option>
                <option value="ATM">ATM — ATM 轉帳</option>
                <option value="ApplePay">ApplePay — Apple Pay</option>
                <option value="GooglePay">GooglePay — Google Pay</option>
            </select>
        </div>
        <div class="form-row"><label>訊息</label><input type="text" id="p_message" value="付款成功"></div>
        <div class="submit-row"><button class="submit-btn" onclick="sendWebhook('payment')">送出付款通知</button></div>
    </div>

    <div class="tab-content" id="tab-refund">
        <div class="form-row"><label>MerTradeNo *</label><input type="text" id="r_merTradeNo" placeholder="原交易的商店訂單編號（必填）"></div>
        <div class="form-row"><label>TradeNo</label><input type="text" id="r_tradeNo" placeholder="PayUni 交易編號"></div>
        <div class="form-row"><label>退款金額</label><input type="text" id="r_refundAmt" value="100" placeholder="新台幣元"></div>
        <div class="form-row"><label>原支付方式</label><select id="r_paymentType" disabled><option value="2">2 — ATM 轉帳</option></select></div>
        <div class="form-row"><label>退款結果</label>
            <select id="r_refundStatus" onchange="onRefundStatusChange()">
                <option value="1">1 — 轉匯成功</option>
                <option value="2">2 — 轉匯失敗</option>
            </select>
        </div>
        <div class="form-row"><label>到帳日</label><input type="text" id="r_bankDate" placeholder="YYYY-MM-DD"></div>
        <div class="submit-row"><button class="submit-btn" onclick="sendWebhook('refund')">送出退款通知</button></div>
    </div>

    <div class="tab-content" id="tab-shipping">
        <div class="radio-group">
            <label><input type="radio" name="shipDirection" value="forward" checked onchange="onShipDirectionChange()"> 正向物流</label>
            <label><input type="radio" name="shipDirection" value="reverse" onchange="onShipDirectionChange()"> 售後退貨物流</label>
        </div>
        <div class="form-row">
            <label>訂單編號 *</label>
            <input type="text" id="s_orderNo" placeholder="輸入訂單編號（如 O400280260）">
            <button class="lookup-btn" onclick="lookupShipTradeNo()">查詢</button>
        </div>
        <div class="lookup-msg" id="lookupMsg"></div>
        <div class="form-row"><label>ShipTradeNo</label><input type="text" id="s_shipTradeNo" readonly placeholder="查詢後自動帶入"></div>
        <div class="form-row"><label>物流狀態</label>
            <select id="s_shipStatus" onchange="onShipStatusChange()">
                <option value="21">21 — 待出貨</option>
                <option value="31" selected>31 — 配送中</option>
                <option value="32">32 — 待取貨（超商）</option>
                <option value="11">11 — 已取貨（完成）</option>
                <option value="33">33 — 異常訂單</option>
                <option value="41">41 — 已取消</option>
            </select>
        </div>
        <div class="form-row"><label>物流類型</label>
            <select id="s_lgsType">
                <option value="HOME">HOME — 宅配（黑貓）</option>
                <option value="C2C">C2C — 超商店到店</option>
            </select>
        </div>
        <div class="form-row"><label>ShipType</label><input type="text" id="s_shipType" value="" placeholder="配送類型（可留空）"></div>
        <div class="form-row"><label>訊息</label><input type="text" id="s_message" value="配送中"></div>
        <div class="submit-row"><button class="submit-btn" onclick="sendWebhook('shipping')">送出物流通知</button></div>
    </div>

    <div class="result-section" id="resultSection">
        <h3>發送結果</h3>
        <div id="resultStatus" class="result-status"></div>
        <div class="result-detail"><details open><summary>回應內容</summary><pre id="resultResponse"></pre></details></div>
        <div class="result-detail"><details><summary>加密前 Payload</summary><pre id="resultPayload"></pre></details></div>
    </div>

    <div class="history-section" id="historySection" style="display:none;">
        <h3>發送歷史</h3>
        <div id="historyList"></div>
    </div>
</div>

<script>
function onEnvChange() {
    const env = document.getElementById('envSelect').value;
    document.getElementById('customUrlRow').style.display = env === 'custom' ? 'flex' : 'none';
}

// 目標 URL：send 由 webhook-service 容器內發出，本機需用 Docker 內部地址
const ENV_URLS = {
    self: 'http://payuni-webhook-service:9946/webhooks/payuni/notify',
    dev: 'https://test-api.example.com/webhook/payuni/notify'
};

function getWebhookUrl() {
    const env = document.getElementById('envSelect').value;
    if (env === 'custom') {
        const url = document.getElementById('customUrl').value.trim();
        if (!url) { alert('請輸入自訂 URL'); return null; }
        return url;
    }
    return ENV_URLS[env];
}

function switchTab(tabName) {
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
    const tabs = ['payment', 'refund', 'shipping'];
    document.querySelectorAll('.tab-btn')[tabs.indexOf(tabName)].classList.add('active');
    document.getElementById('tab-' + tabName).classList.add('active');
}

const tradeStatusMessages = { '0': '取號成功', '1': '付款成功', '2': '付款失敗', '3': '付款取消' };
const forwardShipStatuses = [
    ['21', '21 — 待出貨'],
    ['31', '31 — 配送中'],
    ['32', '32 — 待取貨（超商）'],
    ['11', '11 — 已取貨（完成）'],
    ['33', '33 — 異常訂單'],
    ['41', '41 — 已取消']
];
const returnShipStatuses = [
    ['92', '92 — 退貨處理中'],
    ['31', '31 — 退貨運送中'],
    ['11', '11 — 退貨已取件'],
    ['53', '53 — 退貨已送達'],
    ['33', '33 — 退貨物流異常']
];
const shipStatusMessages = {
    forward: { '21': '待出貨', '31': '配送中', '32': '待取貨', '11': '已取貨', '33': '異常訂單', '41': '已取消' },
    reverse: { '92': '退貨處理中', '31': '退貨運送中', '11': '退貨已取件', '53': '退貨已送達', '33': '退貨物流異常' }
};

function onTradeStatusChange() {
    document.getElementById('p_message').value = tradeStatusMessages[document.getElementById('p_tradeStatus').value] || '';
}
function onRefundStatusChange() {
    const s = document.getElementById('r_refundStatus').value;
    document.getElementById('r_bankDate').value = s === '1' ? new Date().toISOString().split('T')[0] : '-';
}
function currentShipDirection() {
    return document.querySelector('input[name="shipDirection"]:checked').value;
}
function onShipDirectionChange() {
    const direction = currentShipDirection();
    const options = direction === 'reverse' ? returnShipStatuses : forwardShipStatuses;
    const select = document.getElementById('s_shipStatus');
    select.innerHTML = options.map(([value, label]) => '<option value="' + value + '">' + label + '</option>').join('');
    select.value = direction === 'reverse' ? '31' : '31';
    document.getElementById('s_shipTradeNo').value = '';
    document.getElementById('lookupMsg').textContent = '';
    onShipStatusChange();
}
function onShipStatusChange() {
    const direction = currentShipDirection();
    document.getElementById('s_message').value = (shipStatusMessages[direction] || {})[document.getElementById('s_shipStatus').value] || '';
}
function initDefaults() {
    const today = new Date().toISOString().split('T')[0];
    document.getElementById('r_bankDate').value = today;
    onShipDirectionChange();
}

async function lookupShipTradeNo() {
    const orderNo = document.getElementById('s_orderNo').value.trim();
    if (!orderNo) { alert('請輸入訂單編號'); return; }
    const isReturn = currentShipDirection() === 'reverse';
    const msgEl = document.getElementById('lookupMsg');
    msgEl.textContent = '查詢中...';
    msgEl.className = 'lookup-msg';

    try {
        const resp = await fetch('/dev/webhook-simulator/lookup?orderNo=' + encodeURIComponent(orderNo) + '&isReturn=' + isReturn);
        const data = await resp.json();
        if (data.success) {
            document.getElementById('s_shipTradeNo').value = data.shipTradeNo;
            if (data.lgsType) document.getElementById('s_lgsType').value = data.lgsType;
            if (data.shipType) document.getElementById('s_shipType').value = data.shipType;
            if (data.merId) document.getElementById('merIdInput').value = data.merId;
            msgEl.textContent = 'ShipTradeNo: ' + data.shipTradeNo;
            msgEl.className = 'lookup-msg ok';
        } else {
            document.getElementById('s_shipTradeNo').value = '';
            msgEl.textContent = data.message || '查無資料';
            msgEl.className = 'lookup-msg err';
        }
    } catch (err) {
        msgEl.textContent = '查詢失敗: ' + err.message;
        msgEl.className = 'lookup-msg err';
    }
}

async function sendWebhook(type) {
    const targetUrl = getWebhookUrl();
    if (!targetUrl) return;
    const merId = document.getElementById('merIdInput').value.trim();
    let params = {}, typeName = '';

    if (type === 'payment') {
        const merTradeNo = document.getElementById('p_merTradeNo').value.trim();
        if (!merTradeNo) { alert('請輸入 MerTradeNo'); return; }
        let tradeNo = document.getElementById('p_tradeNo').value.trim();
        if (!tradeNo) tradeNo = 'T' + Date.now();
        params = { MerID: merId, MerTradeNo: merTradeNo, TradeNo: tradeNo, TradeStatus: document.getElementById('p_tradeStatus').value, TradeAmt: document.getElementById('p_tradeAmt').value, PayType: document.getElementById('p_payType').value, Message: document.getElementById('p_message').value, TradeDate: new Date().toISOString().replace('T', ' ').substring(0, 19) };
        typeName = '付款通知';
    } else if (type === 'refund') {
        const merTradeNo = document.getElementById('r_merTradeNo').value.trim();
        if (!merTradeNo) { alert('請輸入 MerTradeNo'); return; }
        const rs = document.getElementById('r_refundStatus').value;
        params = { Status: rs === '1' ? 'SUCCESS' : 'FAIL', Message: rs === '1' ? 'SUCCESS' : 'FAIL', MerID: merId, MerTradeNo: merTradeNo, TradeNo: document.getElementById('r_tradeNo').value.trim(), RefundAmt: document.getElementById('r_refundAmt').value, PaymentType: '2', RefundStatus: rs, BankDate: document.getElementById('r_bankDate').value };
        typeName = '退款轉匯';
    } else if (type === 'shipping') {
        const shipTradeNo = document.getElementById('s_shipTradeNo').value.trim();
        if (!shipTradeNo) { alert('請先查詢 ShipTradeNo'); return; }
        const direction = currentShipDirection();
        const message = document.getElementById('s_message').value;
        params = { MerID: merId, ShipTradeNo: shipTradeNo, ShipStatus: document.getElementById('s_shipStatus').value, ShipStatusDesc: message, ShipStatusTime: new Date().toISOString().replace('T', ' ').substring(0, 19), ShipType: document.getElementById('s_shipType').value, LgsType: document.getElementById('s_lgsType').value, Message: message };
        typeName = direction === 'reverse' ? '售後退貨貨態' : '物流狀態';
    }

    const btn = document.querySelector('.tab-content.active .submit-btn');
    const origText = btn.textContent;
    btn.disabled = true; btn.textContent = '發送中...';

    try {
        const resp = await fetch('/dev/webhook-simulator/send', {
            method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ targetUrl, merId, eventType: type, params })
        });
        const result = await resp.json();
        showResult(result.success, result);
        addHistory(type, typeName, params, result.success);
    } catch (err) {
        showResult(false, { webhookResponse: err.message });
    } finally {
        btn.disabled = false; btn.textContent = origText;
    }
}

function showResult(ok, result) {
    const section = document.getElementById('resultSection');
    section.style.display = 'block';
    const statusEl = document.getElementById('resultStatus');
    statusEl.className = 'result-status ' + (ok ? 'success' : 'error');
    statusEl.textContent = ok ? 'OK' : 'Error';
    document.getElementById('resultResponse').textContent = result.webhookResponse ? 'HTTP ' + (result.webhookStatusCode || '') + '\\n\\n' + result.webhookResponse : JSON.stringify(result, null, 2);
    if (result.rawPayload) document.getElementById('resultPayload').textContent = result.rawPayload;
    section.scrollIntoView({ behavior: 'smooth' });
}

const history = [];
function addHistory(type, typeName, params, ok) {
    const key = type === 'shipping' ? params.ShipTradeNo : params.MerTradeNo;
    const tagType = type === 'shipping' && typeName.includes('售後') ? 'aftersales' : type;
    const statusText = type === 'payment'
        ? tradeStatusMessages[params.TradeStatus]
        : type === 'refund'
            ? (params.RefundStatus === '1' ? '轉匯成功' : '轉匯失敗')
            : (params.Message || params.ShipStatusDesc || params.ShipStatus);
    history.unshift({ type, typeName, key, statusText, ok, time: new Date().toLocaleTimeString('zh-TW', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) });
    history[0].tagType = tagType;
    if (history.length > 20) history.pop();
    renderHistory();
}

function renderHistory() {
    document.getElementById('historySection').style.display = 'block';
    document.getElementById('historyList').innerHTML = history.map(h =>
        '<div class="history-item"><span class="tag ' + (h.tagType || h.type) + '">' + h.typeName + '</span><span>' + h.key + ' &rarr; ' + h.statusText + '</span><span style="color:' + (h.ok ? '#2e7d32' : '#c62828') + '">' + (h.ok ? 'OK' : 'ERR') + '</span><span class="time">' + h.time + '</span></div>'
    ).join('');
}

document.addEventListener('DOMContentLoaded', initDefaults);
</script>
</body>
</html>
""";
}
