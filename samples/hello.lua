#!/usr/bin/env lua5.4
-- lua5.4 asks for Lua 5.4.
-- devkit-deps: ["lustache@1.3.1-0"]

local lustache = require("lustache")
print(lustache:render("Hello, {{name}}!", { name = "world" }))
