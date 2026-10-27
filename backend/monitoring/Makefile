up:
	docker compose up -d
	@./scripts/init-tempo-datasource.sh

down:
	docker compose down

restart:
	docker compose restart

logs:
	docker compose logs -f

# 首次設定：從 .example 複製 target 檔案，再填入實際 IP
init-targets:
	@for f in prometheus/targets/*.json.example; do \
		target="$${f%.example}"; \
		if [ ! -f "$$target" ]; then \
			cp "$$f" "$$target"; \
			echo "Created $$target — 請填入實際 IP"; \
		else \
			echo "$$target already exists, skipped"; \
		fi \
	done
