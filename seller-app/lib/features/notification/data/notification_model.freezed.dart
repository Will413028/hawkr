// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'notification_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$NotificationItem {

// ignore: invalid_annotation_target
@JsonKey(fromJson: _parseInt) int get id; String get type; String get title; String get body; String? get refType; String? get refId; bool get isRead; DateTime? get readAt; DateTime get createdAt;/// 各 type 附帶的額外資料（結構依 type 而異）
 Map<String, dynamic>? get data;
/// Create a copy of NotificationItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$NotificationItemCopyWith<NotificationItem> get copyWith => _$NotificationItemCopyWithImpl<NotificationItem>(this as NotificationItem, _$identity);

  /// Serializes this NotificationItem to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is NotificationItem&&(identical(other.id, id) || other.id == id)&&(identical(other.type, type) || other.type == type)&&(identical(other.title, title) || other.title == title)&&(identical(other.body, body) || other.body == body)&&(identical(other.refType, refType) || other.refType == refType)&&(identical(other.refId, refId) || other.refId == refId)&&(identical(other.isRead, isRead) || other.isRead == isRead)&&(identical(other.readAt, readAt) || other.readAt == readAt)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other.data, data));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,type,title,body,refType,refId,isRead,readAt,createdAt,const DeepCollectionEquality().hash(data));

@override
String toString() {
  return 'NotificationItem(id: $id, type: $type, title: $title, body: $body, refType: $refType, refId: $refId, isRead: $isRead, readAt: $readAt, createdAt: $createdAt, data: $data)';
}


}

/// @nodoc
abstract mixin class $NotificationItemCopyWith<$Res>  {
  factory $NotificationItemCopyWith(NotificationItem value, $Res Function(NotificationItem) _then) = _$NotificationItemCopyWithImpl;
@useResult
$Res call({
@JsonKey(fromJson: _parseInt) int id, String type, String title, String body, String? refType, String? refId, bool isRead, DateTime? readAt, DateTime createdAt, Map<String, dynamic>? data
});




}
/// @nodoc
class _$NotificationItemCopyWithImpl<$Res>
    implements $NotificationItemCopyWith<$Res> {
  _$NotificationItemCopyWithImpl(this._self, this._then);

  final NotificationItem _self;
  final $Res Function(NotificationItem) _then;

/// Create a copy of NotificationItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? type = null,Object? title = null,Object? body = null,Object? refType = freezed,Object? refId = freezed,Object? isRead = null,Object? readAt = freezed,Object? createdAt = null,Object? data = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,type: null == type ? _self.type : type // ignore: cast_nullable_to_non_nullable
as String,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,body: null == body ? _self.body : body // ignore: cast_nullable_to_non_nullable
as String,refType: freezed == refType ? _self.refType : refType // ignore: cast_nullable_to_non_nullable
as String?,refId: freezed == refId ? _self.refId : refId // ignore: cast_nullable_to_non_nullable
as String?,isRead: null == isRead ? _self.isRead : isRead // ignore: cast_nullable_to_non_nullable
as bool,readAt: freezed == readAt ? _self.readAt : readAt // ignore: cast_nullable_to_non_nullable
as DateTime?,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime,data: freezed == data ? _self.data : data // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,
  ));
}

}


/// Adds pattern-matching-related methods to [NotificationItem].
extension NotificationItemPatterns on NotificationItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _NotificationItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _NotificationItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _NotificationItem value)  $default,){
final _that = this;
switch (_that) {
case _NotificationItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _NotificationItem value)?  $default,){
final _that = this;
switch (_that) {
case _NotificationItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(fromJson: _parseInt)  int id,  String type,  String title,  String body,  String? refType,  String? refId,  bool isRead,  DateTime? readAt,  DateTime createdAt,  Map<String, dynamic>? data)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _NotificationItem() when $default != null:
return $default(_that.id,_that.type,_that.title,_that.body,_that.refType,_that.refId,_that.isRead,_that.readAt,_that.createdAt,_that.data);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(fromJson: _parseInt)  int id,  String type,  String title,  String body,  String? refType,  String? refId,  bool isRead,  DateTime? readAt,  DateTime createdAt,  Map<String, dynamic>? data)  $default,) {final _that = this;
switch (_that) {
case _NotificationItem():
return $default(_that.id,_that.type,_that.title,_that.body,_that.refType,_that.refId,_that.isRead,_that.readAt,_that.createdAt,_that.data);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(fromJson: _parseInt)  int id,  String type,  String title,  String body,  String? refType,  String? refId,  bool isRead,  DateTime? readAt,  DateTime createdAt,  Map<String, dynamic>? data)?  $default,) {final _that = this;
switch (_that) {
case _NotificationItem() when $default != null:
return $default(_that.id,_that.type,_that.title,_that.body,_that.refType,_that.refId,_that.isRead,_that.readAt,_that.createdAt,_that.data);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _NotificationItem implements NotificationItem {
  const _NotificationItem({@JsonKey(fromJson: _parseInt) this.id = 0, this.type = '', this.title = '', this.body = '', this.refType, this.refId, this.isRead = false, this.readAt, required this.createdAt, final  Map<String, dynamic>? data}): _data = data;
  factory _NotificationItem.fromJson(Map<String, dynamic> json) => _$NotificationItemFromJson(json);

// ignore: invalid_annotation_target
@override@JsonKey(fromJson: _parseInt) final  int id;
@override@JsonKey() final  String type;
@override@JsonKey() final  String title;
@override@JsonKey() final  String body;
@override final  String? refType;
@override final  String? refId;
@override@JsonKey() final  bool isRead;
@override final  DateTime? readAt;
@override final  DateTime createdAt;
/// 各 type 附帶的額外資料（結構依 type 而異）
 final  Map<String, dynamic>? _data;
/// 各 type 附帶的額外資料（結構依 type 而異）
@override Map<String, dynamic>? get data {
  final value = _data;
  if (value == null) return null;
  if (_data is EqualUnmodifiableMapView) return _data;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(value);
}


/// Create a copy of NotificationItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$NotificationItemCopyWith<_NotificationItem> get copyWith => __$NotificationItemCopyWithImpl<_NotificationItem>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$NotificationItemToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _NotificationItem&&(identical(other.id, id) || other.id == id)&&(identical(other.type, type) || other.type == type)&&(identical(other.title, title) || other.title == title)&&(identical(other.body, body) || other.body == body)&&(identical(other.refType, refType) || other.refType == refType)&&(identical(other.refId, refId) || other.refId == refId)&&(identical(other.isRead, isRead) || other.isRead == isRead)&&(identical(other.readAt, readAt) || other.readAt == readAt)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other._data, _data));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,type,title,body,refType,refId,isRead,readAt,createdAt,const DeepCollectionEquality().hash(_data));

@override
String toString() {
  return 'NotificationItem(id: $id, type: $type, title: $title, body: $body, refType: $refType, refId: $refId, isRead: $isRead, readAt: $readAt, createdAt: $createdAt, data: $data)';
}


}

/// @nodoc
abstract mixin class _$NotificationItemCopyWith<$Res> implements $NotificationItemCopyWith<$Res> {
  factory _$NotificationItemCopyWith(_NotificationItem value, $Res Function(_NotificationItem) _then) = __$NotificationItemCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(fromJson: _parseInt) int id, String type, String title, String body, String? refType, String? refId, bool isRead, DateTime? readAt, DateTime createdAt, Map<String, dynamic>? data
});




}
/// @nodoc
class __$NotificationItemCopyWithImpl<$Res>
    implements _$NotificationItemCopyWith<$Res> {
  __$NotificationItemCopyWithImpl(this._self, this._then);

  final _NotificationItem _self;
  final $Res Function(_NotificationItem) _then;

/// Create a copy of NotificationItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? type = null,Object? title = null,Object? body = null,Object? refType = freezed,Object? refId = freezed,Object? isRead = null,Object? readAt = freezed,Object? createdAt = null,Object? data = freezed,}) {
  return _then(_NotificationItem(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,type: null == type ? _self.type : type // ignore: cast_nullable_to_non_nullable
as String,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,body: null == body ? _self.body : body // ignore: cast_nullable_to_non_nullable
as String,refType: freezed == refType ? _self.refType : refType // ignore: cast_nullable_to_non_nullable
as String?,refId: freezed == refId ? _self.refId : refId // ignore: cast_nullable_to_non_nullable
as String?,isRead: null == isRead ? _self.isRead : isRead // ignore: cast_nullable_to_non_nullable
as bool,readAt: freezed == readAt ? _self.readAt : readAt // ignore: cast_nullable_to_non_nullable
as DateTime?,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime,data: freezed == data ? _self._data : data // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,
  ));
}


}


/// @nodoc
mixin _$NotificationListResult {

// ignore: invalid_annotation_target
@JsonKey(name: 'records') List<NotificationItem> get records;// ignore: invalid_annotation_target
@JsonKey(fromJson: _parseInt) int get total;// ignore: invalid_annotation_target
@JsonKey(name: 'pageSize', fromJson: _parseInt) int get pageSize; bool get hasMore;
/// Create a copy of NotificationListResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$NotificationListResultCopyWith<NotificationListResult> get copyWith => _$NotificationListResultCopyWithImpl<NotificationListResult>(this as NotificationListResult, _$identity);

  /// Serializes this NotificationListResult to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is NotificationListResult&&const DeepCollectionEquality().equals(other.records, records)&&(identical(other.total, total) || other.total == total)&&(identical(other.pageSize, pageSize) || other.pageSize == pageSize)&&(identical(other.hasMore, hasMore) || other.hasMore == hasMore));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(records),total,pageSize,hasMore);

@override
String toString() {
  return 'NotificationListResult(records: $records, total: $total, pageSize: $pageSize, hasMore: $hasMore)';
}


}

/// @nodoc
abstract mixin class $NotificationListResultCopyWith<$Res>  {
  factory $NotificationListResultCopyWith(NotificationListResult value, $Res Function(NotificationListResult) _then) = _$NotificationListResultCopyWithImpl;
@useResult
$Res call({
@JsonKey(name: 'records') List<NotificationItem> records,@JsonKey(fromJson: _parseInt) int total,@JsonKey(name: 'pageSize', fromJson: _parseInt) int pageSize, bool hasMore
});




}
/// @nodoc
class _$NotificationListResultCopyWithImpl<$Res>
    implements $NotificationListResultCopyWith<$Res> {
  _$NotificationListResultCopyWithImpl(this._self, this._then);

  final NotificationListResult _self;
  final $Res Function(NotificationListResult) _then;

/// Create a copy of NotificationListResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? records = null,Object? total = null,Object? pageSize = null,Object? hasMore = null,}) {
  return _then(_self.copyWith(
records: null == records ? _self.records : records // ignore: cast_nullable_to_non_nullable
as List<NotificationItem>,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,pageSize: null == pageSize ? _self.pageSize : pageSize // ignore: cast_nullable_to_non_nullable
as int,hasMore: null == hasMore ? _self.hasMore : hasMore // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

}


/// Adds pattern-matching-related methods to [NotificationListResult].
extension NotificationListResultPatterns on NotificationListResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _NotificationListResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _NotificationListResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _NotificationListResult value)  $default,){
final _that = this;
switch (_that) {
case _NotificationListResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _NotificationListResult value)?  $default,){
final _that = this;
switch (_that) {
case _NotificationListResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(name: 'records')  List<NotificationItem> records, @JsonKey(fromJson: _parseInt)  int total, @JsonKey(name: 'pageSize', fromJson: _parseInt)  int pageSize,  bool hasMore)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _NotificationListResult() when $default != null:
return $default(_that.records,_that.total,_that.pageSize,_that.hasMore);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(name: 'records')  List<NotificationItem> records, @JsonKey(fromJson: _parseInt)  int total, @JsonKey(name: 'pageSize', fromJson: _parseInt)  int pageSize,  bool hasMore)  $default,) {final _that = this;
switch (_that) {
case _NotificationListResult():
return $default(_that.records,_that.total,_that.pageSize,_that.hasMore);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(name: 'records')  List<NotificationItem> records, @JsonKey(fromJson: _parseInt)  int total, @JsonKey(name: 'pageSize', fromJson: _parseInt)  int pageSize,  bool hasMore)?  $default,) {final _that = this;
switch (_that) {
case _NotificationListResult() when $default != null:
return $default(_that.records,_that.total,_that.pageSize,_that.hasMore);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _NotificationListResult implements NotificationListResult {
  const _NotificationListResult({@JsonKey(name: 'records') final  List<NotificationItem> records = const [], @JsonKey(fromJson: _parseInt) this.total = 0, @JsonKey(name: 'pageSize', fromJson: _parseInt) this.pageSize = 20, this.hasMore = false}): _records = records;
  factory _NotificationListResult.fromJson(Map<String, dynamic> json) => _$NotificationListResultFromJson(json);

// ignore: invalid_annotation_target
 final  List<NotificationItem> _records;
// ignore: invalid_annotation_target
@override@JsonKey(name: 'records') List<NotificationItem> get records {
  if (_records is EqualUnmodifiableListView) return _records;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_records);
}

// ignore: invalid_annotation_target
@override@JsonKey(fromJson: _parseInt) final  int total;
// ignore: invalid_annotation_target
@override@JsonKey(name: 'pageSize', fromJson: _parseInt) final  int pageSize;
@override@JsonKey() final  bool hasMore;

/// Create a copy of NotificationListResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$NotificationListResultCopyWith<_NotificationListResult> get copyWith => __$NotificationListResultCopyWithImpl<_NotificationListResult>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$NotificationListResultToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _NotificationListResult&&const DeepCollectionEquality().equals(other._records, _records)&&(identical(other.total, total) || other.total == total)&&(identical(other.pageSize, pageSize) || other.pageSize == pageSize)&&(identical(other.hasMore, hasMore) || other.hasMore == hasMore));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_records),total,pageSize,hasMore);

@override
String toString() {
  return 'NotificationListResult(records: $records, total: $total, pageSize: $pageSize, hasMore: $hasMore)';
}


}

/// @nodoc
abstract mixin class _$NotificationListResultCopyWith<$Res> implements $NotificationListResultCopyWith<$Res> {
  factory _$NotificationListResultCopyWith(_NotificationListResult value, $Res Function(_NotificationListResult) _then) = __$NotificationListResultCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(name: 'records') List<NotificationItem> records,@JsonKey(fromJson: _parseInt) int total,@JsonKey(name: 'pageSize', fromJson: _parseInt) int pageSize, bool hasMore
});




}
/// @nodoc
class __$NotificationListResultCopyWithImpl<$Res>
    implements _$NotificationListResultCopyWith<$Res> {
  __$NotificationListResultCopyWithImpl(this._self, this._then);

  final _NotificationListResult _self;
  final $Res Function(_NotificationListResult) _then;

/// Create a copy of NotificationListResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? records = null,Object? total = null,Object? pageSize = null,Object? hasMore = null,}) {
  return _then(_NotificationListResult(
records: null == records ? _self._records : records // ignore: cast_nullable_to_non_nullable
as List<NotificationItem>,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,pageSize: null == pageSize ? _self.pageSize : pageSize // ignore: cast_nullable_to_non_nullable
as int,hasMore: null == hasMore ? _self.hasMore : hasMore // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}


}


/// @nodoc
mixin _$UnreadCount {

// ignore: invalid_annotation_target
@JsonKey(name: 'count', fromJson: _parseInt) int get count;
/// Create a copy of UnreadCount
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$UnreadCountCopyWith<UnreadCount> get copyWith => _$UnreadCountCopyWithImpl<UnreadCount>(this as UnreadCount, _$identity);

  /// Serializes this UnreadCount to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is UnreadCount&&(identical(other.count, count) || other.count == count));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,count);

@override
String toString() {
  return 'UnreadCount(count: $count)';
}


}

/// @nodoc
abstract mixin class $UnreadCountCopyWith<$Res>  {
  factory $UnreadCountCopyWith(UnreadCount value, $Res Function(UnreadCount) _then) = _$UnreadCountCopyWithImpl;
@useResult
$Res call({
@JsonKey(name: 'count', fromJson: _parseInt) int count
});




}
/// @nodoc
class _$UnreadCountCopyWithImpl<$Res>
    implements $UnreadCountCopyWith<$Res> {
  _$UnreadCountCopyWithImpl(this._self, this._then);

  final UnreadCount _self;
  final $Res Function(UnreadCount) _then;

/// Create a copy of UnreadCount
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? count = null,}) {
  return _then(_self.copyWith(
count: null == count ? _self.count : count // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [UnreadCount].
extension UnreadCountPatterns on UnreadCount {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _UnreadCount value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _UnreadCount() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _UnreadCount value)  $default,){
final _that = this;
switch (_that) {
case _UnreadCount():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _UnreadCount value)?  $default,){
final _that = this;
switch (_that) {
case _UnreadCount() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(name: 'count', fromJson: _parseInt)  int count)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _UnreadCount() when $default != null:
return $default(_that.count);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(name: 'count', fromJson: _parseInt)  int count)  $default,) {final _that = this;
switch (_that) {
case _UnreadCount():
return $default(_that.count);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(name: 'count', fromJson: _parseInt)  int count)?  $default,) {final _that = this;
switch (_that) {
case _UnreadCount() when $default != null:
return $default(_that.count);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _UnreadCount implements UnreadCount {
  const _UnreadCount({@JsonKey(name: 'count', fromJson: _parseInt) this.count = 0});
  factory _UnreadCount.fromJson(Map<String, dynamic> json) => _$UnreadCountFromJson(json);

// ignore: invalid_annotation_target
@override@JsonKey(name: 'count', fromJson: _parseInt) final  int count;

/// Create a copy of UnreadCount
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$UnreadCountCopyWith<_UnreadCount> get copyWith => __$UnreadCountCopyWithImpl<_UnreadCount>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$UnreadCountToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _UnreadCount&&(identical(other.count, count) || other.count == count));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,count);

@override
String toString() {
  return 'UnreadCount(count: $count)';
}


}

/// @nodoc
abstract mixin class _$UnreadCountCopyWith<$Res> implements $UnreadCountCopyWith<$Res> {
  factory _$UnreadCountCopyWith(_UnreadCount value, $Res Function(_UnreadCount) _then) = __$UnreadCountCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(name: 'count', fromJson: _parseInt) int count
});




}
/// @nodoc
class __$UnreadCountCopyWithImpl<$Res>
    implements _$UnreadCountCopyWith<$Res> {
  __$UnreadCountCopyWithImpl(this._self, this._then);

  final _UnreadCount _self;
  final $Res Function(_UnreadCount) _then;

/// Create a copy of UnreadCount
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? count = null,}) {
  return _then(_UnreadCount(
count: null == count ? _self.count : count // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}


/// @nodoc
mixin _$NotificationPreferenceItem {

 String get category; bool get enabled; String get title; String get description;/// 後端強制開啟、前端不可關閉（例如賣家的 shipment_anomaly）。
 bool get forced;
/// Create a copy of NotificationPreferenceItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$NotificationPreferenceItemCopyWith<NotificationPreferenceItem> get copyWith => _$NotificationPreferenceItemCopyWithImpl<NotificationPreferenceItem>(this as NotificationPreferenceItem, _$identity);

  /// Serializes this NotificationPreferenceItem to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is NotificationPreferenceItem&&(identical(other.category, category) || other.category == category)&&(identical(other.enabled, enabled) || other.enabled == enabled)&&(identical(other.title, title) || other.title == title)&&(identical(other.description, description) || other.description == description)&&(identical(other.forced, forced) || other.forced == forced));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,category,enabled,title,description,forced);

@override
String toString() {
  return 'NotificationPreferenceItem(category: $category, enabled: $enabled, title: $title, description: $description, forced: $forced)';
}


}

/// @nodoc
abstract mixin class $NotificationPreferenceItemCopyWith<$Res>  {
  factory $NotificationPreferenceItemCopyWith(NotificationPreferenceItem value, $Res Function(NotificationPreferenceItem) _then) = _$NotificationPreferenceItemCopyWithImpl;
@useResult
$Res call({
 String category, bool enabled, String title, String description, bool forced
});




}
/// @nodoc
class _$NotificationPreferenceItemCopyWithImpl<$Res>
    implements $NotificationPreferenceItemCopyWith<$Res> {
  _$NotificationPreferenceItemCopyWithImpl(this._self, this._then);

  final NotificationPreferenceItem _self;
  final $Res Function(NotificationPreferenceItem) _then;

/// Create a copy of NotificationPreferenceItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? category = null,Object? enabled = null,Object? title = null,Object? description = null,Object? forced = null,}) {
  return _then(_self.copyWith(
category: null == category ? _self.category : category // ignore: cast_nullable_to_non_nullable
as String,enabled: null == enabled ? _self.enabled : enabled // ignore: cast_nullable_to_non_nullable
as bool,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,forced: null == forced ? _self.forced : forced // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

}


/// Adds pattern-matching-related methods to [NotificationPreferenceItem].
extension NotificationPreferenceItemPatterns on NotificationPreferenceItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _NotificationPreferenceItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _NotificationPreferenceItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _NotificationPreferenceItem value)  $default,){
final _that = this;
switch (_that) {
case _NotificationPreferenceItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _NotificationPreferenceItem value)?  $default,){
final _that = this;
switch (_that) {
case _NotificationPreferenceItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String category,  bool enabled,  String title,  String description,  bool forced)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _NotificationPreferenceItem() when $default != null:
return $default(_that.category,_that.enabled,_that.title,_that.description,_that.forced);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String category,  bool enabled,  String title,  String description,  bool forced)  $default,) {final _that = this;
switch (_that) {
case _NotificationPreferenceItem():
return $default(_that.category,_that.enabled,_that.title,_that.description,_that.forced);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String category,  bool enabled,  String title,  String description,  bool forced)?  $default,) {final _that = this;
switch (_that) {
case _NotificationPreferenceItem() when $default != null:
return $default(_that.category,_that.enabled,_that.title,_that.description,_that.forced);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _NotificationPreferenceItem implements NotificationPreferenceItem {
  const _NotificationPreferenceItem({required this.category, this.enabled = true, this.title = '', this.description = '', this.forced = false});
  factory _NotificationPreferenceItem.fromJson(Map<String, dynamic> json) => _$NotificationPreferenceItemFromJson(json);

@override final  String category;
@override@JsonKey() final  bool enabled;
@override@JsonKey() final  String title;
@override@JsonKey() final  String description;
/// 後端強制開啟、前端不可關閉（例如賣家的 shipment_anomaly）。
@override@JsonKey() final  bool forced;

/// Create a copy of NotificationPreferenceItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$NotificationPreferenceItemCopyWith<_NotificationPreferenceItem> get copyWith => __$NotificationPreferenceItemCopyWithImpl<_NotificationPreferenceItem>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$NotificationPreferenceItemToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _NotificationPreferenceItem&&(identical(other.category, category) || other.category == category)&&(identical(other.enabled, enabled) || other.enabled == enabled)&&(identical(other.title, title) || other.title == title)&&(identical(other.description, description) || other.description == description)&&(identical(other.forced, forced) || other.forced == forced));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,category,enabled,title,description,forced);

@override
String toString() {
  return 'NotificationPreferenceItem(category: $category, enabled: $enabled, title: $title, description: $description, forced: $forced)';
}


}

/// @nodoc
abstract mixin class _$NotificationPreferenceItemCopyWith<$Res> implements $NotificationPreferenceItemCopyWith<$Res> {
  factory _$NotificationPreferenceItemCopyWith(_NotificationPreferenceItem value, $Res Function(_NotificationPreferenceItem) _then) = __$NotificationPreferenceItemCopyWithImpl;
@override @useResult
$Res call({
 String category, bool enabled, String title, String description, bool forced
});




}
/// @nodoc
class __$NotificationPreferenceItemCopyWithImpl<$Res>
    implements _$NotificationPreferenceItemCopyWith<$Res> {
  __$NotificationPreferenceItemCopyWithImpl(this._self, this._then);

  final _NotificationPreferenceItem _self;
  final $Res Function(_NotificationPreferenceItem) _then;

/// Create a copy of NotificationPreferenceItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? category = null,Object? enabled = null,Object? title = null,Object? description = null,Object? forced = null,}) {
  return _then(_NotificationPreferenceItem(
category: null == category ? _self.category : category // ignore: cast_nullable_to_non_nullable
as String,enabled: null == enabled ? _self.enabled : enabled // ignore: cast_nullable_to_non_nullable
as bool,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,forced: null == forced ? _self.forced : forced // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}


}

// dart format on
