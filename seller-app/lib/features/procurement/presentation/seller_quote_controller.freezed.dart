// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'seller_quote_controller.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$SellerQuoteNotice {

 String get message; SellerQuoteNoticeTone get tone;/// ⛔ `STALE_SELLER_RESPONSE` 等「依據已過期」的錯誤會把它設成 true：
/// 必須明確告知賣家報價已變動，並要求他自己重新載入，
/// **不得靜默改成送最新版**。
 bool get requiresReload;/// CodeMeta 錯誤碼，沒有時是空字串（給測試與紀錄用）。
 String get code;
/// Create a copy of SellerQuoteNotice
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerQuoteNoticeCopyWith<SellerQuoteNotice> get copyWith => _$SellerQuoteNoticeCopyWithImpl<SellerQuoteNotice>(this as SellerQuoteNotice, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerQuoteNotice&&(identical(other.message, message) || other.message == message)&&(identical(other.tone, tone) || other.tone == tone)&&(identical(other.requiresReload, requiresReload) || other.requiresReload == requiresReload)&&(identical(other.code, code) || other.code == code));
}


@override
int get hashCode => Object.hash(runtimeType,message,tone,requiresReload,code);

@override
String toString() {
  return 'SellerQuoteNotice(message: $message, tone: $tone, requiresReload: $requiresReload, code: $code)';
}


}

/// @nodoc
abstract mixin class $SellerQuoteNoticeCopyWith<$Res>  {
  factory $SellerQuoteNoticeCopyWith(SellerQuoteNotice value, $Res Function(SellerQuoteNotice) _then) = _$SellerQuoteNoticeCopyWithImpl;
@useResult
$Res call({
 String message, SellerQuoteNoticeTone tone, bool requiresReload, String code
});




}
/// @nodoc
class _$SellerQuoteNoticeCopyWithImpl<$Res>
    implements $SellerQuoteNoticeCopyWith<$Res> {
  _$SellerQuoteNoticeCopyWithImpl(this._self, this._then);

  final SellerQuoteNotice _self;
  final $Res Function(SellerQuoteNotice) _then;

/// Create a copy of SellerQuoteNotice
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? message = null,Object? tone = null,Object? requiresReload = null,Object? code = null,}) {
  return _then(_self.copyWith(
message: null == message ? _self.message : message // ignore: cast_nullable_to_non_nullable
as String,tone: null == tone ? _self.tone : tone // ignore: cast_nullable_to_non_nullable
as SellerQuoteNoticeTone,requiresReload: null == requiresReload ? _self.requiresReload : requiresReload // ignore: cast_nullable_to_non_nullable
as bool,code: null == code ? _self.code : code // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [SellerQuoteNotice].
extension SellerQuoteNoticePatterns on SellerQuoteNotice {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerQuoteNotice value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerQuoteNotice() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerQuoteNotice value)  $default,){
final _that = this;
switch (_that) {
case _SellerQuoteNotice():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerQuoteNotice value)?  $default,){
final _that = this;
switch (_that) {
case _SellerQuoteNotice() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String message,  SellerQuoteNoticeTone tone,  bool requiresReload,  String code)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerQuoteNotice() when $default != null:
return $default(_that.message,_that.tone,_that.requiresReload,_that.code);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String message,  SellerQuoteNoticeTone tone,  bool requiresReload,  String code)  $default,) {final _that = this;
switch (_that) {
case _SellerQuoteNotice():
return $default(_that.message,_that.tone,_that.requiresReload,_that.code);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String message,  SellerQuoteNoticeTone tone,  bool requiresReload,  String code)?  $default,) {final _that = this;
switch (_that) {
case _SellerQuoteNotice() when $default != null:
return $default(_that.message,_that.tone,_that.requiresReload,_that.code);case _:
  return null;

}
}

}

/// @nodoc


class _SellerQuoteNotice implements SellerQuoteNotice {
  const _SellerQuoteNotice({required this.message, this.tone = SellerQuoteNoticeTone.info, this.requiresReload = false, this.code = ''});
  

@override final  String message;
@override@JsonKey() final  SellerQuoteNoticeTone tone;
/// ⛔ `STALE_SELLER_RESPONSE` 等「依據已過期」的錯誤會把它設成 true：
/// 必須明確告知賣家報價已變動，並要求他自己重新載入，
/// **不得靜默改成送最新版**。
@override@JsonKey() final  bool requiresReload;
/// CodeMeta 錯誤碼，沒有時是空字串（給測試與紀錄用）。
@override@JsonKey() final  String code;

/// Create a copy of SellerQuoteNotice
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerQuoteNoticeCopyWith<_SellerQuoteNotice> get copyWith => __$SellerQuoteNoticeCopyWithImpl<_SellerQuoteNotice>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerQuoteNotice&&(identical(other.message, message) || other.message == message)&&(identical(other.tone, tone) || other.tone == tone)&&(identical(other.requiresReload, requiresReload) || other.requiresReload == requiresReload)&&(identical(other.code, code) || other.code == code));
}


@override
int get hashCode => Object.hash(runtimeType,message,tone,requiresReload,code);

@override
String toString() {
  return 'SellerQuoteNotice(message: $message, tone: $tone, requiresReload: $requiresReload, code: $code)';
}


}

/// @nodoc
abstract mixin class _$SellerQuoteNoticeCopyWith<$Res> implements $SellerQuoteNoticeCopyWith<$Res> {
  factory _$SellerQuoteNoticeCopyWith(_SellerQuoteNotice value, $Res Function(_SellerQuoteNotice) _then) = __$SellerQuoteNoticeCopyWithImpl;
@override @useResult
$Res call({
 String message, SellerQuoteNoticeTone tone, bool requiresReload, String code
});




}
/// @nodoc
class __$SellerQuoteNoticeCopyWithImpl<$Res>
    implements _$SellerQuoteNoticeCopyWith<$Res> {
  __$SellerQuoteNoticeCopyWithImpl(this._self, this._then);

  final _SellerQuoteNotice _self;
  final $Res Function(_SellerQuoteNotice) _then;

/// Create a copy of SellerQuoteNotice
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? message = null,Object? tone = null,Object? requiresReload = null,Object? code = null,}) {
  return _then(_SellerQuoteNotice(
message: null == message ? _self.message : message // ignore: cast_nullable_to_non_nullable
as String,tone: null == tone ? _self.tone : tone // ignore: cast_nullable_to_non_nullable
as SellerQuoteNoticeTone,requiresReload: null == requiresReload ? _self.requiresReload : requiresReload // ignore: cast_nullable_to_non_nullable
as bool,code: null == code ? _self.code : code // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$SellerQuoteFormState {

 SellerFacingPublication get publication;/// 賣家看得到的需求內容（九欄，deny-by-default）。
 SellerFacingPayload get sellerFacingPayload;/// 表單目前的十七欄內容。
 SellerQuotePayload get quote;/// 草稿 CAS：`""` ＝ 還沒有草稿，下一次 PUT 會建立第一版。
 String get draftVersion;/// 伺服器上最新的正式回價（詳情的 `latestResponse`）；null ＝ 我還沒有正式回價。
 SellerResponseVersionResponse? get latestResponse;/// 有命令在飛。
 bool get busy;/// 目前的橫幅訊息。
 SellerQuoteNotice? get notice;/// 已經按過送出（決定要不要把欄位錯誤紅字攤開）。
 bool get submitAttempted;
/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SellerQuoteFormStateCopyWith<SellerQuoteFormState> get copyWith => _$SellerQuoteFormStateCopyWithImpl<SellerQuoteFormState>(this as SellerQuoteFormState, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SellerQuoteFormState&&(identical(other.publication, publication) || other.publication == publication)&&(identical(other.sellerFacingPayload, sellerFacingPayload) || other.sellerFacingPayload == sellerFacingPayload)&&(identical(other.quote, quote) || other.quote == quote)&&(identical(other.draftVersion, draftVersion) || other.draftVersion == draftVersion)&&(identical(other.latestResponse, latestResponse) || other.latestResponse == latestResponse)&&(identical(other.busy, busy) || other.busy == busy)&&(identical(other.notice, notice) || other.notice == notice)&&(identical(other.submitAttempted, submitAttempted) || other.submitAttempted == submitAttempted));
}


@override
int get hashCode => Object.hash(runtimeType,publication,sellerFacingPayload,quote,draftVersion,latestResponse,busy,notice,submitAttempted);

@override
String toString() {
  return 'SellerQuoteFormState(publication: $publication, sellerFacingPayload: $sellerFacingPayload, quote: $quote, draftVersion: $draftVersion, latestResponse: $latestResponse, busy: $busy, notice: $notice, submitAttempted: $submitAttempted)';
}


}

/// @nodoc
abstract mixin class $SellerQuoteFormStateCopyWith<$Res>  {
  factory $SellerQuoteFormStateCopyWith(SellerQuoteFormState value, $Res Function(SellerQuoteFormState) _then) = _$SellerQuoteFormStateCopyWithImpl;
@useResult
$Res call({
 SellerFacingPublication publication, SellerFacingPayload sellerFacingPayload, SellerQuotePayload quote, String draftVersion, SellerResponseVersionResponse? latestResponse, bool busy, SellerQuoteNotice? notice, bool submitAttempted
});


$SellerFacingPublicationCopyWith<$Res> get publication;$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload;$SellerQuotePayloadCopyWith<$Res> get quote;$SellerResponseVersionResponseCopyWith<$Res>? get latestResponse;$SellerQuoteNoticeCopyWith<$Res>? get notice;

}
/// @nodoc
class _$SellerQuoteFormStateCopyWithImpl<$Res>
    implements $SellerQuoteFormStateCopyWith<$Res> {
  _$SellerQuoteFormStateCopyWithImpl(this._self, this._then);

  final SellerQuoteFormState _self;
  final $Res Function(SellerQuoteFormState) _then;

/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? publication = null,Object? sellerFacingPayload = null,Object? quote = null,Object? draftVersion = null,Object? latestResponse = freezed,Object? busy = null,Object? notice = freezed,Object? submitAttempted = null,}) {
  return _then(_self.copyWith(
publication: null == publication ? _self.publication : publication // ignore: cast_nullable_to_non_nullable
as SellerFacingPublication,sellerFacingPayload: null == sellerFacingPayload ? _self.sellerFacingPayload : sellerFacingPayload // ignore: cast_nullable_to_non_nullable
as SellerFacingPayload,quote: null == quote ? _self.quote : quote // ignore: cast_nullable_to_non_nullable
as SellerQuotePayload,draftVersion: null == draftVersion ? _self.draftVersion : draftVersion // ignore: cast_nullable_to_non_nullable
as String,latestResponse: freezed == latestResponse ? _self.latestResponse : latestResponse // ignore: cast_nullable_to_non_nullable
as SellerResponseVersionResponse?,busy: null == busy ? _self.busy : busy // ignore: cast_nullable_to_non_nullable
as bool,notice: freezed == notice ? _self.notice : notice // ignore: cast_nullable_to_non_nullable
as SellerQuoteNotice?,submitAttempted: null == submitAttempted ? _self.submitAttempted : submitAttempted // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}
/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPublicationCopyWith<$Res> get publication {
  
  return $SellerFacingPublicationCopyWith<$Res>(_self.publication, (value) {
    return _then(_self.copyWith(publication: value));
  });
}/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload {
  
  return $SellerFacingPayloadCopyWith<$Res>(_self.sellerFacingPayload, (value) {
    return _then(_self.copyWith(sellerFacingPayload: value));
  });
}/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerQuotePayloadCopyWith<$Res> get quote {
  
  return $SellerQuotePayloadCopyWith<$Res>(_self.quote, (value) {
    return _then(_self.copyWith(quote: value));
  });
}/// Create a copy of SellerQuoteFormState
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
}/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerQuoteNoticeCopyWith<$Res>? get notice {
    if (_self.notice == null) {
    return null;
  }

  return $SellerQuoteNoticeCopyWith<$Res>(_self.notice!, (value) {
    return _then(_self.copyWith(notice: value));
  });
}
}


/// Adds pattern-matching-related methods to [SellerQuoteFormState].
extension SellerQuoteFormStatePatterns on SellerQuoteFormState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SellerQuoteFormState value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SellerQuoteFormState() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SellerQuoteFormState value)  $default,){
final _that = this;
switch (_that) {
case _SellerQuoteFormState():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SellerQuoteFormState value)?  $default,){
final _that = this;
switch (_that) {
case _SellerQuoteFormState() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload,  SellerQuotePayload quote,  String draftVersion,  SellerResponseVersionResponse? latestResponse,  bool busy,  SellerQuoteNotice? notice,  bool submitAttempted)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SellerQuoteFormState() when $default != null:
return $default(_that.publication,_that.sellerFacingPayload,_that.quote,_that.draftVersion,_that.latestResponse,_that.busy,_that.notice,_that.submitAttempted);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload,  SellerQuotePayload quote,  String draftVersion,  SellerResponseVersionResponse? latestResponse,  bool busy,  SellerQuoteNotice? notice,  bool submitAttempted)  $default,) {final _that = this;
switch (_that) {
case _SellerQuoteFormState():
return $default(_that.publication,_that.sellerFacingPayload,_that.quote,_that.draftVersion,_that.latestResponse,_that.busy,_that.notice,_that.submitAttempted);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( SellerFacingPublication publication,  SellerFacingPayload sellerFacingPayload,  SellerQuotePayload quote,  String draftVersion,  SellerResponseVersionResponse? latestResponse,  bool busy,  SellerQuoteNotice? notice,  bool submitAttempted)?  $default,) {final _that = this;
switch (_that) {
case _SellerQuoteFormState() when $default != null:
return $default(_that.publication,_that.sellerFacingPayload,_that.quote,_that.draftVersion,_that.latestResponse,_that.busy,_that.notice,_that.submitAttempted);case _:
  return null;

}
}

}

/// @nodoc


class _SellerQuoteFormState extends SellerQuoteFormState {
  const _SellerQuoteFormState({required this.publication, this.sellerFacingPayload = const SellerFacingPayload(), required this.quote, this.draftVersion = '', this.latestResponse, this.busy = false, this.notice, this.submitAttempted = false}): super._();
  

@override final  SellerFacingPublication publication;
/// 賣家看得到的需求內容（九欄，deny-by-default）。
@override@JsonKey() final  SellerFacingPayload sellerFacingPayload;
/// 表單目前的十七欄內容。
@override final  SellerQuotePayload quote;
/// 草稿 CAS：`""` ＝ 還沒有草稿，下一次 PUT 會建立第一版。
@override@JsonKey() final  String draftVersion;
/// 伺服器上最新的正式回價（詳情的 `latestResponse`）；null ＝ 我還沒有正式回價。
@override final  SellerResponseVersionResponse? latestResponse;
/// 有命令在飛。
@override@JsonKey() final  bool busy;
/// 目前的橫幅訊息。
@override final  SellerQuoteNotice? notice;
/// 已經按過送出（決定要不要把欄位錯誤紅字攤開）。
@override@JsonKey() final  bool submitAttempted;

/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SellerQuoteFormStateCopyWith<_SellerQuoteFormState> get copyWith => __$SellerQuoteFormStateCopyWithImpl<_SellerQuoteFormState>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SellerQuoteFormState&&(identical(other.publication, publication) || other.publication == publication)&&(identical(other.sellerFacingPayload, sellerFacingPayload) || other.sellerFacingPayload == sellerFacingPayload)&&(identical(other.quote, quote) || other.quote == quote)&&(identical(other.draftVersion, draftVersion) || other.draftVersion == draftVersion)&&(identical(other.latestResponse, latestResponse) || other.latestResponse == latestResponse)&&(identical(other.busy, busy) || other.busy == busy)&&(identical(other.notice, notice) || other.notice == notice)&&(identical(other.submitAttempted, submitAttempted) || other.submitAttempted == submitAttempted));
}


@override
int get hashCode => Object.hash(runtimeType,publication,sellerFacingPayload,quote,draftVersion,latestResponse,busy,notice,submitAttempted);

@override
String toString() {
  return 'SellerQuoteFormState(publication: $publication, sellerFacingPayload: $sellerFacingPayload, quote: $quote, draftVersion: $draftVersion, latestResponse: $latestResponse, busy: $busy, notice: $notice, submitAttempted: $submitAttempted)';
}


}

/// @nodoc
abstract mixin class _$SellerQuoteFormStateCopyWith<$Res> implements $SellerQuoteFormStateCopyWith<$Res> {
  factory _$SellerQuoteFormStateCopyWith(_SellerQuoteFormState value, $Res Function(_SellerQuoteFormState) _then) = __$SellerQuoteFormStateCopyWithImpl;
@override @useResult
$Res call({
 SellerFacingPublication publication, SellerFacingPayload sellerFacingPayload, SellerQuotePayload quote, String draftVersion, SellerResponseVersionResponse? latestResponse, bool busy, SellerQuoteNotice? notice, bool submitAttempted
});


@override $SellerFacingPublicationCopyWith<$Res> get publication;@override $SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload;@override $SellerQuotePayloadCopyWith<$Res> get quote;@override $SellerResponseVersionResponseCopyWith<$Res>? get latestResponse;@override $SellerQuoteNoticeCopyWith<$Res>? get notice;

}
/// @nodoc
class __$SellerQuoteFormStateCopyWithImpl<$Res>
    implements _$SellerQuoteFormStateCopyWith<$Res> {
  __$SellerQuoteFormStateCopyWithImpl(this._self, this._then);

  final _SellerQuoteFormState _self;
  final $Res Function(_SellerQuoteFormState) _then;

/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? publication = null,Object? sellerFacingPayload = null,Object? quote = null,Object? draftVersion = null,Object? latestResponse = freezed,Object? busy = null,Object? notice = freezed,Object? submitAttempted = null,}) {
  return _then(_SellerQuoteFormState(
publication: null == publication ? _self.publication : publication // ignore: cast_nullable_to_non_nullable
as SellerFacingPublication,sellerFacingPayload: null == sellerFacingPayload ? _self.sellerFacingPayload : sellerFacingPayload // ignore: cast_nullable_to_non_nullable
as SellerFacingPayload,quote: null == quote ? _self.quote : quote // ignore: cast_nullable_to_non_nullable
as SellerQuotePayload,draftVersion: null == draftVersion ? _self.draftVersion : draftVersion // ignore: cast_nullable_to_non_nullable
as String,latestResponse: freezed == latestResponse ? _self.latestResponse : latestResponse // ignore: cast_nullable_to_non_nullable
as SellerResponseVersionResponse?,busy: null == busy ? _self.busy : busy // ignore: cast_nullable_to_non_nullable
as bool,notice: freezed == notice ? _self.notice : notice // ignore: cast_nullable_to_non_nullable
as SellerQuoteNotice?,submitAttempted: null == submitAttempted ? _self.submitAttempted : submitAttempted // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPublicationCopyWith<$Res> get publication {
  
  return $SellerFacingPublicationCopyWith<$Res>(_self.publication, (value) {
    return _then(_self.copyWith(publication: value));
  });
}/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerFacingPayloadCopyWith<$Res> get sellerFacingPayload {
  
  return $SellerFacingPayloadCopyWith<$Res>(_self.sellerFacingPayload, (value) {
    return _then(_self.copyWith(sellerFacingPayload: value));
  });
}/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerQuotePayloadCopyWith<$Res> get quote {
  
  return $SellerQuotePayloadCopyWith<$Res>(_self.quote, (value) {
    return _then(_self.copyWith(quote: value));
  });
}/// Create a copy of SellerQuoteFormState
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
}/// Create a copy of SellerQuoteFormState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SellerQuoteNoticeCopyWith<$Res>? get notice {
    if (_self.notice == null) {
    return null;
  }

  return $SellerQuoteNoticeCopyWith<$Res>(_self.notice!, (value) {
    return _then(_self.copyWith(notice: value));
  });
}
}

// dart format on
