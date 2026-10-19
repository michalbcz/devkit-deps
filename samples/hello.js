#!/usr/bin/env node20
// node20 asks for Node 20. Pin the same way with --runtime-version 20.
// devkit-deps: ["left-pad@1.3.0"]

const leftPad = require("left-pad");
console.log(leftPad("hi", 5, "."));
