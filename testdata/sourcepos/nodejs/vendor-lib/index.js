const pulumi = require("@pulumi/pulumi");
class VendorBox extends pulumi.dynamic.Resource {
  constructor(name, args, opts) {
    super({ create: async () => ({ id: "v", outs: {} }) }, name, args || {}, opts);
  }
}
module.exports = { VendorBox };
