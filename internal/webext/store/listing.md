# Chrome Web Store listing: Killer Chicken 9000

What the owner fills in on the Chrome Web Store developer dashboard
(ROADMAP 2.17). Package: `make chrome-store-zip` →
`kc9k-chrome-store-<version>.zip` (Manifest V3, no `key`, no
`update_url`). Pictures: this folder (`node internal/webext/store/render.js`
draws them again).

## Store listing tab

- **Name** (from the package): Killer Chicken 9000
- **Summary** (from the package, 132 characters at most): Enforces Killer
  Chicken 9000 blocks inside the browser.
- **Category**: Productivity → Workflow & Planning
- **Language**: English
- **Visibility** (Distribution tab): **Unlisted**. Killer Chicken 9000
  installs it on its own by Chrome's policy; nobody needs to find it.
- **Store icon**: `icon-128.png` from the package
  (`internal/webext/files/icon-128.png`).
- **Small promo tile** (440×280): `small-tile.png`.
- **Screenshots** (1280×800): `screenshot-1-blocked.png`,
  `screenshot-2-toolbar.png`, `screenshot-3-search.png`,
  `screenshot-4-dark.png`.
- **Homepage / support URL**: the website once it exists (ROADMAP 2.14);
  optional until then (the repository only if it is public).

**Description**:

> Killer Chicken 9000 is a program for your own computer that helps you
> stay away from what distracts you: websites, apps, words on pages, at the
> times you choose, with locks you can't undo on a whim.
>
> This extension is its part inside Chrome. It does nothing on its own:
> Killer Chicken 9000 installs it, and it carries out the blocks you set up
> in the program.
>
> - Pages of a blocked site go to a block page (or close, or show a
>   picture) while the block is on.
> - Pages with blocked phrases in their title or text are blocked or
>   changed, without showing what they hide first.
> - Look actions: greyscale, no images, no video, mute, dark.
> - Search results leading to blocked sites are taken off search pages.
> - Dark pages follow the circadian rhythm's dark mode.
> - The toolbar button shows how long until a site is blocked, and adds the
>   site to a block.
>
> Everything stays on your computer: the extension talks only to Killer
> Chicken 9000 on the same computer (127.0.0.1). No account, no tracking,
> no ads.
>
> You need Killer Chicken 9000 installed. Installed alone, the extension
> shows that the program isn't answering and blocks nothing.

## Privacy practices tab

**Single purpose**:

> Carries out, inside Chrome, the blocks the user set up in the Killer
> Chicken 9000 program on the same computer: it blocks or changes the
> pages those blocks name, and shows on its toolbar button how long until
> a site is blocked.

**Permission justifications**:

| Permission | Why |
|---|---|
| `webRequest` | Sees each page and frame request so a page of a blocked site is stopped before it loads, and pictures or videos are not loaded where a look action takes them away. |
| `webRequestBlocking` | Stopping a request needs it. Chrome gives it only to extensions installed by policy, which is how Killer Chicken 9000 installs this one. |
| `storage` | Keeps the last blocks received from the program, so they hold while the program restarts, and the service worker's state while Chrome restarts it. Stored only in the browser. |
| `tabs` | Reads each tab's address and title to judge it against the blocks; sends a blocked tab to the block page, closes or mutes tabs when a block says so; the toolbar button reads its own tab. |
| `idle` | Stops counting time on sites (allowances, statistics in the program) while the person is away from the computer. |
| `alarms` | Wakes the service worker should Chrome stop it, so blocks keep holding. |
| `downloads` | Cancels downloads from pages that a "no downloads" block names. |
| `scripting` | Inserts the look actions' style sheets (greyscale, no images, no video) into a tab's frames. |
| Host permission `<all_urls>` | The person decides which sites their blocks name, so any site can be blocked or changed; the page script reads a page's title and text for blocked phrases; the extension talks to the program at `http://127.0.0.1:47613`. |

**Remote code**: No, I am not using remote code. Every script is in the
package.

**Data usage**: tick none of the kinds of data. The extension reads the
addresses, titles and text of pages to judge them against the person's
blocks, and sends what it found (which block acted; the sites used and time
spent on them, for allowances and statistics; sites and words while the
person records them for a list) only to Killer Chicken 9000 on the same computer,
at `127.0.0.1`. Nothing is sent off the device, so nothing
is collected in the store's sense. Tick all three certifications:

- I do not sell or transfer user data to third parties, outside of the
  approved use cases.
- I do not use or transfer user data for purposes that are unrelated to my
  item's single purpose.
- I do not use or transfer user data to determine creditworthiness or for
  lending purposes.

To recheck when cross-device sync (ROADMAP 2.10) is built: sync is the
program's, not the extension's, but the privacy policy then names what the
program uploads.

**Privacy policy URL** (required, must be public): the website's copy of
`privacy-policy.md` once it exists (ROADMAP 2.14); until then any public
page with its text (a public repository's file, a gist, a Google Doc
shared to anyone with the link).

## For the reviewer (Distribution tab → notes, if asked)

> The extension is installed by the Killer Chicken 9000 desktop program
> through Chrome's ExtensionInstallForcelist policy, which is why it uses
> webRequestBlocking. Without the program it shows "Killer Chicken 9000
> isn't answering" on its toolbar button and blocks nothing. It only
> connects to 127.0.0.1:47613, the program on the same computer.
