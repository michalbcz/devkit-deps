# devkit-deps: ["git::https://github.com/janet-lang/spork.git::v1.3.0"]
# Run installs this Git tag with jpm and points Janet at the local module tree.
(import spork/base64 :as base64)

(def encoded (base64/encode "Majkl DevKit"))
(print (string "Encoded: " encoded))
(print (string "Decoded: " (base64/decode encoded)))
