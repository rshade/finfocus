const { Box } = require("./lib/box");
const { Pair } = require("./lib/pair");
const { VendorBox } = require("vendor-lib");
const { PackedBox } = require("packed-lib");

const web = new Box("web", { size: "t3.large" });
const workers = ["a", "b", "c"].map((n) => new Box(`worker-${n}`, { size: "t3.small" }));
const pair = new Pair("edge");
const vendored = new VendorBox("vendored");
const packed = new PackedBox("packed");

exports.web = web.id;
