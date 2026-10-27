// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'product_listing_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$PendingAiDraft {

 String? get name; String? get origin; String? get categoryId; String? get description; List<String> get tags; List<SpecDraftPending> get skus; List<CertificationPending> get certifications; Set<String> get filledFields; int get patchVersion;
/// Create a copy of PendingAiDraft
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PendingAiDraftCopyWith<PendingAiDraft> get copyWith => _$PendingAiDraftCopyWithImpl<PendingAiDraft>(this as PendingAiDraft, _$identity);

  /// Serializes this PendingAiDraft to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is PendingAiDraft&&(identical(other.name, name) || other.name == name)&&(identical(other.origin, origin) || other.origin == origin)&&(identical(other.categoryId, categoryId) || other.categoryId == categoryId)&&(identical(other.description, description) || other.description == description)&&const DeepCollectionEquality().equals(other.tags, tags)&&const DeepCollectionEquality().equals(other.skus, skus)&&const DeepCollectionEquality().equals(other.certifications, certifications)&&const DeepCollectionEquality().equals(other.filledFields, filledFields)&&(identical(other.patchVersion, patchVersion) || other.patchVersion == patchVersion));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,name,origin,categoryId,description,const DeepCollectionEquality().hash(tags),const DeepCollectionEquality().hash(skus),const DeepCollectionEquality().hash(certifications),const DeepCollectionEquality().hash(filledFields),patchVersion);

@override
String toString() {
  return 'PendingAiDraft(name: $name, origin: $origin, categoryId: $categoryId, description: $description, tags: $tags, skus: $skus, certifications: $certifications, filledFields: $filledFields, patchVersion: $patchVersion)';
}


}

/// @nodoc
abstract mixin class $PendingAiDraftCopyWith<$Res>  {
  factory $PendingAiDraftCopyWith(PendingAiDraft value, $Res Function(PendingAiDraft) _then) = _$PendingAiDraftCopyWithImpl;
@useResult
$Res call({
 String? name, String? origin, String? categoryId, String? description, List<String> tags, List<SpecDraftPending> skus, List<CertificationPending> certifications, Set<String> filledFields, int patchVersion
});




}
/// @nodoc
class _$PendingAiDraftCopyWithImpl<$Res>
    implements $PendingAiDraftCopyWith<$Res> {
  _$PendingAiDraftCopyWithImpl(this._self, this._then);

  final PendingAiDraft _self;
  final $Res Function(PendingAiDraft) _then;

/// Create a copy of PendingAiDraft
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? name = freezed,Object? origin = freezed,Object? categoryId = freezed,Object? description = freezed,Object? tags = null,Object? skus = null,Object? certifications = null,Object? filledFields = null,Object? patchVersion = null,}) {
  return _then(_self.copyWith(
name: freezed == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String?,origin: freezed == origin ? _self.origin : origin // ignore: cast_nullable_to_non_nullable
as String?,categoryId: freezed == categoryId ? _self.categoryId : categoryId // ignore: cast_nullable_to_non_nullable
as String?,description: freezed == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String?,tags: null == tags ? _self.tags : tags // ignore: cast_nullable_to_non_nullable
as List<String>,skus: null == skus ? _self.skus : skus // ignore: cast_nullable_to_non_nullable
as List<SpecDraftPending>,certifications: null == certifications ? _self.certifications : certifications // ignore: cast_nullable_to_non_nullable
as List<CertificationPending>,filledFields: null == filledFields ? _self.filledFields : filledFields // ignore: cast_nullable_to_non_nullable
as Set<String>,patchVersion: null == patchVersion ? _self.patchVersion : patchVersion // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [PendingAiDraft].
extension PendingAiDraftPatterns on PendingAiDraft {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _PendingAiDraft value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _PendingAiDraft() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _PendingAiDraft value)  $default,){
final _that = this;
switch (_that) {
case _PendingAiDraft():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _PendingAiDraft value)?  $default,){
final _that = this;
switch (_that) {
case _PendingAiDraft() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String? name,  String? origin,  String? categoryId,  String? description,  List<String> tags,  List<SpecDraftPending> skus,  List<CertificationPending> certifications,  Set<String> filledFields,  int patchVersion)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _PendingAiDraft() when $default != null:
return $default(_that.name,_that.origin,_that.categoryId,_that.description,_that.tags,_that.skus,_that.certifications,_that.filledFields,_that.patchVersion);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String? name,  String? origin,  String? categoryId,  String? description,  List<String> tags,  List<SpecDraftPending> skus,  List<CertificationPending> certifications,  Set<String> filledFields,  int patchVersion)  $default,) {final _that = this;
switch (_that) {
case _PendingAiDraft():
return $default(_that.name,_that.origin,_that.categoryId,_that.description,_that.tags,_that.skus,_that.certifications,_that.filledFields,_that.patchVersion);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String? name,  String? origin,  String? categoryId,  String? description,  List<String> tags,  List<SpecDraftPending> skus,  List<CertificationPending> certifications,  Set<String> filledFields,  int patchVersion)?  $default,) {final _that = this;
switch (_that) {
case _PendingAiDraft() when $default != null:
return $default(_that.name,_that.origin,_that.categoryId,_that.description,_that.tags,_that.skus,_that.certifications,_that.filledFields,_that.patchVersion);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _PendingAiDraft implements PendingAiDraft {
  const _PendingAiDraft({this.name, this.origin, this.categoryId, this.description, final  List<String> tags = const [], final  List<SpecDraftPending> skus = const [], final  List<CertificationPending> certifications = const [], final  Set<String> filledFields = const {}, this.patchVersion = 0}): _tags = tags,_skus = skus,_certifications = certifications,_filledFields = filledFields;
  factory _PendingAiDraft.fromJson(Map<String, dynamic> json) => _$PendingAiDraftFromJson(json);

@override final  String? name;
@override final  String? origin;
@override final  String? categoryId;
@override final  String? description;
 final  List<String> _tags;
@override@JsonKey() List<String> get tags {
  if (_tags is EqualUnmodifiableListView) return _tags;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_tags);
}

 final  List<SpecDraftPending> _skus;
@override@JsonKey() List<SpecDraftPending> get skus {
  if (_skus is EqualUnmodifiableListView) return _skus;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_skus);
}

 final  List<CertificationPending> _certifications;
@override@JsonKey() List<CertificationPending> get certifications {
  if (_certifications is EqualUnmodifiableListView) return _certifications;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_certifications);
}

 final  Set<String> _filledFields;
@override@JsonKey() Set<String> get filledFields {
  if (_filledFields is EqualUnmodifiableSetView) return _filledFields;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableSetView(_filledFields);
}

@override@JsonKey() final  int patchVersion;

/// Create a copy of PendingAiDraft
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PendingAiDraftCopyWith<_PendingAiDraft> get copyWith => __$PendingAiDraftCopyWithImpl<_PendingAiDraft>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$PendingAiDraftToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _PendingAiDraft&&(identical(other.name, name) || other.name == name)&&(identical(other.origin, origin) || other.origin == origin)&&(identical(other.categoryId, categoryId) || other.categoryId == categoryId)&&(identical(other.description, description) || other.description == description)&&const DeepCollectionEquality().equals(other._tags, _tags)&&const DeepCollectionEquality().equals(other._skus, _skus)&&const DeepCollectionEquality().equals(other._certifications, _certifications)&&const DeepCollectionEquality().equals(other._filledFields, _filledFields)&&(identical(other.patchVersion, patchVersion) || other.patchVersion == patchVersion));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,name,origin,categoryId,description,const DeepCollectionEquality().hash(_tags),const DeepCollectionEquality().hash(_skus),const DeepCollectionEquality().hash(_certifications),const DeepCollectionEquality().hash(_filledFields),patchVersion);

@override
String toString() {
  return 'PendingAiDraft(name: $name, origin: $origin, categoryId: $categoryId, description: $description, tags: $tags, skus: $skus, certifications: $certifications, filledFields: $filledFields, patchVersion: $patchVersion)';
}


}

/// @nodoc
abstract mixin class _$PendingAiDraftCopyWith<$Res> implements $PendingAiDraftCopyWith<$Res> {
  factory _$PendingAiDraftCopyWith(_PendingAiDraft value, $Res Function(_PendingAiDraft) _then) = __$PendingAiDraftCopyWithImpl;
@override @useResult
$Res call({
 String? name, String? origin, String? categoryId, String? description, List<String> tags, List<SpecDraftPending> skus, List<CertificationPending> certifications, Set<String> filledFields, int patchVersion
});




}
/// @nodoc
class __$PendingAiDraftCopyWithImpl<$Res>
    implements _$PendingAiDraftCopyWith<$Res> {
  __$PendingAiDraftCopyWithImpl(this._self, this._then);

  final _PendingAiDraft _self;
  final $Res Function(_PendingAiDraft) _then;

/// Create a copy of PendingAiDraft
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? name = freezed,Object? origin = freezed,Object? categoryId = freezed,Object? description = freezed,Object? tags = null,Object? skus = null,Object? certifications = null,Object? filledFields = null,Object? patchVersion = null,}) {
  return _then(_PendingAiDraft(
name: freezed == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String?,origin: freezed == origin ? _self.origin : origin // ignore: cast_nullable_to_non_nullable
as String?,categoryId: freezed == categoryId ? _self.categoryId : categoryId // ignore: cast_nullable_to_non_nullable
as String?,description: freezed == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String?,tags: null == tags ? _self._tags : tags // ignore: cast_nullable_to_non_nullable
as List<String>,skus: null == skus ? _self._skus : skus // ignore: cast_nullable_to_non_nullable
as List<SpecDraftPending>,certifications: null == certifications ? _self._certifications : certifications // ignore: cast_nullable_to_non_nullable
as List<CertificationPending>,filledFields: null == filledFields ? _self._filledFields : filledFields // ignore: cast_nullable_to_non_nullable
as Set<String>,patchVersion: null == patchVersion ? _self.patchVersion : patchVersion // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}


/// @nodoc
mixin _$SpecDraftPending {

 String get skuName; int get price;// PayUni #758: AI 助手可建議特價（0/空=無特價）
 int get salePrice; String get unit; int get weight; int get stockQty;
/// Create a copy of SpecDraftPending
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SpecDraftPendingCopyWith<SpecDraftPending> get copyWith => _$SpecDraftPendingCopyWithImpl<SpecDraftPending>(this as SpecDraftPending, _$identity);

  /// Serializes this SpecDraftPending to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SpecDraftPending&&(identical(other.skuName, skuName) || other.skuName == skuName)&&(identical(other.price, price) || other.price == price)&&(identical(other.salePrice, salePrice) || other.salePrice == salePrice)&&(identical(other.unit, unit) || other.unit == unit)&&(identical(other.weight, weight) || other.weight == weight)&&(identical(other.stockQty, stockQty) || other.stockQty == stockQty));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,skuName,price,salePrice,unit,weight,stockQty);

@override
String toString() {
  return 'SpecDraftPending(skuName: $skuName, price: $price, salePrice: $salePrice, unit: $unit, weight: $weight, stockQty: $stockQty)';
}


}

/// @nodoc
abstract mixin class $SpecDraftPendingCopyWith<$Res>  {
  factory $SpecDraftPendingCopyWith(SpecDraftPending value, $Res Function(SpecDraftPending) _then) = _$SpecDraftPendingCopyWithImpl;
@useResult
$Res call({
 String skuName, int price, int salePrice, String unit, int weight, int stockQty
});




}
/// @nodoc
class _$SpecDraftPendingCopyWithImpl<$Res>
    implements $SpecDraftPendingCopyWith<$Res> {
  _$SpecDraftPendingCopyWithImpl(this._self, this._then);

  final SpecDraftPending _self;
  final $Res Function(SpecDraftPending) _then;

/// Create a copy of SpecDraftPending
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? skuName = null,Object? price = null,Object? salePrice = null,Object? unit = null,Object? weight = null,Object? stockQty = null,}) {
  return _then(_self.copyWith(
skuName: null == skuName ? _self.skuName : skuName // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,salePrice: null == salePrice ? _self.salePrice : salePrice // ignore: cast_nullable_to_non_nullable
as int,unit: null == unit ? _self.unit : unit // ignore: cast_nullable_to_non_nullable
as String,weight: null == weight ? _self.weight : weight // ignore: cast_nullable_to_non_nullable
as int,stockQty: null == stockQty ? _self.stockQty : stockQty // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [SpecDraftPending].
extension SpecDraftPendingPatterns on SpecDraftPending {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SpecDraftPending value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SpecDraftPending() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SpecDraftPending value)  $default,){
final _that = this;
switch (_that) {
case _SpecDraftPending():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SpecDraftPending value)?  $default,){
final _that = this;
switch (_that) {
case _SpecDraftPending() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String skuName,  int price,  int salePrice,  String unit,  int weight,  int stockQty)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SpecDraftPending() when $default != null:
return $default(_that.skuName,_that.price,_that.salePrice,_that.unit,_that.weight,_that.stockQty);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String skuName,  int price,  int salePrice,  String unit,  int weight,  int stockQty)  $default,) {final _that = this;
switch (_that) {
case _SpecDraftPending():
return $default(_that.skuName,_that.price,_that.salePrice,_that.unit,_that.weight,_that.stockQty);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String skuName,  int price,  int salePrice,  String unit,  int weight,  int stockQty)?  $default,) {final _that = this;
switch (_that) {
case _SpecDraftPending() when $default != null:
return $default(_that.skuName,_that.price,_that.salePrice,_that.unit,_that.weight,_that.stockQty);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SpecDraftPending implements SpecDraftPending {
  const _SpecDraftPending({required this.skuName, this.price = 0, this.salePrice = 0, this.unit = '', this.weight = 0, this.stockQty = 0});
  factory _SpecDraftPending.fromJson(Map<String, dynamic> json) => _$SpecDraftPendingFromJson(json);

@override final  String skuName;
@override@JsonKey() final  int price;
// PayUni #758: AI 助手可建議特價（0/空=無特價）
@override@JsonKey() final  int salePrice;
@override@JsonKey() final  String unit;
@override@JsonKey() final  int weight;
@override@JsonKey() final  int stockQty;

/// Create a copy of SpecDraftPending
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SpecDraftPendingCopyWith<_SpecDraftPending> get copyWith => __$SpecDraftPendingCopyWithImpl<_SpecDraftPending>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SpecDraftPendingToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SpecDraftPending&&(identical(other.skuName, skuName) || other.skuName == skuName)&&(identical(other.price, price) || other.price == price)&&(identical(other.salePrice, salePrice) || other.salePrice == salePrice)&&(identical(other.unit, unit) || other.unit == unit)&&(identical(other.weight, weight) || other.weight == weight)&&(identical(other.stockQty, stockQty) || other.stockQty == stockQty));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,skuName,price,salePrice,unit,weight,stockQty);

@override
String toString() {
  return 'SpecDraftPending(skuName: $skuName, price: $price, salePrice: $salePrice, unit: $unit, weight: $weight, stockQty: $stockQty)';
}


}

/// @nodoc
abstract mixin class _$SpecDraftPendingCopyWith<$Res> implements $SpecDraftPendingCopyWith<$Res> {
  factory _$SpecDraftPendingCopyWith(_SpecDraftPending value, $Res Function(_SpecDraftPending) _then) = __$SpecDraftPendingCopyWithImpl;
@override @useResult
$Res call({
 String skuName, int price, int salePrice, String unit, int weight, int stockQty
});




}
/// @nodoc
class __$SpecDraftPendingCopyWithImpl<$Res>
    implements _$SpecDraftPendingCopyWith<$Res> {
  __$SpecDraftPendingCopyWithImpl(this._self, this._then);

  final _SpecDraftPending _self;
  final $Res Function(_SpecDraftPending) _then;

/// Create a copy of SpecDraftPending
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? skuName = null,Object? price = null,Object? salePrice = null,Object? unit = null,Object? weight = null,Object? stockQty = null,}) {
  return _then(_SpecDraftPending(
skuName: null == skuName ? _self.skuName : skuName // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,salePrice: null == salePrice ? _self.salePrice : salePrice // ignore: cast_nullable_to_non_nullable
as int,unit: null == unit ? _self.unit : unit // ignore: cast_nullable_to_non_nullable
as String,weight: null == weight ? _self.weight : weight // ignore: cast_nullable_to_non_nullable
as int,stockQty: null == stockQty ? _self.stockQty : stockQty // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}


/// @nodoc
mixin _$CertificationPending {

 String get name; String get certNo;
/// Create a copy of CertificationPending
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CertificationPendingCopyWith<CertificationPending> get copyWith => _$CertificationPendingCopyWithImpl<CertificationPending>(this as CertificationPending, _$identity);

  /// Serializes this CertificationPending to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CertificationPending&&(identical(other.name, name) || other.name == name)&&(identical(other.certNo, certNo) || other.certNo == certNo));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,name,certNo);

@override
String toString() {
  return 'CertificationPending(name: $name, certNo: $certNo)';
}


}

/// @nodoc
abstract mixin class $CertificationPendingCopyWith<$Res>  {
  factory $CertificationPendingCopyWith(CertificationPending value, $Res Function(CertificationPending) _then) = _$CertificationPendingCopyWithImpl;
@useResult
$Res call({
 String name, String certNo
});




}
/// @nodoc
class _$CertificationPendingCopyWithImpl<$Res>
    implements $CertificationPendingCopyWith<$Res> {
  _$CertificationPendingCopyWithImpl(this._self, this._then);

  final CertificationPending _self;
  final $Res Function(CertificationPending) _then;

/// Create a copy of CertificationPending
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? name = null,Object? certNo = null,}) {
  return _then(_self.copyWith(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,certNo: null == certNo ? _self.certNo : certNo // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [CertificationPending].
extension CertificationPendingPatterns on CertificationPending {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _CertificationPending value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _CertificationPending() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _CertificationPending value)  $default,){
final _that = this;
switch (_that) {
case _CertificationPending():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _CertificationPending value)?  $default,){
final _that = this;
switch (_that) {
case _CertificationPending() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String name,  String certNo)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _CertificationPending() when $default != null:
return $default(_that.name,_that.certNo);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String name,  String certNo)  $default,) {final _that = this;
switch (_that) {
case _CertificationPending():
return $default(_that.name,_that.certNo);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String name,  String certNo)?  $default,) {final _that = this;
switch (_that) {
case _CertificationPending() when $default != null:
return $default(_that.name,_that.certNo);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _CertificationPending implements CertificationPending {
  const _CertificationPending({required this.name, required this.certNo});
  factory _CertificationPending.fromJson(Map<String, dynamic> json) => _$CertificationPendingFromJson(json);

@override final  String name;
@override final  String certNo;

/// Create a copy of CertificationPending
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CertificationPendingCopyWith<_CertificationPending> get copyWith => __$CertificationPendingCopyWithImpl<_CertificationPending>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$CertificationPendingToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _CertificationPending&&(identical(other.name, name) || other.name == name)&&(identical(other.certNo, certNo) || other.certNo == certNo));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,name,certNo);

@override
String toString() {
  return 'CertificationPending(name: $name, certNo: $certNo)';
}


}

/// @nodoc
abstract mixin class _$CertificationPendingCopyWith<$Res> implements $CertificationPendingCopyWith<$Res> {
  factory _$CertificationPendingCopyWith(_CertificationPending value, $Res Function(_CertificationPending) _then) = __$CertificationPendingCopyWithImpl;
@override @useResult
$Res call({
 String name, String certNo
});




}
/// @nodoc
class __$CertificationPendingCopyWithImpl<$Res>
    implements _$CertificationPendingCopyWith<$Res> {
  __$CertificationPendingCopyWithImpl(this._self, this._then);

  final _CertificationPending _self;
  final $Res Function(_CertificationPending) _then;

/// Create a copy of CertificationPending
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? name = null,Object? certNo = null,}) {
  return _then(_CertificationPending(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,certNo: null == certNo ? _self.certNo : certNo // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$ChatMessage {

 String get role;// 'user' | 'assistant'
 String get text;
/// Create a copy of ChatMessage
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ChatMessageCopyWith<ChatMessage> get copyWith => _$ChatMessageCopyWithImpl<ChatMessage>(this as ChatMessage, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ChatMessage&&(identical(other.role, role) || other.role == role)&&(identical(other.text, text) || other.text == text));
}


@override
int get hashCode => Object.hash(runtimeType,role,text);

@override
String toString() {
  return 'ChatMessage(role: $role, text: $text)';
}


}

/// @nodoc
abstract mixin class $ChatMessageCopyWith<$Res>  {
  factory $ChatMessageCopyWith(ChatMessage value, $Res Function(ChatMessage) _then) = _$ChatMessageCopyWithImpl;
@useResult
$Res call({
 String role, String text
});




}
/// @nodoc
class _$ChatMessageCopyWithImpl<$Res>
    implements $ChatMessageCopyWith<$Res> {
  _$ChatMessageCopyWithImpl(this._self, this._then);

  final ChatMessage _self;
  final $Res Function(ChatMessage) _then;

/// Create a copy of ChatMessage
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? role = null,Object? text = null,}) {
  return _then(_self.copyWith(
role: null == role ? _self.role : role // ignore: cast_nullable_to_non_nullable
as String,text: null == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [ChatMessage].
extension ChatMessagePatterns on ChatMessage {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ChatMessage value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ChatMessage() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ChatMessage value)  $default,){
final _that = this;
switch (_that) {
case _ChatMessage():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ChatMessage value)?  $default,){
final _that = this;
switch (_that) {
case _ChatMessage() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String role,  String text)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ChatMessage() when $default != null:
return $default(_that.role,_that.text);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String role,  String text)  $default,) {final _that = this;
switch (_that) {
case _ChatMessage():
return $default(_that.role,_that.text);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String role,  String text)?  $default,) {final _that = this;
switch (_that) {
case _ChatMessage() when $default != null:
return $default(_that.role,_that.text);case _:
  return null;

}
}

}

/// @nodoc


class _ChatMessage implements ChatMessage {
  const _ChatMessage({required this.role, this.text = ''});
  

@override final  String role;
// 'user' | 'assistant'
@override@JsonKey() final  String text;

/// Create a copy of ChatMessage
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ChatMessageCopyWith<_ChatMessage> get copyWith => __$ChatMessageCopyWithImpl<_ChatMessage>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ChatMessage&&(identical(other.role, role) || other.role == role)&&(identical(other.text, text) || other.text == text));
}


@override
int get hashCode => Object.hash(runtimeType,role,text);

@override
String toString() {
  return 'ChatMessage(role: $role, text: $text)';
}


}

/// @nodoc
abstract mixin class _$ChatMessageCopyWith<$Res> implements $ChatMessageCopyWith<$Res> {
  factory _$ChatMessageCopyWith(_ChatMessage value, $Res Function(_ChatMessage) _then) = __$ChatMessageCopyWithImpl;
@override @useResult
$Res call({
 String role, String text
});




}
/// @nodoc
class __$ChatMessageCopyWithImpl<$Res>
    implements _$ChatMessageCopyWith<$Res> {
  __$ChatMessageCopyWithImpl(this._self, this._then);

  final _ChatMessage _self;
  final $Res Function(_ChatMessage) _then;

/// Create a copy of ChatMessage
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? role = null,Object? text = null,}) {
  return _then(_ChatMessage(
role: null == role ? _self.role : role // ignore: cast_nullable_to_non_nullable
as String,text: null == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
