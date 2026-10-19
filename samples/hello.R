#!/usr/bin/env Rscript
# devkit-deps: ["praise@1.0.0"]
# Pin R with: devkit-deps run --runtime-version 4.4 hello.R

cat(praise::praise(), "\n", sep = "")
