# Styles

`styles/` is the global CSS and migration area.

```txt
styles/
  globals.css     Tailwind entry and CSS import manifest
  foundation/     design tokens, theme bridge, reset/layout foundations
  components/     legacy/shared component CSS during migration
  features/       legacy feature CSS during migration
  legacy/         preserved historical CSS
```

New UI should prefer Tailwind utilities, `components/ui`, and design tokens.
Avoid adding new large feature CSS files unless the selector family is being
migrated deliberately.
