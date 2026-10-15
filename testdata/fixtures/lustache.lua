-- devkit-deps: ["lustache@1.3.1-0"]
local lustache = require("lustache")
print(lustache:render("Release {{version}}: {{feature}}", {
    version = "2.0", feature = "Offline tools"
}))
