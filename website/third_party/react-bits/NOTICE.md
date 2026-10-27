# React Bits interaction references

The seller pricing page and buyer product-certification cards use independently
adapted versions of interaction patterns demonstrated by React Bits'
`AnimatedContent` and `SpotlightCard` components. The implementations are scoped
to their Hawkr surfaces, use the project's own design tokens, and do not
redistribute the upstream components as a library.

- Upstream project: <https://github.com/DavidHDev/react-bits>
- Seller pricing reviewed revision: `4e0e030193b563be6be33d928f77d0d01cefe237`
- Product certification reviewed revision: `1ba7ecd4e570d20392fb62c923efb00f5b440642`
- Copyright: Copyright (c) 2026 David Haz
- License: MIT + Commons Clause License Condition v1.0

The upstream license text is preserved in [LICENSE.md](./LICENSE.md).

## Grainient background

The Hawkr shared Grainient renderer (`lib/animation/grainient-renderer.ts`) is adapted from React Bits'
`Grainient` TypeScript/CSS implementation. It preserves the upstream shader
design while replacing the OGL integration with a project-local native WebGL2
renderer. The upstream shader's reverse-edge `smoothstep` was expressed as an
equivalent inverse ascending-edge operation because reverse edges are undefined
by GLSL ES 3.00.

The shared adaptation blends four brand colors, adds a dark-mode brightness
uniform, and caps resolution and frame rate. Its deferred persistent host lives
outside route content; reduced-motion and unsupported devices use a CSS fallback.

- Source: <https://github.com/DavidHDev/react-bits/blob/0e69e737242df1d257b4e5e399b01ae1d7901375/src/ts-default/Backgrounds/Grainient/Grainient.tsx>
- Reviewed revision: `0e69e737242df1d257b4e5e399b01ae1d7901375`
- Source blob: `99a38ba1df516a3f958d3e72ac9ab49258005f16`
- Copyright: Copyright (c) 2026 David Haz
- License: MIT + Commons Clause License Condition v1.0
