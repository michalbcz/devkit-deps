#!/usr/bin/env php8.3
<?php
// php8.3 asks for PHP 8.3, a prefix of the version php prints.
// The package line stays after <?php. Composer installs vendor/package@version.
// devkit-deps: ["symfony/yaml@7.4.21"]
// devkit-deps run prepends php/vendor/autoload.php. DEVKIT_DEPS_DIR is that cache.
// This require loads the same autoloader. php itself skips these comments.

require getenv('DEVKIT_DEPS_DIR') . '/php/vendor/autoload.php';

use Symfony\Component\Yaml\Yaml;

echo Yaml::parse("hello: world\n")['hello'], "\n";
