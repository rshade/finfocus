const pulumi = require("@pulumi/pulumi");

const web = new Box("web");
const worker = new Box("worker");

exports.done = true;
