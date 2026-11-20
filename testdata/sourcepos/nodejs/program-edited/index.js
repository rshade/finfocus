const { Box } = require("./lib/box");
const { Pair } = require("./lib/pair");
const { VendorBox } = require("vendor-lib");
const { PackedBox } = require("packed-lib");

// Public entry point.
// Sized for the launch traffic.
const web = new Box("web", { size: "t3.xlarge" });
const workers = ["a", "b", "c"].map((n) => new Box(`worker-${n}`, { size: "t3.small" }));
const pair = new Pair("edge");
const vendored = new VendorBox("vendored");
const packed = new PackedBox("packed");

const extra = new Box("extra", { size: "t3.micro" });

exports.web = web.id;
