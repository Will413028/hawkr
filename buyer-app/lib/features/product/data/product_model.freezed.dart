// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'product_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$Product {

@JsonKey(name: 'skuId') int get skuId; String get title; int get price; String get image;@JsonKey(name: 'categoryId') int? get categoryId;@JsonKey(name: 'sellerId') int? get sellerId;@JsonKey(name: 'merchantName') String? get merchantName; int? get stock;@JsonKey(name: 'isFavorited') bool get isFavorited;@JsonKey(name: 'isRestockSubscribed') bool get isRestockSubscribed;@JsonKey(name: 'selfOwned') bool get selfOwned;
/// Create a copy of Product
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ProductCopyWith<Product> get copyWith => _$ProductCopyWithImpl<Product>(this as Product, _$identity);

  /// Serializes this Product to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Product&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.title, title) || other.title == title)&&(identical(other.price, price) || other.price == price)&&(identical(other.image, image) || other.image == image)&&(identical(other.categoryId, categoryId) || other.categoryId == categoryId)&&(identical(other.sellerId, sellerId) || other.sellerId == sellerId)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.stock, stock) || other.stock == stock)&&(identical(other.isFavorited, isFavorited) || other.isFavorited == isFavorited)&&(identical(other.isRestockSubscribed, isRestockSubscribed) || other.isRestockSubscribed == isRestockSubscribed)&&(identical(other.selfOwned, selfOwned) || other.selfOwned == selfOwned));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,skuId,title,price,image,categoryId,sellerId,merchantName,stock,isFavorited,isRestockSubscribed,selfOwned);

@override
String toString() {
  return 'Product(skuId: $skuId, title: $title, price: $price, image: $image, categoryId: $categoryId, sellerId: $sellerId, merchantName: $merchantName, stock: $stock, isFavorited: $isFavorited, isRestockSubscribed: $isRestockSubscribed, selfOwned: $selfOwned)';
}


}

/// @nodoc
abstract mixin class $ProductCopyWith<$Res>  {
  factory $ProductCopyWith(Product value, $Res Function(Product) _then) = _$ProductCopyWithImpl;
@useResult
$Res call({
@JsonKey(name: 'skuId') int skuId, String title, int price, String image,@JsonKey(name: 'categoryId') int? categoryId,@JsonKey(name: 'sellerId') int? sellerId,@JsonKey(name: 'merchantName') String? merchantName, int? stock,@JsonKey(name: 'isFavorited') bool isFavorited,@JsonKey(name: 'isRestockSubscribed') bool isRestockSubscribed,@JsonKey(name: 'selfOwned') bool selfOwned
});




}
/// @nodoc
class _$ProductCopyWithImpl<$Res>
    implements $ProductCopyWith<$Res> {
  _$ProductCopyWithImpl(this._self, this._then);

  final Product _self;
  final $Res Function(Product) _then;

/// Create a copy of Product
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? skuId = null,Object? title = null,Object? price = null,Object? image = null,Object? categoryId = freezed,Object? sellerId = freezed,Object? merchantName = freezed,Object? stock = freezed,Object? isFavorited = null,Object? isRestockSubscribed = null,Object? selfOwned = null,}) {
  return _then(_self.copyWith(
skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as int,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,image: null == image ? _self.image : image // ignore: cast_nullable_to_non_nullable
as String,categoryId: freezed == categoryId ? _self.categoryId : categoryId // ignore: cast_nullable_to_non_nullable
as int?,sellerId: freezed == sellerId ? _self.sellerId : sellerId // ignore: cast_nullable_to_non_nullable
as int?,merchantName: freezed == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String?,stock: freezed == stock ? _self.stock : stock // ignore: cast_nullable_to_non_nullable
as int?,isFavorited: null == isFavorited ? _self.isFavorited : isFavorited // ignore: cast_nullable_to_non_nullable
as bool,isRestockSubscribed: null == isRestockSubscribed ? _self.isRestockSubscribed : isRestockSubscribed // ignore: cast_nullable_to_non_nullable
as bool,selfOwned: null == selfOwned ? _self.selfOwned : selfOwned // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

}


/// Adds pattern-matching-related methods to [Product].
extension ProductPatterns on Product {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Product value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Product() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Product value)  $default,){
final _that = this;
switch (_that) {
case _Product():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Product value)?  $default,){
final _that = this;
switch (_that) {
case _Product() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(name: 'skuId')  int skuId,  String title,  int price,  String image, @JsonKey(name: 'categoryId')  int? categoryId, @JsonKey(name: 'sellerId')  int? sellerId, @JsonKey(name: 'merchantName')  String? merchantName,  int? stock, @JsonKey(name: 'isFavorited')  bool isFavorited, @JsonKey(name: 'isRestockSubscribed')  bool isRestockSubscribed, @JsonKey(name: 'selfOwned')  bool selfOwned)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Product() when $default != null:
return $default(_that.skuId,_that.title,_that.price,_that.image,_that.categoryId,_that.sellerId,_that.merchantName,_that.stock,_that.isFavorited,_that.isRestockSubscribed,_that.selfOwned);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(name: 'skuId')  int skuId,  String title,  int price,  String image, @JsonKey(name: 'categoryId')  int? categoryId, @JsonKey(name: 'sellerId')  int? sellerId, @JsonKey(name: 'merchantName')  String? merchantName,  int? stock, @JsonKey(name: 'isFavorited')  bool isFavorited, @JsonKey(name: 'isRestockSubscribed')  bool isRestockSubscribed, @JsonKey(name: 'selfOwned')  bool selfOwned)  $default,) {final _that = this;
switch (_that) {
case _Product():
return $default(_that.skuId,_that.title,_that.price,_that.image,_that.categoryId,_that.sellerId,_that.merchantName,_that.stock,_that.isFavorited,_that.isRestockSubscribed,_that.selfOwned);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(name: 'skuId')  int skuId,  String title,  int price,  String image, @JsonKey(name: 'categoryId')  int? categoryId, @JsonKey(name: 'sellerId')  int? sellerId, @JsonKey(name: 'merchantName')  String? merchantName,  int? stock, @JsonKey(name: 'isFavorited')  bool isFavorited, @JsonKey(name: 'isRestockSubscribed')  bool isRestockSubscribed, @JsonKey(name: 'selfOwned')  bool selfOwned)?  $default,) {final _that = this;
switch (_that) {
case _Product() when $default != null:
return $default(_that.skuId,_that.title,_that.price,_that.image,_that.categoryId,_that.sellerId,_that.merchantName,_that.stock,_that.isFavorited,_that.isRestockSubscribed,_that.selfOwned);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Product implements Product {
  const _Product({@JsonKey(name: 'skuId') required this.skuId, required this.title, required this.price, required this.image, @JsonKey(name: 'categoryId') this.categoryId, @JsonKey(name: 'sellerId') this.sellerId, @JsonKey(name: 'merchantName') this.merchantName, this.stock, @JsonKey(name: 'isFavorited') this.isFavorited = false, @JsonKey(name: 'isRestockSubscribed') this.isRestockSubscribed = false, @JsonKey(name: 'selfOwned') this.selfOwned = false});
  factory _Product.fromJson(Map<String, dynamic> json) => _$ProductFromJson(json);

@override@JsonKey(name: 'skuId') final  int skuId;
@override final  String title;
@override final  int price;
@override final  String image;
@override@JsonKey(name: 'categoryId') final  int? categoryId;
@override@JsonKey(name: 'sellerId') final  int? sellerId;
@override@JsonKey(name: 'merchantName') final  String? merchantName;
@override final  int? stock;
@override@JsonKey(name: 'isFavorited') final  bool isFavorited;
@override@JsonKey(name: 'isRestockSubscribed') final  bool isRestockSubscribed;
@override@JsonKey(name: 'selfOwned') final  bool selfOwned;

/// Create a copy of Product
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ProductCopyWith<_Product> get copyWith => __$ProductCopyWithImpl<_Product>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$ProductToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Product&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.title, title) || other.title == title)&&(identical(other.price, price) || other.price == price)&&(identical(other.image, image) || other.image == image)&&(identical(other.categoryId, categoryId) || other.categoryId == categoryId)&&(identical(other.sellerId, sellerId) || other.sellerId == sellerId)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.stock, stock) || other.stock == stock)&&(identical(other.isFavorited, isFavorited) || other.isFavorited == isFavorited)&&(identical(other.isRestockSubscribed, isRestockSubscribed) || other.isRestockSubscribed == isRestockSubscribed)&&(identical(other.selfOwned, selfOwned) || other.selfOwned == selfOwned));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,skuId,title,price,image,categoryId,sellerId,merchantName,stock,isFavorited,isRestockSubscribed,selfOwned);

@override
String toString() {
  return 'Product(skuId: $skuId, title: $title, price: $price, image: $image, categoryId: $categoryId, sellerId: $sellerId, merchantName: $merchantName, stock: $stock, isFavorited: $isFavorited, isRestockSubscribed: $isRestockSubscribed, selfOwned: $selfOwned)';
}


}

/// @nodoc
abstract mixin class _$ProductCopyWith<$Res> implements $ProductCopyWith<$Res> {
  factory _$ProductCopyWith(_Product value, $Res Function(_Product) _then) = __$ProductCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(name: 'skuId') int skuId, String title, int price, String image,@JsonKey(name: 'categoryId') int? categoryId,@JsonKey(name: 'sellerId') int? sellerId,@JsonKey(name: 'merchantName') String? merchantName, int? stock,@JsonKey(name: 'isFavorited') bool isFavorited,@JsonKey(name: 'isRestockSubscribed') bool isRestockSubscribed,@JsonKey(name: 'selfOwned') bool selfOwned
});




}
/// @nodoc
class __$ProductCopyWithImpl<$Res>
    implements _$ProductCopyWith<$Res> {
  __$ProductCopyWithImpl(this._self, this._then);

  final _Product _self;
  final $Res Function(_Product) _then;

/// Create a copy of Product
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? skuId = null,Object? title = null,Object? price = null,Object? image = null,Object? categoryId = freezed,Object? sellerId = freezed,Object? merchantName = freezed,Object? stock = freezed,Object? isFavorited = null,Object? isRestockSubscribed = null,Object? selfOwned = null,}) {
  return _then(_Product(
skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as int,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,image: null == image ? _self.image : image // ignore: cast_nullable_to_non_nullable
as String,categoryId: freezed == categoryId ? _self.categoryId : categoryId // ignore: cast_nullable_to_non_nullable
as int?,sellerId: freezed == sellerId ? _self.sellerId : sellerId // ignore: cast_nullable_to_non_nullable
as int?,merchantName: freezed == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String?,stock: freezed == stock ? _self.stock : stock // ignore: cast_nullable_to_non_nullable
as int?,isFavorited: null == isFavorited ? _self.isFavorited : isFavorited // ignore: cast_nullable_to_non_nullable
as bool,isRestockSubscribed: null == isRestockSubscribed ? _self.isRestockSubscribed : isRestockSubscribed // ignore: cast_nullable_to_non_nullable
as bool,selfOwned: null == selfOwned ? _self.selfOwned : selfOwned // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}


}


/// @nodoc
mixin _$SearchSuggest {

 List<String> get suggestions;
/// Create a copy of SearchSuggest
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SearchSuggestCopyWith<SearchSuggest> get copyWith => _$SearchSuggestCopyWithImpl<SearchSuggest>(this as SearchSuggest, _$identity);

  /// Serializes this SearchSuggest to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SearchSuggest&&const DeepCollectionEquality().equals(other.suggestions, suggestions));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(suggestions));

@override
String toString() {
  return 'SearchSuggest(suggestions: $suggestions)';
}


}

/// @nodoc
abstract mixin class $SearchSuggestCopyWith<$Res>  {
  factory $SearchSuggestCopyWith(SearchSuggest value, $Res Function(SearchSuggest) _then) = _$SearchSuggestCopyWithImpl;
@useResult
$Res call({
 List<String> suggestions
});




}
/// @nodoc
class _$SearchSuggestCopyWithImpl<$Res>
    implements $SearchSuggestCopyWith<$Res> {
  _$SearchSuggestCopyWithImpl(this._self, this._then);

  final SearchSuggest _self;
  final $Res Function(SearchSuggest) _then;

/// Create a copy of SearchSuggest
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? suggestions = null,}) {
  return _then(_self.copyWith(
suggestions: null == suggestions ? _self.suggestions : suggestions // ignore: cast_nullable_to_non_nullable
as List<String>,
  ));
}

}


/// Adds pattern-matching-related methods to [SearchSuggest].
extension SearchSuggestPatterns on SearchSuggest {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SearchSuggest value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SearchSuggest() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SearchSuggest value)  $default,){
final _that = this;
switch (_that) {
case _SearchSuggest():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SearchSuggest value)?  $default,){
final _that = this;
switch (_that) {
case _SearchSuggest() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<String> suggestions)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SearchSuggest() when $default != null:
return $default(_that.suggestions);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<String> suggestions)  $default,) {final _that = this;
switch (_that) {
case _SearchSuggest():
return $default(_that.suggestions);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<String> suggestions)?  $default,) {final _that = this;
switch (_that) {
case _SearchSuggest() when $default != null:
return $default(_that.suggestions);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SearchSuggest implements SearchSuggest {
  const _SearchSuggest({final  List<String> suggestions = const []}): _suggestions = suggestions;
  factory _SearchSuggest.fromJson(Map<String, dynamic> json) => _$SearchSuggestFromJson(json);

 final  List<String> _suggestions;
@override@JsonKey() List<String> get suggestions {
  if (_suggestions is EqualUnmodifiableListView) return _suggestions;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_suggestions);
}


/// Create a copy of SearchSuggest
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SearchSuggestCopyWith<_SearchSuggest> get copyWith => __$SearchSuggestCopyWithImpl<_SearchSuggest>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SearchSuggestToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SearchSuggest&&const DeepCollectionEquality().equals(other._suggestions, _suggestions));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_suggestions));

@override
String toString() {
  return 'SearchSuggest(suggestions: $suggestions)';
}


}

/// @nodoc
abstract mixin class _$SearchSuggestCopyWith<$Res> implements $SearchSuggestCopyWith<$Res> {
  factory _$SearchSuggestCopyWith(_SearchSuggest value, $Res Function(_SearchSuggest) _then) = __$SearchSuggestCopyWithImpl;
@override @useResult
$Res call({
 List<String> suggestions
});




}
/// @nodoc
class __$SearchSuggestCopyWithImpl<$Res>
    implements _$SearchSuggestCopyWith<$Res> {
  __$SearchSuggestCopyWithImpl(this._self, this._then);

  final _SearchSuggest _self;
  final $Res Function(_SearchSuggest) _then;

/// Create a copy of SearchSuggest
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? suggestions = null,}) {
  return _then(_SearchSuggest(
suggestions: null == suggestions ? _self._suggestions : suggestions // ignore: cast_nullable_to_non_nullable
as List<String>,
  ));
}


}

// dart format on
