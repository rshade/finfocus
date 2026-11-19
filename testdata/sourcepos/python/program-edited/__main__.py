import pulumi
import pulumi_command as command
from lib.pair import Pair

# moved down by two lines
# and resized
web = command.local.Command("web", create="echo web-v2")
edge = Pair("edge")
extra = command.local.Command("extra", create="echo extra")
pulumi.export("web", web.stdout)
