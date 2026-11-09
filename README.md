# MyGo Music

A native desktop music player written in Go with [MyGo](https://github.com/egoist/mygo), no WebView. It plays your local files out of the box, and everything else comes from plugins.

## Features

- Local files (MP3, FLAC, Ogg Vorbis, WAV, and ALAC, FLAC or AAC in .m4a and .aac) with tags, covers and `.lrc` lyrics
- Playlists, liked songs, search, albums and artists
- Now Playing with lyric stage styles, cover wall, queue, shuffle and repeat
- Notch player on macOS, tray, media keys
- Audio cache, fades on pause and seek, loudness normalization
- Session restore
- Podcast captions, written down on the device as episodes play
- English and Simplified Chinese
- Plugins can add accounts, catalogs, podcasts, stations, comments and pages of their own

## Build

```bash
go tool mygo dev     # run in development
go tool mygo build   # package into build/
go test ./internal/...
```

## Podcast captions

Episodes are written down on the device as they play, with [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) and Qwen3-ASR, which the app downloads when the listener asks for them. The libraries come from this repository's `sherpa-onnx-<version>` prerelease, which only changes with that version:

```bash
go run ./scripts/sherpa -sign "Developer ID Application: …"
gh release create sherpa-onnx-2.0.1 --prerelease --latest=false build/sherpa/*
```

## Plugins

A plugin is a folder in the app's `plugins` directory (Settings > Plugins > + > Show Plugins Folder) with a `plugin.json` and one JavaScript or TypeScript file. Install one with the + in Settings > Plugins from GitHub, by its repository or its address, or from its folder or a zip of it on this computer, or drop the folder in by hand. Remove one from its ⋯ menu there. There's nothing to build, and the same plugin runs on every platform. [`examples/demo-plugin`](examples/demo-plugin) does everything a plugin can with made-up data: copy it to start.

```json
{
  "id": "example",
  "name": "Example",
  "version": "1.0.0",
  "minAppVersion": "0.1.8",
  "main": "index.js",
  "playback": "stream",
  "permissions": { "network": ["api.example.com"] },
  "settings": [{ "key": "apiKey", "label": "API Key", "help": "From your account page", "link": "https://example.com/keys" }],
  "navigation": [
    { "builtin": "home" },
    { "title": "Charts", "page": "charts", "icon": "chart" },
    { "title": "Stations", "builtin": "stations" }
  ]
}
```

```js
export async function search({ query, kind, offset }) {
  const res = await fetch(`https://api.example.com/search?q=${encodeURIComponent(query)}&type=${kind}&offset=${offset}`, {
    headers: { authorization: `Bearer ${host.setting("apiKey")}` },
  });
  return { tracks: (await res.json()).items.map(toTrack), more: false };
}

export function stream({ id, quality }) {
  return { url: `https://api.example.com/audio/${id}.mp3`, type: "mp3" };
}
```

Export a function for each thing the plugin does. Each takes one object of parameters and returns a value or a promise. What it doesn't export, the app leaves out of its pages.

| Export | Parameters | Returns |
| --- | --- | --- |
| `signIn`, `signOut`, `account` | | `{id, name, imageUrl, page}`; `account` returns `null` when signed out. `page` is a `browse` page of the listener's own, which the account menu opens as My Page |
| `setSetting` | `{key, value}` | |
| `home` | | `Browse` |
| `browse` | `{page}` | `Browse` |
| `search` | `{query, kind, offset}`, where `kind` is `tracks`, `albums`, `artists`, `playlists` or `shows` | `{tracks or albums or ..., more}` |
| `suggest` | `{query}` | `[string]` |
| `recent` | | `[Track]` |
| `played` | `{id}` | |
| `liked` | `{offset}` | `{tracks, more}` |
| `isLiked` | `{ids}` | `{id: bool}` |
| `like`, `likeEpisode` | `{id, on}` | `like` likes a song, and `likeEpisode` an episode: without it, episodes show no heart |
| `album` | `{id}` | `{album, tracks}` |
| `artist` | `{id}` | `{artist, tracks, albums}` |
| `saveAlbum`, `followArtist`, `savePlaylist`, `subscribeShow` | `{id, on}` | |
| `savedAlbums` | `{offset}` | `{albums, more}` |
| `playlists` | | `[Playlist]` |
| `playlist` | `{id}` | `Playlist` with `tracks` |
| `createPlaylist` | `{name}` | `Playlist` |
| `renamePlaylist` | `{id, name}` | |
| `deletePlaylist` | `{id}` | |
| `addToPlaylist`, `removeFromPlaylist` | `{id, trackIds}` | |
| `movePlaylistTrack` | `{id, from, to}` | |
| `shows` | | `[Show]` |
| `show` | `{id, offset}` | `{show, episodes, more}` |
| `stations` | | `[Station]` |
| `stationNext` | `{id}` | `[Track]` |
| `radio` (optional) | `{kind, id}`, where `kind` is `track` or `artist` | `Station` with a source-defined session ID |
| `related` (optional) | `{id}` | `Browse` with related music |
| `comments` | `{trackId, offset}` | `{comments, total, more}` |
| `likeComment` | `{trackId, commentId, on}` | |
| `postComment` | `{trackId, text}` | `Comment` |
| `lyrics` | `{id, track?}` | `{lrc, translation}`, `{lines}` with timed `words`, `{text}` for unsynced lyrics, or `{instrumental: true}` |
| `stream` | `{id, quality}` | `{url, type, headers, size, gain, peak}`, where `type` is `mp3`, `flac`, `ogg`, `wav`, `m4a` (ALAC, FLAC or AAC-LC, whole or in fragments as a DASH stream serves it) or `aac` (AAC-LC in ADTS) |
| `remotePlay` | `{ids, start, position}` | |
| `remotePause`, `remoteResume` | | |
| `remoteSeek` | `{position}` | |
| `remoteVolume` | `{percent}` | |
| `remoteState` | | `{playing, trackId, position, duration, volume, device}` or `null` |
| `remoteDevices` | | `[{id, name, active}]` |
| `remoteUse` | `{deviceId}` | |

The shapes are in [`internal/music/music.go`](internal/music/music.go) by their JSON names. Times are in milliseconds. A `Browse` is `{title, subtitle, imageUrl, description, sections}`: with an `imageUrl`, such as a listener's avatar, it's headed round, as an artist's page is, and `description` goes under the title a line to each line. A section can hold `tracks`, `albums`, `artists`, `playlists`, `shows`, `stations`, `links` (`{title, subtitle, imageUrl, page}`), a `morePage`, and `empty`, what it says while it holds nothing. A source's first section of `tracks` leads its Home as a card of the day's songs, which opens a page of all of them, headed by the section's `pageTitle` if it has one, else its `title`, and its `subtitle` after the count of songs. A show's `publisherPage` is a `browse` page of who publishes it, which their name on the show's page opens.

An `imageUrl` can say where its size goes, with `{size}`, for an image server that makes pictures the size asked: `https://img.example.com/cover.jpg?param={size}y{size}` or `https://img.example.com/{size}/cover.jpg`. The app fills in how many pixels across it shows the picture at, 40 in a row and 600 in Now Playing, or 600 where nothing asks for a size. `%7Bsize%7D`, as JavaScript's `URL` writes it in a query, counts too. An app older than 0.1.20 fetches `{size}` as it is, so a plugin that gives it wants a `minAppVersion` of 0.1.20. Without it, give pictures at about 800 pixels if the service can, rather than as large as they were made: the app fetches and decodes the whole of each, a few megabytes for one thousands of pixels across, to show it in a row.

Of the `playlists`, those with `mine` are the listener's own, listed under My Playlists, and the rest are ones they saved. One that is `mine` and also `fixed`, such as the songs they liked, is kept by the source itself: the app doesn't offer to rename it, delete it, add songs to it, take them out or move them.

`navigation` is the plugin's sidebar, in its order. An entry opens a `browse` page of its own by `page`, or one of the app's by `builtin`: `home`, `liked`, `savedAlbums`, `stations` or `shows`, which can be given a `title`. Without `navigation`, the sidebar has an entry for each of the app's pages the plugin can do. Up to four stations show as a switch over one card, and more as a grid.

`minAppVersion` is the oldest version of MyGo Music the plugin works with. An older one won't install it, and says to update first. Versions before 0.1.8 don't read it.

### Translating a plugin

Put the translations in `locales/<language>.json` next to `plugin.json`: `locales/zh-CN.json` for Simplified Chinese, the language the app speaks besides English. Each one maps the English text to the translation:

```json
{ "Example": "示例", "API Key": "API 密钥", "From your account page": "在你的账号页面获取", "Charts": "排行榜" }
```

The app translates the plugin's `name`, its settings' `label` and `help` and its `navigation` titles with them. For what the plugin says itself, such as a page's titles or an error's message, `host.t(text)` answers the translation, or `text` when there's none, and `host.language()` the listener's language, `en` or `zh-CN`. Both follow the listener as they change it, and the page they're on loads again.

With `"playback": "stream"`, the app plays what `stream` answers. With `"playback": "remote"`, the plugin makes the sound itself, or has another program do so, and the app calls the `remote` functions and polls `remoteState`.

Throw an error with a `code` to say what's wrong: `signIn` prompts the listener to sign in, `setup` sends them to the plugin's settings, `unavailable` means this can't be played, and `unsupported` means the plugin doesn't do this. Any other error shows its message.

### Publishing a plugin

Put it in a public GitHub repository, with `plugin.json` at the top. The app takes the latest release: a `.zip` asset named for the platform, such as `plugin-darwin-arm64.zip`, or one named for none, or else the source. Without a release it takes the latest commit of the default branch. A new tag or commit is a new version, which shows as an update in the settings. An update asking for more permissions asks the listener again.

Add the topic `mygo-music-plugin` to the repository to have it listed on the [plugins page](https://mygo-music.egoist.dev/plugins).

Link to `mygo-music://install-plugin?repo=owner/repo` for an install button: it opens the app, which downloads the plugin and asks the listener before installing it. The development app (`mygo dev`) answers `mygo-music-dev://` instead.

### What a plugin can reach

A plugin has the JavaScript standard library, timers, `fetch` and its relatives, `URL`, `console` and `host`:

| | |
| --- | --- |
| `host.platform`, `host.arch` | such as `darwin`, `arm64` |
| `host.dir`, `host.dataDir` | the plugin's folder, and one for its data |
| `host.storage.get(key)`, `.set(key, value)`, `.remove(key)` | JSON values that outlive the app |
| `host.setting(key)` | what the listener filled in |
| `host.t(text)`, `host.language()` | the text in the listener's language, from the plugin's `locales`, and that language: `en` or `zh-CN` |
| `host.removeDataDir(name)` | removes a folder inside `host.dataDir` |
| `host.crypto.md5(text)` | its hash, in hex |
| `host.crypto.aesEcbEncrypt(key, text)` | AES-ECB with PKCS#7 padding, in hex, for services whose requests are made that way |
| `host.openURL(url)` | opens a web page in the listener's browser |
| `host.signInPrompt({qr, text})` | what the sign-in window shows while `signIn` goes on: a QR code to scan, and what to do |
| `host.waitForRedirect({port, path, timeout, page})` | the end of a browser sign-in: listens on `127.0.0.1` and answers the query it's redirected back with. Needs the network permission |
| `host.webSignIn({url, cookiesOf, until, userAgent})` | a sign-in on the service's own page, for one that has no other way in: the app shows `url` in a window of its own, apart from every other page and forgotten after, and once the cookies named in `until` are all there, closes it and answers `{cookie}`, the Cookie header a request to `cookiesOf` would send. `cookiesOf` has to be a host the network permission reaches. `userAgent` is what the page is told it's shown in |
| `host.spawn(command, args)` | starts a program, by a path from the plugin's folder or absolute, or a name on `PATH`. Needs the process permission to name it. Returns `{pid, write(text), closeStdin(), readLine(), wait(), kill(), errorOutput()}` |

Everything else takes a permission in `plugin.json`:

- `"network": ["api.example.com", "*.example.com"]` lists the hosts `fetch` can reach, or `["*"]` for any. Without it, `fetch` reaches nothing.
- `"requestHeaders"` are headers the app sends with what it fetches itself for the plugin, its pictures (`image`) and its songs' audio (`audio`), for a service that keeps those behind its session as well as its API:

  ```json
  "requestHeaders": [
    { "hosts": ["{setting:server}"], "kinds": ["image", "audio"], "headers": { "cookie": "music-token={storage:token}" } },
    { "hosts": ["img.example.com"], "kinds": ["image"], "headers": { "referer": "https://example.com/" } }
  ]
  ```

  `hosts` are hosts or `*.domain`, which the network permission has to reach, or `{setting:key}`, the host of the server a setting names, such as `https://nas.local:5666`, while the network permission reaches it. `*` isn't one. Without `kinds`, they go with both. In `headers`, `{setting:key}` is what the listener filled in and `{storage:key}` what the plugin keeps with `host.storage`, as each is when the request goes out: a token the plugin stores anew goes with the next one, and none is kept in a URL. A header whose value isn't there yet, as before signing in, is left out. The headers a `stream` answers come first. Neither goes on to another host a request is redirected to.
- `"process": ["bin/helper-*", "ffmpeg"]` lists the programs `host.spawn` can start, for what JavaScript can't do: a path from the plugin's folder or absolute, or a name looked up on `PATH`. A path only matches a path and a name only a name, and `*` matches within one folder, as in a glob. `["*"]`, or `true`, asks for any program, which the listener is warned about. The listener has to allow it in Settings first, as a program can do anything they can. Programs end when the plugin does, and once allowed, `fetch` can reach this machine too, to talk to them. Ship one binary per platform and pick it with `host.platform` and `host.arch`.

## Credits

- [folia-major](https://github.com/chthollyphile/folia-major): the lyric stage animations

## License

AGPL-3.0. See [LICENSE](LICENSE).

## Seeded radio and discovery

Lyrics providers may accept optional `track` metadata alongside the ID.
Unsynced `text` is displayed as scrollable text without seeking or fabricated
timestamps. Only an explicit `instrumental: true` result is called instrumental;
missing lyrics and request failures have separate states. Retrying a failed
lookup leaves playback untouched and ignores older track or retry responses.

Sources may implement the optional `radio` and `related` exports. A source with both `radio` and `stationNext` gets a **Start Song Radio** menu action and a **Start Artist Radio** button; `related` gets a **Related Music** menu action. These are optional Go interfaces, so existing sources keep their `Source` implementation.

Song radio retains the current song's audio, position and paused state. When
another song is selected, that song starts first while recommendations load.
The first successful batch replaces only the queue after the seed; history is
retained in play order. A natural completion during preparation waits for the
batch. New playback, leaving radio or another radio request invalidates old
responses. The source's recommendation batches should omit the seed; the host
also removes it from the initial batch. Artist and ordinary station playback
continue to start from their returned queue.

A `Station` may set `public: true` to allow anonymous starts; by default the previous sign-in requirement applies. The source remains responsible for returning `signIn` when the service needs it. Station IDs must survive a session restore or carry enough information to reconstruct their endpoint.

`artist` may additionally return `sections: Section[]`. When present, the artist page preserves the source's grouped discography and `morePage` links. All tracks in those sections are stamped with their source as on other browse pages. `Track.playlistItemId` optionally identifies a duplicate song's occurrence for reordering.

**Settings > Playback > Autoplay Related Music** continues a naturally completed queue with a seeded radio when enabled. It is off by default, gives repeat modes priority, and excludes podcast episodes. Manual Next retains its existing behavior. A later playback request or seek invalidates an old recommendation response; pausing while it loads stays paused.

For a separate local preview app with its own identifier and no configured release feed:

```bash
MYGO_PREVIEW=1 go tool mygo build -sign - -skip-notarize -skip-dmg
```

To test the companion YouTube plugin in the real runtime, without network access:

```bash
YOUTUBE_PLUGIN_DIR=/absolute/path/to/youtube-plugin go test ./internal/plugin -run TestYouTubePluginRuntime -v
```

For a local installation that uses the normal app name, identifier and data,
build with `MYGO_LOCAL_BUILD=1` instead of `MYGO_PREVIEW=1`. Both development
build modes omit the upstream release feed, so an upstream update cannot
replace local feature work.

## Sidebar and library

Navigation items may set `group: "explore"` to become tabs in a single Explore
entry, or `group: "library"` to join personal-library navigation. Existing
custom pages still appear as discovery entries. Sources with `playlists` get
one playlist library with All, Created by Me and Saved by Me filters, local
search and name sorting. Those operations do not reorder the source's data.

Sidebar pins are stored locally per source in `settings.json`. The first
successful nonempty playlist load seeds at most six shortcuts for migration.
An explicitly empty set remains empty. Playlist, album and artist pages and
cards offer Pin/Unpin; shortcuts can be dragged, or moved with their context
menu. Pin order survives restart and never modifies a cloud playlist.

The sidebar search names its active source and supports both Cmd+K and Cmd+F.

## Discovery homepage (0.1.25)

A source's `home` may set `layout: "discovery"` for compact pinned shortcuts,
up to eight quick picks, a seeded Surprise Me radio, mix recommendations and
an optional related shelf from the current song. Other sources keep their
existing homepage. Each recommendation shelf shows one card row; See All
retains the complete cached collection.

Sections may return ordered `entries`, each containing one of `track`,
`album`, `artist`, `playlist`, `station` or `link`. Mixed shelves preserve that
order instead of splitting content into separate grids by type. Typed lists
remain available for older hosts and track actions. Track sources are stamped
in both representations, including songs nested inside playlist entries.
