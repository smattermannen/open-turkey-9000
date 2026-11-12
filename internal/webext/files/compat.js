"use strict";

// Chrome has the same extension APIs as Firefox under the name chrome (with
// promises in Manifest V3); Firefox has both. The scripts use browser.
if (typeof globalThis.browser === "undefined") {
  globalThis.browser = chrome;
}
