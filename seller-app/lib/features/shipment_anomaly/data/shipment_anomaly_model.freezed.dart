// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'shipment_anomaly_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$ShipmentAnomaly {

 String get shipmentId; String get orderNo; String get anomalyType; String get anomalyTypeLabel; String get severity; String get severityLabel; String get shippingMethod; String get shipmentStatus; String get detectedAt; String get ackAction;// Backend Gateway 回的欄位是 `actedAt`（非 ackedAt），對齊 gateway/internal/handler/shipment_anomaly.go
 String get actedAt;
/// Create a copy of ShipmentAnomaly
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ShipmentAnomalyCopyWith<ShipmentAnomaly> get copyWith => _$ShipmentAnomalyCopyWithImpl<ShipmentAnomaly>(this as ShipmentAnomaly, _$identity);

  /// Serializes this ShipmentAnomaly to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ShipmentAnomaly&&(identical(other.shipmentId, shipmentId) || other.shipmentId == shipmentId)&&(identical(other.orderNo, orderNo) || other.orderNo == orderNo)&&(identical(other.anomalyType, anomalyType) || other.anomalyType == anomalyType)&&(identical(other.anomalyTypeLabel, anomalyTypeLabel) || other.anomalyTypeLabel == anomalyTypeLabel)&&(identical(other.severity, severity) || other.severity == severity)&&(identical(other.severityLabel, severityLabel) || other.severityLabel == severityLabel)&&(identical(other.shippingMethod, shippingMethod) || other.shippingMethod == shippingMethod)&&(identical(other.shipmentStatus, shipmentStatus) || other.shipmentStatus == shipmentStatus)&&(identical(other.detectedAt, detectedAt) || other.detectedAt == detectedAt)&&(identical(other.ackAction, ackAction) || other.ackAction == ackAction)&&(identical(other.actedAt, actedAt) || other.actedAt == actedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,shipmentId,orderNo,anomalyType,anomalyTypeLabel,severity,severityLabel,shippingMethod,shipmentStatus,detectedAt,ackAction,actedAt);

@override
String toString() {
  return 'ShipmentAnomaly(shipmentId: $shipmentId, orderNo: $orderNo, anomalyType: $anomalyType, anomalyTypeLabel: $anomalyTypeLabel, severity: $severity, severityLabel: $severityLabel, shippingMethod: $shippingMethod, shipmentStatus: $shipmentStatus, detectedAt: $detectedAt, ackAction: $ackAction, actedAt: $actedAt)';
}


}

/// @nodoc
abstract mixin class $ShipmentAnomalyCopyWith<$Res>  {
  factory $ShipmentAnomalyCopyWith(ShipmentAnomaly value, $Res Function(ShipmentAnomaly) _then) = _$ShipmentAnomalyCopyWithImpl;
@useResult
$Res call({
 String shipmentId, String orderNo, String anomalyType, String anomalyTypeLabel, String severity, String severityLabel, String shippingMethod, String shipmentStatus, String detectedAt, String ackAction, String actedAt
});




}
/// @nodoc
class _$ShipmentAnomalyCopyWithImpl<$Res>
    implements $ShipmentAnomalyCopyWith<$Res> {
  _$ShipmentAnomalyCopyWithImpl(this._self, this._then);

  final ShipmentAnomaly _self;
  final $Res Function(ShipmentAnomaly) _then;

/// Create a copy of ShipmentAnomaly
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? shipmentId = null,Object? orderNo = null,Object? anomalyType = null,Object? anomalyTypeLabel = null,Object? severity = null,Object? severityLabel = null,Object? shippingMethod = null,Object? shipmentStatus = null,Object? detectedAt = null,Object? ackAction = null,Object? actedAt = null,}) {
  return _then(_self.copyWith(
shipmentId: null == shipmentId ? _self.shipmentId : shipmentId // ignore: cast_nullable_to_non_nullable
as String,orderNo: null == orderNo ? _self.orderNo : orderNo // ignore: cast_nullable_to_non_nullable
as String,anomalyType: null == anomalyType ? _self.anomalyType : anomalyType // ignore: cast_nullable_to_non_nullable
as String,anomalyTypeLabel: null == anomalyTypeLabel ? _self.anomalyTypeLabel : anomalyTypeLabel // ignore: cast_nullable_to_non_nullable
as String,severity: null == severity ? _self.severity : severity // ignore: cast_nullable_to_non_nullable
as String,severityLabel: null == severityLabel ? _self.severityLabel : severityLabel // ignore: cast_nullable_to_non_nullable
as String,shippingMethod: null == shippingMethod ? _self.shippingMethod : shippingMethod // ignore: cast_nullable_to_non_nullable
as String,shipmentStatus: null == shipmentStatus ? _self.shipmentStatus : shipmentStatus // ignore: cast_nullable_to_non_nullable
as String,detectedAt: null == detectedAt ? _self.detectedAt : detectedAt // ignore: cast_nullable_to_non_nullable
as String,ackAction: null == ackAction ? _self.ackAction : ackAction // ignore: cast_nullable_to_non_nullable
as String,actedAt: null == actedAt ? _self.actedAt : actedAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [ShipmentAnomaly].
extension ShipmentAnomalyPatterns on ShipmentAnomaly {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ShipmentAnomaly value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ShipmentAnomaly() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ShipmentAnomaly value)  $default,){
final _that = this;
switch (_that) {
case _ShipmentAnomaly():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ShipmentAnomaly value)?  $default,){
final _that = this;
switch (_that) {
case _ShipmentAnomaly() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String shipmentId,  String orderNo,  String anomalyType,  String anomalyTypeLabel,  String severity,  String severityLabel,  String shippingMethod,  String shipmentStatus,  String detectedAt,  String ackAction,  String actedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ShipmentAnomaly() when $default != null:
return $default(_that.shipmentId,_that.orderNo,_that.anomalyType,_that.anomalyTypeLabel,_that.severity,_that.severityLabel,_that.shippingMethod,_that.shipmentStatus,_that.detectedAt,_that.ackAction,_that.actedAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String shipmentId,  String orderNo,  String anomalyType,  String anomalyTypeLabel,  String severity,  String severityLabel,  String shippingMethod,  String shipmentStatus,  String detectedAt,  String ackAction,  String actedAt)  $default,) {final _that = this;
switch (_that) {
case _ShipmentAnomaly():
return $default(_that.shipmentId,_that.orderNo,_that.anomalyType,_that.anomalyTypeLabel,_that.severity,_that.severityLabel,_that.shippingMethod,_that.shipmentStatus,_that.detectedAt,_that.ackAction,_that.actedAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String shipmentId,  String orderNo,  String anomalyType,  String anomalyTypeLabel,  String severity,  String severityLabel,  String shippingMethod,  String shipmentStatus,  String detectedAt,  String ackAction,  String actedAt)?  $default,) {final _that = this;
switch (_that) {
case _ShipmentAnomaly() when $default != null:
return $default(_that.shipmentId,_that.orderNo,_that.anomalyType,_that.anomalyTypeLabel,_that.severity,_that.severityLabel,_that.shippingMethod,_that.shipmentStatus,_that.detectedAt,_that.ackAction,_that.actedAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _ShipmentAnomaly implements ShipmentAnomaly {
  const _ShipmentAnomaly({this.shipmentId = '', this.orderNo = '', this.anomalyType = '', this.anomalyTypeLabel = '', this.severity = '', this.severityLabel = '', this.shippingMethod = '', this.shipmentStatus = '', this.detectedAt = '', this.ackAction = '', this.actedAt = ''});
  factory _ShipmentAnomaly.fromJson(Map<String, dynamic> json) => _$ShipmentAnomalyFromJson(json);

@override@JsonKey() final  String shipmentId;
@override@JsonKey() final  String orderNo;
@override@JsonKey() final  String anomalyType;
@override@JsonKey() final  String anomalyTypeLabel;
@override@JsonKey() final  String severity;
@override@JsonKey() final  String severityLabel;
@override@JsonKey() final  String shippingMethod;
@override@JsonKey() final  String shipmentStatus;
@override@JsonKey() final  String detectedAt;
@override@JsonKey() final  String ackAction;
// Backend Gateway 回的欄位是 `actedAt`（非 ackedAt），對齊 gateway/internal/handler/shipment_anomaly.go
@override@JsonKey() final  String actedAt;

/// Create a copy of ShipmentAnomaly
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ShipmentAnomalyCopyWith<_ShipmentAnomaly> get copyWith => __$ShipmentAnomalyCopyWithImpl<_ShipmentAnomaly>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$ShipmentAnomalyToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ShipmentAnomaly&&(identical(other.shipmentId, shipmentId) || other.shipmentId == shipmentId)&&(identical(other.orderNo, orderNo) || other.orderNo == orderNo)&&(identical(other.anomalyType, anomalyType) || other.anomalyType == anomalyType)&&(identical(other.anomalyTypeLabel, anomalyTypeLabel) || other.anomalyTypeLabel == anomalyTypeLabel)&&(identical(other.severity, severity) || other.severity == severity)&&(identical(other.severityLabel, severityLabel) || other.severityLabel == severityLabel)&&(identical(other.shippingMethod, shippingMethod) || other.shippingMethod == shippingMethod)&&(identical(other.shipmentStatus, shipmentStatus) || other.shipmentStatus == shipmentStatus)&&(identical(other.detectedAt, detectedAt) || other.detectedAt == detectedAt)&&(identical(other.ackAction, ackAction) || other.ackAction == ackAction)&&(identical(other.actedAt, actedAt) || other.actedAt == actedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,shipmentId,orderNo,anomalyType,anomalyTypeLabel,severity,severityLabel,shippingMethod,shipmentStatus,detectedAt,ackAction,actedAt);

@override
String toString() {
  return 'ShipmentAnomaly(shipmentId: $shipmentId, orderNo: $orderNo, anomalyType: $anomalyType, anomalyTypeLabel: $anomalyTypeLabel, severity: $severity, severityLabel: $severityLabel, shippingMethod: $shippingMethod, shipmentStatus: $shipmentStatus, detectedAt: $detectedAt, ackAction: $ackAction, actedAt: $actedAt)';
}


}

/// @nodoc
abstract mixin class _$ShipmentAnomalyCopyWith<$Res> implements $ShipmentAnomalyCopyWith<$Res> {
  factory _$ShipmentAnomalyCopyWith(_ShipmentAnomaly value, $Res Function(_ShipmentAnomaly) _then) = __$ShipmentAnomalyCopyWithImpl;
@override @useResult
$Res call({
 String shipmentId, String orderNo, String anomalyType, String anomalyTypeLabel, String severity, String severityLabel, String shippingMethod, String shipmentStatus, String detectedAt, String ackAction, String actedAt
});




}
/// @nodoc
class __$ShipmentAnomalyCopyWithImpl<$Res>
    implements _$ShipmentAnomalyCopyWith<$Res> {
  __$ShipmentAnomalyCopyWithImpl(this._self, this._then);

  final _ShipmentAnomaly _self;
  final $Res Function(_ShipmentAnomaly) _then;

/// Create a copy of ShipmentAnomaly
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? shipmentId = null,Object? orderNo = null,Object? anomalyType = null,Object? anomalyTypeLabel = null,Object? severity = null,Object? severityLabel = null,Object? shippingMethod = null,Object? shipmentStatus = null,Object? detectedAt = null,Object? ackAction = null,Object? actedAt = null,}) {
  return _then(_ShipmentAnomaly(
shipmentId: null == shipmentId ? _self.shipmentId : shipmentId // ignore: cast_nullable_to_non_nullable
as String,orderNo: null == orderNo ? _self.orderNo : orderNo // ignore: cast_nullable_to_non_nullable
as String,anomalyType: null == anomalyType ? _self.anomalyType : anomalyType // ignore: cast_nullable_to_non_nullable
as String,anomalyTypeLabel: null == anomalyTypeLabel ? _self.anomalyTypeLabel : anomalyTypeLabel // ignore: cast_nullable_to_non_nullable
as String,severity: null == severity ? _self.severity : severity // ignore: cast_nullable_to_non_nullable
as String,severityLabel: null == severityLabel ? _self.severityLabel : severityLabel // ignore: cast_nullable_to_non_nullable
as String,shippingMethod: null == shippingMethod ? _self.shippingMethod : shippingMethod // ignore: cast_nullable_to_non_nullable
as String,shipmentStatus: null == shipmentStatus ? _self.shipmentStatus : shipmentStatus // ignore: cast_nullable_to_non_nullable
as String,detectedAt: null == detectedAt ? _self.detectedAt : detectedAt // ignore: cast_nullable_to_non_nullable
as String,ackAction: null == ackAction ? _self.ackAction : ackAction // ignore: cast_nullable_to_non_nullable
as String,actedAt: null == actedAt ? _self.actedAt : actedAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$ShipmentAnomalyListResult {

 List<ShipmentAnomaly> get records;@JsonKey(fromJson: _parseInt) int get total;
/// Create a copy of ShipmentAnomalyListResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ShipmentAnomalyListResultCopyWith<ShipmentAnomalyListResult> get copyWith => _$ShipmentAnomalyListResultCopyWithImpl<ShipmentAnomalyListResult>(this as ShipmentAnomalyListResult, _$identity);

  /// Serializes this ShipmentAnomalyListResult to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ShipmentAnomalyListResult&&const DeepCollectionEquality().equals(other.records, records)&&(identical(other.total, total) || other.total == total));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(records),total);

@override
String toString() {
  return 'ShipmentAnomalyListResult(records: $records, total: $total)';
}


}

/// @nodoc
abstract mixin class $ShipmentAnomalyListResultCopyWith<$Res>  {
  factory $ShipmentAnomalyListResultCopyWith(ShipmentAnomalyListResult value, $Res Function(ShipmentAnomalyListResult) _then) = _$ShipmentAnomalyListResultCopyWithImpl;
@useResult
$Res call({
 List<ShipmentAnomaly> records,@JsonKey(fromJson: _parseInt) int total
});




}
/// @nodoc
class _$ShipmentAnomalyListResultCopyWithImpl<$Res>
    implements $ShipmentAnomalyListResultCopyWith<$Res> {
  _$ShipmentAnomalyListResultCopyWithImpl(this._self, this._then);

  final ShipmentAnomalyListResult _self;
  final $Res Function(ShipmentAnomalyListResult) _then;

/// Create a copy of ShipmentAnomalyListResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? records = null,Object? total = null,}) {
  return _then(_self.copyWith(
records: null == records ? _self.records : records // ignore: cast_nullable_to_non_nullable
as List<ShipmentAnomaly>,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [ShipmentAnomalyListResult].
extension ShipmentAnomalyListResultPatterns on ShipmentAnomalyListResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ShipmentAnomalyListResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ShipmentAnomalyListResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ShipmentAnomalyListResult value)  $default,){
final _that = this;
switch (_that) {
case _ShipmentAnomalyListResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ShipmentAnomalyListResult value)?  $default,){
final _that = this;
switch (_that) {
case _ShipmentAnomalyListResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<ShipmentAnomaly> records, @JsonKey(fromJson: _parseInt)  int total)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ShipmentAnomalyListResult() when $default != null:
return $default(_that.records,_that.total);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<ShipmentAnomaly> records, @JsonKey(fromJson: _parseInt)  int total)  $default,) {final _that = this;
switch (_that) {
case _ShipmentAnomalyListResult():
return $default(_that.records,_that.total);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<ShipmentAnomaly> records, @JsonKey(fromJson: _parseInt)  int total)?  $default,) {final _that = this;
switch (_that) {
case _ShipmentAnomalyListResult() when $default != null:
return $default(_that.records,_that.total);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _ShipmentAnomalyListResult implements ShipmentAnomalyListResult {
  const _ShipmentAnomalyListResult({final  List<ShipmentAnomaly> records = const [], @JsonKey(fromJson: _parseInt) this.total = 0}): _records = records;
  factory _ShipmentAnomalyListResult.fromJson(Map<String, dynamic> json) => _$ShipmentAnomalyListResultFromJson(json);

 final  List<ShipmentAnomaly> _records;
@override@JsonKey() List<ShipmentAnomaly> get records {
  if (_records is EqualUnmodifiableListView) return _records;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_records);
}

@override@JsonKey(fromJson: _parseInt) final  int total;

/// Create a copy of ShipmentAnomalyListResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ShipmentAnomalyListResultCopyWith<_ShipmentAnomalyListResult> get copyWith => __$ShipmentAnomalyListResultCopyWithImpl<_ShipmentAnomalyListResult>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$ShipmentAnomalyListResultToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ShipmentAnomalyListResult&&const DeepCollectionEquality().equals(other._records, _records)&&(identical(other.total, total) || other.total == total));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_records),total);

@override
String toString() {
  return 'ShipmentAnomalyListResult(records: $records, total: $total)';
}


}

/// @nodoc
abstract mixin class _$ShipmentAnomalyListResultCopyWith<$Res> implements $ShipmentAnomalyListResultCopyWith<$Res> {
  factory _$ShipmentAnomalyListResultCopyWith(_ShipmentAnomalyListResult value, $Res Function(_ShipmentAnomalyListResult) _then) = __$ShipmentAnomalyListResultCopyWithImpl;
@override @useResult
$Res call({
 List<ShipmentAnomaly> records,@JsonKey(fromJson: _parseInt) int total
});




}
/// @nodoc
class __$ShipmentAnomalyListResultCopyWithImpl<$Res>
    implements _$ShipmentAnomalyListResultCopyWith<$Res> {
  __$ShipmentAnomalyListResultCopyWithImpl(this._self, this._then);

  final _ShipmentAnomalyListResult _self;
  final $Res Function(_ShipmentAnomalyListResult) _then;

/// Create a copy of ShipmentAnomalyListResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? records = null,Object? total = null,}) {
  return _then(_ShipmentAnomalyListResult(
records: null == records ? _self._records : records // ignore: cast_nullable_to_non_nullable
as List<ShipmentAnomaly>,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}


/// @nodoc
mixin _$ShipmentAnomalyStats {

@JsonKey(fromJson: _parseInt) int get total;// Backend Gateway 實際回的欄位是 highSeverityCount / mediumSeverityCount / lowSeverityCount
// (gateway/internal/handler/shipment_anomaly.go DTO + shipping-service proto)
@JsonKey(name: 'highSeverityCount', fromJson: _parseInt) int get highCount;@JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt) int get mediumCount;@JsonKey(name: 'lowSeverityCount', fromJson: _parseInt) int get lowCount;
/// Create a copy of ShipmentAnomalyStats
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ShipmentAnomalyStatsCopyWith<ShipmentAnomalyStats> get copyWith => _$ShipmentAnomalyStatsCopyWithImpl<ShipmentAnomalyStats>(this as ShipmentAnomalyStats, _$identity);

  /// Serializes this ShipmentAnomalyStats to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ShipmentAnomalyStats&&(identical(other.total, total) || other.total == total)&&(identical(other.highCount, highCount) || other.highCount == highCount)&&(identical(other.mediumCount, mediumCount) || other.mediumCount == mediumCount)&&(identical(other.lowCount, lowCount) || other.lowCount == lowCount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,total,highCount,mediumCount,lowCount);

@override
String toString() {
  return 'ShipmentAnomalyStats(total: $total, highCount: $highCount, mediumCount: $mediumCount, lowCount: $lowCount)';
}


}

/// @nodoc
abstract mixin class $ShipmentAnomalyStatsCopyWith<$Res>  {
  factory $ShipmentAnomalyStatsCopyWith(ShipmentAnomalyStats value, $Res Function(ShipmentAnomalyStats) _then) = _$ShipmentAnomalyStatsCopyWithImpl;
@useResult
$Res call({
@JsonKey(fromJson: _parseInt) int total,@JsonKey(name: 'highSeverityCount', fromJson: _parseInt) int highCount,@JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt) int mediumCount,@JsonKey(name: 'lowSeverityCount', fromJson: _parseInt) int lowCount
});




}
/// @nodoc
class _$ShipmentAnomalyStatsCopyWithImpl<$Res>
    implements $ShipmentAnomalyStatsCopyWith<$Res> {
  _$ShipmentAnomalyStatsCopyWithImpl(this._self, this._then);

  final ShipmentAnomalyStats _self;
  final $Res Function(ShipmentAnomalyStats) _then;

/// Create a copy of ShipmentAnomalyStats
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? total = null,Object? highCount = null,Object? mediumCount = null,Object? lowCount = null,}) {
  return _then(_self.copyWith(
total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,highCount: null == highCount ? _self.highCount : highCount // ignore: cast_nullable_to_non_nullable
as int,mediumCount: null == mediumCount ? _self.mediumCount : mediumCount // ignore: cast_nullable_to_non_nullable
as int,lowCount: null == lowCount ? _self.lowCount : lowCount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [ShipmentAnomalyStats].
extension ShipmentAnomalyStatsPatterns on ShipmentAnomalyStats {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ShipmentAnomalyStats value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ShipmentAnomalyStats() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ShipmentAnomalyStats value)  $default,){
final _that = this;
switch (_that) {
case _ShipmentAnomalyStats():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ShipmentAnomalyStats value)?  $default,){
final _that = this;
switch (_that) {
case _ShipmentAnomalyStats() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(fromJson: _parseInt)  int total, @JsonKey(name: 'highSeverityCount', fromJson: _parseInt)  int highCount, @JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt)  int mediumCount, @JsonKey(name: 'lowSeverityCount', fromJson: _parseInt)  int lowCount)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ShipmentAnomalyStats() when $default != null:
return $default(_that.total,_that.highCount,_that.mediumCount,_that.lowCount);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(fromJson: _parseInt)  int total, @JsonKey(name: 'highSeverityCount', fromJson: _parseInt)  int highCount, @JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt)  int mediumCount, @JsonKey(name: 'lowSeverityCount', fromJson: _parseInt)  int lowCount)  $default,) {final _that = this;
switch (_that) {
case _ShipmentAnomalyStats():
return $default(_that.total,_that.highCount,_that.mediumCount,_that.lowCount);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(fromJson: _parseInt)  int total, @JsonKey(name: 'highSeverityCount', fromJson: _parseInt)  int highCount, @JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt)  int mediumCount, @JsonKey(name: 'lowSeverityCount', fromJson: _parseInt)  int lowCount)?  $default,) {final _that = this;
switch (_that) {
case _ShipmentAnomalyStats() when $default != null:
return $default(_that.total,_that.highCount,_that.mediumCount,_that.lowCount);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _ShipmentAnomalyStats implements ShipmentAnomalyStats {
  const _ShipmentAnomalyStats({@JsonKey(fromJson: _parseInt) this.total = 0, @JsonKey(name: 'highSeverityCount', fromJson: _parseInt) this.highCount = 0, @JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt) this.mediumCount = 0, @JsonKey(name: 'lowSeverityCount', fromJson: _parseInt) this.lowCount = 0});
  factory _ShipmentAnomalyStats.fromJson(Map<String, dynamic> json) => _$ShipmentAnomalyStatsFromJson(json);

@override@JsonKey(fromJson: _parseInt) final  int total;
// Backend Gateway 實際回的欄位是 highSeverityCount / mediumSeverityCount / lowSeverityCount
// (gateway/internal/handler/shipment_anomaly.go DTO + shipping-service proto)
@override@JsonKey(name: 'highSeverityCount', fromJson: _parseInt) final  int highCount;
@override@JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt) final  int mediumCount;
@override@JsonKey(name: 'lowSeverityCount', fromJson: _parseInt) final  int lowCount;

/// Create a copy of ShipmentAnomalyStats
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ShipmentAnomalyStatsCopyWith<_ShipmentAnomalyStats> get copyWith => __$ShipmentAnomalyStatsCopyWithImpl<_ShipmentAnomalyStats>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$ShipmentAnomalyStatsToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ShipmentAnomalyStats&&(identical(other.total, total) || other.total == total)&&(identical(other.highCount, highCount) || other.highCount == highCount)&&(identical(other.mediumCount, mediumCount) || other.mediumCount == mediumCount)&&(identical(other.lowCount, lowCount) || other.lowCount == lowCount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,total,highCount,mediumCount,lowCount);

@override
String toString() {
  return 'ShipmentAnomalyStats(total: $total, highCount: $highCount, mediumCount: $mediumCount, lowCount: $lowCount)';
}


}

/// @nodoc
abstract mixin class _$ShipmentAnomalyStatsCopyWith<$Res> implements $ShipmentAnomalyStatsCopyWith<$Res> {
  factory _$ShipmentAnomalyStatsCopyWith(_ShipmentAnomalyStats value, $Res Function(_ShipmentAnomalyStats) _then) = __$ShipmentAnomalyStatsCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(fromJson: _parseInt) int total,@JsonKey(name: 'highSeverityCount', fromJson: _parseInt) int highCount,@JsonKey(name: 'mediumSeverityCount', fromJson: _parseInt) int mediumCount,@JsonKey(name: 'lowSeverityCount', fromJson: _parseInt) int lowCount
});




}
/// @nodoc
class __$ShipmentAnomalyStatsCopyWithImpl<$Res>
    implements _$ShipmentAnomalyStatsCopyWith<$Res> {
  __$ShipmentAnomalyStatsCopyWithImpl(this._self, this._then);

  final _ShipmentAnomalyStats _self;
  final $Res Function(_ShipmentAnomalyStats) _then;

/// Create a copy of ShipmentAnomalyStats
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? total = null,Object? highCount = null,Object? mediumCount = null,Object? lowCount = null,}) {
  return _then(_ShipmentAnomalyStats(
total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,highCount: null == highCount ? _self.highCount : highCount // ignore: cast_nullable_to_non_nullable
as int,mediumCount: null == mediumCount ? _self.mediumCount : mediumCount // ignore: cast_nullable_to_non_nullable
as int,lowCount: null == lowCount ? _self.lowCount : lowCount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
