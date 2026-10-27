// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'media_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$UploadSession {

/// 上傳會話 ID（UUID）
 String get sessionId;/// OSS 預簽名上傳 URL（isSecure=true 時為空）
 String get uploadUrl;/// OSS 物件路徑
 String get objectKey;
/// Create a copy of UploadSession
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$UploadSessionCopyWith<UploadSession> get copyWith => _$UploadSessionCopyWithImpl<UploadSession>(this as UploadSession, _$identity);

  /// Serializes this UploadSession to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is UploadSession&&(identical(other.sessionId, sessionId) || other.sessionId == sessionId)&&(identical(other.uploadUrl, uploadUrl) || other.uploadUrl == uploadUrl)&&(identical(other.objectKey, objectKey) || other.objectKey == objectKey));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,sessionId,uploadUrl,objectKey);

@override
String toString() {
  return 'UploadSession(sessionId: $sessionId, uploadUrl: $uploadUrl, objectKey: $objectKey)';
}


}

/// @nodoc
abstract mixin class $UploadSessionCopyWith<$Res>  {
  factory $UploadSessionCopyWith(UploadSession value, $Res Function(UploadSession) _then) = _$UploadSessionCopyWithImpl;
@useResult
$Res call({
 String sessionId, String uploadUrl, String objectKey
});




}
/// @nodoc
class _$UploadSessionCopyWithImpl<$Res>
    implements $UploadSessionCopyWith<$Res> {
  _$UploadSessionCopyWithImpl(this._self, this._then);

  final UploadSession _self;
  final $Res Function(UploadSession) _then;

/// Create a copy of UploadSession
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? sessionId = null,Object? uploadUrl = null,Object? objectKey = null,}) {
  return _then(_self.copyWith(
sessionId: null == sessionId ? _self.sessionId : sessionId // ignore: cast_nullable_to_non_nullable
as String,uploadUrl: null == uploadUrl ? _self.uploadUrl : uploadUrl // ignore: cast_nullable_to_non_nullable
as String,objectKey: null == objectKey ? _self.objectKey : objectKey // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [UploadSession].
extension UploadSessionPatterns on UploadSession {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _UploadSession value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _UploadSession() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _UploadSession value)  $default,){
final _that = this;
switch (_that) {
case _UploadSession():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _UploadSession value)?  $default,){
final _that = this;
switch (_that) {
case _UploadSession() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String sessionId,  String uploadUrl,  String objectKey)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _UploadSession() when $default != null:
return $default(_that.sessionId,_that.uploadUrl,_that.objectKey);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String sessionId,  String uploadUrl,  String objectKey)  $default,) {final _that = this;
switch (_that) {
case _UploadSession():
return $default(_that.sessionId,_that.uploadUrl,_that.objectKey);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String sessionId,  String uploadUrl,  String objectKey)?  $default,) {final _that = this;
switch (_that) {
case _UploadSession() when $default != null:
return $default(_that.sessionId,_that.uploadUrl,_that.objectKey);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _UploadSession implements UploadSession {
  const _UploadSession({required this.sessionId, this.uploadUrl = '', this.objectKey = ''});
  factory _UploadSession.fromJson(Map<String, dynamic> json) => _$UploadSessionFromJson(json);

/// 上傳會話 ID（UUID）
@override final  String sessionId;
/// OSS 預簽名上傳 URL（isSecure=true 時為空）
@override@JsonKey() final  String uploadUrl;
/// OSS 物件路徑
@override@JsonKey() final  String objectKey;

/// Create a copy of UploadSession
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$UploadSessionCopyWith<_UploadSession> get copyWith => __$UploadSessionCopyWithImpl<_UploadSession>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$UploadSessionToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _UploadSession&&(identical(other.sessionId, sessionId) || other.sessionId == sessionId)&&(identical(other.uploadUrl, uploadUrl) || other.uploadUrl == uploadUrl)&&(identical(other.objectKey, objectKey) || other.objectKey == objectKey));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,sessionId,uploadUrl,objectKey);

@override
String toString() {
  return 'UploadSession(sessionId: $sessionId, uploadUrl: $uploadUrl, objectKey: $objectKey)';
}


}

/// @nodoc
abstract mixin class _$UploadSessionCopyWith<$Res> implements $UploadSessionCopyWith<$Res> {
  factory _$UploadSessionCopyWith(_UploadSession value, $Res Function(_UploadSession) _then) = __$UploadSessionCopyWithImpl;
@override @useResult
$Res call({
 String sessionId, String uploadUrl, String objectKey
});




}
/// @nodoc
class __$UploadSessionCopyWithImpl<$Res>
    implements _$UploadSessionCopyWith<$Res> {
  __$UploadSessionCopyWithImpl(this._self, this._then);

  final _UploadSession _self;
  final $Res Function(_UploadSession) _then;

/// Create a copy of UploadSession
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? sessionId = null,Object? uploadUrl = null,Object? objectKey = null,}) {
  return _then(_UploadSession(
sessionId: null == sessionId ? _self.sessionId : sessionId // ignore: cast_nullable_to_non_nullable
as String,uploadUrl: null == uploadUrl ? _self.uploadUrl : uploadUrl // ignore: cast_nullable_to_non_nullable
as String,objectKey: null == objectKey ? _self.objectKey : objectKey // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$MediaUploadResult {

/// CDN URL（優先使用）
 String get url;/// Gateway URL（備用）
 String get fileUrl;/// 物件路徑
 String get objectKey;/// 檔案大小（bytes）
 int get size;/// 原始大小（壓縮前）
 int? get originalSize;/// 原始檔名
 String? get filename;/// Staged Token（Session 上傳流程產生，用於 finalize / product creation）
 String? get stagedToken;/// 預覽 URL（有效期 24 小時）
 String? get previewUrl;
/// Create a copy of MediaUploadResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$MediaUploadResultCopyWith<MediaUploadResult> get copyWith => _$MediaUploadResultCopyWithImpl<MediaUploadResult>(this as MediaUploadResult, _$identity);

  /// Serializes this MediaUploadResult to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is MediaUploadResult&&(identical(other.url, url) || other.url == url)&&(identical(other.fileUrl, fileUrl) || other.fileUrl == fileUrl)&&(identical(other.objectKey, objectKey) || other.objectKey == objectKey)&&(identical(other.size, size) || other.size == size)&&(identical(other.originalSize, originalSize) || other.originalSize == originalSize)&&(identical(other.filename, filename) || other.filename == filename)&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken)&&(identical(other.previewUrl, previewUrl) || other.previewUrl == previewUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,url,fileUrl,objectKey,size,originalSize,filename,stagedToken,previewUrl);

@override
String toString() {
  return 'MediaUploadResult(url: $url, fileUrl: $fileUrl, objectKey: $objectKey, size: $size, originalSize: $originalSize, filename: $filename, stagedToken: $stagedToken, previewUrl: $previewUrl)';
}


}

/// @nodoc
abstract mixin class $MediaUploadResultCopyWith<$Res>  {
  factory $MediaUploadResultCopyWith(MediaUploadResult value, $Res Function(MediaUploadResult) _then) = _$MediaUploadResultCopyWithImpl;
@useResult
$Res call({
 String url, String fileUrl, String objectKey, int size, int? originalSize, String? filename, String? stagedToken, String? previewUrl
});




}
/// @nodoc
class _$MediaUploadResultCopyWithImpl<$Res>
    implements $MediaUploadResultCopyWith<$Res> {
  _$MediaUploadResultCopyWithImpl(this._self, this._then);

  final MediaUploadResult _self;
  final $Res Function(MediaUploadResult) _then;

/// Create a copy of MediaUploadResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? url = null,Object? fileUrl = null,Object? objectKey = null,Object? size = null,Object? originalSize = freezed,Object? filename = freezed,Object? stagedToken = freezed,Object? previewUrl = freezed,}) {
  return _then(_self.copyWith(
url: null == url ? _self.url : url // ignore: cast_nullable_to_non_nullable
as String,fileUrl: null == fileUrl ? _self.fileUrl : fileUrl // ignore: cast_nullable_to_non_nullable
as String,objectKey: null == objectKey ? _self.objectKey : objectKey // ignore: cast_nullable_to_non_nullable
as String,size: null == size ? _self.size : size // ignore: cast_nullable_to_non_nullable
as int,originalSize: freezed == originalSize ? _self.originalSize : originalSize // ignore: cast_nullable_to_non_nullable
as int?,filename: freezed == filename ? _self.filename : filename // ignore: cast_nullable_to_non_nullable
as String?,stagedToken: freezed == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String?,previewUrl: freezed == previewUrl ? _self.previewUrl : previewUrl // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [MediaUploadResult].
extension MediaUploadResultPatterns on MediaUploadResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _MediaUploadResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _MediaUploadResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _MediaUploadResult value)  $default,){
final _that = this;
switch (_that) {
case _MediaUploadResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _MediaUploadResult value)?  $default,){
final _that = this;
switch (_that) {
case _MediaUploadResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String url,  String fileUrl,  String objectKey,  int size,  int? originalSize,  String? filename,  String? stagedToken,  String? previewUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _MediaUploadResult() when $default != null:
return $default(_that.url,_that.fileUrl,_that.objectKey,_that.size,_that.originalSize,_that.filename,_that.stagedToken,_that.previewUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String url,  String fileUrl,  String objectKey,  int size,  int? originalSize,  String? filename,  String? stagedToken,  String? previewUrl)  $default,) {final _that = this;
switch (_that) {
case _MediaUploadResult():
return $default(_that.url,_that.fileUrl,_that.objectKey,_that.size,_that.originalSize,_that.filename,_that.stagedToken,_that.previewUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String url,  String fileUrl,  String objectKey,  int size,  int? originalSize,  String? filename,  String? stagedToken,  String? previewUrl)?  $default,) {final _that = this;
switch (_that) {
case _MediaUploadResult() when $default != null:
return $default(_that.url,_that.fileUrl,_that.objectKey,_that.size,_that.originalSize,_that.filename,_that.stagedToken,_that.previewUrl);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _MediaUploadResult extends MediaUploadResult {
  const _MediaUploadResult({this.url = '', this.fileUrl = '', required this.objectKey, required this.size, this.originalSize, this.filename, this.stagedToken, this.previewUrl}): super._();
  factory _MediaUploadResult.fromJson(Map<String, dynamic> json) => _$MediaUploadResultFromJson(json);

/// CDN URL（優先使用）
@override@JsonKey() final  String url;
/// Gateway URL（備用）
@override@JsonKey() final  String fileUrl;
/// 物件路徑
@override final  String objectKey;
/// 檔案大小（bytes）
@override final  int size;
/// 原始大小（壓縮前）
@override final  int? originalSize;
/// 原始檔名
@override final  String? filename;
/// Staged Token（Session 上傳流程產生，用於 finalize / product creation）
@override final  String? stagedToken;
/// 預覽 URL（有效期 24 小時）
@override final  String? previewUrl;

/// Create a copy of MediaUploadResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$MediaUploadResultCopyWith<_MediaUploadResult> get copyWith => __$MediaUploadResultCopyWithImpl<_MediaUploadResult>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$MediaUploadResultToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _MediaUploadResult&&(identical(other.url, url) || other.url == url)&&(identical(other.fileUrl, fileUrl) || other.fileUrl == fileUrl)&&(identical(other.objectKey, objectKey) || other.objectKey == objectKey)&&(identical(other.size, size) || other.size == size)&&(identical(other.originalSize, originalSize) || other.originalSize == originalSize)&&(identical(other.filename, filename) || other.filename == filename)&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken)&&(identical(other.previewUrl, previewUrl) || other.previewUrl == previewUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,url,fileUrl,objectKey,size,originalSize,filename,stagedToken,previewUrl);

@override
String toString() {
  return 'MediaUploadResult(url: $url, fileUrl: $fileUrl, objectKey: $objectKey, size: $size, originalSize: $originalSize, filename: $filename, stagedToken: $stagedToken, previewUrl: $previewUrl)';
}


}

/// @nodoc
abstract mixin class _$MediaUploadResultCopyWith<$Res> implements $MediaUploadResultCopyWith<$Res> {
  factory _$MediaUploadResultCopyWith(_MediaUploadResult value, $Res Function(_MediaUploadResult) _then) = __$MediaUploadResultCopyWithImpl;
@override @useResult
$Res call({
 String url, String fileUrl, String objectKey, int size, int? originalSize, String? filename, String? stagedToken, String? previewUrl
});




}
/// @nodoc
class __$MediaUploadResultCopyWithImpl<$Res>
    implements _$MediaUploadResultCopyWith<$Res> {
  __$MediaUploadResultCopyWithImpl(this._self, this._then);

  final _MediaUploadResult _self;
  final $Res Function(_MediaUploadResult) _then;

/// Create a copy of MediaUploadResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? url = null,Object? fileUrl = null,Object? objectKey = null,Object? size = null,Object? originalSize = freezed,Object? filename = freezed,Object? stagedToken = freezed,Object? previewUrl = freezed,}) {
  return _then(_MediaUploadResult(
url: null == url ? _self.url : url // ignore: cast_nullable_to_non_nullable
as String,fileUrl: null == fileUrl ? _self.fileUrl : fileUrl // ignore: cast_nullable_to_non_nullable
as String,objectKey: null == objectKey ? _self.objectKey : objectKey // ignore: cast_nullable_to_non_nullable
as String,size: null == size ? _self.size : size // ignore: cast_nullable_to_non_nullable
as int,originalSize: freezed == originalSize ? _self.originalSize : originalSize // ignore: cast_nullable_to_non_nullable
as int?,filename: freezed == filename ? _self.filename : filename // ignore: cast_nullable_to_non_nullable
as String?,stagedToken: freezed == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String?,previewUrl: freezed == previewUrl ? _self.previewUrl : previewUrl // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}


/// @nodoc
mixin _$PrivateUploadResult {

/// Upload Session ID（用於追蹤）
 String get referenceId;/// 物件路徑
 String get objectKey;/// 原始大小
 int get originalSize;/// 文件類型（id_card, bank_account, contract, document）
 String get docType;/// Staged Token（用於 finalize 轉為正式 asset）
 String? get stagedToken;
/// Create a copy of PrivateUploadResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PrivateUploadResultCopyWith<PrivateUploadResult> get copyWith => _$PrivateUploadResultCopyWithImpl<PrivateUploadResult>(this as PrivateUploadResult, _$identity);

  /// Serializes this PrivateUploadResult to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is PrivateUploadResult&&(identical(other.referenceId, referenceId) || other.referenceId == referenceId)&&(identical(other.objectKey, objectKey) || other.objectKey == objectKey)&&(identical(other.originalSize, originalSize) || other.originalSize == originalSize)&&(identical(other.docType, docType) || other.docType == docType)&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,referenceId,objectKey,originalSize,docType,stagedToken);

@override
String toString() {
  return 'PrivateUploadResult(referenceId: $referenceId, objectKey: $objectKey, originalSize: $originalSize, docType: $docType, stagedToken: $stagedToken)';
}


}

/// @nodoc
abstract mixin class $PrivateUploadResultCopyWith<$Res>  {
  factory $PrivateUploadResultCopyWith(PrivateUploadResult value, $Res Function(PrivateUploadResult) _then) = _$PrivateUploadResultCopyWithImpl;
@useResult
$Res call({
 String referenceId, String objectKey, int originalSize, String docType, String? stagedToken
});




}
/// @nodoc
class _$PrivateUploadResultCopyWithImpl<$Res>
    implements $PrivateUploadResultCopyWith<$Res> {
  _$PrivateUploadResultCopyWithImpl(this._self, this._then);

  final PrivateUploadResult _self;
  final $Res Function(PrivateUploadResult) _then;

/// Create a copy of PrivateUploadResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? referenceId = null,Object? objectKey = null,Object? originalSize = null,Object? docType = null,Object? stagedToken = freezed,}) {
  return _then(_self.copyWith(
referenceId: null == referenceId ? _self.referenceId : referenceId // ignore: cast_nullable_to_non_nullable
as String,objectKey: null == objectKey ? _self.objectKey : objectKey // ignore: cast_nullable_to_non_nullable
as String,originalSize: null == originalSize ? _self.originalSize : originalSize // ignore: cast_nullable_to_non_nullable
as int,docType: null == docType ? _self.docType : docType // ignore: cast_nullable_to_non_nullable
as String,stagedToken: freezed == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [PrivateUploadResult].
extension PrivateUploadResultPatterns on PrivateUploadResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _PrivateUploadResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _PrivateUploadResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _PrivateUploadResult value)  $default,){
final _that = this;
switch (_that) {
case _PrivateUploadResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _PrivateUploadResult value)?  $default,){
final _that = this;
switch (_that) {
case _PrivateUploadResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String referenceId,  String objectKey,  int originalSize,  String docType,  String? stagedToken)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _PrivateUploadResult() when $default != null:
return $default(_that.referenceId,_that.objectKey,_that.originalSize,_that.docType,_that.stagedToken);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String referenceId,  String objectKey,  int originalSize,  String docType,  String? stagedToken)  $default,) {final _that = this;
switch (_that) {
case _PrivateUploadResult():
return $default(_that.referenceId,_that.objectKey,_that.originalSize,_that.docType,_that.stagedToken);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String referenceId,  String objectKey,  int originalSize,  String docType,  String? stagedToken)?  $default,) {final _that = this;
switch (_that) {
case _PrivateUploadResult() when $default != null:
return $default(_that.referenceId,_that.objectKey,_that.originalSize,_that.docType,_that.stagedToken);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _PrivateUploadResult implements PrivateUploadResult {
  const _PrivateUploadResult({required this.referenceId, required this.objectKey, required this.originalSize, required this.docType, this.stagedToken});
  factory _PrivateUploadResult.fromJson(Map<String, dynamic> json) => _$PrivateUploadResultFromJson(json);

/// Upload Session ID（用於追蹤）
@override final  String referenceId;
/// 物件路徑
@override final  String objectKey;
/// 原始大小
@override final  int originalSize;
/// 文件類型（id_card, bank_account, contract, document）
@override final  String docType;
/// Staged Token（用於 finalize 轉為正式 asset）
@override final  String? stagedToken;

/// Create a copy of PrivateUploadResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PrivateUploadResultCopyWith<_PrivateUploadResult> get copyWith => __$PrivateUploadResultCopyWithImpl<_PrivateUploadResult>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$PrivateUploadResultToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _PrivateUploadResult&&(identical(other.referenceId, referenceId) || other.referenceId == referenceId)&&(identical(other.objectKey, objectKey) || other.objectKey == objectKey)&&(identical(other.originalSize, originalSize) || other.originalSize == originalSize)&&(identical(other.docType, docType) || other.docType == docType)&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,referenceId,objectKey,originalSize,docType,stagedToken);

@override
String toString() {
  return 'PrivateUploadResult(referenceId: $referenceId, objectKey: $objectKey, originalSize: $originalSize, docType: $docType, stagedToken: $stagedToken)';
}


}

/// @nodoc
abstract mixin class _$PrivateUploadResultCopyWith<$Res> implements $PrivateUploadResultCopyWith<$Res> {
  factory _$PrivateUploadResultCopyWith(_PrivateUploadResult value, $Res Function(_PrivateUploadResult) _then) = __$PrivateUploadResultCopyWithImpl;
@override @useResult
$Res call({
 String referenceId, String objectKey, int originalSize, String docType, String? stagedToken
});




}
/// @nodoc
class __$PrivateUploadResultCopyWithImpl<$Res>
    implements _$PrivateUploadResultCopyWith<$Res> {
  __$PrivateUploadResultCopyWithImpl(this._self, this._then);

  final _PrivateUploadResult _self;
  final $Res Function(_PrivateUploadResult) _then;

/// Create a copy of PrivateUploadResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? referenceId = null,Object? objectKey = null,Object? originalSize = null,Object? docType = null,Object? stagedToken = freezed,}) {
  return _then(_PrivateUploadResult(
referenceId: null == referenceId ? _self.referenceId : referenceId // ignore: cast_nullable_to_non_nullable
as String,objectKey: null == objectKey ? _self.objectKey : objectKey // ignore: cast_nullable_to_non_nullable
as String,originalSize: null == originalSize ? _self.originalSize : originalSize // ignore: cast_nullable_to_non_nullable
as int,docType: null == docType ? _self.docType : docType // ignore: cast_nullable_to_non_nullable
as String,stagedToken: freezed == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}


/// @nodoc
mixin _$FinalizedAsset {

 String get stagedToken; String get assetId; String get objectKey;
/// Create a copy of FinalizedAsset
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$FinalizedAssetCopyWith<FinalizedAsset> get copyWith => _$FinalizedAssetCopyWithImpl<FinalizedAsset>(this as FinalizedAsset, _$identity);

  /// Serializes this FinalizedAsset to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is FinalizedAsset&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken)&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.objectKey, objectKey) || other.objectKey == objectKey));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,stagedToken,assetId,objectKey);

@override
String toString() {
  return 'FinalizedAsset(stagedToken: $stagedToken, assetId: $assetId, objectKey: $objectKey)';
}


}

/// @nodoc
abstract mixin class $FinalizedAssetCopyWith<$Res>  {
  factory $FinalizedAssetCopyWith(FinalizedAsset value, $Res Function(FinalizedAsset) _then) = _$FinalizedAssetCopyWithImpl;
@useResult
$Res call({
 String stagedToken, String assetId, String objectKey
});




}
/// @nodoc
class _$FinalizedAssetCopyWithImpl<$Res>
    implements $FinalizedAssetCopyWith<$Res> {
  _$FinalizedAssetCopyWithImpl(this._self, this._then);

  final FinalizedAsset _self;
  final $Res Function(FinalizedAsset) _then;

/// Create a copy of FinalizedAsset
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? stagedToken = null,Object? assetId = null,Object? objectKey = null,}) {
  return _then(_self.copyWith(
stagedToken: null == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String,assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,objectKey: null == objectKey ? _self.objectKey : objectKey // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [FinalizedAsset].
extension FinalizedAssetPatterns on FinalizedAsset {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _FinalizedAsset value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _FinalizedAsset() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _FinalizedAsset value)  $default,){
final _that = this;
switch (_that) {
case _FinalizedAsset():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _FinalizedAsset value)?  $default,){
final _that = this;
switch (_that) {
case _FinalizedAsset() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String stagedToken,  String assetId,  String objectKey)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _FinalizedAsset() when $default != null:
return $default(_that.stagedToken,_that.assetId,_that.objectKey);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String stagedToken,  String assetId,  String objectKey)  $default,) {final _that = this;
switch (_that) {
case _FinalizedAsset():
return $default(_that.stagedToken,_that.assetId,_that.objectKey);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String stagedToken,  String assetId,  String objectKey)?  $default,) {final _that = this;
switch (_that) {
case _FinalizedAsset() when $default != null:
return $default(_that.stagedToken,_that.assetId,_that.objectKey);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _FinalizedAsset implements FinalizedAsset {
  const _FinalizedAsset({required this.stagedToken, required this.assetId, required this.objectKey});
  factory _FinalizedAsset.fromJson(Map<String, dynamic> json) => _$FinalizedAssetFromJson(json);

@override final  String stagedToken;
@override final  String assetId;
@override final  String objectKey;

/// Create a copy of FinalizedAsset
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$FinalizedAssetCopyWith<_FinalizedAsset> get copyWith => __$FinalizedAssetCopyWithImpl<_FinalizedAsset>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$FinalizedAssetToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _FinalizedAsset&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken)&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.objectKey, objectKey) || other.objectKey == objectKey));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,stagedToken,assetId,objectKey);

@override
String toString() {
  return 'FinalizedAsset(stagedToken: $stagedToken, assetId: $assetId, objectKey: $objectKey)';
}


}

/// @nodoc
abstract mixin class _$FinalizedAssetCopyWith<$Res> implements $FinalizedAssetCopyWith<$Res> {
  factory _$FinalizedAssetCopyWith(_FinalizedAsset value, $Res Function(_FinalizedAsset) _then) = __$FinalizedAssetCopyWithImpl;
@override @useResult
$Res call({
 String stagedToken, String assetId, String objectKey
});




}
/// @nodoc
class __$FinalizedAssetCopyWithImpl<$Res>
    implements _$FinalizedAssetCopyWith<$Res> {
  __$FinalizedAssetCopyWithImpl(this._self, this._then);

  final _FinalizedAsset _self;
  final $Res Function(_FinalizedAsset) _then;

/// Create a copy of FinalizedAsset
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? stagedToken = null,Object? assetId = null,Object? objectKey = null,}) {
  return _then(_FinalizedAsset(
stagedToken: null == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String,assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,objectKey: null == objectKey ? _self.objectKey : objectKey // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
