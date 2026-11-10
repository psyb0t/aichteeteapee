# Documentation

Prefix: DOC.

- **DOC2:** `docs/` is the GitHub Pages site (Jekyll 3 with the just-the-docs theme, `docs/_config.yml`). Keep `docs/` flat: a file name is its URL.
- **DOC5:** Wrap any code block that contains `{{` or `{%` in `<!-- {% raw %} -->` and `<!-- {% endraw %} -->`, or Liquid eats it.
- **DOC6:** Prove a docs change with `make docs` (GitHub's own `jekyll-build-pages` image, output in `.testing/docs-site`) and a link check of that output. A clean build alone proves nothing about links.
