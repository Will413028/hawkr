// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'ai_style_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$AiStyle {

 String get id; String get name; String get label; String get prompt; String get thumbnailKey; String get sortOrder;
/// Create a copy of AiStyle
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AiStyleCopyWith<AiStyle> get copyWith => _$AiStyleCopyWithImpl<AiStyle>(this as AiStyle, _$identity);

  /// Serializes this AiStyle to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AiStyle&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.label, label) || other.label == label)&&(identical(other.prompt, prompt) || other.prompt == prompt)&&(identical(other.thumbnailKey, thumbnailKey) || other.thumbnailKey == thumbnailKey)&&(identical(other.sortOrder, sortOrder) || other.sortOrder == sortOrder));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,name,label,prompt,thumbnailKey,sortOrder);

@override
String toString() {
  return 'AiStyle(id: $id, name: $name, label: $label, prompt: $prompt, thumbnailKey: $thumbnailKey, sortOrder: $sortOrder)';
}


}

/// @nodoc
abstract mixin class $AiStyleCopyWith<$Res>  {
  factory $AiStyleCopyWith(AiStyle value, $Res Function(AiStyle) _then) = _$AiStyleCopyWithImpl;
@useResult
$Res call({
 String id, String name, String label, String prompt, String thumbnailKey, String sortOrder
});




}
/// @nodoc
class _$AiStyleCopyWithImpl<$Res>
    implements $AiStyleCopyWith<$Res> {
  _$AiStyleCopyWithImpl(this._self, this._then);

  final AiStyle _self;
  final $Res Function(AiStyle) _then;

/// Create a copy of AiStyle
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? name = null,Object? label = null,Object? prompt = null,Object? thumbnailKey = null,Object? sortOrder = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,label: null == label ? _self.label : label // ignore: cast_nullable_to_non_nullable
as String,prompt: null == prompt ? _self.prompt : prompt // ignore: cast_nullable_to_non_nullable
as String,thumbnailKey: null == thumbnailKey ? _self.thumbnailKey : thumbnailKey // ignore: cast_nullable_to_non_nullable
as String,sortOrder: null == sortOrder ? _self.sortOrder : sortOrder // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AiStyle].
extension AiStylePatterns on AiStyle {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AiStyle value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AiStyle() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AiStyle value)  $default,){
final _that = this;
switch (_that) {
case _AiStyle():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AiStyle value)?  $default,){
final _that = this;
switch (_that) {
case _AiStyle() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String name,  String label,  String prompt,  String thumbnailKey,  String sortOrder)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AiStyle() when $default != null:
return $default(_that.id,_that.name,_that.label,_that.prompt,_that.thumbnailKey,_that.sortOrder);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String name,  String label,  String prompt,  String thumbnailKey,  String sortOrder)  $default,) {final _that = this;
switch (_that) {
case _AiStyle():
return $default(_that.id,_that.name,_that.label,_that.prompt,_that.thumbnailKey,_that.sortOrder);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String name,  String label,  String prompt,  String thumbnailKey,  String sortOrder)?  $default,) {final _that = this;
switch (_that) {
case _AiStyle() when $default != null:
return $default(_that.id,_that.name,_that.label,_that.prompt,_that.thumbnailKey,_that.sortOrder);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AiStyle implements AiStyle {
  const _AiStyle({required this.id, required this.name, required this.label, required this.prompt, this.thumbnailKey = '', this.sortOrder = '0'});
  factory _AiStyle.fromJson(Map<String, dynamic> json) => _$AiStyleFromJson(json);

@override final  String id;
@override final  String name;
@override final  String label;
@override final  String prompt;
@override@JsonKey() final  String thumbnailKey;
@override@JsonKey() final  String sortOrder;

/// Create a copy of AiStyle
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AiStyleCopyWith<_AiStyle> get copyWith => __$AiStyleCopyWithImpl<_AiStyle>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$AiStyleToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AiStyle&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.label, label) || other.label == label)&&(identical(other.prompt, prompt) || other.prompt == prompt)&&(identical(other.thumbnailKey, thumbnailKey) || other.thumbnailKey == thumbnailKey)&&(identical(other.sortOrder, sortOrder) || other.sortOrder == sortOrder));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,name,label,prompt,thumbnailKey,sortOrder);

@override
String toString() {
  return 'AiStyle(id: $id, name: $name, label: $label, prompt: $prompt, thumbnailKey: $thumbnailKey, sortOrder: $sortOrder)';
}


}

/// @nodoc
abstract mixin class _$AiStyleCopyWith<$Res> implements $AiStyleCopyWith<$Res> {
  factory _$AiStyleCopyWith(_AiStyle value, $Res Function(_AiStyle) _then) = __$AiStyleCopyWithImpl;
@override @useResult
$Res call({
 String id, String name, String label, String prompt, String thumbnailKey, String sortOrder
});




}
/// @nodoc
class __$AiStyleCopyWithImpl<$Res>
    implements _$AiStyleCopyWith<$Res> {
  __$AiStyleCopyWithImpl(this._self, this._then);

  final _AiStyle _self;
  final $Res Function(_AiStyle) _then;

/// Create a copy of AiStyle
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? name = null,Object? label = null,Object? prompt = null,Object? thumbnailKey = null,Object? sortOrder = null,}) {
  return _then(_AiStyle(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,label: null == label ? _self.label : label // ignore: cast_nullable_to_non_nullable
as String,prompt: null == prompt ? _self.prompt : prompt // ignore: cast_nullable_to_non_nullable
as String,thumbnailKey: null == thumbnailKey ? _self.thumbnailKey : thumbnailKey // ignore: cast_nullable_to_non_nullable
as String,sortOrder: null == sortOrder ? _self.sortOrder : sortOrder // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$AiStylesResponse {

 List<AiStyle> get styles; String get defaultPrompt; String get weeklyUsage; String get weeklyLimit;
/// Create a copy of AiStylesResponse
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AiStylesResponseCopyWith<AiStylesResponse> get copyWith => _$AiStylesResponseCopyWithImpl<AiStylesResponse>(this as AiStylesResponse, _$identity);

  /// Serializes this AiStylesResponse to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AiStylesResponse&&const DeepCollectionEquality().equals(other.styles, styles)&&(identical(other.defaultPrompt, defaultPrompt) || other.defaultPrompt == defaultPrompt)&&(identical(other.weeklyUsage, weeklyUsage) || other.weeklyUsage == weeklyUsage)&&(identical(other.weeklyLimit, weeklyLimit) || other.weeklyLimit == weeklyLimit));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(styles),defaultPrompt,weeklyUsage,weeklyLimit);

@override
String toString() {
  return 'AiStylesResponse(styles: $styles, defaultPrompt: $defaultPrompt, weeklyUsage: $weeklyUsage, weeklyLimit: $weeklyLimit)';
}


}

/// @nodoc
abstract mixin class $AiStylesResponseCopyWith<$Res>  {
  factory $AiStylesResponseCopyWith(AiStylesResponse value, $Res Function(AiStylesResponse) _then) = _$AiStylesResponseCopyWithImpl;
@useResult
$Res call({
 List<AiStyle> styles, String defaultPrompt, String weeklyUsage, String weeklyLimit
});




}
/// @nodoc
class _$AiStylesResponseCopyWithImpl<$Res>
    implements $AiStylesResponseCopyWith<$Res> {
  _$AiStylesResponseCopyWithImpl(this._self, this._then);

  final AiStylesResponse _self;
  final $Res Function(AiStylesResponse) _then;

/// Create a copy of AiStylesResponse
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? styles = null,Object? defaultPrompt = null,Object? weeklyUsage = null,Object? weeklyLimit = null,}) {
  return _then(_self.copyWith(
styles: null == styles ? _self.styles : styles // ignore: cast_nullable_to_non_nullable
as List<AiStyle>,defaultPrompt: null == defaultPrompt ? _self.defaultPrompt : defaultPrompt // ignore: cast_nullable_to_non_nullable
as String,weeklyUsage: null == weeklyUsage ? _self.weeklyUsage : weeklyUsage // ignore: cast_nullable_to_non_nullable
as String,weeklyLimit: null == weeklyLimit ? _self.weeklyLimit : weeklyLimit // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AiStylesResponse].
extension AiStylesResponsePatterns on AiStylesResponse {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AiStylesResponse value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AiStylesResponse() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AiStylesResponse value)  $default,){
final _that = this;
switch (_that) {
case _AiStylesResponse():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AiStylesResponse value)?  $default,){
final _that = this;
switch (_that) {
case _AiStylesResponse() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<AiStyle> styles,  String defaultPrompt,  String weeklyUsage,  String weeklyLimit)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AiStylesResponse() when $default != null:
return $default(_that.styles,_that.defaultPrompt,_that.weeklyUsage,_that.weeklyLimit);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<AiStyle> styles,  String defaultPrompt,  String weeklyUsage,  String weeklyLimit)  $default,) {final _that = this;
switch (_that) {
case _AiStylesResponse():
return $default(_that.styles,_that.defaultPrompt,_that.weeklyUsage,_that.weeklyLimit);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<AiStyle> styles,  String defaultPrompt,  String weeklyUsage,  String weeklyLimit)?  $default,) {final _that = this;
switch (_that) {
case _AiStylesResponse() when $default != null:
return $default(_that.styles,_that.defaultPrompt,_that.weeklyUsage,_that.weeklyLimit);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AiStylesResponse implements AiStylesResponse {
  const _AiStylesResponse({final  List<AiStyle> styles = const [], this.defaultPrompt = '', this.weeklyUsage = '0', this.weeklyLimit = '10'}): _styles = styles;
  factory _AiStylesResponse.fromJson(Map<String, dynamic> json) => _$AiStylesResponseFromJson(json);

 final  List<AiStyle> _styles;
@override@JsonKey() List<AiStyle> get styles {
  if (_styles is EqualUnmodifiableListView) return _styles;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_styles);
}

@override@JsonKey() final  String defaultPrompt;
@override@JsonKey() final  String weeklyUsage;
@override@JsonKey() final  String weeklyLimit;

/// Create a copy of AiStylesResponse
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AiStylesResponseCopyWith<_AiStylesResponse> get copyWith => __$AiStylesResponseCopyWithImpl<_AiStylesResponse>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$AiStylesResponseToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AiStylesResponse&&const DeepCollectionEquality().equals(other._styles, _styles)&&(identical(other.defaultPrompt, defaultPrompt) || other.defaultPrompt == defaultPrompt)&&(identical(other.weeklyUsage, weeklyUsage) || other.weeklyUsage == weeklyUsage)&&(identical(other.weeklyLimit, weeklyLimit) || other.weeklyLimit == weeklyLimit));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_styles),defaultPrompt,weeklyUsage,weeklyLimit);

@override
String toString() {
  return 'AiStylesResponse(styles: $styles, defaultPrompt: $defaultPrompt, weeklyUsage: $weeklyUsage, weeklyLimit: $weeklyLimit)';
}


}

/// @nodoc
abstract mixin class _$AiStylesResponseCopyWith<$Res> implements $AiStylesResponseCopyWith<$Res> {
  factory _$AiStylesResponseCopyWith(_AiStylesResponse value, $Res Function(_AiStylesResponse) _then) = __$AiStylesResponseCopyWithImpl;
@override @useResult
$Res call({
 List<AiStyle> styles, String defaultPrompt, String weeklyUsage, String weeklyLimit
});




}
/// @nodoc
class __$AiStylesResponseCopyWithImpl<$Res>
    implements _$AiStylesResponseCopyWith<$Res> {
  __$AiStylesResponseCopyWithImpl(this._self, this._then);

  final _AiStylesResponse _self;
  final $Res Function(_AiStylesResponse) _then;

/// Create a copy of AiStylesResponse
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? styles = null,Object? defaultPrompt = null,Object? weeklyUsage = null,Object? weeklyLimit = null,}) {
  return _then(_AiStylesResponse(
styles: null == styles ? _self._styles : styles // ignore: cast_nullable_to_non_nullable
as List<AiStyle>,defaultPrompt: null == defaultPrompt ? _self.defaultPrompt : defaultPrompt // ignore: cast_nullable_to_non_nullable
as String,weeklyUsage: null == weeklyUsage ? _self.weeklyUsage : weeklyUsage // ignore: cast_nullable_to_non_nullable
as String,weeklyLimit: null == weeklyLimit ? _self.weeklyLimit : weeklyLimit // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
