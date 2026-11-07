<?php
// App mínima para el e2e de versaDeploy
header('Content-Type: text/plain');
// sessions are written by PHP-FPM's user: a wrong pool group breaks them (RHEL)
if (!session_start()) {
    http_response_code(500);
    exit("session_start failed\n");
}
echo 'versa-e2e php ' . PHP_VERSION . "\n";
