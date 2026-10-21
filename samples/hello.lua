#!/usr/bin/env lua5.4
-- lua5.4 asks for Lua 5.4, a prefix of the version lua prints.
-- devkit-deps: ["lustache@1.3.1-0"]
-- devkit-deps run installs the rock with LuaRocks and sets LUA_PATH.
-- require() loads it from there. lua itself skips these comments.

local lustache = require("lustache")
print(lustache:render("Hello, {{name}}!", { name = "world" }))
