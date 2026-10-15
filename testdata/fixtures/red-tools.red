Red [Title: "Pinned Git source library"]
; devkit-deps: ["https://github.com/rebolek/red-tools.git#565774c8fb6f1817e04393fa5f034fe6d363c17f"]
; Git checks out the pinned source; Red explicitly loads the library file.
library: to-file rejoin [get-env "DEVKIT_DEPS_DIR" "/sources/0/format.red"]
do library
expanded: detab/count copy "^-hello^/" 2
print rejoin ["Expanded length: " length? expanded]
