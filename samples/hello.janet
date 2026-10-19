#!/usr/bin/env janet
# devkit-deps: ["git::https://github.com/janet-lang/spork.git::v1.3.0"]
# Pin Janet with: devkit-deps run --runtime-version 1.35.0 hello.janet

(import spork/base64 :as base64)
(print (base64/encode "hi"))
