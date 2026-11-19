using Pulumi;
using Pulumi.Command.Local;

return await Deployment.RunAsync(() =>
{
    var web = new Command("web", new CommandArgs { Create = "echo web" });
    var edge = new Pair("edge");
    return new Dictionary<string, object?> { ["web"] = web.Stdout };
});
