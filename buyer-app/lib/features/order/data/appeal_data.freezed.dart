// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'appeal_data.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$AppealEvent {

 String get eventType; String get timestamp; String get label; String get description;
/// Create a copy of AppealEvent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AppealEventCopyWith<AppealEvent> get copyWith => _$AppealEventCopyWithImpl<AppealEvent>(this as AppealEvent, _$identity);

  /// Serializes this AppealEvent to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AppealEvent&&(identical(other.eventType, eventType) || other.eventType == eventType)&&(identical(other.timestamp, timestamp) || other.timestamp == timestamp)&&(identical(other.label, label) || other.label == label)&&(identical(other.description, description) || other.description == description));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,eventType,timestamp,label,description);

@override
String toString() {
  return 'AppealEvent(eventType: $eventType, timestamp: $timestamp, label: $label, description: $description)';
}


}

/// @nodoc
abstract mixin class $AppealEventCopyWith<$Res>  {
  factory $AppealEventCopyWith(AppealEvent value, $Res Function(AppealEvent) _then) = _$AppealEventCopyWithImpl;
@useResult
$Res call({
 String eventType, String timestamp, String label, String description
});




}
/// @nodoc
class _$AppealEventCopyWithImpl<$Res>
    implements $AppealEventCopyWith<$Res> {
  _$AppealEventCopyWithImpl(this._self, this._then);

  final AppealEvent _self;
  final $Res Function(AppealEvent) _then;

/// Create a copy of AppealEvent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? eventType = null,Object? timestamp = null,Object? label = null,Object? description = null,}) {
  return _then(_self.copyWith(
eventType: null == eventType ? _self.eventType : eventType // ignore: cast_nullable_to_non_nullable
as String,timestamp: null == timestamp ? _self.timestamp : timestamp // ignore: cast_nullable_to_non_nullable
as String,label: null == label ? _self.label : label // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AppealEvent].
extension AppealEventPatterns on AppealEvent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AppealEvent value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AppealEvent() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AppealEvent value)  $default,){
final _that = this;
switch (_that) {
case _AppealEvent():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AppealEvent value)?  $default,){
final _that = this;
switch (_that) {
case _AppealEvent() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String eventType,  String timestamp,  String label,  String description)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AppealEvent() when $default != null:
return $default(_that.eventType,_that.timestamp,_that.label,_that.description);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String eventType,  String timestamp,  String label,  String description)  $default,) {final _that = this;
switch (_that) {
case _AppealEvent():
return $default(_that.eventType,_that.timestamp,_that.label,_that.description);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String eventType,  String timestamp,  String label,  String description)?  $default,) {final _that = this;
switch (_that) {
case _AppealEvent() when $default != null:
return $default(_that.eventType,_that.timestamp,_that.label,_that.description);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AppealEvent implements AppealEvent {
  const _AppealEvent({this.eventType = '', this.timestamp = '', this.label = '', this.description = ''});
  factory _AppealEvent.fromJson(Map<String, dynamic> json) => _$AppealEventFromJson(json);

@override@JsonKey() final  String eventType;
@override@JsonKey() final  String timestamp;
@override@JsonKey() final  String label;
@override@JsonKey() final  String description;

/// Create a copy of AppealEvent
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AppealEventCopyWith<_AppealEvent> get copyWith => __$AppealEventCopyWithImpl<_AppealEvent>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$AppealEventToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AppealEvent&&(identical(other.eventType, eventType) || other.eventType == eventType)&&(identical(other.timestamp, timestamp) || other.timestamp == timestamp)&&(identical(other.label, label) || other.label == label)&&(identical(other.description, description) || other.description == description));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,eventType,timestamp,label,description);

@override
String toString() {
  return 'AppealEvent(eventType: $eventType, timestamp: $timestamp, label: $label, description: $description)';
}


}

/// @nodoc
abstract mixin class _$AppealEventCopyWith<$Res> implements $AppealEventCopyWith<$Res> {
  factory _$AppealEventCopyWith(_AppealEvent value, $Res Function(_AppealEvent) _then) = __$AppealEventCopyWithImpl;
@override @useResult
$Res call({
 String eventType, String timestamp, String label, String description
});




}
/// @nodoc
class __$AppealEventCopyWithImpl<$Res>
    implements _$AppealEventCopyWith<$Res> {
  __$AppealEventCopyWithImpl(this._self, this._then);

  final _AppealEvent _self;
  final $Res Function(_AppealEvent) _then;

/// Create a copy of AppealEvent
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? eventType = null,Object? timestamp = null,Object? label = null,Object? description = null,}) {
  return _then(_AppealEvent(
eventType: null == eventType ? _self.eventType : eventType // ignore: cast_nullable_to_non_nullable
as String,timestamp: null == timestamp ? _self.timestamp : timestamp // ignore: cast_nullable_to_non_nullable
as String,label: null == label ? _self.label : label // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$PlatformDecision {

 String get outcome; String get refundAmount; String get reason; String get decidedAt;
/// Create a copy of PlatformDecision
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PlatformDecisionCopyWith<PlatformDecision> get copyWith => _$PlatformDecisionCopyWithImpl<PlatformDecision>(this as PlatformDecision, _$identity);

  /// Serializes this PlatformDecision to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is PlatformDecision&&(identical(other.outcome, outcome) || other.outcome == outcome)&&(identical(other.refundAmount, refundAmount) || other.refundAmount == refundAmount)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.decidedAt, decidedAt) || other.decidedAt == decidedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,outcome,refundAmount,reason,decidedAt);

@override
String toString() {
  return 'PlatformDecision(outcome: $outcome, refundAmount: $refundAmount, reason: $reason, decidedAt: $decidedAt)';
}


}

/// @nodoc
abstract mixin class $PlatformDecisionCopyWith<$Res>  {
  factory $PlatformDecisionCopyWith(PlatformDecision value, $Res Function(PlatformDecision) _then) = _$PlatformDecisionCopyWithImpl;
@useResult
$Res call({
 String outcome, String refundAmount, String reason, String decidedAt
});




}
/// @nodoc
class _$PlatformDecisionCopyWithImpl<$Res>
    implements $PlatformDecisionCopyWith<$Res> {
  _$PlatformDecisionCopyWithImpl(this._self, this._then);

  final PlatformDecision _self;
  final $Res Function(PlatformDecision) _then;

/// Create a copy of PlatformDecision
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? outcome = null,Object? refundAmount = null,Object? reason = null,Object? decidedAt = null,}) {
  return _then(_self.copyWith(
outcome: null == outcome ? _self.outcome : outcome // ignore: cast_nullable_to_non_nullable
as String,refundAmount: null == refundAmount ? _self.refundAmount : refundAmount // ignore: cast_nullable_to_non_nullable
as String,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,decidedAt: null == decidedAt ? _self.decidedAt : decidedAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [PlatformDecision].
extension PlatformDecisionPatterns on PlatformDecision {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _PlatformDecision value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _PlatformDecision() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _PlatformDecision value)  $default,){
final _that = this;
switch (_that) {
case _PlatformDecision():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _PlatformDecision value)?  $default,){
final _that = this;
switch (_that) {
case _PlatformDecision() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String outcome,  String refundAmount,  String reason,  String decidedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _PlatformDecision() when $default != null:
return $default(_that.outcome,_that.refundAmount,_that.reason,_that.decidedAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String outcome,  String refundAmount,  String reason,  String decidedAt)  $default,) {final _that = this;
switch (_that) {
case _PlatformDecision():
return $default(_that.outcome,_that.refundAmount,_that.reason,_that.decidedAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String outcome,  String refundAmount,  String reason,  String decidedAt)?  $default,) {final _that = this;
switch (_that) {
case _PlatformDecision() when $default != null:
return $default(_that.outcome,_that.refundAmount,_that.reason,_that.decidedAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _PlatformDecision implements PlatformDecision {
  const _PlatformDecision({this.outcome = '', this.refundAmount = '', this.reason = '', this.decidedAt = ''});
  factory _PlatformDecision.fromJson(Map<String, dynamic> json) => _$PlatformDecisionFromJson(json);

@override@JsonKey() final  String outcome;
@override@JsonKey() final  String refundAmount;
@override@JsonKey() final  String reason;
@override@JsonKey() final  String decidedAt;

/// Create a copy of PlatformDecision
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PlatformDecisionCopyWith<_PlatformDecision> get copyWith => __$PlatformDecisionCopyWithImpl<_PlatformDecision>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$PlatformDecisionToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _PlatformDecision&&(identical(other.outcome, outcome) || other.outcome == outcome)&&(identical(other.refundAmount, refundAmount) || other.refundAmount == refundAmount)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.decidedAt, decidedAt) || other.decidedAt == decidedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,outcome,refundAmount,reason,decidedAt);

@override
String toString() {
  return 'PlatformDecision(outcome: $outcome, refundAmount: $refundAmount, reason: $reason, decidedAt: $decidedAt)';
}


}

/// @nodoc
abstract mixin class _$PlatformDecisionCopyWith<$Res> implements $PlatformDecisionCopyWith<$Res> {
  factory _$PlatformDecisionCopyWith(_PlatformDecision value, $Res Function(_PlatformDecision) _then) = __$PlatformDecisionCopyWithImpl;
@override @useResult
$Res call({
 String outcome, String refundAmount, String reason, String decidedAt
});




}
/// @nodoc
class __$PlatformDecisionCopyWithImpl<$Res>
    implements _$PlatformDecisionCopyWith<$Res> {
  __$PlatformDecisionCopyWithImpl(this._self, this._then);

  final _PlatformDecision _self;
  final $Res Function(_PlatformDecision) _then;

/// Create a copy of PlatformDecision
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? outcome = null,Object? refundAmount = null,Object? reason = null,Object? decidedAt = null,}) {
  return _then(_PlatformDecision(
outcome: null == outcome ? _self.outcome : outcome // ignore: cast_nullable_to_non_nullable
as String,refundAmount: null == refundAmount ? _self.refundAmount : refundAmount // ignore: cast_nullable_to_non_nullable
as String,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,decidedAt: null == decidedAt ? _self.decidedAt : decidedAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$ArbitrationData {

 String get state; List<AppealEvent> get timeline; PlatformDecision? get decision;
/// Create a copy of ArbitrationData
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ArbitrationDataCopyWith<ArbitrationData> get copyWith => _$ArbitrationDataCopyWithImpl<ArbitrationData>(this as ArbitrationData, _$identity);

  /// Serializes this ArbitrationData to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ArbitrationData&&(identical(other.state, state) || other.state == state)&&const DeepCollectionEquality().equals(other.timeline, timeline)&&(identical(other.decision, decision) || other.decision == decision));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,state,const DeepCollectionEquality().hash(timeline),decision);

@override
String toString() {
  return 'ArbitrationData(state: $state, timeline: $timeline, decision: $decision)';
}


}

/// @nodoc
abstract mixin class $ArbitrationDataCopyWith<$Res>  {
  factory $ArbitrationDataCopyWith(ArbitrationData value, $Res Function(ArbitrationData) _then) = _$ArbitrationDataCopyWithImpl;
@useResult
$Res call({
 String state, List<AppealEvent> timeline, PlatformDecision? decision
});


$PlatformDecisionCopyWith<$Res>? get decision;

}
/// @nodoc
class _$ArbitrationDataCopyWithImpl<$Res>
    implements $ArbitrationDataCopyWith<$Res> {
  _$ArbitrationDataCopyWithImpl(this._self, this._then);

  final ArbitrationData _self;
  final $Res Function(ArbitrationData) _then;

/// Create a copy of ArbitrationData
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? state = null,Object? timeline = null,Object? decision = freezed,}) {
  return _then(_self.copyWith(
state: null == state ? _self.state : state // ignore: cast_nullable_to_non_nullable
as String,timeline: null == timeline ? _self.timeline : timeline // ignore: cast_nullable_to_non_nullable
as List<AppealEvent>,decision: freezed == decision ? _self.decision : decision // ignore: cast_nullable_to_non_nullable
as PlatformDecision?,
  ));
}
/// Create a copy of ArbitrationData
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$PlatformDecisionCopyWith<$Res>? get decision {
    if (_self.decision == null) {
    return null;
  }

  return $PlatformDecisionCopyWith<$Res>(_self.decision!, (value) {
    return _then(_self.copyWith(decision: value));
  });
}
}


/// Adds pattern-matching-related methods to [ArbitrationData].
extension ArbitrationDataPatterns on ArbitrationData {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ArbitrationData value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ArbitrationData() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ArbitrationData value)  $default,){
final _that = this;
switch (_that) {
case _ArbitrationData():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ArbitrationData value)?  $default,){
final _that = this;
switch (_that) {
case _ArbitrationData() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String state,  List<AppealEvent> timeline,  PlatformDecision? decision)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ArbitrationData() when $default != null:
return $default(_that.state,_that.timeline,_that.decision);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String state,  List<AppealEvent> timeline,  PlatformDecision? decision)  $default,) {final _that = this;
switch (_that) {
case _ArbitrationData():
return $default(_that.state,_that.timeline,_that.decision);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String state,  List<AppealEvent> timeline,  PlatformDecision? decision)?  $default,) {final _that = this;
switch (_that) {
case _ArbitrationData() when $default != null:
return $default(_that.state,_that.timeline,_that.decision);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _ArbitrationData implements ArbitrationData {
  const _ArbitrationData({this.state = '', final  List<AppealEvent> timeline = const [], this.decision}): _timeline = timeline;
  factory _ArbitrationData.fromJson(Map<String, dynamic> json) => _$ArbitrationDataFromJson(json);

@override@JsonKey() final  String state;
 final  List<AppealEvent> _timeline;
@override@JsonKey() List<AppealEvent> get timeline {
  if (_timeline is EqualUnmodifiableListView) return _timeline;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_timeline);
}

@override final  PlatformDecision? decision;

/// Create a copy of ArbitrationData
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ArbitrationDataCopyWith<_ArbitrationData> get copyWith => __$ArbitrationDataCopyWithImpl<_ArbitrationData>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$ArbitrationDataToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ArbitrationData&&(identical(other.state, state) || other.state == state)&&const DeepCollectionEquality().equals(other._timeline, _timeline)&&(identical(other.decision, decision) || other.decision == decision));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,state,const DeepCollectionEquality().hash(_timeline),decision);

@override
String toString() {
  return 'ArbitrationData(state: $state, timeline: $timeline, decision: $decision)';
}


}

/// @nodoc
abstract mixin class _$ArbitrationDataCopyWith<$Res> implements $ArbitrationDataCopyWith<$Res> {
  factory _$ArbitrationDataCopyWith(_ArbitrationData value, $Res Function(_ArbitrationData) _then) = __$ArbitrationDataCopyWithImpl;
@override @useResult
$Res call({
 String state, List<AppealEvent> timeline, PlatformDecision? decision
});


@override $PlatformDecisionCopyWith<$Res>? get decision;

}
/// @nodoc
class __$ArbitrationDataCopyWithImpl<$Res>
    implements _$ArbitrationDataCopyWith<$Res> {
  __$ArbitrationDataCopyWithImpl(this._self, this._then);

  final _ArbitrationData _self;
  final $Res Function(_ArbitrationData) _then;

/// Create a copy of ArbitrationData
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? state = null,Object? timeline = null,Object? decision = freezed,}) {
  return _then(_ArbitrationData(
state: null == state ? _self.state : state // ignore: cast_nullable_to_non_nullable
as String,timeline: null == timeline ? _self._timeline : timeline // ignore: cast_nullable_to_non_nullable
as List<AppealEvent>,decision: freezed == decision ? _self.decision : decision // ignore: cast_nullable_to_non_nullable
as PlatformDecision?,
  ));
}

/// Create a copy of ArbitrationData
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$PlatformDecisionCopyWith<$Res>? get decision {
    if (_self.decision == null) {
    return null;
  }

  return $PlatformDecisionCopyWith<$Res>(_self.decision!, (value) {
    return _then(_self.copyWith(decision: value));
  });
}
}

// dart format on
