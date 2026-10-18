# 22. Website: Next.js or TanStack Start for devagotchi.com

[Index](../devagotchi-spec.md) · The public site (landing, docs, comparisons such as "vs Desktop Commander",
changelog, blog, downloads) must rank well. The hub app stays the existing Vite SPA in `web/`. Sources read
2026-10-18.

## Facts

| Topic | Next.js 16 (App Router) | TanStack Start 1.0 |
|---|---|---|
| Maturity | Major version 16; Partial Prerendering replaced by `cacheComponents` ([upgrade to 16](https://nextjs.org/docs/app/guides/upgrading/version-16), [cacheComponents](https://nextjs.org/docs/app/api-reference/config/next-config-js/cacheComponents)) | **1.0 stable on 2026-10-13**, five days before this research ([blog](https://tanstack.com/blog/tanstack-start-1-0)) |
| Rendering | Static shell + streamed dynamic parts with Cache Components ([migrating](https://nextjs.org/docs/app/guides/migrating-to-cache-components)) | SSR by default; per route `ssr: false` or `'data-only'` ([selective SSR](https://tanstack.com/start/latest/docs/framework/react/guide/selective-ssr)); build-time prerender with `crawlLinks` ([prerendering](https://tanstack.com/start/latest/docs/framework/react/guide/static-prerendering)) |
| Metadata | `metadata` / `generateMetadata`; metadata can stream, but HTML-limited bots get blocking metadata ([generateMetadata](https://nextjs.org/docs/app/api-reference/functions/generate-metadata)) | Route `head()` for title, meta, OG, canonical links, JSON-LD scripts from loader data ([SEO guide](https://tanstack.com/start/latest/docs/framework/react/guide/seo)) |
| Sitemap / robots / OG images | File conventions `sitemap.ts`, `robots.ts`, `opengraph-image` ([sitemap](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/sitemap), [robots](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/robots), [OG](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/opengraph-image)) | Built-in sitemap when prerendering crawls links; robots and dynamic sitemaps as server routes ([SEO guide](https://tanstack.com/start/latest/docs/framework/react/guide/seo)); no OG image generator found (**unverified** absence) |
| Server Components | Central to the model | Optional; the TanStack team removed RSC from tanstack.com ([blog](https://tanstack.com/blog/tanstack-start-1-0)) |
| Build / hosting | Turbopack | Vite (or Rsbuild) + Nitro; Netlify, Cloudflare, Vercel, Render, Railway targets named ([blog](https://tanstack.com/blog/tanstack-start-1-0)) |
| React Aria | Official Next.js App Router setup: `I18nProvider` in a `"use client"` provider, `lang`/`dir` on `<html>`, Turbopack locale plugin (Next ≥15.4), `RouterProvider` with `useRouter().push` ([frameworks](https://react-aria.adobe.com/frameworks)) | Not on React Aria's framework list; the **Vite** setup applies to the bundle (**inference**); SSR locale injection would follow the React Router pattern (`getLocalizationScript`) by hand |

Both deliver server-rendered or prerendered HTML, per-page metadata, canonical links, JSON-LD, sitemaps and robots,
which is what technical SEO needs.

## Recommendation: Next.js 16, mostly static (inference)

1. **Lowest SEO risk today.** Metadata, sitemap, robots and OG-image generation are file conventions with years of
   production use; TanStack Start's stable API is days old.
2. **React Aria has a documented Next.js path**, including SSR locale matching, so RAC overlays (Tooltip, Popover,
   Modal) hydrate without mismatches. For TanStack Start we would be first to write that glue.
3. **The site is content, not an app.** Marketing, docs and comparison pages are static; Cache Components give a
   static shell with small dynamic parts (latest version, download links) without a separate API.
4. **Keep RSC thin.** RAC components are client components; pages stay server-rendered layouts with client islands
   for interactive parts. Data the site needs (versions, changelog) comes from the signed release index
   ([19](19-public-product-spec.md)) at build time.
5. **Watch rules.** Put title/description/canonical in static `metadata` where possible so no bot depends on
   streamed metadata; test with Google's Rich Results Test and a no-JS fetch in CI.

**When to switch to TanStack Start:** if the site grows into an app that shares many components and routes with
`web/` (Vite) or if we want to avoid RSC entirely. Revisit after Start has had a few minor releases.

## Site rules
- Pages and copy are data (MDX in the repo, edited by Jesse); no hardcoded download URLs (read from the index).
- Every interactive widget is React Aria Components with our CSS tokens; no shadcn, no Radix.
- Accessibility: one `h1` per page, skip link, visible focus, WCAG 2.2 AA colour contrast, reduced-motion support.
- Comparison pages state facts with links (benchmarks from [17](17-standalone-release.md), lessons from
  [25](25-dc-lessons.md)); no claims about DC we cannot source.
- Use DC's name only factually; do not use any vendor's logo or name in our branding (Anthropic forbids this for
  Claude Code, [legal](https://code.claude.com/docs/en/legal-and-compliance)).
