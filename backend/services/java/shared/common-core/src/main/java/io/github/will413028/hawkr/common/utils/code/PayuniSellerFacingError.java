package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni 合作商店 API 錯誤碼與商店／會員審核狀態碼的賣家可讀文案。來源 https://docs.payuni.com.tw/web/#/9/66 與 #/9/138
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PayuniSellerFacingError implements CodeMeta {

    AGENT01001("AGENT01001", "AGENT01001", "系統設定有誤，請聯繫客服協助處理", "請填入代理商代號；平台憑證問題"),
    AGENT01002("AGENT01002", "AGENT01002", "系統設定有誤，請聯繫客服協助處理", "查無符合代理商資料；平台憑證問題"),
    AGENT01003("AGENT01003", "AGENT01003", "系統驗證失敗，請聯繫客服協助處理", "資料HASH比對不符合；平台加解密問題"),
    AGENT01004("AGENT01004", "AGENT01004", "系統驗證失敗，請聯繫客服協助處理", "資料解密失敗；平台加解密問題"),
    AGENT01005("AGENT01005", "AGENT01005", "系統驗證失敗，請聯繫客服協助處理", "解密資料不存在；平台加解密問題"),
    AGENT02001("AGENT02001", "AGENT02001", "物流服務尚未設定，請聯繫客服協助開通", "查無符合商店物流資料；物流 API 專用"),
    AGENT02002("AGENT02002", "AGENT02002", "宅配物流服務尚未開通，請聯繫客服協助", "商店B2C物流服務尚未開通；物流 API 專用"),
    AGENT02003("AGENT02003", "AGENT02003", "超商物流服務尚未開通，請聯繫客服協助", "商店C2C物流服務尚未開通；物流 API 專用"),
    AGENT02004("AGENT02004", "AGENT02004", "宅配物流服務未啟用，請聯繫客服協助", "商店未啟用B2C物流服務；物流 API 專用"),
    AGENT02005("AGENT02005", "AGENT02005", "超商物流服務未啟用，請聯繫客服協助", "商店未啟用C2C物流服務；物流 API 專用"),
    AGENT03001("AGENT03001", "AGENT03001", "請填寫姓名或公司名稱", "請填入會員名稱；來源 Name"),
    AGENT03002("AGENT03002", "AGENT03002", "請填寫統一編號或身分證字號", "請填入統編或身分證字號；來源 UBN"),
    AGENT03003("AGENT03003", "AGENT03003", "統一編號或身分證字號位數過長，請確認後重新填寫", "統編或身分證字號長度過長；來源 UBN"),
    AGENT03004("AGENT03004", "AGENT03004", "統一編號或身分證字號格式有誤，請確認後重新填寫", "統編或身分證字號格式有誤；來源 UBN"),
    AGENT03005("AGENT03005", "AGENT03005", "請填寫聯絡電話", "請填入電話；來源 ContactTel"),
    AGENT03006("AGENT03006", "AGENT03006", "聯絡電話位數過長，請確認後重新填寫", "電話長度過長；來源 ContactTel"),
    AGENT03007("AGENT03007", "AGENT03007", "聯絡電話格式不符，請確認後重新填寫", "電話格式不符；來源 ContactTel"),
    AGENT03008("AGENT03008", "AGENT03008", "請填寫電子信箱", "請填入會員電子信箱；來源 Email"),
    AGENT03009("AGENT03009", "AGENT03009", "電子信箱格式有誤，請確認後重新填寫", "會員電子信箱格式錯誤；來源 Email"),
    AGENT03010("AGENT03010", "AGENT03010", "請填寫聯絡地址", "請填入會員地址；來源 MerAddr"),
    AGENT03011("AGENT03011", "AGENT03011", "金流帳號建立失敗，請聯繫客服協助處理", "請填入會員帳號；本平台未送此欄位"),
    AGENT03012("AGENT03012", "AGENT03012", "金流帳號建立失敗，請聯繫客服協助處理", "會員登入帳號長度過長；本平台未送此欄位"),
    AGENT03013("AGENT03013", "AGENT03013", "金流帳戶資料需由平台補送，請聯繫客服協助處理", "請填入會員銀行帳戶總行與分行代碼；BindSellerRequest 無銀行欄位"),
    AGENT03014("AGENT03014", "AGENT03014", "金流帳戶資料需由平台補送，請聯繫客服協助處理", "會員銀行帳戶總行與分行代碼長度過長；BindSellerRequest 無銀行欄位"),
    AGENT03015("AGENT03015", "AGENT03015", "金流帳戶資料需由平台補送，請聯繫客服協助處理", "請填入會員銀行帳戶；BindSellerRequest 無銀行欄位"),
    AGENT03016("AGENT03016", "AGENT03016", "金流帳戶資料需由平台補送，請聯繫客服協助處理", "會員銀行帳戶長度過長；BindSellerRequest 無銀行欄位"),
    AGENT03017("AGENT03017", "AGENT03017", "金流帳戶資料需由平台補送，請聯繫客服協助處理", "請填入會員銀行帳戶戶名；BindSellerRequest 無銀行欄位"),
    AGENT03019("AGENT03019", "AGENT03019", "這組電子信箱或手機號碼已被其他 PayUni 帳號使用，請聯繫客服協助確認", "會員帳號或手機或Email資料重複；PayUni 端既有會員"),
    AGENT03020("AGENT03020", "AGENT03020", "金流帳戶建立失敗，請聯繫客服協助處理", "會員建立銀行帳號失敗；可能是銀行代號不存在"),
    AGENT03021("AGENT03021", "AGENT03021", "金流系統暫時無法處理，請稍後再確認申請狀態", "會員更新資料失敗；PayUni 端暫時性"),
    AGENT03022("AGENT03022", "AGENT03022", "金流帳戶資料讀取失敗，請聯繫客服協助處理", "會員銀行資料取得失敗"),
    AGENT03023("AGENT03023", "AGENT03023", "金流帳戶資料更新失敗，請聯繫客服協助處理", "會員銀行資料更新失敗"),
    AGENT03024("AGENT03024", "AGENT03024", "金流帳號建立失敗，請聯繫客服協助處理", "缺少會員編號；平台送出參數問題"),
    AGENT03025("AGENT03025", "AGENT03025", "金流系統暫時無法處理，請稍後再確認申請狀態", "取得會員資料失敗；PayUni 端暫時性"),
    AGENT03026("AGENT03026", "AGENT03026", "金流系統暫時無法處理，請稍後再確認申請狀態", "修改會員資料失敗；PayUni 端暫時性"),
    AGENT03027("AGENT03027", "AGENT03027", "姓名或公司名稱格式有誤，請確認後重新填寫", "會員名稱格式錯誤；來源 Name"),
    AGENT03028("AGENT03028", "AGENT03028", "姓名長度不可超過 25 個字，請確認後重新填寫", "個人會員名稱長度不得超過25字元；來源 Name"),
    AGENT03029("AGENT03029", "AGENT03029", "公司名稱長度不可超過 200 個字，請確認後重新填寫", "企業會員名稱長度不得超過200字元；來源 Name"),
    AGENT03031("AGENT03031", "AGENT03031", "這個 PayUni 會員已被停權，無法開通收款，請聯繫客服協助確認", "此會員已停權，不可建立商店"),
    AGENT04000("AGENT04000", "AGENT04000", "商店編號產生失敗，請聯繫客服協助處理", "請填入商店代號；來源平台生成 MerID"),
    AGENT04001("AGENT04001", "AGENT04001", "請填寫商店電子信箱", "請填入商店電子信箱；來源 MerEmail"),
    AGENT04002("AGENT04002", "AGENT04002", "商店電子信箱格式有誤，請確認後重新填寫", "商店電子信箱格式錯誤；來源 MerEmail"),
    AGENT04003("AGENT04003", "AGENT04003", "請填寫商店名稱", "請填入商店名稱；來源 MerName"),
    AGENT04004("AGENT04004", "AGENT04004", "請填寫商店英文名稱", "請填入商店英文名稱；來源 MerEngName"),
    AGENT04005("AGENT04005", "AGENT04005", "商店網址設定有誤，請聯繫客服協助處理", "請填入商店網址；來源平台生成 MerURL"),
    AGENT04006("AGENT04006", "AGENT04006", "商店網址設定有誤，請聯繫客服協助處理", "商店網址格式錯誤；來源平台生成 MerURL"),
    AGENT04007("AGENT04007", "AGENT04007", "請填寫商店地址", "請填入商店地址；來源 MerAddr"),
    AGENT04008("AGENT04008", "AGENT04008", "商店電話需由平台補送，請聯繫客服協助處理", "請填入商店電話；本平台未送 MerTel"),
    AGENT04009("AGENT04009", "AGENT04009", "商店電話需由平台補送，請聯繫客服協助處理", "商店電話長度過長；本平台未送 MerTel"),
    AGENT04010("AGENT04010", "AGENT04010", "商店電話需由平台補送，請聯繫客服協助處理", "商店電話格式錯誤；本平台未送 MerTel"),
    AGENT04011("AGENT04011", "AGENT04011", "請填寫商店簡介", "請填入商店簡介；來源 MerDesc"),
    AGENT04012("AGENT04012", "AGENT04012", "商店類型設定有誤，請聯繫客服協助處理", "請填入販賣類型；來源平台常數 SellType"),
    AGENT04013("AGENT04013", "AGENT04013", "商店類型設定有誤，請聯繫客服協助處理", "販賣類型需為數字型態；來源平台常數 SellType"),
    AGENT04014("AGENT04014", "AGENT04014", "商店類型設定有誤，請聯繫客服協助處理", "販賣類型長度過長；來源平台常數 SellType"),
    AGENT04015("AGENT04015", "AGENT04015", "商店行業別設定有誤，請聯繫客服協助處理", "請填入行業別；來源平台常數 MerMCC"),
    AGENT04016("AGENT04016", "AGENT04016", "商店行業別設定有誤，請聯繫客服協助處理", "行業別長度過長；來源平台常數 MerMCC"),
    AGENT04017("AGENT04017", "AGENT04017", "商店編號已被使用，請聯繫客服協助確認", "商店代號已存在；優先走 recoverFromExistingMerchant 自癒"),
    AGENT04018("AGENT04018", "AGENT04018", "商品交付設定有誤，請聯繫客服協助處理", "商品交付類型須為數字型態；來源平台常數 GoodsRate"),
    AGENT04019("AGENT04019", "AGENT04019", "商品交付設定有誤，請聯繫客服協助處理", "商品交付類型代碼長度過長；來源平台常數 GoodsRate"),
    AGENT04020("AGENT04020", "AGENT04020", "商品交付設定有誤，請聯繫客服協助處理", "請填入商品交付類型；來源平台常數 GoodsRate"),
    AGENT04021("AGENT04021", "AGENT04021", "金流系統暫時無法建立商店，請稍後再確認申請狀態", "商店建立失敗；PayUni 端暫時性"),
    AGENT04022("AGENT04022", "AGENT04022", "請填寫商店地址的郵遞區號", "請填入商店地址郵遞區號；來源 MerAddrPost"),
    AGENT04023("AGENT04023", "AGENT04023", "商店地址的郵遞區號查不到，請確認縣市與行政區後重新填寫", "商店地址郵遞區號不存在；來源 MerAddrPost"),
    AGENT04024("AGENT04024", "AGENT04024", "商店編號產生失敗，請聯繫客服協助處理", "請填入商店編號；來源平台生成 MerID"),
    AGENT04025("AGENT04025", "AGENT04025", "商店編號產生失敗，請聯繫客服協助處理", "商店代號長度不可超過20位；來源平台生成 MerID"),
    AGENT04026("AGENT04026", "AGENT04026", "商店行業別設定有誤，請聯繫客服協助處理", "行業別資料必須為數字；來源平台常數 MerMCC"),
    AGENT04027("AGENT04027", "AGENT04027", "商店簡介字數過長，請精簡後重新填寫", "商店簡介字數過長；來源 MerDesc"),
    AGENT04028("AGENT04028", "AGENT04028", "商店英文名稱只能使用英文、數字、半形空白與常見符號，請確認後重新填寫", "國外卡英文帳單名稱字元限制；來源 MerEngName"),
    AGENT04029("AGENT04029", "AGENT04029", "商店英文名稱不可超過 50 個字元，請確認後重新填寫", "國外卡英文帳單名稱僅容許50字元；來源 MerEngName"),
    AGENT05001("AGENT05001", "AGENT05001", "系統設定有誤，請聯繫客服協助處理", "代理商未啟用；平台設定"),
    AGENT05002("AGENT05002", "AGENT05002", "商店編號產生失敗，請聯繫客服協助處理", "商店代號前贅字錯誤；平台設定"),
    AGENT05003("AGENT05003", "AGENT05003", "系統設定有誤，請聯繫客服協助處理", "請填入代理商編號；平台設定"),
    AGENT05004("AGENT05004", "AGENT05004", "系統設定有誤，請聯繫客服協助處理", "請填入代理商前綴字詞；平台設定"),
    AGENT05005("AGENT05005", "AGENT05005", "系統設定有誤，請聯繫客服協助處理", "代理商前綴字詞超過長度；平台設定"),
    AGENT05006("AGENT05006", "AGENT05006", "商店編號產生失敗，請聯繫客服協助處理", "商店代號在前贅字後只能放數字；平台設定"),
    AGENT05007("AGENT05007", "AGENT05007", "系統設定有誤，請聯繫客服協助處理", "ReturnURL網址格式錯誤；平台設定"),
    AGENT05008("AGENT05008", "AGENT05008", "系統設定有誤，請聯繫客服協助處理", "帶入合作商店方式錯誤；平台設定"),
    AGENT06001("AGENT06001", "AGENT06001", "請填寫聯絡人姓名", "請填入聯絡人姓名；來源 ContactName"),
    AGENT06002("AGENT06002", "AGENT06002", "聯絡人英文姓名需由平台補送，請聯繫客服協助處理", "請填入聯絡人英文姓名；本平台未送此欄位"),
    AGENT06003("AGENT06003", "AGENT06003", "請填寫聯絡人市話", "請填入聯絡人電話；來源 ContactTel"),
    AGENT06004("AGENT06004", "AGENT06004", "聯絡人市話位數過長，請確認後重新填寫", "聯絡人電話長度過長；來源 ContactTel"),
    AGENT06005("AGENT06005", "AGENT06005", "聯絡人市話格式需為區碼加號碼，區碼以 0 開頭，例如 02-9888888", "聯絡人電話格式錯誤；來源 ContactTel"),
    AGENT06006("AGENT06006", "AGENT06006", "請填寫聯絡人手機號碼", "請填入聯絡人行動電話；來源 ContactMobile"),
    AGENT06007("AGENT06007", "AGENT06007", "聯絡人手機號碼位數過長，請確認後重新填寫", "聯絡人行動電話長度過長；來源 ContactMobile"),
    AGENT06008("AGENT06008", "AGENT06008", "聯絡人手機號碼格式有誤，請確認後重新填寫", "聯絡人行動電話格式不符；來源 ContactMobile"),
    AGENT06009("AGENT06009", "AGENT06009", "聯絡人手機與市話請至少填寫一項", "兩者擇一必填；來源 ContactMobile/ContactTel"),
    AGENT07001("AGENT07001", "AGENT07001", "收款方式設定有誤，請聯繫客服協助處理", "請填入至少一種支付方式；來源平台常數 Payment"),
    AGENT07002("AGENT07002", "AGENT07002", "收款方式設定有誤，請聯繫客服協助處理", "支付方式設定錯誤；來源平台常數 Payment"),
    AGENT07003("AGENT07003", "AGENT07003", "金流系統暫時無法處理，請稍後再確認申請狀態", "系統錯誤；商店編號未正確儲存"),
    AGENT07004("AGENT07004", "AGENT07004", "手續費設定需由平台調整，請聯繫客服協助處理", "手續費設定錯誤；平台與 PayUni 約定值"),
    AGENT07005("AGENT07005", "AGENT07005", "撥款天期設定需由平台調整，請聯繫客服協助處理", "撥款天期設定錯誤；平台與 PayUni 約定值"),
    AGENT07006("AGENT07006", "AGENT07006", "金流系統暫時無法處理，請稍後再確認申請狀態", "系統錯誤；更新商店手續費失敗"),
    AGENT07007("AGENT07007", "AGENT07007", "金流系統暫時無法處理，請稍後再確認申請狀態", "系統錯誤；回傳參數空白"),
    AGENT07008("AGENT07008", "AGENT07008", "金流系統暫時無法處理，請稍後再確認申請狀態", "系統錯誤；綁定商店資料異常"),
    AGENT07009("AGENT07009", "AGENT07009", "這個會員類型無法開通收款，請聯繫客服協助確認", "不接受此會員類型；來源平台常數 MerType"),
    AGENT07010("AGENT07010", "AGENT07010", "系統設定有誤，請聯繫客服協助處理", "來源IP錯誤；平台出口 IP 未列白名單"),
    AGENT07011("AGENT07011", "AGENT07011", "商品交付設定有誤，請聯繫客服協助處理", "商品交付完成時間不可空白；來源平台常數 DeliverTime"),
    AGENT07012("AGENT07012", "AGENT07012", "商品交付設定有誤，請聯繫客服協助處理", "商品交付時間需為數字型態；來源平台常數 DeliverTime"),
    AGENT07013("AGENT07013", "AGENT07013", "商品交付設定有誤，請聯繫客服協助處理", "商品交付時間超過長度；來源平台常數 DeliverTime"),
    AGENT07014("AGENT07014", "AGENT07014", "商品交付設定有誤，請聯繫客服協助處理", "商品交付類型比例不可空白；來源平台常數 GoodsRate"),
    AGENT07015("AGENT07015", "AGENT07015", "履約保證設定有誤，請聯繫客服協助處理", "履約保證類型不可空白；來源平台常數 EscrowType"),
    AGENT07016("AGENT07016", "AGENT07016", "客單價設定有誤，請聯繫客服協助處理", "商品平均客單價不可空白；來源平台常數 MerUnitPrice"),
    AGENT07017("AGENT07017", "AGENT07017", "客單價設定有誤，請聯繫客服協助處理", "商品平均客單價需為數字型態；來源平台常數 MerUnitPrice"),
    AGENT07018("AGENT07018", "AGENT07018", "交易類型設定有誤，請聯繫客服協助處理", "請填入至少一種商店交易類型；平台送出參數"),
    AGENT07019("AGENT07019", "AGENT07019", "交易類型設定有誤，請聯繫客服協助處理", "商店交易類型設定錯誤；平台送出參數"),
    AGENT07020("AGENT07020", "AGENT07020", "手續費設定需由平台調整，請聯繫客服協助處理", "請填入至少一種手續費支付設定；平台送出參數"),
    AGENT07021("AGENT07021", "AGENT07021", "手續費設定需由平台調整，請聯繫客服協助處理", "手續費率設定錯誤；平台送出參數"),
    AGENT07022("AGENT07022", "AGENT07022", "商品交付設定有誤，請聯繫客服協助處理", "商品交付類型比例設定錯誤；來源平台常數 GoodsRate"),
    AGENT07023("AGENT07023", "AGENT07023", "查不到這家商店的金流資料，請聯繫客服協助確認", "查無商店代號"),
    AGENT07024("AGENT07024", "AGENT07024", "提領設定需由平台調整，請聯繫客服協助處理", "設定免費提領次數異常；平台送出參數"),
    AGENT07025("AGENT07025", "AGENT07025", "收款方式設定有誤，請聯繫客服協助處理", "支付關閉方式設定錯誤；平台送出參數"),
    AGENT07026("AGENT07026", "AGENT07026", "商店網址設定有誤，請聯繫客服協助處理", "商店網址或商店敘述請擇一填入；來源平台生成 MerURL"),
    AGENT07027("AGENT07027", "AGENT07027", "查不到這家商店的金流資料，請聯繫客服協助確認", "請勿查詢非屬於您的商店資料"),
    AGENT07028("AGENT07028", "AGENT07028", "客單價設定有誤，請聯繫客服協助處理", "商品平均客單價須大於0；來源平台常數 MerUnitPrice"),
    AGENT07030("AGENT07030", "AGENT07030", "收款方式設定有誤，請聯繫客服協助處理", "開關參數衝突；平台送出參數"),
    AGENT07031("AGENT07031", "AGENT07031", "收款方式設定有誤，請聯繫客服協助處理", "已申請開通的支付工具無法變更；平台送出參數"),
    AGENT08001("AGENT08001", "AGENT08001", "請填寫負責人姓名", "請輸入負責人姓名；來源 MCName"),
    AGENT08002("AGENT08002", "AGENT08002", "請填寫負責人身分證字號", "請輸入負責人身份證號碼；來源 MCID"),
    AGENT08004("AGENT08004", "AGENT08004", "負責人身分證字號格式有誤，只能是英文字母加數字，請確認後重新填寫", "格式錯誤僅允許A至Z與數字；來源 MCID"),
    AGENT08005("AGENT08005", "AGENT08005", "主要聯絡人的電子信箱格式有誤，請確認後重新填寫", "主聯絡人電子郵件格式錯誤；來源 MBCMail"),
    AGENT09001("AGENT09001", "AGENT09001", "請填寫主要聯絡人姓名", "請輸入主聯絡人姓名；來源 MBCName"),
    AGENT09002("AGENT09002", "AGENT09002", "請填寫主要聯絡人電話", "請輸入主聯絡人電話；來源 MBCTel"),
    AGENT09003("AGENT09003", "AGENT09003", "請填寫主要聯絡人電子信箱", "請輸入主聯絡人E-Mail；來源 MBCMail"),
    MER_1("MER_1", "MER_1", "收款功能已開通", "MerStatus=1 啟用；正常態不下發"),
    MER_2("MER_2", "MER_2", "這家商店的金流服務已關閉，請聯繫客服協助確認", "MerStatus=2 關閉"),
    MER_8("MER_8", "MER_8", "金流申請未通過審核，請確認資料後重新提交", "MerStatus=8 拒絕"),
    MER_9("MER_9", "MER_9", "金流申請審核中", "MerStatus=9 審核中；正常態不下發"),
    MER_91("MER_91", "MER_91", "金流審核需要補件，請依指示補齊資料後重新提交", "MerStatus=91 待補資料"),
    MER_92("MER_92", "MER_92", "金流申請審核未通過，請確認資料後重新提交", "MerStatus=92 審核失敗"),
    MER_99("MER_99", "MER_99", "這家商店的金流服務已停用，請聯繫客服協助確認", "MerStatus=99 停用或停權"),
    MEMBER_1("MEMBER_1", "MEMBER_1", "金流會員資料已通過", "MemberStatus=1 啟用；正常態不下發"),
    MEMBER_5("MEMBER_5", "MEMBER_5", "請查收 PayUni 寄送的通知信，並依指示至 PayUni 會員後台填寫會員資料", "MemberStatus=5 待補資料；沿用既有 payuniMemberPendingGuidance"),
    MEMBER_8("MEMBER_8", "MEMBER_8", "金流會員審核未通過，請確認個人或公司資料後重新提交", "MemberStatus=8 拒絕"),
    MEMBER_9("MEMBER_9", "MEMBER_9", "金流會員資料審核中", "MemberStatus=9 待審核；正常態不下發"),
    MEMBER_92("MEMBER_92", "MEMBER_92", "金流會員審核未通過，請確認個人或公司資料後重新提交", "MemberStatus=92 審核失敗"),
    MEMBER_99("MEMBER_99", "MEMBER_99", "金流會員帳號已停權，請聯繫客服協助確認", "MemberStatus=99 關閉或停權"),
    PAYUNI_UNKNOWN("PAYUNI_UNKNOWN", "PAYUNI_UNKNOWN", "申請未通過，請聯繫客服協助確認", "**非 PayUni 代碼**：PayUni 回了本目錄未登錄的代碼時的 fail-closed 文案。登在這裡是為了讓這句賣家可見的話與其他文案同一個 SSOT；程式仍保留一份同字串作為 CodeMeta 查不到時的最後防線");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PayuniSellerFacingError(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, PayuniSellerFacingError> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayuniSellerFacingError::code, e -> e));

    private static final Map<String, PayuniSellerFacingError> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayuniSellerFacingError::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PayuniSellerFacingError fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PayuniSellerFacingError fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
