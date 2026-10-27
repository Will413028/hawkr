# Components

`components/` is for cross-app React UI.

```txt
components/
  ui/       generic design-system primitives
  shared/   Hawkr shared product UI
```

`components/ui` should stay product-agnostic: buttons, cards, dialogs, inputs,
tabs, uploaders, and other primitives.

`components/shared` can carry Hawkr product meaning and be reused across
features: nav, footer, product cards, brand/logo helpers, and
workspace shells.

Feature-only UI belongs in `features/<domain>/components`, not here.
