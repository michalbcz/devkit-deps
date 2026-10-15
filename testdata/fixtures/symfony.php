<?php
// devkit-deps: ["symfony/yaml@7.4.21"]
// Composer installs the package locally; load that environment's generated autoloader.
require getenv('DEVKIT_DEPS_DIR') . '/php/vendor/autoload.php';

use Symfony\Component\Yaml\Yaml;

$config = Yaml::parse("tool: Majkl DevKit\nmanaged: true\n");
echo 'Tool: ' . $config['tool'] . PHP_EOL;
echo 'Managed: ' . ($config['managed'] ? 'yes' : 'no') . PHP_EOL;
