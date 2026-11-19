// This is a generated file - do not edit.
//
// Generated from hawkr/assistant/v1alpha1/assistant.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/struct.pb.dart' as $0;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../common/v1alpha1/error_reason.pbenum.dart' as $2;
import 'assistant.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'assistant.pbenum.dart';

class SendAssistantMessageRequest extends $pb.GeneratedMessage {
  factory SendAssistantMessageRequest({
    AssistantTopic? topic,
    $core.String? sessionId,
    $core.String? message,
    $core.String? imageUrl,
  }) {
    final result = SendAssistantMessageRequest._();
    if (topic != null) result.topic = topic;
    if (sessionId != null) result.sessionId = sessionId;
    if (message != null) result.message = message;
    if (imageUrl != null) result.imageUrl = imageUrl;
    return result;
  }

  SendAssistantMessageRequest._();

  factory SendAssistantMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendAssistantMessageRequest()..mergeFromBuffer(data, registry);
  factory SendAssistantMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendAssistantMessageRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendAssistantMessageRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: SendAssistantMessageRequest.$_createMessage)
    ..aE<AssistantTopic>(1, _omitFieldNames ? '' : 'topic',
        enumValues: AssistantTopic.values)
    ..aOS(2, _omitFieldNames ? '' : 'sessionId')
    ..aOS(3, _omitFieldNames ? '' : 'message')
    ..aOS(4, _omitFieldNames ? '' : 'imageUrl')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendAssistantMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendAssistantMessageRequest copyWith(
          void Function(SendAssistantMessageRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SendAssistantMessageRequest))
          as SendAssistantMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SendAssistantMessageRequest() / SendAssistantMessageRequest.new instead')
  static SendAssistantMessageRequest create() =>
      SendAssistantMessageRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SendAssistantMessageRequest._();
  @$core.override
  SendAssistantMessageRequest createEmptyInstance() =>
      SendAssistantMessageRequest._();
  @$core.pragma('dart2js:noInline')
  static SendAssistantMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendAssistantMessageRequest>(
          SendAssistantMessageRequest.$_createMessage);
  static SendAssistantMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  AssistantTopic get topic => $_getN(0);
  @$pb.TagNumber(1)
  set topic(AssistantTopic value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTopic() => $_has(0);
  @$pb.TagNumber(1)
  void clearTopic() => $_clearField(1);

  /// 要接續的對話；空字串表示開新的對話。
  @$pb.TagNumber(2)
  $core.String get sessionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sessionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSessionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSessionId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get message => $_getSZ(2);
  @$pb.TagNumber(3)
  set message($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMessage() => $_has(2);
  @$pb.TagNumber(3)
  void clearMessage() => $_clearField(3);

  /// 附帶的圖片網址；沒有圖時不設。
  @$pb.TagNumber(4)
  $core.String get imageUrl => $_getSZ(3);
  @$pb.TagNumber(4)
  set imageUrl($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasImageUrl() => $_has(3);
  @$pb.TagNumber(4)
  void clearImageUrl() => $_clearField(4);
}

class SendAssistantMessageResponse extends $pb.GeneratedMessage {
  factory SendAssistantMessageResponse({
    $core.String? turnId,
    $core.String? sessionId,
  }) {
    final result = SendAssistantMessageResponse._();
    if (turnId != null) result.turnId = turnId;
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  SendAssistantMessageResponse._();

  factory SendAssistantMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendAssistantMessageResponse()..mergeFromBuffer(data, registry);
  factory SendAssistantMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendAssistantMessageResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendAssistantMessageResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: SendAssistantMessageResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'turnId')
    ..aOS(2, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendAssistantMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendAssistantMessageResponse copyWith(
          void Function(SendAssistantMessageResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SendAssistantMessageResponse))
          as SendAssistantMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SendAssistantMessageResponse() / SendAssistantMessageResponse.new instead')
  static SendAssistantMessageResponse create() =>
      SendAssistantMessageResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SendAssistantMessageResponse._();
  @$core.override
  SendAssistantMessageResponse createEmptyInstance() =>
      SendAssistantMessageResponse._();
  @$core.pragma('dart2js:noInline')
  static SendAssistantMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendAssistantMessageResponse>(
          SendAssistantMessageResponse.$_createMessage);
  static SendAssistantMessageResponse? _defaultInstance;

  /// 這一輪的 ID，傳給 WatchAssistantTurn。
  @$pb.TagNumber(1)
  $core.String get turnId => $_getSZ(0);
  @$pb.TagNumber(1)
  set turnId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTurnId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTurnId() => $_clearField(1);

  /// 這一輪所屬的對話；開新對話時是新建的 ID。
  @$pb.TagNumber(2)
  $core.String get sessionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sessionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSessionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSessionId() => $_clearField(2);
}

class WatchAssistantTurnRequest extends $pb.GeneratedMessage {
  factory WatchAssistantTurnRequest({
    $core.String? turnId,
    $core.String? cursor,
  }) {
    final result = WatchAssistantTurnRequest._();
    if (turnId != null) result.turnId = turnId;
    if (cursor != null) result.cursor = cursor;
    return result;
  }

  WatchAssistantTurnRequest._();

  factory WatchAssistantTurnRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchAssistantTurnRequest()..mergeFromBuffer(data, registry);
  factory WatchAssistantTurnRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchAssistantTurnRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchAssistantTurnRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: WatchAssistantTurnRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'turnId')
    ..aOS(2, _omitFieldNames ? '' : 'cursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchAssistantTurnRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchAssistantTurnRequest copyWith(
          void Function(WatchAssistantTurnRequest) updates) =>
      super.copyWith((message) => updates(message as WatchAssistantTurnRequest))
          as WatchAssistantTurnRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use WatchAssistantTurnRequest() / WatchAssistantTurnRequest.new instead')
  static WatchAssistantTurnRequest create() => WatchAssistantTurnRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      WatchAssistantTurnRequest._();
  @$core.override
  WatchAssistantTurnRequest createEmptyInstance() =>
      WatchAssistantTurnRequest._();
  @$core.pragma('dart2js:noInline')
  static WatchAssistantTurnRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WatchAssistantTurnRequest>(
          WatchAssistantTurnRequest.$_createMessage);
  static WatchAssistantTurnRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get turnId => $_getSZ(0);
  @$pb.TagNumber(1)
  set turnId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTurnId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTurnId() => $_clearField(1);

  /// 上一次收到的事件的 cursor，從它之後接續；空字串表示從這一輪的第一個事件開始。client 當成不透明的字串。
  @$pb.TagNumber(2)
  $core.String get cursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set cursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearCursor() => $_clearField(2);
}

enum WatchAssistantTurnResponse_Event {
  text,
  toolResult,
  finished,
  error,
  notSet
}

/// WatchAssistantTurnResponse 是串流上的一個事件。
class WatchAssistantTurnResponse extends $pb.GeneratedMessage {
  factory WatchAssistantTurnResponse({
    $core.String? cursor,
    AssistantTextDelta? text,
    AssistantToolResult? toolResult,
    AssistantFinished? finished,
    AssistantStreamError? error,
  }) {
    final result = WatchAssistantTurnResponse._();
    if (cursor != null) result.cursor = cursor;
    if (text != null) result.text = text;
    if (toolResult != null) result.toolResult = toolResult;
    if (finished != null) result.finished = finished;
    if (error != null) result.error = error;
    return result;
  }

  WatchAssistantTurnResponse._();

  factory WatchAssistantTurnResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchAssistantTurnResponse()..mergeFromBuffer(data, registry);
  factory WatchAssistantTurnResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchAssistantTurnResponse()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, WatchAssistantTurnResponse_Event>
      _WatchAssistantTurnResponse_EventByTag = {
    2: WatchAssistantTurnResponse_Event.text,
    3: WatchAssistantTurnResponse_Event.toolResult,
    4: WatchAssistantTurnResponse_Event.finished,
    5: WatchAssistantTurnResponse_Event.error,
    0: WatchAssistantTurnResponse_Event.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchAssistantTurnResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: WatchAssistantTurnResponse.$_createMessage)
    ..oo(0, [2, 3, 4, 5])
    ..aOS(1, _omitFieldNames ? '' : 'cursor')
    ..aOM<AssistantTextDelta>(2, _omitFieldNames ? '' : 'text',
        subBuilder: AssistantTextDelta.$_createMessage)
    ..aOM<AssistantToolResult>(3, _omitFieldNames ? '' : 'toolResult',
        subBuilder: AssistantToolResult.$_createMessage)
    ..aOM<AssistantFinished>(4, _omitFieldNames ? '' : 'finished',
        subBuilder: AssistantFinished.$_createMessage)
    ..aOM<AssistantStreamError>(5, _omitFieldNames ? '' : 'error',
        subBuilder: AssistantStreamError.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchAssistantTurnResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchAssistantTurnResponse copyWith(
          void Function(WatchAssistantTurnResponse) updates) =>
      super.copyWith(
              (message) => updates(message as WatchAssistantTurnResponse))
          as WatchAssistantTurnResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use WatchAssistantTurnResponse() / WatchAssistantTurnResponse.new instead')
  static WatchAssistantTurnResponse create() => WatchAssistantTurnResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      WatchAssistantTurnResponse._();
  @$core.override
  WatchAssistantTurnResponse createEmptyInstance() =>
      WatchAssistantTurnResponse._();
  @$core.pragma('dart2js:noInline')
  static WatchAssistantTurnResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WatchAssistantTurnResponse>(
          WatchAssistantTurnResponse.$_createMessage);
  static WatchAssistantTurnResponse? _defaultInstance;

  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  WatchAssistantTurnResponse_Event whichEvent() =>
      _WatchAssistantTurnResponse_EventByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  void clearEvent() => $_clearField($_whichOneof(0));

  /// 這個事件的位置。client 記下最後收到的值，重新訂閱時帶回。
  @$pb.TagNumber(1)
  $core.String get cursor => $_getSZ(0);
  @$pb.TagNumber(1)
  set cursor($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCursor() => $_has(0);
  @$pb.TagNumber(1)
  void clearCursor() => $_clearField(1);

  /// 回覆文字的增量，client 依序串接。
  @$pb.TagNumber(2)
  AssistantTextDelta get text => $_getN(1);
  @$pb.TagNumber(2)
  set text(AssistantTextDelta value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasText() => $_has(1);
  @$pb.TagNumber(2)
  void clearText() => $_clearField(2);
  @$pb.TagNumber(2)
  AssistantTextDelta ensureText() => $_ensure(1);

  /// 工具執行的結果。
  @$pb.TagNumber(3)
  AssistantToolResult get toolResult => $_getN(2);
  @$pb.TagNumber(3)
  set toolResult(AssistantToolResult value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasToolResult() => $_has(2);
  @$pb.TagNumber(3)
  void clearToolResult() => $_clearField(3);
  @$pb.TagNumber(3)
  AssistantToolResult ensureToolResult() => $_ensure(2);

  /// 這一輪正常結束。
  @$pb.TagNumber(4)
  AssistantFinished get finished => $_getN(3);
  @$pb.TagNumber(4)
  set finished(AssistantFinished value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasFinished() => $_has(3);
  @$pb.TagNumber(4)
  void clearFinished() => $_clearField(4);
  @$pb.TagNumber(4)
  AssistantFinished ensureFinished() => $_ensure(3);

  /// 這一輪失敗。
  @$pb.TagNumber(5)
  AssistantStreamError get error => $_getN(4);
  @$pb.TagNumber(5)
  set error(AssistantStreamError value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasError() => $_has(4);
  @$pb.TagNumber(5)
  void clearError() => $_clearField(5);
  @$pb.TagNumber(5)
  AssistantStreamError ensureError() => $_ensure(4);
}

class AssistantTextDelta extends $pb.GeneratedMessage {
  factory AssistantTextDelta({
    $core.String? text,
  }) {
    final result = AssistantTextDelta._();
    if (text != null) result.text = text;
    return result;
  }

  AssistantTextDelta._();

  factory AssistantTextDelta.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantTextDelta()..mergeFromBuffer(data, registry);
  factory AssistantTextDelta.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantTextDelta()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AssistantTextDelta',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: AssistantTextDelta.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'text')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantTextDelta clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantTextDelta copyWith(void Function(AssistantTextDelta) updates) =>
      super.copyWith((message) => updates(message as AssistantTextDelta))
          as AssistantTextDelta;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AssistantTextDelta() / AssistantTextDelta.new instead')
  static AssistantTextDelta create() => AssistantTextDelta._();
  static $pb.GeneratedMessage $_createMessage() => AssistantTextDelta._();
  @$core.override
  AssistantTextDelta createEmptyInstance() => AssistantTextDelta._();
  @$core.pragma('dart2js:noInline')
  static AssistantTextDelta getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AssistantTextDelta>(
          AssistantTextDelta.$_createMessage);
  static AssistantTextDelta? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get text => $_getSZ(0);
  @$pb.TagNumber(1)
  set text($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasText() => $_has(0);
  @$pb.TagNumber(1)
  void clearText() => $_clearField(1);
}

/// AssistantToolResult 說明助理呼叫了哪個工具，以及它有沒有成功；工具的回傳內容不對外。
class AssistantToolResult extends $pb.GeneratedMessage {
  factory AssistantToolResult({
    $core.String? toolName,
    $core.bool? failed,
    $0.Struct? arguments,
  }) {
    final result = AssistantToolResult._();
    if (toolName != null) result.toolName = toolName;
    if (failed != null) result.failed = failed;
    if (arguments != null) result.arguments = arguments;
    return result;
  }

  AssistantToolResult._();

  factory AssistantToolResult.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantToolResult()..mergeFromBuffer(data, registry);
  factory AssistantToolResult.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantToolResult()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AssistantToolResult',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: AssistantToolResult.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'toolName')
    ..aOB(2, _omitFieldNames ? '' : 'failed')
    ..aOM<$0.Struct>(3, _omitFieldNames ? '' : 'arguments',
        subBuilder: $0.Struct.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantToolResult clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantToolResult copyWith(void Function(AssistantToolResult) updates) =>
      super.copyWith((message) => updates(message as AssistantToolResult))
          as AssistantToolResult;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use AssistantToolResult() / AssistantToolResult.new instead')
  static AssistantToolResult create() => AssistantToolResult._();
  static $pb.GeneratedMessage $_createMessage() => AssistantToolResult._();
  @$core.override
  AssistantToolResult createEmptyInstance() => AssistantToolResult._();
  @$core.pragma('dart2js:noInline')
  static AssistantToolResult getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AssistantToolResult>(
          AssistantToolResult.$_createMessage);
  static AssistantToolResult? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get toolName => $_getSZ(0);
  @$pb.TagNumber(1)
  set toolName($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasToolName() => $_has(0);
  @$pb.TagNumber(1)
  void clearToolName() => $_clearField(1);

  /// 工具執行失敗；助理仍會接著回覆。
  @$pb.TagNumber(2)
  $core.bool get failed => $_getBF(1);
  @$pb.TagNumber(2)
  set failed($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFailed() => $_has(1);
  @$pb.TagNumber(2)
  void clearFailed() => $_clearField(2);

  /// 助理呼叫工具時帶的參數（物件，鍵是工具的參數名）；沒有參數時不設。
  @$pb.TagNumber(3)
  $0.Struct get arguments => $_getN(2);
  @$pb.TagNumber(3)
  set arguments($0.Struct value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasArguments() => $_has(2);
  @$pb.TagNumber(3)
  void clearArguments() => $_clearField(3);
  @$pb.TagNumber(3)
  $0.Struct ensureArguments() => $_ensure(2);
}

class AssistantFinished extends $pb.GeneratedMessage {
  factory AssistantFinished({
    $core.String? sessionId,
  }) {
    final result = AssistantFinished._();
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  AssistantFinished._();

  factory AssistantFinished.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantFinished()..mergeFromBuffer(data, registry);
  factory AssistantFinished.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantFinished()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AssistantFinished',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: AssistantFinished.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantFinished clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantFinished copyWith(void Function(AssistantFinished) updates) =>
      super.copyWith((message) => updates(message as AssistantFinished))
          as AssistantFinished;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AssistantFinished() / AssistantFinished.new instead')
  static AssistantFinished create() => AssistantFinished._();
  static $pb.GeneratedMessage $_createMessage() => AssistantFinished._();
  @$core.override
  AssistantFinished createEmptyInstance() => AssistantFinished._();
  @$core.pragma('dart2js:noInline')
  static AssistantFinished getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AssistantFinished>(
          AssistantFinished.$_createMessage);
  static AssistantFinished? _defaultInstance;

  /// 這一輪所屬的對話。
  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);
}

/// AssistantStreamError 是這一輪的失敗，之後沒有其他事件。
class AssistantStreamError extends $pb.GeneratedMessage {
  factory AssistantStreamError({
    $2.ErrorReason? reason,
    $core.String? localizedMessage,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? metadata,
  }) {
    final result = AssistantStreamError._();
    if (reason != null) result.reason = reason;
    if (localizedMessage != null) result.localizedMessage = localizedMessage;
    if (metadata != null) result.metadata.addEntries(metadata);
    return result;
  }

  AssistantStreamError._();

  factory AssistantStreamError.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantStreamError()..mergeFromBuffer(data, registry);
  factory AssistantStreamError.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantStreamError()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AssistantStreamError',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: AssistantStreamError.$_createMessage)
    ..aE<$2.ErrorReason>(1, _omitFieldNames ? '' : 'reason',
        enumValues: $2.ErrorReason.values)
    ..aOS(2, _omitFieldNames ? '' : 'localizedMessage')
    ..m<$core.String, $core.String>(3, _omitFieldNames ? '' : 'metadata',
        entryClassName: 'AssistantStreamError.MetadataEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('hawkr.assistant.v1alpha1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantStreamError clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantStreamError copyWith(void Function(AssistantStreamError) updates) =>
      super.copyWith((message) => updates(message as AssistantStreamError))
          as AssistantStreamError;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AssistantStreamError() / AssistantStreamError.new instead')
  static AssistantStreamError create() => AssistantStreamError._();
  static $pb.GeneratedMessage $_createMessage() => AssistantStreamError._();
  @$core.override
  AssistantStreamError createEmptyInstance() => AssistantStreamError._();
  @$core.pragma('dart2js:noInline')
  static AssistantStreamError getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AssistantStreamError>(
          AssistantStreamError.$_createMessage);
  static AssistantStreamError? _defaultInstance;

  @$pb.TagNumber(1)
  $2.ErrorReason get reason => $_getN(0);
  @$pb.TagNumber(1)
  set reason($2.ErrorReason value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);

  /// 給使用者看的文字（zh-TW 或 en，依請求的語系）。
  @$pb.TagNumber(2)
  $core.String get localizedMessage => $_getSZ(1);
  @$pb.TagNumber(2)
  set localizedMessage($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLocalizedMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearLocalizedMessage() => $_clearField(2);

  /// 與 google.rpc.ErrorInfo.metadata 同規則：key 是 lowerCamelCase。
  @$pb.TagNumber(3)
  $pb.PbMap<$core.String, $core.String> get metadata => $_getMap(2);
}

class GetLatestAssistantSessionRequest extends $pb.GeneratedMessage {
  factory GetLatestAssistantSessionRequest() =>
      GetLatestAssistantSessionRequest._();

  GetLatestAssistantSessionRequest._();

  factory GetLatestAssistantSessionRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetLatestAssistantSessionRequest()..mergeFromBuffer(data, registry);
  factory GetLatestAssistantSessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetLatestAssistantSessionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetLatestAssistantSessionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: GetLatestAssistantSessionRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetLatestAssistantSessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetLatestAssistantSessionRequest copyWith(
          void Function(GetLatestAssistantSessionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetLatestAssistantSessionRequest))
          as GetLatestAssistantSessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetLatestAssistantSessionRequest() / GetLatestAssistantSessionRequest.new instead')
  static GetLatestAssistantSessionRequest create() =>
      GetLatestAssistantSessionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetLatestAssistantSessionRequest._();
  @$core.override
  GetLatestAssistantSessionRequest createEmptyInstance() =>
      GetLatestAssistantSessionRequest._();
  @$core.pragma('dart2js:noInline')
  static GetLatestAssistantSessionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetLatestAssistantSessionRequest>(
          GetLatestAssistantSessionRequest.$_createMessage);
  static GetLatestAssistantSessionRequest? _defaultInstance;
}

class GetLatestAssistantSessionResponse extends $pb.GeneratedMessage {
  factory GetLatestAssistantSessionResponse({
    $core.String? sessionId,
    $core.Iterable<AssistantMessage>? messages,
  }) {
    final result = GetLatestAssistantSessionResponse._();
    if (sessionId != null) result.sessionId = sessionId;
    if (messages != null) result.messages.addAll(messages);
    return result;
  }

  GetLatestAssistantSessionResponse._();

  factory GetLatestAssistantSessionResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetLatestAssistantSessionResponse()..mergeFromBuffer(data, registry);
  factory GetLatestAssistantSessionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetLatestAssistantSessionResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetLatestAssistantSessionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: GetLatestAssistantSessionResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..pPM<AssistantMessage>(2, _omitFieldNames ? '' : 'messages',
        subBuilder: AssistantMessage.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetLatestAssistantSessionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetLatestAssistantSessionResponse copyWith(
          void Function(GetLatestAssistantSessionResponse) updates) =>
      super.copyWith((message) =>
              updates(message as GetLatestAssistantSessionResponse))
          as GetLatestAssistantSessionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetLatestAssistantSessionResponse() / GetLatestAssistantSessionResponse.new instead')
  static GetLatestAssistantSessionResponse create() =>
      GetLatestAssistantSessionResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetLatestAssistantSessionResponse._();
  @$core.override
  GetLatestAssistantSessionResponse createEmptyInstance() =>
      GetLatestAssistantSessionResponse._();
  @$core.pragma('dart2js:noInline')
  static GetLatestAssistantSessionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetLatestAssistantSessionResponse>(
          GetLatestAssistantSessionResponse.$_createMessage);
  static GetLatestAssistantSessionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  /// 時間由舊到新。
  @$pb.TagNumber(2)
  $pb.PbList<AssistantMessage> get messages => $_getList(1);
}

class AssistantMessage extends $pb.GeneratedMessage {
  factory AssistantMessage({
    AssistantMessageRole? role,
    $core.String? text,
    $1.Timestamp? createTime,
  }) {
    final result = AssistantMessage._();
    if (role != null) result.role = role;
    if (text != null) result.text = text;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  AssistantMessage._();

  factory AssistantMessage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantMessage()..mergeFromBuffer(data, registry);
  factory AssistantMessage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AssistantMessage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AssistantMessage',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.assistant.v1alpha1'),
      createEmptyInstance: AssistantMessage.$_createMessage)
    ..aE<AssistantMessageRole>(1, _omitFieldNames ? '' : 'role',
        enumValues: AssistantMessageRole.values)
    ..aOS(2, _omitFieldNames ? '' : 'text')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantMessage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssistantMessage copyWith(void Function(AssistantMessage) updates) =>
      super.copyWith((message) => updates(message as AssistantMessage))
          as AssistantMessage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AssistantMessage() / AssistantMessage.new instead')
  static AssistantMessage create() => AssistantMessage._();
  static $pb.GeneratedMessage $_createMessage() => AssistantMessage._();
  @$core.override
  AssistantMessage createEmptyInstance() => AssistantMessage._();
  @$core.pragma('dart2js:noInline')
  static AssistantMessage getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AssistantMessage>(
          AssistantMessage.$_createMessage);
  static AssistantMessage? _defaultInstance;

  @$pb.TagNumber(1)
  AssistantMessageRole get role => $_getN(0);
  @$pb.TagNumber(1)
  set role(AssistantMessageRole value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRole() => $_has(0);
  @$pb.TagNumber(1)
  void clearRole() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get text => $_getSZ(1);
  @$pb.TagNumber(2)
  set text($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasText() => $_has(1);
  @$pb.TagNumber(2)
  void clearText() => $_clearField(2);

  @$pb.TagNumber(3)
  $1.Timestamp get createTime => $_getN(2);
  @$pb.TagNumber(3)
  set createTime($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCreateTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearCreateTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureCreateTime() => $_ensure(2);
}

/// AssistantAppService 是登入的買家的助理。身分取自 session，帳號沒有買家角色時回 permission_denied。
class AssistantAppServiceApi {
  final $pb.RpcClient _client;

  AssistantAppServiceApi(this._client);

  /// SendAssistantMessage 送出一則訊息，開始一輪助理回覆。伺服器受理後立刻回輪次 ID，回覆內容從
  /// WatchAssistantTurn 取得。session_id 空字串表示開新的對話。
  $async.Future<SendAssistantMessageResponse> sendAssistantMessage(
          $pb.ClientContext? ctx, SendAssistantMessageRequest request) =>
      _client.invoke<SendAssistantMessageResponse>(ctx, 'AssistantAppService',
          'SendAssistantMessage', request, SendAssistantMessageResponse());

  /// WatchAssistantTurn 訂閱一輪的事件，從 cursor 之後開始（空字串表示從頭）。只讀；伺服器在這一輪結束
  /// （finished 或 error 事件之後）或滿 100 秒時結束串流。
  $async.Future<WatchAssistantTurnResponse> watchAssistantTurn(
          $pb.ClientContext? ctx, WatchAssistantTurnRequest request) =>
      _client.invoke<WatchAssistantTurnResponse>(ctx, 'AssistantAppService',
          'WatchAssistantTurn', request, WatchAssistantTurnResponse());

  /// GetLatestAssistantSession 回傳呼叫者最近一次的對話與訊息；沒有對話時 session_id 是空字串。
  $async.Future<GetLatestAssistantSessionResponse> getLatestAssistantSession(
          $pb.ClientContext? ctx, GetLatestAssistantSessionRequest request) =>
      _client.invoke<GetLatestAssistantSessionResponse>(
          ctx,
          'AssistantAppService',
          'GetLatestAssistantSession',
          request,
          GetLatestAssistantSessionResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
