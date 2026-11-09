// The configuration of the app, which `mygo dev` and `mygo build` evaluate with
// Bun, of the command running. The development app (`mygo dev`) opens links of a
// scheme of its own, for the installed app to keep mygo-music:// links.
const preview = process.env.MYGO_PREVIEW === "1";

export default ({ command }: { command: string }) => ({
  name: preview ? "MyGo Music Preview" : "MyGo Music",
  identifier: preview ? "dev.maxbeau.music.preview" : "dev.egoist.music",
  slug: preview ? "mygo-music-preview" : "mygo-music",
  version: "0.1.23",
  out: "build",
  urlSchemes: [preview ? "mygo-music-preview" : command === "dev" ? "mygo-music-dev" : "mygo-music"],
  updates: preview ? undefined : {
    publicKey: "WzJsOGNIuf6mcEqo5ff8jub+NoQQOEk4JXreLPYjgyQ=",
    github: "egoist/mygo-music",
  },
  macos: {
    icon: "icon/AppIcon.icon",
    minimumSystemVersion: "13.0",
    signingIdentity: "Developer ID Application: xinyu zeng (GJE9R5VE87)",
    notarize: { keychainProfile: "notary" },
  },
  linux: {
    comment: "A music player for your files and more",
    categories: ["AudioVideo", "Audio", "Player"],
    webview: false,
  },
});
