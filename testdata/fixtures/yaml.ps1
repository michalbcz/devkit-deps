# devkit-deps: ["powershell-yaml@0.4.12"]
Import-Module powershell-yaml -RequiredVersion 0.4.12 -ErrorAction Stop
$config = ConvertFrom-Yaml "name: MajklDevKit`nitems: [3, 5, 8]"
Write-Output "App: $($config.name)"
Write-Output "Items: $($config.items.Count)"
