import '../../../shared/utils/platform_io.dart';
import '../../../shared/utils/hk_log.dart';
import 'universal_ocr_service.dart';

/// 存摺 OCR 辨識結果
class BankBookOcrResult {
  final String? bankName; // 銀行名稱 (含分行)
  final String? accountNumber; // 帳號
  final String? rawText; // 原始辨識文字
  final OcrEngine? engine; // 使用的 OCR 引擎

  BankBookOcrResult({
    this.bankName,
    this.accountNumber,
    this.rawText,
    this.engine,
  });

  bool get hasData => bankName != null || accountNumber != null;

  @override
  String toString() {
    // 個資合規（issue #608 §2）：銀行帳號屬敏感個資，一律遮蔽。
    final maskedAccount = accountNumber == null ? 'null' : '***';
    return 'BankBookOcrResult(bankName: $bankName, accountNumber: $maskedAccount, engine: $engine)';
  }
}

/// 存摺 OCR 服務
///
/// 整合 UniversalOcrService：
/// - iOS 平台：使用原生 Vision 框架
class BankBookOcrService {
  /// 台灣主要銀行列表（按優先順序排列）
  /// 包含銀行代碼 and 標準名稱的映射
  static const Map<String, String> _bankNameMapping = {
    // 郵局（代碼 700）- 帳號14位
    '郵政存簿儲金': '中華郵政',
    '郵政儲金': '中華郵政',
    '中華郵政': '中華郵政',
    '郵局': '中華郵政',
    // 八大行庫
    '臺灣銀行': '臺灣銀行',
    '台灣銀行': '臺灣銀行',
    '臺灣土地銀行': '臺灣土地銀行',
    '土地銀行': '臺灣土地銀行',
    '合作金庫': '合作金庫商業銀行',
    '合庫商業銀行': '合作金庫商業銀行',
    '合庫': '合作金庫商業銀行',
    '第一商業銀行': '第一商業銀行',
    '第一銀行': '第一商業銀行',
    '一銀': '第一商業銀行',
    '華南商業銀行': '華南商業銀行',
    '華南銀行': '華南商業銀行',
    '彰化商業銀行': '彰化商業銀行',
    '彰化銀行': '彰化商業銀行',
    '彰銀': '彰化商業銀行',
    '兆豐國際商業銀行': '兆豐國際商業銀行',
    '兆豐銀行': '兆豐國際商業銀行',
    '臺灣中小企業銀行': '臺灣中小企業銀行',
    '台灣企銀': '臺灣中小企業銀行',
    '臺灣企銀': '臺灣中小企業銀行',
    // 民營銀行
    '台北富邦商業銀行': '台北富邦商業銀行',
    '台北富邦銀行': '台北富邦商業銀行',
    '台北富邦': '台北富邦商業銀行',
    '富邦銀行': '台北富邦商業銀行',
    '國泰世華商業銀行': '國泰世華商業銀行',
    '國泰世華銀行': '國泰世華商業銀行',
    '國泰世華': '國泰世華商業銀行',
    '玉山商業銀行': '玉山商業銀行',
    '玉山銀行': '玉山商業銀行',
    '台新國際商業銀行': '台新國際商業銀行',
    '台新銀行': '台新國際商業銀行',
    '中國信託商業銀行': '中國信託商業銀行',
    '中國信託': '中國信託商業銀行',
    '中信銀行': '中國信託商業銀行',
    '永豐商業銀行': '永豐商業銀行',
    '永豐銀行': '永豐商業銀行',
    '元大商業銀行': '元大商業銀行',
    '元大銀行': '元大商業銀行',
    '凱基商業銀行': '凱基商業銀行',
    '凱基銀行': '凱基商業銀行',
    '聯邦商業銀行': '聯邦商業銀行',
    '聯邦銀行': '聯邦商業銀行',
    '遠東國際商業銀行': '遠東國際商業銀行',
    '遠東銀行': '遠東國際商業銀行',
    '上海商業儲蓄銀行': '上海商業儲蓄銀行',
    '上海商銀': '上海商業儲蓄銀行',
    '臺灣新光商業銀行': '臺灣新光商業銀行',
    '新光銀行': '臺灣新光商業銀行',
    '台灣新光': '臺灣新光商業銀行',
    '臺中商業銀行': '臺中商業銀行',
    '台中銀行': '臺中商業銀行',
    '臺中銀行': '臺中商業銀行',
    '高雄銀行': '高雄銀行',
    '京城商業銀行': '京城商業銀行',
    '京城銀行': '京城商業銀行',
    '瑞興商業銀行': '瑞興商業銀行',
    '瑞興銀行': '瑞興商業銀行',
    '華泰商業銀行': '華泰商業銀行',
    '華泰銀行': '華泰商業銀行',
    '陽信商業銀行': '陽信商業銀行',
    '陽信銀行': '陽信商業銀行',
    '板信商業銀行': '板信商業銀行',
    '板信銀行': '板信商業銀行',
    '三信商業銀行': '三信商業銀行',
    '三信銀行': '三信商業銀行',
    '安泰商業銀行': '安泰商業銀行',
    '安泰銀行': '安泰商業銀行',
    '日盛國際商業銀行': '日盛國際商業銀行',
    '日盛銀行': '日盛國際商業銀行',
    '星展銀行': '星展銀行',
    '渣打國際商業銀行': '渣打國際商業銀行',
    '渣打銀行': '渣打國際商業銀行',
    '滙豐銀行': '滙豐銀行',
    'HSBC': '滙豐銀行',
    '花旗銀行': '花旗銀行',
    'Citibank': '花旗銀行',
  };

  /// 檢查是否為郵局
  static bool _isPostOffice(String? bankName) {
    if (bankName == null) return false;
    return bankName.contains('郵政') ||
        bankName.contains('郵局') ||
        bankName == '中華郵政';
  }

  /// 辨識存摺封面
  static Future<BankBookOcrResult> recognize(File imageFile) async {
    try {
      HKLog.logD('[BankBookOCR] Recognizing bank book...');
      final result = await UniversalOcrService.recognizeText(imageFile);

      if (!result.success) {
        HKLog.logD('[BankBookOCR] OCR error: ${result.errorMessage}');
        return BankBookOcrResult(
          rawText: 'Error: ${result.errorMessage}',
          engine: result.engine,
        );
      }

      // 個資合規（issue #608 §2）：OCR 原始文字含銀行帳號，禁止落 log。
      HKLog.logD(
        '[BankBookOCR] OCR raw text redacted (len=${result.text.length})',
      );
      final parsed = _parseBankBook(result.text);
      return BankBookOcrResult(
        bankName: parsed.bankName,
        accountNumber: parsed.accountNumber,
        rawText: result.text,
        engine: result.engine,
      );
    } catch (e) {
      HKLog.logD('[BankBookOCR] Error recognizing bank book: $e');
      return BankBookOcrResult(rawText: 'Error: $e');
    }
  }

  // ============================================================
  // Parsing Logic (Shared)
  // ============================================================

  /// 解析存摺文字
  static BankBookOcrResult _parseBankBook(String text) {
    String? bankName;
    String? standardBankName;
    String? accountNumber;
    bool isPostOffice = false;

    final lines = text.split('\n');

    // 1. 辨識銀行名稱（使用映射表）
    for (final entry in _bankNameMapping.entries) {
      if (text.contains(entry.key)) {
        bankName = entry.key;
        standardBankName = entry.value;
        isPostOffice = _isPostOffice(entry.key);
        HKLog.logD(
          '[BankBookOCR] Found bank: $bankName -> $standardBankName (isPostOffice: $isPostOffice)',
        );
        break;
      }
    }

    // 2. 嘗試找分行名稱
    String? branch;
    final branchRegex = RegExp(r'([^\s]{2,8}(?:分行|分局|支局))');
    final branchMatch = branchRegex.firstMatch(text);
    if (branchMatch != null) {
      branch = branchMatch.group(1);
      HKLog.logD('[BankBookOCR] Found branch: $branch');
    }

    // 組合銀行名稱（標準名稱 + 分行）
    if (standardBankName != null) {
      if (branch != null && !branch.contains(standardBankName)) {
        bankName = '$standardBankName $branch';
      } else {
        bankName = standardBankName;
      }
    }

    // 3. 辨識帳號
    // 郵局帳號：14位（局號7位 + 帳號7位），可能顯示為 XXXXXXX-XXXXXXX
    // 一般銀行帳號：10-14位
    if (isPostOffice) {
      accountNumber = _extractPostOfficeAccount(text, lines);
    } else {
      accountNumber = _extractBankAccount(text, lines);
    }

    return BankBookOcrResult(
      bankName: bankName,
      accountNumber: accountNumber,
      rawText: text,
    );
  }

  /// 提取郵局帳號（14位：局號7位 + 帳號7位）
  static String? _extractPostOfficeAccount(String text, List<String> lines) {
    String? accountNumber;

    HKLog.logD('[BankBookOCR] _extractPostOfficeAccount called');
    // 個資合規（issue #608 §2）：原始文字含帳號，禁止落 log。
    HKLog.logD(
      '[BankBookOCR] Raw text for post office redacted (len=${text.length})',
    );

    // 格式1: 完整14位數字
    final fullAccountRegex = RegExp(r'(\d{14})');
    final fullMatch = fullAccountRegex.firstMatch(text);
    if (fullMatch != null) {
      accountNumber = fullMatch.group(0);
      HKLog.logD(
        '[BankBookOCR] Found post office account (14 digits, redacted)',
      );
      return accountNumber;
    }

    // 格式2: 局號-帳號（7-7）或有空格分隔
    final splitAccountRegex = RegExp(r'(\d{7})[\s\-\.]+(\d{7})');
    final splitMatch = splitAccountRegex.firstMatch(text);
    if (splitMatch != null) {
      accountNumber = '${splitMatch.group(1)}${splitMatch.group(2)}';
      HKLog.logD('[BankBookOCR] Found post office account (7-7, redacted)');
      return accountNumber;
    }

    // 格式3: 找任意兩組 7 位數字（可能是局號和帳號）
    final sevenDigitMatches = RegExp(r'\d{7}').allMatches(text).toList();
    HKLog.logD(
      '[BankBookOCR] Found ${sevenDigitMatches.length} 7-digit numbers',
    );
    if (sevenDigitMatches.length >= 2) {
      final first = sevenDigitMatches[0].group(0)!;
      final second = sevenDigitMatches[1].group(0)!;
      accountNumber = '$first$second';
      HKLog.logD('[BankBookOCR] Combined two 7-digit numbers (redacted)');
      return accountNumber;
    }

    // 格式4: 找「局號」和「帳號」標籤
    String? officeCode;
    String? accNum;
    for (final line in lines) {
      // 個資合規（issue #608 §2）：原始行文字含帳號，禁止落 log。
      HKLog.logD('[BankBookOCR] Checking line (redacted)');
      if (line.contains('局號') ||
          line.contains('郵局代號') ||
          line.contains('儲金') ||
          line.contains('儲匯')) {
        final match = RegExp(r'(\d{6,8})').firstMatch(line);
        if (match != null) {
          officeCode = match.group(0);
          // 確保是 7 位
          if (officeCode!.length == 6) officeCode = '0$officeCode';
          if (officeCode.length > 7) officeCode = officeCode.substring(0, 7);
          HKLog.logD('[BankBookOCR] Found office code (redacted)');
        }
      }
      if (line.contains('帳號') || line.contains('戶號') || line.contains('號碼')) {
        final match = RegExp(r'(\d{6,8})').firstMatch(line);
        if (match != null) {
          accNum = match.group(0);
          // 確保是 7 位
          if (accNum!.length == 6) accNum = '0$accNum';
          if (accNum.length > 7) accNum = accNum.substring(0, 7);
          HKLog.logD('[BankBookOCR] Found account part (redacted)');
        }
      }
    }
    if (officeCode != null && accNum != null) {
      accountNumber = '$officeCode$accNum';
      HKLog.logD('[BankBookOCR] Combined post office account (redacted)');
      return accountNumber;
    }

    // 格式5: 找 12-14 位的連續數字
    final longNumberRegex = RegExp(r'\d{12,14}');
    final longMatch = longNumberRegex.firstMatch(text);
    if (longMatch != null) {
      accountNumber = longMatch.group(0);
      // 如果是 12 或 13 位，可能需要補零
      while (accountNumber!.length < 14) {
        accountNumber = '0$accountNumber';
      }
      HKLog.logD('[BankBookOCR] Found long number (12-14 digits, redacted)');
      return accountNumber;
    }

    // 格式6: 找任意長度合理的數字串（備用）
    final anyNumberRegex = RegExp(r'\d{10,}');
    final anyMatch = anyNumberRegex.firstMatch(text);
    if (anyMatch != null) {
      accountNumber = anyMatch.group(0);
      // 取前 14 位
      if (accountNumber!.length > 14) {
        accountNumber = accountNumber.substring(0, 14);
      }
      HKLog.logD('[BankBookOCR] Found number (10+ digits, redacted)');
      return accountNumber;
    }

    // 最後備用：使用一般銀行帳號擷取
    HKLog.logD('[BankBookOCR] Falling back to general account extraction');
    return _extractBankAccount(text, lines, minLength: 10, maxLength: 14);
  }

  /// 提取一般銀行帳號（10-14位）
  static String? _extractBankAccount(
    String text,
    List<String> lines, {
    int minLength = 10,
    int maxLength = 14,
  }) {
    String? accountNumber;

    HKLog.logD(
      '[BankBookOCR] _extractBankAccount: minLength=$minLength, maxLength=$maxLength',
    );
    // 個資合規（issue #608 §2）：原始文字含帳號，禁止落 log。
    HKLog.logD('[BankBookOCR] Text to search redacted (len=${text.length})');

    // 1. 找包含「帳號」標籤的行
    for (final line in lines) {
      if (line.contains('帳號') ||
          line.contains('戶號') ||
          line.contains('Account') ||
          line.contains('A/C')) {
        HKLog.logD('[BankBookOCR] Found labeled line (redacted)');
        // 先找有分隔符的帳號
        final separatedMatch = RegExp(r'(\d[\d\-\s]{8,18}\d)').firstMatch(line);
        if (separatedMatch != null) {
          final digitsOnly = separatedMatch
              .group(0)!
              .replaceAll(RegExp(r'[^\d]'), '');
          if (digitsOnly.length >= minLength &&
              digitsOnly.length <= maxLength) {
            accountNumber = digitsOnly;
            HKLog.logD(
              '[BankBookOCR] Found account from labeled line (redacted)',
            );
            return accountNumber;
          }
        }
        // 再找純數字（使用正確的字串插值）
        final numRegex = RegExp('(\\d{$minLength,$maxLength})');
        final numMatch = numRegex.firstMatch(line);
        if (numMatch != null) {
          accountNumber = numMatch.group(0);
          HKLog.logD(
            '[BankBookOCR] Found account from labeled line (digits only, redacted)',
          );
          return accountNumber;
        }
      }
    }

    // 2. 找連續數字（可能有分隔符）
    final accountRegex = RegExp(r'(\d[\d\-\s]{8,16}\d)');
    final accountMatches = accountRegex.allMatches(text).toList();
    HKLog.logD(
      '[BankBookOCR] Found ${accountMatches.length} potential account patterns',
    );

    for (final match in accountMatches) {
      final candidate = match.group(0)!;
      final digitsOnly = candidate.replaceAll(RegExp(r'[^\d]'), '');
      // 個資合規（issue #608 §2）：候選帳號數字禁止落 log，僅記錄長度。
      HKLog.logD(
        '[BankBookOCR] Checking candidate (redacted, len: ${digitsOnly.length})',
      );
      if (digitsOnly.length >= minLength && digitsOnly.length <= maxLength) {
        accountNumber = digitsOnly;
        HKLog.logD('[BankBookOCR] Found account number (redacted)');
        return accountNumber;
      }
    }

    // 3. 找所有符合長度的數字串
    final allNumbersRegex = RegExp('\\d{$minLength,$maxLength}');
    final allNumbers = allNumbersRegex.allMatches(text).toList();
    HKLog.logD(
      '[BankBookOCR] Found ${allNumbers.length} numbers with length $minLength-$maxLength',
    );
    if (allNumbers.isNotEmpty) {
      // 優先取較長的數字串（更可能是帳號）
      String best = '';
      for (final match in allNumbers) {
        final num = match.group(0)!;
        HKLog.logD(
          '[BankBookOCR] Candidate number (redacted, len: ${num.length})',
        );
        if (num.length > best.length) {
          best = num;
        }
      }
      if (best.isNotEmpty) {
        accountNumber = best;
        HKLog.logD('[BankBookOCR] Found account (fallback, redacted)');
        return accountNumber;
      }
    }

    // 4. 最後嘗試：找任何 10 位以上的連續數字
    final anyLongNumber = RegExp(r'\d{10,}').firstMatch(text);
    if (anyLongNumber != null) {
      final num = anyLongNumber.group(0)!;
      // 截取最多 maxLength 位
      accountNumber = num.length > maxLength
          ? num.substring(0, maxLength)
          : num;
      HKLog.logD('[BankBookOCR] Found long number (last resort, redacted)');
      return accountNumber;
    }

    HKLog.logD('[BankBookOCR] No account number found');
    return null;
  }

  /// 釋放資源
  static void dispose() {
    // 資源已由原生端自動釋放
  }
}
