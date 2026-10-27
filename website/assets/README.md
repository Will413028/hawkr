# Assets

`assets/` stores module assets that are imported by TypeScript/TSX and bundled
with the app.

Current source of truth:

```txt
assets/fonts/
```

Rules:

- Do not put URL-served images here. Use `public/images`.
- UI icons are not assets: use `AppIcon` (`components/shared/AppIcon.tsx`).
