import 'dart:typed_data';

import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/transport.dart';

ErrorDetail _detail(String type, dynamic message) =>
    ErrorDetail(type, message.writeToBuffer() as Uint8List);

/// [message] 是伺服器給使用者的句子（`LocalizedMessage`）；不給就不帶。
ConnectException reasonError(Code code, String reason, {String? message}) =>
    ConnectException(
      code,
      reason,
      details: [
        _detail(
          'google.rpc.ErrorInfo',
          ErrorInfo()
            ..reason = reason
            ..domain = 'hawkr',
        ),
        if (message != null)
          _detail(
            'google.rpc.LocalizedMessage',
            LocalizedMessage()
              ..locale = 'zh-TW'
              ..message = message,
          ),
      ],
    );

ConnectException fieldError(String field, String message) =>
    fieldErrors({field: message});

/// 多個欄位同時有錯（欄位路徑 -> 給使用者的訊息）。
ConnectException fieldErrors(Map<String, String> messages) => ConnectException(
  Code.invalidArgument,
  'invalid',
  details: [
    _detail(
      'google.rpc.BadRequest',
      BadRequest()
        ..fieldViolations.addAll([
          for (final MapEntry(key: field, value: message) in messages.entries)
            BadRequest_FieldViolation()
              ..field_1 = field
              ..description = 'invalid'
              ..localizedMessage = (LocalizedMessage()
                ..locale = 'zh-TW'
                ..message = message),
        ]),
    ),
  ],
);
