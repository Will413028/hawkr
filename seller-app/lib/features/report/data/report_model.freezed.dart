// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'report_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$ReportRecord {

 String get id; String get targetType; String get targetId; String get reasonCode; String get description; String get status; String? get resolvedAction; String? get resolvedBy;@JsonKey(fromJson: _parseMsNullable) int? get resolvedAtMs;@JsonKey(fromJson: _parseMs) int get createTimeMs; String get recentActionCount;
/// Create a copy of ReportRecord
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ReportRecordCopyWith<ReportRecord> get copyWith => _$ReportRecordCopyWithImpl<ReportRecord>(this as ReportRecord, _$identity);

  /// Serializes this ReportRecord to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ReportRecord&&(identical(other.id, id) || other.id == id)&&(identical(other.targetType, targetType) || other.targetType == targetType)&&(identical(other.targetId, targetId) || other.targetId == targetId)&&(identical(other.reasonCode, reasonCode) || other.reasonCode == reasonCode)&&(identical(other.description, description) || other.description == description)&&(identical(other.status, status) || other.status == status)&&(identical(other.resolvedAction, resolvedAction) || other.resolvedAction == resolvedAction)&&(identical(other.resolvedBy, resolvedBy) || other.resolvedBy == resolvedBy)&&(identical(other.resolvedAtMs, resolvedAtMs) || other.resolvedAtMs == resolvedAtMs)&&(identical(other.createTimeMs, createTimeMs) || other.createTimeMs == createTimeMs)&&(identical(other.recentActionCount, recentActionCount) || other.recentActionCount == recentActionCount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,targetType,targetId,reasonCode,description,status,resolvedAction,resolvedBy,resolvedAtMs,createTimeMs,recentActionCount);

@override
String toString() {
  return 'ReportRecord(id: $id, targetType: $targetType, targetId: $targetId, reasonCode: $reasonCode, description: $description, status: $status, resolvedAction: $resolvedAction, resolvedBy: $resolvedBy, resolvedAtMs: $resolvedAtMs, createTimeMs: $createTimeMs, recentActionCount: $recentActionCount)';
}


}

/// @nodoc
abstract mixin class $ReportRecordCopyWith<$Res>  {
  factory $ReportRecordCopyWith(ReportRecord value, $Res Function(ReportRecord) _then) = _$ReportRecordCopyWithImpl;
@useResult
$Res call({
 String id, String targetType, String targetId, String reasonCode, String description, String status, String? resolvedAction, String? resolvedBy,@JsonKey(fromJson: _parseMsNullable) int? resolvedAtMs,@JsonKey(fromJson: _parseMs) int createTimeMs, String recentActionCount
});




}
/// @nodoc
class _$ReportRecordCopyWithImpl<$Res>
    implements $ReportRecordCopyWith<$Res> {
  _$ReportRecordCopyWithImpl(this._self, this._then);

  final ReportRecord _self;
  final $Res Function(ReportRecord) _then;

/// Create a copy of ReportRecord
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? targetType = null,Object? targetId = null,Object? reasonCode = null,Object? description = null,Object? status = null,Object? resolvedAction = freezed,Object? resolvedBy = freezed,Object? resolvedAtMs = freezed,Object? createTimeMs = null,Object? recentActionCount = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,targetType: null == targetType ? _self.targetType : targetType // ignore: cast_nullable_to_non_nullable
as String,targetId: null == targetId ? _self.targetId : targetId // ignore: cast_nullable_to_non_nullable
as String,reasonCode: null == reasonCode ? _self.reasonCode : reasonCode // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,resolvedAction: freezed == resolvedAction ? _self.resolvedAction : resolvedAction // ignore: cast_nullable_to_non_nullable
as String?,resolvedBy: freezed == resolvedBy ? _self.resolvedBy : resolvedBy // ignore: cast_nullable_to_non_nullable
as String?,resolvedAtMs: freezed == resolvedAtMs ? _self.resolvedAtMs : resolvedAtMs // ignore: cast_nullable_to_non_nullable
as int?,createTimeMs: null == createTimeMs ? _self.createTimeMs : createTimeMs // ignore: cast_nullable_to_non_nullable
as int,recentActionCount: null == recentActionCount ? _self.recentActionCount : recentActionCount // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [ReportRecord].
extension ReportRecordPatterns on ReportRecord {
/// A variant of `map` that fallback to returning `orElse`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ReportRecord value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ReportRecord() when $default != null:
return $default(_that);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// Callbacks receives the raw object, upcasted.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case final Subclass2 value:
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ReportRecord value)  $default,){
final _that = this;
switch (_that) {
case _ReportRecord():
return $default(_that);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `map` that fallback to returning `null`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ReportRecord value)?  $default,){
final _that = this;
switch (_that) {
case _ReportRecord() when $default != null:
return $default(_that);case _:
  return null;

}
}
/// A variant of `when` that fallback to an `orElse` callback.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String targetType,  String targetId,  String reasonCode,  String description,  String status,  String? resolvedAction,  String? resolvedBy, @JsonKey(fromJson: _parseMsNullable)  int? resolvedAtMs, @JsonKey(fromJson: _parseMs)  int createTimeMs,  String recentActionCount)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ReportRecord() when $default != null:
return $default(_that.id,_that.targetType,_that.targetId,_that.reasonCode,_that.description,_that.status,_that.resolvedAction,_that.resolvedBy,_that.resolvedAtMs,_that.createTimeMs,_that.recentActionCount);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// As opposed to `map`, this offers destructuring.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case Subclass2(:final field2):
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String targetType,  String targetId,  String reasonCode,  String description,  String status,  String? resolvedAction,  String? resolvedBy, @JsonKey(fromJson: _parseMsNullable)  int? resolvedAtMs, @JsonKey(fromJson: _parseMs)  int createTimeMs,  String recentActionCount)  $default,) {final _that = this;
switch (_that) {
case _ReportRecord():
return $default(_that.id,_that.targetType,_that.targetId,_that.reasonCode,_that.description,_that.status,_that.resolvedAction,_that.resolvedBy,_that.resolvedAtMs,_that.createTimeMs,_that.recentActionCount);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `when` that fallback to returning `null`
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String targetType,  String targetId,  String reasonCode,  String description,  String status,  String? resolvedAction,  String? resolvedBy, @JsonKey(fromJson: _parseMsNullable)  int? resolvedAtMs, @JsonKey(fromJson: _parseMs)  int createTimeMs,  String recentActionCount)?  $default,) {final _that = this;
switch (_that) {
case _ReportRecord() when $default != null:
return $default(_that.id,_that.targetType,_that.targetId,_that.reasonCode,_that.description,_that.status,_that.resolvedAction,_that.resolvedBy,_that.resolvedAtMs,_that.createTimeMs,_that.recentActionCount);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _ReportRecord implements ReportRecord {
  const _ReportRecord({this.id = '', this.targetType = '', this.targetId = '', this.reasonCode = '', this.description = '', this.status = '', this.resolvedAction, this.resolvedBy, @JsonKey(fromJson: _parseMsNullable) this.resolvedAtMs, @JsonKey(fromJson: _parseMs) this.createTimeMs = 0, this.recentActionCount = '0'});
  factory _ReportRecord.fromJson(Map<String, dynamic> json) => _$ReportRecordFromJson(json);

@override@JsonKey() final  String id;
@override@JsonKey() final  String targetType;
@override@JsonKey() final  String targetId;
@override@JsonKey() final  String reasonCode;
@override@JsonKey() final  String description;
@override@JsonKey() final  String status;
@override final  String? resolvedAction;
@override final  String? resolvedBy;
@override@JsonKey(fromJson: _parseMsNullable) final  int? resolvedAtMs;
@override@JsonKey(fromJson: _parseMs) final  int createTimeMs;
@override@JsonKey() final  String recentActionCount;

/// Create a copy of ReportRecord
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ReportRecordCopyWith<_ReportRecord> get copyWith => __$ReportRecordCopyWithImpl<_ReportRecord>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$ReportRecordToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ReportRecord&&(identical(other.id, id) || other.id == id)&&(identical(other.targetType, targetType) || other.targetType == targetType)&&(identical(other.targetId, targetId) || other.targetId == targetId)&&(identical(other.reasonCode, reasonCode) || other.reasonCode == reasonCode)&&(identical(other.description, description) || other.description == description)&&(identical(other.status, status) || other.status == status)&&(identical(other.resolvedAction, resolvedAction) || other.resolvedAction == resolvedAction)&&(identical(other.resolvedBy, resolvedBy) || other.resolvedBy == resolvedBy)&&(identical(other.resolvedAtMs, resolvedAtMs) || other.resolvedAtMs == resolvedAtMs)&&(identical(other.createTimeMs, createTimeMs) || other.createTimeMs == createTimeMs)&&(identical(other.recentActionCount, recentActionCount) || other.recentActionCount == recentActionCount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,targetType,targetId,reasonCode,description,status,resolvedAction,resolvedBy,resolvedAtMs,createTimeMs,recentActionCount);

@override
String toString() {
  return 'ReportRecord(id: $id, targetType: $targetType, targetId: $targetId, reasonCode: $reasonCode, description: $description, status: $status, resolvedAction: $resolvedAction, resolvedBy: $resolvedBy, resolvedAtMs: $resolvedAtMs, createTimeMs: $createTimeMs, recentActionCount: $recentActionCount)';
}


}

/// @nodoc
abstract mixin class _$ReportRecordCopyWith<$Res> implements $ReportRecordCopyWith<$Res> {
  factory _$ReportRecordCopyWith(_ReportRecord value, $Res Function(_ReportRecord) _then) = __$ReportRecordCopyWithImpl;
@override @useResult
$Res call({
 String id, String targetType, String targetId, String reasonCode, String description, String status, String? resolvedAction, String? resolvedBy,@JsonKey(fromJson: _parseMsNullable) int? resolvedAtMs,@JsonKey(fromJson: _parseMs) int createTimeMs, String recentActionCount
});




}
/// @nodoc
class __$ReportRecordCopyWithImpl<$Res>
    implements _$ReportRecordCopyWith<$Res> {
  __$ReportRecordCopyWithImpl(this._self, this._then);

  final _ReportRecord _self;
  final $Res Function(_ReportRecord) _then;

/// Create a copy of ReportRecord
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? targetType = null,Object? targetId = null,Object? reasonCode = null,Object? description = null,Object? status = null,Object? resolvedAction = freezed,Object? resolvedBy = freezed,Object? resolvedAtMs = freezed,Object? createTimeMs = null,Object? recentActionCount = null,}) {
  return _then(_ReportRecord(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,targetType: null == targetType ? _self.targetType : targetType // ignore: cast_nullable_to_non_nullable
as String,targetId: null == targetId ? _self.targetId : targetId // ignore: cast_nullable_to_non_nullable
as String,reasonCode: null == reasonCode ? _self.reasonCode : reasonCode // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,resolvedAction: freezed == resolvedAction ? _self.resolvedAction : resolvedAction // ignore: cast_nullable_to_non_nullable
as String?,resolvedBy: freezed == resolvedBy ? _self.resolvedBy : resolvedBy // ignore: cast_nullable_to_non_nullable
as String?,resolvedAtMs: freezed == resolvedAtMs ? _self.resolvedAtMs : resolvedAtMs // ignore: cast_nullable_to_non_nullable
as int?,createTimeMs: null == createTimeMs ? _self.createTimeMs : createTimeMs // ignore: cast_nullable_to_non_nullable
as int,recentActionCount: null == recentActionCount ? _self.recentActionCount : recentActionCount // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
