import pulumi
import pulumi_command as command


class Pair(pulumi.ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("shop:index:Pair", name, None, opts)
        for side in ("left", "right"):
            command.local.Command(f"{name}-{side}", create=f"echo {side}", opts=pulumi.ResourceOptions(parent=self))
