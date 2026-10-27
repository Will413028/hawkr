// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'after_sale_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$AfterSaleCase {

 String get caseNo; String get orderNo; String get buyerId; String get storeId; String get buyerName;/// 買家頭像顯示 URL（Gateway 組）。契約欄名 `avatarUrl`，與 `avatarKey` 成對
/// （49 §6.1；舊名 `buyerAvatarUrl` 已於 2026-08-05 收斂移除）。
 String get avatarUrl; String get shippingAddress; String get temperatureCode; String get status; String get reason; String get description; String get refundableAmount; String get returnShipmentNo; String get returnShippingMethod; String? get shipTradeNo;@JsonKey(readValue: _readCaseValue) String get providerStatus;@JsonKey(readValue: _readCaseValue) String get providerMessage;@JsonKey(readValue: _readCaseValue) String get refundTaskStatus;@JsonKey(readValue: _readCaseValue) String get refundFailureMessage;@JsonKey(readValue: _readCaseValue) String get estimatedRefundDate;@JsonKey(readValue: _readCaseValue) String get bankDate; String? get odno; String? get validationNo; bool get needRefundPage; String get createdAt; String get closedAt; List<AfterSaleItem> get items; List<String> get shipmentPhotoUrls; List<Map<String, dynamic>> get evidence; List<AfterSaleResolution> get resolutions;// B-130 / backend issue #568：處理時限與平台接手狀態
 String get processDeadline; bool get autoEscalated; int get processDeadlineHours;/// 申訴狀態（AppealState：'0'=未申訴；'10' APPEALED / '20' INVESTIGATING
/// 進入平台仲裁）。後端列表 summary 已回傳；賣家卡片需據此把狀態折疊成
/// 「申訴仲裁中」，否則只看 status 會顯示已拒絕 / 待處理。
 String get appealState;
/// Create a copy of AfterSaleCase
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AfterSaleCaseCopyWith<AfterSaleCase> get copyWith => _$AfterSaleCaseCopyWithImpl<AfterSaleCase>(this as AfterSaleCase, _$identity);

  /// Serializes this AfterSaleCase to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AfterSaleCase&&(identical(other.caseNo, caseNo) || other.caseNo == caseNo)&&(identical(other.orderNo, orderNo) || other.orderNo == orderNo)&&(identical(other.buyerId, buyerId) || other.buyerId == buyerId)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.buyerName, buyerName) || other.buyerName == buyerName)&&(identical(other.avatarUrl, avatarUrl) || other.avatarUrl == avatarUrl)&&(identical(other.shippingAddress, shippingAddress) || other.shippingAddress == shippingAddress)&&(identical(other.temperatureCode, temperatureCode) || other.temperatureCode == temperatureCode)&&(identical(other.status, status) || other.status == status)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.description, description) || other.description == description)&&(identical(other.refundableAmount, refundableAmount) || other.refundableAmount == refundableAmount)&&(identical(other.returnShipmentNo, returnShipmentNo) || other.returnShipmentNo == returnShipmentNo)&&(identical(other.returnShippingMethod, returnShippingMethod) || other.returnShippingMethod == returnShippingMethod)&&(identical(other.shipTradeNo, shipTradeNo) || other.shipTradeNo == shipTradeNo)&&(identical(other.providerStatus, providerStatus) || other.providerStatus == providerStatus)&&(identical(other.providerMessage, providerMessage) || other.providerMessage == providerMessage)&&(identical(other.refundTaskStatus, refundTaskStatus) || other.refundTaskStatus == refundTaskStatus)&&(identical(other.refundFailureMessage, refundFailureMessage) || other.refundFailureMessage == refundFailureMessage)&&(identical(other.estimatedRefundDate, estimatedRefundDate) || other.estimatedRefundDate == estimatedRefundDate)&&(identical(other.bankDate, bankDate) || other.bankDate == bankDate)&&(identical(other.odno, odno) || other.odno == odno)&&(identical(other.validationNo, validationNo) || other.validationNo == validationNo)&&(identical(other.needRefundPage, needRefundPage) || other.needRefundPage == needRefundPage)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.closedAt, closedAt) || other.closedAt == closedAt)&&const DeepCollectionEquality().equals(other.items, items)&&const DeepCollectionEquality().equals(other.shipmentPhotoUrls, shipmentPhotoUrls)&&const DeepCollectionEquality().equals(other.evidence, evidence)&&const DeepCollectionEquality().equals(other.resolutions, resolutions)&&(identical(other.processDeadline, processDeadline) || other.processDeadline == processDeadline)&&(identical(other.autoEscalated, autoEscalated) || other.autoEscalated == autoEscalated)&&(identical(other.processDeadlineHours, processDeadlineHours) || other.processDeadlineHours == processDeadlineHours)&&(identical(other.appealState, appealState) || other.appealState == appealState));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,caseNo,orderNo,buyerId,storeId,buyerName,avatarUrl,shippingAddress,temperatureCode,status,reason,description,refundableAmount,returnShipmentNo,returnShippingMethod,shipTradeNo,providerStatus,providerMessage,refundTaskStatus,refundFailureMessage,estimatedRefundDate,bankDate,odno,validationNo,needRefundPage,createdAt,closedAt,const DeepCollectionEquality().hash(items),const DeepCollectionEquality().hash(shipmentPhotoUrls),const DeepCollectionEquality().hash(evidence),const DeepCollectionEquality().hash(resolutions),processDeadline,autoEscalated,processDeadlineHours,appealState]);

@override
String toString() {
  return 'AfterSaleCase(caseNo: $caseNo, orderNo: $orderNo, buyerId: $buyerId, storeId: $storeId, buyerName: $buyerName, avatarUrl: $avatarUrl, shippingAddress: $shippingAddress, temperatureCode: $temperatureCode, status: $status, reason: $reason, description: $description, refundableAmount: $refundableAmount, returnShipmentNo: $returnShipmentNo, returnShippingMethod: $returnShippingMethod, shipTradeNo: $shipTradeNo, providerStatus: $providerStatus, providerMessage: $providerMessage, refundTaskStatus: $refundTaskStatus, refundFailureMessage: $refundFailureMessage, estimatedRefundDate: $estimatedRefundDate, bankDate: $bankDate, odno: $odno, validationNo: $validationNo, needRefundPage: $needRefundPage, createdAt: $createdAt, closedAt: $closedAt, items: $items, shipmentPhotoUrls: $shipmentPhotoUrls, evidence: $evidence, resolutions: $resolutions, processDeadline: $processDeadline, autoEscalated: $autoEscalated, processDeadlineHours: $processDeadlineHours, appealState: $appealState)';
}


}

/// @nodoc
abstract mixin class $AfterSaleCaseCopyWith<$Res>  {
  factory $AfterSaleCaseCopyWith(AfterSaleCase value, $Res Function(AfterSaleCase) _then) = _$AfterSaleCaseCopyWithImpl;
@useResult
$Res call({
 String caseNo, String orderNo, String buyerId, String storeId, String buyerName, String avatarUrl, String shippingAddress, String temperatureCode, String status, String reason, String description, String refundableAmount, String returnShipmentNo, String returnShippingMethod, String? shipTradeNo,@JsonKey(readValue: _readCaseValue) String providerStatus,@JsonKey(readValue: _readCaseValue) String providerMessage,@JsonKey(readValue: _readCaseValue) String refundTaskStatus,@JsonKey(readValue: _readCaseValue) String refundFailureMessage,@JsonKey(readValue: _readCaseValue) String estimatedRefundDate,@JsonKey(readValue: _readCaseValue) String bankDate, String? odno, String? validationNo, bool needRefundPage, String createdAt, String closedAt, List<AfterSaleItem> items, List<String> shipmentPhotoUrls, List<Map<String, dynamic>> evidence, List<AfterSaleResolution> resolutions, String processDeadline, bool autoEscalated, int processDeadlineHours, String appealState
});




}
/// @nodoc
class _$AfterSaleCaseCopyWithImpl<$Res>
    implements $AfterSaleCaseCopyWith<$Res> {
  _$AfterSaleCaseCopyWithImpl(this._self, this._then);

  final AfterSaleCase _self;
  final $Res Function(AfterSaleCase) _then;

/// Create a copy of AfterSaleCase
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? caseNo = null,Object? orderNo = null,Object? buyerId = null,Object? storeId = null,Object? buyerName = null,Object? avatarUrl = null,Object? shippingAddress = null,Object? temperatureCode = null,Object? status = null,Object? reason = null,Object? description = null,Object? refundableAmount = null,Object? returnShipmentNo = null,Object? returnShippingMethod = null,Object? shipTradeNo = freezed,Object? providerStatus = null,Object? providerMessage = null,Object? refundTaskStatus = null,Object? refundFailureMessage = null,Object? estimatedRefundDate = null,Object? bankDate = null,Object? odno = freezed,Object? validationNo = freezed,Object? needRefundPage = null,Object? createdAt = null,Object? closedAt = null,Object? items = null,Object? shipmentPhotoUrls = null,Object? evidence = null,Object? resolutions = null,Object? processDeadline = null,Object? autoEscalated = null,Object? processDeadlineHours = null,Object? appealState = null,}) {
  return _then(_self.copyWith(
caseNo: null == caseNo ? _self.caseNo : caseNo // ignore: cast_nullable_to_non_nullable
as String,orderNo: null == orderNo ? _self.orderNo : orderNo // ignore: cast_nullable_to_non_nullable
as String,buyerId: null == buyerId ? _self.buyerId : buyerId // ignore: cast_nullable_to_non_nullable
as String,storeId: null == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String,buyerName: null == buyerName ? _self.buyerName : buyerName // ignore: cast_nullable_to_non_nullable
as String,avatarUrl: null == avatarUrl ? _self.avatarUrl : avatarUrl // ignore: cast_nullable_to_non_nullable
as String,shippingAddress: null == shippingAddress ? _self.shippingAddress : shippingAddress // ignore: cast_nullable_to_non_nullable
as String,temperatureCode: null == temperatureCode ? _self.temperatureCode : temperatureCode // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,refundableAmount: null == refundableAmount ? _self.refundableAmount : refundableAmount // ignore: cast_nullable_to_non_nullable
as String,returnShipmentNo: null == returnShipmentNo ? _self.returnShipmentNo : returnShipmentNo // ignore: cast_nullable_to_non_nullable
as String,returnShippingMethod: null == returnShippingMethod ? _self.returnShippingMethod : returnShippingMethod // ignore: cast_nullable_to_non_nullable
as String,shipTradeNo: freezed == shipTradeNo ? _self.shipTradeNo : shipTradeNo // ignore: cast_nullable_to_non_nullable
as String?,providerStatus: null == providerStatus ? _self.providerStatus : providerStatus // ignore: cast_nullable_to_non_nullable
as String,providerMessage: null == providerMessage ? _self.providerMessage : providerMessage // ignore: cast_nullable_to_non_nullable
as String,refundTaskStatus: null == refundTaskStatus ? _self.refundTaskStatus : refundTaskStatus // ignore: cast_nullable_to_non_nullable
as String,refundFailureMessage: null == refundFailureMessage ? _self.refundFailureMessage : refundFailureMessage // ignore: cast_nullable_to_non_nullable
as String,estimatedRefundDate: null == estimatedRefundDate ? _self.estimatedRefundDate : estimatedRefundDate // ignore: cast_nullable_to_non_nullable
as String,bankDate: null == bankDate ? _self.bankDate : bankDate // ignore: cast_nullable_to_non_nullable
as String,odno: freezed == odno ? _self.odno : odno // ignore: cast_nullable_to_non_nullable
as String?,validationNo: freezed == validationNo ? _self.validationNo : validationNo // ignore: cast_nullable_to_non_nullable
as String?,needRefundPage: null == needRefundPage ? _self.needRefundPage : needRefundPage // ignore: cast_nullable_to_non_nullable
as bool,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String,closedAt: null == closedAt ? _self.closedAt : closedAt // ignore: cast_nullable_to_non_nullable
as String,items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as List<AfterSaleItem>,shipmentPhotoUrls: null == shipmentPhotoUrls ? _self.shipmentPhotoUrls : shipmentPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,evidence: null == evidence ? _self.evidence : evidence // ignore: cast_nullable_to_non_nullable
as List<Map<String, dynamic>>,resolutions: null == resolutions ? _self.resolutions : resolutions // ignore: cast_nullable_to_non_nullable
as List<AfterSaleResolution>,processDeadline: null == processDeadline ? _self.processDeadline : processDeadline // ignore: cast_nullable_to_non_nullable
as String,autoEscalated: null == autoEscalated ? _self.autoEscalated : autoEscalated // ignore: cast_nullable_to_non_nullable
as bool,processDeadlineHours: null == processDeadlineHours ? _self.processDeadlineHours : processDeadlineHours // ignore: cast_nullable_to_non_nullable
as int,appealState: null == appealState ? _self.appealState : appealState // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AfterSaleCase].
extension AfterSaleCasePatterns on AfterSaleCase {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AfterSaleCase value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AfterSaleCase() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AfterSaleCase value)  $default,){
final _that = this;
switch (_that) {
case _AfterSaleCase():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AfterSaleCase value)?  $default,){
final _that = this;
switch (_that) {
case _AfterSaleCase() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String caseNo,  String orderNo,  String buyerId,  String storeId,  String buyerName,  String avatarUrl,  String shippingAddress,  String temperatureCode,  String status,  String reason,  String description,  String refundableAmount,  String returnShipmentNo,  String returnShippingMethod,  String? shipTradeNo, @JsonKey(readValue: _readCaseValue)  String providerStatus, @JsonKey(readValue: _readCaseValue)  String providerMessage, @JsonKey(readValue: _readCaseValue)  String refundTaskStatus, @JsonKey(readValue: _readCaseValue)  String refundFailureMessage, @JsonKey(readValue: _readCaseValue)  String estimatedRefundDate, @JsonKey(readValue: _readCaseValue)  String bankDate,  String? odno,  String? validationNo,  bool needRefundPage,  String createdAt,  String closedAt,  List<AfterSaleItem> items,  List<String> shipmentPhotoUrls,  List<Map<String, dynamic>> evidence,  List<AfterSaleResolution> resolutions,  String processDeadline,  bool autoEscalated,  int processDeadlineHours,  String appealState)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AfterSaleCase() when $default != null:
return $default(_that.caseNo,_that.orderNo,_that.buyerId,_that.storeId,_that.buyerName,_that.avatarUrl,_that.shippingAddress,_that.temperatureCode,_that.status,_that.reason,_that.description,_that.refundableAmount,_that.returnShipmentNo,_that.returnShippingMethod,_that.shipTradeNo,_that.providerStatus,_that.providerMessage,_that.refundTaskStatus,_that.refundFailureMessage,_that.estimatedRefundDate,_that.bankDate,_that.odno,_that.validationNo,_that.needRefundPage,_that.createdAt,_that.closedAt,_that.items,_that.shipmentPhotoUrls,_that.evidence,_that.resolutions,_that.processDeadline,_that.autoEscalated,_that.processDeadlineHours,_that.appealState);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String caseNo,  String orderNo,  String buyerId,  String storeId,  String buyerName,  String avatarUrl,  String shippingAddress,  String temperatureCode,  String status,  String reason,  String description,  String refundableAmount,  String returnShipmentNo,  String returnShippingMethod,  String? shipTradeNo, @JsonKey(readValue: _readCaseValue)  String providerStatus, @JsonKey(readValue: _readCaseValue)  String providerMessage, @JsonKey(readValue: _readCaseValue)  String refundTaskStatus, @JsonKey(readValue: _readCaseValue)  String refundFailureMessage, @JsonKey(readValue: _readCaseValue)  String estimatedRefundDate, @JsonKey(readValue: _readCaseValue)  String bankDate,  String? odno,  String? validationNo,  bool needRefundPage,  String createdAt,  String closedAt,  List<AfterSaleItem> items,  List<String> shipmentPhotoUrls,  List<Map<String, dynamic>> evidence,  List<AfterSaleResolution> resolutions,  String processDeadline,  bool autoEscalated,  int processDeadlineHours,  String appealState)  $default,) {final _that = this;
switch (_that) {
case _AfterSaleCase():
return $default(_that.caseNo,_that.orderNo,_that.buyerId,_that.storeId,_that.buyerName,_that.avatarUrl,_that.shippingAddress,_that.temperatureCode,_that.status,_that.reason,_that.description,_that.refundableAmount,_that.returnShipmentNo,_that.returnShippingMethod,_that.shipTradeNo,_that.providerStatus,_that.providerMessage,_that.refundTaskStatus,_that.refundFailureMessage,_that.estimatedRefundDate,_that.bankDate,_that.odno,_that.validationNo,_that.needRefundPage,_that.createdAt,_that.closedAt,_that.items,_that.shipmentPhotoUrls,_that.evidence,_that.resolutions,_that.processDeadline,_that.autoEscalated,_that.processDeadlineHours,_that.appealState);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String caseNo,  String orderNo,  String buyerId,  String storeId,  String buyerName,  String avatarUrl,  String shippingAddress,  String temperatureCode,  String status,  String reason,  String description,  String refundableAmount,  String returnShipmentNo,  String returnShippingMethod,  String? shipTradeNo, @JsonKey(readValue: _readCaseValue)  String providerStatus, @JsonKey(readValue: _readCaseValue)  String providerMessage, @JsonKey(readValue: _readCaseValue)  String refundTaskStatus, @JsonKey(readValue: _readCaseValue)  String refundFailureMessage, @JsonKey(readValue: _readCaseValue)  String estimatedRefundDate, @JsonKey(readValue: _readCaseValue)  String bankDate,  String? odno,  String? validationNo,  bool needRefundPage,  String createdAt,  String closedAt,  List<AfterSaleItem> items,  List<String> shipmentPhotoUrls,  List<Map<String, dynamic>> evidence,  List<AfterSaleResolution> resolutions,  String processDeadline,  bool autoEscalated,  int processDeadlineHours,  String appealState)?  $default,) {final _that = this;
switch (_that) {
case _AfterSaleCase() when $default != null:
return $default(_that.caseNo,_that.orderNo,_that.buyerId,_that.storeId,_that.buyerName,_that.avatarUrl,_that.shippingAddress,_that.temperatureCode,_that.status,_that.reason,_that.description,_that.refundableAmount,_that.returnShipmentNo,_that.returnShippingMethod,_that.shipTradeNo,_that.providerStatus,_that.providerMessage,_that.refundTaskStatus,_that.refundFailureMessage,_that.estimatedRefundDate,_that.bankDate,_that.odno,_that.validationNo,_that.needRefundPage,_that.createdAt,_that.closedAt,_that.items,_that.shipmentPhotoUrls,_that.evidence,_that.resolutions,_that.processDeadline,_that.autoEscalated,_that.processDeadlineHours,_that.appealState);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AfterSaleCase implements AfterSaleCase {
  const _AfterSaleCase({this.caseNo = '', this.orderNo = '', this.buyerId = '', this.storeId = '', this.buyerName = '', this.avatarUrl = '', this.shippingAddress = '', this.temperatureCode = '', this.status = '', this.reason = '', this.description = '', this.refundableAmount = '', this.returnShipmentNo = '', this.returnShippingMethod = '', this.shipTradeNo, @JsonKey(readValue: _readCaseValue) this.providerStatus = '', @JsonKey(readValue: _readCaseValue) this.providerMessage = '', @JsonKey(readValue: _readCaseValue) this.refundTaskStatus = '', @JsonKey(readValue: _readCaseValue) this.refundFailureMessage = '', @JsonKey(readValue: _readCaseValue) this.estimatedRefundDate = '', @JsonKey(readValue: _readCaseValue) this.bankDate = '', this.odno, this.validationNo, this.needRefundPage = false, this.createdAt = '', this.closedAt = '', final  List<AfterSaleItem> items = const [], final  List<String> shipmentPhotoUrls = const [], final  List<Map<String, dynamic>> evidence = const [], final  List<AfterSaleResolution> resolutions = const [], this.processDeadline = '', this.autoEscalated = false, this.processDeadlineHours = 0, this.appealState = ''}): _items = items,_shipmentPhotoUrls = shipmentPhotoUrls,_evidence = evidence,_resolutions = resolutions;
  factory _AfterSaleCase.fromJson(Map<String, dynamic> json) => _$AfterSaleCaseFromJson(json);

@override@JsonKey() final  String caseNo;
@override@JsonKey() final  String orderNo;
@override@JsonKey() final  String buyerId;
@override@JsonKey() final  String storeId;
@override@JsonKey() final  String buyerName;
/// 買家頭像顯示 URL（Gateway 組）。契約欄名 `avatarUrl`，與 `avatarKey` 成對
/// （49 §6.1；舊名 `buyerAvatarUrl` 已於 2026-08-05 收斂移除）。
@override@JsonKey() final  String avatarUrl;
@override@JsonKey() final  String shippingAddress;
@override@JsonKey() final  String temperatureCode;
@override@JsonKey() final  String status;
@override@JsonKey() final  String reason;
@override@JsonKey() final  String description;
@override@JsonKey() final  String refundableAmount;
@override@JsonKey() final  String returnShipmentNo;
@override@JsonKey() final  String returnShippingMethod;
@override final  String? shipTradeNo;
@override@JsonKey(readValue: _readCaseValue) final  String providerStatus;
@override@JsonKey(readValue: _readCaseValue) final  String providerMessage;
@override@JsonKey(readValue: _readCaseValue) final  String refundTaskStatus;
@override@JsonKey(readValue: _readCaseValue) final  String refundFailureMessage;
@override@JsonKey(readValue: _readCaseValue) final  String estimatedRefundDate;
@override@JsonKey(readValue: _readCaseValue) final  String bankDate;
@override final  String? odno;
@override final  String? validationNo;
@override@JsonKey() final  bool needRefundPage;
@override@JsonKey() final  String createdAt;
@override@JsonKey() final  String closedAt;
 final  List<AfterSaleItem> _items;
@override@JsonKey() List<AfterSaleItem> get items {
  if (_items is EqualUnmodifiableListView) return _items;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_items);
}

 final  List<String> _shipmentPhotoUrls;
@override@JsonKey() List<String> get shipmentPhotoUrls {
  if (_shipmentPhotoUrls is EqualUnmodifiableListView) return _shipmentPhotoUrls;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_shipmentPhotoUrls);
}

 final  List<Map<String, dynamic>> _evidence;
@override@JsonKey() List<Map<String, dynamic>> get evidence {
  if (_evidence is EqualUnmodifiableListView) return _evidence;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_evidence);
}

 final  List<AfterSaleResolution> _resolutions;
@override@JsonKey() List<AfterSaleResolution> get resolutions {
  if (_resolutions is EqualUnmodifiableListView) return _resolutions;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_resolutions);
}

// B-130 / backend issue #568：處理時限與平台接手狀態
@override@JsonKey() final  String processDeadline;
@override@JsonKey() final  bool autoEscalated;
@override@JsonKey() final  int processDeadlineHours;
/// 申訴狀態（AppealState：'0'=未申訴；'10' APPEALED / '20' INVESTIGATING
/// 進入平台仲裁）。後端列表 summary 已回傳；賣家卡片需據此把狀態折疊成
/// 「申訴仲裁中」，否則只看 status 會顯示已拒絕 / 待處理。
@override@JsonKey() final  String appealState;

/// Create a copy of AfterSaleCase
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AfterSaleCaseCopyWith<_AfterSaleCase> get copyWith => __$AfterSaleCaseCopyWithImpl<_AfterSaleCase>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$AfterSaleCaseToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AfterSaleCase&&(identical(other.caseNo, caseNo) || other.caseNo == caseNo)&&(identical(other.orderNo, orderNo) || other.orderNo == orderNo)&&(identical(other.buyerId, buyerId) || other.buyerId == buyerId)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.buyerName, buyerName) || other.buyerName == buyerName)&&(identical(other.avatarUrl, avatarUrl) || other.avatarUrl == avatarUrl)&&(identical(other.shippingAddress, shippingAddress) || other.shippingAddress == shippingAddress)&&(identical(other.temperatureCode, temperatureCode) || other.temperatureCode == temperatureCode)&&(identical(other.status, status) || other.status == status)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.description, description) || other.description == description)&&(identical(other.refundableAmount, refundableAmount) || other.refundableAmount == refundableAmount)&&(identical(other.returnShipmentNo, returnShipmentNo) || other.returnShipmentNo == returnShipmentNo)&&(identical(other.returnShippingMethod, returnShippingMethod) || other.returnShippingMethod == returnShippingMethod)&&(identical(other.shipTradeNo, shipTradeNo) || other.shipTradeNo == shipTradeNo)&&(identical(other.providerStatus, providerStatus) || other.providerStatus == providerStatus)&&(identical(other.providerMessage, providerMessage) || other.providerMessage == providerMessage)&&(identical(other.refundTaskStatus, refundTaskStatus) || other.refundTaskStatus == refundTaskStatus)&&(identical(other.refundFailureMessage, refundFailureMessage) || other.refundFailureMessage == refundFailureMessage)&&(identical(other.estimatedRefundDate, estimatedRefundDate) || other.estimatedRefundDate == estimatedRefundDate)&&(identical(other.bankDate, bankDate) || other.bankDate == bankDate)&&(identical(other.odno, odno) || other.odno == odno)&&(identical(other.validationNo, validationNo) || other.validationNo == validationNo)&&(identical(other.needRefundPage, needRefundPage) || other.needRefundPage == needRefundPage)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.closedAt, closedAt) || other.closedAt == closedAt)&&const DeepCollectionEquality().equals(other._items, _items)&&const DeepCollectionEquality().equals(other._shipmentPhotoUrls, _shipmentPhotoUrls)&&const DeepCollectionEquality().equals(other._evidence, _evidence)&&const DeepCollectionEquality().equals(other._resolutions, _resolutions)&&(identical(other.processDeadline, processDeadline) || other.processDeadline == processDeadline)&&(identical(other.autoEscalated, autoEscalated) || other.autoEscalated == autoEscalated)&&(identical(other.processDeadlineHours, processDeadlineHours) || other.processDeadlineHours == processDeadlineHours)&&(identical(other.appealState, appealState) || other.appealState == appealState));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,caseNo,orderNo,buyerId,storeId,buyerName,avatarUrl,shippingAddress,temperatureCode,status,reason,description,refundableAmount,returnShipmentNo,returnShippingMethod,shipTradeNo,providerStatus,providerMessage,refundTaskStatus,refundFailureMessage,estimatedRefundDate,bankDate,odno,validationNo,needRefundPage,createdAt,closedAt,const DeepCollectionEquality().hash(_items),const DeepCollectionEquality().hash(_shipmentPhotoUrls),const DeepCollectionEquality().hash(_evidence),const DeepCollectionEquality().hash(_resolutions),processDeadline,autoEscalated,processDeadlineHours,appealState]);

@override
String toString() {
  return 'AfterSaleCase(caseNo: $caseNo, orderNo: $orderNo, buyerId: $buyerId, storeId: $storeId, buyerName: $buyerName, avatarUrl: $avatarUrl, shippingAddress: $shippingAddress, temperatureCode: $temperatureCode, status: $status, reason: $reason, description: $description, refundableAmount: $refundableAmount, returnShipmentNo: $returnShipmentNo, returnShippingMethod: $returnShippingMethod, shipTradeNo: $shipTradeNo, providerStatus: $providerStatus, providerMessage: $providerMessage, refundTaskStatus: $refundTaskStatus, refundFailureMessage: $refundFailureMessage, estimatedRefundDate: $estimatedRefundDate, bankDate: $bankDate, odno: $odno, validationNo: $validationNo, needRefundPage: $needRefundPage, createdAt: $createdAt, closedAt: $closedAt, items: $items, shipmentPhotoUrls: $shipmentPhotoUrls, evidence: $evidence, resolutions: $resolutions, processDeadline: $processDeadline, autoEscalated: $autoEscalated, processDeadlineHours: $processDeadlineHours, appealState: $appealState)';
}


}

/// @nodoc
abstract mixin class _$AfterSaleCaseCopyWith<$Res> implements $AfterSaleCaseCopyWith<$Res> {
  factory _$AfterSaleCaseCopyWith(_AfterSaleCase value, $Res Function(_AfterSaleCase) _then) = __$AfterSaleCaseCopyWithImpl;
@override @useResult
$Res call({
 String caseNo, String orderNo, String buyerId, String storeId, String buyerName, String avatarUrl, String shippingAddress, String temperatureCode, String status, String reason, String description, String refundableAmount, String returnShipmentNo, String returnShippingMethod, String? shipTradeNo,@JsonKey(readValue: _readCaseValue) String providerStatus,@JsonKey(readValue: _readCaseValue) String providerMessage,@JsonKey(readValue: _readCaseValue) String refundTaskStatus,@JsonKey(readValue: _readCaseValue) String refundFailureMessage,@JsonKey(readValue: _readCaseValue) String estimatedRefundDate,@JsonKey(readValue: _readCaseValue) String bankDate, String? odno, String? validationNo, bool needRefundPage, String createdAt, String closedAt, List<AfterSaleItem> items, List<String> shipmentPhotoUrls, List<Map<String, dynamic>> evidence, List<AfterSaleResolution> resolutions, String processDeadline, bool autoEscalated, int processDeadlineHours, String appealState
});




}
/// @nodoc
class __$AfterSaleCaseCopyWithImpl<$Res>
    implements _$AfterSaleCaseCopyWith<$Res> {
  __$AfterSaleCaseCopyWithImpl(this._self, this._then);

  final _AfterSaleCase _self;
  final $Res Function(_AfterSaleCase) _then;

/// Create a copy of AfterSaleCase
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? caseNo = null,Object? orderNo = null,Object? buyerId = null,Object? storeId = null,Object? buyerName = null,Object? avatarUrl = null,Object? shippingAddress = null,Object? temperatureCode = null,Object? status = null,Object? reason = null,Object? description = null,Object? refundableAmount = null,Object? returnShipmentNo = null,Object? returnShippingMethod = null,Object? shipTradeNo = freezed,Object? providerStatus = null,Object? providerMessage = null,Object? refundTaskStatus = null,Object? refundFailureMessage = null,Object? estimatedRefundDate = null,Object? bankDate = null,Object? odno = freezed,Object? validationNo = freezed,Object? needRefundPage = null,Object? createdAt = null,Object? closedAt = null,Object? items = null,Object? shipmentPhotoUrls = null,Object? evidence = null,Object? resolutions = null,Object? processDeadline = null,Object? autoEscalated = null,Object? processDeadlineHours = null,Object? appealState = null,}) {
  return _then(_AfterSaleCase(
caseNo: null == caseNo ? _self.caseNo : caseNo // ignore: cast_nullable_to_non_nullable
as String,orderNo: null == orderNo ? _self.orderNo : orderNo // ignore: cast_nullable_to_non_nullable
as String,buyerId: null == buyerId ? _self.buyerId : buyerId // ignore: cast_nullable_to_non_nullable
as String,storeId: null == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String,buyerName: null == buyerName ? _self.buyerName : buyerName // ignore: cast_nullable_to_non_nullable
as String,avatarUrl: null == avatarUrl ? _self.avatarUrl : avatarUrl // ignore: cast_nullable_to_non_nullable
as String,shippingAddress: null == shippingAddress ? _self.shippingAddress : shippingAddress // ignore: cast_nullable_to_non_nullable
as String,temperatureCode: null == temperatureCode ? _self.temperatureCode : temperatureCode // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,refundableAmount: null == refundableAmount ? _self.refundableAmount : refundableAmount // ignore: cast_nullable_to_non_nullable
as String,returnShipmentNo: null == returnShipmentNo ? _self.returnShipmentNo : returnShipmentNo // ignore: cast_nullable_to_non_nullable
as String,returnShippingMethod: null == returnShippingMethod ? _self.returnShippingMethod : returnShippingMethod // ignore: cast_nullable_to_non_nullable
as String,shipTradeNo: freezed == shipTradeNo ? _self.shipTradeNo : shipTradeNo // ignore: cast_nullable_to_non_nullable
as String?,providerStatus: null == providerStatus ? _self.providerStatus : providerStatus // ignore: cast_nullable_to_non_nullable
as String,providerMessage: null == providerMessage ? _self.providerMessage : providerMessage // ignore: cast_nullable_to_non_nullable
as String,refundTaskStatus: null == refundTaskStatus ? _self.refundTaskStatus : refundTaskStatus // ignore: cast_nullable_to_non_nullable
as String,refundFailureMessage: null == refundFailureMessage ? _self.refundFailureMessage : refundFailureMessage // ignore: cast_nullable_to_non_nullable
as String,estimatedRefundDate: null == estimatedRefundDate ? _self.estimatedRefundDate : estimatedRefundDate // ignore: cast_nullable_to_non_nullable
as String,bankDate: null == bankDate ? _self.bankDate : bankDate // ignore: cast_nullable_to_non_nullable
as String,odno: freezed == odno ? _self.odno : odno // ignore: cast_nullable_to_non_nullable
as String?,validationNo: freezed == validationNo ? _self.validationNo : validationNo // ignore: cast_nullable_to_non_nullable
as String?,needRefundPage: null == needRefundPage ? _self.needRefundPage : needRefundPage // ignore: cast_nullable_to_non_nullable
as bool,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String,closedAt: null == closedAt ? _self.closedAt : closedAt // ignore: cast_nullable_to_non_nullable
as String,items: null == items ? _self._items : items // ignore: cast_nullable_to_non_nullable
as List<AfterSaleItem>,shipmentPhotoUrls: null == shipmentPhotoUrls ? _self._shipmentPhotoUrls : shipmentPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,evidence: null == evidence ? _self._evidence : evidence // ignore: cast_nullable_to_non_nullable
as List<Map<String, dynamic>>,resolutions: null == resolutions ? _self._resolutions : resolutions // ignore: cast_nullable_to_non_nullable
as List<AfterSaleResolution>,processDeadline: null == processDeadline ? _self.processDeadline : processDeadline // ignore: cast_nullable_to_non_nullable
as String,autoEscalated: null == autoEscalated ? _self.autoEscalated : autoEscalated // ignore: cast_nullable_to_non_nullable
as bool,processDeadlineHours: null == processDeadlineHours ? _self.processDeadlineHours : processDeadlineHours // ignore: cast_nullable_to_non_nullable
as int,appealState: null == appealState ? _self.appealState : appealState // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$AfterSaleItem {

 String get orderItemId; String get skuId; String get skuName; String get quantity;// PayUni #758: 新欄位（成交單價 / 原價）
 String get unitPrice; String get listPrice;/// Deprecated: 舊欄位 fallback。
 String get originalPrice; String get actualPayAmount; String get shareDiscount;/// 商品圖 objectKey（SSOT；49 §6.1）。Gateway 成對回 imageKey / imageUrl，
/// 後者是它由 key 組出的衍生物
/// （`aftersales.go` 的 `ImageUrl: buildURL(mediaBase, item.ImageKey)`），
/// 顯示 URL 交回 buildImageUrl 唯一組裝入口。
 String get imageKey;
/// Create a copy of AfterSaleItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AfterSaleItemCopyWith<AfterSaleItem> get copyWith => _$AfterSaleItemCopyWithImpl<AfterSaleItem>(this as AfterSaleItem, _$identity);

  /// Serializes this AfterSaleItem to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AfterSaleItem&&(identical(other.orderItemId, orderItemId) || other.orderItemId == orderItemId)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.skuName, skuName) || other.skuName == skuName)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unitPrice, unitPrice) || other.unitPrice == unitPrice)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.originalPrice, originalPrice) || other.originalPrice == originalPrice)&&(identical(other.actualPayAmount, actualPayAmount) || other.actualPayAmount == actualPayAmount)&&(identical(other.shareDiscount, shareDiscount) || other.shareDiscount == shareDiscount)&&(identical(other.imageKey, imageKey) || other.imageKey == imageKey));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,orderItemId,skuId,skuName,quantity,unitPrice,listPrice,originalPrice,actualPayAmount,shareDiscount,imageKey);

@override
String toString() {
  return 'AfterSaleItem(orderItemId: $orderItemId, skuId: $skuId, skuName: $skuName, quantity: $quantity, unitPrice: $unitPrice, listPrice: $listPrice, originalPrice: $originalPrice, actualPayAmount: $actualPayAmount, shareDiscount: $shareDiscount, imageKey: $imageKey)';
}


}

/// @nodoc
abstract mixin class $AfterSaleItemCopyWith<$Res>  {
  factory $AfterSaleItemCopyWith(AfterSaleItem value, $Res Function(AfterSaleItem) _then) = _$AfterSaleItemCopyWithImpl;
@useResult
$Res call({
 String orderItemId, String skuId, String skuName, String quantity, String unitPrice, String listPrice, String originalPrice, String actualPayAmount, String shareDiscount, String imageKey
});




}
/// @nodoc
class _$AfterSaleItemCopyWithImpl<$Res>
    implements $AfterSaleItemCopyWith<$Res> {
  _$AfterSaleItemCopyWithImpl(this._self, this._then);

  final AfterSaleItem _self;
  final $Res Function(AfterSaleItem) _then;

/// Create a copy of AfterSaleItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? orderItemId = null,Object? skuId = null,Object? skuName = null,Object? quantity = null,Object? unitPrice = null,Object? listPrice = null,Object? originalPrice = null,Object? actualPayAmount = null,Object? shareDiscount = null,Object? imageKey = null,}) {
  return _then(_self.copyWith(
orderItemId: null == orderItemId ? _self.orderItemId : orderItemId // ignore: cast_nullable_to_non_nullable
as String,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as String,skuName: null == skuName ? _self.skuName : skuName // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as String,unitPrice: null == unitPrice ? _self.unitPrice : unitPrice // ignore: cast_nullable_to_non_nullable
as String,listPrice: null == listPrice ? _self.listPrice : listPrice // ignore: cast_nullable_to_non_nullable
as String,originalPrice: null == originalPrice ? _self.originalPrice : originalPrice // ignore: cast_nullable_to_non_nullable
as String,actualPayAmount: null == actualPayAmount ? _self.actualPayAmount : actualPayAmount // ignore: cast_nullable_to_non_nullable
as String,shareDiscount: null == shareDiscount ? _self.shareDiscount : shareDiscount // ignore: cast_nullable_to_non_nullable
as String,imageKey: null == imageKey ? _self.imageKey : imageKey // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AfterSaleItem].
extension AfterSaleItemPatterns on AfterSaleItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AfterSaleItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AfterSaleItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AfterSaleItem value)  $default,){
final _that = this;
switch (_that) {
case _AfterSaleItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AfterSaleItem value)?  $default,){
final _that = this;
switch (_that) {
case _AfterSaleItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String orderItemId,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String shareDiscount,  String imageKey)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AfterSaleItem() when $default != null:
return $default(_that.orderItemId,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.shareDiscount,_that.imageKey);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String orderItemId,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String shareDiscount,  String imageKey)  $default,) {final _that = this;
switch (_that) {
case _AfterSaleItem():
return $default(_that.orderItemId,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.shareDiscount,_that.imageKey);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String orderItemId,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String shareDiscount,  String imageKey)?  $default,) {final _that = this;
switch (_that) {
case _AfterSaleItem() when $default != null:
return $default(_that.orderItemId,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.shareDiscount,_that.imageKey);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AfterSaleItem implements AfterSaleItem {
  const _AfterSaleItem({this.orderItemId = '', this.skuId = '', this.skuName = '', this.quantity = '', this.unitPrice = '', this.listPrice = '', this.originalPrice = '', this.actualPayAmount = '', this.shareDiscount = '', this.imageKey = ''});
  factory _AfterSaleItem.fromJson(Map<String, dynamic> json) => _$AfterSaleItemFromJson(json);

@override@JsonKey() final  String orderItemId;
@override@JsonKey() final  String skuId;
@override@JsonKey() final  String skuName;
@override@JsonKey() final  String quantity;
// PayUni #758: 新欄位（成交單價 / 原價）
@override@JsonKey() final  String unitPrice;
@override@JsonKey() final  String listPrice;
/// Deprecated: 舊欄位 fallback。
@override@JsonKey() final  String originalPrice;
@override@JsonKey() final  String actualPayAmount;
@override@JsonKey() final  String shareDiscount;
/// 商品圖 objectKey（SSOT；49 §6.1）。Gateway 成對回 imageKey / imageUrl，
/// 後者是它由 key 組出的衍生物
/// （`aftersales.go` 的 `ImageUrl: buildURL(mediaBase, item.ImageKey)`），
/// 顯示 URL 交回 buildImageUrl 唯一組裝入口。
@override@JsonKey() final  String imageKey;

/// Create a copy of AfterSaleItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AfterSaleItemCopyWith<_AfterSaleItem> get copyWith => __$AfterSaleItemCopyWithImpl<_AfterSaleItem>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$AfterSaleItemToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AfterSaleItem&&(identical(other.orderItemId, orderItemId) || other.orderItemId == orderItemId)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.skuName, skuName) || other.skuName == skuName)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unitPrice, unitPrice) || other.unitPrice == unitPrice)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.originalPrice, originalPrice) || other.originalPrice == originalPrice)&&(identical(other.actualPayAmount, actualPayAmount) || other.actualPayAmount == actualPayAmount)&&(identical(other.shareDiscount, shareDiscount) || other.shareDiscount == shareDiscount)&&(identical(other.imageKey, imageKey) || other.imageKey == imageKey));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,orderItemId,skuId,skuName,quantity,unitPrice,listPrice,originalPrice,actualPayAmount,shareDiscount,imageKey);

@override
String toString() {
  return 'AfterSaleItem(orderItemId: $orderItemId, skuId: $skuId, skuName: $skuName, quantity: $quantity, unitPrice: $unitPrice, listPrice: $listPrice, originalPrice: $originalPrice, actualPayAmount: $actualPayAmount, shareDiscount: $shareDiscount, imageKey: $imageKey)';
}


}

/// @nodoc
abstract mixin class _$AfterSaleItemCopyWith<$Res> implements $AfterSaleItemCopyWith<$Res> {
  factory _$AfterSaleItemCopyWith(_AfterSaleItem value, $Res Function(_AfterSaleItem) _then) = __$AfterSaleItemCopyWithImpl;
@override @useResult
$Res call({
 String orderItemId, String skuId, String skuName, String quantity, String unitPrice, String listPrice, String originalPrice, String actualPayAmount, String shareDiscount, String imageKey
});




}
/// @nodoc
class __$AfterSaleItemCopyWithImpl<$Res>
    implements _$AfterSaleItemCopyWith<$Res> {
  __$AfterSaleItemCopyWithImpl(this._self, this._then);

  final _AfterSaleItem _self;
  final $Res Function(_AfterSaleItem) _then;

/// Create a copy of AfterSaleItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? orderItemId = null,Object? skuId = null,Object? skuName = null,Object? quantity = null,Object? unitPrice = null,Object? listPrice = null,Object? originalPrice = null,Object? actualPayAmount = null,Object? shareDiscount = null,Object? imageKey = null,}) {
  return _then(_AfterSaleItem(
orderItemId: null == orderItemId ? _self.orderItemId : orderItemId // ignore: cast_nullable_to_non_nullable
as String,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as String,skuName: null == skuName ? _self.skuName : skuName // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as String,unitPrice: null == unitPrice ? _self.unitPrice : unitPrice // ignore: cast_nullable_to_non_nullable
as String,listPrice: null == listPrice ? _self.listPrice : listPrice // ignore: cast_nullable_to_non_nullable
as String,originalPrice: null == originalPrice ? _self.originalPrice : originalPrice // ignore: cast_nullable_to_non_nullable
as String,actualPayAmount: null == actualPayAmount ? _self.actualPayAmount : actualPayAmount // ignore: cast_nullable_to_non_nullable
as String,shareDiscount: null == shareDiscount ? _self.shareDiscount : shareDiscount // ignore: cast_nullable_to_non_nullable
as String,imageKey: null == imageKey ? _self.imageKey : imageKey // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$AfterSaleResolution {

 String get resolutionType; String get refundAmount; String get couponAmount; String get couponMinSpend; String get couponMessage; String get rejectReason; String get rejectMessage; String get negotiationReason; String get createdAt;
/// Create a copy of AfterSaleResolution
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AfterSaleResolutionCopyWith<AfterSaleResolution> get copyWith => _$AfterSaleResolutionCopyWithImpl<AfterSaleResolution>(this as AfterSaleResolution, _$identity);

  /// Serializes this AfterSaleResolution to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AfterSaleResolution&&(identical(other.resolutionType, resolutionType) || other.resolutionType == resolutionType)&&(identical(other.refundAmount, refundAmount) || other.refundAmount == refundAmount)&&(identical(other.couponAmount, couponAmount) || other.couponAmount == couponAmount)&&(identical(other.couponMinSpend, couponMinSpend) || other.couponMinSpend == couponMinSpend)&&(identical(other.couponMessage, couponMessage) || other.couponMessage == couponMessage)&&(identical(other.rejectReason, rejectReason) || other.rejectReason == rejectReason)&&(identical(other.rejectMessage, rejectMessage) || other.rejectMessage == rejectMessage)&&(identical(other.negotiationReason, negotiationReason) || other.negotiationReason == negotiationReason)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,resolutionType,refundAmount,couponAmount,couponMinSpend,couponMessage,rejectReason,rejectMessage,negotiationReason,createdAt);

@override
String toString() {
  return 'AfterSaleResolution(resolutionType: $resolutionType, refundAmount: $refundAmount, couponAmount: $couponAmount, couponMinSpend: $couponMinSpend, couponMessage: $couponMessage, rejectReason: $rejectReason, rejectMessage: $rejectMessage, negotiationReason: $negotiationReason, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class $AfterSaleResolutionCopyWith<$Res>  {
  factory $AfterSaleResolutionCopyWith(AfterSaleResolution value, $Res Function(AfterSaleResolution) _then) = _$AfterSaleResolutionCopyWithImpl;
@useResult
$Res call({
 String resolutionType, String refundAmount, String couponAmount, String couponMinSpend, String couponMessage, String rejectReason, String rejectMessage, String negotiationReason, String createdAt
});




}
/// @nodoc
class _$AfterSaleResolutionCopyWithImpl<$Res>
    implements $AfterSaleResolutionCopyWith<$Res> {
  _$AfterSaleResolutionCopyWithImpl(this._self, this._then);

  final AfterSaleResolution _self;
  final $Res Function(AfterSaleResolution) _then;

/// Create a copy of AfterSaleResolution
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? resolutionType = null,Object? refundAmount = null,Object? couponAmount = null,Object? couponMinSpend = null,Object? couponMessage = null,Object? rejectReason = null,Object? rejectMessage = null,Object? negotiationReason = null,Object? createdAt = null,}) {
  return _then(_self.copyWith(
resolutionType: null == resolutionType ? _self.resolutionType : resolutionType // ignore: cast_nullable_to_non_nullable
as String,refundAmount: null == refundAmount ? _self.refundAmount : refundAmount // ignore: cast_nullable_to_non_nullable
as String,couponAmount: null == couponAmount ? _self.couponAmount : couponAmount // ignore: cast_nullable_to_non_nullable
as String,couponMinSpend: null == couponMinSpend ? _self.couponMinSpend : couponMinSpend // ignore: cast_nullable_to_non_nullable
as String,couponMessage: null == couponMessage ? _self.couponMessage : couponMessage // ignore: cast_nullable_to_non_nullable
as String,rejectReason: null == rejectReason ? _self.rejectReason : rejectReason // ignore: cast_nullable_to_non_nullable
as String,rejectMessage: null == rejectMessage ? _self.rejectMessage : rejectMessage // ignore: cast_nullable_to_non_nullable
as String,negotiationReason: null == negotiationReason ? _self.negotiationReason : negotiationReason // ignore: cast_nullable_to_non_nullable
as String,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AfterSaleResolution].
extension AfterSaleResolutionPatterns on AfterSaleResolution {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AfterSaleResolution value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AfterSaleResolution() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AfterSaleResolution value)  $default,){
final _that = this;
switch (_that) {
case _AfterSaleResolution():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AfterSaleResolution value)?  $default,){
final _that = this;
switch (_that) {
case _AfterSaleResolution() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String resolutionType,  String refundAmount,  String couponAmount,  String couponMinSpend,  String couponMessage,  String rejectReason,  String rejectMessage,  String negotiationReason,  String createdAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AfterSaleResolution() when $default != null:
return $default(_that.resolutionType,_that.refundAmount,_that.couponAmount,_that.couponMinSpend,_that.couponMessage,_that.rejectReason,_that.rejectMessage,_that.negotiationReason,_that.createdAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String resolutionType,  String refundAmount,  String couponAmount,  String couponMinSpend,  String couponMessage,  String rejectReason,  String rejectMessage,  String negotiationReason,  String createdAt)  $default,) {final _that = this;
switch (_that) {
case _AfterSaleResolution():
return $default(_that.resolutionType,_that.refundAmount,_that.couponAmount,_that.couponMinSpend,_that.couponMessage,_that.rejectReason,_that.rejectMessage,_that.negotiationReason,_that.createdAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String resolutionType,  String refundAmount,  String couponAmount,  String couponMinSpend,  String couponMessage,  String rejectReason,  String rejectMessage,  String negotiationReason,  String createdAt)?  $default,) {final _that = this;
switch (_that) {
case _AfterSaleResolution() when $default != null:
return $default(_that.resolutionType,_that.refundAmount,_that.couponAmount,_that.couponMinSpend,_that.couponMessage,_that.rejectReason,_that.rejectMessage,_that.negotiationReason,_that.createdAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AfterSaleResolution implements AfterSaleResolution {
  const _AfterSaleResolution({this.resolutionType = '', this.refundAmount = '', this.couponAmount = '', this.couponMinSpend = '', this.couponMessage = '', this.rejectReason = '', this.rejectMessage = '', this.negotiationReason = '', this.createdAt = ''});
  factory _AfterSaleResolution.fromJson(Map<String, dynamic> json) => _$AfterSaleResolutionFromJson(json);

@override@JsonKey() final  String resolutionType;
@override@JsonKey() final  String refundAmount;
@override@JsonKey() final  String couponAmount;
@override@JsonKey() final  String couponMinSpend;
@override@JsonKey() final  String couponMessage;
@override@JsonKey() final  String rejectReason;
@override@JsonKey() final  String rejectMessage;
@override@JsonKey() final  String negotiationReason;
@override@JsonKey() final  String createdAt;

/// Create a copy of AfterSaleResolution
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AfterSaleResolutionCopyWith<_AfterSaleResolution> get copyWith => __$AfterSaleResolutionCopyWithImpl<_AfterSaleResolution>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$AfterSaleResolutionToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AfterSaleResolution&&(identical(other.resolutionType, resolutionType) || other.resolutionType == resolutionType)&&(identical(other.refundAmount, refundAmount) || other.refundAmount == refundAmount)&&(identical(other.couponAmount, couponAmount) || other.couponAmount == couponAmount)&&(identical(other.couponMinSpend, couponMinSpend) || other.couponMinSpend == couponMinSpend)&&(identical(other.couponMessage, couponMessage) || other.couponMessage == couponMessage)&&(identical(other.rejectReason, rejectReason) || other.rejectReason == rejectReason)&&(identical(other.rejectMessage, rejectMessage) || other.rejectMessage == rejectMessage)&&(identical(other.negotiationReason, negotiationReason) || other.negotiationReason == negotiationReason)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,resolutionType,refundAmount,couponAmount,couponMinSpend,couponMessage,rejectReason,rejectMessage,negotiationReason,createdAt);

@override
String toString() {
  return 'AfterSaleResolution(resolutionType: $resolutionType, refundAmount: $refundAmount, couponAmount: $couponAmount, couponMinSpend: $couponMinSpend, couponMessage: $couponMessage, rejectReason: $rejectReason, rejectMessage: $rejectMessage, negotiationReason: $negotiationReason, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class _$AfterSaleResolutionCopyWith<$Res> implements $AfterSaleResolutionCopyWith<$Res> {
  factory _$AfterSaleResolutionCopyWith(_AfterSaleResolution value, $Res Function(_AfterSaleResolution) _then) = __$AfterSaleResolutionCopyWithImpl;
@override @useResult
$Res call({
 String resolutionType, String refundAmount, String couponAmount, String couponMinSpend, String couponMessage, String rejectReason, String rejectMessage, String negotiationReason, String createdAt
});




}
/// @nodoc
class __$AfterSaleResolutionCopyWithImpl<$Res>
    implements _$AfterSaleResolutionCopyWith<$Res> {
  __$AfterSaleResolutionCopyWithImpl(this._self, this._then);

  final _AfterSaleResolution _self;
  final $Res Function(_AfterSaleResolution) _then;

/// Create a copy of AfterSaleResolution
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? resolutionType = null,Object? refundAmount = null,Object? couponAmount = null,Object? couponMinSpend = null,Object? couponMessage = null,Object? rejectReason = null,Object? rejectMessage = null,Object? negotiationReason = null,Object? createdAt = null,}) {
  return _then(_AfterSaleResolution(
resolutionType: null == resolutionType ? _self.resolutionType : resolutionType // ignore: cast_nullable_to_non_nullable
as String,refundAmount: null == refundAmount ? _self.refundAmount : refundAmount // ignore: cast_nullable_to_non_nullable
as String,couponAmount: null == couponAmount ? _self.couponAmount : couponAmount // ignore: cast_nullable_to_non_nullable
as String,couponMinSpend: null == couponMinSpend ? _self.couponMinSpend : couponMinSpend // ignore: cast_nullable_to_non_nullable
as String,couponMessage: null == couponMessage ? _self.couponMessage : couponMessage // ignore: cast_nullable_to_non_nullable
as String,rejectReason: null == rejectReason ? _self.rejectReason : rejectReason // ignore: cast_nullable_to_non_nullable
as String,rejectMessage: null == rejectMessage ? _self.rejectMessage : rejectMessage // ignore: cast_nullable_to_non_nullable
as String,negotiationReason: null == negotiationReason ? _self.negotiationReason : negotiationReason // ignore: cast_nullable_to_non_nullable
as String,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
