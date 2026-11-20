const pulumi = require("@pulumi/pulumi");
class PackedBox extends pulumi.dynamic.Resource {
  constructor(name, args, opts) {
    super({ create: async () => ({ id: "p", outs: {} }) }, name, args || {}, opts);
  }
}
module.exports = { PackedBox };
