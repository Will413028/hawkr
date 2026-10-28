# Hawkr

English | [繁體中文](README.zh-TW.md)

An e-commerce platform serving buyers, sellers, and platform operators (no longer in operation). This repository merges the `dev` branches of five formerly separate repositories with their full commit history; credentials and keys have been removed from history or replaced with `REDACTED`.

| Directory | Original repository | Stack |
|---|---|---|
| `backend/` | backend-payuni | Go modular monolith, being migrated from the Go / Java microservices and REST gateway that remain alongside it |
| `website/` | hawkr-website | Next.js consumer website |
| `admin/` | hawkr-admin | Next.js admin |
| `buyer-app/` | hawkr_buyer | Flutter |
| `seller-app/` | hawkr_seller | Flutter |
| `api/` | — | Protobuf + Connect contract for the external API |
| `packages/` | — | Shared packages: generated API clients (TypeScript, Dart), auth, UI text, icons, address codes |
