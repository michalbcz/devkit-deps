#!/usr/bin/env pwsh
# devkit-deps: ["powershell-yaml@0.4.12"]
# Pin PowerShell with: devkit-deps run --runtime-version 7.4 hello.ps1

Import-Module powershell-yaml -RequiredVersion 0.4.12 -ErrorAction Stop
Write-Output (ConvertFrom-Yaml "hello: world").hello
