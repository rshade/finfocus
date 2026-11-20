import pulumi
import pulumi_command as command
from lib.pair import Pair

web = command.local.Command("web", create="echo web")
edge = Pair("edge")
pulumi.export("web", web.stdout)
