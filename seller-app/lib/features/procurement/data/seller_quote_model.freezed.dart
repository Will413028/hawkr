// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'seller_quote_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$OfferedQuality {

 String get grade; String get inspectionNotes;
/// Create a copy of OfferedQuality
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$OfferedQualityCopyWith<OfferedQuality> get copyWith => _$OfferedQualityCopyWithImpl<OfferedQuality>(this as OfferedQuality, _$identity);

  /// Serializes this OfferedQuality to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is OfferedQuality&&(identical(other.grade, grade) || other.grade == grade)&&(identical(other.inspectionNotes, inspectionNotes) || other.inspectionNotes == inspectionNotes));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,grade,inspectionNotes);

@override
String toString() {
  return 'OfferedQuality(grade: $grade, inspectionNotes: $inspectionNotes)';
}


}

/// @nodoc
abstract mixin class $OfferedQualityCopyWith<$Res>  {
  factory $OfferedQualityCopyWith(OfferedQuality value, $Res Function(OfferedQuality) _then) = _$OfferedQualityCopyWithImpl;
@useResult
$Res call({
 String grade, String inspectionNotes
});




}
/// @nodoc
class _$OfferedQualityCopyWithImpl<$Res>
    implements $OfferedQualityCopyWith<$Res> {
  _$OfferedQualityCopyWithImpl(this._self, this._then);

  final OfferedQuality _self;
  final $Res Function(OfferedQuality) _then;

/// Create a copy of OfferedQuality
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? grade = null,Object? inspectionNotes = null,}) {
  return _then(_self.copyWith(
grade: null == grade ? _self.grade : grade // ignore: cast_nullable_to_non_nullable
as String,inspectionNotes: null == inspectionNotes ? _self.inspectionNotes : inspectionNotes // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [OfferedQuality].
extension OfferedQualityPatterns on OfferedQuality {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _OfferedQuality value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _OfferedQuality() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _OfferedQuality value)  $default,){
final _that = this;
switch (_that) {
case _OfferedQuality():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _OfferedQuality value)?  $default,){
final _that = this;
switch (_that) {
case _OfferedQuality() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String grade,  String inspectionNotes)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _OfferedQuality() when $default != null:
return $default(_that.grade,_that.inspectionNotes);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String grade,  String inspectionNotes)  $default,) {final _that = this;
switch (_that) {
case _OfferedQuality():
return $default(_that.grade,_that.inspectionNotes);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String grade,  String inspectionNotes)?  $default,) {final _that = this;
switch (_that) {
case _OfferedQuality() when $default != null:
return $default(_that.grade,_that.inspectionNotes);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _OfferedQuality implements OfferedQuality {
  const _OfferedQuality({this.grade = '', this.inspectionNotes = ''});
  factory _OfferedQuality.fromJson(Map<String, dynamic> json) => _$OfferedQualityFromJson(json);

@override@JsonKey() final  String grade;
@override@JsonKey() final  String inspectionNotes;

/// Create a copy of OfferedQuality
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$OfferedQualityCopyWith<_OfferedQuality> get copyWith => __$OfferedQualityCopyWithImpl<_OfferedQuality>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$OfferedQualityToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _OfferedQuality&&(identical(other.grade, grade) || other.grade == grade)&&(identical(other.inspectionNotes, inspectionNotes) || other.inspectionNotes == inspectionNotes));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,grade,inspectionNotes);

@override
String toString() {
  return 'OfferedQuality(grade: $grade, inspectionNotes: $inspectionNotes)';
}


}

/// @nodoc
abstract mixin class _$OfferedQualityCopyWith<$Res> implements $OfferedQualityCopyWith<$Res> {
  factory _$OfferedQualityCopyWith(_OfferedQuality value, $Res Function(_OfferedQuality) _then) = __$OfferedQualityCopyWithImpl;
@override @useResult
$Res call({
 String grade, String inspectionNotes
});




}
/// @nodoc
class __$OfferedQualityCopyWithImpl<$Res>
    implements _$OfferedQualityCopyWith<$Res> {
  __$OfferedQualityCopyWithImpl(this._self, this._then);

  final _OfferedQuality _self;
  final $Res Function(_OfferedQuality) _then;

/// Create a copy of OfferedQuality
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? grade = null,Object? inspectionNotes = null,}) {
  return _then(_OfferedQuality(
grade: null == grade ? _self.grade : grade // ignore: cast_nullable_to_non_nullable
as String,inspectionNotes: null == inspectionNotes ? _self.inspectionNotes : inspectionNotes // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$UnitBasis {

 String get quantity; String get unit;
/// Create a copy of UnitBasis
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$UnitBasisCopyWith<UnitBasis> get copyWith => _$UnitBasisCopyWithImpl<UnitBasis>(this as UnitBasis, _$identity);

  /// Serializes this UnitBasis to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is UnitBasis&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unit, unit) || other.unit == unit));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,quantity,unit);

@override
String toString() {
  return 'UnitBasis(quantity: $quantity, unit: $unit)';
}


}

/// @nodoc
abstract mixin class $UnitBasisCopyWith<$Res>  {
  factory $UnitBasisCopyWith(UnitBasis value, $Res Function(UnitBasis) _then) = _$UnitBasisCopyWithImpl;
@useResult
$Res call({
 String quantity, String unit
});




}
/// @nodoc
class _$UnitBasisCopyWithImpl<$Res>
    implements $UnitBasisCopyWith<$Res> {
  _$UnitBasisCopyWithImpl(this._self, this._then);

  final UnitBasis _self;
  final $Res Function(UnitBasis) _then;

/// Create a copy of UnitBasis
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? quantity = null,Object? unit = null,}) {
  return _then(_self.copyWith(
quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as String,unit: null == unit ? _self.unit : unit // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [UnitBasis].
extension UnitBasisPatterns on UnitBasis {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _UnitBasis value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _UnitBasis() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _UnitBasis value)  $default,){
final _that = this;
switch (_that) {
case _UnitBasis():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _UnitBasis value)?  $default,){
final _that = this;
switch (_that) {
case _UnitBasis() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String quantity,  String unit)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _UnitBasis() when $default != null:
return $default(_that.quantity,_that.unit);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String quantity,  String unit)  $default,) {final _that = this;
switch (_that) {
case _UnitBasis():
return $default(_that.quantity,_that.unit);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String quantity,  String unit)?  $default,) {final _that = this;
switch (_that) {
case _UnitBasis() when $default != null:
return $default(_that.quantity,_that.unit);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _UnitBasis implements UnitBasis {
  const _UnitBasis({this.quantity = '', this.unit = ''});
  factory _UnitBasis.fromJson(Map<String, dynamic> json) => _$UnitBasisFromJson(json);

@override@JsonKey() final  String quantity;
@override@JsonKey() final  String unit;

/// Create a copy of UnitBasis
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$UnitBasisCopyWith<_UnitBasis> get copyWith => __$UnitBasisCopyWithImpl<_UnitBasis>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$UnitBasisToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _UnitBasis&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unit, unit) || other.unit == unit));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,quantity,unit);

@override
String toString() {
  return 'UnitBasis(quantity: $quantity, unit: $unit)';
}


}

/// @nodoc
abstract mixin class _$UnitBasisCopyWith<$Res> implements $UnitBasisCopyWith<$Res> {
  factory _$UnitBasisCopyWith(_UnitBasis value, $Res Function(_UnitBasis) _then) = __$UnitBasisCopyWithImpl;
@override @useResult
$Res call({
 String quantity, String unit
});




}
/// @nodoc
class __$UnitBasisCopyWithImpl<$Res>
    implements _$UnitBasisCopyWith<$Res> {
  __$UnitBasisCopyWithImpl(this._self, this._then);

  final _UnitBasis _self;
  final $Res Function(_UnitBasis) _then;

/// Create a copy of UnitBasis
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? quantity = null,Object? unit = null,}) {
  return _then(_UnitBasis(
quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as String,unit: null == unit ? _self.unit : unit // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$SellerQuotePayload {

 String get responseType;@JsonKey(toJson: _offeredQualityToJson) OfferedQuality get offeredQuality;@JsonKey(toJson: _unitBasisToJson) UnitBasis get unitBasis; String get offeredQuantity; String get offeredQuantityUnit; String get conversionBasis; String get currency; String get goodsAmount; String get shippingAmount; String get taxAmount; String get taxInclusionBasis; String get totalAmount; String get leadTimeDays; String get promisedDeliveryAt; String get validUntil; String get termsNote; String get deviationDisclosure;
/// Create a copy of SellerQuotePayload
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerQuotePayloadCopyWith<SellerQuotePayload> get copyWith => _$SellerQuotePayloadCopyWithImpl<SellerQuotePayload>(this as SellerQuotePayload, _$identity);

  /// Serializes this SellerQuotePayload to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerQuotePayload&&(identical(other.responseType, responseType) || other.responseType == responseType)&&(identical(other.offeredQuality, offeredQuality) || other.offeredQuality == offeredQuality)&&(identical(other.unitBasis, unitBasis) || other.unitBasis == unitBasis)&&(identical(other.offeredQuantity, offeredQuantity) || other.offeredQuantity == offeredQuantity)&&(identical(other.offeredQuantityUnit, offeredQuantityUnit) || other.offeredQuantityUnit == offeredQuantityUnit)&&(identical(other.conversionBasis, conversionBasis) || other.conversionBasis == conversionBasis)&&(identical(other.currency, currency) || other.currency == currency)&&(identical(other.goodsAmount, goodsAmount) || other.goodsAmount == goodsAmount)&&(identical(other.shippingAmount, shippingAmount) || other.shippingAmount == shippingAmount)&&(identical(other.taxAmount, taxAmount) || other.taxAmount == taxAmount)&&(identical(other.taxInclusionBasis, taxInclusionBasis) || other.taxInclusionBasis == taxInclusionBasis)&&(identical(other.totalAmount, totalAmount) || other.totalAmount == totalAmount)&&(identical(other.leadTimeDays, leadTimeDays) || other.leadTimeDays == leadTimeDays)&&(identical(other.promisedDeliveryAt, promisedDeliveryAt) || other.promisedDeliveryAt == promisedDeliveryAt)&&(identical(other.validUntil, validUntil) || other.validUntil == validUntil)&&(identical(other.termsNote, termsNote) || other.termsNote == termsNote)&&(identical(other.deviationDisclosure, deviationDisclosure) || other.deviationDisclosure == deviationDisclosure));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,responseType,offeredQuality,unitBasis,offeredQuantity,offeredQuantityUnit,conversionBasis,currency,goodsAmount,shippingAmount,taxAmount,taxInclusionBasis,totalAmount,leadTimeDays,promisedDeliveryAt,validUntil,termsNote,deviationDisclosure);

@override
String toString() {
  return 'SellerQuotePayload(responseType: $responseType, offeredQuality: $offeredQuality, unitBasis: $unitBasis, offeredQuantity: $offeredQuantity, offeredQuantityUnit: $offeredQuantityUnit, conversionBasis: $conversionBasis, currency: $currency, goodsAmount: $goodsAmount, shippingAmount: $shippingAmount, taxAmount: $taxAmount, taxInclusionBasis: $taxInclusionBasis, totalAmount: $totalAmount, leadTimeDays: $leadTimeDays, promisedDeliveryAt: $promisedDeliveryAt, validUntil: $validUntil, termsNote: $termsNote, deviationDisclosure: $deviationDisclosure)';
}


}

/// @nodoc
abstract mixin class $SellerQuotePayloadCopyWith<$Res>  {
  factory $SellerQuotePayloadCopyWith(SellerQuotePayload value, $Res Function(SellerQuotePayload) _then) = _$SellerQuotePayloadCopyWithImpl;
@useResult
$Res call({
 String responseType,@JsonKey(toJson: _offeredQualityToJson) OfferedQuality offeredQuality,@JsonKey(toJson: _unitBasisToJson) UnitBasis unitBasis, String offeredQuantity, String offeredQuantityUnit, String conversionBasis, String currency, String goodsAmount, String shippingAmount, String taxAmount, String taxInclusionBasis, String totalAmount, String leadTimeDays, String promisedDeliveryAt, String validUntil, String termsNote, String deviationDisclosure
});


$OfferedQualityCopyWith<$Res> get offeredQuality;$UnitBasisCopyWith<$Res> get unitBasis;

}
/// @nodoc
class _$SellerQuotePayloadCopyWithImpl<$Res>
    implements $SellerQuotePayloadCopyWith<$Res> {
  _$SellerQuotePayloadCopyWithImpl(this._self, this._then);

  final SellerQuotePayload _self;
  final $Res Function(SellerQuotePayload) _then;

/// Create a copy of SellerQuotePayload
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? responseType = null,Object? offeredQuality = null,Object? unitBasis = null,Object? offeredQuantity = null,Object? offeredQuantityUnit = null,Object? conversionBasis = null,Object? currency = null,Object? goodsAmount = null,Object? shippingAmount = null,Object? taxAmount = null,Object? taxInclusionBasis = null,Object? totalAmount = null,Object? leadTimeDays = null,Object? promisedDeliveryAt = null,Object? validUntil = null,Object? termsNote = null,Object? deviationDisclosure = null,}) {
  return _then(_self.copyWith(
responseType: null == responseType ? _self.responseType : responseType // ignore: cast_nullable_to_non_nullable
as String,offeredQuality: null == offeredQuality ? _self.offeredQuality : offeredQuality // ignore: cast_nullable_to_non_nullable
as OfferedQuality,unitBasis: null == unitBasis ? _self.unitBasis : unitBasis // ignore: cast_nullable_to_non_nullable
as UnitBasis,offeredQuantity: null == offeredQuantity ? _self.offeredQuantity : offeredQuantity // ignore: cast_nullable_to_non_nullable
as String,offeredQuantityUnit: null == offeredQuantityUnit ? _self.offeredQuantityUnit : offeredQuantityUnit // ignore: cast_nullable_to_non_nullable
as String,conversionBasis: null == conversionBasis ? _self.conversionBasis : conversionBasis // ignore: cast_nullable_to_non_nullable
as String,currency: null == currency ? _self.currency : currency // ignore: cast_nullable_to_non_nullable
as String,goodsAmount: null == goodsAmount ? _self.goodsAmount : goodsAmount // ignore: cast_nullable_to_non_nullable
as String,shippingAmount: null == shippingAmount ? _self.shippingAmount : shippingAmount // ignore: cast_nullable_to_non_nullable
as String,taxAmount: null == taxAmount ? _self.taxAmount : taxAmount // ignore: cast_nullable_to_non_nullable
as String,taxInclusionBasis: null == taxInclusionBasis ? _self.taxInclusionBasis : taxInclusionBasis // ignore: cast_nullable_to_non_nullable
as String,totalAmount: null == totalAmount ? _self.totalAmount : totalAmount // ignore: cast_nullable_to_non_nullable
as String,leadTimeDays: null == leadTimeDays ? _self.leadTimeDays : leadTimeDays // ignore: cast_nullable_to_non_nullable
as String,promisedDeliveryAt: null == promisedDeliveryAt ? _self.promisedDeliveryAt : promisedDeliveryAt // ignore: cast_nullable_to_non_nullable
as String,validUntil: null == validUntil ? _self.validUntil : validUntil // ignore: cast_nullable_to_non_nullable
as String,termsNote: null == termsNote ? _self.termsNote : termsNote // ignore: cast_nullable_to_non_nullable
as String,deviationDisclosure: null == deviationDisclosure ? _self.deviationDisclosure : deviationDisclosure // ignore: cast_nullable_to_non_nullable
as String,
  ));
}
/// Create a copy of SellerQuotePayload
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$OfferedQualityCopyWith<$Res> get offeredQuality {
  
  return $OfferedQualityCopyWith<$Res>(_self.offeredQuality, (value) {
    return _then(_self.copyWith(offeredQuality: value));
  });
}/// Create a copy of SellerQuotePayload
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$UnitBasisCopyWith<$Res> get unitBasis {
  
  return $UnitBasisCopyWith<$Res>(_self.unitBasis, (value) {
    return _then(_self.copyWith(unitBasis: value));
  });
}
}


/// Adds pattern-matching-related methods to [SellerQuotePayload].
extension SellerQuotePayloadPatterns on SellerQuotePayload {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerQuotePayload value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerQuotePayload() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerQuotePayload value)  $default,){
final _that = this;
switch (_that) {
case _SellerQuotePayload():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerQuotePayload value)?  $default,){
final _that = this;
switch (_that) {
case _SellerQuotePayload() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String responseType, @JsonKey(toJson: _offeredQualityToJson)  OfferedQuality offeredQuality, @JsonKey(toJson: _unitBasisToJson)  UnitBasis unitBasis,  String offeredQuantity,  String offeredQuantityUnit,  String conversionBasis,  String currency,  String goodsAmount,  String shippingAmount,  String taxAmount,  String taxInclusionBasis,  String totalAmount,  String leadTimeDays,  String promisedDeliveryAt,  String validUntil,  String termsNote,  String deviationDisclosure)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerQuotePayload() when $default != null:
return $default(_that.responseType,_that.offeredQuality,_that.unitBasis,_that.offeredQuantity,_that.offeredQuantityUnit,_that.conversionBasis,_that.currency,_that.goodsAmount,_that.shippingAmount,_that.taxAmount,_that.taxInclusionBasis,_that.totalAmount,_that.leadTimeDays,_that.promisedDeliveryAt,_that.validUntil,_that.termsNote,_that.deviationDisclosure);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String responseType, @JsonKey(toJson: _offeredQualityToJson)  OfferedQuality offeredQuality, @JsonKey(toJson: _unitBasisToJson)  UnitBasis unitBasis,  String offeredQuantity,  String offeredQuantityUnit,  String conversionBasis,  String currency,  String goodsAmount,  String shippingAmount,  String taxAmount,  String taxInclusionBasis,  String totalAmount,  String leadTimeDays,  String promisedDeliveryAt,  String validUntil,  String termsNote,  String deviationDisclosure)  $default,) {final _that = this;
switch (_that) {
case _SellerQuotePayload():
return $default(_that.responseType,_that.offeredQuality,_that.unitBasis,_that.offeredQuantity,_that.offeredQuantityUnit,_that.conversionBasis,_that.currency,_that.goodsAmount,_that.shippingAmount,_that.taxAmount,_that.taxInclusionBasis,_that.totalAmount,_that.leadTimeDays,_that.promisedDeliveryAt,_that.validUntil,_that.termsNote,_that.deviationDisclosure);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String responseType, @JsonKey(toJson: _offeredQualityToJson)  OfferedQuality offeredQuality, @JsonKey(toJson: _unitBasisToJson)  UnitBasis unitBasis,  String offeredQuantity,  String offeredQuantityUnit,  String conversionBasis,  String currency,  String goodsAmount,  String shippingAmount,  String taxAmount,  String taxInclusionBasis,  String totalAmount,  String leadTimeDays,  String promisedDeliveryAt,  String validUntil,  String termsNote,  String deviationDisclosure)?  $default,) {final _that = this;
switch (_that) {
case _SellerQuotePayload() when $default != null:
return $default(_that.responseType,_that.offeredQuality,_that.unitBasis,_that.offeredQuantity,_that.offeredQuantityUnit,_that.conversionBasis,_that.currency,_that.goodsAmount,_that.shippingAmount,_that.taxAmount,_that.taxInclusionBasis,_that.totalAmount,_that.leadTimeDays,_that.promisedDeliveryAt,_that.validUntil,_that.termsNote,_that.deviationDisclosure);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SellerQuotePayload extends SellerQuotePayload {
  const _SellerQuotePayload({this.responseType = '', @JsonKey(toJson: _offeredQualityToJson) this.offeredQuality = const OfferedQuality(), @JsonKey(toJson: _unitBasisToJson) this.unitBasis = const UnitBasis(), this.offeredQuantity = '', this.offeredQuantityUnit = '', this.conversionBasis = '', this.currency = kQuoteCurrencyTwd, this.goodsAmount = '', this.shippingAmount = '', this.taxAmount = '', this.taxInclusionBasis = '', this.totalAmount = '', this.leadTimeDays = '', this.promisedDeliveryAt = '', this.validUntil = '', this.termsNote = '', this.deviationDisclosure = ''}): super._();
  factory _SellerQuotePayload.fromJson(Map<String, dynamic> json) => _$SellerQuotePayloadFromJson(json);

@override@JsonKey() final  String responseType;
@override@JsonKey(toJson: _offeredQualityToJson) final  OfferedQuality offeredQuality;
@override@JsonKey(toJson: _unitBasisToJson) final  UnitBasis unitBasis;
@override@JsonKey() final  String offeredQuantity;
@override@JsonKey() final  String offeredQuantityUnit;
@override@JsonKey() final  String conversionBasis;
@override@JsonKey() final  String currency;
@override@JsonKey() final  String goodsAmount;
@override@JsonKey() final  String shippingAmount;
@override@JsonKey() final  String taxAmount;
@override@JsonKey() final  String taxInclusionBasis;
@override@JsonKey() final  String totalAmount;
@override@JsonKey() final  String leadTimeDays;
@override@JsonKey() final  String promisedDeliveryAt;
@override@JsonKey() final  String validUntil;
@override@JsonKey() final  String termsNote;
@override@JsonKey() final  String deviationDisclosure;

/// Create a copy of SellerQuotePayload
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerQuotePayloadCopyWith<_SellerQuotePayload> get copyWith => __$SellerQuotePayloadCopyWithImpl<_SellerQuotePayload>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SellerQuotePayloadToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerQuotePayload&&(identical(other.responseType, responseType) || other.responseType == responseType)&&(identical(other.offeredQuality, offeredQuality) || other.offeredQuality == offeredQuality)&&(identical(other.unitBasis, unitBasis) || other.unitBasis == unitBasis)&&(identical(other.offeredQuantity, offeredQuantity) || other.offeredQuantity == offeredQuantity)&&(identical(other.offeredQuantityUnit, offeredQuantityUnit) || other.offeredQuantityUnit == offeredQuantityUnit)&&(identical(other.conversionBasis, conversionBasis) || other.conversionBasis == conversionBasis)&&(identical(other.currency, currency) || other.currency == currency)&&(identical(other.goodsAmount, goodsAmount) || other.goodsAmount == goodsAmount)&&(identical(other.shippingAmount, shippingAmount) || other.shippingAmount == shippingAmount)&&(identical(other.taxAmount, taxAmount) || other.taxAmount == taxAmount)&&(identical(other.taxInclusionBasis, taxInclusionBasis) || other.taxInclusionBasis == taxInclusionBasis)&&(identical(other.totalAmount, totalAmount) || other.totalAmount == totalAmount)&&(identical(other.leadTimeDays, leadTimeDays) || other.leadTimeDays == leadTimeDays)&&(identical(other.promisedDeliveryAt, promisedDeliveryAt) || other.promisedDeliveryAt == promisedDeliveryAt)&&(identical(other.validUntil, validUntil) || other.validUntil == validUntil)&&(identical(other.termsNote, termsNote) || other.termsNote == termsNote)&&(identical(other.deviationDisclosure, deviationDisclosure) || other.deviationDisclosure == deviationDisclosure));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,responseType,offeredQuality,unitBasis,offeredQuantity,offeredQuantityUnit,conversionBasis,currency,goodsAmount,shippingAmount,taxAmount,taxInclusionBasis,totalAmount,leadTimeDays,promisedDeliveryAt,validUntil,termsNote,deviationDisclosure);

@override
String toString() {
  return 'SellerQuotePayload(responseType: $responseType, offeredQuality: $offeredQuality, unitBasis: $unitBasis, offeredQuantity: $offeredQuantity, offeredQuantityUnit: $offeredQuantityUnit, conversionBasis: $conversionBasis, currency: $currency, goodsAmount: $goodsAmount, shippingAmount: $shippingAmount, taxAmount: $taxAmount, taxInclusionBasis: $taxInclusionBasis, totalAmount: $totalAmount, leadTimeDays: $leadTimeDays, promisedDeliveryAt: $promisedDeliveryAt, validUntil: $validUntil, termsNote: $termsNote, deviationDisclosure: $deviationDisclosure)';
}


}

/// @nodoc
abstract mixin class _$SellerQuotePayloadCopyWith<$Res> implements $SellerQuotePayloadCopyWith<$Res> {
  factory _$SellerQuotePayloadCopyWith(_SellerQuotePayload value, $Res Function(_SellerQuotePayload) _then) = __$SellerQuotePayloadCopyWithImpl;
@override @useResult
$Res call({
 String responseType,@JsonKey(toJson: _offeredQualityToJson) OfferedQuality offeredQuality,@JsonKey(toJson: _unitBasisToJson) UnitBasis unitBasis, String offeredQuantity, String offeredQuantityUnit, String conversionBasis, String currency, String goodsAmount, String shippingAmount, String taxAmount, String taxInclusionBasis, String totalAmount, String leadTimeDays, String promisedDeliveryAt, String validUntil, String termsNote, String deviationDisclosure
});


@override $OfferedQualityCopyWith<$Res> get offeredQuality;@override $UnitBasisCopyWith<$Res> get unitBasis;

}
/// @nodoc
class __$SellerQuotePayloadCopyWithImpl<$Res>
    implements _$SellerQuotePayloadCopyWith<$Res> {
  __$SellerQuotePayloadCopyWithImpl(this._self, this._then);

  final _SellerQuotePayload _self;
  final $Res Function(_SellerQuotePayload) _then;

/// Create a copy of SellerQuotePayload
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? responseType = null,Object? offeredQuality = null,Object? unitBasis = null,Object? offeredQuantity = null,Object? offeredQuantityUnit = null,Object? conversionBasis = null,Object? currency = null,Object? goodsAmount = null,Object? shippingAmount = null,Object? taxAmount = null,Object? taxInclusionBasis = null,Object? totalAmount = null,Object? leadTimeDays = null,Object? promisedDeliveryAt = null,Object? validUntil = null,Object? termsNote = null,Object? deviationDisclosure = null,}) {
  return _then(_SellerQuotePayload(
responseType: null == responseType ? _self.responseType : responseType // ignore: cast_nullable_to_non_nullable
as String,offeredQuality: null == offeredQuality ? _self.offeredQuality : offeredQuality // ignore: cast_nullable_to_non_nullable
as OfferedQuality,unitBasis: null == unitBasis ? _self.unitBasis : unitBasis // ignore: cast_nullable_to_non_nullable
as UnitBasis,offeredQuantity: null == offeredQuantity ? _self.offeredQuantity : offeredQuantity // ignore: cast_nullable_to_non_nullable
as String,offeredQuantityUnit: null == offeredQuantityUnit ? _self.offeredQuantityUnit : offeredQuantityUnit // ignore: cast_nullable_to_non_nullable
as String,conversionBasis: null == conversionBasis ? _self.conversionBasis : conversionBasis // ignore: cast_nullable_to_non_nullable
as String,currency: null == currency ? _self.currency : currency // ignore: cast_nullable_to_non_nullable
as String,goodsAmount: null == goodsAmount ? _self.goodsAmount : goodsAmount // ignore: cast_nullable_to_non_nullable
as String,shippingAmount: null == shippingAmount ? _self.shippingAmount : shippingAmount // ignore: cast_nullable_to_non_nullable
as String,taxAmount: null == taxAmount ? _self.taxAmount : taxAmount // ignore: cast_nullable_to_non_nullable
as String,taxInclusionBasis: null == taxInclusionBasis ? _self.taxInclusionBasis : taxInclusionBasis // ignore: cast_nullable_to_non_nullable
as String,totalAmount: null == totalAmount ? _self.totalAmount : totalAmount // ignore: cast_nullable_to_non_nullable
as String,leadTimeDays: null == leadTimeDays ? _self.leadTimeDays : leadTimeDays // ignore: cast_nullable_to_non_nullable
as String,promisedDeliveryAt: null == promisedDeliveryAt ? _self.promisedDeliveryAt : promisedDeliveryAt // ignore: cast_nullable_to_non_nullable
as String,validUntil: null == validUntil ? _self.validUntil : validUntil // ignore: cast_nullable_to_non_nullable
as String,termsNote: null == termsNote ? _self.termsNote : termsNote // ignore: cast_nullable_to_non_nullable
as String,deviationDisclosure: null == deviationDisclosure ? _self.deviationDisclosure : deviationDisclosure // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

/// Create a copy of SellerQuotePayload
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$OfferedQualityCopyWith<$Res> get offeredQuality {
  
  return $OfferedQualityCopyWith<$Res>(_self.offeredQuality, (value) {
    return _then(_self.copyWith(offeredQuality: value));
  });
}/// Create a copy of SellerQuotePayload
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$UnitBasisCopyWith<$Res> get unitBasis {
  
  return $UnitBasisCopyWith<$Res>(_self.unitBasis, (value) {
    return _then(_self.copyWith(unitBasis: value));
  });
}
}


/// @nodoc
mixin _$SellerResponseVersionResponse {

 String get sellerResponseId; String get sellerResponseVersion; String get publicationId; String get status; String get payloadHash; String get submittedAt; String get supersededByVersion; bool get outsideBuyerRequestedRange;@JsonKey(toJson: _quoteToJson) SellerQuotePayload get quote; bool get replayed;
/// Create a copy of SellerResponseVersionResponse
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerResponseVersionResponseCopyWith<SellerResponseVersionResponse> get copyWith => _$SellerResponseVersionResponseCopyWithImpl<SellerResponseVersionResponse>(this as SellerResponseVersionResponse, _$identity);

  /// Serializes this SellerResponseVersionResponse to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerResponseVersionResponse&&(identical(other.sellerResponseId, sellerResponseId) || other.sellerResponseId == sellerResponseId)&&(identical(other.sellerResponseVersion, sellerResponseVersion) || other.sellerResponseVersion == sellerResponseVersion)&&(identical(other.publicationId, publicationId) || other.publicationId == publicationId)&&(identical(other.status, status) || other.status == status)&&(identical(other.payloadHash, payloadHash) || other.payloadHash == payloadHash)&&(identical(other.submittedAt, submittedAt) || other.submittedAt == submittedAt)&&(identical(other.supersededByVersion, supersededByVersion) || other.supersededByVersion == supersededByVersion)&&(identical(other.outsideBuyerRequestedRange, outsideBuyerRequestedRange) || other.outsideBuyerRequestedRange == outsideBuyerRequestedRange)&&(identical(other.quote, quote) || other.quote == quote)&&(identical(other.replayed, replayed) || other.replayed == replayed));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,sellerResponseId,sellerResponseVersion,publicationId,status,payloadHash,submittedAt,supersededByVersion,outsideBuyerRequestedRange,quote,replayed);

@override
String toString() {
  return 'SellerResponseVersionResponse(sellerResponseId: $sellerResponseId, sellerResponseVersion: $sellerResponseVersion, publicationId: $publicationId, status: $status, payloadHash: $payloadHash, submittedAt: $submittedAt, supersededByVersion: $supersededByVersion, outsideBuyerRequestedRange: $outsideBuyerRequestedRange, quote: $quote, replayed: $replayed)';
}


}

/// @nodoc
abstract mixin class $SellerResponseVersionResponseCopyWith<$Res>  {
  factory $SellerResponseVersionResponseCopyWith(SellerResponseVersionResponse value, $Res Function(SellerResponseVersionResponse) _then) = _$SellerResponseVersionResponseCopyWithImpl;
@useResult
$Res call({
 String sellerResponseId, String sellerResponseVersion, String publicationId, String status, String payloadHash, String submittedAt, String supersededByVersion, bool outsideBuyerRequestedRange,@JsonKey(toJson: _quoteToJson) SellerQuotePayload quote, bool replayed
});


$SellerQuotePayloadCopyWith<$Res> get quote;

}
/// @nodoc
class _$SellerResponseVersionResponseCopyWithImpl<$Res>
    implements $SellerResponseVersionResponseCopyWith<$Res> {
  _$SellerResponseVersionResponseCopyWithImpl(this._self, this._then);

  final SellerResponseVersionResponse _self;
  final $Res Function(SellerResponseVersionResponse) _then;

/// Create a copy of SellerResponseVersionResponse
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? sellerResponseId = null,Object? sellerResponseVersion = null,Object? publicationId = null,Object? status = null,Object? payloadHash = null,Object? submittedAt = null,Object? supersededByVersion = null,Object? outsideBuyerRequestedRange = null,Object? quote = null,Object? replayed = null,}) {
  return _then(_self.copyWith(
sellerResponseId: null == sellerResponseId ? _self.sellerResponseId : sellerResponseId // ignore: cast_nullable_to_non_nullable
as String,sellerResponseVersion: null == sellerResponseVersion ? _self.sellerResponseVersion : sellerResponseVersion // ignore: cast_nullable_to_non_nullable
as String,publicationId: null == publicationId ? _self.publicationId : publicationId // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,payloadHash: null == payloadHash ? _self.payloadHash : payloadHash // ignore: cast_nullable_to_non_nullable
as String,submittedAt: null == submittedAt ? _self.submittedAt : submittedAt // ignore: cast_nullable_to_non_nullable
as String,supersededByVersion: null == supersededByVersion ? _self.supersededByVersion : supersededByVersion // ignore: cast_nullable_to_non_nullable
as String,outsideBuyerRequestedRange: null == outsideBuyerRequestedRange ? _self.outsideBuyerRequestedRange : outsideBuyerRequestedRange // ignore: cast_nullable_to_non_nullable
as bool,quote: null == quote ? _self.quote : quote // ignore: cast_nullable_to_non_nullable
as SellerQuotePayload,replayed: null == replayed ? _self.replayed : replayed // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}
/// Create a copy of SellerResponseVersionResponse
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerQuotePayloadCopyWith<$Res> get quote {
  
  return $SellerQuotePayloadCopyWith<$Res>(_self.quote, (value) {
    return _then(_self.copyWith(quote: value));
  });
}
}


/// Adds pattern-matching-related methods to [SellerResponseVersionResponse].
extension SellerResponseVersionResponsePatterns on SellerResponseVersionResponse {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerResponseVersionResponse value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerResponseVersionResponse() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerResponseVersionResponse value)  $default,){
final _that = this;
switch (_that) {
case _SellerResponseVersionResponse():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerResponseVersionResponse value)?  $default,){
final _that = this;
switch (_that) {
case _SellerResponseVersionResponse() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String sellerResponseId,  String sellerResponseVersion,  String publicationId,  String status,  String payloadHash,  String submittedAt,  String supersededByVersion,  bool outsideBuyerRequestedRange, @JsonKey(toJson: _quoteToJson)  SellerQuotePayload quote,  bool replayed)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerResponseVersionResponse() when $default != null:
return $default(_that.sellerResponseId,_that.sellerResponseVersion,_that.publicationId,_that.status,_that.payloadHash,_that.submittedAt,_that.supersededByVersion,_that.outsideBuyerRequestedRange,_that.quote,_that.replayed);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String sellerResponseId,  String sellerResponseVersion,  String publicationId,  String status,  String payloadHash,  String submittedAt,  String supersededByVersion,  bool outsideBuyerRequestedRange, @JsonKey(toJson: _quoteToJson)  SellerQuotePayload quote,  bool replayed)  $default,) {final _that = this;
switch (_that) {
case _SellerResponseVersionResponse():
return $default(_that.sellerResponseId,_that.sellerResponseVersion,_that.publicationId,_that.status,_that.payloadHash,_that.submittedAt,_that.supersededByVersion,_that.outsideBuyerRequestedRange,_that.quote,_that.replayed);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String sellerResponseId,  String sellerResponseVersion,  String publicationId,  String status,  String payloadHash,  String submittedAt,  String supersededByVersion,  bool outsideBuyerRequestedRange, @JsonKey(toJson: _quoteToJson)  SellerQuotePayload quote,  bool replayed)?  $default,) {final _that = this;
switch (_that) {
case _SellerResponseVersionResponse() when $default != null:
return $default(_that.sellerResponseId,_that.sellerResponseVersion,_that.publicationId,_that.status,_that.payloadHash,_that.submittedAt,_that.supersededByVersion,_that.outsideBuyerRequestedRange,_that.quote,_that.replayed);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SellerResponseVersionResponse extends SellerResponseVersionResponse {
  const _SellerResponseVersionResponse({this.sellerResponseId = '', this.sellerResponseVersion = '', this.publicationId = '', this.status = '', this.payloadHash = '', this.submittedAt = '', this.supersededByVersion = '', this.outsideBuyerRequestedRange = false, @JsonKey(toJson: _quoteToJson) this.quote = const SellerQuotePayload(), this.replayed = false}): super._();
  factory _SellerResponseVersionResponse.fromJson(Map<String, dynamic> json) => _$SellerResponseVersionResponseFromJson(json);

@override@JsonKey() final  String sellerResponseId;
@override@JsonKey() final  String sellerResponseVersion;
@override@JsonKey() final  String publicationId;
@override@JsonKey() final  String status;
@override@JsonKey() final  String payloadHash;
@override@JsonKey() final  String submittedAt;
@override@JsonKey() final  String supersededByVersion;
@override@JsonKey() final  bool outsideBuyerRequestedRange;
@override@JsonKey(toJson: _quoteToJson) final  SellerQuotePayload quote;
@override@JsonKey() final  bool replayed;

/// Create a copy of SellerResponseVersionResponse
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerResponseVersionResponseCopyWith<_SellerResponseVersionResponse> get copyWith => __$SellerResponseVersionResponseCopyWithImpl<_SellerResponseVersionResponse>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SellerResponseVersionResponseToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerResponseVersionResponse&&(identical(other.sellerResponseId, sellerResponseId) || other.sellerResponseId == sellerResponseId)&&(identical(other.sellerResponseVersion, sellerResponseVersion) || other.sellerResponseVersion == sellerResponseVersion)&&(identical(other.publicationId, publicationId) || other.publicationId == publicationId)&&(identical(other.status, status) || other.status == status)&&(identical(other.payloadHash, payloadHash) || other.payloadHash == payloadHash)&&(identical(other.submittedAt, submittedAt) || other.submittedAt == submittedAt)&&(identical(other.supersededByVersion, supersededByVersion) || other.supersededByVersion == supersededByVersion)&&(identical(other.outsideBuyerRequestedRange, outsideBuyerRequestedRange) || other.outsideBuyerRequestedRange == outsideBuyerRequestedRange)&&(identical(other.quote, quote) || other.quote == quote)&&(identical(other.replayed, replayed) || other.replayed == replayed));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,sellerResponseId,sellerResponseVersion,publicationId,status,payloadHash,submittedAt,supersededByVersion,outsideBuyerRequestedRange,quote,replayed);

@override
String toString() {
  return 'SellerResponseVersionResponse(sellerResponseId: $sellerResponseId, sellerResponseVersion: $sellerResponseVersion, publicationId: $publicationId, status: $status, payloadHash: $payloadHash, submittedAt: $submittedAt, supersededByVersion: $supersededByVersion, outsideBuyerRequestedRange: $outsideBuyerRequestedRange, quote: $quote, replayed: $replayed)';
}


}

/// @nodoc
abstract mixin class _$SellerResponseVersionResponseCopyWith<$Res> implements $SellerResponseVersionResponseCopyWith<$Res> {
  factory _$SellerResponseVersionResponseCopyWith(_SellerResponseVersionResponse value, $Res Function(_SellerResponseVersionResponse) _then) = __$SellerResponseVersionResponseCopyWithImpl;
@override @useResult
$Res call({
 String sellerResponseId, String sellerResponseVersion, String publicationId, String status, String payloadHash, String submittedAt, String supersededByVersion, bool outsideBuyerRequestedRange,@JsonKey(toJson: _quoteToJson) SellerQuotePayload quote, bool replayed
});


@override $SellerQuotePayloadCopyWith<$Res> get quote;

}
/// @nodoc
class __$SellerResponseVersionResponseCopyWithImpl<$Res>
    implements _$SellerResponseVersionResponseCopyWith<$Res> {
  __$SellerResponseVersionResponseCopyWithImpl(this._self, this._then);

  final _SellerResponseVersionResponse _self;
  final $Res Function(_SellerResponseVersionResponse) _then;

/// Create a copy of SellerResponseVersionResponse
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? sellerResponseId = null,Object? sellerResponseVersion = null,Object? publicationId = null,Object? status = null,Object? payloadHash = null,Object? submittedAt = null,Object? supersededByVersion = null,Object? outsideBuyerRequestedRange = null,Object? quote = null,Object? replayed = null,}) {
  return _then(_SellerResponseVersionResponse(
sellerResponseId: null == sellerResponseId ? _self.sellerResponseId : sellerResponseId // ignore: cast_nullable_to_non_nullable
as String,sellerResponseVersion: null == sellerResponseVersion ? _self.sellerResponseVersion : sellerResponseVersion // ignore: cast_nullable_to_non_nullable
as String,publicationId: null == publicationId ? _self.publicationId : publicationId // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,payloadHash: null == payloadHash ? _self.payloadHash : payloadHash // ignore: cast_nullable_to_non_nullable
as String,submittedAt: null == submittedAt ? _self.submittedAt : submittedAt // ignore: cast_nullable_to_non_nullable
as String,supersededByVersion: null == supersededByVersion ? _self.supersededByVersion : supersededByVersion // ignore: cast_nullable_to_non_nullable
as String,outsideBuyerRequestedRange: null == outsideBuyerRequestedRange ? _self.outsideBuyerRequestedRange : outsideBuyerRequestedRange // ignore: cast_nullable_to_non_nullable
as bool,quote: null == quote ? _self.quote : quote // ignore: cast_nullable_to_non_nullable
as SellerQuotePayload,replayed: null == replayed ? _self.replayed : replayed // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

/// Create a copy of SellerResponseVersionResponse
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerQuotePayloadCopyWith<$Res> get quote {
  
  return $SellerQuotePayloadCopyWith<$Res>(_self.quote, (value) {
    return _then(_self.copyWith(quote: value));
  });
}
}


/// @nodoc
mixin _$SellerResponseDraftResponse {

 String get draftVersion; String get updatedAt;@JsonKey(toJson: _quoteToJson) SellerQuotePayload get draft;
/// Create a copy of SellerResponseDraftResponse
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerResponseDraftResponseCopyWith<SellerResponseDraftResponse> get copyWith => _$SellerResponseDraftResponseCopyWithImpl<SellerResponseDraftResponse>(this as SellerResponseDraftResponse, _$identity);

  /// Serializes this SellerResponseDraftResponse to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerResponseDraftResponse&&(identical(other.draftVersion, draftVersion) || other.draftVersion == draftVersion)&&(identical(other.updatedAt, updatedAt) || other.updatedAt == updatedAt)&&(identical(other.draft, draft) || other.draft == draft));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,draftVersion,updatedAt,draft);

@override
String toString() {
  return 'SellerResponseDraftResponse(draftVersion: $draftVersion, updatedAt: $updatedAt, draft: $draft)';
}


}

/// @nodoc
abstract mixin class $SellerResponseDraftResponseCopyWith<$Res>  {
  factory $SellerResponseDraftResponseCopyWith(SellerResponseDraftResponse value, $Res Function(SellerResponseDraftResponse) _then) = _$SellerResponseDraftResponseCopyWithImpl;
@useResult
$Res call({
 String draftVersion, String updatedAt,@JsonKey(toJson: _quoteToJson) SellerQuotePayload draft
});


$SellerQuotePayloadCopyWith<$Res> get draft;

}
/// @nodoc
class _$SellerResponseDraftResponseCopyWithImpl<$Res>
    implements $SellerResponseDraftResponseCopyWith<$Res> {
  _$SellerResponseDraftResponseCopyWithImpl(this._self, this._then);

  final SellerResponseDraftResponse _self;
  final $Res Function(SellerResponseDraftResponse) _then;

/// Create a copy of SellerResponseDraftResponse
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? draftVersion = null,Object? updatedAt = null,Object? draft = null,}) {
  return _then(_self.copyWith(
draftVersion: null == draftVersion ? _self.draftVersion : draftVersion // ignore: cast_nullable_to_non_nullable
as String,updatedAt: null == updatedAt ? _self.updatedAt : updatedAt // ignore: cast_nullable_to_non_nullable
as String,draft: null == draft ? _self.draft : draft // ignore: cast_nullable_to_non_nullable
as SellerQuotePayload,
  ));
}
/// Create a copy of SellerResponseDraftResponse
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerQuotePayloadCopyWith<$Res> get draft {
  
  return $SellerQuotePayloadCopyWith<$Res>(_self.draft, (value) {
    return _then(_self.copyWith(draft: value));
  });
}
}


/// Adds pattern-matching-related methods to [SellerResponseDraftResponse].
extension SellerResponseDraftResponsePatterns on SellerResponseDraftResponse {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerResponseDraftResponse value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerResponseDraftResponse() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerResponseDraftResponse value)  $default,){
final _that = this;
switch (_that) {
case _SellerResponseDraftResponse():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerResponseDraftResponse value)?  $default,){
final _that = this;
switch (_that) {
case _SellerResponseDraftResponse() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String draftVersion,  String updatedAt, @JsonKey(toJson: _quoteToJson)  SellerQuotePayload draft)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerResponseDraftResponse() when $default != null:
return $default(_that.draftVersion,_that.updatedAt,_that.draft);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String draftVersion,  String updatedAt, @JsonKey(toJson: _quoteToJson)  SellerQuotePayload draft)  $default,) {final _that = this;
switch (_that) {
case _SellerResponseDraftResponse():
return $default(_that.draftVersion,_that.updatedAt,_that.draft);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String draftVersion,  String updatedAt, @JsonKey(toJson: _quoteToJson)  SellerQuotePayload draft)?  $default,) {final _that = this;
switch (_that) {
case _SellerResponseDraftResponse() when $default != null:
return $default(_that.draftVersion,_that.updatedAt,_that.draft);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SellerResponseDraftResponse implements SellerResponseDraftResponse {
  const _SellerResponseDraftResponse({this.draftVersion = '', this.updatedAt = '', @JsonKey(toJson: _quoteToJson) this.draft = const SellerQuotePayload()});
  factory _SellerResponseDraftResponse.fromJson(Map<String, dynamic> json) => _$SellerResponseDraftResponseFromJson(json);

@override@JsonKey() final  String draftVersion;
@override@JsonKey() final  String updatedAt;
@override@JsonKey(toJson: _quoteToJson) final  SellerQuotePayload draft;

/// Create a copy of SellerResponseDraftResponse
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerResponseDraftResponseCopyWith<_SellerResponseDraftResponse> get copyWith => __$SellerResponseDraftResponseCopyWithImpl<_SellerResponseDraftResponse>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SellerResponseDraftResponseToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerResponseDraftResponse&&(identical(other.draftVersion, draftVersion) || other.draftVersion == draftVersion)&&(identical(other.updatedAt, updatedAt) || other.updatedAt == updatedAt)&&(identical(other.draft, draft) || other.draft == draft));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,draftVersion,updatedAt,draft);

@override
String toString() {
  return 'SellerResponseDraftResponse(draftVersion: $draftVersion, updatedAt: $updatedAt, draft: $draft)';
}


}

/// @nodoc
abstract mixin class _$SellerResponseDraftResponseCopyWith<$Res> implements $SellerResponseDraftResponseCopyWith<$Res> {
  factory _$SellerResponseDraftResponseCopyWith(_SellerResponseDraftResponse value, $Res Function(_SellerResponseDraftResponse) _then) = __$SellerResponseDraftResponseCopyWithImpl;
@override @useResult
$Res call({
 String draftVersion, String updatedAt,@JsonKey(toJson: _quoteToJson) SellerQuotePayload draft
});


@override $SellerQuotePayloadCopyWith<$Res> get draft;

}
/// @nodoc
class __$SellerResponseDraftResponseCopyWithImpl<$Res>
    implements _$SellerResponseDraftResponseCopyWith<$Res> {
  __$SellerResponseDraftResponseCopyWithImpl(this._self, this._then);

  final _SellerResponseDraftResponse _self;
  final $Res Function(_SellerResponseDraftResponse) _then;

/// Create a copy of SellerResponseDraftResponse
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? draftVersion = null,Object? updatedAt = null,Object? draft = null,}) {
  return _then(_SellerResponseDraftResponse(
draftVersion: null == draftVersion ? _self.draftVersion : draftVersion // ignore: cast_nullable_to_non_nullable
as String,updatedAt: null == updatedAt ? _self.updatedAt : updatedAt // ignore: cast_nullable_to_non_nullable
as String,draft: null == draft ? _self.draft : draft // ignore: cast_nullable_to_non_nullable
as SellerQuotePayload,
  ));
}

/// Create a copy of SellerResponseDraftResponse
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerQuotePayloadCopyWith<$Res> get draft {
  
  return $SellerQuotePayloadCopyWith<$Res>(_self.draft, (value) {
    return _then(_self.copyWith(draft: value));
  });
}
}


/// @nodoc
mixin _$SellerFacingPublication {

 String get publicationId; String get publicationVersion; String get inviteId; String get lane; String get sellerFacingPayloadHash; String get deliveryState; String get publishedAt;
/// Create a copy of SellerFacingPublication
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerFacingPublicationCopyWith<SellerFacingPublication> get copyWith => _$SellerFacingPublicationCopyWithImpl<SellerFacingPublication>(this as SellerFacingPublication, _$identity);

  /// Serializes this SellerFacingPublication to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerFacingPublication&&(identical(other.publicationId, publicationId) || other.publicationId == publicationId)&&(identical(other.publicationVersion, publicationVersion) || other.publicationVersion == publicationVersion)&&(identical(other.inviteId, inviteId) || other.inviteId == inviteId)&&(identical(other.lane, lane) || other.lane == lane)&&(identical(other.sellerFacingPayloadHash, sellerFacingPayloadHash) || other.sellerFacingPayloadHash == sellerFacingPayloadHash)&&(identical(other.deliveryState, deliveryState) || other.deliveryState == deliveryState)&&(identical(other.publishedAt, publishedAt) || other.publishedAt == publishedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,publicationId,publicationVersion,inviteId,lane,sellerFacingPayloadHash,deliveryState,publishedAt);

@override
String toString() {
  return 'SellerFacingPublication(publicationId: $publicationId, publicationVersion: $publicationVersion, inviteId: $inviteId, lane: $lane, sellerFacingPayloadHash: $sellerFacingPayloadHash, deliveryState: $deliveryState, publishedAt: $publishedAt)';
}


}

/// @nodoc
abstract mixin class $SellerFacingPublicationCopyWith<$Res>  {
  factory $SellerFacingPublicationCopyWith(SellerFacingPublication value, $Res Function(SellerFacingPublication) _then) = _$SellerFacingPublicationCopyWithImpl;
@useResult
$Res call({
 String publicationId, String publicationVersion, String inviteId, String lane, String sellerFacingPayloadHash, String deliveryState, String publishedAt
});




}
/// @nodoc
class _$SellerFacingPublicationCopyWithImpl<$Res>
    implements $SellerFacingPublicationCopyWith<$Res> {
  _$SellerFacingPublicationCopyWithImpl(this._self, this._then);

  final SellerFacingPublication _self;
  final $Res Function(SellerFacingPublication) _then;

/// Create a copy of SellerFacingPublication
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? publicationId = null,Object? publicationVersion = null,Object? inviteId = null,Object? lane = null,Object? sellerFacingPayloadHash = null,Object? deliveryState = null,Object? publishedAt = null,}) {
  return _then(_self.copyWith(
publicationId: null == publicationId ? _self.publicationId : publicationId // ignore: cast_nullable_to_non_nullable
as String,publicationVersion: null == publicationVersion ? _self.publicationVersion : publicationVersion // ignore: cast_nullable_to_non_nullable
as String,inviteId: null == inviteId ? _self.inviteId : inviteId // ignore: cast_nullable_to_non_nullable
as String,lane: null == lane ? _self.lane : lane // ignore: cast_nullable_to_non_nullable
as String,sellerFacingPayloadHash: null == sellerFacingPayloadHash ? _self.sellerFacingPayloadHash : sellerFacingPayloadHash // ignore: cast_nullable_to_non_nullable
as String,deliveryState: null == deliveryState ? _self.deliveryState : deliveryState // ignore: cast_nullable_to_non_nullable
as String,publishedAt: null == publishedAt ? _self.publishedAt : publishedAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [SellerFacingPublication].
extension SellerFacingPublicationPatterns on SellerFacingPublication {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerFacingPublication value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerFacingPublication() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerFacingPublication value)  $default,){
final _that = this;
switch (_that) {
case _SellerFacingPublication():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerFacingPublication value)?  $default,){
final _that = this;
switch (_that) {
case _SellerFacingPublication() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String publicationId,  String publicationVersion,  String inviteId,  String lane,  String sellerFacingPayloadHash,  String deliveryState,  String publishedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerFacingPublication() when $default != null:
return $default(_that.publicationId,_that.publicationVersion,_that.inviteId,_that.lane,_that.sellerFacingPayloadHash,_that.deliveryState,_that.publishedAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String publicationId,  String publicationVersion,  String inviteId,  String lane,  String sellerFacingPayloadHash,  String deliveryState,  String publishedAt)  $default,) {final _that = this;
switch (_that) {
case _SellerFacingPublication():
return $default(_that.publicationId,_that.publicationVersion,_that.inviteId,_that.lane,_that.sellerFacingPayloadHash,_that.deliveryState,_that.publishedAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String publicationId,  String publicationVersion,  String inviteId,  String lane,  String sellerFacingPayloadHash,  String deliveryState,  String publishedAt)?  $default,) {final _that = this;
switch (_that) {
case _SellerFacingPublication() when $default != null:
return $default(_that.publicationId,_that.publicationVersion,_that.inviteId,_that.lane,_that.sellerFacingPayloadHash,_that.deliveryState,_that.publishedAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SellerFacingPublication implements SellerFacingPublication {
  const _SellerFacingPublication({this.publicationId = '', this.publicationVersion = '', this.inviteId = '', this.lane = '', this.sellerFacingPayloadHash = '', this.deliveryState = '', this.publishedAt = ''});
  factory _SellerFacingPublication.fromJson(Map<String, dynamic> json) => _$SellerFacingPublicationFromJson(json);

@override@JsonKey() final  String publicationId;
@override@JsonKey() final  String publicationVersion;
@override@JsonKey() final  String inviteId;
@override@JsonKey() final  String lane;
@override@JsonKey() final  String sellerFacingPayloadHash;
@override@JsonKey() final  String deliveryState;
@override@JsonKey() final  String publishedAt;

/// Create a copy of SellerFacingPublication
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerFacingPublicationCopyWith<_SellerFacingPublication> get copyWith => __$SellerFacingPublicationCopyWithImpl<_SellerFacingPublication>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SellerFacingPublicationToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerFacingPublication&&(identical(other.publicationId, publicationId) || other.publicationId == publicationId)&&(identical(other.publicationVersion, publicationVersion) || other.publicationVersion == publicationVersion)&&(identical(other.inviteId, inviteId) || other.inviteId == inviteId)&&(identical(other.lane, lane) || other.lane == lane)&&(identical(other.sellerFacingPayloadHash, sellerFacingPayloadHash) || other.sellerFacingPayloadHash == sellerFacingPayloadHash)&&(identical(other.deliveryState, deliveryState) || other.deliveryState == deliveryState)&&(identical(other.publishedAt, publishedAt) || other.publishedAt == publishedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,publicationId,publicationVersion,inviteId,lane,sellerFacingPayloadHash,deliveryState,publishedAt);

@override
String toString() {
  return 'SellerFacingPublication(publicationId: $publicationId, publicationVersion: $publicationVersion, inviteId: $inviteId, lane: $lane, sellerFacingPayloadHash: $sellerFacingPayloadHash, deliveryState: $deliveryState, publishedAt: $publishedAt)';
}


}

/// @nodoc
abstract mixin class _$SellerFacingPublicationCopyWith<$Res> implements $SellerFacingPublicationCopyWith<$Res> {
  factory _$SellerFacingPublicationCopyWith(_SellerFacingPublication value, $Res Function(_SellerFacingPublication) _then) = __$SellerFacingPublicationCopyWithImpl;
@override @useResult
$Res call({
 String publicationId, String publicationVersion, String inviteId, String lane, String sellerFacingPayloadHash, String deliveryState, String publishedAt
});




}
/// @nodoc
class __$SellerFacingPublicationCopyWithImpl<$Res>
    implements _$SellerFacingPublicationCopyWith<$Res> {
  __$SellerFacingPublicationCopyWithImpl(this._self, this._then);

  final _SellerFacingPublication _self;
  final $Res Function(_SellerFacingPublication) _then;

/// Create a copy of SellerFacingPublication
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? publicationId = null,Object? publicationVersion = null,Object? inviteId = null,Object? lane = null,Object? sellerFacingPayloadHash = null,Object? deliveryState = null,Object? publishedAt = null,}) {
  return _then(_SellerFacingPublication(
publicationId: null == publicationId ? _self.publicationId : publicationId // ignore: cast_nullable_to_non_nullable
as String,publicationVersion: null == publicationVersion ? _self.publicationVersion : publicationVersion // ignore: cast_nullable_to_non_nullable
as String,inviteId: null == inviteId ? _self.inviteId : inviteId // ignore: cast_nullable_to_non_nullable
as String,lane: null == lane ? _self.lane : lane // ignore: cast_nullable_to_non_nullable
as String,sellerFacingPayloadHash: null == sellerFacingPayloadHash ? _self.sellerFacingPayloadHash : sellerFacingPayloadHash // ignore: cast_nullable_to_non_nullable
as String,deliveryState: null == deliveryState ? _self.deliveryState : deliveryState // ignore: cast_nullable_to_non_nullable
as String,publishedAt: null == publishedAt ? _self.publishedAt : publishedAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$SellerFacingPayload {

 String get item; String get itemLabel; String get quantity; String get unit; String get packaging; String get deliveryCity; String get deliveryDate; String get responseDeadline; String get buyerNote;
/// Create a copy of SellerFacingPayload
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerFacingPayloadCopyWith<SellerFacingPayload> get copyWith => _$SellerFacingPayloadCopyWithImpl<SellerFacingPayload>(this as SellerFacingPayload, _$identity);

  /// Serializes this SellerFacingPayload to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerFacingPayload&&(identical(other.item, item) || other.item == item)&&(identical(other.itemLabel, itemLabel) || other.itemLabel == itemLabel)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unit, unit) || other.unit == unit)&&(identical(other.packaging, packaging) || other.packaging == packaging)&&(identical(other.deliveryCity, deliveryCity) || other.deliveryCity == deliveryCity)&&(identical(other.deliveryDate, deliveryDate) || other.deliveryDate == deliveryDate)&&(identical(other.responseDeadline, responseDeadline) || other.responseDeadline == responseDeadline)&&(identical(other.buyerNote, buyerNote) || other.buyerNote == buyerNote));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,item,itemLabel,quantity,unit,packaging,deliveryCity,deliveryDate,responseDeadline,buyerNote);

@override
String toString() {
  return 'SellerFacingPayload(item: $item, itemLabel: $itemLabel, quantity: $quantity, unit: $unit, packaging: $packaging, deliveryCity: $deliveryCity, deliveryDate: $deliveryDate, responseDeadline: $responseDeadline, buyerNote: $buyerNote)';
}


}

/// @nodoc
abstract mixin class $SellerFacingPayloadCopyWith<$Res>  {
  factory $SellerFacingPayloadCopyWith(SellerFacingPayload value, $Res Function(SellerFacingPayload) _then) = _$SellerFacingPayloadCopyWithImpl;
@useResult
$Res call({
 String item, String itemLabel, String quantity, String unit, String packaging, String deliveryCity, String deliveryDate, String responseDeadline, String buyerNote
});




}
/// @nodoc
class _$SellerFacingPayloadCopyWithImpl<$Res>
    implements $SellerFacingPayloadCopyWith<$Res> {
  _$SellerFacingPayloadCopyWithImpl(this._self, this._then);

  final SellerFacingPayload _self;
  final $Res Function(SellerFacingPayload) _then;

/// Create a copy of SellerFacingPayload
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? item = null,Object? itemLabel = null,Object? quantity = null,Object? unit = null,Object? packaging = null,Object? deliveryCity = null,Object? deliveryDate = null,Object? responseDeadline = null,Object? buyerNote = null,}) {
  return _then(_self.copyWith(
item: null == item ? _self.item : item // ignore: cast_nullable_to_non_nullable
as String,itemLabel: null == itemLabel ? _self.itemLabel : itemLabel // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as String,unit: null == unit ? _self.unit : unit // ignore: cast_nullable_to_non_nullable
as String,packaging: null == packaging ? _self.packaging : packaging // ignore: cast_nullable_to_non_nullable
as String,deliveryCity: null == deliveryCity ? _self.deliveryCity : deliveryCity // ignore: cast_nullable_to_non_nullable
as String,deliveryDate: null == deliveryDate ? _self.deliveryDate : deliveryDate // ignore: cast_nullable_to_non_nullable
as String,responseDeadline: null == responseDeadline ? _self.responseDeadline : responseDeadline // ignore: cast_nullable_to_non_nullable
as String,buyerNote: null == buyerNote ? _self.buyerNote : buyerNote // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [SellerFacingPayload].
extension SellerFacingPayloadPatterns on SellerFacingPayload {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerFacingPayload value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerFacingPayload() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerFacingPayload value)  $default,){
final _that = this;
switch (_that) {
case _SellerFacingPayload():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerFacingPayload value)?  $default,){
final _that = this;
switch (_that) {
case _SellerFacingPayload() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String item,  String itemLabel,  String quantity,  String unit,  String packaging,  String deliveryCity,  String deliveryDate,  String responseDeadline,  String buyerNote)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerFacingPayload() when $default != null:
return $default(_that.item,_that.itemLabel,_that.quantity,_that.unit,_that.packaging,_that.deliveryCity,_that.deliveryDate,_that.responseDeadline,_that.buyerNote);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String item,  String itemLabel,  String quantity,  String unit,  String packaging,  String deliveryCity,  String deliveryDate,  String responseDeadline,  String buyerNote)  $default,) {final _that = this;
switch (_that) {
case _SellerFacingPayload():
return $default(_that.item,_that.itemLabel,_that.quantity,_that.unit,_that.packaging,_that.deliveryCity,_that.deliveryDate,_that.responseDeadline,_that.buyerNote);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String item,  String itemLabel,  String quantity,  String unit,  String packaging,  String deliveryCity,  String deliveryDate,  String responseDeadline,  String buyerNote)?  $default,) {final _that = this;
switch (_that) {
case _SellerFacingPayload() when $default != null:
return $default(_that.item,_that.itemLabel,_that.quantity,_that.unit,_that.packaging,_that.deliveryCity,_that.deliveryDate,_that.responseDeadline,_that.buyerNote);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SellerFacingPayload extends SellerFacingPayload {
  const _SellerFacingPayload({this.item = '', this.itemLabel = '', this.quantity = '', this.unit = '', this.packaging = '', this.deliveryCity = '', this.deliveryDate = '', this.responseDeadline = '', this.buyerNote = ''}): super._();
  factory _SellerFacingPayload.fromJson(Map<String, dynamic> json) => _$SellerFacingPayloadFromJson(json);

@override@JsonKey() final  String item;
@override@JsonKey() final  String itemLabel;
@override@JsonKey() final  String quantity;
@override@JsonKey() final  String unit;
@override@JsonKey() final  String packaging;
@override@JsonKey() final  String deliveryCity;
@override@JsonKey() final  String deliveryDate;
@override@JsonKey() final  String responseDeadline;
@override@JsonKey() final  String buyerNote;

/// Create a copy of SellerFacingPayload
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerFacingPayloadCopyWith<_SellerFacingPayload> get copyWith => __$SellerFacingPayloadCopyWithImpl<_SellerFacingPayload>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SellerFacingPayloadToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerFacingPayload&&(identical(other.item, item) || other.item == item)&&(identical(other.itemLabel, itemLabel) || other.itemLabel == itemLabel)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unit, unit) || other.unit == unit)&&(identical(other.packaging, packaging) || other.packaging == packaging)&&(identical(other.deliveryCity, deliveryCity) || other.deliveryCity == deliveryCity)&&(identical(other.deliveryDate, deliveryDate) || other.deliveryDate == deliveryDate)&&(identical(other.responseDeadline, responseDeadline) || other.responseDeadline == responseDeadline)&&(identical(other.buyerNote, buyerNote) || other.buyerNote == buyerNote));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,item,itemLabel,quantity,unit,packaging,deliveryCity,deliveryDate,responseDeadline,buyerNote);

@override
String toString() {
  return 'SellerFacingPayload(item: $item, itemLabel: $itemLabel, quantity: $quantity, unit: $unit, packaging: $packaging, deliveryCity: $deliveryCity, deliveryDate: $deliveryDate, responseDeadline: $responseDeadline, buyerNote: $buyerNote)';
}


}

/// @nodoc
abstract mixin class _$SellerFacingPayloadCopyWith<$Res> implements $SellerFacingPayloadCopyWith<$Res> {
  factory _$SellerFacingPayloadCopyWith(_SellerFacingPayload value, $Res Function(_SellerFacingPayload) _then) = __$SellerFacingPayloadCopyWithImpl;
@override @useResult
$Res call({
 String item, String itemLabel, String quantity, String unit, String packaging, String deliveryCity, String deliveryDate, String responseDeadline, String buyerNote
});




}
/// @nodoc
class __$SellerFacingPayloadCopyWithImpl<$Res>
    implements _$SellerFacingPayloadCopyWith<$Res> {
  __$SellerFacingPayloadCopyWithImpl(this._self, this._then);

  final _SellerFacingPayload _self;
  final $Res Function(_SellerFacingPayload) _then;

/// Create a copy of SellerFacingPayload
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? item = null,Object? itemLabel = null,Object? quantity = null,Object? unit = null,Object? packaging = null,Object? deliveryCity = null,Object? deliveryDate = null,Object? responseDeadline = null,Object? buyerNote = null,}) {
  return _then(_SellerFacingPayload(
item: null == item ? _self.item : item // ignore: cast_nullable_to_non_nullable
as String,itemLabel: null == itemLabel ? _self.itemLabel : itemLabel // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as String,unit: null == unit ? _self.unit : unit // ignore: cast_nullable_to_non_nullable
as String,packaging: null == packaging ? _self.packaging : packaging // ignore: cast_nullable_to_non_nullable
as String,deliveryCity: null == deliveryCity ? _self.deliveryCity : deliveryCity // ignore: cast_nullable_to_non_nullable
as String,deliveryDate: null == deliveryDate ? _self.deliveryDate : deliveryDate // ignore: cast_nullable_to_non_nullable
as String,responseDeadline: null == responseDeadline ? _self.responseDeadline : responseDeadline // ignore: cast_nullable_to_non_nullable
as String,buyerNote: null == buyerNote ? _self.buyerNote : buyerNote // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$SellerPublicationSummary {

 SellerFacingPublication get publication; SellerFacingPayload get sellerFacingPayload;
/// Create a copy of SellerPublicationSummary
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerPublicationSummaryCopyWith<SellerPublicationSummary> get copyWith => _$SellerPublicationSummaryCopyWithImpl<SellerPublicationSummary>(this as SellerPublicationSummary, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerPublicationSummary&&(identical(other.publication, publication) || other.publication == publication)&&(identical(other.sellerFacingPayload, sellerFacingPayload) || other.sellerFacingPayload == sellerFacingPayload));
}


@override
int get hashCode => Object.hash(runtimeType,publication,sellerFacingPayload);

@override
String toString() {
  return 'SellerPublicationSummary(publication: $publication, sellerFacingPayload: $sellerFacingPayload)';
}


}

/// @nodoc
abstract mixin class $SellerPublicationSummaryCopyWith<$Res>  {
  factory $SellerPublicationSummaryCopyWith(SellerPublicationSummary value, $Res Function(SellerPublicationSummary) _then) = _$SellerPublicationSummaryCopyWithImpl;
@useResult
$Res call({
 SellerFacingPublication publication, SellerFacingPayload sellerFacingPayload
});


$SellerFacingPublicationCopyWith<$Res> get publication;$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload;

}
/// @nodoc
class _$SellerPublicationSummaryCopyWithImpl<$Res>
    implements $SellerPublicationSummaryCopyWith<$Res> {
  _$SellerPublicationSummaryCopyWithImpl(this._self, this._then);

  final SellerPublicationSummary _self;
  final $Res Function(SellerPublicationSummary) _then;

/// Create a copy of SellerPublicationSummary
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? publication = null,Object? sellerFacingPayload = null,}) {
  return _then(_self.copyWith(
publication: null == publication ? _self.publication : publication // ignore: cast_nullable_to_non_nullable
as SellerFacingPublication,sellerFacingPayload: null == sellerFacingPayload ? _self.sellerFacingPayload : sellerFacingPayload // ignore: cast_nullable_to_non_nullable
as SellerFacingPayload,
  ));
}
/// Create a copy of SellerPublicationSummary
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPublicationCopyWith<$Res> get publication {
  
  return $SellerFacingPublicationCopyWith<$Res>(_self.publication, (value) {
    return _then(_self.copyWith(publication: value));
  });
}/// Create a copy of SellerPublicationSummary
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload {
  
  return $SellerFacingPayloadCopyWith<$Res>(_self.sellerFacingPayload, (value) {
    return _then(_self.copyWith(sellerFacingPayload: value));
  });
}
}


/// Adds pattern-matching-related methods to [SellerPublicationSummary].
extension SellerPublicationSummaryPatterns on SellerPublicationSummary {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerPublicationSummary value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerPublicationSummary() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerPublicationSummary value)  $default,){
final _that = this;
switch (_that) {
case _SellerPublicationSummary():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerPublicationSummary value)?  $default,){
final _that = this;
switch (_that) {
case _SellerPublicationSummary() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerPublicationSummary() when $default != null:
return $default(_that.publication,_that.sellerFacingPayload);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload)  $default,) {final _that = this;
switch (_that) {
case _SellerPublicationSummary():
return $default(_that.publication,_that.sellerFacingPayload);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload)?  $default,) {final _that = this;
switch (_that) {
case _SellerPublicationSummary() when $default != null:
return $default(_that.publication,_that.sellerFacingPayload);case _:
  return null;

}
}

}

/// @nodoc


class _SellerPublicationSummary implements SellerPublicationSummary {
  const _SellerPublicationSummary({required this.publication, this.sellerFacingPayload = const SellerFacingPayload()});
  

@override final  SellerFacingPublication publication;
@override@JsonKey() final  SellerFacingPayload sellerFacingPayload;

/// Create a copy of SellerPublicationSummary
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerPublicationSummaryCopyWith<_SellerPublicationSummary> get copyWith => __$SellerPublicationSummaryCopyWithImpl<_SellerPublicationSummary>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerPublicationSummary&&(identical(other.publication, publication) || other.publication == publication)&&(identical(other.sellerFacingPayload, sellerFacingPayload) || other.sellerFacingPayload == sellerFacingPayload));
}


@override
int get hashCode => Object.hash(runtimeType,publication,sellerFacingPayload);

@override
String toString() {
  return 'SellerPublicationSummary(publication: $publication, sellerFacingPayload: $sellerFacingPayload)';
}


}

/// @nodoc
abstract mixin class _$SellerPublicationSummaryCopyWith<$Res> implements $SellerPublicationSummaryCopyWith<$Res> {
  factory _$SellerPublicationSummaryCopyWith(_SellerPublicationSummary value, $Res Function(_SellerPublicationSummary) _then) = __$SellerPublicationSummaryCopyWithImpl;
@override @useResult
$Res call({
 SellerFacingPublication publication, SellerFacingPayload sellerFacingPayload
});


@override $SellerFacingPublicationCopyWith<$Res> get publication;@override $SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload;

}
/// @nodoc
class __$SellerPublicationSummaryCopyWithImpl<$Res>
    implements _$SellerPublicationSummaryCopyWith<$Res> {
  __$SellerPublicationSummaryCopyWithImpl(this._self, this._then);

  final _SellerPublicationSummary _self;
  final $Res Function(_SellerPublicationSummary) _then;

/// Create a copy of SellerPublicationSummary
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? publication = null,Object? sellerFacingPayload = null,}) {
  return _then(_SellerPublicationSummary(
publication: null == publication ? _self.publication : publication // ignore: cast_nullable_to_non_nullable
as SellerFacingPublication,sellerFacingPayload: null == sellerFacingPayload ? _self.sellerFacingPayload : sellerFacingPayload // ignore: cast_nullable_to_non_nullable
as SellerFacingPayload,
  ));
}

/// Create a copy of SellerPublicationSummary
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPublicationCopyWith<$Res> get publication {
  
  return $SellerFacingPublicationCopyWith<$Res>(_self.publication, (value) {
    return _then(_self.copyWith(publication: value));
  });
}/// Create a copy of SellerPublicationSummary
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload {
  
  return $SellerFacingPayloadCopyWith<$Res>(_self.sellerFacingPayload, (value) {
    return _then(_self.copyWith(sellerFacingPayload: value));
  });
}
}

/// @nodoc
mixin _$SellerPublicationListItem {

 SellerPublicationSummary get summary; String get latestResponseStatus; bool get hasDraft;
/// Create a copy of SellerPublicationListItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerPublicationListItemCopyWith<SellerPublicationListItem> get copyWith => _$SellerPublicationListItemCopyWithImpl<SellerPublicationListItem>(this as SellerPublicationListItem, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerPublicationListItem&&(identical(other.summary, summary) || other.summary == summary)&&(identical(other.latestResponseStatus, latestResponseStatus) || other.latestResponseStatus == latestResponseStatus)&&(identical(other.hasDraft, hasDraft) || other.hasDraft == hasDraft));
}


@override
int get hashCode => Object.hash(runtimeType,summary,latestResponseStatus,hasDraft);

@override
String toString() {
  return 'SellerPublicationListItem(summary: $summary, latestResponseStatus: $latestResponseStatus, hasDraft: $hasDraft)';
}


}

/// @nodoc
abstract mixin class $SellerPublicationListItemCopyWith<$Res>  {
  factory $SellerPublicationListItemCopyWith(SellerPublicationListItem value, $Res Function(SellerPublicationListItem) _then) = _$SellerPublicationListItemCopyWithImpl;
@useResult
$Res call({
 SellerPublicationSummary summary, String latestResponseStatus, bool hasDraft
});


$SellerPublicationSummaryCopyWith<$Res> get summary;

}
/// @nodoc
class _$SellerPublicationListItemCopyWithImpl<$Res>
    implements $SellerPublicationListItemCopyWith<$Res> {
  _$SellerPublicationListItemCopyWithImpl(this._self, this._then);

  final SellerPublicationListItem _self;
  final $Res Function(SellerPublicationListItem) _then;

/// Create a copy of SellerPublicationListItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? summary = null,Object? latestResponseStatus = null,Object? hasDraft = null,}) {
  return _then(_self.copyWith(
summary: null == summary ? _self.summary : summary // ignore: cast_nullable_to_non_nullable
as SellerPublicationSummary,latestResponseStatus: null == latestResponseStatus ? _self.latestResponseStatus : latestResponseStatus // ignore: cast_nullable_to_non_nullable
as String,hasDraft: null == hasDraft ? _self.hasDraft : hasDraft // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}
/// Create a copy of SellerPublicationListItem
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerPublicationSummaryCopyWith<$Res> get summary {
  
  return $SellerPublicationSummaryCopyWith<$Res>(_self.summary, (value) {
    return _then(_self.copyWith(summary: value));
  });
}
}


/// Adds pattern-matching-related methods to [SellerPublicationListItem].
extension SellerPublicationListItemPatterns on SellerPublicationListItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerPublicationListItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerPublicationListItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerPublicationListItem value)  $default,){
final _that = this;
switch (_that) {
case _SellerPublicationListItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerPublicationListItem value)?  $default,){
final _that = this;
switch (_that) {
case _SellerPublicationListItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( SellerPublicationSummary summary,  String latestResponseStatus,  bool hasDraft)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerPublicationListItem() when $default != null:
return $default(_that.summary,_that.latestResponseStatus,_that.hasDraft);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( SellerPublicationSummary summary,  String latestResponseStatus,  bool hasDraft)  $default,) {final _that = this;
switch (_that) {
case _SellerPublicationListItem():
return $default(_that.summary,_that.latestResponseStatus,_that.hasDraft);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( SellerPublicationSummary summary,  String latestResponseStatus,  bool hasDraft)?  $default,) {final _that = this;
switch (_that) {
case _SellerPublicationListItem() when $default != null:
return $default(_that.summary,_that.latestResponseStatus,_that.hasDraft);case _:
  return null;

}
}

}

/// @nodoc


class _SellerPublicationListItem extends SellerPublicationListItem {
  const _SellerPublicationListItem({required this.summary, this.latestResponseStatus = '', this.hasDraft = false}): super._();
  

@override final  SellerPublicationSummary summary;
@override@JsonKey() final  String latestResponseStatus;
@override@JsonKey() final  bool hasDraft;

/// Create a copy of SellerPublicationListItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerPublicationListItemCopyWith<_SellerPublicationListItem> get copyWith => __$SellerPublicationListItemCopyWithImpl<_SellerPublicationListItem>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerPublicationListItem&&(identical(other.summary, summary) || other.summary == summary)&&(identical(other.latestResponseStatus, latestResponseStatus) || other.latestResponseStatus == latestResponseStatus)&&(identical(other.hasDraft, hasDraft) || other.hasDraft == hasDraft));
}


@override
int get hashCode => Object.hash(runtimeType,summary,latestResponseStatus,hasDraft);

@override
String toString() {
  return 'SellerPublicationListItem(summary: $summary, latestResponseStatus: $latestResponseStatus, hasDraft: $hasDraft)';
}


}

/// @nodoc
abstract mixin class _$SellerPublicationListItemCopyWith<$Res> implements $SellerPublicationListItemCopyWith<$Res> {
  factory _$SellerPublicationListItemCopyWith(_SellerPublicationListItem value, $Res Function(_SellerPublicationListItem) _then) = __$SellerPublicationListItemCopyWithImpl;
@override @useResult
$Res call({
 SellerPublicationSummary summary, String latestResponseStatus, bool hasDraft
});


@override $SellerPublicationSummaryCopyWith<$Res> get summary;

}
/// @nodoc
class __$SellerPublicationListItemCopyWithImpl<$Res>
    implements _$SellerPublicationListItemCopyWith<$Res> {
  __$SellerPublicationListItemCopyWithImpl(this._self, this._then);

  final _SellerPublicationListItem _self;
  final $Res Function(_SellerPublicationListItem) _then;

/// Create a copy of SellerPublicationListItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? summary = null,Object? latestResponseStatus = null,Object? hasDraft = null,}) {
  return _then(_SellerPublicationListItem(
summary: null == summary ? _self.summary : summary // ignore: cast_nullable_to_non_nullable
as SellerPublicationSummary,latestResponseStatus: null == latestResponseStatus ? _self.latestResponseStatus : latestResponseStatus // ignore: cast_nullable_to_non_nullable
as String,hasDraft: null == hasDraft ? _self.hasDraft : hasDraft // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

/// Create a copy of SellerPublicationListItem
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerPublicationSummaryCopyWith<$Res> get summary {
  
  return $SellerPublicationSummaryCopyWith<$Res>(_self.summary, (value) {
    return _then(_self.copyWith(summary: value));
  });
}
}

/// @nodoc
mixin _$SellerPublicationDetail {

 SellerFacingPublication get publication; SellerFacingPayload get sellerFacingPayload; SellerResponseDraftResponse? get draft; SellerResponseVersionResponse? get latestResponse;
/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerPublicationDetailCopyWith<SellerPublicationDetail> get copyWith => _$SellerPublicationDetailCopyWithImpl<SellerPublicationDetail>(this as SellerPublicationDetail, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerPublicationDetail&&(identical(other.publication, publication) || other.publication == publication)&&(identical(other.sellerFacingPayload, sellerFacingPayload) || other.sellerFacingPayload == sellerFacingPayload)&&(identical(other.draft, draft) || other.draft == draft)&&(identical(other.latestResponse, latestResponse) || other.latestResponse == latestResponse));
}


@override
int get hashCode => Object.hash(runtimeType,publication,sellerFacingPayload,draft,latestResponse);

@override
String toString() {
  return 'SellerPublicationDetail(publication: $publication, sellerFacingPayload: $sellerFacingPayload, draft: $draft, latestResponse: $latestResponse)';
}


}

/// @nodoc
abstract mixin class $SellerPublicationDetailCopyWith<$Res>  {
  factory $SellerPublicationDetailCopyWith(SellerPublicationDetail value, $Res Function(SellerPublicationDetail) _then) = _$SellerPublicationDetailCopyWithImpl;
@useResult
$Res call({
 SellerFacingPublication publication, SellerFacingPayload sellerFacingPayload, SellerResponseDraftResponse? draft, SellerResponseVersionResponse? latestResponse
});


$SellerFacingPublicationCopyWith<$Res> get publication;$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload;$SellerResponseDraftResponseCopyWith<$Res>? get draft;$SellerResponseVersionResponseCopyWith<$Res>? get latestResponse;

}
/// @nodoc
class _$SellerPublicationDetailCopyWithImpl<$Res>
    implements $SellerPublicationDetailCopyWith<$Res> {
  _$SellerPublicationDetailCopyWithImpl(this._self, this._then);

  final SellerPublicationDetail _self;
  final $Res Function(SellerPublicationDetail) _then;

/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? publication = null,Object? sellerFacingPayload = null,Object? draft = freezed,Object? latestResponse = freezed,}) {
  return _then(_self.copyWith(
publication: null == publication ? _self.publication : publication // ignore: cast_nullable_to_non_nullable
as SellerFacingPublication,sellerFacingPayload: null == sellerFacingPayload ? _self.sellerFacingPayload : sellerFacingPayload // ignore: cast_nullable_to_non_nullable
as SellerFacingPayload,draft: freezed == draft ? _self.draft : draft // ignore: cast_nullable_to_non_nullable
as SellerResponseDraftResponse?,latestResponse: freezed == latestResponse ? _self.latestResponse : latestResponse // ignore: cast_nullable_to_non_nullable
as SellerResponseVersionResponse?,
  ));
}
/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPublicationCopyWith<$Res> get publication {
  
  return $SellerFacingPublicationCopyWith<$Res>(_self.publication, (value) {
    return _then(_self.copyWith(publication: value));
  });
}/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload {
  
  return $SellerFacingPayloadCopyWith<$Res>(_self.sellerFacingPayload, (value) {
    return _then(_self.copyWith(sellerFacingPayload: value));
  });
}/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerResponseDraftResponseCopyWith<$Res>? get draft {
    if (_self.draft == null) {
    return null;
  }

  return $SellerResponseDraftResponseCopyWith<$Res>(_self.draft!, (value) {
    return _then(_self.copyWith(draft: value));
  });
}/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerResponseVersionResponseCopyWith<$Res>? get latestResponse {
    if (_self.latestResponse == null) {
    return null;
  }

  return $SellerResponseVersionResponseCopyWith<$Res>(_self.latestResponse!, (value) {
    return _then(_self.copyWith(latestResponse: value));
  });
}
}


/// Adds pattern-matching-related methods to [SellerPublicationDetail].
extension SellerPublicationDetailPatterns on SellerPublicationDetail {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerPublicationDetail value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerPublicationDetail() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerPublicationDetail value)  $default,){
final _that = this;
switch (_that) {
case _SellerPublicationDetail():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerPublicationDetail value)?  $default,){
final _that = this;
switch (_that) {
case _SellerPublicationDetail() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload,  SellerResponseDraftResponse? draft,  SellerResponseVersionResponse? latestResponse)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerPublicationDetail() when $default != null:
return $default(_that.publication,_that.sellerFacingPayload,_that.draft,_that.latestResponse);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload,  SellerResponseDraftResponse? draft,  SellerResponseVersionResponse? latestResponse)  $default,) {final _that = this;
switch (_that) {
case _SellerPublicationDetail():
return $default(_that.publication,_that.sellerFacingPayload,_that.draft,_that.latestResponse);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload,  SellerResponseDraftResponse? draft,  SellerResponseVersionResponse? latestResponse)?  $default,) {final _that = this;
switch (_that) {
case _SellerPublicationDetail() when $default != null:
return $default(_that.publication,_that.sellerFacingPayload,_that.draft,_that.latestResponse);case _:
  return null;

}
}

}

/// @nodoc


class _SellerPublicationDetail extends SellerPublicationDetail {
  const _SellerPublicationDetail({required this.publication, this.sellerFacingPayload = const SellerFacingPayload(), this.draft, this.latestResponse}): super._();
  

@override final  SellerFacingPublication publication;
@override@JsonKey() final  SellerFacingPayload sellerFacingPayload;
@override final  SellerResponseDraftResponse? draft;
@override final  SellerResponseVersionResponse? latestResponse;

/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerPublicationDetailCopyWith<_SellerPublicationDetail> get copyWith => __$SellerPublicationDetailCopyWithImpl<_SellerPublicationDetail>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerPublicationDetail&&(identical(other.publication, publication) || other.publication == publication)&&(identical(other.sellerFacingPayload, sellerFacingPayload) || other.sellerFacingPayload == sellerFacingPayload)&&(identical(other.draft, draft) || other.draft == draft)&&(identical(other.latestResponse, latestResponse) || other.latestResponse == latestResponse));
}


@override
int get hashCode => Object.hash(runtimeType,publication,sellerFacingPayload,draft,latestResponse);

@override
String toString() {
  return 'SellerPublicationDetail(publication: $publication, sellerFacingPayload: $sellerFacingPayload, draft: $draft, latestResponse: $latestResponse)';
}


}

/// @nodoc
abstract mixin class _$SellerPublicationDetailCopyWith<$Res> implements $SellerPublicationDetailCopyWith<$Res> {
  factory _$SellerPublicationDetailCopyWith(_SellerPublicationDetail value, $Res Function(_SellerPublicationDetail) _then) = __$SellerPublicationDetailCopyWithImpl;
@override @useResult
$Res call({
 SellerFacingPublication publication, SellerFacingPayload sellerFacingPayload, SellerResponseDraftResponse? draft, SellerResponseVersionResponse? latestResponse
});


@override $SellerFacingPublicationCopyWith<$Res> get publication;@override $SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload;@override $SellerResponseDraftResponseCopyWith<$Res>? get draft;@override $SellerResponseVersionResponseCopyWith<$Res>? get latestResponse;

}
/// @nodoc
class __$SellerPublicationDetailCopyWithImpl<$Res>
    implements _$SellerPublicationDetailCopyWith<$Res> {
  __$SellerPublicationDetailCopyWithImpl(this._self, this._then);

  final _SellerPublicationDetail _self;
  final $Res Function(_SellerPublicationDetail) _then;

/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? publication = null,Object? sellerFacingPayload = null,Object? draft = freezed,Object? latestResponse = freezed,}) {
  return _then(_SellerPublicationDetail(
publication: null == publication ? _self.publication : publication // ignore: cast_nullable_to_non_nullable
as SellerFacingPublication,sellerFacingPayload: null == sellerFacingPayload ? _self.sellerFacingPayload : sellerFacingPayload // ignore: cast_nullable_to_non_nullable
as SellerFacingPayload,draft: freezed == draft ? _self.draft : draft // ignore: cast_nullable_to_non_nullable
as SellerResponseDraftResponse?,latestResponse: freezed == latestResponse ? _self.latestResponse : latestResponse // ignore: cast_nullable_to_non_nullable
as SellerResponseVersionResponse?,
  ));
}

/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPublicationCopyWith<$Res> get publication {
  
  return $SellerFacingPublicationCopyWith<$Res>(_self.publication, (value) {
    return _then(_self.copyWith(publication: value));
  });
}/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload {
  
  return $SellerFacingPayloadCopyWith<$Res>(_self.sellerFacingPayload, (value) {
    return _then(_self.copyWith(sellerFacingPayload: value));
  });
}/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerResponseDraftResponseCopyWith<$Res>? get draft {
    if (_self.draft == null) {
    return null;
  }

  return $SellerResponseDraftResponseCopyWith<$Res>(_self.draft!, (value) {
    return _then(_self.copyWith(draft: value));
  });
}/// Create a copy of SellerPublicationDetail
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerResponseVersionResponseCopyWith<$Res>? get latestResponse {
    if (_self.latestResponse == null) {
    return null;
  }

  return $SellerResponseVersionResponseCopyWith<$Res>(_self.latestResponse!, (value) {
    return _then(_self.copyWith(latestResponse: value));
  });
}
}

// dart format on
