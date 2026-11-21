// This is a generated file - do not edit.
//
// Generated from hawkr/procurement/v1alpha1/procurement.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../../google/type/date.pb.dart' as $0;
import '../../common/v1alpha1/error_reason.pbenum.dart' as $3;
import '../../common/v1alpha1/store_card.pb.dart' as $2;
import 'procurement.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'procurement.pbenum.dart';

class QuestionOption extends $pb.GeneratedMessage {
  factory QuestionOption({
    $core.String? optionId,
    $core.String? label,
  }) {
    final result = QuestionOption._();
    if (optionId != null) result.optionId = optionId;
    if (label != null) result.label = label;
    return result;
  }

  QuestionOption._();

  factory QuestionOption.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      QuestionOption()..mergeFromBuffer(data, registry);
  factory QuestionOption.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      QuestionOption()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'QuestionOption',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: QuestionOption.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'optionId')
    ..aOS(2, _omitFieldNames ? '' : 'label')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QuestionOption clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QuestionOption copyWith(void Function(QuestionOption) updates) =>
      super.copyWith((message) => updates(message as QuestionOption))
          as QuestionOption;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use QuestionOption() / QuestionOption.new instead')
  static QuestionOption create() => QuestionOption._();
  static $pb.GeneratedMessage $_createMessage() => QuestionOption._();
  @$core.override
  QuestionOption createEmptyInstance() => QuestionOption._();
  @$core.pragma('dart2js:noInline')
  static QuestionOption getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<QuestionOption>(
          QuestionOption.$_createMessage);
  static QuestionOption? _defaultInstance;

  /// 回答時帶回的選項 ID。
  @$pb.TagNumber(1)
  $core.String get optionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set optionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOptionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOptionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get label => $_getSZ(1);
  @$pb.TagNumber(2)
  set label($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLabel() => $_has(1);
  @$pb.TagNumber(2)
  void clearLabel() => $_clearField(2);
}

/// ClarifyQuestion 是伺服器要買家回答的一個問題。
class ClarifyQuestion extends $pb.GeneratedMessage {
  factory ClarifyQuestion({
    SlotKey? slotKey,
    $core.String? prompt,
    $core.Iterable<QuestionOption>? options,
    $core.Iterable<QuestionOption>? unitOptions,
    $core.bool? allowFreeText,
    $core.bool? canSkip,
  }) {
    final result = ClarifyQuestion._();
    if (slotKey != null) result.slotKey = slotKey;
    if (prompt != null) result.prompt = prompt;
    if (options != null) result.options.addAll(options);
    if (unitOptions != null) result.unitOptions.addAll(unitOptions);
    if (allowFreeText != null) result.allowFreeText = allowFreeText;
    if (canSkip != null) result.canSkip = canSkip;
    return result;
  }

  ClarifyQuestion._();

  factory ClarifyQuestion.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClarifyQuestion()..mergeFromBuffer(data, registry);
  factory ClarifyQuestion.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClarifyQuestion()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClarifyQuestion',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ClarifyQuestion.$_createMessage)
    ..aE<SlotKey>(1, _omitFieldNames ? '' : 'slotKey',
        enumValues: SlotKey.values)
    ..aOS(2, _omitFieldNames ? '' : 'prompt')
    ..pPM<QuestionOption>(3, _omitFieldNames ? '' : 'options',
        subBuilder: QuestionOption.$_createMessage)
    ..pPM<QuestionOption>(4, _omitFieldNames ? '' : 'unitOptions',
        subBuilder: QuestionOption.$_createMessage)
    ..aOB(5, _omitFieldNames ? '' : 'allowFreeText')
    ..aOB(6, _omitFieldNames ? '' : 'canSkip')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClarifyQuestion clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClarifyQuestion copyWith(void Function(ClarifyQuestion) updates) =>
      super.copyWith((message) => updates(message as ClarifyQuestion))
          as ClarifyQuestion;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ClarifyQuestion() / ClarifyQuestion.new instead')
  static ClarifyQuestion create() => ClarifyQuestion._();
  static $pb.GeneratedMessage $_createMessage() => ClarifyQuestion._();
  @$core.override
  ClarifyQuestion createEmptyInstance() => ClarifyQuestion._();
  @$core.pragma('dart2js:noInline')
  static ClarifyQuestion getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ClarifyQuestion>(
          ClarifyQuestion.$_createMessage);
  static ClarifyQuestion? _defaultInstance;

  @$pb.TagNumber(1)
  SlotKey get slotKey => $_getN(0);
  @$pb.TagNumber(1)
  set slotKey(SlotKey value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSlotKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlotKey() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get prompt => $_getSZ(1);
  @$pb.TagNumber(2)
  set prompt($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPrompt() => $_has(1);
  @$pb.TagNumber(2)
  void clearPrompt() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<QuestionOption> get options => $_getList(2);

  /// 數量這類複合欄位的單位選項。
  @$pb.TagNumber(4)
  $pb.PbList<QuestionOption> get unitOptions => $_getList(3);

  @$pb.TagNumber(5)
  $core.bool get allowFreeText => $_getBF(4);
  @$pb.TagNumber(5)
  set allowFreeText($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAllowFreeText() => $_has(4);
  @$pb.TagNumber(5)
  void clearAllowFreeText() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get canSkip => $_getBF(5);
  @$pb.TagNumber(6)
  set canSkip($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCanSkip() => $_has(5);
  @$pb.TagNumber(6)
  void clearCanSkip() => $_clearField(6);
}

class Step extends $pb.GeneratedMessage {
  factory Step({
    $core.String? code,
    StepStatus? status,
    $core.String? label,
    $fixnum.Int64? durationMs,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? safeArgs,
    $core.String? text,
  }) {
    final result = Step._();
    if (code != null) result.code = code;
    if (status != null) result.status = status;
    if (label != null) result.label = label;
    if (durationMs != null) result.durationMs = durationMs;
    if (safeArgs != null) result.safeArgs.addEntries(safeArgs);
    if (text != null) result.text = text;
    return result;
  }

  Step._();

  factory Step.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Step()..mergeFromBuffer(data, registry);
  factory Step.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Step()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Step',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: Step.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..aE<StepStatus>(2, _omitFieldNames ? '' : 'status',
        enumValues: StepStatus.values)
    ..aOS(3, _omitFieldNames ? '' : 'label')
    ..aInt64(4, _omitFieldNames ? '' : 'durationMs')
    ..m<$core.String, $core.String>(5, _omitFieldNames ? '' : 'safeArgs',
        entryClassName: 'Step.SafeArgsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('hawkr.procurement.v1alpha1'))
    ..aOS(6, _omitFieldNames ? '' : 'text')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Step clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Step copyWith(void Function(Step) updates) =>
      super.copyWith((message) => updates(message as Step)) as Step;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Step() / Step.new instead')
  static Step create() => Step._();
  static $pb.GeneratedMessage $_createMessage() => Step._();
  @$core.override
  Step createEmptyInstance() => Step._();
  @$core.pragma('dart2js:noInline')
  static Step getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Step>(Step.$_createMessage);
  static Step? _defaultInstance;

  /// 步驟的代碼，client 依它對應固定的顯示。
  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);

  @$pb.TagNumber(2)
  StepStatus get status => $_getN(1);
  @$pb.TagNumber(2)
  set status(StepStatus value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  /// 伺服器的固定文字。
  @$pb.TagNumber(3)
  $core.String get label => $_getSZ(2);
  @$pb.TagNumber(3)
  set label($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLabel() => $_has(2);
  @$pb.TagNumber(3)
  void clearLabel() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get durationMs => $_getI64(3);
  @$pb.TagNumber(4)
  set durationMs($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDurationMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearDurationMs() => $_clearField(4);

  /// 顯示用的動態文案要的少數值，只在部分 code 上有。
  @$pb.TagNumber(5)
  $pb.PbMap<$core.String, $core.String> get safeArgs => $_getMap(4);

  /// 這一步實際發生了什麼的一句話；產生逾時或被擋下時為空，client 顯示 label。
  @$pb.TagNumber(6)
  $core.String get text => $_getSZ(5);
  @$pb.TagNumber(6)
  set text($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasText() => $_has(5);
  @$pb.TagNumber(6)
  void clearText() => $_clearField(6);
}

class RecordedField extends $pb.GeneratedMessage {
  factory RecordedField({
    SlotKey? key,
    $core.String? label,
    $core.String? value,
  }) {
    final result = RecordedField._();
    if (key != null) result.key = key;
    if (label != null) result.label = label;
    if (value != null) result.value = value;
    return result;
  }

  RecordedField._();

  factory RecordedField.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordedField()..mergeFromBuffer(data, registry);
  factory RecordedField.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordedField()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecordedField',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RecordedField.$_createMessage)
    ..aE<SlotKey>(1, _omitFieldNames ? '' : 'key', enumValues: SlotKey.values)
    ..aOS(2, _omitFieldNames ? '' : 'label')
    ..aOS(3, _omitFieldNames ? '' : 'value')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordedField clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordedField copyWith(void Function(RecordedField) updates) =>
      super.copyWith((message) => updates(message as RecordedField))
          as RecordedField;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RecordedField() / RecordedField.new instead')
  static RecordedField create() => RecordedField._();
  static $pb.GeneratedMessage $_createMessage() => RecordedField._();
  @$core.override
  RecordedField createEmptyInstance() => RecordedField._();
  @$core.pragma('dart2js:noInline')
  static RecordedField getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RecordedField>(
          RecordedField.$_createMessage);
  static RecordedField? _defaultInstance;

  @$pb.TagNumber(1)
  SlotKey get key => $_getN(0);
  @$pb.TagNumber(1)
  set key(SlotKey value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearKey() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get label => $_getSZ(1);
  @$pb.TagNumber(2)
  set label($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLabel() => $_has(1);
  @$pb.TagNumber(2)
  void clearLabel() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get value => $_getSZ(2);
  @$pb.TagNumber(3)
  set value($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasValue() => $_has(2);
  @$pb.TagNumber(3)
  void clearValue() => $_clearField(3);
}

class RejectedField extends $pb.GeneratedMessage {
  factory RejectedField({
    SlotKey? key,
    $core.String? reason,
  }) {
    final result = RejectedField._();
    if (key != null) result.key = key;
    if (reason != null) result.reason = reason;
    return result;
  }

  RejectedField._();

  factory RejectedField.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RejectedField()..mergeFromBuffer(data, registry);
  factory RejectedField.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RejectedField()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RejectedField',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RejectedField.$_createMessage)
    ..aE<SlotKey>(1, _omitFieldNames ? '' : 'key', enumValues: SlotKey.values)
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RejectedField clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RejectedField copyWith(void Function(RejectedField) updates) =>
      super.copyWith((message) => updates(message as RejectedField))
          as RejectedField;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RejectedField() / RejectedField.new instead')
  static RejectedField create() => RejectedField._();
  static $pb.GeneratedMessage $_createMessage() => RejectedField._();
  @$core.override
  RejectedField createEmptyInstance() => RejectedField._();
  @$core.pragma('dart2js:noInline')
  static RejectedField getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RejectedField>(
          RejectedField.$_createMessage);
  static RejectedField? _defaultInstance;

  @$pb.TagNumber(1)
  SlotKey get key => $_getN(0);
  @$pb.TagNumber(1)
  set key(SlotKey value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearKey() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

/// ItemResolution 是買家的品項原話被判定成平台分類樹的哪個節點，以及判定的方式。必須對買家可見且可更正。
class ItemResolution extends $pb.GeneratedMessage {
  factory ItemResolution({
    ItemResolutionKind? kind,
    $core.String? label,
    $core.String? displayValue,
  }) {
    final result = ItemResolution._();
    if (kind != null) result.kind = kind;
    if (label != null) result.label = label;
    if (displayValue != null) result.displayValue = displayValue;
    return result;
  }

  ItemResolution._();

  factory ItemResolution.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ItemResolution()..mergeFromBuffer(data, registry);
  factory ItemResolution.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ItemResolution()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ItemResolution',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ItemResolution.$_createMessage)
    ..aE<ItemResolutionKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: ItemResolutionKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'label')
    ..aOS(3, _omitFieldNames ? '' : 'displayValue')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ItemResolution clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ItemResolution copyWith(void Function(ItemResolution) updates) =>
      super.copyWith((message) => updates(message as ItemResolution))
          as ItemResolution;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ItemResolution() / ItemResolution.new instead')
  static ItemResolution create() => ItemResolution._();
  static $pb.GeneratedMessage $_createMessage() => ItemResolution._();
  @$core.override
  ItemResolution createEmptyInstance() => ItemResolution._();
  @$core.pragma('dart2js:noInline')
  static ItemResolution getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ItemResolution>(
          ItemResolution.$_createMessage);
  static ItemResolution? _defaultInstance;

  @$pb.TagNumber(1)
  ItemResolutionKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(ItemResolutionKind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  /// 分類名稱，或買家的描述。
  @$pb.TagNumber(2)
  $core.String get label => $_getSZ(1);
  @$pb.TagNumber(2)
  set label($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLabel() => $_has(1);
  @$pb.TagNumber(2)
  void clearLabel() => $_clearField(2);

  /// 伺服器組好的顯示字串。client 不用 kind 與 label 自己重拼，slots 面板與本輪記錄的 chip 才是同一套文案。
  @$pb.TagNumber(3)
  $core.String get displayValue => $_getSZ(2);
  @$pb.TagNumber(3)
  set displayValue($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDisplayValue() => $_has(2);
  @$pb.TagNumber(3)
  void clearDisplayValue() => $_clearField(3);
}

class Slots extends $pb.GeneratedMessage {
  factory Slots({
    $core.String? item,
    $core.int? quantity,
    $core.String? unit,
    $core.String? packaging,
    $fixnum.Int64? budgetPerUnit,
    $core.String? deliveryCity,
    $0.Date? deliveryDate,
    $core.bool? exactMatchRequired,
    ItemResolution? itemResolution,
  }) {
    final result = Slots._();
    if (item != null) result.item = item;
    if (quantity != null) result.quantity = quantity;
    if (unit != null) result.unit = unit;
    if (packaging != null) result.packaging = packaging;
    if (budgetPerUnit != null) result.budgetPerUnit = budgetPerUnit;
    if (deliveryCity != null) result.deliveryCity = deliveryCity;
    if (deliveryDate != null) result.deliveryDate = deliveryDate;
    if (exactMatchRequired != null)
      result.exactMatchRequired = exactMatchRequired;
    if (itemResolution != null) result.itemResolution = itemResolution;
    return result;
  }

  Slots._();

  factory Slots.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Slots()..mergeFromBuffer(data, registry);
  factory Slots.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Slots()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Slots',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: Slots.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'item')
    ..aI(2, _omitFieldNames ? '' : 'quantity')
    ..aOS(3, _omitFieldNames ? '' : 'unit')
    ..aOS(4, _omitFieldNames ? '' : 'packaging')
    ..aInt64(5, _omitFieldNames ? '' : 'budgetPerUnit')
    ..aOS(6, _omitFieldNames ? '' : 'deliveryCity')
    ..aOM<$0.Date>(7, _omitFieldNames ? '' : 'deliveryDate',
        subBuilder: $0.Date.$_createMessage)
    ..aOB(8, _omitFieldNames ? '' : 'exactMatchRequired')
    ..aOM<ItemResolution>(9, _omitFieldNames ? '' : 'itemResolution',
        subBuilder: ItemResolution.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Slots clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Slots copyWith(void Function(Slots) updates) =>
      super.copyWith((message) => updates(message as Slots)) as Slots;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Slots() / Slots.new instead')
  static Slots create() => Slots._();
  static $pb.GeneratedMessage $_createMessage() => Slots._();
  @$core.override
  Slots createEmptyInstance() => Slots._();
  @$core.pragma('dart2js:noInline')
  static Slots getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Slots>(Slots.$_createMessage);
  static Slots? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get item => $_getSZ(0);
  @$pb.TagNumber(1)
  set item($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasItem() => $_has(0);
  @$pb.TagNumber(1)
  void clearItem() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get quantity => $_getIZ(1);
  @$pb.TagNumber(2)
  set quantity($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasQuantity() => $_has(1);
  @$pb.TagNumber(2)
  void clearQuantity() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get unit => $_getSZ(2);
  @$pb.TagNumber(3)
  set unit($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasUnit() => $_has(2);
  @$pb.TagNumber(3)
  void clearUnit() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get packaging => $_getSZ(3);
  @$pb.TagNumber(4)
  set packaging($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPackaging() => $_has(3);
  @$pb.TagNumber(4)
  void clearPackaging() => $_clearField(4);

  /// 每單位的預算，新台幣元。
  @$pb.TagNumber(5)
  $fixnum.Int64 get budgetPerUnit => $_getI64(4);
  @$pb.TagNumber(5)
  set budgetPerUnit($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasBudgetPerUnit() => $_has(4);
  @$pb.TagNumber(5)
  void clearBudgetPerUnit() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get deliveryCity => $_getSZ(5);
  @$pb.TagNumber(6)
  set deliveryCity($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDeliveryCity() => $_has(5);
  @$pb.TagNumber(6)
  void clearDeliveryCity() => $_clearField(6);

  @$pb.TagNumber(7)
  $0.Date get deliveryDate => $_getN(6);
  @$pb.TagNumber(7)
  set deliveryDate($0.Date value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasDeliveryDate() => $_has(6);
  @$pb.TagNumber(7)
  void clearDeliveryDate() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.Date ensureDeliveryDate() => $_ensure(6);

  @$pb.TagNumber(8)
  $core.bool get exactMatchRequired => $_getBF(7);
  @$pb.TagNumber(8)
  set exactMatchRequired($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasExactMatchRequired() => $_has(7);
  @$pb.TagNumber(8)
  void clearExactMatchRequired() => $_clearField(8);

  /// 尚未判定時不設。
  @$pb.TagNumber(9)
  ItemResolution get itemResolution => $_getN(8);
  @$pb.TagNumber(9)
  set itemResolution(ItemResolution value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasItemResolution() => $_has(8);
  @$pb.TagNumber(9)
  void clearItemResolution() => $_clearField(9);
  @$pb.TagNumber(9)
  ItemResolution ensureItemResolution() => $_ensure(8);
}

class ItemDisambiguationOption extends $pb.GeneratedMessage {
  factory ItemDisambiguationOption({
    $core.String? label,
  }) {
    final result = ItemDisambiguationOption._();
    if (label != null) result.label = label;
    return result;
  }

  ItemDisambiguationOption._();

  factory ItemDisambiguationOption.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ItemDisambiguationOption()..mergeFromBuffer(data, registry);
  factory ItemDisambiguationOption.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ItemDisambiguationOption()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ItemDisambiguationOption',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ItemDisambiguationOption.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'label')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ItemDisambiguationOption clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ItemDisambiguationOption copyWith(
          void Function(ItemDisambiguationOption) updates) =>
      super.copyWith((message) => updates(message as ItemDisambiguationOption))
          as ItemDisambiguationOption;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ItemDisambiguationOption() / ItemDisambiguationOption.new instead')
  static ItemDisambiguationOption create() => ItemDisambiguationOption._();
  static $pb.GeneratedMessage $_createMessage() => ItemDisambiguationOption._();
  @$core.override
  ItemDisambiguationOption createEmptyInstance() =>
      ItemDisambiguationOption._();
  @$core.pragma('dart2js:noInline')
  static ItemDisambiguationOption getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ItemDisambiguationOption>(
          ItemDisambiguationOption.$_createMessage);
  static ItemDisambiguationOption? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get label => $_getSZ(0);
  @$pb.TagNumber(1)
  set label($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasLabel() => $_has(0);
  @$pb.TagNumber(1)
  void clearLabel() => $_clearField(1);
}

/// ItemDisambiguation 是品項有多個可能的分類時，可供買家選的分類。
class ItemDisambiguation extends $pb.GeneratedMessage {
  factory ItemDisambiguation({
    $core.Iterable<ItemDisambiguationOption>? candidates,
  }) {
    final result = ItemDisambiguation._();
    if (candidates != null) result.candidates.addAll(candidates);
    return result;
  }

  ItemDisambiguation._();

  factory ItemDisambiguation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ItemDisambiguation()..mergeFromBuffer(data, registry);
  factory ItemDisambiguation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ItemDisambiguation()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ItemDisambiguation',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ItemDisambiguation.$_createMessage)
    ..pPM<ItemDisambiguationOption>(1, _omitFieldNames ? '' : 'candidates',
        subBuilder: ItemDisambiguationOption.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ItemDisambiguation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ItemDisambiguation copyWith(void Function(ItemDisambiguation) updates) =>
      super.copyWith((message) => updates(message as ItemDisambiguation))
          as ItemDisambiguation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ItemDisambiguation() / ItemDisambiguation.new instead')
  static ItemDisambiguation create() => ItemDisambiguation._();
  static $pb.GeneratedMessage $_createMessage() => ItemDisambiguation._();
  @$core.override
  ItemDisambiguation createEmptyInstance() => ItemDisambiguation._();
  @$core.pragma('dart2js:noInline')
  static ItemDisambiguation getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ItemDisambiguation>(
          ItemDisambiguation.$_createMessage);
  static ItemDisambiguation? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ItemDisambiguationOption> get candidates => $_getList(0);
}

class Block extends $pb.GeneratedMessage {
  factory Block({
    BlockKind? kind,
    $core.String? text,
  }) {
    final result = Block._();
    if (kind != null) result.kind = kind;
    if (text != null) result.text = text;
    return result;
  }

  Block._();

  factory Block.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Block()..mergeFromBuffer(data, registry);
  factory Block.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Block()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Block',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: Block.$_createMessage)
    ..aE<BlockKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: BlockKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'text')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Block clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Block copyWith(void Function(Block) updates) =>
      super.copyWith((message) => updates(message as Block)) as Block;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Block() / Block.new instead')
  static Block create() => Block._();
  static $pb.GeneratedMessage $_createMessage() => Block._();
  @$core.override
  Block createEmptyInstance() => Block._();
  @$core.pragma('dart2js:noInline')
  static Block getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Block>(Block.$_createMessage);
  static Block? _defaultInstance;

  @$pb.TagNumber(1)
  BlockKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(BlockKind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get text => $_getSZ(1);
  @$pb.TagNumber(2)
  set text($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasText() => $_has(1);
  @$pb.TagNumber(2)
  void clearText() => $_clearField(2);
}

/// EvidenceRef 是輔助佐證的連結，只會是通過安全檢查的 https 網址。
class EvidenceRef extends $pb.GeneratedMessage {
  factory EvidenceRef({
    $core.String? url,
    $core.String? label,
  }) {
    final result = EvidenceRef._();
    if (url != null) result.url = url;
    if (label != null) result.label = label;
    return result;
  }

  EvidenceRef._();

  factory EvidenceRef.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EvidenceRef()..mergeFromBuffer(data, registry);
  factory EvidenceRef.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EvidenceRef()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EvidenceRef',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: EvidenceRef.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'url')
    ..aOS(2, _omitFieldNames ? '' : 'label')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EvidenceRef clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EvidenceRef copyWith(void Function(EvidenceRef) updates) =>
      super.copyWith((message) => updates(message as EvidenceRef))
          as EvidenceRef;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use EvidenceRef() / EvidenceRef.new instead')
  static EvidenceRef create() => EvidenceRef._();
  static $pb.GeneratedMessage $_createMessage() => EvidenceRef._();
  @$core.override
  EvidenceRef createEmptyInstance() => EvidenceRef._();
  @$core.pragma('dart2js:noInline')
  static EvidenceRef getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<EvidenceRef>(
          EvidenceRef.$_createMessage);
  static EvidenceRef? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get url => $_getSZ(0);
  @$pb.TagNumber(1)
  set url($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUrl() => $_has(0);
  @$pb.TagNumber(1)
  void clearUrl() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get label => $_getSZ(1);
  @$pb.TagNumber(2)
  set label($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLabel() => $_has(1);
  @$pb.TagNumber(2)
  void clearLabel() => $_clearField(2);
}

class Interrupt extends $pb.GeneratedMessage {
  factory Interrupt({
    $core.String? interruptId,
    InterruptKind? kind,
  }) {
    final result = Interrupt._();
    if (interruptId != null) result.interruptId = interruptId;
    if (kind != null) result.kind = kind;
    return result;
  }

  Interrupt._();

  factory Interrupt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Interrupt()..mergeFromBuffer(data, registry);
  factory Interrupt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Interrupt()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Interrupt',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: Interrupt.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'interruptId')
    ..aE<InterruptKind>(2, _omitFieldNames ? '' : 'kind',
        enumValues: InterruptKind.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Interrupt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Interrupt copyWith(void Function(Interrupt) updates) =>
      super.copyWith((message) => updates(message as Interrupt)) as Interrupt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Interrupt() / Interrupt.new instead')
  static Interrupt create() => Interrupt._();
  static $pb.GeneratedMessage $_createMessage() => Interrupt._();
  @$core.override
  Interrupt createEmptyInstance() => Interrupt._();
  @$core.pragma('dart2js:noInline')
  static Interrupt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Interrupt>(Interrupt.$_createMessage);
  static Interrupt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get interruptId => $_getSZ(0);
  @$pb.TagNumber(1)
  set interruptId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInterruptId() => $_has(0);
  @$pb.TagNumber(1)
  void clearInterruptId() => $_clearField(1);

  @$pb.TagNumber(2)
  InterruptKind get kind => $_getN(1);
  @$pb.TagNumber(2)
  set kind(InterruptKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);
}

class QuotaSnapshot extends $pb.GeneratedMessage {
  factory QuotaSnapshot({
    $core.String? planCode,
    $core.int? remaining,
    $core.int? limit,
    $1.Timestamp? resetTime,
  }) {
    final result = QuotaSnapshot._();
    if (planCode != null) result.planCode = planCode;
    if (remaining != null) result.remaining = remaining;
    if (limit != null) result.limit = limit;
    if (resetTime != null) result.resetTime = resetTime;
    return result;
  }

  QuotaSnapshot._();

  factory QuotaSnapshot.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      QuotaSnapshot()..mergeFromBuffer(data, registry);
  factory QuotaSnapshot.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      QuotaSnapshot()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'QuotaSnapshot',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: QuotaSnapshot.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'planCode')
    ..aI(2, _omitFieldNames ? '' : 'remaining')
    ..aI(3, _omitFieldNames ? '' : 'limit')
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'resetTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QuotaSnapshot clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QuotaSnapshot copyWith(void Function(QuotaSnapshot) updates) =>
      super.copyWith((message) => updates(message as QuotaSnapshot))
          as QuotaSnapshot;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use QuotaSnapshot() / QuotaSnapshot.new instead')
  static QuotaSnapshot create() => QuotaSnapshot._();
  static $pb.GeneratedMessage $_createMessage() => QuotaSnapshot._();
  @$core.override
  QuotaSnapshot createEmptyInstance() => QuotaSnapshot._();
  @$core.pragma('dart2js:noInline')
  static QuotaSnapshot getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<QuotaSnapshot>(
          QuotaSnapshot.$_createMessage);
  static QuotaSnapshot? _defaultInstance;

  /// 買家的會員方案代碼。
  @$pb.TagNumber(1)
  $core.String get planCode => $_getSZ(0);
  @$pb.TagNumber(1)
  set planCode($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPlanCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearPlanCode() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get remaining => $_getIZ(1);
  @$pb.TagNumber(2)
  set remaining($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRemaining() => $_has(1);
  @$pb.TagNumber(2)
  void clearRemaining() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get limit => $_getIZ(2);
  @$pb.TagNumber(3)
  set limit($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLimit() => $_has(2);
  @$pb.TagNumber(3)
  void clearLimit() => $_clearField(3);

  /// 下一期開始的時間，額度在這時重置。
  @$pb.TagNumber(4)
  $1.Timestamp get resetTime => $_getN(3);
  @$pb.TagNumber(4)
  set resetTime($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasResetTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearResetTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureResetTime() => $_ensure(3);
}

/// TurnResult 是一輪釐清結束時的需求狀態。
class TurnResult extends $pb.GeneratedMessage {
  factory TurnResult({
    $core.String? requestId,
    $core.String? conversationId,
    $core.String? revisionId,
    Slots? slots,
    $core.Iterable<RejectedField>? rejected,
    $core.Iterable<SlotKey>? missingRequired,
    $core.Iterable<RecordedField>? recordedThisTurn,
    $core.bool? degraded,
    $core.String? degradedReason,
    $core.String? reply,
    $core.Iterable<Step>? steps,
    ClarifyQuestion? question,
    $core.Iterable<Block>? blocks,
    $core.String? textFallback,
    $core.Iterable<EvidenceRef>? evidenceRefs,
    $core.String? messageId,
    $1.Timestamp? createTime,
    $core.String? runId,
    Interrupt? interrupt,
    QuotaSnapshot? quota,
  }) {
    final result = TurnResult._();
    if (requestId != null) result.requestId = requestId;
    if (conversationId != null) result.conversationId = conversationId;
    if (revisionId != null) result.revisionId = revisionId;
    if (slots != null) result.slots = slots;
    if (rejected != null) result.rejected.addAll(rejected);
    if (missingRequired != null) result.missingRequired.addAll(missingRequired);
    if (recordedThisTurn != null)
      result.recordedThisTurn.addAll(recordedThisTurn);
    if (degraded != null) result.degraded = degraded;
    if (degradedReason != null) result.degradedReason = degradedReason;
    if (reply != null) result.reply = reply;
    if (steps != null) result.steps.addAll(steps);
    if (question != null) result.question = question;
    if (blocks != null) result.blocks.addAll(blocks);
    if (textFallback != null) result.textFallback = textFallback;
    if (evidenceRefs != null) result.evidenceRefs.addAll(evidenceRefs);
    if (messageId != null) result.messageId = messageId;
    if (createTime != null) result.createTime = createTime;
    if (runId != null) result.runId = runId;
    if (interrupt != null) result.interrupt = interrupt;
    if (quota != null) result.quota = quota;
    return result;
  }

  TurnResult._();

  factory TurnResult.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnResult()..mergeFromBuffer(data, registry);
  factory TurnResult.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnResult()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnResult',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: TurnResult.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'conversationId')
    ..aOS(3, _omitFieldNames ? '' : 'revisionId')
    ..aOM<Slots>(4, _omitFieldNames ? '' : 'slots',
        subBuilder: Slots.$_createMessage)
    ..pPM<RejectedField>(5, _omitFieldNames ? '' : 'rejected',
        subBuilder: RejectedField.$_createMessage)
    ..pc<SlotKey>(
        6, _omitFieldNames ? '' : 'missingRequired', $pb.PbFieldType.KE,
        valueOf: SlotKey.valueOf,
        enumValues: SlotKey.values,
        defaultEnumValue: SlotKey.SLOT_KEY_UNSPECIFIED)
    ..pPM<RecordedField>(7, _omitFieldNames ? '' : 'recordedThisTurn',
        subBuilder: RecordedField.$_createMessage)
    ..aOB(8, _omitFieldNames ? '' : 'degraded')
    ..aOS(9, _omitFieldNames ? '' : 'degradedReason')
    ..aOS(10, _omitFieldNames ? '' : 'reply')
    ..pPM<Step>(11, _omitFieldNames ? '' : 'steps',
        subBuilder: Step.$_createMessage)
    ..aOM<ClarifyQuestion>(12, _omitFieldNames ? '' : 'question',
        subBuilder: ClarifyQuestion.$_createMessage)
    ..pPM<Block>(13, _omitFieldNames ? '' : 'blocks',
        subBuilder: Block.$_createMessage)
    ..aOS(14, _omitFieldNames ? '' : 'textFallback')
    ..pPM<EvidenceRef>(15, _omitFieldNames ? '' : 'evidenceRefs',
        subBuilder: EvidenceRef.$_createMessage)
    ..aOS(16, _omitFieldNames ? '' : 'messageId')
    ..aOM<$1.Timestamp>(17, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOS(18, _omitFieldNames ? '' : 'runId')
    ..aOM<Interrupt>(19, _omitFieldNames ? '' : 'interrupt',
        subBuilder: Interrupt.$_createMessage)
    ..aOM<QuotaSnapshot>(20, _omitFieldNames ? '' : 'quota',
        subBuilder: QuotaSnapshot.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnResult clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnResult copyWith(void Function(TurnResult) updates) =>
      super.copyWith((message) => updates(message as TurnResult)) as TurnResult;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TurnResult() / TurnResult.new instead')
  static TurnResult create() => TurnResult._();
  static $pb.GeneratedMessage $_createMessage() => TurnResult._();
  @$core.override
  TurnResult createEmptyInstance() => TurnResult._();
  @$core.pragma('dart2js:noInline')
  static TurnResult getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TurnResult>(TurnResult.$_createMessage);
  static TurnResult? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get conversationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set conversationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConversationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearConversationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get revisionId => $_getSZ(2);
  @$pb.TagNumber(3)
  set revisionId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRevisionId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRevisionId() => $_clearField(3);

  @$pb.TagNumber(4)
  Slots get slots => $_getN(3);
  @$pb.TagNumber(4)
  set slots(Slots value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSlots() => $_has(3);
  @$pb.TagNumber(4)
  void clearSlots() => $_clearField(4);
  @$pb.TagNumber(4)
  Slots ensureSlots() => $_ensure(3);

  @$pb.TagNumber(5)
  $pb.PbList<RejectedField> get rejected => $_getList(4);

  @$pb.TagNumber(6)
  $pb.PbList<SlotKey> get missingRequired => $_getList(5);

  @$pb.TagNumber(7)
  $pb.PbList<RecordedField> get recordedThisTurn => $_getList(6);

  @$pb.TagNumber(8)
  $core.bool get degraded => $_getBF(7);
  @$pb.TagNumber(8)
  set degraded($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasDegraded() => $_has(7);
  @$pb.TagNumber(8)
  void clearDegraded() => $_clearField(8);

  /// 降級的原因，有界的代碼；degraded 為 false 時是空字串。
  @$pb.TagNumber(9)
  $core.String get degradedReason => $_getSZ(8);
  @$pb.TagNumber(9)
  set degradedReason($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasDegradedReason() => $_has(8);
  @$pb.TagNumber(9)
  void clearDegradedReason() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get reply => $_getSZ(9);
  @$pb.TagNumber(10)
  set reply($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasReply() => $_has(9);
  @$pb.TagNumber(10)
  void clearReply() => $_clearField(10);

  @$pb.TagNumber(11)
  $pb.PbList<Step> get steps => $_getList(10);

  /// 待買家回答的問題；沒有時不設。
  @$pb.TagNumber(12)
  ClarifyQuestion get question => $_getN(11);
  @$pb.TagNumber(12)
  set question(ClarifyQuestion value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasQuestion() => $_has(11);
  @$pb.TagNumber(12)
  void clearQuestion() => $_clearField(12);
  @$pb.TagNumber(12)
  ClarifyQuestion ensureQuestion() => $_ensure(11);

  @$pb.TagNumber(13)
  $pb.PbList<Block> get blocks => $_getList(12);

  @$pb.TagNumber(14)
  $core.String get textFallback => $_getSZ(13);
  @$pb.TagNumber(14)
  set textFallback($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasTextFallback() => $_has(13);
  @$pb.TagNumber(14)
  void clearTextFallback() => $_clearField(14);

  @$pb.TagNumber(15)
  $pb.PbList<EvidenceRef> get evidenceRefs => $_getList(14);

  @$pb.TagNumber(16)
  $core.String get messageId => $_getSZ(15);
  @$pb.TagNumber(16)
  set messageId($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasMessageId() => $_has(15);
  @$pb.TagNumber(16)
  void clearMessageId() => $_clearField(16);

  @$pb.TagNumber(17)
  $1.Timestamp get createTime => $_getN(16);
  @$pb.TagNumber(17)
  set createTime($1.Timestamp value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasCreateTime() => $_has(16);
  @$pb.TagNumber(17)
  void clearCreateTime() => $_clearField(17);
  @$pb.TagNumber(17)
  $1.Timestamp ensureCreateTime() => $_ensure(16);

  @$pb.TagNumber(18)
  $core.String get runId => $_getSZ(17);
  @$pb.TagNumber(18)
  set runId($core.String value) => $_setString(17, value);
  @$pb.TagNumber(18)
  $core.bool hasRunId() => $_has(17);
  @$pb.TagNumber(18)
  void clearRunId() => $_clearField(18);

  /// 這一輪停在等待輸入時設定；client 不把它當成串流中斷。
  @$pb.TagNumber(19)
  Interrupt get interrupt => $_getN(18);
  @$pb.TagNumber(19)
  set interrupt(Interrupt value) => $_setField(19, value);
  @$pb.TagNumber(19)
  $core.bool hasInterrupt() => $_has(18);
  @$pb.TagNumber(19)
  void clearInterrupt() => $_clearField(19);
  @$pb.TagNumber(19)
  Interrupt ensureInterrupt() => $_ensure(18);

  @$pb.TagNumber(20)
  QuotaSnapshot get quota => $_getN(19);
  @$pb.TagNumber(20)
  set quota(QuotaSnapshot value) => $_setField(20, value);
  @$pb.TagNumber(20)
  $core.bool hasQuota() => $_has(19);
  @$pb.TagNumber(20)
  void clearQuota() => $_clearField(20);
  @$pb.TagNumber(20)
  QuotaSnapshot ensureQuota() => $_ensure(19);
}

class RunSummary extends $pb.GeneratedMessage {
  factory RunSummary({
    $core.String? runId,
    RunType? runType,
    RunStatus? status,
    $core.String? lastCursor,
    $core.bool? terminal,
  }) {
    final result = RunSummary._();
    if (runId != null) result.runId = runId;
    if (runType != null) result.runType = runType;
    if (status != null) result.status = status;
    if (lastCursor != null) result.lastCursor = lastCursor;
    if (terminal != null) result.terminal = terminal;
    return result;
  }

  RunSummary._();

  factory RunSummary.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RunSummary()..mergeFromBuffer(data, registry);
  factory RunSummary.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RunSummary()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RunSummary',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RunSummary.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'runId')
    ..aE<RunType>(2, _omitFieldNames ? '' : 'runType',
        enumValues: RunType.values)
    ..aE<RunStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: RunStatus.values)
    ..aOS(4, _omitFieldNames ? '' : 'lastCursor')
    ..aOB(5, _omitFieldNames ? '' : 'terminal')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunSummary clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunSummary copyWith(void Function(RunSummary) updates) =>
      super.copyWith((message) => updates(message as RunSummary)) as RunSummary;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RunSummary() / RunSummary.new instead')
  static RunSummary create() => RunSummary._();
  static $pb.GeneratedMessage $_createMessage() => RunSummary._();
  @$core.override
  RunSummary createEmptyInstance() => RunSummary._();
  @$core.pragma('dart2js:noInline')
  static RunSummary getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RunSummary>(RunSummary.$_createMessage);
  static RunSummary? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get runId => $_getSZ(0);
  @$pb.TagNumber(1)
  set runId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunId() => $_clearField(1);

  @$pb.TagNumber(2)
  RunType get runType => $_getN(1);
  @$pb.TagNumber(2)
  set runType(RunType value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasRunType() => $_has(1);
  @$pb.TagNumber(2)
  void clearRunType() => $_clearField(2);

  @$pb.TagNumber(3)
  RunStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(RunStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  /// 這個 run 最新事件的 cursor；還沒有事件時是空字串。
  @$pb.TagNumber(4)
  $core.String get lastCursor => $_getSZ(3);
  @$pb.TagNumber(4)
  set lastCursor($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasLastCursor() => $_has(3);
  @$pb.TagNumber(4)
  void clearLastCursor() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get terminal => $_getBF(4);
  @$pb.TagNumber(5)
  set terminal($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTerminal() => $_has(4);
  @$pb.TagNumber(5)
  void clearTerminal() => $_clearField(5);
}

class OpenConfirmation extends $pb.GeneratedMessage {
  factory OpenConfirmation({
    $core.String? interruptId,
    InterruptKind? kind,
    $1.Timestamp? expireTime,
    ClarifyQuestion? question,
  }) {
    final result = OpenConfirmation._();
    if (interruptId != null) result.interruptId = interruptId;
    if (kind != null) result.kind = kind;
    if (expireTime != null) result.expireTime = expireTime;
    if (question != null) result.question = question;
    return result;
  }

  OpenConfirmation._();

  factory OpenConfirmation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OpenConfirmation()..mergeFromBuffer(data, registry);
  factory OpenConfirmation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OpenConfirmation()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OpenConfirmation',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: OpenConfirmation.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'interruptId')
    ..aE<InterruptKind>(2, _omitFieldNames ? '' : 'kind',
        enumValues: InterruptKind.values)
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'expireTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<ClarifyQuestion>(4, _omitFieldNames ? '' : 'question',
        subBuilder: ClarifyQuestion.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OpenConfirmation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OpenConfirmation copyWith(void Function(OpenConfirmation) updates) =>
      super.copyWith((message) => updates(message as OpenConfirmation))
          as OpenConfirmation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use OpenConfirmation() / OpenConfirmation.new instead')
  static OpenConfirmation create() => OpenConfirmation._();
  static $pb.GeneratedMessage $_createMessage() => OpenConfirmation._();
  @$core.override
  OpenConfirmation createEmptyInstance() => OpenConfirmation._();
  @$core.pragma('dart2js:noInline')
  static OpenConfirmation getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<OpenConfirmation>(
          OpenConfirmation.$_createMessage);
  static OpenConfirmation? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get interruptId => $_getSZ(0);
  @$pb.TagNumber(1)
  set interruptId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInterruptId() => $_has(0);
  @$pb.TagNumber(1)
  void clearInterruptId() => $_clearField(1);

  @$pb.TagNumber(2)
  InterruptKind get kind => $_getN(1);
  @$pb.TagNumber(2)
  set kind(InterruptKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);

  /// 沒有設定表示不會到期。
  @$pb.TagNumber(3)
  $1.Timestamp get expireTime => $_getN(2);
  @$pb.TagNumber(3)
  set expireTime($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasExpireTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpireTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureExpireTime() => $_ensure(2);

  /// 待回答的問題，讓重新載入之後能還原同一個畫面。
  @$pb.TagNumber(4)
  ClarifyQuestion get question => $_getN(3);
  @$pb.TagNumber(4)
  set question(ClarifyQuestion value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasQuestion() => $_has(3);
  @$pb.TagNumber(4)
  void clearQuestion() => $_clearField(4);
  @$pb.TagNumber(4)
  ClarifyQuestion ensureQuestion() => $_ensure(3);
}

/// PublishedRfq 是已成立的詢價發布結果。
class PublishedRfq extends $pb.GeneratedMessage {
  factory PublishedRfq({
    $core.String? publicationId,
    $core.String? version,
    $1.Timestamp? publishTime,
    $core.int? recipientCount,
    $core.int? queuedCount,
    $core.String? confirmedRfqSnapshotId,
    $core.String? rfqSnapshotHash,
  }) {
    final result = PublishedRfq._();
    if (publicationId != null) result.publicationId = publicationId;
    if (version != null) result.version = version;
    if (publishTime != null) result.publishTime = publishTime;
    if (recipientCount != null) result.recipientCount = recipientCount;
    if (queuedCount != null) result.queuedCount = queuedCount;
    if (confirmedRfqSnapshotId != null)
      result.confirmedRfqSnapshotId = confirmedRfqSnapshotId;
    if (rfqSnapshotHash != null) result.rfqSnapshotHash = rfqSnapshotHash;
    return result;
  }

  PublishedRfq._();

  factory PublishedRfq.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishedRfq()..mergeFromBuffer(data, registry);
  factory PublishedRfq.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishedRfq()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishedRfq',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: PublishedRfq.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'publicationId')
    ..aOS(2, _omitFieldNames ? '' : 'version')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aI(4, _omitFieldNames ? '' : 'recipientCount')
    ..aI(5, _omitFieldNames ? '' : 'queuedCount')
    ..aOS(6, _omitFieldNames ? '' : 'confirmedRfqSnapshotId')
    ..aOS(7, _omitFieldNames ? '' : 'rfqSnapshotHash')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishedRfq clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishedRfq copyWith(void Function(PublishedRfq) updates) =>
      super.copyWith((message) => updates(message as PublishedRfq))
          as PublishedRfq;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PublishedRfq() / PublishedRfq.new instead')
  static PublishedRfq create() => PublishedRfq._();
  static $pb.GeneratedMessage $_createMessage() => PublishedRfq._();
  @$core.override
  PublishedRfq createEmptyInstance() => PublishedRfq._();
  @$core.pragma('dart2js:noInline')
  static PublishedRfq getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PublishedRfq>(
          PublishedRfq.$_createMessage);
  static PublishedRfq? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get publicationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set publicationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPublicationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublicationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get version => $_getSZ(1);
  @$pb.TagNumber(2)
  set version($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  $1.Timestamp get publishTime => $_getN(2);
  @$pb.TagNumber(3)
  set publishTime($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPublishTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearPublishTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensurePublishTime() => $_ensure(2);

  /// 發布的對象數；公開池是 0。
  @$pb.TagNumber(4)
  $core.int get recipientCount => $_getIZ(3);
  @$pb.TagNumber(4)
  set recipientCount($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRecipientCount() => $_has(3);
  @$pb.TagNumber(4)
  void clearRecipientCount() => $_clearField(4);

  /// 已排入投遞的通知數；公開池是 0。
  @$pb.TagNumber(5)
  $core.int get queuedCount => $_getIZ(4);
  @$pb.TagNumber(5)
  set queuedCount($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasQueuedCount() => $_has(4);
  @$pb.TagNumber(5)
  void clearQueuedCount() => $_clearField(5);

  /// 這次發布依據的詢價確認快照（ConfirmRfqSnapshotResponse.confirmed_rfq_snapshot_id）。
  @$pb.TagNumber(6)
  $core.String get confirmedRfqSnapshotId => $_getSZ(5);
  @$pb.TagNumber(6)
  set confirmedRfqSnapshotId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasConfirmedRfqSnapshotId() => $_has(5);
  @$pb.TagNumber(6)
  void clearConfirmedRfqSnapshotId() => $_clearField(6);

  /// 該快照的 hash，與 ConfirmRfqSnapshotResponse.payload_hash 是同一個值，原樣帶進 AcceptSellerQuoteRequest.expected_rfq_snapshot_hash；
  /// 讓買家重新載入之後仍能接受報價。
  @$pb.TagNumber(7)
  $core.String get rfqSnapshotHash => $_getSZ(6);
  @$pb.TagNumber(7)
  set rfqSnapshotHash($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRfqSnapshotHash() => $_has(6);
  @$pb.TagNumber(7)
  void clearRfqSnapshotHash() => $_clearField(7);
}

class ConversationMessage extends $pb.GeneratedMessage {
  factory ConversationMessage({
    $core.String? id,
    ConversationRole? role,
    $core.String? text,
    $core.Iterable<Step>? steps,
    $core.Iterable<RecordedField>? recordedThisTurn,
    $1.Timestamp? createTime,
  }) {
    final result = ConversationMessage._();
    if (id != null) result.id = id;
    if (role != null) result.role = role;
    if (text != null) result.text = text;
    if (steps != null) result.steps.addAll(steps);
    if (recordedThisTurn != null)
      result.recordedThisTurn.addAll(recordedThisTurn);
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  ConversationMessage._();

  factory ConversationMessage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConversationMessage()..mergeFromBuffer(data, registry);
  factory ConversationMessage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConversationMessage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConversationMessage',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ConversationMessage.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<ConversationRole>(2, _omitFieldNames ? '' : 'role',
        enumValues: ConversationRole.values)
    ..aOS(3, _omitFieldNames ? '' : 'text')
    ..pPM<Step>(4, _omitFieldNames ? '' : 'steps',
        subBuilder: Step.$_createMessage)
    ..pPM<RecordedField>(5, _omitFieldNames ? '' : 'recordedThisTurn',
        subBuilder: RecordedField.$_createMessage)
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConversationMessage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConversationMessage copyWith(void Function(ConversationMessage) updates) =>
      super.copyWith((message) => updates(message as ConversationMessage))
          as ConversationMessage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ConversationMessage() / ConversationMessage.new instead')
  static ConversationMessage create() => ConversationMessage._();
  static $pb.GeneratedMessage $_createMessage() => ConversationMessage._();
  @$core.override
  ConversationMessage createEmptyInstance() => ConversationMessage._();
  @$core.pragma('dart2js:noInline')
  static ConversationMessage getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConversationMessage>(
          ConversationMessage.$_createMessage);
  static ConversationMessage? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  ConversationRole get role => $_getN(1);
  @$pb.TagNumber(2)
  set role(ConversationRole value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasRole() => $_has(1);
  @$pb.TagNumber(2)
  void clearRole() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get text => $_getSZ(2);
  @$pb.TagNumber(3)
  set text($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasText() => $_has(2);
  @$pb.TagNumber(3)
  void clearText() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<Step> get steps => $_getList(3);

  @$pb.TagNumber(5)
  $pb.PbList<RecordedField> get recordedThisTurn => $_getList(4);

  @$pb.TagNumber(6)
  $1.Timestamp get createTime => $_getN(5);
  @$pb.TagNumber(6)
  set createTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCreateTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearCreateTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureCreateTime() => $_ensure(5);
}

class ConversationSummary extends $pb.GeneratedMessage {
  factory ConversationSummary({
    $core.String? id,
    $core.String? title,
    $core.String? lastMessagePreview,
    $1.Timestamp? updateTime,
  }) {
    final result = ConversationSummary._();
    if (id != null) result.id = id;
    if (title != null) result.title = title;
    if (lastMessagePreview != null)
      result.lastMessagePreview = lastMessagePreview;
    if (updateTime != null) result.updateTime = updateTime;
    return result;
  }

  ConversationSummary._();

  factory ConversationSummary.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConversationSummary()..mergeFromBuffer(data, registry);
  factory ConversationSummary.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConversationSummary()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConversationSummary',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ConversationSummary.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..aOS(3, _omitFieldNames ? '' : 'lastMessagePreview')
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConversationSummary clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConversationSummary copyWith(void Function(ConversationSummary) updates) =>
      super.copyWith((message) => updates(message as ConversationSummary))
          as ConversationSummary;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ConversationSummary() / ConversationSummary.new instead')
  static ConversationSummary create() => ConversationSummary._();
  static $pb.GeneratedMessage $_createMessage() => ConversationSummary._();
  @$core.override
  ConversationSummary createEmptyInstance() => ConversationSummary._();
  @$core.pragma('dart2js:noInline')
  static ConversationSummary getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConversationSummary>(
          ConversationSummary.$_createMessage);
  static ConversationSummary? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get lastMessagePreview => $_getSZ(2);
  @$pb.TagNumber(3)
  set lastMessagePreview($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLastMessagePreview() => $_has(2);
  @$pb.TagNumber(3)
  void clearLastMessagePreview() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Timestamp get updateTime => $_getN(3);
  @$pb.TagNumber(4)
  set updateTime($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasUpdateTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearUpdateTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureUpdateTime() => $_ensure(3);
}

class SubmitTurnRequest extends $pb.GeneratedMessage {
  factory SubmitTurnRequest({
    $core.String? requestId,
    $core.String? message,
    $core.String? clientRequestId,
  }) {
    final result = SubmitTurnRequest._();
    if (requestId != null) result.requestId = requestId;
    if (message != null) result.message = message;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    return result;
  }

  SubmitTurnRequest._();

  factory SubmitTurnRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitTurnRequest()..mergeFromBuffer(data, registry);
  factory SubmitTurnRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitTurnRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitTurnRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SubmitTurnRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'message')
    ..aOS(3, _omitFieldNames ? '' : 'clientRequestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitTurnRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitTurnRequest copyWith(void Function(SubmitTurnRequest) updates) =>
      super.copyWith((message) => updates(message as SubmitTurnRequest))
          as SubmitTurnRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SubmitTurnRequest() / SubmitTurnRequest.new instead')
  static SubmitTurnRequest create() => SubmitTurnRequest._();
  static $pb.GeneratedMessage $_createMessage() => SubmitTurnRequest._();
  @$core.override
  SubmitTurnRequest createEmptyInstance() => SubmitTurnRequest._();
  @$core.pragma('dart2js:noInline')
  static SubmitTurnRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SubmitTurnRequest>(
          SubmitTurnRequest.$_createMessage);
  static SubmitTurnRequest? _defaultInstance;

  /// 要接續的需求；空字串表示新建。
  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  /// 買家的訊息，最多 2000 個字元。
  @$pb.TagNumber(2)
  $core.String get message => $_getSZ(1);
  @$pb.TagNumber(2)
  set message($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessage() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get clientRequestId => $_getSZ(2);
  @$pb.TagNumber(3)
  set clientRequestId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasClientRequestId() => $_has(2);
  @$pb.TagNumber(3)
  void clearClientRequestId() => $_clearField(3);
}

class SubmitTurnResponse extends $pb.GeneratedMessage {
  factory SubmitTurnResponse({
    $core.String? requestId,
    $core.String? conversationId,
    $core.String? runId,
  }) {
    final result = SubmitTurnResponse._();
    if (requestId != null) result.requestId = requestId;
    if (conversationId != null) result.conversationId = conversationId;
    if (runId != null) result.runId = runId;
    return result;
  }

  SubmitTurnResponse._();

  factory SubmitTurnResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitTurnResponse()..mergeFromBuffer(data, registry);
  factory SubmitTurnResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitTurnResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitTurnResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SubmitTurnResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'conversationId')
    ..aOS(3, _omitFieldNames ? '' : 'runId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitTurnResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitTurnResponse copyWith(void Function(SubmitTurnResponse) updates) =>
      super.copyWith((message) => updates(message as SubmitTurnResponse))
          as SubmitTurnResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SubmitTurnResponse() / SubmitTurnResponse.new instead')
  static SubmitTurnResponse create() => SubmitTurnResponse._();
  static $pb.GeneratedMessage $_createMessage() => SubmitTurnResponse._();
  @$core.override
  SubmitTurnResponse createEmptyInstance() => SubmitTurnResponse._();
  @$core.pragma('dart2js:noInline')
  static SubmitTurnResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitTurnResponse>(
          SubmitTurnResponse.$_createMessage);
  static SubmitTurnResponse? _defaultInstance;

  /// 需求的 ID；新建時是新的。
  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get conversationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set conversationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConversationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearConversationId() => $_clearField(2);

  /// 這一輪的 run，傳給 WatchRun。
  @$pb.TagNumber(3)
  $core.String get runId => $_getSZ(2);
  @$pb.TagNumber(3)
  set runId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRunId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRunId() => $_clearField(3);
}

class GetSnapshotRequest extends $pb.GeneratedMessage {
  factory GetSnapshotRequest({
    $core.String? requestId,
  }) {
    final result = GetSnapshotRequest._();
    if (requestId != null) result.requestId = requestId;
    return result;
  }

  GetSnapshotRequest._();

  factory GetSnapshotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSnapshotRequest()..mergeFromBuffer(data, registry);
  factory GetSnapshotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSnapshotRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSnapshotRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetSnapshotRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSnapshotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSnapshotRequest copyWith(void Function(GetSnapshotRequest) updates) =>
      super.copyWith((message) => updates(message as GetSnapshotRequest))
          as GetSnapshotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetSnapshotRequest() / GetSnapshotRequest.new instead')
  static GetSnapshotRequest create() => GetSnapshotRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetSnapshotRequest._();
  @$core.override
  GetSnapshotRequest createEmptyInstance() => GetSnapshotRequest._();
  @$core.pragma('dart2js:noInline')
  static GetSnapshotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSnapshotRequest>(
          GetSnapshotRequest.$_createMessage);
  static GetSnapshotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);
}

class GetSnapshotResponse extends $pb.GeneratedMessage {
  factory GetSnapshotResponse({
    $core.String? requestId,
    $core.String? revisionId,
    RequestStatus? status,
    Slots? slots,
    $core.Iterable<SlotKey>? missingRequired,
    ItemDisambiguation? itemDisambiguation,
    $core.Iterable<RunSummary>? runs,
    $core.String? activeRunId,
    OpenConfirmation? openConfirmation,
    PublishedRfq? publishedRfq,
  }) {
    final result = GetSnapshotResponse._();
    if (requestId != null) result.requestId = requestId;
    if (revisionId != null) result.revisionId = revisionId;
    if (status != null) result.status = status;
    if (slots != null) result.slots = slots;
    if (missingRequired != null) result.missingRequired.addAll(missingRequired);
    if (itemDisambiguation != null)
      result.itemDisambiguation = itemDisambiguation;
    if (runs != null) result.runs.addAll(runs);
    if (activeRunId != null) result.activeRunId = activeRunId;
    if (openConfirmation != null) result.openConfirmation = openConfirmation;
    if (publishedRfq != null) result.publishedRfq = publishedRfq;
    return result;
  }

  GetSnapshotResponse._();

  factory GetSnapshotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSnapshotResponse()..mergeFromBuffer(data, registry);
  factory GetSnapshotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSnapshotResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSnapshotResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetSnapshotResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'revisionId')
    ..aE<RequestStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: RequestStatus.values)
    ..aOM<Slots>(4, _omitFieldNames ? '' : 'slots',
        subBuilder: Slots.$_createMessage)
    ..pc<SlotKey>(
        5, _omitFieldNames ? '' : 'missingRequired', $pb.PbFieldType.KE,
        valueOf: SlotKey.valueOf,
        enumValues: SlotKey.values,
        defaultEnumValue: SlotKey.SLOT_KEY_UNSPECIFIED)
    ..aOM<ItemDisambiguation>(6, _omitFieldNames ? '' : 'itemDisambiguation',
        subBuilder: ItemDisambiguation.$_createMessage)
    ..pPM<RunSummary>(7, _omitFieldNames ? '' : 'runs',
        subBuilder: RunSummary.$_createMessage)
    ..aOS(8, _omitFieldNames ? '' : 'activeRunId')
    ..aOM<OpenConfirmation>(9, _omitFieldNames ? '' : 'openConfirmation',
        subBuilder: OpenConfirmation.$_createMessage)
    ..aOM<PublishedRfq>(10, _omitFieldNames ? '' : 'publishedRfq',
        subBuilder: PublishedRfq.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSnapshotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSnapshotResponse copyWith(void Function(GetSnapshotResponse) updates) =>
      super.copyWith((message) => updates(message as GetSnapshotResponse))
          as GetSnapshotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use GetSnapshotResponse() / GetSnapshotResponse.new instead')
  static GetSnapshotResponse create() => GetSnapshotResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetSnapshotResponse._();
  @$core.override
  GetSnapshotResponse createEmptyInstance() => GetSnapshotResponse._();
  @$core.pragma('dart2js:noInline')
  static GetSnapshotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSnapshotResponse>(
          GetSnapshotResponse.$_createMessage);
  static GetSnapshotResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get revisionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set revisionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRevisionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRevisionId() => $_clearField(2);

  @$pb.TagNumber(3)
  RequestStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(RequestStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  Slots get slots => $_getN(3);
  @$pb.TagNumber(4)
  set slots(Slots value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSlots() => $_has(3);
  @$pb.TagNumber(4)
  void clearSlots() => $_clearField(4);
  @$pb.TagNumber(4)
  Slots ensureSlots() => $_ensure(3);

  @$pb.TagNumber(5)
  $pb.PbList<SlotKey> get missingRequired => $_getList(4);

  /// 品項有多個可能的分類，等買家選；沒有時不設。
  @$pb.TagNumber(6)
  ItemDisambiguation get itemDisambiguation => $_getN(5);
  @$pb.TagNumber(6)
  set itemDisambiguation(ItemDisambiguation value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasItemDisambiguation() => $_has(5);
  @$pb.TagNumber(6)
  void clearItemDisambiguation() => $_clearField(6);
  @$pb.TagNumber(6)
  ItemDisambiguation ensureItemDisambiguation() => $_ensure(5);

  @$pb.TagNumber(7)
  $pb.PbList<RunSummary> get runs => $_getList(6);

  /// 進行中的 run；沒有時是空字串。
  @$pb.TagNumber(8)
  $core.String get activeRunId => $_getSZ(7);
  @$pb.TagNumber(8)
  set activeRunId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasActiveRunId() => $_has(7);
  @$pb.TagNumber(8)
  void clearActiveRunId() => $_clearField(8);

  /// 只在進行中的 run 停在等待輸入時設定。
  @$pb.TagNumber(9)
  OpenConfirmation get openConfirmation => $_getN(8);
  @$pb.TagNumber(9)
  set openConfirmation(OpenConfirmation value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasOpenConfirmation() => $_has(8);
  @$pb.TagNumber(9)
  void clearOpenConfirmation() => $_clearField(9);
  @$pb.TagNumber(9)
  OpenConfirmation ensureOpenConfirmation() => $_ensure(8);

  @$pb.TagNumber(10)
  PublishedRfq get publishedRfq => $_getN(9);
  @$pb.TagNumber(10)
  set publishedRfq(PublishedRfq value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasPublishedRfq() => $_has(9);
  @$pb.TagNumber(10)
  void clearPublishedRfq() => $_clearField(10);
  @$pb.TagNumber(10)
  PublishedRfq ensurePublishedRfq() => $_ensure(9);
}

class MatchedFact extends $pb.GeneratedMessage {
  factory MatchedFact({
    SlotKey? slotKey,
    $core.String? label,
  }) {
    final result = MatchedFact._();
    if (slotKey != null) result.slotKey = slotKey;
    if (label != null) result.label = label;
    return result;
  }

  MatchedFact._();

  factory MatchedFact.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MatchedFact()..mergeFromBuffer(data, registry);
  factory MatchedFact.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MatchedFact()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchedFact',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: MatchedFact.$_createMessage)
    ..aE<SlotKey>(1, _omitFieldNames ? '' : 'slotKey',
        enumValues: SlotKey.values)
    ..aOS(2, _omitFieldNames ? '' : 'label')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchedFact clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchedFact copyWith(void Function(MatchedFact) updates) =>
      super.copyWith((message) => updates(message as MatchedFact))
          as MatchedFact;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MatchedFact() / MatchedFact.new instead')
  static MatchedFact create() => MatchedFact._();
  static $pb.GeneratedMessage $_createMessage() => MatchedFact._();
  @$core.override
  MatchedFact createEmptyInstance() => MatchedFact._();
  @$core.pragma('dart2js:noInline')
  static MatchedFact getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MatchedFact>(
          MatchedFact.$_createMessage);
  static MatchedFact? _defaultInstance;

  @$pb.TagNumber(1)
  SlotKey get slotKey => $_getN(0);
  @$pb.TagNumber(1)
  set slotKey(SlotKey value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSlotKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlotKey() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get label => $_getSZ(1);
  @$pb.TagNumber(2)
  set label($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLabel() => $_has(1);
  @$pb.TagNumber(2)
  void clearLabel() => $_clearField(2);
}

class UnverifiableFact extends $pb.GeneratedMessage {
  factory UnverifiableFact({
    SlotKey? slotKey,
    $core.String? reason,
  }) {
    final result = UnverifiableFact._();
    if (slotKey != null) result.slotKey = slotKey;
    if (reason != null) result.reason = reason;
    return result;
  }

  UnverifiableFact._();

  factory UnverifiableFact.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnverifiableFact()..mergeFromBuffer(data, registry);
  factory UnverifiableFact.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnverifiableFact()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnverifiableFact',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: UnverifiableFact.$_createMessage)
    ..aE<SlotKey>(1, _omitFieldNames ? '' : 'slotKey',
        enumValues: SlotKey.values)
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnverifiableFact clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnverifiableFact copyWith(void Function(UnverifiableFact) updates) =>
      super.copyWith((message) => updates(message as UnverifiableFact))
          as UnverifiableFact;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UnverifiableFact() / UnverifiableFact.new instead')
  static UnverifiableFact create() => UnverifiableFact._();
  static $pb.GeneratedMessage $_createMessage() => UnverifiableFact._();
  @$core.override
  UnverifiableFact createEmptyInstance() => UnverifiableFact._();
  @$core.pragma('dart2js:noInline')
  static UnverifiableFact getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<UnverifiableFact>(
          UnverifiableFact.$_createMessage);
  static UnverifiableFact? _defaultInstance;

  @$pb.TagNumber(1)
  SlotKey get slotKey => $_getN(0);
  @$pb.TagNumber(1)
  set slotKey(SlotKey value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSlotKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearSlotKey() => $_clearField(1);

  /// 伺服器依買家填的值組出來的具體說明，例如需向賣家確認什麼。
  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

/// Candidate 是一位候選賣家：只有結構化欄位，不含研究摘要、定位等自由文字。
class Candidate extends $pb.GeneratedMessage {
  factory Candidate({
    $core.String? id,
    $core.double? score,
    $core.Iterable<MatchedFact>? matchedFacts,
    $core.Iterable<UnverifiableFact>? unverifiableFacts,
    $1.Timestamp? factsTime,
    $core.bool? selected,
    $2.StoreCard? store,
  }) {
    final result = Candidate._();
    if (id != null) result.id = id;
    if (score != null) result.score = score;
    if (matchedFacts != null) result.matchedFacts.addAll(matchedFacts);
    if (unverifiableFacts != null)
      result.unverifiableFacts.addAll(unverifiableFacts);
    if (factsTime != null) result.factsTime = factsTime;
    if (selected != null) result.selected = selected;
    if (store != null) result.store = store;
    return result;
  }

  Candidate._();

  factory Candidate.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Candidate()..mergeFromBuffer(data, registry);
  factory Candidate.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Candidate()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Candidate',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: Candidate.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aD(3, _omitFieldNames ? '' : 'score')
    ..pPM<MatchedFact>(4, _omitFieldNames ? '' : 'matchedFacts',
        subBuilder: MatchedFact.$_createMessage)
    ..pPM<UnverifiableFact>(5, _omitFieldNames ? '' : 'unverifiableFacts',
        subBuilder: UnverifiableFact.$_createMessage)
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'factsTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOB(7, _omitFieldNames ? '' : 'selected')
    ..aOM<$2.StoreCard>(8, _omitFieldNames ? '' : 'store',
        subBuilder: $2.StoreCard.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Candidate clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Candidate copyWith(void Function(Candidate) updates) =>
      super.copyWith((message) => updates(message as Candidate)) as Candidate;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Candidate() / Candidate.new instead')
  static Candidate create() => Candidate._();
  static $pb.GeneratedMessage $_createMessage() => Candidate._();
  @$core.override
  Candidate createEmptyInstance() => Candidate._();
  @$core.pragma('dart2js:noInline')
  static Candidate getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Candidate>(Candidate.$_createMessage);
  static Candidate? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 店名不在這裡：名稱只從 store（StoreCard）讀，沒公開過的店家沒有卡片、名稱不能被讀出。
  @$pb.TagNumber(3)
  $core.double get score => $_getN(1);
  @$pb.TagNumber(3)
  set score($core.double value) => $_setDouble(1, value);
  @$pb.TagNumber(3)
  $core.bool hasScore() => $_has(1);
  @$pb.TagNumber(3)
  void clearScore() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<MatchedFact> get matchedFacts => $_getList(2);

  @$pb.TagNumber(5)
  $pb.PbList<UnverifiableFact> get unverifiableFacts => $_getList(3);

  /// 以上事實成立的時間。
  @$pb.TagNumber(6)
  $1.Timestamp get factsTime => $_getN(4);
  @$pb.TagNumber(6)
  set factsTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasFactsTime() => $_has(4);
  @$pb.TagNumber(6)
  void clearFactsTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureFactsTime() => $_ensure(4);

  @$pb.TagNumber(7)
  $core.bool get selected => $_getBF(5);
  @$pb.TagNumber(7)
  set selected($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(7)
  $core.bool hasSelected() => $_has(5);
  @$pb.TagNumber(7)
  void clearSelected() => $_clearField(7);

  /// 賣家的店家卡片。
  @$pb.TagNumber(8)
  $2.StoreCard get store => $_getN(6);
  @$pb.TagNumber(8)
  set store($2.StoreCard value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasStore() => $_has(6);
  @$pb.TagNumber(8)
  void clearStore() => $_clearField(8);
  @$pb.TagNumber(8)
  $2.StoreCard ensureStore() => $_ensure(6);
}

class StartCandidateSearchRequest extends $pb.GeneratedMessage {
  factory StartCandidateSearchRequest({
    $core.String? requestId,
    $core.String? clientRequestId,
  }) {
    final result = StartCandidateSearchRequest._();
    if (requestId != null) result.requestId = requestId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    return result;
  }

  StartCandidateSearchRequest._();

  factory StartCandidateSearchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StartCandidateSearchRequest()..mergeFromBuffer(data, registry);
  factory StartCandidateSearchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StartCandidateSearchRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartCandidateSearchRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: StartCandidateSearchRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCandidateSearchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCandidateSearchRequest copyWith(
          void Function(StartCandidateSearchRequest) updates) =>
      super.copyWith(
              (message) => updates(message as StartCandidateSearchRequest))
          as StartCandidateSearchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use StartCandidateSearchRequest() / StartCandidateSearchRequest.new instead')
  static StartCandidateSearchRequest create() =>
      StartCandidateSearchRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      StartCandidateSearchRequest._();
  @$core.override
  StartCandidateSearchRequest createEmptyInstance() =>
      StartCandidateSearchRequest._();
  @$core.pragma('dart2js:noInline')
  static StartCandidateSearchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartCandidateSearchRequest>(
          StartCandidateSearchRequest.$_createMessage);
  static StartCandidateSearchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);
}

class StartCandidateSearchResponse extends $pb.GeneratedMessage {
  factory StartCandidateSearchResponse({
    $core.String? requestId,
    $core.String? runId,
    RunStatus? status,
    $core.String? candidateSetId,
  }) {
    final result = StartCandidateSearchResponse._();
    if (requestId != null) result.requestId = requestId;
    if (runId != null) result.runId = runId;
    if (status != null) result.status = status;
    if (candidateSetId != null) result.candidateSetId = candidateSetId;
    return result;
  }

  StartCandidateSearchResponse._();

  factory StartCandidateSearchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StartCandidateSearchResponse()..mergeFromBuffer(data, registry);
  factory StartCandidateSearchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StartCandidateSearchResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartCandidateSearchResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: StartCandidateSearchResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'runId')
    ..aE<RunStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: RunStatus.values)
    ..aOS(4, _omitFieldNames ? '' : 'candidateSetId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCandidateSearchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCandidateSearchResponse copyWith(
          void Function(StartCandidateSearchResponse) updates) =>
      super.copyWith(
              (message) => updates(message as StartCandidateSearchResponse))
          as StartCandidateSearchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use StartCandidateSearchResponse() / StartCandidateSearchResponse.new instead')
  static StartCandidateSearchResponse create() =>
      StartCandidateSearchResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      StartCandidateSearchResponse._();
  @$core.override
  StartCandidateSearchResponse createEmptyInstance() =>
      StartCandidateSearchResponse._();
  @$core.pragma('dart2js:noInline')
  static StartCandidateSearchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartCandidateSearchResponse>(
          StartCandidateSearchResponse.$_createMessage);
  static StartCandidateSearchResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  /// 這次搜尋的 run；與候選集是不同的 ID。
  @$pb.TagNumber(2)
  $core.String get runId => $_getSZ(1);
  @$pb.TagNumber(2)
  set runId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRunId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRunId() => $_clearField(2);

  @$pb.TagNumber(3)
  RunStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(RunStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get candidateSetId => $_getSZ(3);
  @$pb.TagNumber(4)
  set candidateSetId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCandidateSetId() => $_has(3);
  @$pb.TagNumber(4)
  void clearCandidateSetId() => $_clearField(4);
}

class GetCandidateSearchRequest extends $pb.GeneratedMessage {
  factory GetCandidateSearchRequest({
    $core.String? requestId,
    $core.String? runId,
  }) {
    final result = GetCandidateSearchRequest._();
    if (requestId != null) result.requestId = requestId;
    if (runId != null) result.runId = runId;
    return result;
  }

  GetCandidateSearchRequest._();

  factory GetCandidateSearchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCandidateSearchRequest()..mergeFromBuffer(data, registry);
  factory GetCandidateSearchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCandidateSearchRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCandidateSearchRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetCandidateSearchRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'runId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCandidateSearchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCandidateSearchRequest copyWith(
          void Function(GetCandidateSearchRequest) updates) =>
      super.copyWith((message) => updates(message as GetCandidateSearchRequest))
          as GetCandidateSearchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetCandidateSearchRequest() / GetCandidateSearchRequest.new instead')
  static GetCandidateSearchRequest create() => GetCandidateSearchRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetCandidateSearchRequest._();
  @$core.override
  GetCandidateSearchRequest createEmptyInstance() =>
      GetCandidateSearchRequest._();
  @$core.pragma('dart2js:noInline')
  static GetCandidateSearchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCandidateSearchRequest>(
          GetCandidateSearchRequest.$_createMessage);
  static GetCandidateSearchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get runId => $_getSZ(1);
  @$pb.TagNumber(2)
  set runId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRunId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRunId() => $_clearField(2);
}

class GetCandidateSearchResponse extends $pb.GeneratedMessage {
  factory GetCandidateSearchResponse({
    $core.String? requestId,
    $core.String? runId,
    $core.String? candidateSetId,
    RunStatus? status,
    RunFailureReason? failureReason,
    $core.Iterable<Candidate>? candidates,
  }) {
    final result = GetCandidateSearchResponse._();
    if (requestId != null) result.requestId = requestId;
    if (runId != null) result.runId = runId;
    if (candidateSetId != null) result.candidateSetId = candidateSetId;
    if (status != null) result.status = status;
    if (failureReason != null) result.failureReason = failureReason;
    if (candidates != null) result.candidates.addAll(candidates);
    return result;
  }

  GetCandidateSearchResponse._();

  factory GetCandidateSearchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCandidateSearchResponse()..mergeFromBuffer(data, registry);
  factory GetCandidateSearchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCandidateSearchResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCandidateSearchResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetCandidateSearchResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'runId')
    ..aOS(3, _omitFieldNames ? '' : 'candidateSetId')
    ..aE<RunStatus>(4, _omitFieldNames ? '' : 'status',
        enumValues: RunStatus.values)
    ..aE<RunFailureReason>(5, _omitFieldNames ? '' : 'failureReason',
        enumValues: RunFailureReason.values)
    ..pPM<Candidate>(6, _omitFieldNames ? '' : 'candidates',
        subBuilder: Candidate.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCandidateSearchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCandidateSearchResponse copyWith(
          void Function(GetCandidateSearchResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetCandidateSearchResponse))
          as GetCandidateSearchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetCandidateSearchResponse() / GetCandidateSearchResponse.new instead')
  static GetCandidateSearchResponse create() => GetCandidateSearchResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetCandidateSearchResponse._();
  @$core.override
  GetCandidateSearchResponse createEmptyInstance() =>
      GetCandidateSearchResponse._();
  @$core.pragma('dart2js:noInline')
  static GetCandidateSearchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCandidateSearchResponse>(
          GetCandidateSearchResponse.$_createMessage);
  static GetCandidateSearchResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get runId => $_getSZ(1);
  @$pb.TagNumber(2)
  set runId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRunId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRunId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get candidateSetId => $_getSZ(2);
  @$pb.TagNumber(3)
  set candidateSetId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCandidateSetId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCandidateSetId() => $_clearField(3);

  @$pb.TagNumber(4)
  RunStatus get status => $_getN(3);
  @$pb.TagNumber(4)
  set status(RunStatus value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearStatus() => $_clearField(4);

  /// 只在 FAILED 與 EXPIRED 時有值。
  @$pb.TagNumber(5)
  RunFailureReason get failureReason => $_getN(4);
  @$pb.TagNumber(5)
  set failureReason(RunFailureReason value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasFailureReason() => $_has(4);
  @$pb.TagNumber(5)
  void clearFailureReason() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<Candidate> get candidates => $_getList(5);
}

class ToggleSelectionRequest extends $pb.GeneratedMessage {
  factory ToggleSelectionRequest({
    $core.String? requestId,
    $core.String? candidateSetId,
    $core.String? candidateId,
    $core.bool? selected,
  }) {
    final result = ToggleSelectionRequest._();
    if (requestId != null) result.requestId = requestId;
    if (candidateSetId != null) result.candidateSetId = candidateSetId;
    if (candidateId != null) result.candidateId = candidateId;
    if (selected != null) result.selected = selected;
    return result;
  }

  ToggleSelectionRequest._();

  factory ToggleSelectionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToggleSelectionRequest()..mergeFromBuffer(data, registry);
  factory ToggleSelectionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToggleSelectionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ToggleSelectionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ToggleSelectionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'candidateSetId')
    ..aOS(3, _omitFieldNames ? '' : 'candidateId')
    ..aOB(4, _omitFieldNames ? '' : 'selected')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToggleSelectionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToggleSelectionRequest copyWith(
          void Function(ToggleSelectionRequest) updates) =>
      super.copyWith((message) => updates(message as ToggleSelectionRequest))
          as ToggleSelectionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ToggleSelectionRequest() / ToggleSelectionRequest.new instead')
  static ToggleSelectionRequest create() => ToggleSelectionRequest._();
  static $pb.GeneratedMessage $_createMessage() => ToggleSelectionRequest._();
  @$core.override
  ToggleSelectionRequest createEmptyInstance() => ToggleSelectionRequest._();
  @$core.pragma('dart2js:noInline')
  static ToggleSelectionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ToggleSelectionRequest>(
          ToggleSelectionRequest.$_createMessage);
  static ToggleSelectionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get candidateSetId => $_getSZ(1);
  @$pb.TagNumber(2)
  set candidateSetId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCandidateSetId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCandidateSetId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get candidateId => $_getSZ(2);
  @$pb.TagNumber(3)
  set candidateId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCandidateId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCandidateId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get selected => $_getBF(3);
  @$pb.TagNumber(4)
  set selected($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSelected() => $_has(3);
  @$pb.TagNumber(4)
  void clearSelected() => $_clearField(4);
}

class ToggleSelectionResponse extends $pb.GeneratedMessage {
  factory ToggleSelectionResponse({
    $core.String? requestId,
    $core.String? candidateSetId,
    $core.String? candidateId,
    $core.bool? selected,
  }) {
    final result = ToggleSelectionResponse._();
    if (requestId != null) result.requestId = requestId;
    if (candidateSetId != null) result.candidateSetId = candidateSetId;
    if (candidateId != null) result.candidateId = candidateId;
    if (selected != null) result.selected = selected;
    return result;
  }

  ToggleSelectionResponse._();

  factory ToggleSelectionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToggleSelectionResponse()..mergeFromBuffer(data, registry);
  factory ToggleSelectionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToggleSelectionResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ToggleSelectionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ToggleSelectionResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'candidateSetId')
    ..aOS(3, _omitFieldNames ? '' : 'candidateId')
    ..aOB(4, _omitFieldNames ? '' : 'selected')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToggleSelectionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToggleSelectionResponse copyWith(
          void Function(ToggleSelectionResponse) updates) =>
      super.copyWith((message) => updates(message as ToggleSelectionResponse))
          as ToggleSelectionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ToggleSelectionResponse() / ToggleSelectionResponse.new instead')
  static ToggleSelectionResponse create() => ToggleSelectionResponse._();
  static $pb.GeneratedMessage $_createMessage() => ToggleSelectionResponse._();
  @$core.override
  ToggleSelectionResponse createEmptyInstance() => ToggleSelectionResponse._();
  @$core.pragma('dart2js:noInline')
  static ToggleSelectionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ToggleSelectionResponse>(
          ToggleSelectionResponse.$_createMessage);
  static ToggleSelectionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get candidateSetId => $_getSZ(1);
  @$pb.TagNumber(2)
  set candidateSetId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCandidateSetId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCandidateSetId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get candidateId => $_getSZ(2);
  @$pb.TagNumber(3)
  set candidateId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCandidateId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCandidateId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get selected => $_getBF(3);
  @$pb.TagNumber(4)
  set selected($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSelected() => $_has(3);
  @$pb.TagNumber(4)
  void clearSelected() => $_clearField(4);
}

class SubmitAnswerRequest extends $pb.GeneratedMessage {
  factory SubmitAnswerRequest({
    $core.String? requestId,
    SlotKey? slotKey,
    $core.String? optionId,
    $core.String? freeText,
    $core.String? unitOptionId,
    $core.bool? skip,
    $core.String? expectedRevisionId,
    $core.String? clientRequestId,
  }) {
    final result = SubmitAnswerRequest._();
    if (requestId != null) result.requestId = requestId;
    if (slotKey != null) result.slotKey = slotKey;
    if (optionId != null) result.optionId = optionId;
    if (freeText != null) result.freeText = freeText;
    if (unitOptionId != null) result.unitOptionId = unitOptionId;
    if (skip != null) result.skip = skip;
    if (expectedRevisionId != null)
      result.expectedRevisionId = expectedRevisionId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    return result;
  }

  SubmitAnswerRequest._();

  factory SubmitAnswerRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitAnswerRequest()..mergeFromBuffer(data, registry);
  factory SubmitAnswerRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitAnswerRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitAnswerRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SubmitAnswerRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aE<SlotKey>(2, _omitFieldNames ? '' : 'slotKey',
        enumValues: SlotKey.values)
    ..aOS(3, _omitFieldNames ? '' : 'optionId')
    ..aOS(4, _omitFieldNames ? '' : 'freeText')
    ..aOS(5, _omitFieldNames ? '' : 'unitOptionId')
    ..aOB(6, _omitFieldNames ? '' : 'skip')
    ..aOS(7, _omitFieldNames ? '' : 'expectedRevisionId')
    ..aOS(8, _omitFieldNames ? '' : 'clientRequestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAnswerRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAnswerRequest copyWith(void Function(SubmitAnswerRequest) updates) =>
      super.copyWith((message) => updates(message as SubmitAnswerRequest))
          as SubmitAnswerRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SubmitAnswerRequest() / SubmitAnswerRequest.new instead')
  static SubmitAnswerRequest create() => SubmitAnswerRequest._();
  static $pb.GeneratedMessage $_createMessage() => SubmitAnswerRequest._();
  @$core.override
  SubmitAnswerRequest createEmptyInstance() => SubmitAnswerRequest._();
  @$core.pragma('dart2js:noInline')
  static SubmitAnswerRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitAnswerRequest>(
          SubmitAnswerRequest.$_createMessage);
  static SubmitAnswerRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  SlotKey get slotKey => $_getN(1);
  @$pb.TagNumber(2)
  set slotKey(SlotKey value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSlotKey() => $_has(1);
  @$pb.TagNumber(2)
  void clearSlotKey() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get optionId => $_getSZ(2);
  @$pb.TagNumber(3)
  set optionId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOptionId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOptionId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get freeText => $_getSZ(3);
  @$pb.TagNumber(4)
  set freeText($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFreeText() => $_has(3);
  @$pb.TagNumber(4)
  void clearFreeText() => $_clearField(4);

  /// 數量這類複合欄位的單位選項。
  @$pb.TagNumber(5)
  $core.String get unitOptionId => $_getSZ(4);
  @$pb.TagNumber(5)
  set unitOptionId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasUnitOptionId() => $_has(4);
  @$pb.TagNumber(5)
  void clearUnitOptionId() => $_clearField(5);

  /// 略過這個問題；必填欄位不能略過。
  @$pb.TagNumber(6)
  $core.bool get skip => $_getBF(5);
  @$pb.TagNumber(6)
  set skip($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSkip() => $_has(5);
  @$pb.TagNumber(6)
  void clearSkip() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get expectedRevisionId => $_getSZ(6);
  @$pb.TagNumber(7)
  set expectedRevisionId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasExpectedRevisionId() => $_has(6);
  @$pb.TagNumber(7)
  void clearExpectedRevisionId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get clientRequestId => $_getSZ(7);
  @$pb.TagNumber(8)
  set clientRequestId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasClientRequestId() => $_has(7);
  @$pb.TagNumber(8)
  void clearClientRequestId() => $_clearField(8);
}

class SubmitAnswerResponse extends $pb.GeneratedMessage {
  factory SubmitAnswerResponse({
    TurnResult? result,
  }) {
    final result$ = SubmitAnswerResponse._();
    if (result != null) result$.result = result;
    return result$;
  }

  SubmitAnswerResponse._();

  factory SubmitAnswerResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitAnswerResponse()..mergeFromBuffer(data, registry);
  factory SubmitAnswerResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitAnswerResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitAnswerResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SubmitAnswerResponse.$_createMessage)
    ..aOM<TurnResult>(1, _omitFieldNames ? '' : 'result',
        subBuilder: TurnResult.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAnswerResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAnswerResponse copyWith(void Function(SubmitAnswerResponse) updates) =>
      super.copyWith((message) => updates(message as SubmitAnswerResponse))
          as SubmitAnswerResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubmitAnswerResponse() / SubmitAnswerResponse.new instead')
  static SubmitAnswerResponse create() => SubmitAnswerResponse._();
  static $pb.GeneratedMessage $_createMessage() => SubmitAnswerResponse._();
  @$core.override
  SubmitAnswerResponse createEmptyInstance() => SubmitAnswerResponse._();
  @$core.pragma('dart2js:noInline')
  static SubmitAnswerResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitAnswerResponse>(
          SubmitAnswerResponse.$_createMessage);
  static SubmitAnswerResponse? _defaultInstance;

  @$pb.TagNumber(1)
  TurnResult get result => $_getN(0);
  @$pb.TagNumber(1)
  set result(TurnResult value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasResult() => $_has(0);
  @$pb.TagNumber(1)
  void clearResult() => $_clearField(1);
  @$pb.TagNumber(1)
  TurnResult ensureResult() => $_ensure(0);
}

class RewindRequest extends $pb.GeneratedMessage {
  factory RewindRequest({
    $core.String? requestId,
    SlotKey? slotKey,
    $core.String? clientRequestId,
    $core.String? expectedRevisionId,
  }) {
    final result = RewindRequest._();
    if (requestId != null) result.requestId = requestId;
    if (slotKey != null) result.slotKey = slotKey;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (expectedRevisionId != null)
      result.expectedRevisionId = expectedRevisionId;
    return result;
  }

  RewindRequest._();

  factory RewindRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RewindRequest()..mergeFromBuffer(data, registry);
  factory RewindRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RewindRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RewindRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RewindRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aE<SlotKey>(2, _omitFieldNames ? '' : 'slotKey',
        enumValues: SlotKey.values)
    ..aOS(3, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(4, _omitFieldNames ? '' : 'expectedRevisionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RewindRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RewindRequest copyWith(void Function(RewindRequest) updates) =>
      super.copyWith((message) => updates(message as RewindRequest))
          as RewindRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RewindRequest() / RewindRequest.new instead')
  static RewindRequest create() => RewindRequest._();
  static $pb.GeneratedMessage $_createMessage() => RewindRequest._();
  @$core.override
  RewindRequest createEmptyInstance() => RewindRequest._();
  @$core.pragma('dart2js:noInline')
  static RewindRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RewindRequest>(
          RewindRequest.$_createMessage);
  static RewindRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  SlotKey get slotKey => $_getN(1);
  @$pb.TagNumber(2)
  set slotKey(SlotKey value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSlotKey() => $_has(1);
  @$pb.TagNumber(2)
  void clearSlotKey() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get clientRequestId => $_getSZ(2);
  @$pb.TagNumber(3)
  set clientRequestId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasClientRequestId() => $_has(2);
  @$pb.TagNumber(3)
  void clearClientRequestId() => $_clearField(3);

  /// 讀到的需求版本；需求已被改版（別的裝置回答或倒回過）回 ABORTED，reason 是 VERSION_CONFLICT，conflictTarget 是 request_revision。
  @$pb.TagNumber(4)
  $core.String get expectedRevisionId => $_getSZ(3);
  @$pb.TagNumber(4)
  set expectedRevisionId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExpectedRevisionId() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpectedRevisionId() => $_clearField(4);
}

class RewindResponse extends $pb.GeneratedMessage {
  factory RewindResponse({
    TurnResult? result,
  }) {
    final result$ = RewindResponse._();
    if (result != null) result$.result = result;
    return result$;
  }

  RewindResponse._();

  factory RewindResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RewindResponse()..mergeFromBuffer(data, registry);
  factory RewindResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RewindResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RewindResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RewindResponse.$_createMessage)
    ..aOM<TurnResult>(1, _omitFieldNames ? '' : 'result',
        subBuilder: TurnResult.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RewindResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RewindResponse copyWith(void Function(RewindResponse) updates) =>
      super.copyWith((message) => updates(message as RewindResponse))
          as RewindResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RewindResponse() / RewindResponse.new instead')
  static RewindResponse create() => RewindResponse._();
  static $pb.GeneratedMessage $_createMessage() => RewindResponse._();
  @$core.override
  RewindResponse createEmptyInstance() => RewindResponse._();
  @$core.pragma('dart2js:noInline')
  static RewindResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RewindResponse>(
          RewindResponse.$_createMessage);
  static RewindResponse? _defaultInstance;

  @$pb.TagNumber(1)
  TurnResult get result => $_getN(0);
  @$pb.TagNumber(1)
  set result(TurnResult value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasResult() => $_has(0);
  @$pb.TagNumber(1)
  void clearResult() => $_clearField(1);
  @$pb.TagNumber(1)
  TurnResult ensureResult() => $_ensure(0);
}

/// BuyerRequestedRange 是買家預期的單價範圍，單位是新台幣元。
class BuyerRequestedRange extends $pb.GeneratedMessage {
  factory BuyerRequestedRange({
    $fixnum.Int64? lowerBoundAmount,
    $fixnum.Int64? upperBoundAmount,
    $core.int? basisQuantity,
    $core.String? basisUnit,
  }) {
    final result = BuyerRequestedRange._();
    if (lowerBoundAmount != null) result.lowerBoundAmount = lowerBoundAmount;
    if (upperBoundAmount != null) result.upperBoundAmount = upperBoundAmount;
    if (basisQuantity != null) result.basisQuantity = basisQuantity;
    if (basisUnit != null) result.basisUnit = basisUnit;
    return result;
  }

  BuyerRequestedRange._();

  factory BuyerRequestedRange.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BuyerRequestedRange()..mergeFromBuffer(data, registry);
  factory BuyerRequestedRange.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BuyerRequestedRange()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BuyerRequestedRange',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: BuyerRequestedRange.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'lowerBoundAmount')
    ..aInt64(2, _omitFieldNames ? '' : 'upperBoundAmount')
    ..aI(3, _omitFieldNames ? '' : 'basisQuantity')
    ..aOS(4, _omitFieldNames ? '' : 'basisUnit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BuyerRequestedRange clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BuyerRequestedRange copyWith(void Function(BuyerRequestedRange) updates) =>
      super.copyWith((message) => updates(message as BuyerRequestedRange))
          as BuyerRequestedRange;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use BuyerRequestedRange() / BuyerRequestedRange.new instead')
  static BuyerRequestedRange create() => BuyerRequestedRange._();
  static $pb.GeneratedMessage $_createMessage() => BuyerRequestedRange._();
  @$core.override
  BuyerRequestedRange createEmptyInstance() => BuyerRequestedRange._();
  @$core.pragma('dart2js:noInline')
  static BuyerRequestedRange getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BuyerRequestedRange>(
          BuyerRequestedRange.$_createMessage);
  static BuyerRequestedRange? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get lowerBoundAmount => $_getI64(0);
  @$pb.TagNumber(1)
  set lowerBoundAmount($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasLowerBoundAmount() => $_has(0);
  @$pb.TagNumber(1)
  void clearLowerBoundAmount() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get upperBoundAmount => $_getI64(1);
  @$pb.TagNumber(2)
  set upperBoundAmount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUpperBoundAmount() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpperBoundAmount() => $_clearField(2);

  /// 單價的計價基準：每 basis_quantity 個 basis_unit。由伺服器依需求的單位填，請求不帶。
  @$pb.TagNumber(3)
  $core.int get basisQuantity => $_getIZ(2);
  @$pb.TagNumber(3)
  set basisQuantity($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBasisQuantity() => $_has(2);
  @$pb.TagNumber(3)
  void clearBasisQuantity() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get basisUnit => $_getSZ(3);
  @$pb.TagNumber(4)
  set basisUnit($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasBasisUnit() => $_has(3);
  @$pb.TagNumber(4)
  void clearBasisUnit() => $_clearField(4);
}

/// QuoteTotalRange 是伺服器依單價範圍與需求數量算出的詢價總額區間。client 不自己乘：計價基準因單位而異。
class QuoteTotalRange extends $pb.GeneratedMessage {
  factory QuoteTotalRange({
    $fixnum.Int64? lowerBoundAmount,
    $fixnum.Int64? upperBoundAmount,
    $core.int? quantity,
    $core.String? quantityUnit,
  }) {
    final result = QuoteTotalRange._();
    if (lowerBoundAmount != null) result.lowerBoundAmount = lowerBoundAmount;
    if (upperBoundAmount != null) result.upperBoundAmount = upperBoundAmount;
    if (quantity != null) result.quantity = quantity;
    if (quantityUnit != null) result.quantityUnit = quantityUnit;
    return result;
  }

  QuoteTotalRange._();

  factory QuoteTotalRange.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      QuoteTotalRange()..mergeFromBuffer(data, registry);
  factory QuoteTotalRange.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      QuoteTotalRange()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'QuoteTotalRange',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: QuoteTotalRange.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'lowerBoundAmount')
    ..aInt64(2, _omitFieldNames ? '' : 'upperBoundAmount')
    ..aI(3, _omitFieldNames ? '' : 'quantity')
    ..aOS(4, _omitFieldNames ? '' : 'quantityUnit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QuoteTotalRange clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QuoteTotalRange copyWith(void Function(QuoteTotalRange) updates) =>
      super.copyWith((message) => updates(message as QuoteTotalRange))
          as QuoteTotalRange;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use QuoteTotalRange() / QuoteTotalRange.new instead')
  static QuoteTotalRange create() => QuoteTotalRange._();
  static $pb.GeneratedMessage $_createMessage() => QuoteTotalRange._();
  @$core.override
  QuoteTotalRange createEmptyInstance() => QuoteTotalRange._();
  @$core.pragma('dart2js:noInline')
  static QuoteTotalRange getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<QuoteTotalRange>(
          QuoteTotalRange.$_createMessage);
  static QuoteTotalRange? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get lowerBoundAmount => $_getI64(0);
  @$pb.TagNumber(1)
  set lowerBoundAmount($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasLowerBoundAmount() => $_has(0);
  @$pb.TagNumber(1)
  void clearLowerBoundAmount() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get upperBoundAmount => $_getI64(1);
  @$pb.TagNumber(2)
  set upperBoundAmount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUpperBoundAmount() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpperBoundAmount() => $_clearField(2);

  /// 計算採用的數量與單位（取自需求，不是 client 送來的值），讓買家核對。
  @$pb.TagNumber(3)
  $core.int get quantity => $_getIZ(2);
  @$pb.TagNumber(3)
  set quantity($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasQuantity() => $_has(2);
  @$pb.TagNumber(3)
  void clearQuantity() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get quantityUnit => $_getSZ(3);
  @$pb.TagNumber(4)
  set quantityUnit($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasQuantityUnit() => $_has(3);
  @$pb.TagNumber(4)
  void clearQuantityUnit() => $_clearField(4);
}

class PreviewRfqPriceRequest extends $pb.GeneratedMessage {
  factory PreviewRfqPriceRequest({
    $core.String? requestId,
    $core.String? expectedRevisionId,
    $core.String? clientRequestId,
    $core.String? deliveryCity,
    BuyerRequestedRange? buyerRequestedRange,
  }) {
    final result = PreviewRfqPriceRequest._();
    if (requestId != null) result.requestId = requestId;
    if (expectedRevisionId != null)
      result.expectedRevisionId = expectedRevisionId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (deliveryCity != null) result.deliveryCity = deliveryCity;
    if (buyerRequestedRange != null)
      result.buyerRequestedRange = buyerRequestedRange;
    return result;
  }

  PreviewRfqPriceRequest._();

  factory PreviewRfqPriceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewRfqPriceRequest()..mergeFromBuffer(data, registry);
  factory PreviewRfqPriceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewRfqPriceRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PreviewRfqPriceRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: PreviewRfqPriceRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'expectedRevisionId')
    ..aOS(3, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(4, _omitFieldNames ? '' : 'deliveryCity')
    ..aOM<BuyerRequestedRange>(5, _omitFieldNames ? '' : 'buyerRequestedRange',
        subBuilder: BuyerRequestedRange.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewRfqPriceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewRfqPriceRequest copyWith(
          void Function(PreviewRfqPriceRequest) updates) =>
      super.copyWith((message) => updates(message as PreviewRfqPriceRequest))
          as PreviewRfqPriceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PreviewRfqPriceRequest() / PreviewRfqPriceRequest.new instead')
  static PreviewRfqPriceRequest create() => PreviewRfqPriceRequest._();
  static $pb.GeneratedMessage $_createMessage() => PreviewRfqPriceRequest._();
  @$core.override
  PreviewRfqPriceRequest createEmptyInstance() => PreviewRfqPriceRequest._();
  @$core.pragma('dart2js:noInline')
  static PreviewRfqPriceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PreviewRfqPriceRequest>(
          PreviewRfqPriceRequest.$_createMessage);
  static PreviewRfqPriceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get expectedRevisionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set expectedRevisionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExpectedRevisionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpectedRevisionId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get clientRequestId => $_getSZ(2);
  @$pb.TagNumber(3)
  set clientRequestId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasClientRequestId() => $_has(2);
  @$pb.TagNumber(3)
  void clearClientRequestId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deliveryCity => $_getSZ(3);
  @$pb.TagNumber(4)
  set deliveryCity($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeliveryCity() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeliveryCity() => $_clearField(4);

  @$pb.TagNumber(5)
  BuyerRequestedRange get buyerRequestedRange => $_getN(4);
  @$pb.TagNumber(5)
  set buyerRequestedRange(BuyerRequestedRange value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasBuyerRequestedRange() => $_has(4);
  @$pb.TagNumber(5)
  void clearBuyerRequestedRange() => $_clearField(5);
  @$pb.TagNumber(5)
  BuyerRequestedRange ensureBuyerRequestedRange() => $_ensure(4);
}

class PreviewRfqPriceResponse extends $pb.GeneratedMessage {
  factory PreviewRfqPriceResponse({
    $core.String? pricePreviewId,
    $core.String? version,
    BuyerRequestedRange? buyerRequestedRange,
    $core.String? payloadHash,
    QuoteTotalRange? quoteTotalRange,
  }) {
    final result = PreviewRfqPriceResponse._();
    if (pricePreviewId != null) result.pricePreviewId = pricePreviewId;
    if (version != null) result.version = version;
    if (buyerRequestedRange != null)
      result.buyerRequestedRange = buyerRequestedRange;
    if (payloadHash != null) result.payloadHash = payloadHash;
    if (quoteTotalRange != null) result.quoteTotalRange = quoteTotalRange;
    return result;
  }

  PreviewRfqPriceResponse._();

  factory PreviewRfqPriceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewRfqPriceResponse()..mergeFromBuffer(data, registry);
  factory PreviewRfqPriceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewRfqPriceResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PreviewRfqPriceResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: PreviewRfqPriceResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'pricePreviewId')
    ..aOS(2, _omitFieldNames ? '' : 'version')
    ..aOM<BuyerRequestedRange>(3, _omitFieldNames ? '' : 'buyerRequestedRange',
        subBuilder: BuyerRequestedRange.$_createMessage)
    ..aOS(4, _omitFieldNames ? '' : 'payloadHash')
    ..aOM<QuoteTotalRange>(5, _omitFieldNames ? '' : 'quoteTotalRange',
        subBuilder: QuoteTotalRange.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewRfqPriceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewRfqPriceResponse copyWith(
          void Function(PreviewRfqPriceResponse) updates) =>
      super.copyWith((message) => updates(message as PreviewRfqPriceResponse))
          as PreviewRfqPriceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PreviewRfqPriceResponse() / PreviewRfqPriceResponse.new instead')
  static PreviewRfqPriceResponse create() => PreviewRfqPriceResponse._();
  static $pb.GeneratedMessage $_createMessage() => PreviewRfqPriceResponse._();
  @$core.override
  PreviewRfqPriceResponse createEmptyInstance() => PreviewRfqPriceResponse._();
  @$core.pragma('dart2js:noInline')
  static PreviewRfqPriceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PreviewRfqPriceResponse>(
          PreviewRfqPriceResponse.$_createMessage);
  static PreviewRfqPriceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get pricePreviewId => $_getSZ(0);
  @$pb.TagNumber(1)
  set pricePreviewId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPricePreviewId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPricePreviewId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get version => $_getSZ(1);
  @$pb.TagNumber(2)
  set version($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  BuyerRequestedRange get buyerRequestedRange => $_getN(2);
  @$pb.TagNumber(3)
  set buyerRequestedRange(BuyerRequestedRange value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasBuyerRequestedRange() => $_has(2);
  @$pb.TagNumber(3)
  void clearBuyerRequestedRange() => $_clearField(3);
  @$pb.TagNumber(3)
  BuyerRequestedRange ensureBuyerRequestedRange() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get payloadHash => $_getSZ(3);
  @$pb.TagNumber(4)
  set payloadHash($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPayloadHash() => $_has(3);
  @$pb.TagNumber(4)
  void clearPayloadHash() => $_clearField(4);

  /// 需求沒有填數量時不設；不回 0，也不把單價當總額。
  @$pb.TagNumber(5)
  QuoteTotalRange get quoteTotalRange => $_getN(4);
  @$pb.TagNumber(5)
  set quoteTotalRange(QuoteTotalRange value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasQuoteTotalRange() => $_has(4);
  @$pb.TagNumber(5)
  void clearQuoteTotalRange() => $_clearField(5);
  @$pb.TagNumber(5)
  QuoteTotalRange ensureQuoteTotalRange() => $_ensure(4);
}

class ConfirmRfqSnapshotRequest extends $pb.GeneratedMessage {
  factory ConfirmRfqSnapshotRequest({
    $core.String? requestId,
    $core.String? clientRequestId,
    $core.String? expectedRevisionId,
    $core.String? pricePreviewId,
    $core.String? pricePreviewVersion,
    $core.String? payloadHash,
    $core.String? buyerNote,
  }) {
    final result = ConfirmRfqSnapshotRequest._();
    if (requestId != null) result.requestId = requestId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (expectedRevisionId != null)
      result.expectedRevisionId = expectedRevisionId;
    if (pricePreviewId != null) result.pricePreviewId = pricePreviewId;
    if (pricePreviewVersion != null)
      result.pricePreviewVersion = pricePreviewVersion;
    if (payloadHash != null) result.payloadHash = payloadHash;
    if (buyerNote != null) result.buyerNote = buyerNote;
    return result;
  }

  ConfirmRfqSnapshotRequest._();

  factory ConfirmRfqSnapshotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConfirmRfqSnapshotRequest()..mergeFromBuffer(data, registry);
  factory ConfirmRfqSnapshotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConfirmRfqSnapshotRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConfirmRfqSnapshotRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ConfirmRfqSnapshotRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(3, _omitFieldNames ? '' : 'expectedRevisionId')
    ..aOS(4, _omitFieldNames ? '' : 'pricePreviewId')
    ..aOS(5, _omitFieldNames ? '' : 'pricePreviewVersion')
    ..aOS(6, _omitFieldNames ? '' : 'payloadHash')
    ..aOS(7, _omitFieldNames ? '' : 'buyerNote')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConfirmRfqSnapshotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConfirmRfqSnapshotRequest copyWith(
          void Function(ConfirmRfqSnapshotRequest) updates) =>
      super.copyWith((message) => updates(message as ConfirmRfqSnapshotRequest))
          as ConfirmRfqSnapshotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ConfirmRfqSnapshotRequest() / ConfirmRfqSnapshotRequest.new instead')
  static ConfirmRfqSnapshotRequest create() => ConfirmRfqSnapshotRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ConfirmRfqSnapshotRequest._();
  @$core.override
  ConfirmRfqSnapshotRequest createEmptyInstance() =>
      ConfirmRfqSnapshotRequest._();
  @$core.pragma('dart2js:noInline')
  static ConfirmRfqSnapshotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConfirmRfqSnapshotRequest>(
          ConfirmRfqSnapshotRequest.$_createMessage);
  static ConfirmRfqSnapshotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get expectedRevisionId => $_getSZ(2);
  @$pb.TagNumber(3)
  set expectedRevisionId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasExpectedRevisionId() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpectedRevisionId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get pricePreviewId => $_getSZ(3);
  @$pb.TagNumber(4)
  set pricePreviewId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPricePreviewId() => $_has(3);
  @$pb.TagNumber(4)
  void clearPricePreviewId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get pricePreviewVersion => $_getSZ(4);
  @$pb.TagNumber(5)
  set pricePreviewVersion($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPricePreviewVersion() => $_has(4);
  @$pb.TagNumber(5)
  void clearPricePreviewVersion() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get payloadHash => $_getSZ(5);
  @$pb.TagNumber(6)
  set payloadHash($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasPayloadHash() => $_has(5);
  @$pb.TagNumber(6)
  void clearPayloadHash() => $_clearField(6);

  /// 買家自己打的補充說明，原文轉呈給供應商，最多 500 字。伺服器不解析、不用它做候選比對。
  @$pb.TagNumber(7)
  $core.String get buyerNote => $_getSZ(6);
  @$pb.TagNumber(7)
  set buyerNote($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasBuyerNote() => $_has(6);
  @$pb.TagNumber(7)
  void clearBuyerNote() => $_clearField(7);
}

class ConfirmRfqSnapshotResponse extends $pb.GeneratedMessage {
  factory ConfirmRfqSnapshotResponse({
    $core.String? confirmedRfqSnapshotId,
    $core.String? payloadHash,
    $1.Timestamp? expireTime,
  }) {
    final result = ConfirmRfqSnapshotResponse._();
    if (confirmedRfqSnapshotId != null)
      result.confirmedRfqSnapshotId = confirmedRfqSnapshotId;
    if (payloadHash != null) result.payloadHash = payloadHash;
    if (expireTime != null) result.expireTime = expireTime;
    return result;
  }

  ConfirmRfqSnapshotResponse._();

  factory ConfirmRfqSnapshotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConfirmRfqSnapshotResponse()..mergeFromBuffer(data, registry);
  factory ConfirmRfqSnapshotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConfirmRfqSnapshotResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConfirmRfqSnapshotResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ConfirmRfqSnapshotResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'confirmedRfqSnapshotId')
    ..aOS(2, _omitFieldNames ? '' : 'payloadHash')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'expireTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConfirmRfqSnapshotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConfirmRfqSnapshotResponse copyWith(
          void Function(ConfirmRfqSnapshotResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ConfirmRfqSnapshotResponse))
          as ConfirmRfqSnapshotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ConfirmRfqSnapshotResponse() / ConfirmRfqSnapshotResponse.new instead')
  static ConfirmRfqSnapshotResponse create() => ConfirmRfqSnapshotResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ConfirmRfqSnapshotResponse._();
  @$core.override
  ConfirmRfqSnapshotResponse createEmptyInstance() =>
      ConfirmRfqSnapshotResponse._();
  @$core.pragma('dart2js:noInline')
  static ConfirmRfqSnapshotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConfirmRfqSnapshotResponse>(
          ConfirmRfqSnapshotResponse.$_createMessage);
  static ConfirmRfqSnapshotResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get confirmedRfqSnapshotId => $_getSZ(0);
  @$pb.TagNumber(1)
  set confirmedRfqSnapshotId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConfirmedRfqSnapshotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConfirmedRfqSnapshotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get payloadHash => $_getSZ(1);
  @$pb.TagNumber(2)
  set payloadHash($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPayloadHash() => $_has(1);
  @$pb.TagNumber(2)
  void clearPayloadHash() => $_clearField(2);

  /// 快照到期的時間；這個欄位加入之前建立的快照沒有。
  @$pb.TagNumber(3)
  $1.Timestamp get expireTime => $_getN(2);
  @$pb.TagNumber(3)
  set expireTime($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasExpireTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpireTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureExpireTime() => $_ensure(2);
}

class SellerFacingRequestedRange extends $pb.GeneratedMessage {
  factory SellerFacingRequestedRange({
    $core.int? basisQuantity,
    $core.String? basisUnit,
    $fixnum.Int64? lowerBoundAmount,
    $fixnum.Int64? upperBoundAmount,
  }) {
    final result = SellerFacingRequestedRange._();
    if (basisQuantity != null) result.basisQuantity = basisQuantity;
    if (basisUnit != null) result.basisUnit = basisUnit;
    if (lowerBoundAmount != null) result.lowerBoundAmount = lowerBoundAmount;
    if (upperBoundAmount != null) result.upperBoundAmount = upperBoundAmount;
    return result;
  }

  SellerFacingRequestedRange._();

  factory SellerFacingRequestedRange.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerFacingRequestedRange()..mergeFromBuffer(data, registry);
  factory SellerFacingRequestedRange.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerFacingRequestedRange()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerFacingRequestedRange',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SellerFacingRequestedRange.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'basisQuantity')
    ..aOS(2, _omitFieldNames ? '' : 'basisUnit')
    ..aInt64(3, _omitFieldNames ? '' : 'lowerBoundAmount')
    ..aInt64(4, _omitFieldNames ? '' : 'upperBoundAmount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerFacingRequestedRange clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerFacingRequestedRange copyWith(
          void Function(SellerFacingRequestedRange) updates) =>
      super.copyWith(
              (message) => updates(message as SellerFacingRequestedRange))
          as SellerFacingRequestedRange;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SellerFacingRequestedRange() / SellerFacingRequestedRange.new instead')
  static SellerFacingRequestedRange create() => SellerFacingRequestedRange._();
  static $pb.GeneratedMessage $_createMessage() =>
      SellerFacingRequestedRange._();
  @$core.override
  SellerFacingRequestedRange createEmptyInstance() =>
      SellerFacingRequestedRange._();
  @$core.pragma('dart2js:noInline')
  static SellerFacingRequestedRange getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SellerFacingRequestedRange>(
          SellerFacingRequestedRange.$_createMessage);
  static SellerFacingRequestedRange? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get basisQuantity => $_getIZ(0);
  @$pb.TagNumber(1)
  set basisQuantity($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBasisQuantity() => $_has(0);
  @$pb.TagNumber(1)
  void clearBasisQuantity() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get basisUnit => $_getSZ(1);
  @$pb.TagNumber(2)
  set basisUnit($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBasisUnit() => $_has(1);
  @$pb.TagNumber(2)
  void clearBasisUnit() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get lowerBoundAmount => $_getI64(2);
  @$pb.TagNumber(3)
  set lowerBoundAmount($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLowerBoundAmount() => $_has(2);
  @$pb.TagNumber(3)
  void clearLowerBoundAmount() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get upperBoundAmount => $_getI64(3);
  @$pb.TagNumber(4)
  set upperBoundAmount($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasUpperBoundAmount() => $_has(3);
  @$pb.TagNumber(4)
  void clearUpperBoundAmount() => $_clearField(4);
}

/// SellerFacingRfq 是賣家看到的詢價內容，只含白名單內的欄位。買家端的發布預覽與賣家端的詢價用同一個型別，
/// 兩邊看到的內容才不會分家。
class SellerFacingRfq extends $pb.GeneratedMessage {
  factory SellerFacingRfq({
    $core.String? item,
    $core.String? itemLabel,
    $core.int? quantity,
    $core.String? unit,
    $core.String? packaging,
    $core.String? deliveryCity,
    $0.Date? deliveryDate,
    $0.Date? responseDeadline,
    $core.String? buyerNote,
    SellerFacingRequestedRange? buyerRequestedRange,
    $core.String? publicationVersion,
  }) {
    final result = SellerFacingRfq._();
    if (item != null) result.item = item;
    if (itemLabel != null) result.itemLabel = itemLabel;
    if (quantity != null) result.quantity = quantity;
    if (unit != null) result.unit = unit;
    if (packaging != null) result.packaging = packaging;
    if (deliveryCity != null) result.deliveryCity = deliveryCity;
    if (deliveryDate != null) result.deliveryDate = deliveryDate;
    if (responseDeadline != null) result.responseDeadline = responseDeadline;
    if (buyerNote != null) result.buyerNote = buyerNote;
    if (buyerRequestedRange != null)
      result.buyerRequestedRange = buyerRequestedRange;
    if (publicationVersion != null)
      result.publicationVersion = publicationVersion;
    return result;
  }

  SellerFacingRfq._();

  factory SellerFacingRfq.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerFacingRfq()..mergeFromBuffer(data, registry);
  factory SellerFacingRfq.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerFacingRfq()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerFacingRfq',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SellerFacingRfq.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'item')
    ..aOS(2, _omitFieldNames ? '' : 'itemLabel')
    ..aI(3, _omitFieldNames ? '' : 'quantity')
    ..aOS(4, _omitFieldNames ? '' : 'unit')
    ..aOS(5, _omitFieldNames ? '' : 'packaging')
    ..aOS(6, _omitFieldNames ? '' : 'deliveryCity')
    ..aOM<$0.Date>(7, _omitFieldNames ? '' : 'deliveryDate',
        subBuilder: $0.Date.$_createMessage)
    ..aOM<$0.Date>(8, _omitFieldNames ? '' : 'responseDeadline',
        subBuilder: $0.Date.$_createMessage)
    ..aOS(9, _omitFieldNames ? '' : 'buyerNote')
    ..aOM<SellerFacingRequestedRange>(
        10, _omitFieldNames ? '' : 'buyerRequestedRange',
        subBuilder: SellerFacingRequestedRange.$_createMessage)
    ..aOS(11, _omitFieldNames ? '' : 'publicationVersion')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerFacingRfq clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerFacingRfq copyWith(void Function(SellerFacingRfq) updates) =>
      super.copyWith((message) => updates(message as SellerFacingRfq))
          as SellerFacingRfq;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SellerFacingRfq() / SellerFacingRfq.new instead')
  static SellerFacingRfq create() => SellerFacingRfq._();
  static $pb.GeneratedMessage $_createMessage() => SellerFacingRfq._();
  @$core.override
  SellerFacingRfq createEmptyInstance() => SellerFacingRfq._();
  @$core.pragma('dart2js:noInline')
  static SellerFacingRfq getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SellerFacingRfq>(
          SellerFacingRfq.$_createMessage);
  static SellerFacingRfq? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get item => $_getSZ(0);
  @$pb.TagNumber(1)
  set item($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasItem() => $_has(0);
  @$pb.TagNumber(1)
  void clearItem() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get itemLabel => $_getSZ(1);
  @$pb.TagNumber(2)
  set itemLabel($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasItemLabel() => $_has(1);
  @$pb.TagNumber(2)
  void clearItemLabel() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get quantity => $_getIZ(2);
  @$pb.TagNumber(3)
  set quantity($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasQuantity() => $_has(2);
  @$pb.TagNumber(3)
  void clearQuantity() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get unit => $_getSZ(3);
  @$pb.TagNumber(4)
  set unit($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasUnit() => $_has(3);
  @$pb.TagNumber(4)
  void clearUnit() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get packaging => $_getSZ(4);
  @$pb.TagNumber(5)
  set packaging($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPackaging() => $_has(4);
  @$pb.TagNumber(5)
  void clearPackaging() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get deliveryCity => $_getSZ(5);
  @$pb.TagNumber(6)
  set deliveryCity($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDeliveryCity() => $_has(5);
  @$pb.TagNumber(6)
  void clearDeliveryCity() => $_clearField(6);

  @$pb.TagNumber(7)
  $0.Date get deliveryDate => $_getN(6);
  @$pb.TagNumber(7)
  set deliveryDate($0.Date value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasDeliveryDate() => $_has(6);
  @$pb.TagNumber(7)
  void clearDeliveryDate() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.Date ensureDeliveryDate() => $_ensure(6);

  @$pb.TagNumber(8)
  $0.Date get responseDeadline => $_getN(7);
  @$pb.TagNumber(8)
  set responseDeadline($0.Date value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasResponseDeadline() => $_has(7);
  @$pb.TagNumber(8)
  void clearResponseDeadline() => $_clearField(8);
  @$pb.TagNumber(8)
  $0.Date ensureResponseDeadline() => $_ensure(7);

  /// 買家的補充說明，原文。沒有時是空字串。
  @$pb.TagNumber(9)
  $core.String get buyerNote => $_getSZ(8);
  @$pb.TagNumber(9)
  set buyerNote($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasBuyerNote() => $_has(8);
  @$pb.TagNumber(9)
  void clearBuyerNote() => $_clearField(9);

  @$pb.TagNumber(10)
  SellerFacingRequestedRange get buyerRequestedRange => $_getN(9);
  @$pb.TagNumber(10)
  set buyerRequestedRange(SellerFacingRequestedRange value) =>
      $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasBuyerRequestedRange() => $_has(9);
  @$pb.TagNumber(10)
  void clearBuyerRequestedRange() => $_clearField(10);
  @$pb.TagNumber(10)
  SellerFacingRequestedRange ensureBuyerRequestedRange() => $_ensure(9);

  /// 這筆詢價的版本。賣家送出報價時帶回，把報價綁在他看到的那一版。
  @$pb.TagNumber(11)
  $core.String get publicationVersion => $_getSZ(10);
  @$pb.TagNumber(11)
  set publicationVersion($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasPublicationVersion() => $_has(10);
  @$pb.TagNumber(11)
  void clearPublicationVersion() => $_clearField(11);
}

class PreviewRfqPublicationRequest extends $pb.GeneratedMessage {
  factory PreviewRfqPublicationRequest({
    $core.String? requestId,
    $core.String? clientRequestId,
    $core.String? expectedRevisionId,
    $core.String? confirmedRfqSnapshotId,
    AudienceMode? audienceMode,
    $core.String? candidateSetId,
    $core.Iterable<$core.String>? selectedCandidateIds,
    $0.Date? responseDeadline,
  }) {
    final result = PreviewRfqPublicationRequest._();
    if (requestId != null) result.requestId = requestId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (expectedRevisionId != null)
      result.expectedRevisionId = expectedRevisionId;
    if (confirmedRfqSnapshotId != null)
      result.confirmedRfqSnapshotId = confirmedRfqSnapshotId;
    if (audienceMode != null) result.audienceMode = audienceMode;
    if (candidateSetId != null) result.candidateSetId = candidateSetId;
    if (selectedCandidateIds != null)
      result.selectedCandidateIds.addAll(selectedCandidateIds);
    if (responseDeadline != null) result.responseDeadline = responseDeadline;
    return result;
  }

  PreviewRfqPublicationRequest._();

  factory PreviewRfqPublicationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewRfqPublicationRequest()..mergeFromBuffer(data, registry);
  factory PreviewRfqPublicationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewRfqPublicationRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PreviewRfqPublicationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: PreviewRfqPublicationRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(3, _omitFieldNames ? '' : 'expectedRevisionId')
    ..aOS(4, _omitFieldNames ? '' : 'confirmedRfqSnapshotId')
    ..aE<AudienceMode>(5, _omitFieldNames ? '' : 'audienceMode',
        enumValues: AudienceMode.values)
    ..aOS(6, _omitFieldNames ? '' : 'candidateSetId')
    ..pPS(7, _omitFieldNames ? '' : 'selectedCandidateIds')
    ..aOM<$0.Date>(8, _omitFieldNames ? '' : 'responseDeadline',
        subBuilder: $0.Date.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewRfqPublicationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewRfqPublicationRequest copyWith(
          void Function(PreviewRfqPublicationRequest) updates) =>
      super.copyWith(
              (message) => updates(message as PreviewRfqPublicationRequest))
          as PreviewRfqPublicationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PreviewRfqPublicationRequest() / PreviewRfqPublicationRequest.new instead')
  static PreviewRfqPublicationRequest create() =>
      PreviewRfqPublicationRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      PreviewRfqPublicationRequest._();
  @$core.override
  PreviewRfqPublicationRequest createEmptyInstance() =>
      PreviewRfqPublicationRequest._();
  @$core.pragma('dart2js:noInline')
  static PreviewRfqPublicationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PreviewRfqPublicationRequest>(
          PreviewRfqPublicationRequest.$_createMessage);
  static PreviewRfqPublicationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  /// 與 ExecuteRfqPublicationRequest 的 client_request_id 是不同的鍵。
  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get expectedRevisionId => $_getSZ(2);
  @$pb.TagNumber(3)
  set expectedRevisionId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasExpectedRevisionId() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpectedRevisionId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get confirmedRfqSnapshotId => $_getSZ(3);
  @$pb.TagNumber(4)
  set confirmedRfqSnapshotId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasConfirmedRfqSnapshotId() => $_has(3);
  @$pb.TagNumber(4)
  void clearConfirmedRfqSnapshotId() => $_clearField(4);

  @$pb.TagNumber(5)
  AudienceMode get audienceMode => $_getN(4);
  @$pb.TagNumber(5)
  set audienceMode(AudienceMode value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasAudienceMode() => $_has(4);
  @$pb.TagNumber(5)
  void clearAudienceMode() => $_clearField(5);

  /// AUDIENCE_MODE_DIRECT_RECIPIENTS 必填。
  @$pb.TagNumber(6)
  $core.String get candidateSetId => $_getSZ(5);
  @$pb.TagNumber(6)
  set candidateSetId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCandidateSetId() => $_has(5);
  @$pb.TagNumber(6)
  void clearCandidateSetId() => $_clearField(6);

  /// AUDIENCE_MODE_DIRECT_RECIPIENTS 必填，順序有意義。
  @$pb.TagNumber(7)
  $pb.PbList<$core.String> get selectedCandidateIds => $_getList(6);

  /// 賣家回價的期限；沒有設定表示不設期限。
  @$pb.TagNumber(8)
  $0.Date get responseDeadline => $_getN(7);
  @$pb.TagNumber(8)
  set responseDeadline($0.Date value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasResponseDeadline() => $_has(7);
  @$pb.TagNumber(8)
  void clearResponseDeadline() => $_clearField(8);
  @$pb.TagNumber(8)
  $0.Date ensureResponseDeadline() => $_ensure(7);
}

class PreviewRfqPublicationResponse extends $pb.GeneratedMessage {
  factory PreviewRfqPublicationResponse({
    $core.String? approvalId,
    $core.String? approvalVersion,
    $core.String? audienceHash,
    $core.String? sellerFacingPayloadHash,
    SellerFacingRfq? sellerFacingPayload,
    $1.Timestamp? expireTime,
    $core.String? audiencePolicyId,
    $core.int? audiencePolicyVersion,
    $core.int? recipientCount,
  }) {
    final result = PreviewRfqPublicationResponse._();
    if (approvalId != null) result.approvalId = approvalId;
    if (approvalVersion != null) result.approvalVersion = approvalVersion;
    if (audienceHash != null) result.audienceHash = audienceHash;
    if (sellerFacingPayloadHash != null)
      result.sellerFacingPayloadHash = sellerFacingPayloadHash;
    if (sellerFacingPayload != null)
      result.sellerFacingPayload = sellerFacingPayload;
    if (expireTime != null) result.expireTime = expireTime;
    if (audiencePolicyId != null) result.audiencePolicyId = audiencePolicyId;
    if (audiencePolicyVersion != null)
      result.audiencePolicyVersion = audiencePolicyVersion;
    if (recipientCount != null) result.recipientCount = recipientCount;
    return result;
  }

  PreviewRfqPublicationResponse._();

  factory PreviewRfqPublicationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewRfqPublicationResponse()..mergeFromBuffer(data, registry);
  factory PreviewRfqPublicationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PreviewRfqPublicationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PreviewRfqPublicationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: PreviewRfqPublicationResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'approvalId')
    ..aOS(2, _omitFieldNames ? '' : 'approvalVersion')
    ..aOS(3, _omitFieldNames ? '' : 'audienceHash')
    ..aOS(4, _omitFieldNames ? '' : 'sellerFacingPayloadHash')
    ..aOM<SellerFacingRfq>(5, _omitFieldNames ? '' : 'sellerFacingPayload',
        subBuilder: SellerFacingRfq.$_createMessage)
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'expireTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'audiencePolicyId')
    ..aI(8, _omitFieldNames ? '' : 'audiencePolicyVersion')
    ..aI(9, _omitFieldNames ? '' : 'recipientCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewRfqPublicationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PreviewRfqPublicationResponse copyWith(
          void Function(PreviewRfqPublicationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as PreviewRfqPublicationResponse))
          as PreviewRfqPublicationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PreviewRfqPublicationResponse() / PreviewRfqPublicationResponse.new instead')
  static PreviewRfqPublicationResponse create() =>
      PreviewRfqPublicationResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      PreviewRfqPublicationResponse._();
  @$core.override
  PreviewRfqPublicationResponse createEmptyInstance() =>
      PreviewRfqPublicationResponse._();
  @$core.pragma('dart2js:noInline')
  static PreviewRfqPublicationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PreviewRfqPublicationResponse>(
          PreviewRfqPublicationResponse.$_createMessage);
  static PreviewRfqPublicationResponse? _defaultInstance;

  /// 執行時以 approval_id 與 approval_version 做版本檢查，這裡是唯一的來源。
  @$pb.TagNumber(1)
  $core.String get approvalId => $_getSZ(0);
  @$pb.TagNumber(1)
  set approvalId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasApprovalId() => $_has(0);
  @$pb.TagNumber(1)
  void clearApprovalId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get approvalVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set approvalVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApprovalVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearApprovalVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get audienceHash => $_getSZ(2);
  @$pb.TagNumber(3)
  set audienceHash($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAudienceHash() => $_has(2);
  @$pb.TagNumber(3)
  void clearAudienceHash() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get sellerFacingPayloadHash => $_getSZ(3);
  @$pb.TagNumber(4)
  set sellerFacingPayloadHash($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSellerFacingPayloadHash() => $_has(3);
  @$pb.TagNumber(4)
  void clearSellerFacingPayloadHash() => $_clearField(4);

  /// 將要發給賣家的內容。
  @$pb.TagNumber(5)
  SellerFacingRfq get sellerFacingPayload => $_getN(4);
  @$pb.TagNumber(5)
  set sellerFacingPayload(SellerFacingRfq value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasSellerFacingPayload() => $_has(4);
  @$pb.TagNumber(5)
  void clearSellerFacingPayload() => $_clearField(5);
  @$pb.TagNumber(5)
  SellerFacingRfq ensureSellerFacingPayload() => $_ensure(4);

  @$pb.TagNumber(6)
  $1.Timestamp get expireTime => $_getN(5);
  @$pb.TagNumber(6)
  set expireTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasExpireTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearExpireTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureExpireTime() => $_ensure(5);

  /// 受眾政策的身分，讓買家介面講得出這會發給誰；公開池目前是全站未停權的賣家，沒有分類、地區或分級的限制。
  /// AUDIENCE_MODE_DIRECT_RECIPIENTS 時不設。
  @$pb.TagNumber(7)
  $core.String get audiencePolicyId => $_getSZ(6);
  @$pb.TagNumber(7)
  set audiencePolicyId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasAudiencePolicyId() => $_has(6);
  @$pb.TagNumber(7)
  void clearAudiencePolicyId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.int get audiencePolicyVersion => $_getIZ(7);
  @$pb.TagNumber(8)
  set audiencePolicyVersion($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAudiencePolicyVersion() => $_has(7);
  @$pb.TagNumber(8)
  void clearAudiencePolicyVersion() => $_clearField(8);

  /// 指定對象時是選定的家數；公開池是 0。
  @$pb.TagNumber(9)
  $core.int get recipientCount => $_getIZ(8);
  @$pb.TagNumber(9)
  set recipientCount($core.int value) => $_setSignedInt32(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRecipientCount() => $_has(8);
  @$pb.TagNumber(9)
  void clearRecipientCount() => $_clearField(9);
}

class ExecuteRfqPublicationRequest extends $pb.GeneratedMessage {
  factory ExecuteRfqPublicationRequest({
    $core.String? requestId,
    $core.String? clientRequestId,
    $core.String? approvalId,
    $core.String? approvalVersion,
    $core.String? expectedRevisionId,
    $core.String? payloadHash,
  }) {
    final result = ExecuteRfqPublicationRequest._();
    if (requestId != null) result.requestId = requestId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (approvalId != null) result.approvalId = approvalId;
    if (approvalVersion != null) result.approvalVersion = approvalVersion;
    if (expectedRevisionId != null)
      result.expectedRevisionId = expectedRevisionId;
    if (payloadHash != null) result.payloadHash = payloadHash;
    return result;
  }

  ExecuteRfqPublicationRequest._();

  factory ExecuteRfqPublicationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExecuteRfqPublicationRequest()..mergeFromBuffer(data, registry);
  factory ExecuteRfqPublicationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExecuteRfqPublicationRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExecuteRfqPublicationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ExecuteRfqPublicationRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(3, _omitFieldNames ? '' : 'approvalId')
    ..aOS(4, _omitFieldNames ? '' : 'approvalVersion')
    ..aOS(5, _omitFieldNames ? '' : 'expectedRevisionId')
    ..aOS(6, _omitFieldNames ? '' : 'payloadHash')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExecuteRfqPublicationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExecuteRfqPublicationRequest copyWith(
          void Function(ExecuteRfqPublicationRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ExecuteRfqPublicationRequest))
          as ExecuteRfqPublicationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ExecuteRfqPublicationRequest() / ExecuteRfqPublicationRequest.new instead')
  static ExecuteRfqPublicationRequest create() =>
      ExecuteRfqPublicationRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ExecuteRfqPublicationRequest._();
  @$core.override
  ExecuteRfqPublicationRequest createEmptyInstance() =>
      ExecuteRfqPublicationRequest._();
  @$core.pragma('dart2js:noInline')
  static ExecuteRfqPublicationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExecuteRfqPublicationRequest>(
          ExecuteRfqPublicationRequest.$_createMessage);
  static ExecuteRfqPublicationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get approvalId => $_getSZ(2);
  @$pb.TagNumber(3)
  set approvalId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasApprovalId() => $_has(2);
  @$pb.TagNumber(3)
  void clearApprovalId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get approvalVersion => $_getSZ(3);
  @$pb.TagNumber(4)
  set approvalVersion($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasApprovalVersion() => $_has(3);
  @$pb.TagNumber(4)
  void clearApprovalVersion() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get expectedRevisionId => $_getSZ(4);
  @$pb.TagNumber(5)
  set expectedRevisionId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpectedRevisionId() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpectedRevisionId() => $_clearField(5);

  /// 預覽回應的 seller_facing_payload_hash。
  @$pb.TagNumber(6)
  $core.String get payloadHash => $_getSZ(5);
  @$pb.TagNumber(6)
  set payloadHash($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasPayloadHash() => $_has(5);
  @$pb.TagNumber(6)
  void clearPayloadHash() => $_clearField(6);
}

class ExecuteRfqPublicationResponse extends $pb.GeneratedMessage {
  factory ExecuteRfqPublicationResponse({
    $core.String? publicationId,
    $core.String? publicationVersion,
    PublicationState? state,
    AudienceMode? audienceMode,
    $core.String? audienceHash,
    $core.String? sellerFacingPayloadHash,
    $1.Timestamp? publishTime,
    $core.bool? replayed,
  }) {
    final result = ExecuteRfqPublicationResponse._();
    if (publicationId != null) result.publicationId = publicationId;
    if (publicationVersion != null)
      result.publicationVersion = publicationVersion;
    if (state != null) result.state = state;
    if (audienceMode != null) result.audienceMode = audienceMode;
    if (audienceHash != null) result.audienceHash = audienceHash;
    if (sellerFacingPayloadHash != null)
      result.sellerFacingPayloadHash = sellerFacingPayloadHash;
    if (publishTime != null) result.publishTime = publishTime;
    if (replayed != null) result.replayed = replayed;
    return result;
  }

  ExecuteRfqPublicationResponse._();

  factory ExecuteRfqPublicationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExecuteRfqPublicationResponse()..mergeFromBuffer(data, registry);
  factory ExecuteRfqPublicationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExecuteRfqPublicationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExecuteRfqPublicationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ExecuteRfqPublicationResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'publicationId')
    ..aOS(2, _omitFieldNames ? '' : 'publicationVersion')
    ..aE<PublicationState>(3, _omitFieldNames ? '' : 'state',
        enumValues: PublicationState.values)
    ..aE<AudienceMode>(4, _omitFieldNames ? '' : 'audienceMode',
        enumValues: AudienceMode.values)
    ..aOS(5, _omitFieldNames ? '' : 'audienceHash')
    ..aOS(6, _omitFieldNames ? '' : 'sellerFacingPayloadHash')
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOB(8, _omitFieldNames ? '' : 'replayed')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExecuteRfqPublicationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExecuteRfqPublicationResponse copyWith(
          void Function(ExecuteRfqPublicationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ExecuteRfqPublicationResponse))
          as ExecuteRfqPublicationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ExecuteRfqPublicationResponse() / ExecuteRfqPublicationResponse.new instead')
  static ExecuteRfqPublicationResponse create() =>
      ExecuteRfqPublicationResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ExecuteRfqPublicationResponse._();
  @$core.override
  ExecuteRfqPublicationResponse createEmptyInstance() =>
      ExecuteRfqPublicationResponse._();
  @$core.pragma('dart2js:noInline')
  static ExecuteRfqPublicationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExecuteRfqPublicationResponse>(
          ExecuteRfqPublicationResponse.$_createMessage);
  static ExecuteRfqPublicationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get publicationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set publicationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPublicationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublicationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get publicationVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set publicationVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPublicationVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublicationVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  PublicationState get state => $_getN(2);
  @$pb.TagNumber(3)
  set state(PublicationState value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasState() => $_has(2);
  @$pb.TagNumber(3)
  void clearState() => $_clearField(3);

  @$pb.TagNumber(4)
  AudienceMode get audienceMode => $_getN(3);
  @$pb.TagNumber(4)
  set audienceMode(AudienceMode value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasAudienceMode() => $_has(3);
  @$pb.TagNumber(4)
  void clearAudienceMode() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get audienceHash => $_getSZ(4);
  @$pb.TagNumber(5)
  set audienceHash($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAudienceHash() => $_has(4);
  @$pb.TagNumber(5)
  void clearAudienceHash() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get sellerFacingPayloadHash => $_getSZ(5);
  @$pb.TagNumber(6)
  set sellerFacingPayloadHash($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSellerFacingPayloadHash() => $_has(5);
  @$pb.TagNumber(6)
  void clearSellerFacingPayloadHash() => $_clearField(6);

  @$pb.TagNumber(7)
  $1.Timestamp get publishTime => $_getN(6);
  @$pb.TagNumber(7)
  set publishTime($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasPublishTime() => $_has(6);
  @$pb.TagNumber(7)
  void clearPublishTime() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensurePublishTime() => $_ensure(6);

  /// 為 true 表示這是冪等的重送，沒有建立新的發布。
  @$pb.TagNumber(8)
  $core.bool get replayed => $_getBF(7);
  @$pb.TagNumber(8)
  set replayed($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasReplayed() => $_has(7);
  @$pb.TagNumber(8)
  void clearReplayed() => $_clearField(8);
}

class ListConversationsRequest extends $pb.GeneratedMessage {
  factory ListConversationsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListConversationsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListConversationsRequest._();

  factory ListConversationsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListConversationsRequest()..mergeFromBuffer(data, registry);
  factory ListConversationsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListConversationsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListConversationsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ListConversationsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListConversationsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListConversationsRequest copyWith(
          void Function(ListConversationsRequest) updates) =>
      super.copyWith((message) => updates(message as ListConversationsRequest))
          as ListConversationsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListConversationsRequest() / ListConversationsRequest.new instead')
  static ListConversationsRequest create() => ListConversationsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListConversationsRequest._();
  @$core.override
  ListConversationsRequest createEmptyInstance() =>
      ListConversationsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListConversationsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListConversationsRequest>(
          ListConversationsRequest.$_createMessage);
  static ListConversationsRequest? _defaultInstance;

  /// 每頁最多幾筆。0 表示 20；超過 50 時以 50 計；負數回 INVALID_ARGUMENT。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  /// 上一頁回應的 next_page_token；第一頁留空。
  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListConversationsResponse extends $pb.GeneratedMessage {
  factory ListConversationsResponse({
    $core.Iterable<ConversationSummary>? conversations,
    $core.String? nextPageToken,
  }) {
    final result = ListConversationsResponse._();
    if (conversations != null) result.conversations.addAll(conversations);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListConversationsResponse._();

  factory ListConversationsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListConversationsResponse()..mergeFromBuffer(data, registry);
  factory ListConversationsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListConversationsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListConversationsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ListConversationsResponse.$_createMessage)
    ..pPM<ConversationSummary>(1, _omitFieldNames ? '' : 'conversations',
        subBuilder: ConversationSummary.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListConversationsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListConversationsResponse copyWith(
          void Function(ListConversationsResponse) updates) =>
      super.copyWith((message) => updates(message as ListConversationsResponse))
          as ListConversationsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListConversationsResponse() / ListConversationsResponse.new instead')
  static ListConversationsResponse create() => ListConversationsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListConversationsResponse._();
  @$core.override
  ListConversationsResponse createEmptyInstance() =>
      ListConversationsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListConversationsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListConversationsResponse>(
          ListConversationsResponse.$_createMessage);
  static ListConversationsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ConversationSummary> get conversations => $_getList(0);

  /// 下一頁的 token；空字串表示這是最後一頁。
  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetConversationMessagesRequest extends $pb.GeneratedMessage {
  factory GetConversationMessagesRequest({
    $core.String? conversationId,
  }) {
    final result = GetConversationMessagesRequest._();
    if (conversationId != null) result.conversationId = conversationId;
    return result;
  }

  GetConversationMessagesRequest._();

  factory GetConversationMessagesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetConversationMessagesRequest()..mergeFromBuffer(data, registry);
  factory GetConversationMessagesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetConversationMessagesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetConversationMessagesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetConversationMessagesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetConversationMessagesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetConversationMessagesRequest copyWith(
          void Function(GetConversationMessagesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetConversationMessagesRequest))
          as GetConversationMessagesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetConversationMessagesRequest() / GetConversationMessagesRequest.new instead')
  static GetConversationMessagesRequest create() =>
      GetConversationMessagesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetConversationMessagesRequest._();
  @$core.override
  GetConversationMessagesRequest createEmptyInstance() =>
      GetConversationMessagesRequest._();
  @$core.pragma('dart2js:noInline')
  static GetConversationMessagesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetConversationMessagesRequest>(
          GetConversationMessagesRequest.$_createMessage);
  static GetConversationMessagesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);
}

class GetConversationMessagesResponse extends $pb.GeneratedMessage {
  factory GetConversationMessagesResponse({
    $core.String? conversationId,
    $core.String? requestId,
    RequestStatus? status,
    $core.String? title,
    $core.Iterable<ConversationMessage>? messages,
  }) {
    final result = GetConversationMessagesResponse._();
    if (conversationId != null) result.conversationId = conversationId;
    if (requestId != null) result.requestId = requestId;
    if (status != null) result.status = status;
    if (title != null) result.title = title;
    if (messages != null) result.messages.addAll(messages);
    return result;
  }

  GetConversationMessagesResponse._();

  factory GetConversationMessagesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetConversationMessagesResponse()..mergeFromBuffer(data, registry);
  factory GetConversationMessagesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetConversationMessagesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetConversationMessagesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetConversationMessagesResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aE<RequestStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: RequestStatus.values)
    ..aOS(4, _omitFieldNames ? '' : 'title')
    ..pPM<ConversationMessage>(5, _omitFieldNames ? '' : 'messages',
        subBuilder: ConversationMessage.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetConversationMessagesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetConversationMessagesResponse copyWith(
          void Function(GetConversationMessagesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetConversationMessagesResponse))
          as GetConversationMessagesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetConversationMessagesResponse() / GetConversationMessagesResponse.new instead')
  static GetConversationMessagesResponse create() =>
      GetConversationMessagesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetConversationMessagesResponse._();
  @$core.override
  GetConversationMessagesResponse createEmptyInstance() =>
      GetConversationMessagesResponse._();
  @$core.pragma('dart2js:noInline')
  static GetConversationMessagesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetConversationMessagesResponse>(
          GetConversationMessagesResponse.$_createMessage);
  static GetConversationMessagesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  RequestStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(RequestStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get title => $_getSZ(3);
  @$pb.TagNumber(4)
  set title($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTitle() => $_has(3);
  @$pb.TagNumber(4)
  void clearTitle() => $_clearField(4);

  /// 時間由舊到新。
  @$pb.TagNumber(5)
  $pb.PbList<ConversationMessage> get messages => $_getList(4);
}

class GetRuntimeSnapshotRequest extends $pb.GeneratedMessage {
  factory GetRuntimeSnapshotRequest({
    $core.String? conversationId,
  }) {
    final result = GetRuntimeSnapshotRequest._();
    if (conversationId != null) result.conversationId = conversationId;
    return result;
  }

  GetRuntimeSnapshotRequest._();

  factory GetRuntimeSnapshotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetRuntimeSnapshotRequest()..mergeFromBuffer(data, registry);
  factory GetRuntimeSnapshotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetRuntimeSnapshotRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetRuntimeSnapshotRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetRuntimeSnapshotRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRuntimeSnapshotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRuntimeSnapshotRequest copyWith(
          void Function(GetRuntimeSnapshotRequest) updates) =>
      super.copyWith((message) => updates(message as GetRuntimeSnapshotRequest))
          as GetRuntimeSnapshotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetRuntimeSnapshotRequest() / GetRuntimeSnapshotRequest.new instead')
  static GetRuntimeSnapshotRequest create() => GetRuntimeSnapshotRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetRuntimeSnapshotRequest._();
  @$core.override
  GetRuntimeSnapshotRequest createEmptyInstance() =>
      GetRuntimeSnapshotRequest._();
  @$core.pragma('dart2js:noInline')
  static GetRuntimeSnapshotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetRuntimeSnapshotRequest>(
          GetRuntimeSnapshotRequest.$_createMessage);
  static GetRuntimeSnapshotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);
}

class GetRuntimeSnapshotResponse extends $pb.GeneratedMessage {
  factory GetRuntimeSnapshotResponse({
    $core.String? conversationId,
    $core.String? requestId,
    RequestStatus? status,
    $core.String? title,
    $core.Iterable<ConversationMessage>? messages,
    $core.Iterable<RunSummary>? runs,
    $core.String? activeRunId,
    OpenConfirmation? openConfirmation,
    PublishedRfq? publishedRfq,
  }) {
    final result = GetRuntimeSnapshotResponse._();
    if (conversationId != null) result.conversationId = conversationId;
    if (requestId != null) result.requestId = requestId;
    if (status != null) result.status = status;
    if (title != null) result.title = title;
    if (messages != null) result.messages.addAll(messages);
    if (runs != null) result.runs.addAll(runs);
    if (activeRunId != null) result.activeRunId = activeRunId;
    if (openConfirmation != null) result.openConfirmation = openConfirmation;
    if (publishedRfq != null) result.publishedRfq = publishedRfq;
    return result;
  }

  GetRuntimeSnapshotResponse._();

  factory GetRuntimeSnapshotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetRuntimeSnapshotResponse()..mergeFromBuffer(data, registry);
  factory GetRuntimeSnapshotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetRuntimeSnapshotResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetRuntimeSnapshotResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetRuntimeSnapshotResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aE<RequestStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: RequestStatus.values)
    ..aOS(4, _omitFieldNames ? '' : 'title')
    ..pPM<ConversationMessage>(5, _omitFieldNames ? '' : 'messages',
        subBuilder: ConversationMessage.$_createMessage)
    ..pPM<RunSummary>(6, _omitFieldNames ? '' : 'runs',
        subBuilder: RunSummary.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'activeRunId')
    ..aOM<OpenConfirmation>(8, _omitFieldNames ? '' : 'openConfirmation',
        subBuilder: OpenConfirmation.$_createMessage)
    ..aOM<PublishedRfq>(9, _omitFieldNames ? '' : 'publishedRfq',
        subBuilder: PublishedRfq.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRuntimeSnapshotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRuntimeSnapshotResponse copyWith(
          void Function(GetRuntimeSnapshotResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetRuntimeSnapshotResponse))
          as GetRuntimeSnapshotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetRuntimeSnapshotResponse() / GetRuntimeSnapshotResponse.new instead')
  static GetRuntimeSnapshotResponse create() => GetRuntimeSnapshotResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetRuntimeSnapshotResponse._();
  @$core.override
  GetRuntimeSnapshotResponse createEmptyInstance() =>
      GetRuntimeSnapshotResponse._();
  @$core.pragma('dart2js:noInline')
  static GetRuntimeSnapshotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetRuntimeSnapshotResponse>(
          GetRuntimeSnapshotResponse.$_createMessage);
  static GetRuntimeSnapshotResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  RequestStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(RequestStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get title => $_getSZ(3);
  @$pb.TagNumber(4)
  set title($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTitle() => $_has(3);
  @$pb.TagNumber(4)
  void clearTitle() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<ConversationMessage> get messages => $_getList(4);

  @$pb.TagNumber(6)
  $pb.PbList<RunSummary> get runs => $_getList(5);

  @$pb.TagNumber(7)
  $core.String get activeRunId => $_getSZ(6);
  @$pb.TagNumber(7)
  set activeRunId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasActiveRunId() => $_has(6);
  @$pb.TagNumber(7)
  void clearActiveRunId() => $_clearField(7);

  @$pb.TagNumber(8)
  OpenConfirmation get openConfirmation => $_getN(7);
  @$pb.TagNumber(8)
  set openConfirmation(OpenConfirmation value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasOpenConfirmation() => $_has(7);
  @$pb.TagNumber(8)
  void clearOpenConfirmation() => $_clearField(8);
  @$pb.TagNumber(8)
  OpenConfirmation ensureOpenConfirmation() => $_ensure(7);

  /// 最新一次的發布結果；還沒有發布時不設。
  @$pb.TagNumber(9)
  PublishedRfq get publishedRfq => $_getN(8);
  @$pb.TagNumber(9)
  set publishedRfq(PublishedRfq value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasPublishedRfq() => $_has(8);
  @$pb.TagNumber(9)
  void clearPublishedRfq() => $_clearField(9);
  @$pb.TagNumber(9)
  PublishedRfq ensurePublishedRfq() => $_ensure(8);
}

class RenameConversationRequest extends $pb.GeneratedMessage {
  factory RenameConversationRequest({
    $core.String? conversationId,
    $core.String? title,
    $core.String? clientRequestId,
  }) {
    final result = RenameConversationRequest._();
    if (conversationId != null) result.conversationId = conversationId;
    if (title != null) result.title = title;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    return result;
  }

  RenameConversationRequest._();

  factory RenameConversationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RenameConversationRequest()..mergeFromBuffer(data, registry);
  factory RenameConversationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RenameConversationRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RenameConversationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RenameConversationRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..aOS(3, _omitFieldNames ? '' : 'clientRequestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RenameConversationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RenameConversationRequest copyWith(
          void Function(RenameConversationRequest) updates) =>
      super.copyWith((message) => updates(message as RenameConversationRequest))
          as RenameConversationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RenameConversationRequest() / RenameConversationRequest.new instead')
  static RenameConversationRequest create() => RenameConversationRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      RenameConversationRequest._();
  @$core.override
  RenameConversationRequest createEmptyInstance() =>
      RenameConversationRequest._();
  @$core.pragma('dart2js:noInline')
  static RenameConversationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RenameConversationRequest>(
          RenameConversationRequest.$_createMessage);
  static RenameConversationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);

  /// 不能是空的，也不能超過長度上限。
  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get clientRequestId => $_getSZ(2);
  @$pb.TagNumber(3)
  set clientRequestId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasClientRequestId() => $_has(2);
  @$pb.TagNumber(3)
  void clearClientRequestId() => $_clearField(3);
}

class RenameConversationResponse extends $pb.GeneratedMessage {
  factory RenameConversationResponse() => RenameConversationResponse._();

  RenameConversationResponse._();

  factory RenameConversationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RenameConversationResponse()..mergeFromBuffer(data, registry);
  factory RenameConversationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RenameConversationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RenameConversationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RenameConversationResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RenameConversationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RenameConversationResponse copyWith(
          void Function(RenameConversationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RenameConversationResponse))
          as RenameConversationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RenameConversationResponse() / RenameConversationResponse.new instead')
  static RenameConversationResponse create() => RenameConversationResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      RenameConversationResponse._();
  @$core.override
  RenameConversationResponse createEmptyInstance() =>
      RenameConversationResponse._();
  @$core.pragma('dart2js:noInline')
  static RenameConversationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RenameConversationResponse>(
          RenameConversationResponse.$_createMessage);
  static RenameConversationResponse? _defaultInstance;
}

class DeleteConversationRequest extends $pb.GeneratedMessage {
  factory DeleteConversationRequest({
    $core.String? conversationId,
    $core.String? clientRequestId,
  }) {
    final result = DeleteConversationRequest._();
    if (conversationId != null) result.conversationId = conversationId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    return result;
  }

  DeleteConversationRequest._();

  factory DeleteConversationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteConversationRequest()..mergeFromBuffer(data, registry);
  factory DeleteConversationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteConversationRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteConversationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: DeleteConversationRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteConversationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteConversationRequest copyWith(
          void Function(DeleteConversationRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteConversationRequest))
          as DeleteConversationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteConversationRequest() / DeleteConversationRequest.new instead')
  static DeleteConversationRequest create() => DeleteConversationRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      DeleteConversationRequest._();
  @$core.override
  DeleteConversationRequest createEmptyInstance() =>
      DeleteConversationRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteConversationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteConversationRequest>(
          DeleteConversationRequest.$_createMessage);
  static DeleteConversationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);
}

class DeleteConversationResponse extends $pb.GeneratedMessage {
  factory DeleteConversationResponse() => DeleteConversationResponse._();

  DeleteConversationResponse._();

  factory DeleteConversationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteConversationResponse()..mergeFromBuffer(data, registry);
  factory DeleteConversationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteConversationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteConversationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: DeleteConversationResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteConversationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteConversationResponse copyWith(
          void Function(DeleteConversationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as DeleteConversationResponse))
          as DeleteConversationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteConversationResponse() / DeleteConversationResponse.new instead')
  static DeleteConversationResponse create() => DeleteConversationResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      DeleteConversationResponse._();
  @$core.override
  DeleteConversationResponse createEmptyInstance() =>
      DeleteConversationResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteConversationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteConversationResponse>(
          DeleteConversationResponse.$_createMessage);
  static DeleteConversationResponse? _defaultInstance;
}

class SellerQuote extends $pb.GeneratedMessage {
  factory SellerQuote({
    SellerResponseType? responseType,
    OfferedQuality? offeredQuality,
    UnitBasis? unitBasis,
    $core.String? offeredQuantity,
    $core.String? offeredQuantityUnit,
    $core.String? conversionBasis,
    $fixnum.Int64? goodsAmount,
    $fixnum.Int64? shippingAmount,
    $fixnum.Int64? taxAmount,
    TaxInclusionBasis? taxInclusionBasis,
    $fixnum.Int64? totalAmount,
    $core.int? leadTimeDays,
    $1.Timestamp? promisedDeliveryTime,
    $1.Timestamp? validUntilTime,
    $core.String? termsNote,
    $core.String? deviationDisclosure,
  }) {
    final result = SellerQuote._();
    if (responseType != null) result.responseType = responseType;
    if (offeredQuality != null) result.offeredQuality = offeredQuality;
    if (unitBasis != null) result.unitBasis = unitBasis;
    if (offeredQuantity != null) result.offeredQuantity = offeredQuantity;
    if (offeredQuantityUnit != null)
      result.offeredQuantityUnit = offeredQuantityUnit;
    if (conversionBasis != null) result.conversionBasis = conversionBasis;
    if (goodsAmount != null) result.goodsAmount = goodsAmount;
    if (shippingAmount != null) result.shippingAmount = shippingAmount;
    if (taxAmount != null) result.taxAmount = taxAmount;
    if (taxInclusionBasis != null) result.taxInclusionBasis = taxInclusionBasis;
    if (totalAmount != null) result.totalAmount = totalAmount;
    if (leadTimeDays != null) result.leadTimeDays = leadTimeDays;
    if (promisedDeliveryTime != null)
      result.promisedDeliveryTime = promisedDeliveryTime;
    if (validUntilTime != null) result.validUntilTime = validUntilTime;
    if (termsNote != null) result.termsNote = termsNote;
    if (deviationDisclosure != null)
      result.deviationDisclosure = deviationDisclosure;
    return result;
  }

  SellerQuote._();

  factory SellerQuote.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerQuote()..mergeFromBuffer(data, registry);
  factory SellerQuote.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerQuote()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerQuote',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SellerQuote.$_createMessage)
    ..aE<SellerResponseType>(1, _omitFieldNames ? '' : 'responseType',
        enumValues: SellerResponseType.values)
    ..aOM<OfferedQuality>(2, _omitFieldNames ? '' : 'offeredQuality',
        subBuilder: OfferedQuality.$_createMessage)
    ..aOM<UnitBasis>(3, _omitFieldNames ? '' : 'unitBasis',
        subBuilder: UnitBasis.$_createMessage)
    ..aOS(4, _omitFieldNames ? '' : 'offeredQuantity')
    ..aOS(5, _omitFieldNames ? '' : 'offeredQuantityUnit')
    ..aOS(6, _omitFieldNames ? '' : 'conversionBasis')
    ..aInt64(7, _omitFieldNames ? '' : 'goodsAmount')
    ..aInt64(8, _omitFieldNames ? '' : 'shippingAmount')
    ..aInt64(9, _omitFieldNames ? '' : 'taxAmount')
    ..aE<TaxInclusionBasis>(10, _omitFieldNames ? '' : 'taxInclusionBasis',
        enumValues: TaxInclusionBasis.values)
    ..aInt64(11, _omitFieldNames ? '' : 'totalAmount')
    ..aI(12, _omitFieldNames ? '' : 'leadTimeDays')
    ..aOM<$1.Timestamp>(13, _omitFieldNames ? '' : 'promisedDeliveryTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$1.Timestamp>(14, _omitFieldNames ? '' : 'validUntilTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOS(15, _omitFieldNames ? '' : 'termsNote')
    ..aOS(16, _omitFieldNames ? '' : 'deviationDisclosure')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerQuote clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerQuote copyWith(void Function(SellerQuote) updates) =>
      super.copyWith((message) => updates(message as SellerQuote))
          as SellerQuote;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SellerQuote() / SellerQuote.new instead')
  static SellerQuote create() => SellerQuote._();
  static $pb.GeneratedMessage $_createMessage() => SellerQuote._();
  @$core.override
  SellerQuote createEmptyInstance() => SellerQuote._();
  @$core.pragma('dart2js:noInline')
  static SellerQuote getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SellerQuote>(
          SellerQuote.$_createMessage);
  static SellerQuote? _defaultInstance;

  @$pb.TagNumber(1)
  SellerResponseType get responseType => $_getN(0);
  @$pb.TagNumber(1)
  set responseType(SellerResponseType value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasResponseType() => $_has(0);
  @$pb.TagNumber(1)
  void clearResponseType() => $_clearField(1);

  /// 賣家實際提供的品相，必填；平台與 LLM 不代填。
  @$pb.TagNumber(2)
  OfferedQuality get offeredQuality => $_getN(1);
  @$pb.TagNumber(2)
  set offeredQuality(OfferedQuality value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOfferedQuality() => $_has(1);
  @$pb.TagNumber(2)
  void clearOfferedQuality() => $_clearField(2);
  @$pb.TagNumber(2)
  OfferedQuality ensureOfferedQuality() => $_ensure(1);

  @$pb.TagNumber(3)
  UnitBasis get unitBasis => $_getN(2);
  @$pb.TagNumber(3)
  set unitBasis(UnitBasis value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasUnitBasis() => $_has(2);
  @$pb.TagNumber(3)
  void clearUnitBasis() => $_clearField(3);
  @$pb.TagNumber(3)
  UnitBasis ensureUnitBasis() => $_ensure(2);

  /// 十進位字串。
  @$pb.TagNumber(4)
  $core.String get offeredQuantity => $_getSZ(3);
  @$pb.TagNumber(4)
  set offeredQuantity($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOfferedQuantity() => $_has(3);
  @$pb.TagNumber(4)
  void clearOfferedQuantity() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get offeredQuantityUnit => $_getSZ(4);
  @$pb.TagNumber(5)
  set offeredQuantityUnit($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasOfferedQuantityUnit() => $_has(4);
  @$pb.TagNumber(5)
  void clearOfferedQuantityUnit() => $_clearField(5);

  /// 賣家的單位換算說明。
  @$pb.TagNumber(6)
  $core.String get conversionBasis => $_getSZ(5);
  @$pb.TagNumber(6)
  set conversionBasis($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasConversionBasis() => $_has(5);
  @$pb.TagNumber(6)
  void clearConversionBasis() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get goodsAmount => $_getI64(6);
  @$pb.TagNumber(7)
  set goodsAmount($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasGoodsAmount() => $_has(6);
  @$pb.TagNumber(7)
  void clearGoodsAmount() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get shippingAmount => $_getI64(7);
  @$pb.TagNumber(8)
  set shippingAmount($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasShippingAmount() => $_has(7);
  @$pb.TagNumber(8)
  void clearShippingAmount() => $_clearField(8);

  @$pb.TagNumber(9)
  $fixnum.Int64 get taxAmount => $_getI64(8);
  @$pb.TagNumber(9)
  set taxAmount($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasTaxAmount() => $_has(8);
  @$pb.TagNumber(9)
  void clearTaxAmount() => $_clearField(9);

  @$pb.TagNumber(10)
  TaxInclusionBasis get taxInclusionBasis => $_getN(9);
  @$pb.TagNumber(10)
  set taxInclusionBasis(TaxInclusionBasis value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasTaxInclusionBasis() => $_has(9);
  @$pb.TagNumber(10)
  void clearTaxInclusionBasis() => $_clearField(10);

  /// 伺服器驗證等於 goods_amount + shipping_amount + tax_amount，不符回 INVALID_ARGUMENT。
  @$pb.TagNumber(11)
  $fixnum.Int64 get totalAmount => $_getI64(10);
  @$pb.TagNumber(11)
  set totalAmount($fixnum.Int64 value) => $_setInt64(10, value);
  @$pb.TagNumber(11)
  $core.bool hasTotalAmount() => $_has(10);
  @$pb.TagNumber(11)
  void clearTotalAmount() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.int get leadTimeDays => $_getIZ(11);
  @$pb.TagNumber(12)
  set leadTimeDays($core.int value) => $_setSignedInt32(11, value);
  @$pb.TagNumber(12)
  $core.bool hasLeadTimeDays() => $_has(11);
  @$pb.TagNumber(12)
  void clearLeadTimeDays() => $_clearField(12);

  @$pb.TagNumber(13)
  $1.Timestamp get promisedDeliveryTime => $_getN(12);
  @$pb.TagNumber(13)
  set promisedDeliveryTime($1.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasPromisedDeliveryTime() => $_has(12);
  @$pb.TagNumber(13)
  void clearPromisedDeliveryTime() => $_clearField(13);
  @$pb.TagNumber(13)
  $1.Timestamp ensurePromisedDeliveryTime() => $_ensure(12);

  @$pb.TagNumber(14)
  $1.Timestamp get validUntilTime => $_getN(13);
  @$pb.TagNumber(14)
  set validUntilTime($1.Timestamp value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasValidUntilTime() => $_has(13);
  @$pb.TagNumber(14)
  void clearValidUntilTime() => $_clearField(14);
  @$pb.TagNumber(14)
  $1.Timestamp ensureValidUntilTime() => $_ensure(13);

  @$pb.TagNumber(15)
  $core.String get termsNote => $_getSZ(14);
  @$pb.TagNumber(15)
  set termsNote($core.String value) => $_setString(14, value);
  @$pb.TagNumber(15)
  $core.bool hasTermsNote() => $_has(14);
  @$pb.TagNumber(15)
  void clearTermsNote() => $_clearField(15);

  @$pb.TagNumber(16)
  $core.String get deviationDisclosure => $_getSZ(15);
  @$pb.TagNumber(16)
  set deviationDisclosure($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasDeviationDisclosure() => $_has(15);
  @$pb.TagNumber(16)
  void clearDeviationDisclosure() => $_clearField(16);
}

class OfferedQuality extends $pb.GeneratedMessage {
  factory OfferedQuality({
    $core.String? grade,
    $core.String? inspectionNotes,
  }) {
    final result = OfferedQuality._();
    if (grade != null) result.grade = grade;
    if (inspectionNotes != null) result.inspectionNotes = inspectionNotes;
    return result;
  }

  OfferedQuality._();

  factory OfferedQuality.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OfferedQuality()..mergeFromBuffer(data, registry);
  factory OfferedQuality.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OfferedQuality()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OfferedQuality',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: OfferedQuality.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'grade')
    ..aOS(2, _omitFieldNames ? '' : 'inspectionNotes')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OfferedQuality clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OfferedQuality copyWith(void Function(OfferedQuality) updates) =>
      super.copyWith((message) => updates(message as OfferedQuality))
          as OfferedQuality;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use OfferedQuality() / OfferedQuality.new instead')
  static OfferedQuality create() => OfferedQuality._();
  static $pb.GeneratedMessage $_createMessage() => OfferedQuality._();
  @$core.override
  OfferedQuality createEmptyInstance() => OfferedQuality._();
  @$core.pragma('dart2js:noInline')
  static OfferedQuality getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<OfferedQuality>(
          OfferedQuality.$_createMessage);
  static OfferedQuality? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get grade => $_getSZ(0);
  @$pb.TagNumber(1)
  set grade($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGrade() => $_has(0);
  @$pb.TagNumber(1)
  void clearGrade() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get inspectionNotes => $_getSZ(1);
  @$pb.TagNumber(2)
  set inspectionNotes($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInspectionNotes() => $_has(1);
  @$pb.TagNumber(2)
  void clearInspectionNotes() => $_clearField(2);
}

class UnitBasis extends $pb.GeneratedMessage {
  factory UnitBasis({
    $core.String? quantity,
    $core.String? unit,
  }) {
    final result = UnitBasis._();
    if (quantity != null) result.quantity = quantity;
    if (unit != null) result.unit = unit;
    return result;
  }

  UnitBasis._();

  factory UnitBasis.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnitBasis()..mergeFromBuffer(data, registry);
  factory UnitBasis.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnitBasis()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnitBasis',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: UnitBasis.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'quantity')
    ..aOS(2, _omitFieldNames ? '' : 'unit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnitBasis clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnitBasis copyWith(void Function(UnitBasis) updates) =>
      super.copyWith((message) => updates(message as UnitBasis)) as UnitBasis;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UnitBasis() / UnitBasis.new instead')
  static UnitBasis create() => UnitBasis._();
  static $pb.GeneratedMessage $_createMessage() => UnitBasis._();
  @$core.override
  UnitBasis createEmptyInstance() => UnitBasis._();
  @$core.pragma('dart2js:noInline')
  static UnitBasis getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnitBasis>(UnitBasis.$_createMessage);
  static UnitBasis? _defaultInstance;

  /// 十進位字串。
  @$pb.TagNumber(1)
  $core.String get quantity => $_getSZ(0);
  @$pb.TagNumber(1)
  set quantity($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasQuantity() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuantity() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get unit => $_getSZ(1);
  @$pb.TagNumber(2)
  set unit($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUnit() => $_has(1);
  @$pb.TagNumber(2)
  void clearUnit() => $_clearField(2);
}

/// BuyerFacingSellerResponse 是買家看到的一份賣家回價；只有公開的店家資料，不含賣家的草稿與聯絡方式。
class BuyerFacingSellerResponse extends $pb.GeneratedMessage {
  factory BuyerFacingSellerResponse({
    $core.String? sellerResponseId,
    $core.String? sellerResponseVersion,
    SellerResponseStatus? status,
    $core.String? payloadHash,
    $1.Timestamp? submitTime,
    $2.StoreCard? store,
    SellerQuote? quote,
    $core.bool? outsideBuyerRequestedRange,
    $core.bool? shortlisted,
    QuoteAcceptability? acceptability,
  }) {
    final result = BuyerFacingSellerResponse._();
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (sellerResponseVersion != null)
      result.sellerResponseVersion = sellerResponseVersion;
    if (status != null) result.status = status;
    if (payloadHash != null) result.payloadHash = payloadHash;
    if (submitTime != null) result.submitTime = submitTime;
    if (store != null) result.store = store;
    if (quote != null) result.quote = quote;
    if (outsideBuyerRequestedRange != null)
      result.outsideBuyerRequestedRange = outsideBuyerRequestedRange;
    if (shortlisted != null) result.shortlisted = shortlisted;
    if (acceptability != null) result.acceptability = acceptability;
    return result;
  }

  BuyerFacingSellerResponse._();

  factory BuyerFacingSellerResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BuyerFacingSellerResponse()..mergeFromBuffer(data, registry);
  factory BuyerFacingSellerResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      BuyerFacingSellerResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BuyerFacingSellerResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: BuyerFacingSellerResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(2, _omitFieldNames ? '' : 'sellerResponseVersion')
    ..aE<SellerResponseStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: SellerResponseStatus.values)
    ..aOS(4, _omitFieldNames ? '' : 'payloadHash')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'submitTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<$2.StoreCard>(6, _omitFieldNames ? '' : 'store',
        subBuilder: $2.StoreCard.$_createMessage)
    ..aOM<SellerQuote>(7, _omitFieldNames ? '' : 'quote',
        subBuilder: SellerQuote.$_createMessage)
    ..aOB(8, _omitFieldNames ? '' : 'outsideBuyerRequestedRange')
    ..aOB(9, _omitFieldNames ? '' : 'shortlisted')
    ..aE<QuoteAcceptability>(10, _omitFieldNames ? '' : 'acceptability',
        enumValues: QuoteAcceptability.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BuyerFacingSellerResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BuyerFacingSellerResponse copyWith(
          void Function(BuyerFacingSellerResponse) updates) =>
      super.copyWith((message) => updates(message as BuyerFacingSellerResponse))
          as BuyerFacingSellerResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use BuyerFacingSellerResponse() / BuyerFacingSellerResponse.new instead')
  static BuyerFacingSellerResponse create() => BuyerFacingSellerResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      BuyerFacingSellerResponse._();
  @$core.override
  BuyerFacingSellerResponse createEmptyInstance() =>
      BuyerFacingSellerResponse._();
  @$core.pragma('dart2js:noInline')
  static BuyerFacingSellerResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BuyerFacingSellerResponse>(
          BuyerFacingSellerResponse.$_createMessage);
  static BuyerFacingSellerResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sellerResponseId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sellerResponseId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSellerResponseId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSellerResponseId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sellerResponseVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set sellerResponseVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerResponseVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerResponseVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  SellerResponseStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(SellerResponseStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get payloadHash => $_getSZ(3);
  @$pb.TagNumber(4)
  set payloadHash($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPayloadHash() => $_has(3);
  @$pb.TagNumber(4)
  void clearPayloadHash() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get submitTime => $_getN(4);
  @$pb.TagNumber(5)
  set submitTime($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasSubmitTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearSubmitTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureSubmitTime() => $_ensure(4);

  @$pb.TagNumber(6)
  $2.StoreCard get store => $_getN(5);
  @$pb.TagNumber(6)
  set store($2.StoreCard value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasStore() => $_has(5);
  @$pb.TagNumber(6)
  void clearStore() => $_clearField(6);
  @$pb.TagNumber(6)
  $2.StoreCard ensureStore() => $_ensure(5);

  @$pb.TagNumber(7)
  SellerQuote get quote => $_getN(6);
  @$pb.TagNumber(7)
  set quote(SellerQuote value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasQuote() => $_has(6);
  @$pb.TagNumber(7)
  void clearQuote() => $_clearField(7);
  @$pb.TagNumber(7)
  SellerQuote ensureQuote() => $_ensure(6);

  /// 報價超出買家預期的範圍；只標示，不修改賣家的金額。
  @$pb.TagNumber(8)
  $core.bool get outsideBuyerRequestedRange => $_getBF(7);
  @$pb.TagNumber(8)
  set outsideBuyerRequestedRange($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasOutsideBuyerRequestedRange() => $_has(7);
  @$pb.TagNumber(8)
  void clearOutsideBuyerRequestedRange() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get shortlisted => $_getBF(8);
  @$pb.TagNumber(9)
  set shortlisted($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasShortlisted() => $_has(8);
  @$pb.TagNumber(9)
  void clearShortlisted() => $_clearField(9);

  @$pb.TagNumber(10)
  QuoteAcceptability get acceptability => $_getN(9);
  @$pb.TagNumber(10)
  set acceptability(QuoteAcceptability value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasAcceptability() => $_has(9);
  @$pb.TagNumber(10)
  void clearAcceptability() => $_clearField(10);
}

class ListSellerResponsesRequest extends $pb.GeneratedMessage {
  factory ListSellerResponsesRequest({
    $core.String? requestId,
    $core.String? publicationId,
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListSellerResponsesRequest._();
    if (requestId != null) result.requestId = requestId;
    if (publicationId != null) result.publicationId = publicationId;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListSellerResponsesRequest._();

  factory ListSellerResponsesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSellerResponsesRequest()..mergeFromBuffer(data, registry);
  factory ListSellerResponsesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSellerResponsesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSellerResponsesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ListSellerResponsesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'publicationId')
    ..aI(3, _omitFieldNames ? '' : 'pageSize')
    ..aOS(4, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSellerResponsesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSellerResponsesRequest copyWith(
          void Function(ListSellerResponsesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListSellerResponsesRequest))
          as ListSellerResponsesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSellerResponsesRequest() / ListSellerResponsesRequest.new instead')
  static ListSellerResponsesRequest create() => ListSellerResponsesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListSellerResponsesRequest._();
  @$core.override
  ListSellerResponsesRequest createEmptyInstance() =>
      ListSellerResponsesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListSellerResponsesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSellerResponsesRequest>(
          ListSellerResponsesRequest.$_createMessage);
  static ListSellerResponsesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get publicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set publicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPublicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublicationId() => $_clearField(2);

  /// 每頁最多幾筆。0 表示 50；超過 100 時以 100 計；負數回 INVALID_ARGUMENT。
  @$pb.TagNumber(3)
  $core.int get pageSize => $_getIZ(2);
  @$pb.TagNumber(3)
  set pageSize($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageSize() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageSize() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get pageToken => $_getSZ(3);
  @$pb.TagNumber(4)
  set pageToken($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPageToken() => $_has(3);
  @$pb.TagNumber(4)
  void clearPageToken() => $_clearField(4);
}

class ListSellerResponsesResponse extends $pb.GeneratedMessage {
  factory ListSellerResponsesResponse({
    $core.Iterable<BuyerFacingSellerResponse>? sellerResponses,
    $core.String? nextPageToken,
  }) {
    final result = ListSellerResponsesResponse._();
    if (sellerResponses != null) result.sellerResponses.addAll(sellerResponses);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListSellerResponsesResponse._();

  factory ListSellerResponsesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSellerResponsesResponse()..mergeFromBuffer(data, registry);
  factory ListSellerResponsesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSellerResponsesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSellerResponsesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ListSellerResponsesResponse.$_createMessage)
    ..pPM<BuyerFacingSellerResponse>(
        1, _omitFieldNames ? '' : 'sellerResponses',
        subBuilder: BuyerFacingSellerResponse.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSellerResponsesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSellerResponsesResponse copyWith(
          void Function(ListSellerResponsesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListSellerResponsesResponse))
          as ListSellerResponsesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSellerResponsesResponse() / ListSellerResponsesResponse.new instead')
  static ListSellerResponsesResponse create() =>
      ListSellerResponsesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListSellerResponsesResponse._();
  @$core.override
  ListSellerResponsesResponse createEmptyInstance() =>
      ListSellerResponsesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListSellerResponsesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSellerResponsesResponse>(
          ListSellerResponsesResponse.$_createMessage);
  static ListSellerResponsesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<BuyerFacingSellerResponse> get sellerResponses => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class PutShortlistEntryRequest extends $pb.GeneratedMessage {
  factory PutShortlistEntryRequest({
    $core.String? requestId,
    $core.String? sellerResponseId,
    $core.String? sellerResponseVersion,
  }) {
    final result = PutShortlistEntryRequest._();
    if (requestId != null) result.requestId = requestId;
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (sellerResponseVersion != null)
      result.sellerResponseVersion = sellerResponseVersion;
    return result;
  }

  PutShortlistEntryRequest._();

  factory PutShortlistEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PutShortlistEntryRequest()..mergeFromBuffer(data, registry);
  factory PutShortlistEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PutShortlistEntryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PutShortlistEntryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: PutShortlistEntryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(3, _omitFieldNames ? '' : 'sellerResponseVersion')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PutShortlistEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PutShortlistEntryRequest copyWith(
          void Function(PutShortlistEntryRequest) updates) =>
      super.copyWith((message) => updates(message as PutShortlistEntryRequest))
          as PutShortlistEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PutShortlistEntryRequest() / PutShortlistEntryRequest.new instead')
  static PutShortlistEntryRequest create() => PutShortlistEntryRequest._();
  static $pb.GeneratedMessage $_createMessage() => PutShortlistEntryRequest._();
  @$core.override
  PutShortlistEntryRequest createEmptyInstance() =>
      PutShortlistEntryRequest._();
  @$core.pragma('dart2js:noInline')
  static PutShortlistEntryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PutShortlistEntryRequest>(
          PutShortlistEntryRequest.$_createMessage);
  static PutShortlistEntryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sellerResponseId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sellerResponseId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerResponseId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerResponseId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sellerResponseVersion => $_getSZ(2);
  @$pb.TagNumber(3)
  set sellerResponseVersion($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSellerResponseVersion() => $_has(2);
  @$pb.TagNumber(3)
  void clearSellerResponseVersion() => $_clearField(3);
}

class PutShortlistEntryResponse extends $pb.GeneratedMessage {
  factory PutShortlistEntryResponse({
    $core.String? sellerResponseId,
    $core.String? sellerResponseVersion,
    $core.bool? present,
  }) {
    final result = PutShortlistEntryResponse._();
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (sellerResponseVersion != null)
      result.sellerResponseVersion = sellerResponseVersion;
    if (present != null) result.present = present;
    return result;
  }

  PutShortlistEntryResponse._();

  factory PutShortlistEntryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PutShortlistEntryResponse()..mergeFromBuffer(data, registry);
  factory PutShortlistEntryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PutShortlistEntryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PutShortlistEntryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: PutShortlistEntryResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(2, _omitFieldNames ? '' : 'sellerResponseVersion')
    ..aOB(3, _omitFieldNames ? '' : 'present')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PutShortlistEntryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PutShortlistEntryResponse copyWith(
          void Function(PutShortlistEntryResponse) updates) =>
      super.copyWith((message) => updates(message as PutShortlistEntryResponse))
          as PutShortlistEntryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PutShortlistEntryResponse() / PutShortlistEntryResponse.new instead')
  static PutShortlistEntryResponse create() => PutShortlistEntryResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      PutShortlistEntryResponse._();
  @$core.override
  PutShortlistEntryResponse createEmptyInstance() =>
      PutShortlistEntryResponse._();
  @$core.pragma('dart2js:noInline')
  static PutShortlistEntryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PutShortlistEntryResponse>(
          PutShortlistEntryResponse.$_createMessage);
  static PutShortlistEntryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sellerResponseId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sellerResponseId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSellerResponseId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSellerResponseId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sellerResponseVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set sellerResponseVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerResponseVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerResponseVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get present => $_getBF(2);
  @$pb.TagNumber(3)
  set present($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPresent() => $_has(2);
  @$pb.TagNumber(3)
  void clearPresent() => $_clearField(3);
}

class DeleteShortlistEntryRequest extends $pb.GeneratedMessage {
  factory DeleteShortlistEntryRequest({
    $core.String? requestId,
    $core.String? sellerResponseId,
    $core.String? sellerResponseVersion,
  }) {
    final result = DeleteShortlistEntryRequest._();
    if (requestId != null) result.requestId = requestId;
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (sellerResponseVersion != null)
      result.sellerResponseVersion = sellerResponseVersion;
    return result;
  }

  DeleteShortlistEntryRequest._();

  factory DeleteShortlistEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteShortlistEntryRequest()..mergeFromBuffer(data, registry);
  factory DeleteShortlistEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteShortlistEntryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteShortlistEntryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: DeleteShortlistEntryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(3, _omitFieldNames ? '' : 'sellerResponseVersion')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteShortlistEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteShortlistEntryRequest copyWith(
          void Function(DeleteShortlistEntryRequest) updates) =>
      super.copyWith(
              (message) => updates(message as DeleteShortlistEntryRequest))
          as DeleteShortlistEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteShortlistEntryRequest() / DeleteShortlistEntryRequest.new instead')
  static DeleteShortlistEntryRequest create() =>
      DeleteShortlistEntryRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      DeleteShortlistEntryRequest._();
  @$core.override
  DeleteShortlistEntryRequest createEmptyInstance() =>
      DeleteShortlistEntryRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteShortlistEntryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteShortlistEntryRequest>(
          DeleteShortlistEntryRequest.$_createMessage);
  static DeleteShortlistEntryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sellerResponseId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sellerResponseId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerResponseId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerResponseId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sellerResponseVersion => $_getSZ(2);
  @$pb.TagNumber(3)
  set sellerResponseVersion($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSellerResponseVersion() => $_has(2);
  @$pb.TagNumber(3)
  void clearSellerResponseVersion() => $_clearField(3);
}

class DeleteShortlistEntryResponse extends $pb.GeneratedMessage {
  factory DeleteShortlistEntryResponse({
    $core.String? sellerResponseId,
    $core.String? sellerResponseVersion,
    $core.bool? present,
  }) {
    final result = DeleteShortlistEntryResponse._();
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (sellerResponseVersion != null)
      result.sellerResponseVersion = sellerResponseVersion;
    if (present != null) result.present = present;
    return result;
  }

  DeleteShortlistEntryResponse._();

  factory DeleteShortlistEntryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteShortlistEntryResponse()..mergeFromBuffer(data, registry);
  factory DeleteShortlistEntryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteShortlistEntryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteShortlistEntryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: DeleteShortlistEntryResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(2, _omitFieldNames ? '' : 'sellerResponseVersion')
    ..aOB(3, _omitFieldNames ? '' : 'present')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteShortlistEntryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteShortlistEntryResponse copyWith(
          void Function(DeleteShortlistEntryResponse) updates) =>
      super.copyWith(
              (message) => updates(message as DeleteShortlistEntryResponse))
          as DeleteShortlistEntryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteShortlistEntryResponse() / DeleteShortlistEntryResponse.new instead')
  static DeleteShortlistEntryResponse create() =>
      DeleteShortlistEntryResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      DeleteShortlistEntryResponse._();
  @$core.override
  DeleteShortlistEntryResponse createEmptyInstance() =>
      DeleteShortlistEntryResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteShortlistEntryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteShortlistEntryResponse>(
          DeleteShortlistEntryResponse.$_createMessage);
  static DeleteShortlistEntryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sellerResponseId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sellerResponseId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSellerResponseId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSellerResponseId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sellerResponseVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set sellerResponseVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerResponseVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerResponseVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get present => $_getBF(2);
  @$pb.TagNumber(3)
  set present($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPresent() => $_has(2);
  @$pb.TagNumber(3)
  void clearPresent() => $_clearField(3);
}

class AcceptSellerQuoteRequest extends $pb.GeneratedMessage {
  factory AcceptSellerQuoteRequest({
    $core.String? requestId,
    $core.String? clientRequestId,
    $core.String? sellerResponseId,
    $core.String? expectedSellerResponseVersion,
    $core.String? expectedRfqSnapshotHash,
  }) {
    final result = AcceptSellerQuoteRequest._();
    if (requestId != null) result.requestId = requestId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (expectedSellerResponseVersion != null)
      result.expectedSellerResponseVersion = expectedSellerResponseVersion;
    if (expectedRfqSnapshotHash != null)
      result.expectedRfqSnapshotHash = expectedRfqSnapshotHash;
    return result;
  }

  AcceptSellerQuoteRequest._();

  factory AcceptSellerQuoteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AcceptSellerQuoteRequest()..mergeFromBuffer(data, registry);
  factory AcceptSellerQuoteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AcceptSellerQuoteRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptSellerQuoteRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: AcceptSellerQuoteRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(3, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(4, _omitFieldNames ? '' : 'expectedSellerResponseVersion')
    ..aOS(5, _omitFieldNames ? '' : 'expectedRfqSnapshotHash')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptSellerQuoteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptSellerQuoteRequest copyWith(
          void Function(AcceptSellerQuoteRequest) updates) =>
      super.copyWith((message) => updates(message as AcceptSellerQuoteRequest))
          as AcceptSellerQuoteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AcceptSellerQuoteRequest() / AcceptSellerQuoteRequest.new instead')
  static AcceptSellerQuoteRequest create() => AcceptSellerQuoteRequest._();
  static $pb.GeneratedMessage $_createMessage() => AcceptSellerQuoteRequest._();
  @$core.override
  AcceptSellerQuoteRequest createEmptyInstance() =>
      AcceptSellerQuoteRequest._();
  @$core.pragma('dart2js:noInline')
  static AcceptSellerQuoteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptSellerQuoteRequest>(
          AcceptSellerQuoteRequest.$_createMessage);
  static AcceptSellerQuoteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sellerResponseId => $_getSZ(2);
  @$pb.TagNumber(3)
  set sellerResponseId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSellerResponseId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSellerResponseId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get expectedSellerResponseVersion => $_getSZ(3);
  @$pb.TagNumber(4)
  set expectedSellerResponseVersion($core.String value) =>
      $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExpectedSellerResponseVersion() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpectedSellerResponseVersion() => $_clearField(4);

  /// 需求被別的裝置改過（快照不符）回 ABORTED，conflictTarget 是 rfq_snapshot。
  @$pb.TagNumber(5)
  $core.String get expectedRfqSnapshotHash => $_getSZ(4);
  @$pb.TagNumber(5)
  set expectedRfqSnapshotHash($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpectedRfqSnapshotHash() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpectedRfqSnapshotHash() => $_clearField(5);
}

class AcceptSellerQuoteResponse extends $pb.GeneratedMessage {
  factory AcceptSellerQuoteResponse({
    $core.String? buyerAcceptanceId,
    $core.String? sellerResponseId,
    $core.String? sellerResponseVersion,
    $fixnum.Int64? acceptedDealPrice,
    $1.Timestamp? acceptTime,
    OrderProvisioningState? orderProvisioningState,
    $core.String? orderNo,
    $1.Timestamp? paymentDeadline,
    $core.bool? replayed,
  }) {
    final result = AcceptSellerQuoteResponse._();
    if (buyerAcceptanceId != null) result.buyerAcceptanceId = buyerAcceptanceId;
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (sellerResponseVersion != null)
      result.sellerResponseVersion = sellerResponseVersion;
    if (acceptedDealPrice != null) result.acceptedDealPrice = acceptedDealPrice;
    if (acceptTime != null) result.acceptTime = acceptTime;
    if (orderProvisioningState != null)
      result.orderProvisioningState = orderProvisioningState;
    if (orderNo != null) result.orderNo = orderNo;
    if (paymentDeadline != null) result.paymentDeadline = paymentDeadline;
    if (replayed != null) result.replayed = replayed;
    return result;
  }

  AcceptSellerQuoteResponse._();

  factory AcceptSellerQuoteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AcceptSellerQuoteResponse()..mergeFromBuffer(data, registry);
  factory AcceptSellerQuoteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AcceptSellerQuoteResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptSellerQuoteResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: AcceptSellerQuoteResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'buyerAcceptanceId')
    ..aOS(2, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(3, _omitFieldNames ? '' : 'sellerResponseVersion')
    ..aInt64(4, _omitFieldNames ? '' : 'acceptedDealPrice')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'acceptTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aE<OrderProvisioningState>(
        6, _omitFieldNames ? '' : 'orderProvisioningState',
        enumValues: OrderProvisioningState.values)
    ..aOS(7, _omitFieldNames ? '' : 'orderNo')
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'paymentDeadline',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOB(9, _omitFieldNames ? '' : 'replayed')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptSellerQuoteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptSellerQuoteResponse copyWith(
          void Function(AcceptSellerQuoteResponse) updates) =>
      super.copyWith((message) => updates(message as AcceptSellerQuoteResponse))
          as AcceptSellerQuoteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AcceptSellerQuoteResponse() / AcceptSellerQuoteResponse.new instead')
  static AcceptSellerQuoteResponse create() => AcceptSellerQuoteResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      AcceptSellerQuoteResponse._();
  @$core.override
  AcceptSellerQuoteResponse createEmptyInstance() =>
      AcceptSellerQuoteResponse._();
  @$core.pragma('dart2js:noInline')
  static AcceptSellerQuoteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptSellerQuoteResponse>(
          AcceptSellerQuoteResponse.$_createMessage);
  static AcceptSellerQuoteResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get buyerAcceptanceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set buyerAcceptanceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBuyerAcceptanceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBuyerAcceptanceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sellerResponseId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sellerResponseId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerResponseId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerResponseId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sellerResponseVersion => $_getSZ(2);
  @$pb.TagNumber(3)
  set sellerResponseVersion($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSellerResponseVersion() => $_has(2);
  @$pb.TagNumber(3)
  void clearSellerResponseVersion() => $_clearField(3);

  /// 成交價，新台幣元；來源只有被接受的那一版。
  @$pb.TagNumber(4)
  $fixnum.Int64 get acceptedDealPrice => $_getI64(3);
  @$pb.TagNumber(4)
  set acceptedDealPrice($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAcceptedDealPrice() => $_has(3);
  @$pb.TagNumber(4)
  void clearAcceptedDealPrice() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get acceptTime => $_getN(4);
  @$pb.TagNumber(5)
  set acceptTime($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasAcceptTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearAcceptTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureAcceptTime() => $_ensure(4);

  @$pb.TagNumber(6)
  OrderProvisioningState get orderProvisioningState => $_getN(5);
  @$pb.TagNumber(6)
  set orderProvisioningState(OrderProvisioningState value) =>
      $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasOrderProvisioningState() => $_has(5);
  @$pb.TagNumber(6)
  void clearOrderProvisioningState() => $_clearField(6);

  /// 訂單建立之前是空字串。
  @$pb.TagNumber(7)
  $core.String get orderNo => $_getSZ(6);
  @$pb.TagNumber(7)
  set orderNo($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasOrderNo() => $_has(6);
  @$pb.TagNumber(7)
  void clearOrderNo() => $_clearField(7);

  /// 訂單建立之前不設。
  @$pb.TagNumber(8)
  $1.Timestamp get paymentDeadline => $_getN(7);
  @$pb.TagNumber(8)
  set paymentDeadline($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasPaymentDeadline() => $_has(7);
  @$pb.TagNumber(8)
  void clearPaymentDeadline() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensurePaymentDeadline() => $_ensure(7);

  @$pb.TagNumber(9)
  $core.bool get replayed => $_getBF(8);
  @$pb.TagNumber(9)
  set replayed($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasReplayed() => $_has(8);
  @$pb.TagNumber(9)
  void clearReplayed() => $_clearField(9);
}

class WatchRunRequest extends $pb.GeneratedMessage {
  factory WatchRunRequest({
    $core.String? runId,
    $core.String? cursor,
  }) {
    final result = WatchRunRequest._();
    if (runId != null) result.runId = runId;
    if (cursor != null) result.cursor = cursor;
    return result;
  }

  WatchRunRequest._();

  factory WatchRunRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchRunRequest()..mergeFromBuffer(data, registry);
  factory WatchRunRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchRunRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchRunRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: WatchRunRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'runId')
    ..aOS(2, _omitFieldNames ? '' : 'cursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchRunRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchRunRequest copyWith(void Function(WatchRunRequest) updates) =>
      super.copyWith((message) => updates(message as WatchRunRequest))
          as WatchRunRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use WatchRunRequest() / WatchRunRequest.new instead')
  static WatchRunRequest create() => WatchRunRequest._();
  static $pb.GeneratedMessage $_createMessage() => WatchRunRequest._();
  @$core.override
  WatchRunRequest createEmptyInstance() => WatchRunRequest._();
  @$core.pragma('dart2js:noInline')
  static WatchRunRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<WatchRunRequest>(
          WatchRunRequest.$_createMessage);
  static WatchRunRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get runId => $_getSZ(0);
  @$pb.TagNumber(1)
  set runId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunId() => $_clearField(1);

  /// 上一次收到的事件的 cursor，從它之後接續；空字串表示從 run 的第一個事件開始。client 當成不透明的字串。
  @$pb.TagNumber(2)
  $core.String get cursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set cursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearCursor() => $_clearField(2);
}

enum WatchRunResponse_Event {
  runAccepted,
  text,
  step,
  activity,
  message,
  toolResult,
  turnResult,
  finished,
  error,
  notSet
}

/// WatchRunResponse 是串流上的一個事件。
class WatchRunResponse extends $pb.GeneratedMessage {
  factory WatchRunResponse({
    $core.String? cursor,
    $1.Timestamp? eventTime,
    RunAccepted? runAccepted,
    TextDelta? text,
    StepEvent? step,
    Activity? activity,
    MessageEvent? message,
    ToolResult? toolResult,
    TurnResult? turnResult,
    RunFinished? finished,
    RunStreamError? error,
  }) {
    final result = WatchRunResponse._();
    if (cursor != null) result.cursor = cursor;
    if (eventTime != null) result.eventTime = eventTime;
    if (runAccepted != null) result.runAccepted = runAccepted;
    if (text != null) result.text = text;
    if (step != null) result.step = step;
    if (activity != null) result.activity = activity;
    if (message != null) result.message = message;
    if (toolResult != null) result.toolResult = toolResult;
    if (turnResult != null) result.turnResult = turnResult;
    if (finished != null) result.finished = finished;
    if (error != null) result.error = error;
    return result;
  }

  WatchRunResponse._();

  factory WatchRunResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchRunResponse()..mergeFromBuffer(data, registry);
  factory WatchRunResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchRunResponse()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, WatchRunResponse_Event>
      _WatchRunResponse_EventByTag = {
    3: WatchRunResponse_Event.runAccepted,
    4: WatchRunResponse_Event.text,
    5: WatchRunResponse_Event.step,
    6: WatchRunResponse_Event.activity,
    7: WatchRunResponse_Event.message,
    8: WatchRunResponse_Event.toolResult,
    9: WatchRunResponse_Event.turnResult,
    10: WatchRunResponse_Event.finished,
    11: WatchRunResponse_Event.error,
    0: WatchRunResponse_Event.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchRunResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: WatchRunResponse.$_createMessage)
    ..oo(0, [3, 4, 5, 6, 7, 8, 9, 10, 11])
    ..aOS(1, _omitFieldNames ? '' : 'cursor')
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'eventTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<RunAccepted>(3, _omitFieldNames ? '' : 'runAccepted',
        subBuilder: RunAccepted.$_createMessage)
    ..aOM<TextDelta>(4, _omitFieldNames ? '' : 'text',
        subBuilder: TextDelta.$_createMessage)
    ..aOM<StepEvent>(5, _omitFieldNames ? '' : 'step',
        subBuilder: StepEvent.$_createMessage)
    ..aOM<Activity>(6, _omitFieldNames ? '' : 'activity',
        subBuilder: Activity.$_createMessage)
    ..aOM<MessageEvent>(7, _omitFieldNames ? '' : 'message',
        subBuilder: MessageEvent.$_createMessage)
    ..aOM<ToolResult>(8, _omitFieldNames ? '' : 'toolResult',
        subBuilder: ToolResult.$_createMessage)
    ..aOM<TurnResult>(9, _omitFieldNames ? '' : 'turnResult',
        subBuilder: TurnResult.$_createMessage)
    ..aOM<RunFinished>(10, _omitFieldNames ? '' : 'finished',
        subBuilder: RunFinished.$_createMessage)
    ..aOM<RunStreamError>(11, _omitFieldNames ? '' : 'error',
        subBuilder: RunStreamError.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchRunResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchRunResponse copyWith(void Function(WatchRunResponse) updates) =>
      super.copyWith((message) => updates(message as WatchRunResponse))
          as WatchRunResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use WatchRunResponse() / WatchRunResponse.new instead')
  static WatchRunResponse create() => WatchRunResponse._();
  static $pb.GeneratedMessage $_createMessage() => WatchRunResponse._();
  @$core.override
  WatchRunResponse createEmptyInstance() => WatchRunResponse._();
  @$core.pragma('dart2js:noInline')
  static WatchRunResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<WatchRunResponse>(
          WatchRunResponse.$_createMessage);
  static WatchRunResponse? _defaultInstance;

  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  @$pb.TagNumber(8)
  @$pb.TagNumber(9)
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  WatchRunResponse_Event whichEvent() =>
      _WatchRunResponse_EventByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  @$pb.TagNumber(8)
  @$pb.TagNumber(9)
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
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

  @$pb.TagNumber(2)
  $1.Timestamp get eventTime => $_getN(1);
  @$pb.TagNumber(2)
  set eventTime($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasEventTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearEventTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureEventTime() => $_ensure(1);

  /// run 已受理；run 的第一個事件。
  @$pb.TagNumber(3)
  RunAccepted get runAccepted => $_getN(2);
  @$pb.TagNumber(3)
  set runAccepted(RunAccepted value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasRunAccepted() => $_has(2);
  @$pb.TagNumber(3)
  void clearRunAccepted() => $_clearField(3);
  @$pb.TagNumber(3)
  RunAccepted ensureRunAccepted() => $_ensure(2);

  /// 回覆文字的增量，client 依序串接。
  @$pb.TagNumber(4)
  TextDelta get text => $_getN(3);
  @$pb.TagNumber(4)
  set text(TextDelta value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasText() => $_has(3);
  @$pb.TagNumber(4)
  void clearText() => $_clearField(4);
  @$pb.TagNumber(4)
  TextDelta ensureText() => $_ensure(3);

  /// 一個步驟開始、進行或結束。
  @$pb.TagNumber(5)
  StepEvent get step => $_getN(4);
  @$pb.TagNumber(5)
  set step(StepEvent value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasStep() => $_has(4);
  @$pb.TagNumber(5)
  void clearStep() => $_clearField(5);
  @$pb.TagNumber(5)
  StepEvent ensureStep() => $_ensure(4);

  /// 巢狀的進度列。client 依 row_id 合併：同一個 row_id 的後續事件就地更新那一列。
  @$pb.TagNumber(6)
  Activity get activity => $_getN(5);
  @$pb.TagNumber(6)
  set activity(Activity value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasActivity() => $_has(5);
  @$pb.TagNumber(6)
  void clearActivity() => $_clearField(6);
  @$pb.TagNumber(6)
  Activity ensureActivity() => $_ensure(5);

  /// 完整的一則回覆。
  @$pb.TagNumber(7)
  MessageEvent get message => $_getN(6);
  @$pb.TagNumber(7)
  set message(MessageEvent value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasMessage() => $_has(6);
  @$pb.TagNumber(7)
  void clearMessage() => $_clearField(7);
  @$pb.TagNumber(7)
  MessageEvent ensureMessage() => $_ensure(6);

  /// 工具執行的結果。
  @$pb.TagNumber(8)
  ToolResult get toolResult => $_getN(7);
  @$pb.TagNumber(8)
  set toolResult(ToolResult value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasToolResult() => $_has(7);
  @$pb.TagNumber(8)
  void clearToolResult() => $_clearField(8);
  @$pb.TagNumber(8)
  ToolResult ensureToolResult() => $_ensure(7);

  /// 這一輪結束時的需求狀態；停在等待輸入時帶 interrupt。
  @$pb.TagNumber(9)
  TurnResult get turnResult => $_getN(8);
  @$pb.TagNumber(9)
  set turnResult(TurnResult value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasTurnResult() => $_has(8);
  @$pb.TagNumber(9)
  void clearTurnResult() => $_clearField(9);
  @$pb.TagNumber(9)
  TurnResult ensureTurnResult() => $_ensure(8);

  /// run 到達終態。
  @$pb.TagNumber(10)
  RunFinished get finished => $_getN(9);
  @$pb.TagNumber(10)
  set finished(RunFinished value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasFinished() => $_has(9);
  @$pb.TagNumber(10)
  void clearFinished() => $_clearField(10);
  @$pb.TagNumber(10)
  RunFinished ensureFinished() => $_ensure(9);

  /// run 失敗。
  @$pb.TagNumber(11)
  RunStreamError get error => $_getN(10);
  @$pb.TagNumber(11)
  set error(RunStreamError value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasError() => $_has(10);
  @$pb.TagNumber(11)
  void clearError() => $_clearField(11);
  @$pb.TagNumber(11)
  RunStreamError ensureError() => $_ensure(10);
}

class RunAccepted extends $pb.GeneratedMessage {
  factory RunAccepted({
    $core.String? runId,
    $core.String? conversationId,
  }) {
    final result = RunAccepted._();
    if (runId != null) result.runId = runId;
    if (conversationId != null) result.conversationId = conversationId;
    return result;
  }

  RunAccepted._();

  factory RunAccepted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RunAccepted()..mergeFromBuffer(data, registry);
  factory RunAccepted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RunAccepted()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RunAccepted',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RunAccepted.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'runId')
    ..aOS(2, _omitFieldNames ? '' : 'conversationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunAccepted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunAccepted copyWith(void Function(RunAccepted) updates) =>
      super.copyWith((message) => updates(message as RunAccepted))
          as RunAccepted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RunAccepted() / RunAccepted.new instead')
  static RunAccepted create() => RunAccepted._();
  static $pb.GeneratedMessage $_createMessage() => RunAccepted._();
  @$core.override
  RunAccepted createEmptyInstance() => RunAccepted._();
  @$core.pragma('dart2js:noInline')
  static RunAccepted getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RunAccepted>(
          RunAccepted.$_createMessage);
  static RunAccepted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get runId => $_getSZ(0);
  @$pb.TagNumber(1)
  set runId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get conversationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set conversationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConversationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearConversationId() => $_clearField(2);
}

class TextDelta extends $pb.GeneratedMessage {
  factory TextDelta({
    $core.String? text,
  }) {
    final result = TextDelta._();
    if (text != null) result.text = text;
    return result;
  }

  TextDelta._();

  factory TextDelta.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TextDelta()..mergeFromBuffer(data, registry);
  factory TextDelta.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TextDelta()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TextDelta',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: TextDelta.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'text')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TextDelta clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TextDelta copyWith(void Function(TextDelta) updates) =>
      super.copyWith((message) => updates(message as TextDelta)) as TextDelta;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TextDelta() / TextDelta.new instead')
  static TextDelta create() => TextDelta._();
  static $pb.GeneratedMessage $_createMessage() => TextDelta._();
  @$core.override
  TextDelta createEmptyInstance() => TextDelta._();
  @$core.pragma('dart2js:noInline')
  static TextDelta getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TextDelta>(TextDelta.$_createMessage);
  static TextDelta? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get text => $_getSZ(0);
  @$pb.TagNumber(1)
  set text($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasText() => $_has(0);
  @$pb.TagNumber(1)
  void clearText() => $_clearField(1);
}

class StepEvent extends $pb.GeneratedMessage {
  factory StepEvent({
    $core.int? seq,
    Step? step,
  }) {
    final result = StepEvent._();
    if (seq != null) result.seq = seq;
    if (step != null) result.step = step;
    return result;
  }

  StepEvent._();

  factory StepEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StepEvent()..mergeFromBuffer(data, registry);
  factory StepEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StepEvent()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StepEvent',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: StepEvent.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'seq')
    ..aOM<Step>(2, _omitFieldNames ? '' : 'step',
        subBuilder: Step.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StepEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StepEvent copyWith(void Function(StepEvent) updates) =>
      super.copyWith((message) => updates(message as StepEvent)) as StepEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use StepEvent() / StepEvent.new instead')
  static StepEvent create() => StepEvent._();
  static $pb.GeneratedMessage $_createMessage() => StepEvent._();
  @$core.override
  StepEvent createEmptyInstance() => StepEvent._();
  @$core.pragma('dart2js:noInline')
  static StepEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StepEvent>(StepEvent.$_createMessage);
  static StepEvent? _defaultInstance;

  /// 步驟在這個 run 裡的序號。
  @$pb.TagNumber(1)
  $core.int get seq => $_getIZ(0);
  @$pb.TagNumber(1)
  set seq($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSeq() => $_has(0);
  @$pb.TagNumber(1)
  void clearSeq() => $_clearField(1);

  @$pb.TagNumber(2)
  Step get step => $_getN(1);
  @$pb.TagNumber(2)
  set step(Step value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStep() => $_has(1);
  @$pb.TagNumber(2)
  void clearStep() => $_clearField(2);
  @$pb.TagNumber(2)
  Step ensureStep() => $_ensure(1);
}

class Activity extends $pb.GeneratedMessage {
  factory Activity({
    $core.String? rowId,
    $core.int? attempt,
    $core.int? maxAttempts,
    $core.String? retryGroupKey,
    $core.String? code,
    $core.String? label,
    $core.String? text,
    StepStatus? status,
    $core.String? parentRowId,
    $core.int? count,
  }) {
    final result = Activity._();
    if (rowId != null) result.rowId = rowId;
    if (attempt != null) result.attempt = attempt;
    if (maxAttempts != null) result.maxAttempts = maxAttempts;
    if (retryGroupKey != null) result.retryGroupKey = retryGroupKey;
    if (code != null) result.code = code;
    if (label != null) result.label = label;
    if (text != null) result.text = text;
    if (status != null) result.status = status;
    if (parentRowId != null) result.parentRowId = parentRowId;
    if (count != null) result.count = count;
    return result;
  }

  Activity._();

  factory Activity.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Activity()..mergeFromBuffer(data, registry);
  factory Activity.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Activity()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Activity',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: Activity.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'rowId')
    ..aI(2, _omitFieldNames ? '' : 'attempt')
    ..aI(3, _omitFieldNames ? '' : 'maxAttempts')
    ..aOS(4, _omitFieldNames ? '' : 'retryGroupKey')
    ..aOS(5, _omitFieldNames ? '' : 'code')
    ..aOS(6, _omitFieldNames ? '' : 'label')
    ..aOS(7, _omitFieldNames ? '' : 'text')
    ..aE<StepStatus>(8, _omitFieldNames ? '' : 'status',
        enumValues: StepStatus.values)
    ..aOS(9, _omitFieldNames ? '' : 'parentRowId')
    ..aI(10, _omitFieldNames ? '' : 'count')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Activity clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Activity copyWith(void Function(Activity) updates) =>
      super.copyWith((message) => updates(message as Activity)) as Activity;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Activity() / Activity.new instead')
  static Activity create() => Activity._();
  static $pb.GeneratedMessage $_createMessage() => Activity._();
  @$core.override
  Activity createEmptyInstance() => Activity._();
  @$core.pragma('dart2js:noInline')
  static Activity getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Activity>(Activity.$_createMessage);
  static Activity? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get rowId => $_getSZ(0);
  @$pb.TagNumber(1)
  set rowId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRowId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRowId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get attempt => $_getIZ(1);
  @$pb.TagNumber(2)
  set attempt($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAttempt() => $_has(1);
  @$pb.TagNumber(2)
  void clearAttempt() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get maxAttempts => $_getIZ(2);
  @$pb.TagNumber(3)
  set maxAttempts($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMaxAttempts() => $_has(2);
  @$pb.TagNumber(3)
  void clearMaxAttempts() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get retryGroupKey => $_getSZ(3);
  @$pb.TagNumber(4)
  set retryGroupKey($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRetryGroupKey() => $_has(3);
  @$pb.TagNumber(4)
  void clearRetryGroupKey() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get code => $_getSZ(4);
  @$pb.TagNumber(5)
  set code($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCode() => $_has(4);
  @$pb.TagNumber(5)
  void clearCode() => $_clearField(5);

  /// 伺服器的固定文字，不為空。
  @$pb.TagNumber(6)
  $core.String get label => $_getSZ(5);
  @$pb.TagNumber(6)
  set label($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasLabel() => $_has(5);
  @$pb.TagNumber(6)
  void clearLabel() => $_clearField(6);

  /// 這一次實際發生了什麼；可為空，空的時候 client 顯示 label。
  @$pb.TagNumber(7)
  $core.String get text => $_getSZ(6);
  @$pb.TagNumber(7)
  set text($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasText() => $_has(6);
  @$pb.TagNumber(7)
  void clearText() => $_clearField(7);

  @$pb.TagNumber(8)
  StepStatus get status => $_getN(7);
  @$pb.TagNumber(8)
  set status(StepStatus value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasStatus() => $_has(7);
  @$pb.TagNumber(8)
  void clearStatus() => $_clearField(8);

  /// 只在巢狀的子列有值。
  @$pb.TagNumber(9)
  $core.String get parentRowId => $_getSZ(8);
  @$pb.TagNumber(9)
  set parentRowId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasParentRowId() => $_has(8);
  @$pb.TagNumber(9)
  void clearParentRowId() => $_clearField(9);

  /// 只在這一列有計數的概念時有值；沒有與 0 是不同的意思。
  @$pb.TagNumber(10)
  $core.int get count => $_getIZ(9);
  @$pb.TagNumber(10)
  set count($core.int value) => $_setSignedInt32(9, value);
  @$pb.TagNumber(10)
  $core.bool hasCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearCount() => $_clearField(10);
}

class MessageEvent extends $pb.GeneratedMessage {
  factory MessageEvent({
    $core.String? reply,
  }) {
    final result = MessageEvent._();
    if (reply != null) result.reply = reply;
    return result;
  }

  MessageEvent._();

  factory MessageEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MessageEvent()..mergeFromBuffer(data, registry);
  factory MessageEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MessageEvent()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageEvent',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: MessageEvent.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reply')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageEvent copyWith(void Function(MessageEvent) updates) =>
      super.copyWith((message) => updates(message as MessageEvent))
          as MessageEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MessageEvent() / MessageEvent.new instead')
  static MessageEvent create() => MessageEvent._();
  static $pb.GeneratedMessage $_createMessage() => MessageEvent._();
  @$core.override
  MessageEvent createEmptyInstance() => MessageEvent._();
  @$core.pragma('dart2js:noInline')
  static MessageEvent getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MessageEvent>(
          MessageEvent.$_createMessage);
  static MessageEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reply => $_getSZ(0);
  @$pb.TagNumber(1)
  set reply($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReply() => $_has(0);
  @$pb.TagNumber(1)
  void clearReply() => $_clearField(1);
}

/// ToolResult 說明 run 呼叫了哪個工具，以及它有沒有成功；工具的回傳內容不對外。
class ToolResult extends $pb.GeneratedMessage {
  factory ToolResult({
    $core.String? toolName,
    $core.bool? failed,
  }) {
    final result = ToolResult._();
    if (toolName != null) result.toolName = toolName;
    if (failed != null) result.failed = failed;
    return result;
  }

  ToolResult._();

  factory ToolResult.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToolResult()..mergeFromBuffer(data, registry);
  factory ToolResult.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToolResult()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ToolResult',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ToolResult.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'toolName')
    ..aOB(2, _omitFieldNames ? '' : 'failed')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToolResult clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToolResult copyWith(void Function(ToolResult) updates) =>
      super.copyWith((message) => updates(message as ToolResult)) as ToolResult;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ToolResult() / ToolResult.new instead')
  static ToolResult create() => ToolResult._();
  static $pb.GeneratedMessage $_createMessage() => ToolResult._();
  @$core.override
  ToolResult createEmptyInstance() => ToolResult._();
  @$core.pragma('dart2js:noInline')
  static ToolResult getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ToolResult>(ToolResult.$_createMessage);
  static ToolResult? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get toolName => $_getSZ(0);
  @$pb.TagNumber(1)
  set toolName($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasToolName() => $_has(0);
  @$pb.TagNumber(1)
  void clearToolName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get failed => $_getBF(1);
  @$pb.TagNumber(2)
  set failed($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFailed() => $_has(1);
  @$pb.TagNumber(2)
  void clearFailed() => $_clearField(2);
}

class RunFinished extends $pb.GeneratedMessage {
  factory RunFinished({
    RunStatus? status,
    RunFailureReason? failureReason,
  }) {
    final result = RunFinished._();
    if (status != null) result.status = status;
    if (failureReason != null) result.failureReason = failureReason;
    return result;
  }

  RunFinished._();

  factory RunFinished.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RunFinished()..mergeFromBuffer(data, registry);
  factory RunFinished.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RunFinished()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RunFinished',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RunFinished.$_createMessage)
    ..aE<RunStatus>(1, _omitFieldNames ? '' : 'status',
        enumValues: RunStatus.values)
    ..aE<RunFailureReason>(2, _omitFieldNames ? '' : 'failureReason',
        enumValues: RunFailureReason.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunFinished clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunFinished copyWith(void Function(RunFinished) updates) =>
      super.copyWith((message) => updates(message as RunFinished))
          as RunFinished;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RunFinished() / RunFinished.new instead')
  static RunFinished create() => RunFinished._();
  static $pb.GeneratedMessage $_createMessage() => RunFinished._();
  @$core.override
  RunFinished createEmptyInstance() => RunFinished._();
  @$core.pragma('dart2js:noInline')
  static RunFinished getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RunFinished>(
          RunFinished.$_createMessage);
  static RunFinished? _defaultInstance;

  @$pb.TagNumber(1)
  RunStatus get status => $_getN(0);
  @$pb.TagNumber(1)
  set status(RunStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatus() => $_clearField(1);

  /// 只在 FAILED 與 EXPIRED 時有值。
  @$pb.TagNumber(2)
  RunFailureReason get failureReason => $_getN(1);
  @$pb.TagNumber(2)
  set failureReason(RunFailureReason value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasFailureReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearFailureReason() => $_clearField(2);
}

/// RunStreamError 是 run 的失敗，之後沒有其他事件。
class RunStreamError extends $pb.GeneratedMessage {
  factory RunStreamError({
    $3.ErrorReason? reason,
    $core.String? localizedMessage,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? metadata,
  }) {
    final result = RunStreamError._();
    if (reason != null) result.reason = reason;
    if (localizedMessage != null) result.localizedMessage = localizedMessage;
    if (metadata != null) result.metadata.addEntries(metadata);
    return result;
  }

  RunStreamError._();

  factory RunStreamError.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RunStreamError()..mergeFromBuffer(data, registry);
  factory RunStreamError.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RunStreamError()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RunStreamError',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: RunStreamError.$_createMessage)
    ..aE<$3.ErrorReason>(1, _omitFieldNames ? '' : 'reason',
        enumValues: $3.ErrorReason.values)
    ..aOS(2, _omitFieldNames ? '' : 'localizedMessage')
    ..m<$core.String, $core.String>(3, _omitFieldNames ? '' : 'metadata',
        entryClassName: 'RunStreamError.MetadataEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('hawkr.procurement.v1alpha1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunStreamError clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RunStreamError copyWith(void Function(RunStreamError) updates) =>
      super.copyWith((message) => updates(message as RunStreamError))
          as RunStreamError;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RunStreamError() / RunStreamError.new instead')
  static RunStreamError create() => RunStreamError._();
  static $pb.GeneratedMessage $_createMessage() => RunStreamError._();
  @$core.override
  RunStreamError createEmptyInstance() => RunStreamError._();
  @$core.pragma('dart2js:noInline')
  static RunStreamError getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RunStreamError>(
          RunStreamError.$_createMessage);
  static RunStreamError? _defaultInstance;

  @$pb.TagNumber(1)
  $3.ErrorReason get reason => $_getN(0);
  @$pb.TagNumber(1)
  set reason($3.ErrorReason value) => $_setField(1, value);
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

class CancelRunRequest extends $pb.GeneratedMessage {
  factory CancelRunRequest({
    $core.String? runId,
  }) {
    final result = CancelRunRequest._();
    if (runId != null) result.runId = runId;
    return result;
  }

  CancelRunRequest._();

  factory CancelRunRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelRunRequest()..mergeFromBuffer(data, registry);
  factory CancelRunRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelRunRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelRunRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: CancelRunRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'runId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelRunRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelRunRequest copyWith(void Function(CancelRunRequest) updates) =>
      super.copyWith((message) => updates(message as CancelRunRequest))
          as CancelRunRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CancelRunRequest() / CancelRunRequest.new instead')
  static CancelRunRequest create() => CancelRunRequest._();
  static $pb.GeneratedMessage $_createMessage() => CancelRunRequest._();
  @$core.override
  CancelRunRequest createEmptyInstance() => CancelRunRequest._();
  @$core.pragma('dart2js:noInline')
  static CancelRunRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CancelRunRequest>(
          CancelRunRequest.$_createMessage);
  static CancelRunRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get runId => $_getSZ(0);
  @$pb.TagNumber(1)
  set runId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunId() => $_clearField(1);
}

class CancelRunResponse extends $pb.GeneratedMessage {
  factory CancelRunResponse({
    $core.String? runId,
    RunStatus? runStatus,
    CancelDisposition? cancelDisposition,
  }) {
    final result = CancelRunResponse._();
    if (runId != null) result.runId = runId;
    if (runStatus != null) result.runStatus = runStatus;
    if (cancelDisposition != null) result.cancelDisposition = cancelDisposition;
    return result;
  }

  CancelRunResponse._();

  factory CancelRunResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelRunResponse()..mergeFromBuffer(data, registry);
  factory CancelRunResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelRunResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelRunResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: CancelRunResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'runId')
    ..aE<RunStatus>(2, _omitFieldNames ? '' : 'runStatus',
        enumValues: RunStatus.values)
    ..aE<CancelDisposition>(3, _omitFieldNames ? '' : 'cancelDisposition',
        enumValues: CancelDisposition.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelRunResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelRunResponse copyWith(void Function(CancelRunResponse) updates) =>
      super.copyWith((message) => updates(message as CancelRunResponse))
          as CancelRunResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CancelRunResponse() / CancelRunResponse.new instead')
  static CancelRunResponse create() => CancelRunResponse._();
  static $pb.GeneratedMessage $_createMessage() => CancelRunResponse._();
  @$core.override
  CancelRunResponse createEmptyInstance() => CancelRunResponse._();
  @$core.pragma('dart2js:noInline')
  static CancelRunResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CancelRunResponse>(
          CancelRunResponse.$_createMessage);
  static CancelRunResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get runId => $_getSZ(0);
  @$pb.TagNumber(1)
  set runId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunId() => $_clearField(1);

  @$pb.TagNumber(2)
  RunStatus get runStatus => $_getN(1);
  @$pb.TagNumber(2)
  set runStatus(RunStatus value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasRunStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearRunStatus() => $_clearField(2);

  @$pb.TagNumber(3)
  CancelDisposition get cancelDisposition => $_getN(2);
  @$pb.TagNumber(3)
  set cancelDisposition(CancelDisposition value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCancelDisposition() => $_has(2);
  @$pb.TagNumber(3)
  void clearCancelDisposition() => $_clearField(3);
}

/// ClarificationResponse 是對 INTERRUPT_KIND_CLARIFICATION 的回覆：選一個選項，或自填描述。兩者至少要有一個。
class ClarificationResponse extends $pb.GeneratedMessage {
  factory ClarificationResponse({
    $core.String? optionId,
    $core.String? freeText,
  }) {
    final result = ClarificationResponse._();
    if (optionId != null) result.optionId = optionId;
    if (freeText != null) result.freeText = freeText;
    return result;
  }

  ClarificationResponse._();

  factory ClarificationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClarificationResponse()..mergeFromBuffer(data, registry);
  factory ClarificationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClarificationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClarificationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ClarificationResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'optionId')
    ..aOS(2, _omitFieldNames ? '' : 'freeText')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClarificationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClarificationResponse copyWith(
          void Function(ClarificationResponse) updates) =>
      super.copyWith((message) => updates(message as ClarificationResponse))
          as ClarificationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ClarificationResponse() / ClarificationResponse.new instead')
  static ClarificationResponse create() => ClarificationResponse._();
  static $pb.GeneratedMessage $_createMessage() => ClarificationResponse._();
  @$core.override
  ClarificationResponse createEmptyInstance() => ClarificationResponse._();
  @$core.pragma('dart2js:noInline')
  static ClarificationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClarificationResponse>(
          ClarificationResponse.$_createMessage);
  static ClarificationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get optionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set optionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOptionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOptionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get freeText => $_getSZ(1);
  @$pb.TagNumber(2)
  set freeText($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFreeText() => $_has(1);
  @$pb.TagNumber(2)
  void clearFreeText() => $_clearField(2);
}

enum ResumeRunRequest_Response { clarification, notSet }

class ResumeRunRequest extends $pb.GeneratedMessage {
  factory ResumeRunRequest({
    $core.String? interruptId,
    $core.String? clientRequestId,
    ClarificationResponse? clarification,
  }) {
    final result = ResumeRunRequest._();
    if (interruptId != null) result.interruptId = interruptId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (clarification != null) result.clarification = clarification;
    return result;
  }

  ResumeRunRequest._();

  factory ResumeRunRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResumeRunRequest()..mergeFromBuffer(data, registry);
  factory ResumeRunRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResumeRunRequest()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, ResumeRunRequest_Response>
      _ResumeRunRequest_ResponseByTag = {
    3: ResumeRunRequest_Response.clarification,
    0: ResumeRunRequest_Response.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResumeRunRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ResumeRunRequest.$_createMessage)
    ..oo(0, [3])
    ..aOS(1, _omitFieldNames ? '' : 'interruptId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..aOM<ClarificationResponse>(3, _omitFieldNames ? '' : 'clarification',
        subBuilder: ClarificationResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResumeRunRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResumeRunRequest copyWith(void Function(ResumeRunRequest) updates) =>
      super.copyWith((message) => updates(message as ResumeRunRequest))
          as ResumeRunRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ResumeRunRequest() / ResumeRunRequest.new instead')
  static ResumeRunRequest create() => ResumeRunRequest._();
  static $pb.GeneratedMessage $_createMessage() => ResumeRunRequest._();
  @$core.override
  ResumeRunRequest createEmptyInstance() => ResumeRunRequest._();
  @$core.pragma('dart2js:noInline')
  static ResumeRunRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ResumeRunRequest>(
          ResumeRunRequest.$_createMessage);
  static ResumeRunRequest? _defaultInstance;

  @$pb.TagNumber(3)
  ResumeRunRequest_Response whichResponse() =>
      _ResumeRunRequest_ResponseByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(3)
  void clearResponse() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get interruptId => $_getSZ(0);
  @$pb.TagNumber(1)
  set interruptId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInterruptId() => $_has(0);
  @$pb.TagNumber(1)
  void clearInterruptId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  ClarificationResponse get clarification => $_getN(2);
  @$pb.TagNumber(3)
  set clarification(ClarificationResponse value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasClarification() => $_has(2);
  @$pb.TagNumber(3)
  void clearClarification() => $_clearField(3);
  @$pb.TagNumber(3)
  ClarificationResponse ensureClarification() => $_ensure(2);
}

class ResumeRunResponse extends $pb.GeneratedMessage {
  factory ResumeRunResponse({
    $core.String? runId,
    RunStatus? runStatus,
    TurnResult? result,
  }) {
    final result$ = ResumeRunResponse._();
    if (runId != null) result$.runId = runId;
    if (runStatus != null) result$.runStatus = runStatus;
    if (result != null) result$.result = result;
    return result$;
  }

  ResumeRunResponse._();

  factory ResumeRunResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResumeRunResponse()..mergeFromBuffer(data, registry);
  factory ResumeRunResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResumeRunResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResumeRunResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ResumeRunResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'runId')
    ..aE<RunStatus>(2, _omitFieldNames ? '' : 'runStatus',
        enumValues: RunStatus.values)
    ..aOM<TurnResult>(3, _omitFieldNames ? '' : 'result',
        subBuilder: TurnResult.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResumeRunResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResumeRunResponse copyWith(void Function(ResumeRunResponse) updates) =>
      super.copyWith((message) => updates(message as ResumeRunResponse))
          as ResumeRunResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ResumeRunResponse() / ResumeRunResponse.new instead')
  static ResumeRunResponse create() => ResumeRunResponse._();
  static $pb.GeneratedMessage $_createMessage() => ResumeRunResponse._();
  @$core.override
  ResumeRunResponse createEmptyInstance() => ResumeRunResponse._();
  @$core.pragma('dart2js:noInline')
  static ResumeRunResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ResumeRunResponse>(
          ResumeRunResponse.$_createMessage);
  static ResumeRunResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get runId => $_getSZ(0);
  @$pb.TagNumber(1)
  set runId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunId() => $_clearField(1);

  @$pb.TagNumber(2)
  RunStatus get runStatus => $_getN(1);
  @$pb.TagNumber(2)
  set runStatus(RunStatus value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasRunStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearRunStatus() => $_clearField(2);

  /// 這一輪接續實際產生的結果；接續的 run 跑完才有值。冪等重送只回現況、不重跑，這時不設，
  /// client 改讀 GetSnapshot。
  @$pb.TagNumber(3)
  TurnResult get result => $_getN(2);
  @$pb.TagNumber(3)
  set result(TurnResult value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasResult() => $_has(2);
  @$pb.TagNumber(3)
  void clearResult() => $_clearField(3);
  @$pb.TagNumber(3)
  TurnResult ensureResult() => $_ensure(2);
}

class SellerPublicationSummary extends $pb.GeneratedMessage {
  factory SellerPublicationSummary({
    $core.String? publicationId,
    $core.String? publicationVersion,
    $core.String? sellerFacingPayloadHash,
    InviteDeliveryState? deliveryState,
    $1.Timestamp? publishTime,
  }) {
    final result = SellerPublicationSummary._();
    if (publicationId != null) result.publicationId = publicationId;
    if (publicationVersion != null)
      result.publicationVersion = publicationVersion;
    if (sellerFacingPayloadHash != null)
      result.sellerFacingPayloadHash = sellerFacingPayloadHash;
    if (deliveryState != null) result.deliveryState = deliveryState;
    if (publishTime != null) result.publishTime = publishTime;
    return result;
  }

  SellerPublicationSummary._();

  factory SellerPublicationSummary.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerPublicationSummary()..mergeFromBuffer(data, registry);
  factory SellerPublicationSummary.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerPublicationSummary()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerPublicationSummary',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SellerPublicationSummary.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'publicationId')
    ..aOS(2, _omitFieldNames ? '' : 'publicationVersion')
    ..aOS(3, _omitFieldNames ? '' : 'sellerFacingPayloadHash')
    ..aE<InviteDeliveryState>(4, _omitFieldNames ? '' : 'deliveryState',
        enumValues: InviteDeliveryState.values)
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerPublicationSummary clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerPublicationSummary copyWith(
          void Function(SellerPublicationSummary) updates) =>
      super.copyWith((message) => updates(message as SellerPublicationSummary))
          as SellerPublicationSummary;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SellerPublicationSummary() / SellerPublicationSummary.new instead')
  static SellerPublicationSummary create() => SellerPublicationSummary._();
  static $pb.GeneratedMessage $_createMessage() => SellerPublicationSummary._();
  @$core.override
  SellerPublicationSummary createEmptyInstance() =>
      SellerPublicationSummary._();
  @$core.pragma('dart2js:noInline')
  static SellerPublicationSummary getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SellerPublicationSummary>(
          SellerPublicationSummary.$_createMessage);
  static SellerPublicationSummary? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get publicationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set publicationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPublicationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublicationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get publicationVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set publicationVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPublicationVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublicationVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sellerFacingPayloadHash => $_getSZ(2);
  @$pb.TagNumber(3)
  set sellerFacingPayloadHash($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSellerFacingPayloadHash() => $_has(2);
  @$pb.TagNumber(3)
  void clearSellerFacingPayloadHash() => $_clearField(3);

  @$pb.TagNumber(4)
  InviteDeliveryState get deliveryState => $_getN(3);
  @$pb.TagNumber(4)
  set deliveryState(InviteDeliveryState value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasDeliveryState() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeliveryState() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get publishTime => $_getN(4);
  @$pb.TagNumber(5)
  set publishTime($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPublishTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearPublishTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensurePublishTime() => $_ensure(4);
}

/// SellerResponseDraft 是賣家自己的回價草稿。
class SellerResponseDraft extends $pb.GeneratedMessage {
  factory SellerResponseDraft({
    $core.String? draftVersion,
    $1.Timestamp? updateTime,
    SellerQuote? draft,
  }) {
    final result = SellerResponseDraft._();
    if (draftVersion != null) result.draftVersion = draftVersion;
    if (updateTime != null) result.updateTime = updateTime;
    if (draft != null) result.draft = draft;
    return result;
  }

  SellerResponseDraft._();

  factory SellerResponseDraft.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerResponseDraft()..mergeFromBuffer(data, registry);
  factory SellerResponseDraft.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerResponseDraft()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerResponseDraft',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SellerResponseDraft.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'draftVersion')
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<SellerQuote>(3, _omitFieldNames ? '' : 'draft',
        subBuilder: SellerQuote.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerResponseDraft clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerResponseDraft copyWith(void Function(SellerResponseDraft) updates) =>
      super.copyWith((message) => updates(message as SellerResponseDraft))
          as SellerResponseDraft;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SellerResponseDraft() / SellerResponseDraft.new instead')
  static SellerResponseDraft create() => SellerResponseDraft._();
  static $pb.GeneratedMessage $_createMessage() => SellerResponseDraft._();
  @$core.override
  SellerResponseDraft createEmptyInstance() => SellerResponseDraft._();
  @$core.pragma('dart2js:noInline')
  static SellerResponseDraft getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SellerResponseDraft>(
          SellerResponseDraft.$_createMessage);
  static SellerResponseDraft? _defaultInstance;

  /// 修改草稿時帶回；client 當成不透明的字串。
  @$pb.TagNumber(1)
  $core.String get draftVersion => $_getSZ(0);
  @$pb.TagNumber(1)
  set draftVersion($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDraftVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearDraftVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.Timestamp get updateTime => $_getN(1);
  @$pb.TagNumber(2)
  set updateTime($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUpdateTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpdateTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureUpdateTime() => $_ensure(1);

  @$pb.TagNumber(3)
  SellerQuote get draft => $_getN(2);
  @$pb.TagNumber(3)
  set draft(SellerQuote value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasDraft() => $_has(2);
  @$pb.TagNumber(3)
  void clearDraft() => $_clearField(3);
  @$pb.TagNumber(3)
  SellerQuote ensureDraft() => $_ensure(2);
}

/// SellerResponseVersion 是賣家送出的一個正式回價版本，不可變。
class SellerResponseVersion extends $pb.GeneratedMessage {
  factory SellerResponseVersion({
    $core.String? sellerResponseId,
    $core.String? sellerResponseVersion,
    $core.String? publicationId,
    SellerResponseStatus? status,
    $core.String? payloadHash,
    $1.Timestamp? submitTime,
    $core.String? supersededByVersion,
    $core.bool? outsideBuyerRequestedRange,
    SellerQuote? quote,
    $core.bool? replayed,
  }) {
    final result = SellerResponseVersion._();
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (sellerResponseVersion != null)
      result.sellerResponseVersion = sellerResponseVersion;
    if (publicationId != null) result.publicationId = publicationId;
    if (status != null) result.status = status;
    if (payloadHash != null) result.payloadHash = payloadHash;
    if (submitTime != null) result.submitTime = submitTime;
    if (supersededByVersion != null)
      result.supersededByVersion = supersededByVersion;
    if (outsideBuyerRequestedRange != null)
      result.outsideBuyerRequestedRange = outsideBuyerRequestedRange;
    if (quote != null) result.quote = quote;
    if (replayed != null) result.replayed = replayed;
    return result;
  }

  SellerResponseVersion._();

  factory SellerResponseVersion.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerResponseVersion()..mergeFromBuffer(data, registry);
  factory SellerResponseVersion.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerResponseVersion()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerResponseVersion',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SellerResponseVersion.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(2, _omitFieldNames ? '' : 'sellerResponseVersion')
    ..aOS(3, _omitFieldNames ? '' : 'publicationId')
    ..aE<SellerResponseStatus>(4, _omitFieldNames ? '' : 'status',
        enumValues: SellerResponseStatus.values)
    ..aOS(5, _omitFieldNames ? '' : 'payloadHash')
    ..aOM<$1.Timestamp>(6, _omitFieldNames ? '' : 'submitTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'supersededByVersion')
    ..aOB(8, _omitFieldNames ? '' : 'outsideBuyerRequestedRange')
    ..aOM<SellerQuote>(9, _omitFieldNames ? '' : 'quote',
        subBuilder: SellerQuote.$_createMessage)
    ..aOB(10, _omitFieldNames ? '' : 'replayed')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerResponseVersion clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerResponseVersion copyWith(
          void Function(SellerResponseVersion) updates) =>
      super.copyWith((message) => updates(message as SellerResponseVersion))
          as SellerResponseVersion;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SellerResponseVersion() / SellerResponseVersion.new instead')
  static SellerResponseVersion create() => SellerResponseVersion._();
  static $pb.GeneratedMessage $_createMessage() => SellerResponseVersion._();
  @$core.override
  SellerResponseVersion createEmptyInstance() => SellerResponseVersion._();
  @$core.pragma('dart2js:noInline')
  static SellerResponseVersion getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SellerResponseVersion>(
          SellerResponseVersion.$_createMessage);
  static SellerResponseVersion? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sellerResponseId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sellerResponseId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSellerResponseId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSellerResponseId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sellerResponseVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set sellerResponseVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerResponseVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerResponseVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get publicationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set publicationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPublicationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearPublicationId() => $_clearField(3);

  @$pb.TagNumber(4)
  SellerResponseStatus get status => $_getN(3);
  @$pb.TagNumber(4)
  set status(SellerResponseStatus value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearStatus() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get payloadHash => $_getSZ(4);
  @$pb.TagNumber(5)
  set payloadHash($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPayloadHash() => $_has(4);
  @$pb.TagNumber(5)
  void clearPayloadHash() => $_clearField(5);

  @$pb.TagNumber(6)
  $1.Timestamp get submitTime => $_getN(5);
  @$pb.TagNumber(6)
  set submitTime($1.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasSubmitTime() => $_has(5);
  @$pb.TagNumber(6)
  void clearSubmitTime() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Timestamp ensureSubmitTime() => $_ensure(5);

  /// 取代這一版的版本；仍是最新版時是空字串。
  @$pb.TagNumber(7)
  $core.String get supersededByVersion => $_getSZ(6);
  @$pb.TagNumber(7)
  set supersededByVersion($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSupersededByVersion() => $_has(6);
  @$pb.TagNumber(7)
  void clearSupersededByVersion() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get outsideBuyerRequestedRange => $_getBF(7);
  @$pb.TagNumber(8)
  set outsideBuyerRequestedRange($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasOutsideBuyerRequestedRange() => $_has(7);
  @$pb.TagNumber(8)
  void clearOutsideBuyerRequestedRange() => $_clearField(8);

  @$pb.TagNumber(9)
  SellerQuote get quote => $_getN(8);
  @$pb.TagNumber(9)
  set quote(SellerQuote value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasQuote() => $_has(8);
  @$pb.TagNumber(9)
  void clearQuote() => $_clearField(9);
  @$pb.TagNumber(9)
  SellerQuote ensureQuote() => $_ensure(8);

  /// 為 true 表示這是冪等的重送，沒有建立新的版本。
  @$pb.TagNumber(10)
  $core.bool get replayed => $_getBF(9);
  @$pb.TagNumber(10)
  set replayed($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasReplayed() => $_has(9);
  @$pb.TagNumber(10)
  void clearReplayed() => $_clearField(10);
}

/// SellerResponseClosure 是回價被撤回或婉拒之後的結案狀態。
class SellerResponseClosure extends $pb.GeneratedMessage {
  factory SellerResponseClosure({
    $core.String? sellerResponseId,
    $core.String? publicationId,
    $core.String? latestVersion,
    $core.String? withdrawnVersion,
    $1.Timestamp? writeClosedTime,
    $core.String? writeClosedReason,
  }) {
    final result = SellerResponseClosure._();
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (publicationId != null) result.publicationId = publicationId;
    if (latestVersion != null) result.latestVersion = latestVersion;
    if (withdrawnVersion != null) result.withdrawnVersion = withdrawnVersion;
    if (writeClosedTime != null) result.writeClosedTime = writeClosedTime;
    if (writeClosedReason != null) result.writeClosedReason = writeClosedReason;
    return result;
  }

  SellerResponseClosure._();

  factory SellerResponseClosure.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerResponseClosure()..mergeFromBuffer(data, registry);
  factory SellerResponseClosure.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerResponseClosure()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerResponseClosure',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SellerResponseClosure.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(2, _omitFieldNames ? '' : 'publicationId')
    ..aOS(3, _omitFieldNames ? '' : 'latestVersion')
    ..aOS(4, _omitFieldNames ? '' : 'withdrawnVersion')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'writeClosedTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOS(6, _omitFieldNames ? '' : 'writeClosedReason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerResponseClosure clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerResponseClosure copyWith(
          void Function(SellerResponseClosure) updates) =>
      super.copyWith((message) => updates(message as SellerResponseClosure))
          as SellerResponseClosure;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SellerResponseClosure() / SellerResponseClosure.new instead')
  static SellerResponseClosure create() => SellerResponseClosure._();
  static $pb.GeneratedMessage $_createMessage() => SellerResponseClosure._();
  @$core.override
  SellerResponseClosure createEmptyInstance() => SellerResponseClosure._();
  @$core.pragma('dart2js:noInline')
  static SellerResponseClosure getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SellerResponseClosure>(
          SellerResponseClosure.$_createMessage);
  static SellerResponseClosure? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sellerResponseId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sellerResponseId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSellerResponseId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSellerResponseId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get publicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set publicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPublicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublicationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get latestVersion => $_getSZ(2);
  @$pb.TagNumber(3)
  set latestVersion($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLatestVersion() => $_has(2);
  @$pb.TagNumber(3)
  void clearLatestVersion() => $_clearField(3);

  /// 沒有撤回時是空字串。
  @$pb.TagNumber(4)
  $core.String get withdrawnVersion => $_getSZ(3);
  @$pb.TagNumber(4)
  set withdrawnVersion($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasWithdrawnVersion() => $_has(3);
  @$pb.TagNumber(4)
  void clearWithdrawnVersion() => $_clearField(4);

  /// 仍可寫入時不設。
  @$pb.TagNumber(5)
  $1.Timestamp get writeClosedTime => $_getN(4);
  @$pb.TagNumber(5)
  set writeClosedTime($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasWriteClosedTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearWriteClosedTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureWriteClosedTime() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get writeClosedReason => $_getSZ(5);
  @$pb.TagNumber(6)
  set writeClosedReason($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasWriteClosedReason() => $_has(5);
  @$pb.TagNumber(6)
  void clearWriteClosedReason() => $_clearField(6);
}

class SellerPublicationListItem extends $pb.GeneratedMessage {
  factory SellerPublicationListItem({
    SellerPublicationSummary? publication,
    SellerFacingRfq? sellerFacingPayload,
    SellerResponseStatus? latestResponseStatus,
    $core.bool? hasDraft,
  }) {
    final result = SellerPublicationListItem._();
    if (publication != null) result.publication = publication;
    if (sellerFacingPayload != null)
      result.sellerFacingPayload = sellerFacingPayload;
    if (latestResponseStatus != null)
      result.latestResponseStatus = latestResponseStatus;
    if (hasDraft != null) result.hasDraft = hasDraft;
    return result;
  }

  SellerPublicationListItem._();

  factory SellerPublicationListItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerPublicationListItem()..mergeFromBuffer(data, registry);
  factory SellerPublicationListItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SellerPublicationListItem()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SellerPublicationListItem',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SellerPublicationListItem.$_createMessage)
    ..aOM<SellerPublicationSummary>(1, _omitFieldNames ? '' : 'publication',
        subBuilder: SellerPublicationSummary.$_createMessage)
    ..aOM<SellerFacingRfq>(2, _omitFieldNames ? '' : 'sellerFacingPayload',
        subBuilder: SellerFacingRfq.$_createMessage)
    ..aE<SellerResponseStatus>(3, _omitFieldNames ? '' : 'latestResponseStatus',
        enumValues: SellerResponseStatus.values)
    ..aOB(4, _omitFieldNames ? '' : 'hasDraft')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerPublicationListItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SellerPublicationListItem copyWith(
          void Function(SellerPublicationListItem) updates) =>
      super.copyWith((message) => updates(message as SellerPublicationListItem))
          as SellerPublicationListItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SellerPublicationListItem() / SellerPublicationListItem.new instead')
  static SellerPublicationListItem create() => SellerPublicationListItem._();
  static $pb.GeneratedMessage $_createMessage() =>
      SellerPublicationListItem._();
  @$core.override
  SellerPublicationListItem createEmptyInstance() =>
      SellerPublicationListItem._();
  @$core.pragma('dart2js:noInline')
  static SellerPublicationListItem getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SellerPublicationListItem>(
          SellerPublicationListItem.$_createMessage);
  static SellerPublicationListItem? _defaultInstance;

  @$pb.TagNumber(1)
  SellerPublicationSummary get publication => $_getN(0);
  @$pb.TagNumber(1)
  set publication(SellerPublicationSummary value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPublication() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublication() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerPublicationSummary ensurePublication() => $_ensure(0);

  @$pb.TagNumber(2)
  SellerFacingRfq get sellerFacingPayload => $_getN(1);
  @$pb.TagNumber(2)
  set sellerFacingPayload(SellerFacingRfq value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerFacingPayload() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerFacingPayload() => $_clearField(2);
  @$pb.TagNumber(2)
  SellerFacingRfq ensureSellerFacingPayload() => $_ensure(1);

  /// 呼叫者還沒有正式回價時不設。
  @$pb.TagNumber(3)
  SellerResponseStatus get latestResponseStatus => $_getN(2);
  @$pb.TagNumber(3)
  set latestResponseStatus(SellerResponseStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasLatestResponseStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearLatestResponseStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get hasDraft => $_getBF(3);
  @$pb.TagNumber(4)
  set hasDraft($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasHasDraft() => $_has(3);
  @$pb.TagNumber(4)
  void clearHasDraft() => $_clearField(4);
}

class ListSellerPublicationsRequest extends $pb.GeneratedMessage {
  factory ListSellerPublicationsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListSellerPublicationsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListSellerPublicationsRequest._();

  factory ListSellerPublicationsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSellerPublicationsRequest()..mergeFromBuffer(data, registry);
  factory ListSellerPublicationsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSellerPublicationsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSellerPublicationsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ListSellerPublicationsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSellerPublicationsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSellerPublicationsRequest copyWith(
          void Function(ListSellerPublicationsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListSellerPublicationsRequest))
          as ListSellerPublicationsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSellerPublicationsRequest() / ListSellerPublicationsRequest.new instead')
  static ListSellerPublicationsRequest create() =>
      ListSellerPublicationsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListSellerPublicationsRequest._();
  @$core.override
  ListSellerPublicationsRequest createEmptyInstance() =>
      ListSellerPublicationsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListSellerPublicationsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSellerPublicationsRequest>(
          ListSellerPublicationsRequest.$_createMessage);
  static ListSellerPublicationsRequest? _defaultInstance;

  /// 每頁最多幾筆。0 表示 20；超過 50 時以 50 計；負數回 INVALID_ARGUMENT。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListSellerPublicationsResponse extends $pb.GeneratedMessage {
  factory ListSellerPublicationsResponse({
    $core.Iterable<SellerPublicationListItem>? publications,
    $core.String? nextPageToken,
  }) {
    final result = ListSellerPublicationsResponse._();
    if (publications != null) result.publications.addAll(publications);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListSellerPublicationsResponse._();

  factory ListSellerPublicationsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSellerPublicationsResponse()..mergeFromBuffer(data, registry);
  factory ListSellerPublicationsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSellerPublicationsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSellerPublicationsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: ListSellerPublicationsResponse.$_createMessage)
    ..pPM<SellerPublicationListItem>(1, _omitFieldNames ? '' : 'publications',
        subBuilder: SellerPublicationListItem.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSellerPublicationsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSellerPublicationsResponse copyWith(
          void Function(ListSellerPublicationsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListSellerPublicationsResponse))
          as ListSellerPublicationsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSellerPublicationsResponse() / ListSellerPublicationsResponse.new instead')
  static ListSellerPublicationsResponse create() =>
      ListSellerPublicationsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListSellerPublicationsResponse._();
  @$core.override
  ListSellerPublicationsResponse createEmptyInstance() =>
      ListSellerPublicationsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListSellerPublicationsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSellerPublicationsResponse>(
          ListSellerPublicationsResponse.$_createMessage);
  static ListSellerPublicationsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SellerPublicationListItem> get publications => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetSellerPublicationRequest extends $pb.GeneratedMessage {
  factory GetSellerPublicationRequest({
    $core.String? publicationId,
  }) {
    final result = GetSellerPublicationRequest._();
    if (publicationId != null) result.publicationId = publicationId;
    return result;
  }

  GetSellerPublicationRequest._();

  factory GetSellerPublicationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSellerPublicationRequest()..mergeFromBuffer(data, registry);
  factory GetSellerPublicationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSellerPublicationRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSellerPublicationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetSellerPublicationRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'publicationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSellerPublicationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSellerPublicationRequest copyWith(
          void Function(GetSellerPublicationRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetSellerPublicationRequest))
          as GetSellerPublicationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetSellerPublicationRequest() / GetSellerPublicationRequest.new instead')
  static GetSellerPublicationRequest create() =>
      GetSellerPublicationRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetSellerPublicationRequest._();
  @$core.override
  GetSellerPublicationRequest createEmptyInstance() =>
      GetSellerPublicationRequest._();
  @$core.pragma('dart2js:noInline')
  static GetSellerPublicationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSellerPublicationRequest>(
          GetSellerPublicationRequest.$_createMessage);
  static GetSellerPublicationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get publicationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set publicationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPublicationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublicationId() => $_clearField(1);
}

class GetSellerPublicationResponse extends $pb.GeneratedMessage {
  factory GetSellerPublicationResponse({
    SellerPublicationSummary? publication,
    SellerResponseDraft? draft,
    SellerResponseVersion? latestResponse,
    SellerFacingRfq? sellerFacingPayload,
  }) {
    final result = GetSellerPublicationResponse._();
    if (publication != null) result.publication = publication;
    if (draft != null) result.draft = draft;
    if (latestResponse != null) result.latestResponse = latestResponse;
    if (sellerFacingPayload != null)
      result.sellerFacingPayload = sellerFacingPayload;
    return result;
  }

  GetSellerPublicationResponse._();

  factory GetSellerPublicationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSellerPublicationResponse()..mergeFromBuffer(data, registry);
  factory GetSellerPublicationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetSellerPublicationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSellerPublicationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: GetSellerPublicationResponse.$_createMessage)
    ..aOM<SellerPublicationSummary>(1, _omitFieldNames ? '' : 'publication',
        subBuilder: SellerPublicationSummary.$_createMessage)
    ..aOM<SellerResponseDraft>(2, _omitFieldNames ? '' : 'draft',
        subBuilder: SellerResponseDraft.$_createMessage)
    ..aOM<SellerResponseVersion>(3, _omitFieldNames ? '' : 'latestResponse',
        subBuilder: SellerResponseVersion.$_createMessage)
    ..aOM<SellerFacingRfq>(4, _omitFieldNames ? '' : 'sellerFacingPayload',
        subBuilder: SellerFacingRfq.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSellerPublicationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSellerPublicationResponse copyWith(
          void Function(GetSellerPublicationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetSellerPublicationResponse))
          as GetSellerPublicationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetSellerPublicationResponse() / GetSellerPublicationResponse.new instead')
  static GetSellerPublicationResponse create() =>
      GetSellerPublicationResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetSellerPublicationResponse._();
  @$core.override
  GetSellerPublicationResponse createEmptyInstance() =>
      GetSellerPublicationResponse._();
  @$core.pragma('dart2js:noInline')
  static GetSellerPublicationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSellerPublicationResponse>(
          GetSellerPublicationResponse.$_createMessage);
  static GetSellerPublicationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerPublicationSummary get publication => $_getN(0);
  @$pb.TagNumber(1)
  set publication(SellerPublicationSummary value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPublication() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublication() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerPublicationSummary ensurePublication() => $_ensure(0);

  /// 還沒存過草稿時不設。
  @$pb.TagNumber(2)
  SellerResponseDraft get draft => $_getN(1);
  @$pb.TagNumber(2)
  set draft(SellerResponseDraft value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasDraft() => $_has(1);
  @$pb.TagNumber(2)
  void clearDraft() => $_clearField(2);
  @$pb.TagNumber(2)
  SellerResponseDraft ensureDraft() => $_ensure(1);

  /// 還沒有正式回價時不設。
  @$pb.TagNumber(3)
  SellerResponseVersion get latestResponse => $_getN(2);
  @$pb.TagNumber(3)
  set latestResponse(SellerResponseVersion value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasLatestResponse() => $_has(2);
  @$pb.TagNumber(3)
  void clearLatestResponse() => $_clearField(3);
  @$pb.TagNumber(3)
  SellerResponseVersion ensureLatestResponse() => $_ensure(2);

  @$pb.TagNumber(4)
  SellerFacingRfq get sellerFacingPayload => $_getN(3);
  @$pb.TagNumber(4)
  set sellerFacingPayload(SellerFacingRfq value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSellerFacingPayload() => $_has(3);
  @$pb.TagNumber(4)
  void clearSellerFacingPayload() => $_clearField(4);
  @$pb.TagNumber(4)
  SellerFacingRfq ensureSellerFacingPayload() => $_ensure(3);
}

class UpsertSellerResponseDraftRequest extends $pb.GeneratedMessage {
  factory UpsertSellerResponseDraftRequest({
    $core.String? publicationId,
    $core.String? expectedDraftVersion,
    SellerQuote? draft,
  }) {
    final result = UpsertSellerResponseDraftRequest._();
    if (publicationId != null) result.publicationId = publicationId;
    if (expectedDraftVersion != null)
      result.expectedDraftVersion = expectedDraftVersion;
    if (draft != null) result.draft = draft;
    return result;
  }

  UpsertSellerResponseDraftRequest._();

  factory UpsertSellerResponseDraftRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpsertSellerResponseDraftRequest()..mergeFromBuffer(data, registry);
  factory UpsertSellerResponseDraftRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpsertSellerResponseDraftRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpsertSellerResponseDraftRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: UpsertSellerResponseDraftRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'publicationId')
    ..aOS(2, _omitFieldNames ? '' : 'expectedDraftVersion')
    ..aOM<SellerQuote>(3, _omitFieldNames ? '' : 'draft',
        subBuilder: SellerQuote.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertSellerResponseDraftRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertSellerResponseDraftRequest copyWith(
          void Function(UpsertSellerResponseDraftRequest) updates) =>
      super.copyWith(
              (message) => updates(message as UpsertSellerResponseDraftRequest))
          as UpsertSellerResponseDraftRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpsertSellerResponseDraftRequest() / UpsertSellerResponseDraftRequest.new instead')
  static UpsertSellerResponseDraftRequest create() =>
      UpsertSellerResponseDraftRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      UpsertSellerResponseDraftRequest._();
  @$core.override
  UpsertSellerResponseDraftRequest createEmptyInstance() =>
      UpsertSellerResponseDraftRequest._();
  @$core.pragma('dart2js:noInline')
  static UpsertSellerResponseDraftRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpsertSellerResponseDraftRequest>(
          UpsertSellerResponseDraftRequest.$_createMessage);
  static UpsertSellerResponseDraftRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get publicationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set publicationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPublicationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublicationId() => $_clearField(1);

  /// 空字串表示建立第一版；草稿已存在時回 ABORTED，ErrorInfo.reason 是 VERSION_CONFLICT，metadata 的 conflictTarget 是 draft。
  /// 更新時帶讀到的草稿版本，不符同樣回 ABORTED。
  @$pb.TagNumber(2)
  $core.String get expectedDraftVersion => $_getSZ(1);
  @$pb.TagNumber(2)
  set expectedDraftVersion($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExpectedDraftVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpectedDraftVersion() => $_clearField(2);

  @$pb.TagNumber(3)
  SellerQuote get draft => $_getN(2);
  @$pb.TagNumber(3)
  set draft(SellerQuote value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasDraft() => $_has(2);
  @$pb.TagNumber(3)
  void clearDraft() => $_clearField(3);
  @$pb.TagNumber(3)
  SellerQuote ensureDraft() => $_ensure(2);
}

class UpsertSellerResponseDraftResponse extends $pb.GeneratedMessage {
  factory UpsertSellerResponseDraftResponse({
    SellerResponseDraft? draft,
  }) {
    final result = UpsertSellerResponseDraftResponse._();
    if (draft != null) result.draft = draft;
    return result;
  }

  UpsertSellerResponseDraftResponse._();

  factory UpsertSellerResponseDraftResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpsertSellerResponseDraftResponse()..mergeFromBuffer(data, registry);
  factory UpsertSellerResponseDraftResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpsertSellerResponseDraftResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpsertSellerResponseDraftResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: UpsertSellerResponseDraftResponse.$_createMessage)
    ..aOM<SellerResponseDraft>(1, _omitFieldNames ? '' : 'draft',
        subBuilder: SellerResponseDraft.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertSellerResponseDraftResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertSellerResponseDraftResponse copyWith(
          void Function(UpsertSellerResponseDraftResponse) updates) =>
      super.copyWith((message) =>
              updates(message as UpsertSellerResponseDraftResponse))
          as UpsertSellerResponseDraftResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpsertSellerResponseDraftResponse() / UpsertSellerResponseDraftResponse.new instead')
  static UpsertSellerResponseDraftResponse create() =>
      UpsertSellerResponseDraftResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      UpsertSellerResponseDraftResponse._();
  @$core.override
  UpsertSellerResponseDraftResponse createEmptyInstance() =>
      UpsertSellerResponseDraftResponse._();
  @$core.pragma('dart2js:noInline')
  static UpsertSellerResponseDraftResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpsertSellerResponseDraftResponse>(
          UpsertSellerResponseDraftResponse.$_createMessage);
  static UpsertSellerResponseDraftResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerResponseDraft get draft => $_getN(0);
  @$pb.TagNumber(1)
  set draft(SellerResponseDraft value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasDraft() => $_has(0);
  @$pb.TagNumber(1)
  void clearDraft() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerResponseDraft ensureDraft() => $_ensure(0);
}

class SubmitSellerResponseRequest extends $pb.GeneratedMessage {
  factory SubmitSellerResponseRequest({
    $core.String? publicationId,
    $core.String? clientRequestId,
    $core.String? expectedPublicationVersion,
    $core.String? expectedSellerFacingPayloadHash,
    $core.String? expectedSellerResponseVersion,
    SellerQuote? quote,
  }) {
    final result = SubmitSellerResponseRequest._();
    if (publicationId != null) result.publicationId = publicationId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (expectedPublicationVersion != null)
      result.expectedPublicationVersion = expectedPublicationVersion;
    if (expectedSellerFacingPayloadHash != null)
      result.expectedSellerFacingPayloadHash = expectedSellerFacingPayloadHash;
    if (expectedSellerResponseVersion != null)
      result.expectedSellerResponseVersion = expectedSellerResponseVersion;
    if (quote != null) result.quote = quote;
    return result;
  }

  SubmitSellerResponseRequest._();

  factory SubmitSellerResponseRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitSellerResponseRequest()..mergeFromBuffer(data, registry);
  factory SubmitSellerResponseRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitSellerResponseRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitSellerResponseRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SubmitSellerResponseRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'publicationId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(3, _omitFieldNames ? '' : 'expectedPublicationVersion')
    ..aOS(4, _omitFieldNames ? '' : 'expectedSellerFacingPayloadHash')
    ..aOS(5, _omitFieldNames ? '' : 'expectedSellerResponseVersion')
    ..aOM<SellerQuote>(6, _omitFieldNames ? '' : 'quote',
        subBuilder: SellerQuote.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitSellerResponseRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitSellerResponseRequest copyWith(
          void Function(SubmitSellerResponseRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SubmitSellerResponseRequest))
          as SubmitSellerResponseRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubmitSellerResponseRequest() / SubmitSellerResponseRequest.new instead')
  static SubmitSellerResponseRequest create() =>
      SubmitSellerResponseRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SubmitSellerResponseRequest._();
  @$core.override
  SubmitSellerResponseRequest createEmptyInstance() =>
      SubmitSellerResponseRequest._();
  @$core.pragma('dart2js:noInline')
  static SubmitSellerResponseRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitSellerResponseRequest>(
          SubmitSellerResponseRequest.$_createMessage);
  static SubmitSellerResponseRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get publicationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set publicationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPublicationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublicationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get expectedPublicationVersion => $_getSZ(2);
  @$pb.TagNumber(3)
  set expectedPublicationVersion($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasExpectedPublicationVersion() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpectedPublicationVersion() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get expectedSellerFacingPayloadHash => $_getSZ(3);
  @$pb.TagNumber(4)
  set expectedSellerFacingPayloadHash($core.String value) =>
      $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExpectedSellerFacingPayloadHash() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpectedSellerFacingPayloadHash() => $_clearField(4);

  /// 尚無正式回價時是空字串。
  @$pb.TagNumber(5)
  $core.String get expectedSellerResponseVersion => $_getSZ(4);
  @$pb.TagNumber(5)
  set expectedSellerResponseVersion($core.String value) =>
      $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpectedSellerResponseVersion() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpectedSellerResponseVersion() => $_clearField(5);

  @$pb.TagNumber(6)
  SellerQuote get quote => $_getN(5);
  @$pb.TagNumber(6)
  set quote(SellerQuote value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasQuote() => $_has(5);
  @$pb.TagNumber(6)
  void clearQuote() => $_clearField(6);
  @$pb.TagNumber(6)
  SellerQuote ensureQuote() => $_ensure(5);
}

class SubmitSellerResponseResponse extends $pb.GeneratedMessage {
  factory SubmitSellerResponseResponse({
    SellerResponseVersion? sellerResponse,
  }) {
    final result = SubmitSellerResponseResponse._();
    if (sellerResponse != null) result.sellerResponse = sellerResponse;
    return result;
  }

  SubmitSellerResponseResponse._();

  factory SubmitSellerResponseResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitSellerResponseResponse()..mergeFromBuffer(data, registry);
  factory SubmitSellerResponseResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitSellerResponseResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitSellerResponseResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: SubmitSellerResponseResponse.$_createMessage)
    ..aOM<SellerResponseVersion>(1, _omitFieldNames ? '' : 'sellerResponse',
        subBuilder: SellerResponseVersion.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitSellerResponseResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitSellerResponseResponse copyWith(
          void Function(SubmitSellerResponseResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SubmitSellerResponseResponse))
          as SubmitSellerResponseResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubmitSellerResponseResponse() / SubmitSellerResponseResponse.new instead')
  static SubmitSellerResponseResponse create() =>
      SubmitSellerResponseResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SubmitSellerResponseResponse._();
  @$core.override
  SubmitSellerResponseResponse createEmptyInstance() =>
      SubmitSellerResponseResponse._();
  @$core.pragma('dart2js:noInline')
  static SubmitSellerResponseResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitSellerResponseResponse>(
          SubmitSellerResponseResponse.$_createMessage);
  static SubmitSellerResponseResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerResponseVersion get sellerResponse => $_getN(0);
  @$pb.TagNumber(1)
  set sellerResponse(SellerResponseVersion value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSellerResponse() => $_has(0);
  @$pb.TagNumber(1)
  void clearSellerResponse() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerResponseVersion ensureSellerResponse() => $_ensure(0);
}

class WithdrawSellerResponseRequest extends $pb.GeneratedMessage {
  factory WithdrawSellerResponseRequest({
    $core.String? publicationId,
    $core.String? sellerResponseId,
    $core.String? clientRequestId,
    $core.String? expectedSellerResponseVersion,
    $core.String? reasonCode,
  }) {
    final result = WithdrawSellerResponseRequest._();
    if (publicationId != null) result.publicationId = publicationId;
    if (sellerResponseId != null) result.sellerResponseId = sellerResponseId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (expectedSellerResponseVersion != null)
      result.expectedSellerResponseVersion = expectedSellerResponseVersion;
    if (reasonCode != null) result.reasonCode = reasonCode;
    return result;
  }

  WithdrawSellerResponseRequest._();

  factory WithdrawSellerResponseRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WithdrawSellerResponseRequest()..mergeFromBuffer(data, registry);
  factory WithdrawSellerResponseRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WithdrawSellerResponseRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WithdrawSellerResponseRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: WithdrawSellerResponseRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'publicationId')
    ..aOS(2, _omitFieldNames ? '' : 'sellerResponseId')
    ..aOS(3, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(4, _omitFieldNames ? '' : 'expectedSellerResponseVersion')
    ..aOS(5, _omitFieldNames ? '' : 'reasonCode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WithdrawSellerResponseRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WithdrawSellerResponseRequest copyWith(
          void Function(WithdrawSellerResponseRequest) updates) =>
      super.copyWith(
              (message) => updates(message as WithdrawSellerResponseRequest))
          as WithdrawSellerResponseRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use WithdrawSellerResponseRequest() / WithdrawSellerResponseRequest.new instead')
  static WithdrawSellerResponseRequest create() =>
      WithdrawSellerResponseRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      WithdrawSellerResponseRequest._();
  @$core.override
  WithdrawSellerResponseRequest createEmptyInstance() =>
      WithdrawSellerResponseRequest._();
  @$core.pragma('dart2js:noInline')
  static WithdrawSellerResponseRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WithdrawSellerResponseRequest>(
          WithdrawSellerResponseRequest.$_createMessage);
  static WithdrawSellerResponseRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get publicationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set publicationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPublicationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublicationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sellerResponseId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sellerResponseId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSellerResponseId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSellerResponseId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get clientRequestId => $_getSZ(2);
  @$pb.TagNumber(3)
  set clientRequestId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasClientRequestId() => $_has(2);
  @$pb.TagNumber(3)
  void clearClientRequestId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get expectedSellerResponseVersion => $_getSZ(3);
  @$pb.TagNumber(4)
  set expectedSellerResponseVersion($core.String value) =>
      $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExpectedSellerResponseVersion() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpectedSellerResponseVersion() => $_clearField(4);

  /// 賣家填的撤回原因代碼，伺服器原樣記錄、不解析。
  @$pb.TagNumber(5)
  $core.String get reasonCode => $_getSZ(4);
  @$pb.TagNumber(5)
  set reasonCode($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasReasonCode() => $_has(4);
  @$pb.TagNumber(5)
  void clearReasonCode() => $_clearField(5);
}

class WithdrawSellerResponseResponse extends $pb.GeneratedMessage {
  factory WithdrawSellerResponseResponse({
    SellerResponseClosure? closure,
  }) {
    final result = WithdrawSellerResponseResponse._();
    if (closure != null) result.closure = closure;
    return result;
  }

  WithdrawSellerResponseResponse._();

  factory WithdrawSellerResponseResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WithdrawSellerResponseResponse()..mergeFromBuffer(data, registry);
  factory WithdrawSellerResponseResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WithdrawSellerResponseResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WithdrawSellerResponseResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: WithdrawSellerResponseResponse.$_createMessage)
    ..aOM<SellerResponseClosure>(1, _omitFieldNames ? '' : 'closure',
        subBuilder: SellerResponseClosure.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WithdrawSellerResponseResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WithdrawSellerResponseResponse copyWith(
          void Function(WithdrawSellerResponseResponse) updates) =>
      super.copyWith(
              (message) => updates(message as WithdrawSellerResponseResponse))
          as WithdrawSellerResponseResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use WithdrawSellerResponseResponse() / WithdrawSellerResponseResponse.new instead')
  static WithdrawSellerResponseResponse create() =>
      WithdrawSellerResponseResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      WithdrawSellerResponseResponse._();
  @$core.override
  WithdrawSellerResponseResponse createEmptyInstance() =>
      WithdrawSellerResponseResponse._();
  @$core.pragma('dart2js:noInline')
  static WithdrawSellerResponseResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WithdrawSellerResponseResponse>(
          WithdrawSellerResponseResponse.$_createMessage);
  static WithdrawSellerResponseResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerResponseClosure get closure => $_getN(0);
  @$pb.TagNumber(1)
  set closure(SellerResponseClosure value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasClosure() => $_has(0);
  @$pb.TagNumber(1)
  void clearClosure() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerResponseClosure ensureClosure() => $_ensure(0);
}

class DeclineRfqPublicationRequest extends $pb.GeneratedMessage {
  factory DeclineRfqPublicationRequest({
    $core.String? publicationId,
    $core.String? clientRequestId,
    $core.String? reasonCode,
  }) {
    final result = DeclineRfqPublicationRequest._();
    if (publicationId != null) result.publicationId = publicationId;
    if (clientRequestId != null) result.clientRequestId = clientRequestId;
    if (reasonCode != null) result.reasonCode = reasonCode;
    return result;
  }

  DeclineRfqPublicationRequest._();

  factory DeclineRfqPublicationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeclineRfqPublicationRequest()..mergeFromBuffer(data, registry);
  factory DeclineRfqPublicationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeclineRfqPublicationRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineRfqPublicationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: DeclineRfqPublicationRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'publicationId')
    ..aOS(2, _omitFieldNames ? '' : 'clientRequestId')
    ..aOS(3, _omitFieldNames ? '' : 'reasonCode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineRfqPublicationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineRfqPublicationRequest copyWith(
          void Function(DeclineRfqPublicationRequest) updates) =>
      super.copyWith(
              (message) => updates(message as DeclineRfqPublicationRequest))
          as DeclineRfqPublicationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeclineRfqPublicationRequest() / DeclineRfqPublicationRequest.new instead')
  static DeclineRfqPublicationRequest create() =>
      DeclineRfqPublicationRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      DeclineRfqPublicationRequest._();
  @$core.override
  DeclineRfqPublicationRequest createEmptyInstance() =>
      DeclineRfqPublicationRequest._();
  @$core.pragma('dart2js:noInline')
  static DeclineRfqPublicationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineRfqPublicationRequest>(
          DeclineRfqPublicationRequest.$_createMessage);
  static DeclineRfqPublicationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get publicationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set publicationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPublicationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPublicationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get clientRequestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set clientRequestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientRequestId() => $_clearField(2);

  /// 賣家填的婉拒原因代碼，伺服器原樣記錄、不解析。
  @$pb.TagNumber(3)
  $core.String get reasonCode => $_getSZ(2);
  @$pb.TagNumber(3)
  set reasonCode($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReasonCode() => $_has(2);
  @$pb.TagNumber(3)
  void clearReasonCode() => $_clearField(3);
}

class DeclineRfqPublicationResponse extends $pb.GeneratedMessage {
  factory DeclineRfqPublicationResponse({
    SellerResponseClosure? closure,
  }) {
    final result = DeclineRfqPublicationResponse._();
    if (closure != null) result.closure = closure;
    return result;
  }

  DeclineRfqPublicationResponse._();

  factory DeclineRfqPublicationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeclineRfqPublicationResponse()..mergeFromBuffer(data, registry);
  factory DeclineRfqPublicationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeclineRfqPublicationResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineRfqPublicationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.procurement.v1alpha1'),
      createEmptyInstance: DeclineRfqPublicationResponse.$_createMessage)
    ..aOM<SellerResponseClosure>(1, _omitFieldNames ? '' : 'closure',
        subBuilder: SellerResponseClosure.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineRfqPublicationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineRfqPublicationResponse copyWith(
          void Function(DeclineRfqPublicationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as DeclineRfqPublicationResponse))
          as DeclineRfqPublicationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeclineRfqPublicationResponse() / DeclineRfqPublicationResponse.new instead')
  static DeclineRfqPublicationResponse create() =>
      DeclineRfqPublicationResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      DeclineRfqPublicationResponse._();
  @$core.override
  DeclineRfqPublicationResponse createEmptyInstance() =>
      DeclineRfqPublicationResponse._();
  @$core.pragma('dart2js:noInline')
  static DeclineRfqPublicationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineRfqPublicationResponse>(
          DeclineRfqPublicationResponse.$_createMessage);
  static DeclineRfqPublicationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SellerResponseClosure get closure => $_getN(0);
  @$pb.TagNumber(1)
  set closure(SellerResponseClosure value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasClosure() => $_has(0);
  @$pb.TagNumber(1)
  void clearClosure() => $_clearField(1);
  @$pb.TagNumber(1)
  SellerResponseClosure ensureClosure() => $_ensure(0);
}

/// BuyerProcurementAppService 是登入的買家的採購方法。身分取自 session，帳號沒有買家角色時回 permission_denied。
/// 不屬於呼叫者的需求、對話與 run 一律回 not_found。
class BuyerProcurementAppServiceApi {
  final $pb.RpcClient _client;

  BuyerProcurementAppServiceApi(this._client);

  /// SubmitTurn 送出買家的一則訊息，開始一輪釐清。request_id 空字串表示新建需求。伺服器受理後立刻回 run ID，
  /// 這一輪的過程與結果（turn_result 事件）從 WatchRun 取得。每個買家同時進行的輪次有上限，超過回 resource_exhausted；
  /// 本期額度用完回 resource_exhausted，reason 是 AGENT_MONTHLY_QUOTA_EXCEEDED。
  $async.Future<SubmitTurnResponse> submitTurn(
          $pb.ClientContext? ctx, SubmitTurnRequest request) =>
      _client.invoke<SubmitTurnResponse>(ctx, 'BuyerProcurementAppService',
          'SubmitTurn', request, SubmitTurnResponse());

  /// GetSnapshot 回傳一張需求目前的內容與狀態。
  $async.Future<GetSnapshotResponse> getSnapshot(
          $pb.ClientContext? ctx, GetSnapshotRequest request) =>
      _client.invoke<GetSnapshotResponse>(ctx, 'BuyerProcurementAppService',
          'GetSnapshot', request, GetSnapshotResponse());

  /// StartCandidateSearch 開始為需求搜尋候選賣家（背景執行，以 GetCandidateSearch 取得結果）。
  /// 需求還沒齊備回 FAILED_PRECONDITION，reason 是 REQUIREMENT_NOT_READY。
  $async.Future<StartCandidateSearchResponse> startCandidateSearch(
          $pb.ClientContext? ctx, StartCandidateSearchRequest request) =>
      _client.invoke<StartCandidateSearchResponse>(
          ctx,
          'BuyerProcurementAppService',
          'StartCandidateSearch',
          request,
          StartCandidateSearchResponse());

  /// GetCandidateSearch 回傳一次候選搜尋的狀態與目前的候選。
  $async.Future<GetCandidateSearchResponse> getCandidateSearch(
          $pb.ClientContext? ctx, GetCandidateSearchRequest request) =>
      _client.invoke<GetCandidateSearchResponse>(
          ctx,
          'BuyerProcurementAppService',
          'GetCandidateSearch',
          request,
          GetCandidateSearchResponse());

  /// ToggleSelection 勾選或取消勾選一個候選。候選的依據已早於目前的需求版本回 ABORTED，conflictTarget 是 candidate_set。
  $async.Future<ToggleSelectionResponse> toggleSelection(
          $pb.ClientContext? ctx, ToggleSelectionRequest request) =>
      _client.invoke<ToggleSelectionResponse>(ctx, 'BuyerProcurementAppService',
          'ToggleSelection', request, ToggleSelectionResponse());

  /// SubmitAnswer 回答一個釐清問題。需求有待回覆的中斷時改用 ResumeRun，回 FAILED_PRECONDITION，
  /// reason 是 APPROVAL_PENDING。
  $async.Future<SubmitAnswerResponse> submitAnswer(
          $pb.ClientContext? ctx, SubmitAnswerRequest request) =>
      _client.invoke<SubmitAnswerResponse>(ctx, 'BuyerProcurementAppService',
          'SubmitAnswer', request, SubmitAnswerResponse());

  /// Rewind 讓需求退回某個欄位重新回答。有待回覆的中斷時規則同 SubmitAnswer。
  $async.Future<RewindResponse> rewind(
          $pb.ClientContext? ctx, RewindRequest request) =>
      _client.invoke<RewindResponse>(ctx, 'BuyerProcurementAppService',
          'Rewind', request, RewindResponse());

  /// PreviewRfqPrice 依買家預期的單價範圍與需求數量，算出詢價總額區間並存成預覽（有效 15 分鐘）。
  /// 買家沒有給合法的範圍（下界與上界是正整數、下界不大於上界）回 INVALID_ARGUMENT；
  /// 需求已有數量與單位卻算不出總額回 FAILED_PRECONDITION。
  $async.Future<PreviewRfqPriceResponse> previewRfqPrice(
          $pb.ClientContext? ctx, PreviewRfqPriceRequest request) =>
      _client.invoke<PreviewRfqPriceResponse>(ctx, 'BuyerProcurementAppService',
          'PreviewRfqPrice', request, PreviewRfqPriceResponse());

  /// ConfirmRfqSnapshot 對一份價格預覽產生不可變的詢價確認快照。預覽或需求版本過時回 ABORTED。
  $async.Future<ConfirmRfqSnapshotResponse> confirmRfqSnapshot(
          $pb.ClientContext? ctx, ConfirmRfqSnapshotRequest request) =>
      _client.invoke<ConfirmRfqSnapshotResponse>(
          ctx,
          'BuyerProcurementAppService',
          'ConfirmRfqSnapshot',
          request,
          ConfirmRfqSnapshotResponse());

  /// PreviewRfqPublication 依確認快照編出要發給賣家的內容並建立不可變的發布核准，不發布、不通知。
  $async.Future<PreviewRfqPublicationResponse> previewRfqPublication(
          $pb.ClientContext? ctx, PreviewRfqPublicationRequest request) =>
      _client.invoke<PreviewRfqPublicationResponse>(
          ctx,
          'BuyerProcurementAppService',
          'PreviewRfqPublication',
          request,
          PreviewRfqPublicationResponse());

  /// ExecuteRfqPublication 以發布核准的版本執行發布。成功只代表伺服器已建立發布，不代表已投遞或賣家已看到。
  $async.Future<ExecuteRfqPublicationResponse> executeRfqPublication(
          $pb.ClientContext? ctx, ExecuteRfqPublicationRequest request) =>
      _client.invoke<ExecuteRfqPublicationResponse>(
          ctx,
          'BuyerProcurementAppService',
          'ExecuteRfqPublication',
          request,
          ExecuteRfqPublicationResponse());

  /// ListConversations 列出買家的對話，最近更新的在前。
  $async.Future<ListConversationsResponse> listConversations(
          $pb.ClientContext? ctx, ListConversationsRequest request) =>
      _client.invoke<ListConversationsResponse>(
          ctx,
          'BuyerProcurementAppService',
          'ListConversations',
          request,
          ListConversationsResponse());
  $async.Future<GetConversationMessagesResponse> getConversationMessages(
          $pb.ClientContext? ctx, GetConversationMessagesRequest request) =>
      _client.invoke<GetConversationMessagesResponse>(
          ctx,
          'BuyerProcurementAppService',
          'GetConversationMessages',
          request,
          GetConversationMessagesResponse());

  /// GetRuntimeSnapshot 回傳重建一個對話畫面需要的全部事實：訊息、run、待回覆的中斷與發布結果。
  $async.Future<GetRuntimeSnapshotResponse> getRuntimeSnapshot(
          $pb.ClientContext? ctx, GetRuntimeSnapshotRequest request) =>
      _client.invoke<GetRuntimeSnapshotResponse>(
          ctx,
          'BuyerProcurementAppService',
          'GetRuntimeSnapshot',
          request,
          GetRuntimeSnapshotResponse());

  /// ListSellerResponses 列出一張需求已發布的詢價收到的賣家回價（每位賣家的目前版本）。
  $async.Future<ListSellerResponsesResponse> listSellerResponses(
          $pb.ClientContext? ctx, ListSellerResponsesRequest request) =>
      _client.invoke<ListSellerResponsesResponse>(
          ctx,
          'BuyerProcurementAppService',
          'ListSellerResponses',
          request,
          ListSellerResponsesResponse());

  /// PutShortlistEntry 把賣家回價的一個版本加入入圍清單；已在清單時回目前的狀態。
  $async.Future<PutShortlistEntryResponse> putShortlistEntry(
          $pb.ClientContext? ctx, PutShortlistEntryRequest request) =>
      _client.invoke<PutShortlistEntryResponse>(
          ctx,
          'BuyerProcurementAppService',
          'PutShortlistEntry',
          request,
          PutShortlistEntryResponse());

  /// DeleteShortlistEntry 把賣家回價從入圍清單移除；不在清單時回目前的狀態。
  $async.Future<DeleteShortlistEntryResponse> deleteShortlistEntry(
          $pb.ClientContext? ctx, DeleteShortlistEntryRequest request) =>
      _client.invoke<DeleteShortlistEntryResponse>(
          ctx,
          'BuyerProcurementAppService',
          'DeleteShortlistEntry',
          request,
          DeleteShortlistEntryResponse());

  /// AcceptSellerQuote 接受賣家回價的指定版本並凍結成交價，隨後建立待付款訂單。報價已改版、已撤回回 ABORTED，
  /// conflictTarget 是 seller_response；報價已過有效期回 FAILED_PRECONDITION，reason 是 RFQ_RESPONSE_EXPIRED；
  /// 同一張需求已有進行中的成交回 FAILED_PRECONDITION，reason 是 RFQ_CHECKOUT_ALREADY_ACTIVE（metadata 的 orderNo 是
  /// 既有訂單，買家去付款，不是重新接受）；已經成交回 FAILED_PRECONDITION，reason 是 RFQ_ALREADY_AWARDED。
  $async.Future<AcceptSellerQuoteResponse> acceptSellerQuote(
          $pb.ClientContext? ctx, AcceptSellerQuoteRequest request) =>
      _client.invoke<AcceptSellerQuoteResponse>(
          ctx,
          'BuyerProcurementAppService',
          'AcceptSellerQuote',
          request,
          AcceptSellerQuoteResponse());
  $async.Future<RenameConversationResponse> renameConversation(
          $pb.ClientContext? ctx, RenameConversationRequest request) =>
      _client.invoke<RenameConversationResponse>(
          ctx,
          'BuyerProcurementAppService',
          'RenameConversation',
          request,
          RenameConversationResponse());
  $async.Future<DeleteConversationResponse> deleteConversation(
          $pb.ClientContext? ctx, DeleteConversationRequest request) =>
      _client.invoke<DeleteConversationResponse>(
          ctx,
          'BuyerProcurementAppService',
          'DeleteConversation',
          request,
          DeleteConversationResponse());

  /// WatchRun 訂閱一個 run 的事件，從 cursor 之後開始（空字串表示從頭）。只讀；伺服器在 run 結束（finished 或 error 事件之後）
  /// 或滿 100 秒時結束串流。每位買家同時訂閱的串流有上限，超過回 resource_exhausted。
  $async.Future<WatchRunResponse> watchRun(
          $pb.ClientContext? ctx, WatchRunRequest request) =>
      _client.invoke<WatchRunResponse>(ctx, 'BuyerProcurementAppService',
          'WatchRun', request, WatchRunResponse());

  /// CancelRun 取消一個進行中的 run。run 已不在進行中不會回錯，cancel_disposition 說明結果。
  $async.Future<CancelRunResponse> cancelRun(
          $pb.ClientContext? ctx, CancelRunRequest request) =>
      _client.invoke<CancelRunResponse>(ctx, 'BuyerProcurementAppService',
          'CancelRun', request, CancelRunResponse());

  /// ResumeRun 回覆 run 停下來等待的中斷，從中斷處接續。中斷已被回覆回 FAILED_PRECONDITION，
  /// reason 是 APPROVAL_ALREADY_RESOLVED；run 不在等待輸入的狀態回 FAILED_PRECONDITION，reason 是 RUN_NOT_ACTIVE。
  $async.Future<ResumeRunResponse> resumeRun(
          $pb.ClientContext? ctx, ResumeRunRequest request) =>
      _client.invoke<ResumeRunResponse>(ctx, 'BuyerProcurementAppService',
          'ResumeRun', request, ResumeRunResponse());
}

/// SellerProcurementAppService 是登入的賣家的回價方法。店家取自呼叫者目前的店家，需要賣家能力 CAP_MARKETING。
/// 只能看到發給自己的詢價，也只能處理自己的回價；其他的一律回 not_found。
class SellerProcurementAppServiceApi {
  final $pb.RpcClient _client;

  SellerProcurementAppServiceApi(this._client);

  /// ListSellerPublications 列出發給呼叫者的詢價，最新的在前。
  $async.Future<ListSellerPublicationsResponse> listSellerPublications(
          $pb.ClientContext? ctx, ListSellerPublicationsRequest request) =>
      _client.invoke<ListSellerPublicationsResponse>(
          ctx,
          'SellerProcurementAppService',
          'ListSellerPublications',
          request,
          ListSellerPublicationsResponse());

  /// GetSellerPublication 回傳一筆詢價的內容，加上呼叫者自己的草稿與目前的正式回價。只含呼叫者自己的回價。
  $async.Future<GetSellerPublicationResponse> getSellerPublication(
          $pb.ClientContext? ctx, GetSellerPublicationRequest request) =>
      _client.invoke<GetSellerPublicationResponse>(
          ctx,
          'SellerProcurementAppService',
          'GetSellerPublication',
          request,
          GetSellerPublicationResponse());

  /// UpsertSellerResponseDraft 建立或更新回價草稿。草稿不驗金額恆等式，買家讀不到。
  $async.Future<UpsertSellerResponseDraftResponse> upsertSellerResponseDraft(
          $pb.ClientContext? ctx, UpsertSellerResponseDraftRequest request) =>
      _client.invoke<UpsertSellerResponseDraftResponse>(
          ctx,
          'SellerProcurementAppService',
          'UpsertSellerResponseDraft',
          request,
          UpsertSellerResponseDraftResponse());

  /// SubmitSellerResponse 送出正式回價，產生不可變的新版本。詢價已被改版回 ABORTED，conflictTarget 是 publication；
  /// 帶的 expected_seller_response_version 不是目前的版本（賣家已改價或撤回，或空字串卻已有正式回價）回 ABORTED，
  /// conflictTarget 是 seller_response，client 必須明確告知使用者報價已變動，不得改用最新版靜默重送。
  /// 沒有填實際品相、金額恆等式不成立回 INVALID_ARGUMENT。已停止寫入（賣家已婉拒、詢價已結案或他人成交）回
  /// FAILED_PRECONDITION，reason 是 RFQ_RECIPIENT_CLOSED。
  $async.Future<SubmitSellerResponseResponse> submitSellerResponse(
          $pb.ClientContext? ctx, SubmitSellerResponseRequest request) =>
      _client.invoke<SubmitSellerResponseResponse>(
          ctx,
          'SellerProcurementAppService',
          'SubmitSellerResponse',
          request,
          SubmitSellerResponseResponse());

  /// WithdrawSellerResponse 撤回正式回價。
  $async.Future<WithdrawSellerResponseResponse> withdrawSellerResponse(
          $pb.ClientContext? ctx, WithdrawSellerResponseRequest request) =>
      _client.invoke<WithdrawSellerResponseResponse>(
          ctx,
          'SellerProcurementAppService',
          'WithdrawSellerResponse',
          request,
          WithdrawSellerResponseResponse());

  /// DeclineRfqPublication 婉拒一筆詢價，之後不能再回價。
  $async.Future<DeclineRfqPublicationResponse> declineRfqPublication(
          $pb.ClientContext? ctx, DeclineRfqPublicationRequest request) =>
      _client.invoke<DeclineRfqPublicationResponse>(
          ctx,
          'SellerProcurementAppService',
          'DeclineRfqPublication',
          request,
          DeclineRfqPublicationResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
