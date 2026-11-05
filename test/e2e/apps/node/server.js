// App mínima para el e2e de versaDeploy (sin dependencias).
const http = require('node:http');

http.createServer((_req, res) => {
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end(`versa-e2e node ${process.version}\n`);
}).listen(Number(process.env.PORT || 3000), '127.0.0.1');
