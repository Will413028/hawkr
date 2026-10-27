import 'package:connectrpc/connect.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/submit.dart';
import 'package:protobuf/protobuf.dart' show GeneratedMessage;
import 'package:test/test.dart';

ErrorDetail _detail(String type, GeneratedMessage m) =>
    ErrorDetail(type, m.writeToBuffer());

/// 預設文字由呼叫端從目錄注入；替身讓「用了預設文字」可以和任何伺服器文字區分。
String _defaults(Code code) => '預設:${code.name}';

void main() {
  group('submitError', () {
    test('VERSION_CONFLICT 是版本衝突', () {
      final err = ConnectException(
        Code.aborted,
        'etag',
        details: [
          _detail(
            'google.rpc.ErrorInfo',
            ErrorInfo()
              ..reason = 'VERSION_CONFLICT'
              ..domain = 'hawkr',
          ),
        ],
      );
      expect(submitError(err, defaults: _defaults), isA<SubmitConflict>());
    });

    test('BadRequest 轉成以 proto 路徑為 key 的欄位錯誤', () {
      final err = ConnectException(
        Code.invalidArgument,
        'invalid',
        details: [
          _detail(
            'google.rpc.BadRequest',
            BadRequest()
              ..fieldViolations.add(
                BadRequest_FieldViolation()
                  ..field_1 = 'banner.title'
                  ..description = 'required'
                  ..localizedMessage = (LocalizedMessage()
                    ..locale = 'zh-TW'
                    ..message = '請輸入標題'),
              ),
          ),
          _detail(
            'google.rpc.LocalizedMessage',
            LocalizedMessage()
              ..locale = 'zh-TW'
              ..message = '資料有誤',
          ),
        ],
      );
      final got = submitError(err, defaults: _defaults);
      expect(got, isA<SubmitFields>());
      got as SubmitFields;
      expect(got.fields, {'banner.title': '請輸入標題'});
      expect(got.message, '資料有誤');
    });

    test('其他錯誤顯示注入的該 code 預設文字', () {
      final got = submitError(
        ConnectException(Code.notFound, 'dev message'),
        defaults: _defaults,
      );
      expect(got, isA<SubmitMessage>());
      expect((got as SubmitMessage).message, '預設:not_found');
    });

    test('不是 ConnectException 的錯誤顯示 unknown 的預設文字', () {
      final got = submitError(StateError('boom'), defaults: _defaults);
      expect((got as SubmitMessage).message, '預設:unknown');
    });

    test('取消顯示 internal 的預設文字', () {
      final got = submitError(
        ConnectException(Code.canceled, 'x'),
        defaults: _defaults,
      );
      expect((got as SubmitMessage).message, '預設:internal');
    });

    test('欄位只有 description 時，欄位文字是 invalidArgument 的預設文字，不是 description', () {
      final err = ConnectException(
        Code.invalidArgument,
        'invalid',
        details: [
          _detail(
            'google.rpc.BadRequest',
            BadRequest()
              ..fieldViolations.add(
                BadRequest_FieldViolation()
                  ..field_1 = 'banner.title'
                  ..description = 'required',
              ),
          ),
        ],
      );
      final got = submitError(err, defaults: _defaults) as SubmitFields;
      expect(got.fields, {'banner.title': '預設:invalid_argument'});
    });
  });

  group('fieldMessage', () {
    test('比對完整路徑、子欄位與索引，不比對同前綴的其他欄位', () {
      final fields = {
        'banner.action.url': '網址格式錯誤',
        'product.variants[2].name': '請輸入規格名稱',
      };
      expect(fieldMessage(fields, ['banner.action']), '網址格式錯誤');
      expect(fieldMessage(fields, ['product.variants']), '請輸入規格名稱');
      expect(
        fieldMessage({'banner.actions_note': 'x'}, ['banner.action']),
        isNull,
      );
    });

    test('依路徑順序取第一個有錯誤的', () {
      expect(fieldMessage({'b': '乙'}, ['a', 'b']), '乙');
    });
  });
}
