; devkit-deps: ["https://github.com/netwide-assembler/nasm.git#fbdc88565ccccfa0e8c69b21b1953d1eded02e89"]
; .asm is NASM. The checkout is an -i directory, for example %include "misc/c32.mac".
; This is x86-64. macOS links with cc -arch x86_64, so Apple silicon needs Rosetta.

default rel

%ifidn __OUTPUT_FORMAT__, macho64
global _main
_main:
%else
global main
main:
%endif
    xor eax, eax
    ret
