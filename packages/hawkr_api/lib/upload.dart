/// 圖片與機敏檔案的上傳流程（`MediaAppService`），語意與 `@hawkr/api/connect/upload` 相同：
/// 公開圖片走預簽 PUT 後輪詢 `GetUpload`；機敏檔走分段 PUT 到單體的 `/api/files/secure_uploads`，
/// 段序號不符（409）依 `Hawkr-Next-Index` 續傳。HEIC 等由呼叫端先轉成伺服器收的格式。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io' as io;
import 'dart:typed_data';

import 'package:connectrpc/connect.dart';
import 'package:crypto/crypto.dart' as crypto;
import 'package:fixnum/fixnum.dart';

import 'gen/hawkr/media/v1alpha1/media.connect.client.dart';
import 'gen/hawkr/media/v1alpha1/media.pb.dart';

export 'gen/hawkr/media/v1alpha1/media.pbenum.dart'
    show RejectReason, UploadState;

/// 段 PUT 回 409 時帶下一個要送的段序號的回應標頭。
const kNextIndexHeader = 'hawkr-next-index';

/// 一次 PUT 的結果；[headers] 的 key 是小寫。
class UploadHttpResult {
  const UploadHttpResult(this.status, [this.headers = const {}]);
  final int status;
  final Map<String, String> headers;
}

/// 上傳用的 HTTP PUT；測試換掉它。
abstract interface class UploadHttp {
  Future<UploadHttpResult> put(
    Uri url,
    Map<String, String> headers,
    Uint8List body,
  );
}

/// 以 `dart:io` 的 `HttpClient` 送出；Content-Length 由 [body] 的長度決定。預簽網址是跨來源的物件儲存：連線、閒置與整個請求都有逾時，
/// 逾時以 `TimeoutException` 結束（請求被中止）。
class IoUploadHttp implements UploadHttp {
  IoUploadHttp({
    io.HttpClient? client,
    this.requestTimeout = const Duration(seconds: 120),
    Duration connectionTimeout = const Duration(seconds: 15),
    Duration idleTimeout = const Duration(seconds: 30),
  }) : _client = (client ?? io.HttpClient())
         ..connectionTimeout = connectionTimeout
         ..idleTimeout = idleTimeout;
  final io.HttpClient _client;
  final Duration requestTimeout;

  @override
  Future<UploadHttpResult> put(
    Uri url,
    Map<String, String> headers,
    Uint8List body,
  ) async {
    final req = await _client.putUrl(url);
    req.contentLength = body.length;
    headers.forEach((k, v) {
      if (k.toLowerCase() != 'content-length') {
        req.headers.set(k, v, preserveHeaderCase: true);
      }
    });
    req.add(body);
    try {
      return await _send(req).timeout(requestTimeout);
    } on TimeoutException {
      req.abort();
      rethrow;
    }
  }

  Future<UploadHttpResult> _send(io.HttpClientRequest req) async {
    final res = await req.close();
    final out = <String, String>{};
    res.headers.forEach((k, v) => out[k.toLowerCase()] = v.join(','));
    await res.drain<void>();
    return UploadHttpResult(res.statusCode, out);
  }
}

/// 處理被伺服器拒絕（`REJECTED`）；[reason] 對應 `hawkr_ui_text` 的 `RejectReason` 文字。
class UploadRejected implements Exception {
  const UploadRejected(this.reason);
  final RejectReason reason;
  @override
  String toString() => 'UploadRejected(${reason.name})';
}

/// 上傳流程失敗（儲存空間或段端點回非 2xx、輪詢逾時、狀態不是預期）。[status] 是最後一次 HTTP 狀態。
class UploadFailure implements Exception {
  const UploadFailure(this.message, {this.status});
  final String message;
  final int? status;
  @override
  String toString() =>
      'UploadFailure($message${status == null ? '' : ', $status'})';
}

typedef UploadSleep = Future<void> Function(Duration d);

const _imageTypes = {'image/jpeg', 'image/png', 'image/webp', 'image/gif'};

/// 上傳一張公開圖片，回傳 READY 的 staged token。[contentType] 只能是 jpeg、png、webp、gif。
/// 輪詢遇到 RPC 錯誤時下一輪再讀；[maxPolls] 輪仍未 READY 丟 [UploadFailure]；REJECTED 丟 [UploadRejected]。
Future<String> uploadImage({
  required MediaAppServiceClient client,
  required UploadHttp http,
  required Uint8List bytes,
  required String contentType,
  Duration pollInterval = const Duration(seconds: 1),
  int maxPolls = 60,
  UploadSleep sleep = _defaultSleep,
}) async {
  if (!_imageTypes.contains(contentType)) {
    throw UploadFailure('unsupported content type: $contentType');
  }
  final begin = await client.beginUpload(
    BeginUploadRequest(
      size: Int64(bytes.length),
      contentType: contentType,
      md5: base64.encode(crypto.md5.convert(bytes).bytes),
    ),
  );
  final put = await http.put(
    Uri.parse(begin.uploadUrl),
    Map<String, String>.of(begin.uploadHeaders),
    bytes,
  );
  if (put.status < 200 || put.status >= 300) {
    throw UploadFailure('storage put failed', status: put.status);
  }
  await client.completeUpload(
    CompleteUploadRequest(stagedToken: begin.stagedToken),
  );
  for (var i = 0; i < maxPolls; i++) {
    GetUploadResponse res;
    try {
      res = await client.getUpload(
        GetUploadRequest(stagedToken: begin.stagedToken),
      );
    } on ConnectException catch (e) {
      if (e.code == Code.notFound ||
          e.code == Code.permissionDenied ||
          e.code == Code.unauthenticated) {
        rethrow;
      }
      await sleep(pollInterval);
      continue;
    }
    switch (res.state) {
      case UploadState.UPLOAD_STATE_READY:
        return begin.stagedToken;
      case UploadState.UPLOAD_STATE_REJECTED:
        throw UploadRejected(
          res.hasRejectReason()
              ? res.rejectReason
              : RejectReason.REJECT_REASON_UNSPECIFIED,
        );
      case UploadState.UPLOAD_STATE_PENDING:
      case UploadState.UPLOAD_STATE_UPLOADED:
        await sleep(pollInterval);
      default:
        throw UploadFailure('unexpected upload state: ${res.state.name}');
    }
  }
  throw const UploadFailure('upload processing timed out');
}

/// 機敏檔案的分段上傳，回傳 staged token。段送到 `{filesBase}/api/files/secure_uploads/{token}/chunks/{index}`，
/// [headers] 每段送出時讀取（例如 `Authorization`）。409 依 `Hawkr-Next-Index` 續傳，最多 [maxResumes] 次；
/// 408、429、5xx 與連線錯誤同一段最多重送 [maxAttempts] 次。
Future<String> uploadSecure({
  required MediaAppServiceClient client,
  required UploadHttp http,
  required Uri filesBase,
  required Map<String, String> Function() headers,
  required Uint8List bytes,
  int maxAttempts = 3,
  int maxResumes = 8,
  UploadSleep sleep = _defaultSleep,
}) async {
  final begin = await client.beginSecureUpload(
    BeginSecureUploadRequest(size: Int64(bytes.length)),
  );
  final size = begin.chunkSize;
  final count = begin.chunkCount;
  if (size <= 0 || count <= 0 || (count - 1) * size >= bytes.length) {
    throw const UploadFailure('invalid chunk plan');
  }
  var index = 0;
  var resumes = 0;
  var attempt = 0;
  while (index < count) {
    final start = index * size;
    final end = start + size < bytes.length ? start + size : bytes.length;
    final url = filesBase.replace(
      path:
          '${filesBase.path.replaceAll(RegExp(r'/+$'), '')}/api/files/secure_uploads/${begin.stagedToken}/chunks/$index',
    );
    UploadHttpResult? res;
    try {
      res = await http.put(url, {
        ...headers(),
        'Content-Type': 'application/octet-stream',
      }, Uint8List.sublistView(bytes, start, end));
    } on io.SocketException {
      res = null;
    } on io.HttpException {
      res = null;
    } on TimeoutException {
      res = null;
    }
    final status = res?.status;
    if (status != null && status >= 200 && status < 300) {
      index++;
      attempt = 0;
      continue;
    }
    if (status == 409) {
      final next = int.tryParse(res!.headers[kNextIndexHeader] ?? '');
      if (next == null || next < 0 || next > count || ++resumes > maxResumes) {
        throw UploadFailure('chunk resume failed', status: status);
      }
      index = next;
      attempt = 0;
      continue;
    }
    final transient =
        status == null || status == 408 || status == 429 || status >= 500;
    if (!transient || ++attempt >= maxAttempts) {
      throw UploadFailure('chunk put failed', status: status);
    }
    await sleep(Duration(milliseconds: 200 * attempt));
  }
  return begin.stagedToken;
}

Future<void> _defaultSleep(Duration d) => Future<void>.delayed(d);
