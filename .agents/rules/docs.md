# Documentation

Prefix: DOC.

- **DOC2:** Keep authored guides in `docs/`. Docusaurus reads them directly. Add new pages to `website/sidebars.mjs` and preserve published routes and anchors.
- **DOC5:** Use standard Markdown and fenced code blocks in the guides.
- **DOC6:** Run `make docs` and check the result under the `/aichteeteapee/` base path. GitHub Pages deploys `website/build` through `.github/workflows/docs.yml`.
