# Documentation

Prefix: DOC.

- **DOC1:** A change to behaviour, an option, a default, an env var or an error code updates the page under `docs/` that describes it in the same change, plus `CHANGELOG.md` when it ships.
- **DOC2:** `docs/` is the GitHub Pages site (Jekyll 3 with the just-the-docs theme, `docs/_config.yml`). Every page has front matter with a unique `title`, a hand-written `description` of 50 to 170 characters and a `nav_order`. One `#` heading per page. Keep `docs/` flat: a file name is its URL.
- **DOC3:** Page-to-page links use the relative `.md` path (`middleware.md#cors`). A link to a file outside `docs/` uses the full `https://github.com/psyb0t/aichteeteapee/blob/main/<path>` URL, or `tree/main/<path>` for a directory, because a `../` link is dead on the site.
- **DOC4:** `README.md` is the front door: the pitch, one example, short summaries and links. Reference material lives in `docs/`. README links to the site use full `https://psyb0t.github.io/aichteeteapee/...` URLs and links to repository files use full GitHub URLs, so they survive mirrors and pkg.go.dev.
- **DOC5:** Wrap any code block that contains `{{` or `{%` in `<!-- {% raw %} -->` and `<!-- {% endraw %} -->`, or Liquid eats it.
- **DOC6:** Prove a docs change with `make docs` (GitHub's own `jekyll-build-pages` image, output in `.testing/docs-site`) and a link check of that output. A clean build alone proves nothing about links.
- **DOC7:** Keep the README's voice: short, blunt, the occasional swear, the pronunciation joke for each package. No em dashes, "directory" never "folder", no filler vocabulary.
