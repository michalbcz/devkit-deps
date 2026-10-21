#!/usr/bin/env node20
// node20 asks for Node 20. Node reads a bare major from the program name.
// devkit-deps: ["left-pad@1.3.0"]
// devkit-deps run installs the package with npm and sets NODE_PATH.
// require() loads it from there. node itself skips these comments.

const leftPad = require("left-pad");
console.log(leftPad("hi", 5, "."));
