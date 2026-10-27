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

 String get caseNo; String get orderNo; String get buyerId; String get storeId; String get status; String get reason; String get description; String get refundableAmount; String get returnShipmentNo; String get returnShippingMethod;/// 寄件交易單號（退貨物流拋單後後端回填，顯示於退貨物流 timeline）
 String get shipTradeNo; bool get needRefundPage; String get createdAt; String get closedAt; List<AfterSaleItem> get items; List<String> get shipmentPhotoUrls; List<AfterSaleEvidence> get evidence; List<AfterSaleResolution> get resolutions;/// 申訴狀態（'0'=未申訴，其他值見後端 AppealState codemeta）
 String get appealState;/// 是否可以發起申訴（賣家拒絕後買家可於期限內申訴）
 bool get canAppeal;// B-130 / backend issue #568：賣家處理時限與平台接手狀態
 String get processDeadline; bool get autoEscalated; int get processDeadlineHours;// B-125 / B-114 退款進度（對齊 backend issue #568 規格）
/// ATM 退款任務狀態（[RefundTaskStatus]）：後端 WorkerTaskStatus，保留舊數字碼相容。
 String get refundTaskStatus;/// 上次填寫失敗訊息（後端已依狀態收斂，前台可直接顯示）
 String get refundFailureMessage;/// 預計入帳日期（YYYY-MM-DD，銀行處理中時顯示）
 String get estimatedRefundDate;/// 實際匯款日期（YYYY-MM-DD，退款完成時顯示）
 String get bankDate; String get providerStatus; String get providerMessage; String get appealDeadline;// B-93 7-11 出貨便代碼（三段碼，買家寄退時憑此到 ibon 列印託運單）
/// 母代碼（3 碼）— 顯示條件：resolutionType=1 + returnShippingMethod=3002 + status≥20
 String get partnerId;/// 出貨單編號（8 碼）
 String get odno;/// C2C 驗證碼（4 碼）
 String get validationNo;/// 決議類型：1=退貨退款、2=部分退款、3=全額退款免退貨
/// （case 層級欄位，供買家側快速判斷顯示條件，與 resolutions 陣列同步）
 String get caseResolutionType;/// 寄回截止時間（ISO 8601 UTC，狀態 20 時由後端回填，預設賣家同意後 7 天）
/// 用於買家側顯示「剩餘寄回時間」倒數與 D-1 警示 banner。
/// status=33 為「待寄回逾時自動關案」終態（無倒數）。
 String get shipBackDeadlineAt;
/// Create a copy of AfterSaleCase
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AfterSaleCaseCopyWith<AfterSaleCase> get copyWith => _$AfterSaleCaseCopyWithImpl<AfterSaleCase>(this as AfterSaleCase, _$identity);

  /// Serializes this AfterSaleCase to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AfterSaleCase&&(identical(other.caseNo, caseNo) || other.caseNo == caseNo)&&(identical(other.orderNo, orderNo) || other.orderNo == orderNo)&&(identical(other.buyerId, buyerId) || other.buyerId == buyerId)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.status, status) || other.status == status)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.description, description) || other.description == description)&&(identical(other.refundableAmount, refundableAmount) || other.refundableAmount == refundableAmount)&&(identical(other.returnShipmentNo, returnShipmentNo) || other.returnShipmentNo == returnShipmentNo)&&(identical(other.returnShippingMethod, returnShippingMethod) || other.returnShippingMethod == returnShippingMethod)&&(identical(other.shipTradeNo, shipTradeNo) || other.shipTradeNo == shipTradeNo)&&(identical(other.needRefundPage, needRefundPage) || other.needRefundPage == needRefundPage)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.closedAt, closedAt) || other.closedAt == closedAt)&&const DeepCollectionEquality().equals(other.items, items)&&const DeepCollectionEquality().equals(other.shipmentPhotoUrls, shipmentPhotoUrls)&&const DeepCollectionEquality().equals(other.evidence, evidence)&&const DeepCollectionEquality().equals(other.resolutions, resolutions)&&(identical(other.appealState, appealState) || other.appealState == appealState)&&(identical(other.canAppeal, canAppeal) || other.canAppeal == canAppeal)&&(identical(other.processDeadline, processDeadline) || other.processDeadline == processDeadline)&&(identical(other.autoEscalated, autoEscalated) || other.autoEscalated == autoEscalated)&&(identical(other.processDeadlineHours, processDeadlineHours) || other.processDeadlineHours == processDeadlineHours)&&(identical(other.refundTaskStatus, refundTaskStatus) || other.refundTaskStatus == refundTaskStatus)&&(identical(other.refundFailureMessage, refundFailureMessage) || other.refundFailureMessage == refundFailureMessage)&&(identical(other.estimatedRefundDate, estimatedRefundDate) || other.estimatedRefundDate == estimatedRefundDate)&&(identical(other.bankDate, bankDate) || other.bankDate == bankDate)&&(identical(other.providerStatus, providerStatus) || other.providerStatus == providerStatus)&&(identical(other.providerMessage, providerMessage) || other.providerMessage == providerMessage)&&(identical(other.appealDeadline, appealDeadline) || other.appealDeadline == appealDeadline)&&(identical(other.partnerId, partnerId) || other.partnerId == partnerId)&&(identical(other.odno, odno) || other.odno == odno)&&(identical(other.validationNo, validationNo) || other.validationNo == validationNo)&&(identical(other.caseResolutionType, caseResolutionType) || other.caseResolutionType == caseResolutionType)&&(identical(other.shipBackDeadlineAt, shipBackDeadlineAt) || other.shipBackDeadlineAt == shipBackDeadlineAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,caseNo,orderNo,buyerId,storeId,status,reason,description,refundableAmount,returnShipmentNo,returnShippingMethod,shipTradeNo,needRefundPage,createdAt,closedAt,const DeepCollectionEquality().hash(items),const DeepCollectionEquality().hash(shipmentPhotoUrls),const DeepCollectionEquality().hash(evidence),const DeepCollectionEquality().hash(resolutions),appealState,canAppeal,processDeadline,autoEscalated,processDeadlineHours,refundTaskStatus,refundFailureMessage,estimatedRefundDate,bankDate,providerStatus,providerMessage,appealDeadline,partnerId,odno,validationNo,caseResolutionType,shipBackDeadlineAt]);

@override
String toString() {
  return 'AfterSaleCase(caseNo: $caseNo, orderNo: $orderNo, buyerId: $buyerId, storeId: $storeId, status: $status, reason: $reason, description: $description, refundableAmount: $refundableAmount, returnShipmentNo: $returnShipmentNo, returnShippingMethod: $returnShippingMethod, shipTradeNo: $shipTradeNo, needRefundPage: $needRefundPage, createdAt: $createdAt, closedAt: $closedAt, items: $items, shipmentPhotoUrls: $shipmentPhotoUrls, evidence: $evidence, resolutions: $resolutions, appealState: $appealState, canAppeal: $canAppeal, processDeadline: $processDeadline, autoEscalated: $autoEscalated, processDeadlineHours: $processDeadlineHours, refundTaskStatus: $refundTaskStatus, refundFailureMessage: $refundFailureMessage, estimatedRefundDate: $estimatedRefundDate, bankDate: $bankDate, providerStatus: $providerStatus, providerMessage: $providerMessage, appealDeadline: $appealDeadline, partnerId: $partnerId, odno: $odno, validationNo: $validationNo, caseResolutionType: $caseResolutionType, shipBackDeadlineAt: $shipBackDeadlineAt)';
}


}

/// @nodoc
abstract mixin class $AfterSaleCaseCopyWith<$Res>  {
  factory $AfterSaleCaseCopyWith(AfterSaleCase value, $Res Function(AfterSaleCase) _then) = _$AfterSaleCaseCopyWithImpl;
@useResult
$Res call({
 String caseNo, String orderNo, String buyerId, String storeId, String status, String reason, String description, String refundableAmount, String returnShipmentNo, String returnShippingMethod, String shipTradeNo, bool needRefundPage, String createdAt, String closedAt, List<AfterSaleItem> items, List<String> shipmentPhotoUrls, List<AfterSaleEvidence> evidence, List<AfterSaleResolution> resolutions, String appealState, bool canAppeal, String processDeadline, bool autoEscalated, int processDeadlineHours, String refundTaskStatus, String refundFailureMessage, String estimatedRefundDate, String bankDate, String providerStatus, String providerMessage, String appealDeadline, String partnerId, String odno, String validationNo, String caseResolutionType, String shipBackDeadlineAt
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
@pragma('vm:prefer-inline') @override $Res call({Object? caseNo = null,Object? orderNo = null,Object? buyerId = null,Object? storeId = null,Object? status = null,Object? reason = null,Object? description = null,Object? refundableAmount = null,Object? returnShipmentNo = null,Object? returnShippingMethod = null,Object? shipTradeNo = null,Object? needRefundPage = null,Object? createdAt = null,Object? closedAt = null,Object? items = null,Object? shipmentPhotoUrls = null,Object? evidence = null,Object? resolutions = null,Object? appealState = null,Object? canAppeal = null,Object? processDeadline = null,Object? autoEscalated = null,Object? processDeadlineHours = null,Object? refundTaskStatus = null,Object? refundFailureMessage = null,Object? estimatedRefundDate = null,Object? bankDate = null,Object? providerStatus = null,Object? providerMessage = null,Object? appealDeadline = null,Object? partnerId = null,Object? odno = null,Object? validationNo = null,Object? caseResolutionType = null,Object? shipBackDeadlineAt = null,}) {
  return _then(_self.copyWith(
caseNo: null == caseNo ? _self.caseNo : caseNo // ignore: cast_nullable_to_non_nullable
as String,orderNo: null == orderNo ? _self.orderNo : orderNo // ignore: cast_nullable_to_non_nullable
as String,buyerId: null == buyerId ? _self.buyerId : buyerId // ignore: cast_nullable_to_non_nullable
as String,storeId: null == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,refundableAmount: null == refundableAmount ? _self.refundableAmount : refundableAmount // ignore: cast_nullable_to_non_nullable
as String,returnShipmentNo: null == returnShipmentNo ? _self.returnShipmentNo : returnShipmentNo // ignore: cast_nullable_to_non_nullable
as String,returnShippingMethod: null == returnShippingMethod ? _self.returnShippingMethod : returnShippingMethod // ignore: cast_nullable_to_non_nullable
as String,shipTradeNo: null == shipTradeNo ? _self.shipTradeNo : shipTradeNo // ignore: cast_nullable_to_non_nullable
as String,needRefundPage: null == needRefundPage ? _self.needRefundPage : needRefundPage // ignore: cast_nullable_to_non_nullable
as bool,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String,closedAt: null == closedAt ? _self.closedAt : closedAt // ignore: cast_nullable_to_non_nullable
as String,items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as List<AfterSaleItem>,shipmentPhotoUrls: null == shipmentPhotoUrls ? _self.shipmentPhotoUrls : shipmentPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,evidence: null == evidence ? _self.evidence : evidence // ignore: cast_nullable_to_non_nullable
as List<AfterSaleEvidence>,resolutions: null == resolutions ? _self.resolutions : resolutions // ignore: cast_nullable_to_non_nullable
as List<AfterSaleResolution>,appealState: null == appealState ? _self.appealState : appealState // ignore: cast_nullable_to_non_nullable
as String,canAppeal: null == canAppeal ? _self.canAppeal : canAppeal // ignore: cast_nullable_to_non_nullable
as bool,processDeadline: null == processDeadline ? _self.processDeadline : processDeadline // ignore: cast_nullable_to_non_nullable
as String,autoEscalated: null == autoEscalated ? _self.autoEscalated : autoEscalated // ignore: cast_nullable_to_non_nullable
as bool,processDeadlineHours: null == processDeadlineHours ? _self.processDeadlineHours : processDeadlineHours // ignore: cast_nullable_to_non_nullable
as int,refundTaskStatus: null == refundTaskStatus ? _self.refundTaskStatus : refundTaskStatus // ignore: cast_nullable_to_non_nullable
as String,refundFailureMessage: null == refundFailureMessage ? _self.refundFailureMessage : refundFailureMessage // ignore: cast_nullable_to_non_nullable
as String,estimatedRefundDate: null == estimatedRefundDate ? _self.estimatedRefundDate : estimatedRefundDate // ignore: cast_nullable_to_non_nullable
as String,bankDate: null == bankDate ? _self.bankDate : bankDate // ignore: cast_nullable_to_non_nullable
as String,providerStatus: null == providerStatus ? _self.providerStatus : providerStatus // ignore: cast_nullable_to_non_nullable
as String,providerMessage: null == providerMessage ? _self.providerMessage : providerMessage // ignore: cast_nullable_to_non_nullable
as String,appealDeadline: null == appealDeadline ? _self.appealDeadline : appealDeadline // ignore: cast_nullable_to_non_nullable
as String,partnerId: null == partnerId ? _self.partnerId : partnerId // ignore: cast_nullable_to_non_nullable
as String,odno: null == odno ? _self.odno : odno // ignore: cast_nullable_to_non_nullable
as String,validationNo: null == validationNo ? _self.validationNo : validationNo // ignore: cast_nullable_to_non_nullable
as String,caseResolutionType: null == caseResolutionType ? _self.caseResolutionType : caseResolutionType // ignore: cast_nullable_to_non_nullable
as String,shipBackDeadlineAt: null == shipBackDeadlineAt ? _self.shipBackDeadlineAt : shipBackDeadlineAt // ignore: cast_nullable_to_non_nullable
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String caseNo,  String orderNo,  String buyerId,  String storeId,  String status,  String reason,  String description,  String refundableAmount,  String returnShipmentNo,  String returnShippingMethod,  String shipTradeNo,  bool needRefundPage,  String createdAt,  String closedAt,  List<AfterSaleItem> items,  List<String> shipmentPhotoUrls,  List<AfterSaleEvidence> evidence,  List<AfterSaleResolution> resolutions,  String appealState,  bool canAppeal,  String processDeadline,  bool autoEscalated,  int processDeadlineHours,  String refundTaskStatus,  String refundFailureMessage,  String estimatedRefundDate,  String bankDate,  String providerStatus,  String providerMessage,  String appealDeadline,  String partnerId,  String odno,  String validationNo,  String caseResolutionType,  String shipBackDeadlineAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AfterSaleCase() when $default != null:
return $default(_that.caseNo,_that.orderNo,_that.buyerId,_that.storeId,_that.status,_that.reason,_that.description,_that.refundableAmount,_that.returnShipmentNo,_that.returnShippingMethod,_that.shipTradeNo,_that.needRefundPage,_that.createdAt,_that.closedAt,_that.items,_that.shipmentPhotoUrls,_that.evidence,_that.resolutions,_that.appealState,_that.canAppeal,_that.processDeadline,_that.autoEscalated,_that.processDeadlineHours,_that.refundTaskStatus,_that.refundFailureMessage,_that.estimatedRefundDate,_that.bankDate,_that.providerStatus,_that.providerMessage,_that.appealDeadline,_that.partnerId,_that.odno,_that.validationNo,_that.caseResolutionType,_that.shipBackDeadlineAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String caseNo,  String orderNo,  String buyerId,  String storeId,  String status,  String reason,  String description,  String refundableAmount,  String returnShipmentNo,  String returnShippingMethod,  String shipTradeNo,  bool needRefundPage,  String createdAt,  String closedAt,  List<AfterSaleItem> items,  List<String> shipmentPhotoUrls,  List<AfterSaleEvidence> evidence,  List<AfterSaleResolution> resolutions,  String appealState,  bool canAppeal,  String processDeadline,  bool autoEscalated,  int processDeadlineHours,  String refundTaskStatus,  String refundFailureMessage,  String estimatedRefundDate,  String bankDate,  String providerStatus,  String providerMessage,  String appealDeadline,  String partnerId,  String odno,  String validationNo,  String caseResolutionType,  String shipBackDeadlineAt)  $default,) {final _that = this;
switch (_that) {
case _AfterSaleCase():
return $default(_that.caseNo,_that.orderNo,_that.buyerId,_that.storeId,_that.status,_that.reason,_that.description,_that.refundableAmount,_that.returnShipmentNo,_that.returnShippingMethod,_that.shipTradeNo,_that.needRefundPage,_that.createdAt,_that.closedAt,_that.items,_that.shipmentPhotoUrls,_that.evidence,_that.resolutions,_that.appealState,_that.canAppeal,_that.processDeadline,_that.autoEscalated,_that.processDeadlineHours,_that.refundTaskStatus,_that.refundFailureMessage,_that.estimatedRefundDate,_that.bankDate,_that.providerStatus,_that.providerMessage,_that.appealDeadline,_that.partnerId,_that.odno,_that.validationNo,_that.caseResolutionType,_that.shipBackDeadlineAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String caseNo,  String orderNo,  String buyerId,  String storeId,  String status,  String reason,  String description,  String refundableAmount,  String returnShipmentNo,  String returnShippingMethod,  String shipTradeNo,  bool needRefundPage,  String createdAt,  String closedAt,  List<AfterSaleItem> items,  List<String> shipmentPhotoUrls,  List<AfterSaleEvidence> evidence,  List<AfterSaleResolution> resolutions,  String appealState,  bool canAppeal,  String processDeadline,  bool autoEscalated,  int processDeadlineHours,  String refundTaskStatus,  String refundFailureMessage,  String estimatedRefundDate,  String bankDate,  String providerStatus,  String providerMessage,  String appealDeadline,  String partnerId,  String odno,  String validationNo,  String caseResolutionType,  String shipBackDeadlineAt)?  $default,) {final _that = this;
switch (_that) {
case _AfterSaleCase() when $default != null:
return $default(_that.caseNo,_that.orderNo,_that.buyerId,_that.storeId,_that.status,_that.reason,_that.description,_that.refundableAmount,_that.returnShipmentNo,_that.returnShippingMethod,_that.shipTradeNo,_that.needRefundPage,_that.createdAt,_that.closedAt,_that.items,_that.shipmentPhotoUrls,_that.evidence,_that.resolutions,_that.appealState,_that.canAppeal,_that.processDeadline,_that.autoEscalated,_that.processDeadlineHours,_that.refundTaskStatus,_that.refundFailureMessage,_that.estimatedRefundDate,_that.bankDate,_that.providerStatus,_that.providerMessage,_that.appealDeadline,_that.partnerId,_that.odno,_that.validationNo,_that.caseResolutionType,_that.shipBackDeadlineAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AfterSaleCase implements AfterSaleCase {
  const _AfterSaleCase({this.caseNo = '', this.orderNo = '', this.buyerId = '', this.storeId = '', this.status = '', this.reason = '', this.description = '', this.refundableAmount = '', this.returnShipmentNo = '', this.returnShippingMethod = '', this.shipTradeNo = '', this.needRefundPage = false, this.createdAt = '', this.closedAt = '', final  List<AfterSaleItem> items = const [], final  List<String> shipmentPhotoUrls = const [], final  List<AfterSaleEvidence> evidence = const [], final  List<AfterSaleResolution> resolutions = const [], this.appealState = '', this.canAppeal = false, this.processDeadline = '', this.autoEscalated = false, this.processDeadlineHours = 0, this.refundTaskStatus = '', this.refundFailureMessage = '', this.estimatedRefundDate = '', this.bankDate = '', this.providerStatus = '', this.providerMessage = '', this.appealDeadline = '', this.partnerId = '', this.odno = '', this.validationNo = '', this.caseResolutionType = '', this.shipBackDeadlineAt = ''}): _items = items,_shipmentPhotoUrls = shipmentPhotoUrls,_evidence = evidence,_resolutions = resolutions;
  factory _AfterSaleCase.fromJson(Map<String, dynamic> json) => _$AfterSaleCaseFromJson(json);

@override@JsonKey() final  String caseNo;
@override@JsonKey() final  String orderNo;
@override@JsonKey() final  String buyerId;
@override@JsonKey() final  String storeId;
@override@JsonKey() final  String status;
@override@JsonKey() final  String reason;
@override@JsonKey() final  String description;
@override@JsonKey() final  String refundableAmount;
@override@JsonKey() final  String returnShipmentNo;
@override@JsonKey() final  String returnShippingMethod;
/// 寄件交易單號（退貨物流拋單後後端回填，顯示於退貨物流 timeline）
@override@JsonKey() final  String shipTradeNo;
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

 final  List<AfterSaleEvidence> _evidence;
@override@JsonKey() List<AfterSaleEvidence> get evidence {
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

/// 申訴狀態（'0'=未申訴，其他值見後端 AppealState codemeta）
@override@JsonKey() final  String appealState;
/// 是否可以發起申訴（賣家拒絕後買家可於期限內申訴）
@override@JsonKey() final  bool canAppeal;
// B-130 / backend issue #568：賣家處理時限與平台接手狀態
@override@JsonKey() final  String processDeadline;
@override@JsonKey() final  bool autoEscalated;
@override@JsonKey() final  int processDeadlineHours;
// B-125 / B-114 退款進度（對齊 backend issue #568 規格）
/// ATM 退款任務狀態（[RefundTaskStatus]）：後端 WorkerTaskStatus，保留舊數字碼相容。
@override@JsonKey() final  String refundTaskStatus;
/// 上次填寫失敗訊息（後端已依狀態收斂，前台可直接顯示）
@override@JsonKey() final  String refundFailureMessage;
/// 預計入帳日期（YYYY-MM-DD，銀行處理中時顯示）
@override@JsonKey() final  String estimatedRefundDate;
/// 實際匯款日期（YYYY-MM-DD，退款完成時顯示）
@override@JsonKey() final  String bankDate;
@override@JsonKey() final  String providerStatus;
@override@JsonKey() final  String providerMessage;
@override@JsonKey() final  String appealDeadline;
// B-93 7-11 出貨便代碼（三段碼，買家寄退時憑此到 ibon 列印託運單）
/// 母代碼（3 碼）— 顯示條件：resolutionType=1 + returnShippingMethod=3002 + status≥20
@override@JsonKey() final  String partnerId;
/// 出貨單編號（8 碼）
@override@JsonKey() final  String odno;
/// C2C 驗證碼（4 碼）
@override@JsonKey() final  String validationNo;
/// 決議類型：1=退貨退款、2=部分退款、3=全額退款免退貨
/// （case 層級欄位，供買家側快速判斷顯示條件，與 resolutions 陣列同步）
@override@JsonKey() final  String caseResolutionType;
/// 寄回截止時間（ISO 8601 UTC，狀態 20 時由後端回填，預設賣家同意後 7 天）
/// 用於買家側顯示「剩餘寄回時間」倒數與 D-1 警示 banner。
/// status=33 為「待寄回逾時自動關案」終態（無倒數）。
@override@JsonKey() final  String shipBackDeadlineAt;

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
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AfterSaleCase&&(identical(other.caseNo, caseNo) || other.caseNo == caseNo)&&(identical(other.orderNo, orderNo) || other.orderNo == orderNo)&&(identical(other.buyerId, buyerId) || other.buyerId == buyerId)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.status, status) || other.status == status)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.description, description) || other.description == description)&&(identical(other.refundableAmount, refundableAmount) || other.refundableAmount == refundableAmount)&&(identical(other.returnShipmentNo, returnShipmentNo) || other.returnShipmentNo == returnShipmentNo)&&(identical(other.returnShippingMethod, returnShippingMethod) || other.returnShippingMethod == returnShippingMethod)&&(identical(other.shipTradeNo, shipTradeNo) || other.shipTradeNo == shipTradeNo)&&(identical(other.needRefundPage, needRefundPage) || other.needRefundPage == needRefundPage)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.closedAt, closedAt) || other.closedAt == closedAt)&&const DeepCollectionEquality().equals(other._items, _items)&&const DeepCollectionEquality().equals(other._shipmentPhotoUrls, _shipmentPhotoUrls)&&const DeepCollectionEquality().equals(other._evidence, _evidence)&&const DeepCollectionEquality().equals(other._resolutions, _resolutions)&&(identical(other.appealState, appealState) || other.appealState == appealState)&&(identical(other.canAppeal, canAppeal) || other.canAppeal == canAppeal)&&(identical(other.processDeadline, processDeadline) || other.processDeadline == processDeadline)&&(identical(other.autoEscalated, autoEscalated) || other.autoEscalated == autoEscalated)&&(identical(other.processDeadlineHours, processDeadlineHours) || other.processDeadlineHours == processDeadlineHours)&&(identical(other.refundTaskStatus, refundTaskStatus) || other.refundTaskStatus == refundTaskStatus)&&(identical(other.refundFailureMessage, refundFailureMessage) || other.refundFailureMessage == refundFailureMessage)&&(identical(other.estimatedRefundDate, estimatedRefundDate) || other.estimatedRefundDate == estimatedRefundDate)&&(identical(other.bankDate, bankDate) || other.bankDate == bankDate)&&(identical(other.providerStatus, providerStatus) || other.providerStatus == providerStatus)&&(identical(other.providerMessage, providerMessage) || other.providerMessage == providerMessage)&&(identical(other.appealDeadline, appealDeadline) || other.appealDeadline == appealDeadline)&&(identical(other.partnerId, partnerId) || other.partnerId == partnerId)&&(identical(other.odno, odno) || other.odno == odno)&&(identical(other.validationNo, validationNo) || other.validationNo == validationNo)&&(identical(other.caseResolutionType, caseResolutionType) || other.caseResolutionType == caseResolutionType)&&(identical(other.shipBackDeadlineAt, shipBackDeadlineAt) || other.shipBackDeadlineAt == shipBackDeadlineAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,caseNo,orderNo,buyerId,storeId,status,reason,description,refundableAmount,returnShipmentNo,returnShippingMethod,shipTradeNo,needRefundPage,createdAt,closedAt,const DeepCollectionEquality().hash(_items),const DeepCollectionEquality().hash(_shipmentPhotoUrls),const DeepCollectionEquality().hash(_evidence),const DeepCollectionEquality().hash(_resolutions),appealState,canAppeal,processDeadline,autoEscalated,processDeadlineHours,refundTaskStatus,refundFailureMessage,estimatedRefundDate,bankDate,providerStatus,providerMessage,appealDeadline,partnerId,odno,validationNo,caseResolutionType,shipBackDeadlineAt]);

@override
String toString() {
  return 'AfterSaleCase(caseNo: $caseNo, orderNo: $orderNo, buyerId: $buyerId, storeId: $storeId, status: $status, reason: $reason, description: $description, refundableAmount: $refundableAmount, returnShipmentNo: $returnShipmentNo, returnShippingMethod: $returnShippingMethod, shipTradeNo: $shipTradeNo, needRefundPage: $needRefundPage, createdAt: $createdAt, closedAt: $closedAt, items: $items, shipmentPhotoUrls: $shipmentPhotoUrls, evidence: $evidence, resolutions: $resolutions, appealState: $appealState, canAppeal: $canAppeal, processDeadline: $processDeadline, autoEscalated: $autoEscalated, processDeadlineHours: $processDeadlineHours, refundTaskStatus: $refundTaskStatus, refundFailureMessage: $refundFailureMessage, estimatedRefundDate: $estimatedRefundDate, bankDate: $bankDate, providerStatus: $providerStatus, providerMessage: $providerMessage, appealDeadline: $appealDeadline, partnerId: $partnerId, odno: $odno, validationNo: $validationNo, caseResolutionType: $caseResolutionType, shipBackDeadlineAt: $shipBackDeadlineAt)';
}


}

/// @nodoc
abstract mixin class _$AfterSaleCaseCopyWith<$Res> implements $AfterSaleCaseCopyWith<$Res> {
  factory _$AfterSaleCaseCopyWith(_AfterSaleCase value, $Res Function(_AfterSaleCase) _then) = __$AfterSaleCaseCopyWithImpl;
@override @useResult
$Res call({
 String caseNo, String orderNo, String buyerId, String storeId, String status, String reason, String description, String refundableAmount, String returnShipmentNo, String returnShippingMethod, String shipTradeNo, bool needRefundPage, String createdAt, String closedAt, List<AfterSaleItem> items, List<String> shipmentPhotoUrls, List<AfterSaleEvidence> evidence, List<AfterSaleResolution> resolutions, String appealState, bool canAppeal, String processDeadline, bool autoEscalated, int processDeadlineHours, String refundTaskStatus, String refundFailureMessage, String estimatedRefundDate, String bankDate, String providerStatus, String providerMessage, String appealDeadline, String partnerId, String odno, String validationNo, String caseResolutionType, String shipBackDeadlineAt
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
@override @pragma('vm:prefer-inline') $Res call({Object? caseNo = null,Object? orderNo = null,Object? buyerId = null,Object? storeId = null,Object? status = null,Object? reason = null,Object? description = null,Object? refundableAmount = null,Object? returnShipmentNo = null,Object? returnShippingMethod = null,Object? shipTradeNo = null,Object? needRefundPage = null,Object? createdAt = null,Object? closedAt = null,Object? items = null,Object? shipmentPhotoUrls = null,Object? evidence = null,Object? resolutions = null,Object? appealState = null,Object? canAppeal = null,Object? processDeadline = null,Object? autoEscalated = null,Object? processDeadlineHours = null,Object? refundTaskStatus = null,Object? refundFailureMessage = null,Object? estimatedRefundDate = null,Object? bankDate = null,Object? providerStatus = null,Object? providerMessage = null,Object? appealDeadline = null,Object? partnerId = null,Object? odno = null,Object? validationNo = null,Object? caseResolutionType = null,Object? shipBackDeadlineAt = null,}) {
  return _then(_AfterSaleCase(
caseNo: null == caseNo ? _self.caseNo : caseNo // ignore: cast_nullable_to_non_nullable
as String,orderNo: null == orderNo ? _self.orderNo : orderNo // ignore: cast_nullable_to_non_nullable
as String,buyerId: null == buyerId ? _self.buyerId : buyerId // ignore: cast_nullable_to_non_nullable
as String,storeId: null == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,description: null == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String,refundableAmount: null == refundableAmount ? _self.refundableAmount : refundableAmount // ignore: cast_nullable_to_non_nullable
as String,returnShipmentNo: null == returnShipmentNo ? _self.returnShipmentNo : returnShipmentNo // ignore: cast_nullable_to_non_nullable
as String,returnShippingMethod: null == returnShippingMethod ? _self.returnShippingMethod : returnShippingMethod // ignore: cast_nullable_to_non_nullable
as String,shipTradeNo: null == shipTradeNo ? _self.shipTradeNo : shipTradeNo // ignore: cast_nullable_to_non_nullable
as String,needRefundPage: null == needRefundPage ? _self.needRefundPage : needRefundPage // ignore: cast_nullable_to_non_nullable
as bool,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String,closedAt: null == closedAt ? _self.closedAt : closedAt // ignore: cast_nullable_to_non_nullable
as String,items: null == items ? _self._items : items // ignore: cast_nullable_to_non_nullable
as List<AfterSaleItem>,shipmentPhotoUrls: null == shipmentPhotoUrls ? _self._shipmentPhotoUrls : shipmentPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,evidence: null == evidence ? _self._evidence : evidence // ignore: cast_nullable_to_non_nullable
as List<AfterSaleEvidence>,resolutions: null == resolutions ? _self._resolutions : resolutions // ignore: cast_nullable_to_non_nullable
as List<AfterSaleResolution>,appealState: null == appealState ? _self.appealState : appealState // ignore: cast_nullable_to_non_nullable
as String,canAppeal: null == canAppeal ? _self.canAppeal : canAppeal // ignore: cast_nullable_to_non_nullable
as bool,processDeadline: null == processDeadline ? _self.processDeadline : processDeadline // ignore: cast_nullable_to_non_nullable
as String,autoEscalated: null == autoEscalated ? _self.autoEscalated : autoEscalated // ignore: cast_nullable_to_non_nullable
as bool,processDeadlineHours: null == processDeadlineHours ? _self.processDeadlineHours : processDeadlineHours // ignore: cast_nullable_to_non_nullable
as int,refundTaskStatus: null == refundTaskStatus ? _self.refundTaskStatus : refundTaskStatus // ignore: cast_nullable_to_non_nullable
as String,refundFailureMessage: null == refundFailureMessage ? _self.refundFailureMessage : refundFailureMessage // ignore: cast_nullable_to_non_nullable
as String,estimatedRefundDate: null == estimatedRefundDate ? _self.estimatedRefundDate : estimatedRefundDate // ignore: cast_nullable_to_non_nullable
as String,bankDate: null == bankDate ? _self.bankDate : bankDate // ignore: cast_nullable_to_non_nullable
as String,providerStatus: null == providerStatus ? _self.providerStatus : providerStatus // ignore: cast_nullable_to_non_nullable
as String,providerMessage: null == providerMessage ? _self.providerMessage : providerMessage // ignore: cast_nullable_to_non_nullable
as String,appealDeadline: null == appealDeadline ? _self.appealDeadline : appealDeadline // ignore: cast_nullable_to_non_nullable
as String,partnerId: null == partnerId ? _self.partnerId : partnerId // ignore: cast_nullable_to_non_nullable
as String,odno: null == odno ? _self.odno : odno // ignore: cast_nullable_to_non_nullable
as String,validationNo: null == validationNo ? _self.validationNo : validationNo // ignore: cast_nullable_to_non_nullable
as String,caseResolutionType: null == caseResolutionType ? _self.caseResolutionType : caseResolutionType // ignore: cast_nullable_to_non_nullable
as String,shipBackDeadlineAt: null == shipBackDeadlineAt ? _self.shipBackDeadlineAt : shipBackDeadlineAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$AfterSaleEvidence {

 String get assetId; String get imageUrl; String get evidenceType; String get createdAt;
/// Create a copy of AfterSaleEvidence
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AfterSaleEvidenceCopyWith<AfterSaleEvidence> get copyWith => _$AfterSaleEvidenceCopyWithImpl<AfterSaleEvidence>(this as AfterSaleEvidence, _$identity);

  /// Serializes this AfterSaleEvidence to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AfterSaleEvidence&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.evidenceType, evidenceType) || other.evidenceType == evidenceType)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,assetId,imageUrl,evidenceType,createdAt);

@override
String toString() {
  return 'AfterSaleEvidence(assetId: $assetId, imageUrl: $imageUrl, evidenceType: $evidenceType, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class $AfterSaleEvidenceCopyWith<$Res>  {
  factory $AfterSaleEvidenceCopyWith(AfterSaleEvidence value, $Res Function(AfterSaleEvidence) _then) = _$AfterSaleEvidenceCopyWithImpl;
@useResult
$Res call({
 String assetId, String imageUrl, String evidenceType, String createdAt
});




}
/// @nodoc
class _$AfterSaleEvidenceCopyWithImpl<$Res>
    implements $AfterSaleEvidenceCopyWith<$Res> {
  _$AfterSaleEvidenceCopyWithImpl(this._self, this._then);

  final AfterSaleEvidence _self;
  final $Res Function(AfterSaleEvidence) _then;

/// Create a copy of AfterSaleEvidence
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? assetId = null,Object? imageUrl = null,Object? evidenceType = null,Object? createdAt = null,}) {
  return _then(_self.copyWith(
assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,evidenceType: null == evidenceType ? _self.evidenceType : evidenceType // ignore: cast_nullable_to_non_nullable
as String,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AfterSaleEvidence].
extension AfterSaleEvidencePatterns on AfterSaleEvidence {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AfterSaleEvidence value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AfterSaleEvidence() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AfterSaleEvidence value)  $default,){
final _that = this;
switch (_that) {
case _AfterSaleEvidence():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AfterSaleEvidence value)?  $default,){
final _that = this;
switch (_that) {
case _AfterSaleEvidence() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String assetId,  String imageUrl,  String evidenceType,  String createdAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AfterSaleEvidence() when $default != null:
return $default(_that.assetId,_that.imageUrl,_that.evidenceType,_that.createdAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String assetId,  String imageUrl,  String evidenceType,  String createdAt)  $default,) {final _that = this;
switch (_that) {
case _AfterSaleEvidence():
return $default(_that.assetId,_that.imageUrl,_that.evidenceType,_that.createdAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String assetId,  String imageUrl,  String evidenceType,  String createdAt)?  $default,) {final _that = this;
switch (_that) {
case _AfterSaleEvidence() when $default != null:
return $default(_that.assetId,_that.imageUrl,_that.evidenceType,_that.createdAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AfterSaleEvidence implements AfterSaleEvidence {
  const _AfterSaleEvidence({this.assetId = '', this.imageUrl = '', this.evidenceType = '', this.createdAt = ''});
  factory _AfterSaleEvidence.fromJson(Map<String, dynamic> json) => _$AfterSaleEvidenceFromJson(json);

@override@JsonKey() final  String assetId;
@override@JsonKey() final  String imageUrl;
@override@JsonKey() final  String evidenceType;
@override@JsonKey() final  String createdAt;

/// Create a copy of AfterSaleEvidence
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AfterSaleEvidenceCopyWith<_AfterSaleEvidence> get copyWith => __$AfterSaleEvidenceCopyWithImpl<_AfterSaleEvidence>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$AfterSaleEvidenceToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AfterSaleEvidence&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.evidenceType, evidenceType) || other.evidenceType == evidenceType)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,assetId,imageUrl,evidenceType,createdAt);

@override
String toString() {
  return 'AfterSaleEvidence(assetId: $assetId, imageUrl: $imageUrl, evidenceType: $evidenceType, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class _$AfterSaleEvidenceCopyWith<$Res> implements $AfterSaleEvidenceCopyWith<$Res> {
  factory _$AfterSaleEvidenceCopyWith(_AfterSaleEvidence value, $Res Function(_AfterSaleEvidence) _then) = __$AfterSaleEvidenceCopyWithImpl;
@override @useResult
$Res call({
 String assetId, String imageUrl, String evidenceType, String createdAt
});




}
/// @nodoc
class __$AfterSaleEvidenceCopyWithImpl<$Res>
    implements _$AfterSaleEvidenceCopyWith<$Res> {
  __$AfterSaleEvidenceCopyWithImpl(this._self, this._then);

  final _AfterSaleEvidence _self;
  final $Res Function(_AfterSaleEvidence) _then;

/// Create a copy of AfterSaleEvidence
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? assetId = null,Object? imageUrl = null,Object? evidenceType = null,Object? createdAt = null,}) {
  return _then(_AfterSaleEvidence(
assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,evidenceType: null == evidenceType ? _self.evidenceType : evidenceType // ignore: cast_nullable_to_non_nullable
as String,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$AfterSaleItem {

 String get orderItemId; String get skuId; String get skuName; String get quantity;// PayUni #758: 新欄位（成交單價 / 原價）
 String get unitPrice; String get listPrice;/// Deprecated: 舊欄位 fallback。
 String get originalPrice; String get actualPayAmount; String get shareDiscount; String get imageUrl;
/// Create a copy of AfterSaleItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AfterSaleItemCopyWith<AfterSaleItem> get copyWith => _$AfterSaleItemCopyWithImpl<AfterSaleItem>(this as AfterSaleItem, _$identity);

  /// Serializes this AfterSaleItem to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AfterSaleItem&&(identical(other.orderItemId, orderItemId) || other.orderItemId == orderItemId)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.skuName, skuName) || other.skuName == skuName)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unitPrice, unitPrice) || other.unitPrice == unitPrice)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.originalPrice, originalPrice) || other.originalPrice == originalPrice)&&(identical(other.actualPayAmount, actualPayAmount) || other.actualPayAmount == actualPayAmount)&&(identical(other.shareDiscount, shareDiscount) || other.shareDiscount == shareDiscount)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,orderItemId,skuId,skuName,quantity,unitPrice,listPrice,originalPrice,actualPayAmount,shareDiscount,imageUrl);

@override
String toString() {
  return 'AfterSaleItem(orderItemId: $orderItemId, skuId: $skuId, skuName: $skuName, quantity: $quantity, unitPrice: $unitPrice, listPrice: $listPrice, originalPrice: $originalPrice, actualPayAmount: $actualPayAmount, shareDiscount: $shareDiscount, imageUrl: $imageUrl)';
}


}

/// @nodoc
abstract mixin class $AfterSaleItemCopyWith<$Res>  {
  factory $AfterSaleItemCopyWith(AfterSaleItem value, $Res Function(AfterSaleItem) _then) = _$AfterSaleItemCopyWithImpl;
@useResult
$Res call({
 String orderItemId, String skuId, String skuName, String quantity, String unitPrice, String listPrice, String originalPrice, String actualPayAmount, String shareDiscount, String imageUrl
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
@pragma('vm:prefer-inline') @override $Res call({Object? orderItemId = null,Object? skuId = null,Object? skuName = null,Object? quantity = null,Object? unitPrice = null,Object? listPrice = null,Object? originalPrice = null,Object? actualPayAmount = null,Object? shareDiscount = null,Object? imageUrl = null,}) {
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
as String,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String orderItemId,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String shareDiscount,  String imageUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AfterSaleItem() when $default != null:
return $default(_that.orderItemId,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.shareDiscount,_that.imageUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String orderItemId,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String shareDiscount,  String imageUrl)  $default,) {final _that = this;
switch (_that) {
case _AfterSaleItem():
return $default(_that.orderItemId,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.shareDiscount,_that.imageUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String orderItemId,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String shareDiscount,  String imageUrl)?  $default,) {final _that = this;
switch (_that) {
case _AfterSaleItem() when $default != null:
return $default(_that.orderItemId,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.shareDiscount,_that.imageUrl);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AfterSaleItem implements AfterSaleItem {
  const _AfterSaleItem({this.orderItemId = '', this.skuId = '', this.skuName = '', this.quantity = '', this.unitPrice = '', this.listPrice = '', this.originalPrice = '', this.actualPayAmount = '', this.shareDiscount = '', this.imageUrl = ''});
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
@override@JsonKey() final  String imageUrl;

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
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AfterSaleItem&&(identical(other.orderItemId, orderItemId) || other.orderItemId == orderItemId)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.skuName, skuName) || other.skuName == skuName)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unitPrice, unitPrice) || other.unitPrice == unitPrice)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.originalPrice, originalPrice) || other.originalPrice == originalPrice)&&(identical(other.actualPayAmount, actualPayAmount) || other.actualPayAmount == actualPayAmount)&&(identical(other.shareDiscount, shareDiscount) || other.shareDiscount == shareDiscount)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,orderItemId,skuId,skuName,quantity,unitPrice,listPrice,originalPrice,actualPayAmount,shareDiscount,imageUrl);

@override
String toString() {
  return 'AfterSaleItem(orderItemId: $orderItemId, skuId: $skuId, skuName: $skuName, quantity: $quantity, unitPrice: $unitPrice, listPrice: $listPrice, originalPrice: $originalPrice, actualPayAmount: $actualPayAmount, shareDiscount: $shareDiscount, imageUrl: $imageUrl)';
}


}

/// @nodoc
abstract mixin class _$AfterSaleItemCopyWith<$Res> implements $AfterSaleItemCopyWith<$Res> {
  factory _$AfterSaleItemCopyWith(_AfterSaleItem value, $Res Function(_AfterSaleItem) _then) = __$AfterSaleItemCopyWithImpl;
@override @useResult
$Res call({
 String orderItemId, String skuId, String skuName, String quantity, String unitPrice, String listPrice, String originalPrice, String actualPayAmount, String shareDiscount, String imageUrl
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
@override @pragma('vm:prefer-inline') $Res call({Object? orderItemId = null,Object? skuId = null,Object? skuName = null,Object? quantity = null,Object? unitPrice = null,Object? listPrice = null,Object? originalPrice = null,Object? actualPayAmount = null,Object? shareDiscount = null,Object? imageUrl = null,}) {
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
as String,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$AfterSaleResolution {

 String get resolutionType; String get refundAmount; String get couponAmount; String get couponMinSpend; String get couponMessage; String get rejectReason; String get rejectMessage; String get negotiationReason; String get createdAt;// B-96 驗貨扣款（resolutionType=6 INSPECTION_DEDUCTION）
/// 扣款原因（賣家驗貨後填寫，部分退款場景）
 String get deductionReason;/// 驗貨備註（賣家私註）
 String get inspectionNote;
/// Create a copy of AfterSaleResolution
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AfterSaleResolutionCopyWith<AfterSaleResolution> get copyWith => _$AfterSaleResolutionCopyWithImpl<AfterSaleResolution>(this as AfterSaleResolution, _$identity);

  /// Serializes this AfterSaleResolution to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AfterSaleResolution&&(identical(other.resolutionType, resolutionType) || other.resolutionType == resolutionType)&&(identical(other.refundAmount, refundAmount) || other.refundAmount == refundAmount)&&(identical(other.couponAmount, couponAmount) || other.couponAmount == couponAmount)&&(identical(other.couponMinSpend, couponMinSpend) || other.couponMinSpend == couponMinSpend)&&(identical(other.couponMessage, couponMessage) || other.couponMessage == couponMessage)&&(identical(other.rejectReason, rejectReason) || other.rejectReason == rejectReason)&&(identical(other.rejectMessage, rejectMessage) || other.rejectMessage == rejectMessage)&&(identical(other.negotiationReason, negotiationReason) || other.negotiationReason == negotiationReason)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.deductionReason, deductionReason) || other.deductionReason == deductionReason)&&(identical(other.inspectionNote, inspectionNote) || other.inspectionNote == inspectionNote));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,resolutionType,refundAmount,couponAmount,couponMinSpend,couponMessage,rejectReason,rejectMessage,negotiationReason,createdAt,deductionReason,inspectionNote);

@override
String toString() {
  return 'AfterSaleResolution(resolutionType: $resolutionType, refundAmount: $refundAmount, couponAmount: $couponAmount, couponMinSpend: $couponMinSpend, couponMessage: $couponMessage, rejectReason: $rejectReason, rejectMessage: $rejectMessage, negotiationReason: $negotiationReason, createdAt: $createdAt, deductionReason: $deductionReason, inspectionNote: $inspectionNote)';
}


}

/// @nodoc
abstract mixin class $AfterSaleResolutionCopyWith<$Res>  {
  factory $AfterSaleResolutionCopyWith(AfterSaleResolution value, $Res Function(AfterSaleResolution) _then) = _$AfterSaleResolutionCopyWithImpl;
@useResult
$Res call({
 String resolutionType, String refundAmount, String couponAmount, String couponMinSpend, String couponMessage, String rejectReason, String rejectMessage, String negotiationReason, String createdAt, String deductionReason, String inspectionNote
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
@pragma('vm:prefer-inline') @override $Res call({Object? resolutionType = null,Object? refundAmount = null,Object? couponAmount = null,Object? couponMinSpend = null,Object? couponMessage = null,Object? rejectReason = null,Object? rejectMessage = null,Object? negotiationReason = null,Object? createdAt = null,Object? deductionReason = null,Object? inspectionNote = null,}) {
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
as String,deductionReason: null == deductionReason ? _self.deductionReason : deductionReason // ignore: cast_nullable_to_non_nullable
as String,inspectionNote: null == inspectionNote ? _self.inspectionNote : inspectionNote // ignore: cast_nullable_to_non_nullable
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String resolutionType,  String refundAmount,  String couponAmount,  String couponMinSpend,  String couponMessage,  String rejectReason,  String rejectMessage,  String negotiationReason,  String createdAt,  String deductionReason,  String inspectionNote)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AfterSaleResolution() when $default != null:
return $default(_that.resolutionType,_that.refundAmount,_that.couponAmount,_that.couponMinSpend,_that.couponMessage,_that.rejectReason,_that.rejectMessage,_that.negotiationReason,_that.createdAt,_that.deductionReason,_that.inspectionNote);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String resolutionType,  String refundAmount,  String couponAmount,  String couponMinSpend,  String couponMessage,  String rejectReason,  String rejectMessage,  String negotiationReason,  String createdAt,  String deductionReason,  String inspectionNote)  $default,) {final _that = this;
switch (_that) {
case _AfterSaleResolution():
return $default(_that.resolutionType,_that.refundAmount,_that.couponAmount,_that.couponMinSpend,_that.couponMessage,_that.rejectReason,_that.rejectMessage,_that.negotiationReason,_that.createdAt,_that.deductionReason,_that.inspectionNote);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String resolutionType,  String refundAmount,  String couponAmount,  String couponMinSpend,  String couponMessage,  String rejectReason,  String rejectMessage,  String negotiationReason,  String createdAt,  String deductionReason,  String inspectionNote)?  $default,) {final _that = this;
switch (_that) {
case _AfterSaleResolution() when $default != null:
return $default(_that.resolutionType,_that.refundAmount,_that.couponAmount,_that.couponMinSpend,_that.couponMessage,_that.rejectReason,_that.rejectMessage,_that.negotiationReason,_that.createdAt,_that.deductionReason,_that.inspectionNote);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AfterSaleResolution implements AfterSaleResolution {
  const _AfterSaleResolution({this.resolutionType = '', this.refundAmount = '', this.couponAmount = '', this.couponMinSpend = '', this.couponMessage = '', this.rejectReason = '', this.rejectMessage = '', this.negotiationReason = '', this.createdAt = '', this.deductionReason = '', this.inspectionNote = ''});
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
// B-96 驗貨扣款（resolutionType=6 INSPECTION_DEDUCTION）
/// 扣款原因（賣家驗貨後填寫，部分退款場景）
@override@JsonKey() final  String deductionReason;
/// 驗貨備註（賣家私註）
@override@JsonKey() final  String inspectionNote;

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
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AfterSaleResolution&&(identical(other.resolutionType, resolutionType) || other.resolutionType == resolutionType)&&(identical(other.refundAmount, refundAmount) || other.refundAmount == refundAmount)&&(identical(other.couponAmount, couponAmount) || other.couponAmount == couponAmount)&&(identical(other.couponMinSpend, couponMinSpend) || other.couponMinSpend == couponMinSpend)&&(identical(other.couponMessage, couponMessage) || other.couponMessage == couponMessage)&&(identical(other.rejectReason, rejectReason) || other.rejectReason == rejectReason)&&(identical(other.rejectMessage, rejectMessage) || other.rejectMessage == rejectMessage)&&(identical(other.negotiationReason, negotiationReason) || other.negotiationReason == negotiationReason)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.deductionReason, deductionReason) || other.deductionReason == deductionReason)&&(identical(other.inspectionNote, inspectionNote) || other.inspectionNote == inspectionNote));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,resolutionType,refundAmount,couponAmount,couponMinSpend,couponMessage,rejectReason,rejectMessage,negotiationReason,createdAt,deductionReason,inspectionNote);

@override
String toString() {
  return 'AfterSaleResolution(resolutionType: $resolutionType, refundAmount: $refundAmount, couponAmount: $couponAmount, couponMinSpend: $couponMinSpend, couponMessage: $couponMessage, rejectReason: $rejectReason, rejectMessage: $rejectMessage, negotiationReason: $negotiationReason, createdAt: $createdAt, deductionReason: $deductionReason, inspectionNote: $inspectionNote)';
}


}

/// @nodoc
abstract mixin class _$AfterSaleResolutionCopyWith<$Res> implements $AfterSaleResolutionCopyWith<$Res> {
  factory _$AfterSaleResolutionCopyWith(_AfterSaleResolution value, $Res Function(_AfterSaleResolution) _then) = __$AfterSaleResolutionCopyWithImpl;
@override @useResult
$Res call({
 String resolutionType, String refundAmount, String couponAmount, String couponMinSpend, String couponMessage, String rejectReason, String rejectMessage, String negotiationReason, String createdAt, String deductionReason, String inspectionNote
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
@override @pragma('vm:prefer-inline') $Res call({Object? resolutionType = null,Object? refundAmount = null,Object? couponAmount = null,Object? couponMinSpend = null,Object? couponMessage = null,Object? rejectReason = null,Object? rejectMessage = null,Object? negotiationReason = null,Object? createdAt = null,Object? deductionReason = null,Object? inspectionNote = null,}) {
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
as String,deductionReason: null == deductionReason ? _self.deductionReason : deductionReason // ignore: cast_nullable_to_non_nullable
as String,inspectionNote: null == inspectionNote ? _self.inspectionNote : inspectionNote // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
