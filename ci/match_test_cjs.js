// Runs ci/match_test.js without Node, on Cinnamon's own JavaScript (cjs):
//   cjs ci/match_test_cjs.js
// It stands in for the few Node things the test uses.
const GLib = imports.gi.GLib;
const here = GLib.path_get_dirname(imports.system.programInvocationName);
const read = (p) => imports.byteArray.toString(GLib.file_get_contents(GLib.build_filenamev([here, p]))[1]);
globalThis.process = { exitCode: 0, exit: (c) => { process.exitCode = c; } };
globalThis.console = { log: print, error: printerr };
globalThis.atob = (s) => imports.byteArray.toString(GLib.base64_decode(s));
globalThis.Buffer = { from: (s) => ({ toString: () => GLib.base64_encode(imports.byteArray.fromString(s)) }) };
// A small URL: enough for match.js (hostname, pathname, search, searchParams).
if (typeof globalThis.URL === "undefined") {
  globalThis.URL = function (text) {
    const m = /^([a-z][a-z0-9+.-]*):\/\/([^/?#]*)([^?#]*)(\?[^#]*)?/i.exec(text);
    if (!m) {
      throw new TypeError("Invalid URL");
    }
    this.hostname = m[2].replace(/^[^@]*@/, "").replace(/:\d+$/, "").toLowerCase();
    this.pathname = m[3] || "/";
    this.search = m[4] || "";
    const pairs = this.search.slice(1).split("&").filter(Boolean).map((pair) => {
      const i = pair.indexOf("=");
      const decode = (v) => decodeURIComponent(v.replace(/\+/g, " "));
      return i < 0 ? [decode(pair), ""] : [decode(pair.slice(0, i)), decode(pair.slice(i + 1))];
    });
    this.searchParams = { [Symbol.iterator]: () => pairs[Symbol.iterator]() };
  };
}
const test = read("match_test.js").split("\n").filter((line) => !/require\(|vm\.runInThisContext/.test(line)).join("\n");
(0, eval)(read("../internal/webext/files/match.js") + "\n" + test);
if (process.exitCode) {
  imports.system.exit(process.exitCode);
}
print("match tests passed");
