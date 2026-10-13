# Exodialabs brand tokens

The official website's default branch establishes the palette. Source snapshot:
repository `ExodiaLabs/exodialabs-website`, branch `develop`, commit
`c65f6dcd91bf7b94634f9ca289b4e8af1e37335d` (checked 2026-10-13).

Its `docs/design-system.md` documents the Figma “00 — Design System” frame and
states that `assets/styles/settings/tokens.css` mirrors primitive and semantic
variables. We consulted these files through authenticated read-only GitHub
access after the public site returned HTTP 403. These provenance links may
require repository access:

- [Design system source](https://github.com/ExodiaLabs/exodialabs-website/blob/c65f6dcd91bf7b94634f9ca289b4e8af1e37335d/docs/design-system.md)
- [CSS token source](https://github.com/ExodiaLabs/exodialabs-website/blob/c65f6dcd91bf7b94634f9ca289b4e8af1e37335d/assets/styles/settings/tokens.css)

| Official primitive | Exact value | TUI use |
| --- | --- | --- |
| Violet | `#7960a6` | Light-theme identity/accent |
| Lavender | `#bba9dc` | Dark-theme identity/accent |
| White | `#f5f2f9` | Dark-theme foreground, light-theme background |
| Prune | `#15111e` | Dark-theme background, light-theme foreground |
| Grey | `#8c8697` | Dark-theme secondary text |
| Anthracite | `#1d1927`, `#27222f`, `#38323f` | Neutral surfaces and light-theme secondary text |
| Shadow facet | `#543680` | Additional established accent token |

State colors use terminal ANSI roles with explicit text. They introduce no new
Exodialabs HEX primitives. Terminals with fewer colors approximate the palette.
ASCII and `NO_COLOR` preserve navigation and state labels. Automatic theme
selection uses the terminal's background response; if it supplies none, the
dark theme remains the default. The terminal controls fonts.
