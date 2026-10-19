#!/usr/bin/env php8.3
<?php
// php8.3 asks for PHP 8.3.
// devkit-deps: ["symfony/yaml@7.4.21"]

require getenv('DEVKIT_DEPS_DIR') . '/php/vendor/autoload.php';

use Symfony\Component\Yaml\Yaml;

echo Yaml::parse("hello: world\n")['hello'], "\n";
