using Pulumi;
using Pulumi.Command.Local;

class Pair : ComponentResource
{
    public Pair(string name) : base("shop:index:Pair", name)
    {
        foreach (var side in new[] { "left", "right" })
        {
            new Command($"{name}-{side}", new CommandArgs { Create = $"echo {side}" }, new CustomResourceOptions { Parent = this });
        }
    }
}
