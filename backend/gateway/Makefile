.PHONY: swagger swagger-install build run fix-fieldalignment lint-fieldalignment

# 安装 swag 命令行工具
swagger-install:
	@echo "Installing swag..."
	go install github.com/swaggo/swag/cmd/swag@latest

# 生成 Swagger 文档
# 輸出 docs/ 已 gitignore（終端產物，唯一消費者是 Apidog；見 10 檔 §16）。
# 本地想查某端點的 wire 形狀就跑這個，再 grep docs/swagger.yaml，不會污染 diff。
#
# --outputTypes json,yaml：刻意不產 docs.go。那個 .go 只有在專案要嵌 Swagger UI 時
# 才有用，本 Gateway 沒有註冊任何 Swagger UI 路由（routes_infra.go），沒人 import 它。
# 產出來反而有害：docs.go 會 import swaggo/swag，而 docs/ 不入版控後 go.mod 裡沒有
# 這個依賴（go mod tidy 會拿掉），於是「本地跑過 make swagger」的人接著跑
# `go build ./...` / `go test ./...` 就會炸在一個 gitignore 的生成檔上。
swagger:
	@echo "Generating Swagger docs..."
	cd cmd && swag init -g main.go --output ../docs --outputTypes json,yaml --parseDependency --parseInternal --dir ./,../internal/handler
	@echo "Swagger docs generated at docs/ (swagger.json + swagger.yaml)"

# 构建项目
build:
	@echo "Building gateway..."
	go build -o bin/gateway cmd/main.go

# 运行服务
run:
	@echo "Running gateway..."
	go run cmd/main.go

# 清理
clean:
	rm -rf bin/ docs/

# 格式化代码
fmt:
	go fmt ./...

# 运行测试
test:
	go test -v ./...

# 自動修正 struct field alignment（pre-commit lint 的 govet fieldalignment 常見錯誤）
# 用法：make fix-fieldalignment                  # 全包
#       DIRS=./internal/handler/ make fix-fieldalignment  # 指定目錄
fix-fieldalignment:
	@echo "Fixing field alignment in $(or $(DIRS),./...)..."
	go run golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest -fix $(or $(DIRS),./...)

# 僅檢查不修正（CI 用）
lint-fieldalignment:
	go run golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest $(or $(DIRS),./...)
