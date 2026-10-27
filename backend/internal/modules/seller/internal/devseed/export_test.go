package devseed

// ClearChildrenSQL 讓外部測試套件（devseed_test）對真的資料庫執行清除語句。
var ClearChildrenSQL = clearChildrenSQL

// CheckForTest 讓外部測試套件對真的資料庫執行前提檢查（不經 Service 的依賴）。
var CheckForTest = check
