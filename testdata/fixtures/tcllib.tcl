# devkit-deps: ["tcllib@2.0"]
package require sha256
set digest [::sha2::sha256 -hex "abc"]
puts "SHA-256: $digest"
set changed [::sha2::sha256 -hex "abcd"]
puts "Changed input differs: [expr {$digest ne $changed}]"
