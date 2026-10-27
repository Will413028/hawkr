// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'kyc_wizard.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$KycWizardState implements DiagnosticableTreeMixin {

/// 已經從申請（或沒有申請）載入。
 bool get hydrated;/// 申請的身分；還沒有申請也還沒選時是 null（顯示選擇畫面）。
 KycSubject? get subject; KycForm get form; KycForm get baseline; KycDocs get docs; KycDocs get baselineDocs; StoreContactDraft get store; StoreContactDraft get storeBaseline; StoreContactErrors get storeServerErrors; StoreContactDraft? get storeServerErrorsFor;/// 最近一次動作失敗或提醒的文字。
 String? get error;/// 載入後要停在哪一步；沒有申請或沒有缺項時是 null（從頭開始）。
 KycStep? get resumeStep; KycLookup get lookup; Map<KYCField, KycFieldError> get fieldErrors; Set<String> get uploading; bool get saving; bool get storeSaving; bool get submitting; bool get discarding; bool get creating;
/// Create a copy of KycWizardState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$KycWizardStateCopyWith<KycWizardState> get copyWith => _$KycWizardStateCopyWithImpl<KycWizardState>(this as KycWizardState, _$identity);


@override
void debugFillProperties(DiagnosticPropertiesBuilder properties) {
  properties
    ..add(DiagnosticsProperty('type', 'KycWizardState'))
    ..add(DiagnosticsProperty('hydrated', hydrated))..add(DiagnosticsProperty('subject', subject))..add(DiagnosticsProperty('form', form))..add(DiagnosticsProperty('baseline', baseline))..add(DiagnosticsProperty('docs', docs))..add(DiagnosticsProperty('baselineDocs', baselineDocs))..add(DiagnosticsProperty('store', store))..add(DiagnosticsProperty('storeBaseline', storeBaseline))..add(DiagnosticsProperty('storeServerErrors', storeServerErrors))..add(DiagnosticsProperty('storeServerErrorsFor', storeServerErrorsFor))..add(DiagnosticsProperty('error', error))..add(DiagnosticsProperty('resumeStep', resumeStep))..add(DiagnosticsProperty('lookup', lookup))..add(DiagnosticsProperty('fieldErrors', fieldErrors))..add(DiagnosticsProperty('uploading', uploading))..add(DiagnosticsProperty('saving', saving))..add(DiagnosticsProperty('storeSaving', storeSaving))..add(DiagnosticsProperty('submitting', submitting))..add(DiagnosticsProperty('discarding', discarding))..add(DiagnosticsProperty('creating', creating));
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is KycWizardState&&(identical(other.hydrated, hydrated) || other.hydrated == hydrated)&&(identical(other.subject, subject) || other.subject == subject)&&(identical(other.form, form) || other.form == form)&&(identical(other.baseline, baseline) || other.baseline == baseline)&&(identical(other.docs, docs) || other.docs == docs)&&(identical(other.baselineDocs, baselineDocs) || other.baselineDocs == baselineDocs)&&(identical(other.store, store) || other.store == store)&&(identical(other.storeBaseline, storeBaseline) || other.storeBaseline == storeBaseline)&&(identical(other.storeServerErrors, storeServerErrors) || other.storeServerErrors == storeServerErrors)&&(identical(other.storeServerErrorsFor, storeServerErrorsFor) || other.storeServerErrorsFor == storeServerErrorsFor)&&(identical(other.error, error) || other.error == error)&&(identical(other.resumeStep, resumeStep) || other.resumeStep == resumeStep)&&(identical(other.lookup, lookup) || other.lookup == lookup)&&const DeepCollectionEquality().equals(other.fieldErrors, fieldErrors)&&const DeepCollectionEquality().equals(other.uploading, uploading)&&(identical(other.saving, saving) || other.saving == saving)&&(identical(other.storeSaving, storeSaving) || other.storeSaving == storeSaving)&&(identical(other.submitting, submitting) || other.submitting == submitting)&&(identical(other.discarding, discarding) || other.discarding == discarding)&&(identical(other.creating, creating) || other.creating == creating));
}


@override
int get hashCode => Object.hashAll([runtimeType,hydrated,subject,form,baseline,docs,baselineDocs,store,storeBaseline,storeServerErrors,storeServerErrorsFor,error,resumeStep,lookup,const DeepCollectionEquality().hash(fieldErrors),const DeepCollectionEquality().hash(uploading),saving,storeSaving,submitting,discarding,creating]);

@override
String toString({ DiagnosticLevel minLevel = DiagnosticLevel.info }) {
  return 'KycWizardState(hydrated: $hydrated, subject: $subject, form: $form, baseline: $baseline, docs: $docs, baselineDocs: $baselineDocs, store: $store, storeBaseline: $storeBaseline, storeServerErrors: $storeServerErrors, storeServerErrorsFor: $storeServerErrorsFor, error: $error, resumeStep: $resumeStep, lookup: $lookup, fieldErrors: $fieldErrors, uploading: $uploading, saving: $saving, storeSaving: $storeSaving, submitting: $submitting, discarding: $discarding, creating: $creating)';
}


}

/// @nodoc
abstract mixin class $KycWizardStateCopyWith<$Res>  {
  factory $KycWizardStateCopyWith(KycWizardState value, $Res Function(KycWizardState) _then) = _$KycWizardStateCopyWithImpl;
@useResult
$Res call({
 bool hydrated, KycSubject? subject, KycForm form, KycForm baseline, KycDocs docs, KycDocs baselineDocs, StoreContactDraft store, StoreContactDraft storeBaseline, StoreContactErrors storeServerErrors, StoreContactDraft? storeServerErrorsFor, String? error, KycStep? resumeStep, KycLookup lookup, Map<KYCField, KycFieldError> fieldErrors, Set<String> uploading, bool saving, bool storeSaving, bool submitting, bool discarding, bool creating
});




}
/// @nodoc
class _$KycWizardStateCopyWithImpl<$Res>
    implements $KycWizardStateCopyWith<$Res> {
  _$KycWizardStateCopyWithImpl(this._self, this._then);

  final KycWizardState _self;
  final $Res Function(KycWizardState) _then;

/// Create a copy of KycWizardState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? hydrated = null,Object? subject = freezed,Object? form = null,Object? baseline = null,Object? docs = null,Object? baselineDocs = null,Object? store = null,Object? storeBaseline = null,Object? storeServerErrors = null,Object? storeServerErrorsFor = freezed,Object? error = freezed,Object? resumeStep = freezed,Object? lookup = null,Object? fieldErrors = null,Object? uploading = null,Object? saving = null,Object? storeSaving = null,Object? submitting = null,Object? discarding = null,Object? creating = null,}) {
  return _then(_self.copyWith(
hydrated: null == hydrated ? _self.hydrated : hydrated // ignore: cast_nullable_to_non_nullable
as bool,subject: freezed == subject ? _self.subject : subject // ignore: cast_nullable_to_non_nullable
as KycSubject?,form: null == form ? _self.form : form // ignore: cast_nullable_to_non_nullable
as KycForm,baseline: null == baseline ? _self.baseline : baseline // ignore: cast_nullable_to_non_nullable
as KycForm,docs: null == docs ? _self.docs : docs // ignore: cast_nullable_to_non_nullable
as KycDocs,baselineDocs: null == baselineDocs ? _self.baselineDocs : baselineDocs // ignore: cast_nullable_to_non_nullable
as KycDocs,store: null == store ? _self.store : store // ignore: cast_nullable_to_non_nullable
as StoreContactDraft,storeBaseline: null == storeBaseline ? _self.storeBaseline : storeBaseline // ignore: cast_nullable_to_non_nullable
as StoreContactDraft,storeServerErrors: null == storeServerErrors ? _self.storeServerErrors : storeServerErrors // ignore: cast_nullable_to_non_nullable
as StoreContactErrors,storeServerErrorsFor: freezed == storeServerErrorsFor ? _self.storeServerErrorsFor : storeServerErrorsFor // ignore: cast_nullable_to_non_nullable
as StoreContactDraft?,error: freezed == error ? _self.error : error // ignore: cast_nullable_to_non_nullable
as String?,resumeStep: freezed == resumeStep ? _self.resumeStep : resumeStep // ignore: cast_nullable_to_non_nullable
as KycStep?,lookup: null == lookup ? _self.lookup : lookup // ignore: cast_nullable_to_non_nullable
as KycLookup,fieldErrors: null == fieldErrors ? _self.fieldErrors : fieldErrors // ignore: cast_nullable_to_non_nullable
as Map<KYCField, KycFieldError>,uploading: null == uploading ? _self.uploading : uploading // ignore: cast_nullable_to_non_nullable
as Set<String>,saving: null == saving ? _self.saving : saving // ignore: cast_nullable_to_non_nullable
as bool,storeSaving: null == storeSaving ? _self.storeSaving : storeSaving // ignore: cast_nullable_to_non_nullable
as bool,submitting: null == submitting ? _self.submitting : submitting // ignore: cast_nullable_to_non_nullable
as bool,discarding: null == discarding ? _self.discarding : discarding // ignore: cast_nullable_to_non_nullable
as bool,creating: null == creating ? _self.creating : creating // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

}


/// Adds pattern-matching-related methods to [KycWizardState].
extension KycWizardStatePatterns on KycWizardState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _KycWizardState value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _KycWizardState() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _KycWizardState value)  $default,){
final _that = this;
switch (_that) {
case _KycWizardState():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _KycWizardState value)?  $default,){
final _that = this;
switch (_that) {
case _KycWizardState() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( bool hydrated,  KycSubject? subject,  KycForm form,  KycForm baseline,  KycDocs docs,  KycDocs baselineDocs,  StoreContactDraft store,  StoreContactDraft storeBaseline,  StoreContactErrors storeServerErrors,  StoreContactDraft? storeServerErrorsFor,  String? error,  KycStep? resumeStep,  KycLookup lookup,  Map<KYCField, KycFieldError> fieldErrors,  Set<String> uploading,  bool saving,  bool storeSaving,  bool submitting,  bool discarding,  bool creating)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _KycWizardState() when $default != null:
return $default(_that.hydrated,_that.subject,_that.form,_that.baseline,_that.docs,_that.baselineDocs,_that.store,_that.storeBaseline,_that.storeServerErrors,_that.storeServerErrorsFor,_that.error,_that.resumeStep,_that.lookup,_that.fieldErrors,_that.uploading,_that.saving,_that.storeSaving,_that.submitting,_that.discarding,_that.creating);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( bool hydrated,  KycSubject? subject,  KycForm form,  KycForm baseline,  KycDocs docs,  KycDocs baselineDocs,  StoreContactDraft store,  StoreContactDraft storeBaseline,  StoreContactErrors storeServerErrors,  StoreContactDraft? storeServerErrorsFor,  String? error,  KycStep? resumeStep,  KycLookup lookup,  Map<KYCField, KycFieldError> fieldErrors,  Set<String> uploading,  bool saving,  bool storeSaving,  bool submitting,  bool discarding,  bool creating)  $default,) {final _that = this;
switch (_that) {
case _KycWizardState():
return $default(_that.hydrated,_that.subject,_that.form,_that.baseline,_that.docs,_that.baselineDocs,_that.store,_that.storeBaseline,_that.storeServerErrors,_that.storeServerErrorsFor,_that.error,_that.resumeStep,_that.lookup,_that.fieldErrors,_that.uploading,_that.saving,_that.storeSaving,_that.submitting,_that.discarding,_that.creating);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( bool hydrated,  KycSubject? subject,  KycForm form,  KycForm baseline,  KycDocs docs,  KycDocs baselineDocs,  StoreContactDraft store,  StoreContactDraft storeBaseline,  StoreContactErrors storeServerErrors,  StoreContactDraft? storeServerErrorsFor,  String? error,  KycStep? resumeStep,  KycLookup lookup,  Map<KYCField, KycFieldError> fieldErrors,  Set<String> uploading,  bool saving,  bool storeSaving,  bool submitting,  bool discarding,  bool creating)?  $default,) {final _that = this;
switch (_that) {
case _KycWizardState() when $default != null:
return $default(_that.hydrated,_that.subject,_that.form,_that.baseline,_that.docs,_that.baselineDocs,_that.store,_that.storeBaseline,_that.storeServerErrors,_that.storeServerErrorsFor,_that.error,_that.resumeStep,_that.lookup,_that.fieldErrors,_that.uploading,_that.saving,_that.storeSaving,_that.submitting,_that.discarding,_that.creating);case _:
  return null;

}
}

}

/// @nodoc


class _KycWizardState with DiagnosticableTreeMixin implements KycWizardState {
  const _KycWizardState({this.hydrated = false, this.subject, this.form = const KycForm(), this.baseline = const KycForm(), this.docs = const KycDocs(), this.baselineDocs = const KycDocs(), this.store = const StoreContactDraft(), this.storeBaseline = const StoreContactDraft(), this.storeServerErrors = const StoreContactErrors(), this.storeServerErrorsFor, this.error, this.resumeStep, this.lookup = const KycLookupIdle(), final  Map<KYCField, KycFieldError> fieldErrors = const {}, final  Set<String> uploading = const {}, this.saving = false, this.storeSaving = false, this.submitting = false, this.discarding = false, this.creating = false}): _fieldErrors = fieldErrors,_uploading = uploading;
  

/// 已經從申請（或沒有申請）載入。
@override@JsonKey() final  bool hydrated;
/// 申請的身分；還沒有申請也還沒選時是 null（顯示選擇畫面）。
@override final  KycSubject? subject;
@override@JsonKey() final  KycForm form;
@override@JsonKey() final  KycForm baseline;
@override@JsonKey() final  KycDocs docs;
@override@JsonKey() final  KycDocs baselineDocs;
@override@JsonKey() final  StoreContactDraft store;
@override@JsonKey() final  StoreContactDraft storeBaseline;
@override@JsonKey() final  StoreContactErrors storeServerErrors;
@override final  StoreContactDraft? storeServerErrorsFor;
/// 最近一次動作失敗或提醒的文字。
@override final  String? error;
/// 載入後要停在哪一步；沒有申請或沒有缺項時是 null（從頭開始）。
@override final  KycStep? resumeStep;
@override@JsonKey() final  KycLookup lookup;
 final  Map<KYCField, KycFieldError> _fieldErrors;
@override@JsonKey() Map<KYCField, KycFieldError> get fieldErrors {
  if (_fieldErrors is EqualUnmodifiableMapView) return _fieldErrors;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(_fieldErrors);
}

 final  Set<String> _uploading;
@override@JsonKey() Set<String> get uploading {
  if (_uploading is EqualUnmodifiableSetView) return _uploading;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableSetView(_uploading);
}

@override@JsonKey() final  bool saving;
@override@JsonKey() final  bool storeSaving;
@override@JsonKey() final  bool submitting;
@override@JsonKey() final  bool discarding;
@override@JsonKey() final  bool creating;

/// Create a copy of KycWizardState
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$KycWizardStateCopyWith<_KycWizardState> get copyWith => __$KycWizardStateCopyWithImpl<_KycWizardState>(this, _$identity);


@override
void debugFillProperties(DiagnosticPropertiesBuilder properties) {
  properties
    ..add(DiagnosticsProperty('type', 'KycWizardState'))
    ..add(DiagnosticsProperty('hydrated', hydrated))..add(DiagnosticsProperty('subject', subject))..add(DiagnosticsProperty('form', form))..add(DiagnosticsProperty('baseline', baseline))..add(DiagnosticsProperty('docs', docs))..add(DiagnosticsProperty('baselineDocs', baselineDocs))..add(DiagnosticsProperty('store', store))..add(DiagnosticsProperty('storeBaseline', storeBaseline))..add(DiagnosticsProperty('storeServerErrors', storeServerErrors))..add(DiagnosticsProperty('storeServerErrorsFor', storeServerErrorsFor))..add(DiagnosticsProperty('error', error))..add(DiagnosticsProperty('resumeStep', resumeStep))..add(DiagnosticsProperty('lookup', lookup))..add(DiagnosticsProperty('fieldErrors', fieldErrors))..add(DiagnosticsProperty('uploading', uploading))..add(DiagnosticsProperty('saving', saving))..add(DiagnosticsProperty('storeSaving', storeSaving))..add(DiagnosticsProperty('submitting', submitting))..add(DiagnosticsProperty('discarding', discarding))..add(DiagnosticsProperty('creating', creating));
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _KycWizardState&&(identical(other.hydrated, hydrated) || other.hydrated == hydrated)&&(identical(other.subject, subject) || other.subject == subject)&&(identical(other.form, form) || other.form == form)&&(identical(other.baseline, baseline) || other.baseline == baseline)&&(identical(other.docs, docs) || other.docs == docs)&&(identical(other.baselineDocs, baselineDocs) || other.baselineDocs == baselineDocs)&&(identical(other.store, store) || other.store == store)&&(identical(other.storeBaseline, storeBaseline) || other.storeBaseline == storeBaseline)&&(identical(other.storeServerErrors, storeServerErrors) || other.storeServerErrors == storeServerErrors)&&(identical(other.storeServerErrorsFor, storeServerErrorsFor) || other.storeServerErrorsFor == storeServerErrorsFor)&&(identical(other.error, error) || other.error == error)&&(identical(other.resumeStep, resumeStep) || other.resumeStep == resumeStep)&&(identical(other.lookup, lookup) || other.lookup == lookup)&&const DeepCollectionEquality().equals(other._fieldErrors, _fieldErrors)&&const DeepCollectionEquality().equals(other._uploading, _uploading)&&(identical(other.saving, saving) || other.saving == saving)&&(identical(other.storeSaving, storeSaving) || other.storeSaving == storeSaving)&&(identical(other.submitting, submitting) || other.submitting == submitting)&&(identical(other.discarding, discarding) || other.discarding == discarding)&&(identical(other.creating, creating) || other.creating == creating));
}


@override
int get hashCode => Object.hashAll([runtimeType,hydrated,subject,form,baseline,docs,baselineDocs,store,storeBaseline,storeServerErrors,storeServerErrorsFor,error,resumeStep,lookup,const DeepCollectionEquality().hash(_fieldErrors),const DeepCollectionEquality().hash(_uploading),saving,storeSaving,submitting,discarding,creating]);

@override
String toString({ DiagnosticLevel minLevel = DiagnosticLevel.info }) {
  return 'KycWizardState(hydrated: $hydrated, subject: $subject, form: $form, baseline: $baseline, docs: $docs, baselineDocs: $baselineDocs, store: $store, storeBaseline: $storeBaseline, storeServerErrors: $storeServerErrors, storeServerErrorsFor: $storeServerErrorsFor, error: $error, resumeStep: $resumeStep, lookup: $lookup, fieldErrors: $fieldErrors, uploading: $uploading, saving: $saving, storeSaving: $storeSaving, submitting: $submitting, discarding: $discarding, creating: $creating)';
}


}

/// @nodoc
abstract mixin class _$KycWizardStateCopyWith<$Res> implements $KycWizardStateCopyWith<$Res> {
  factory _$KycWizardStateCopyWith(_KycWizardState value, $Res Function(_KycWizardState) _then) = __$KycWizardStateCopyWithImpl;
@override @useResult
$Res call({
 bool hydrated, KycSubject? subject, KycForm form, KycForm baseline, KycDocs docs, KycDocs baselineDocs, StoreContactDraft store, StoreContactDraft storeBaseline, StoreContactErrors storeServerErrors, StoreContactDraft? storeServerErrorsFor, String? error, KycStep? resumeStep, KycLookup lookup, Map<KYCField, KycFieldError> fieldErrors, Set<String> uploading, bool saving, bool storeSaving, bool submitting, bool discarding, bool creating
});




}
/// @nodoc
class __$KycWizardStateCopyWithImpl<$Res>
    implements _$KycWizardStateCopyWith<$Res> {
  __$KycWizardStateCopyWithImpl(this._self, this._then);

  final _KycWizardState _self;
  final $Res Function(_KycWizardState) _then;

/// Create a copy of KycWizardState
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? hydrated = null,Object? subject = freezed,Object? form = null,Object? baseline = null,Object? docs = null,Object? baselineDocs = null,Object? store = null,Object? storeBaseline = null,Object? storeServerErrors = null,Object? storeServerErrorsFor = freezed,Object? error = freezed,Object? resumeStep = freezed,Object? lookup = null,Object? fieldErrors = null,Object? uploading = null,Object? saving = null,Object? storeSaving = null,Object? submitting = null,Object? discarding = null,Object? creating = null,}) {
  return _then(_KycWizardState(
hydrated: null == hydrated ? _self.hydrated : hydrated // ignore: cast_nullable_to_non_nullable
as bool,subject: freezed == subject ? _self.subject : subject // ignore: cast_nullable_to_non_nullable
as KycSubject?,form: null == form ? _self.form : form // ignore: cast_nullable_to_non_nullable
as KycForm,baseline: null == baseline ? _self.baseline : baseline // ignore: cast_nullable_to_non_nullable
as KycForm,docs: null == docs ? _self.docs : docs // ignore: cast_nullable_to_non_nullable
as KycDocs,baselineDocs: null == baselineDocs ? _self.baselineDocs : baselineDocs // ignore: cast_nullable_to_non_nullable
as KycDocs,store: null == store ? _self.store : store // ignore: cast_nullable_to_non_nullable
as StoreContactDraft,storeBaseline: null == storeBaseline ? _self.storeBaseline : storeBaseline // ignore: cast_nullable_to_non_nullable
as StoreContactDraft,storeServerErrors: null == storeServerErrors ? _self.storeServerErrors : storeServerErrors // ignore: cast_nullable_to_non_nullable
as StoreContactErrors,storeServerErrorsFor: freezed == storeServerErrorsFor ? _self.storeServerErrorsFor : storeServerErrorsFor // ignore: cast_nullable_to_non_nullable
as StoreContactDraft?,error: freezed == error ? _self.error : error // ignore: cast_nullable_to_non_nullable
as String?,resumeStep: freezed == resumeStep ? _self.resumeStep : resumeStep // ignore: cast_nullable_to_non_nullable
as KycStep?,lookup: null == lookup ? _self.lookup : lookup // ignore: cast_nullable_to_non_nullable
as KycLookup,fieldErrors: null == fieldErrors ? _self._fieldErrors : fieldErrors // ignore: cast_nullable_to_non_nullable
as Map<KYCField, KycFieldError>,uploading: null == uploading ? _self._uploading : uploading // ignore: cast_nullable_to_non_nullable
as Set<String>,saving: null == saving ? _self.saving : saving // ignore: cast_nullable_to_non_nullable
as bool,storeSaving: null == storeSaving ? _self.storeSaving : storeSaving // ignore: cast_nullable_to_non_nullable
as bool,submitting: null == submitting ? _self.submitting : submitting // ignore: cast_nullable_to_non_nullable
as bool,discarding: null == discarding ? _self.discarding : discarding // ignore: cast_nullable_to_non_nullable
as bool,creating: null == creating ? _self.creating : creating // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}


}

// dart format on
