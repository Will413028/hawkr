package migrate

// TestModule 給外部測試套件（migrate_test）用；integration 測試要 import testdb，而 testdb import 本套件。
var TestModule = testModule
