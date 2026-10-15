# devkit-deps: ["ripgrep@15.1.0"]
# The downloaded tool is available on this script's PATH.
printf '%s\n' 'INFO service ready' 'ERROR disk quota exceeded' 'INFO retrying' 'ERROR upload failed' |
  rg --no-config --color never '^ERROR'
